package facade

import (
	"context"
	"fmt"
	"log"
	"os"
	"steambridge/internal/protocol"
	"steambridge/internal/router"
	"steambridge/internal/steam"
	"steambridge/internal/tun"
	"sync"
	"sync/atomic"
)

type Config struct {
	IfaceName       string
	IfaceID         string
	BootstrapPeerID uint64
}

type Facade struct {
	ifaceName        string
	ifaceID          string
	tunDev           tun.TunInterface
	router           *router.Router
	client           *steam.Client
	table            *router.Table
	wg               sync.WaitGroup
	cancelFunc       context.CancelFunc
	bootstrapPeerID  uint64
	readyChan        chan struct{}
	running          atomic.Bool
	onJoinRequest    func(uint64)
	onJoinResult     func(uint64, bool)
	onJoinConfirm    func(uint64)
	onDisconnect     func()
	abnormalExitOnce sync.Once
}

// SetJoinHandler registers a callback invoked when a join is initiated. Must be
// called before Start so it can be wired to the client at creation.
func (f *Facade) SetJoinHandler(fn func(uint64)) {
	f.onJoinRequest = fn
}

// SetJoinResultHandler registers a callback invoked once per join with the outcome.
// Must be called before Start.
func (f *Facade) SetJoinResultHandler(fn func(uint64, bool)) {
	f.onJoinResult = fn
}

// SetJoinConfirmHandler registers a callback invoked on the host when a non-friend
// requests a P2P session. Must be called before Start.
func (f *Facade) SetJoinConfirmHandler(fn func(uint64)) {
	f.onJoinConfirm = fn
}

// SetDisconnectHandler registers a callback invoked once if an engine goroutine
// exits abnormally (e.g. Steam P2P session dropped). The bridge is torn down
// automatically; the callback is a signal to update UI or log the event.
func (f *Facade) SetDisconnectHandler(fn func()) {
	f.onDisconnect = fn
}

func NewFacade(config Config) *Facade {
	table := router.NewTable()

	return &Facade{
		ifaceName:       config.IfaceName,
		ifaceID:         config.IfaceID,
		table:           table,
		bootstrapPeerID: config.BootstrapPeerID,
		wg:              sync.WaitGroup{},
	}
}

func (f *Facade) Start(ctx context.Context) error {
	if f.running.Load() {
		return nil
	}
	log.Printf("Setting up TUN interface: %s\n", f.ifaceName)
	tunDev, err := tun.NewTUN(f.ifaceName, f.ifaceID)
	if err != nil {
		wd, _ := os.Getwd()
		log.Printf("CWD: %s", wd)
		return fmt.Errorf("could not create TUN device: %w", err)
	}
	f.tunDev = tunDev

	f.router = router.NewRouter(f.tunDev, nil)

	log.Println("Initializing Steamworks API...")
	client, err := steam.NewClient(f.router)
	if err != nil {
		f.tunDev.Close()
		return fmt.Errorf("steam client init: %w", err)
	}
	f.client = client
	f.client.SetJoinHandler(f.onJoinRequest)
	f.client.SetJoinResultHandler(f.onJoinResult)
	f.client.SetJoinConfirmHandler(f.onJoinConfirm)
	f.client.SetJoinable(true)

	if f.bootstrapPeerID != 0 {
		f.client.AddPeer(f.bootstrapPeerID)
	}

	f.router.SetSteamSender(f.client)
	log.Printf("SteamBridge is live on interface '%s'! Waiting for shutdown.\n", f.ifaceName)
	f.running.Store(true)
	f.abnormalExitOnce = sync.Once{} // reset for this Start/Stop cycle
	engineCtx, cancel := context.WithCancel(ctx)
	f.cancelFunc = cancel
	f.wg.Add(2)
	go func() {
		defer f.wg.Done()
		if err := f.router.StartEgress(engineCtx); err != nil && engineCtx.Err() == nil {
			log.Printf("egress loop exited abnormally: %v", err)
			f.abnormalExitOnce.Do(func() {
				if f.onDisconnect != nil {
					f.onDisconnect()
				}
				go f.Stop() //nolint:errcheck
			})
		}
	}()

	go func() {
		defer f.wg.Done()
		if err := f.client.ReadLoop(engineCtx); err != nil && engineCtx.Err() == nil {
			log.Printf("read loop exited abnormally: %v", err)
			f.abnormalExitOnce.Do(func() {
				if f.onDisconnect != nil {
					f.onDisconnect()
				}
				go f.Stop() //nolint:errcheck
			})
		}
	}()

	if f.bootstrapPeerID != 0 {
		f.client.SendControlMessage(f.bootstrapPeerID, protocol.ActionRequestIP, 0)
		log.Printf("Bootstrapped peer %v. Requesting IP address...", f.bootstrapPeerID)
	}

	return nil
}

func (f *Facade) Stop() error {
	// Idempotent: only the first call does real work.
	if !f.running.CompareAndSwap(true, false) {
		return nil
	}

	if f.client != nil {
		f.client.SetJoinable(false)
		f.client.Disconnect() // tell all peers we are leaving
	}
	if f.bootstrapPeerID != 0 && f.client != nil {
		f.client.SendControlMessage(f.bootstrapPeerID, protocol.ActionNackIP, 0)
	}

	// Cancel the engine context so ReadLoop exits its ctx.Done() check.
	if f.cancelFunc != nil {
		f.cancelFunc()
	}

	// Wake any goroutine blocked in tunDev.Read so StartEgress can return.
	if f.tunDev != nil {
		f.tunDev.Unblock() //nolint:errcheck
	}

	// Wait for both engine goroutines to exit before freeing resources.
	f.wg.Wait()

	// Now safe to free — no goroutines are touching these.
	if f.tunDev != nil {
		f.tunDev.Close() //nolint:errcheck
	}
	if f.client != nil {
		f.client.Close()
	}

	return nil
}

func (f *Facade) Join(host uint64) error {
	if !f.running.Load() {
		return fmt.Errorf("bridge not running")
	}
	f.client.Join(host)
	return nil
}

// RespondToJoinRequest forwards the host's accept/reject decision to the client.
func (f *Facade) RespondToJoinRequest(host uint64, accept bool) error {
	if !f.running.Load() {
		return fmt.Errorf("bridge not running")
	}
	f.client.RespondToJoinRequest(host, accept)
	return nil
}

func (f *Facade) OpenFriendsOverlay() {
	if f.client != nil {
		f.client.OpenFriendsOverlay()
	}
}

func (f *Facade) AddPort(port uint16) {
	f.router.AddPort(port)
}

func (f *Facade) RemovePort(port uint16) {
	f.router.RemovePort(port)
}

func (f *Facade) SetFirewall(enabled bool) {
	f.router.SetFirewall(enabled)
}

func (f *Facade) GetLocalSteamID() uint64 {
	return f.client.GetLocalSteamID()
}

func (f *Facade) IsRunning() bool {
	return f.running.Load()
}

func (f *Facade) GetLocalIP() uint32 {
	if f.client == nil {
		return 0
	}
	return f.client.GetLocalIP()
}

func (f *Facade) GetPeerTable() map[uint32]uint64 {
	if f.router == nil {
		return nil
	}
	return f.router.GetPeers()
}

func (f *Facade) GetFirewallState() bool {
	if f.router == nil {
		return false
	}
	return f.router.GetFirewallState()
}

func (f *Facade) GetAllowedPorts() []uint16 {
	if f.router == nil {
		return nil
	}
	return f.router.GetAllowedPorts()
}
