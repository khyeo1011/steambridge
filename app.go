package main

import (
	"context"
	"fmt"
	"steambridge/internal/facade"
	"steambridge/internal/utils"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type StatusPayload struct {
	Running   bool   `json:"running"`
	LocalIP   string `json:"localIP"`
	SteamID   string `json:"steamID"`
	PeerCount int    `json:"peerCount"`
}

type PeerInfo struct {
	SteamID string `json:"steamID"`
	IP      string `json:"ip"`
}

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
	f := facade.NewFacade(config)
	return &App{facade: f}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) domReady(ctx context.Context) {
	runtime.LogDebug(ctx, "Wails UI booted. Engine standing by")
}

func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	return false
}

func (a *App) shutdown(ctx context.Context) {
	runtime.LogDebug(ctx, "Shutdown signal received from GUI. Tearing down TAP interface...")
	a.facade.Stop()
}

func (a *App) StartBridge() error {
	runtime.LogDebug(a.ctx, "Starting bridge")
	if err := a.facade.Start(a.ctx); err != nil {
		runtime.LogErrorf(a.ctx, "Failed to start network: %v", err)
		return err
	}
	return nil
}

func (a *App) StopBridge() error {
	runtime.LogDebug(a.ctx, "Stopping bridge")
	return a.facade.Stop()
}

func (a *App) GetStatus() StatusPayload {
	peerTable := a.facade.GetPeerTable()
	localIP := a.facade.GetLocalIP()
	steamID := uint64(0)
	if a.facade.IsRunning() {
		steamID = a.facade.GetLocalSteamID()
	}
	return StatusPayload{
		Running:   a.facade.IsRunning(),
		LocalIP:   utils.IntIPtoString(localIP),
		SteamID:   fmt.Sprintf("%d", steamID),
		PeerCount: len(peerTable),
	}
}

func (a *App) GetPeers() []PeerInfo {
	table := a.facade.GetPeerTable()
	peers := make([]PeerInfo, 0, len(table))
	for ip, steamID := range table {
		peers = append(peers, PeerInfo{
			SteamID: fmt.Sprintf("%d", steamID),
			IP:      utils.IntIPtoString(ip),
		})
	}
	return peers
}

func (a *App) GetFirewallState() bool {
	return a.facade.GetFirewallState()
}

func (a *App) GetAllowedPorts() []uint16 {
	ports := a.facade.GetAllowedPorts()
	if ports == nil {
		return []uint16{}
	}
	return ports
}

func (a *App) AddPort(port uint16) {
	a.facade.AddPort(port)
}

func (a *App) RemovePort(port uint16) {
	a.facade.RemovePort(port)
}

func (a *App) ToggleFirewall(enabled bool) {
	a.facade.SetFirewall(enabled)
}

func (a *App) JoinLobby(steamID uint64) error {
	runtime.LogDebugf(a.ctx, "Attempting to join Steam ID %d (0 means hosting)", steamID)
	return fmt.Errorf("not implemented")
}
