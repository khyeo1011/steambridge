package main

import (
	"context"
	"fmt"
	"log"
	"steambridge/internal/facade"
)

// App struct
type App struct {
	ctx    context.Context
	facade *facade.Facade
}

func NewApp() *App {
	config := facade.Config{
		IfaceName:       "steambridge0",
		IfaceID:         "tap0901",
		BootstrapPeerID: 0,
	}
	facade, err := facade.NewFacade(config)
	if err != nil {
		log.Fatalf("Error creating facade: %s", err)
	}
	return &App{facade: facade}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	log.Println("Wails UI booting. Starting SteamBridge engine...")

	if err := a.facade.Start(context.Background()); err != nil {
		log.Fatalf("Error starting facade: %s", err)
	}

}

func (a *App) domReady(ctx context.Context) {
}

func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	return false
}

func (a *App) shutdown(ctx context.Context) {
	log.Println("Shutdown signal received from GUI. Tearing down TAP interface...")

	a.facade.Stop()
}

func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}
