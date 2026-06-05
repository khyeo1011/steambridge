package main

import (
	"context"
	"fmt"
	"steambridge/internal/facade"
	"steambridge/internal/utils"

	"github.com/wailsapp/wails/v2/pkg/runtime"
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
	facade := facade.NewFacade(config)
	return &App{facade: facade}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) domReady(ctx context.Context) {
	runtime.LogDebug(ctx, "Wails UI booted. Engine standing by")
	a.InitNetwork()
}

func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	return false
}

func (a *App) shutdown(ctx context.Context) {
	runtime.LogDebug(ctx, "Shutdown signal received from GUI. Tearing down TAP interface...")

	a.facade.Stop()
}

func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

func (a *App) InitNetwork() {
	runtime.LogDebug(a.ctx, "Initializing TAP interface")
	if err := a.facade.Start(a.ctx); err != nil {
		runtime.LogErrorf(a.ctx, "Failed to start network: %v", err)
		return
	}
	x, y := utils.SteamIDToTapCoords(a.facade.GetLocalSteamID())
	runtime.LogDebugf(a.ctx, "Initializing tap interface IP: 10.209.%d.%d", x, y)
}

func (a *App) JoinLobby(steamID uint64) error {
	runtime.LogDebugf(a.ctx, "Attempting to join Steam ID %d(0 means hosting)", steamID)
	return fmt.Errorf("not implemented")
}
