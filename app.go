package main

import (
	"context"
	"fmt"
	"steambridge/internal/facade"
	"steambridge/internal/utils"
	"strconv"

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
	// SteamID emitted as a string to avoid JS float64 precision loss.
	a.facade.SetJoinHandler(func(id uint64) {
		runtime.EventsEmit(a.ctx, "joinRequest", fmt.Sprintf("%d", id))
	})
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

// JoinLobby initiates a join with a host by SteamID. The ID is passed as a
// string because 17-digit SteamIDs exceed JS's safe integer range.
func (a *App) JoinLobby(steamID string) error {
	id, err := strconv.ParseUint(steamID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid Steam ID %q: %w", steamID, err)
	}
	runtime.LogDebugf(a.ctx, "Attempting to join Steam ID %d", id)
	return a.facade.Join(id)
}

func (a *App) OpenFriendsOverlay() {
	a.facade.OpenFriendsOverlay()
}
