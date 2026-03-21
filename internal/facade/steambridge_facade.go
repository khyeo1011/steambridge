package facade

import (
	"context"
	"fmt"
	"log"
	"steambridge/internal/protocol"
	"steambridge/internal/steam"
	"steambridge/internal/switchboard"
	"steambridge/internal/tap"
	"sync"
)

type Config struct {
	IfaceName       string
	IfaceID         string
	BootstrapPeerID uint64
}

type Facade struct {
	ifaceName       string
	ifaceID         string
	tapDev          *tap.Device
	router          *switchboard.Router
	client          *steam.Client
	table           *switchboard.Table
	wg              sync.WaitGroup
	cancelFunc      context.CancelFunc
	bootstrapPeerID uint64
}

func NewFacade(config Config) *Facade {
	table := switchboard.NewTable()

	return &Facade{
		ifaceName:       config.IfaceName,
		ifaceID:         config.IfaceID,
		table:           table,
		bootstrapPeerID: config.BootstrapPeerID,
		wg:              sync.WaitGroup{},
	}
}

func (f *Facade) Start(ctx context.Context) error {
	log.Printf("Setting up TAP interface: %s\n", f.ifaceName)
	tapDev, err := tap.NewDevice(f.ifaceName, f.ifaceID)
	if err != nil {
		return fmt.Errorf("could not create TAP device: %w", err)
	}
	f.tapDev = tapDev

	f.router = switchboard.NewRouter(f.tapDev, nil, f.table)

	log.Println("Initializing Steamworks API...")
	f.client = steam.NewClient(f.router)

	if f.bootstrapPeerID != 0 {
		f.client.AddPeer(f.bootstrapPeerID)
	}

	f.router.SetSteamSender(f.client)
	log.Printf("SteamBridge is live on interface '%s'! Waiting for GUI shutdown.\n", f.ifaceName)
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

	if f.tapDev != nil {
		f.tapDev.Close()
	}

	if f.client != nil {
		f.client.Close()
	}

	f.wg.Wait()

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
