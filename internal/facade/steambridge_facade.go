package facade

import (
	"context"
	"fmt"
	"log"
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
	ifaceName       string
	ifaceID         string
	tunDev          tun.TunInterface
	router          *router.Router
	client          *steam.Client
	table           *router.Table
	wg              sync.WaitGroup
	cancelFunc      context.CancelFunc
	bootstrapPeerID uint64
	readyChan       chan struct{}
	running         atomic.Bool
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
	log.Printf("Setting up TAP interface: %s\n", f.ifaceName)
	tunDev, err := tun.NewTUN(f.ifaceName, f.ifaceID)
	if err != nil {
		return fmt.Errorf("could not create TAP device: %w", err)
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

	if f.bootstrapPeerID != 0 {
		f.client.AddPeer(f.bootstrapPeerID)
	}

	f.router.SetSteamSender(f.client)
	log.Printf("SteamBridge is live on interface '%s'! Waiting for shutdown.\n", f.ifaceName)
	f.running.Store(true)
	engineCtx, cancel := context.WithCancel(ctx)
	f.cancelFunc = cancel
	f.wg.Add(2)
	go func() {
		defer f.wg.Done()
		f.router.StartEgress(engineCtx)
	}()

	go func() {
		defer f.wg.Done()
		f.client.ReadLoop(engineCtx)
	}()

	if f.bootstrapPeerID != 0 {
		f.client.SendControlMessage(f.bootstrapPeerID, protocol.ActionRequestIP, 0)
		log.Printf("Bootstrapped peer %v. Requesting IP address...", f.bootstrapPeerID)
	}

	return nil
}

func (f *Facade) Stop() error {
	if f.bootstrapPeerID != 0 {
		f.client.SendControlMessage(f.bootstrapPeerID, protocol.ActionNackIP, 0)
	}
	if f.cancelFunc != nil {
		f.cancelFunc()
	}

	if f.tunDev != nil {
		f.tunDev.Close()
	}

	if f.client != nil {
		f.client.Close()
	}

	f.wg.Wait()
	f.running.Store(false)

	return nil
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
