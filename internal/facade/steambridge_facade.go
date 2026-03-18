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

func NewFacade(config Config) (*Facade, error) {
	table := switchboard.NewTable()

	log.Printf("Setting up TAP interface: %s\n", config.IfaceName)
	tapDev, err := tap.NewDevice(config.IfaceName, config.IfaceID)
	if err != nil {
		return nil, fmt.Errorf("could not create TAP device: %w", err)
	}

	router := switchboard.NewRouter(tapDev, nil, table)

	log.Println("Initializing Steamworks API...")
	client := steam.NewClient(router)

	if config.BootstrapPeerID != 0 {
		client.AddPeer(config.BootstrapPeerID)
	}

	router.SetSteamSender(client)

	log.Printf("SteamBridge is live on interface '%s'! Press Ctrl+C to exit.\n", config.IfaceName)

	return &Facade{
		ifaceName:       config.IfaceName,
		ifaceID:         config.IfaceID,
		tapDev:          tapDev,
		router:          router,
		client:          client,
		table:           table,
		wg:              sync.WaitGroup{},
		cancelFunc:      nil,
		bootstrapPeerID: config.BootstrapPeerID,
	}, nil
}

func (f *Facade) Start(ctx context.Context) error {
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
