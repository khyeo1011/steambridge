# SteamBridge - Codebase Design Evaluation

**Branch:** `frontend` (Phase 2.1 complete)
**Date:** 2026-06-13

---

## 1. Current Design Patterns

### 1.1 Architecture: Facade-Orchestrated Wails Desktop App

The app is a Wails desktop app (Go backend + Next.js frontend). `app.go:NewApp()` creates a `facade.Facade` which orchestrates all subsystems. The bridge is started explicitly by the user via `StartBridge()` — not auto-started on launch.

```
Next.js Dashboard
      │  (Wails RPC — 2s poll + immediate mutations)
      ▼
App (Wails) --> Facade --> Router --> DPI + TUN
                               |
                               +--> SteamClient --> steam_bridge.cpp (C++ Steamworks)
                                          |
                                          +--> IPAM Pool
```

**Wails RPC surface (app.go):**

| Method | Returns | Purpose |
|--------|---------|---------|
| `StartBridge()` | `error` | Starts Facade; idempotent (double-start is a no-op) |
| `StopBridge()` | `error` | Stops Facade and tears down TUN |
| `GetStatus()` | `StatusPayload` | Running state, local IP, Steam ID, peer count |
| `GetPeers()` | `[]PeerInfo` | All peers with Steam ID + assigned IP from router table |
| `GetFirewallState()` | `bool` | Current firewall on/off |
| `GetAllowedPorts()` | `[]uint16` | Current port allowlist |
| `AddPort(uint16)` | — | Add port to allowlist |
| `RemovePort(uint16)` | — | Remove port from allowlist |
| `ToggleFirewall(bool)` | — | Set firewall enabled/disabled |

SteamIDs cross the RPC boundary as `string` to avoid JS float64 precision loss.

### 1.2 Package Structure

| Package | Responsibility | Pattern |
|---------|---------------|---------|
| `internal/facade` | Lifecycle orchestration + running state | Facade |
| `internal/router` | Packet multiplexing + firewall + peer table | Bridge + Stateful Table |
| `internal/steam` | Steamworks SDK binding + local IP tracking | C-Bridge + purego |
| `internal/tun` | OS-level virtual NIC abstraction | Interface |
| `internal/dpi` | Layer 3/4 packet inspection | Stateless Filter |
| `internal/ipam` | IP lease management | Counter Pool |
| `internal/protocol` | Control message types | Constants |
| `internal/utils` | Helpers | Utility |
| `cbridge/` | C++ Steamworks shim | Native Bridge |
| `frontend/pages/` | Wails-rendered dashboard | Next.js |
| `frontend/wailsjs/` | Auto-generated Wails bindings + browser mock | Generated + Mock |

### 1.3 Data Flow

**Egress (OS --> Steam):**
1. `TunInterface.Read()` grabs raw packet from OS TUN device
2. `dpi.IsValidLan()` validates IPv4 source/destination is RFC1918
3. `dpi.IsAllowedPort()` checks TCP/UDP ports against firewall allowlist
4. Router looks up destination IP in `Table` for SteamID
5. `steam.Client.SendToPeer()` or `SendToAll()` transmits via Steamworks P2P

**Ingress (Steam --> OS):**
1. `Client.ReadLoop()` polls `Bridge_Receive()` for incoming P2P packets
2. Control messages (IPAM handshake) handled in ReadLoop switch
3. Data packets validated by DPI, source IP updated in `Table`, written to TUN device
4. On IPAM ACK confirmed: `Client.localIP.Store(msg.IP)` records this node's VPN IP

**GUI reads (dashboard polling, 2s interval):**
- `Router.GetPeers()` → `Table.Snapshot()` — copy of IP→SteamID map
- `Router.GetFirewallState()` — atomic bool read
- `Router.GetAllowedPorts()` — sync.Map range
- `Client.GetLocalIP()` — atomic uint32 read
- All accessors are nil-guarded on Facade; safe to call before bridge is started

### 1.4 Platform Abstraction

| Layer | Windows | Linux |
|-------|---------|-------|
| TUN device | `wintun.Adapter` | `water.Interface` + `netlink` |
| Library loading | `syscall.LoadLibrary` | `purego.Dlopen` |
| IP config | `netsh interface ip set address` | `sudo ip addr add` |

### 1.5 Frontend Development Mode

`frontend/wailsjs/wailsjs/go/main/mock.ts` provides a browser mock for all Wails bindings. `isWails()` detects the runtime by checking `window['go']`. The dashboard (`index.tsx`) routes all calls through this shim, so `npm run dev` gives a fully interactive UI without a live Go backend.

---

## 2. Technical Debt (Code-Level)

### 2.1 HIGH: IP Offset Mismatch Between Ingress and Egress Paths

**Status: Resolved in Phase 1.** Egress offset corrected to `payload[tagLen+16 : tagLen+20]`. Invariant documented in `router.go` via constants and comments.

### 2.2 HIGH: `steam.NewClient()` Panics on Library Load Failure

**Status: Resolved in Phase 1.** `NewClient` returns `(*Client, error)`; `Facade.Start()` propagates the error and closes the TUN device on failure.

### 2.3 MEDIUM: IPAM Deadlock Risk in ReadLoop

**File:** `internal/steam/client.go` — `ActionRequestIP` case

`Pool.Allocate()` acquires `p.mu.Lock()`. If `SendControlMessage` blocks (Steam P2P send queue full), the IPAM mutex is held during the send, potentially blocking other goroutines.

**Recommendation:** Copy the allocated IP out under the lock, release before calling `SendControlMessage`.

### 2.4 MEDIUM: Linux SetIP Requires sudo

**File:** `internal/tun/device_linux.go`

`exec.Command("sudo", "ip", "addr", "add", ...)` requires passwordless sudo. The `netlink` library is already imported — use `netlink.AddrAdd` directly instead. See Phase 3.1 in HANDOFF.md for the fix.

### 2.5 MEDIUM: No IPv6 Support

**Files:** `internal/dpi/inspector.go`, `internal/router/router.go`

IPv6 traffic is silently dropped. Modern games or services using IPv6 will break silently.

**Recommendation:** Add package-level comment documenting the IPv4-only invariant, or add IPv6 support.

### 2.6 LOW: `SendToPeerReliable()` Is Unused

**File:** `internal/steam/client.go`

`SendToPeerReliable()` is defined but only called from `SendControlMessage`. All data sends use `SendToPeer`. The method name suggests it was intended for data too. Either document the intent or remove.

### 2.7 LOW: Bridge.go Package-Level State

**File:** `internal/steam/bridge.go`

Function pointers (`bridgeInit`, `bridgeSend`, …) are stored at package level. If `LoadLibrary()` is called twice, they are silently overwritten. The `Facade.running` guard prevents double-start, but the underlying risk remains.

**Recommendation:** Wrap in a `SteamBridge` struct (see Phase 3.2 in HANDOFF.md).

### 2.8 LOW: Host Node Has No VPN IP

The IPAM protocol only assigns IPs to guest nodes (via `ActionRequestIP`/`ActionOfferIP`). The host node never receives an offer, so `Client.localIP` stays `0` and `GetStatus().localIP` shows `0.0.0.0` for the host.

**Recommendation:** Either assign the host a fixed IP (e.g. `10.8.0.1`) at bridge start, or derive it from the SteamID hash that is already computed in `app.go`.

---

## 3. Next Steps for the Project

### Phase 2: Feature Completion

#### ~~6. Implement Real GUI Dashboard~~ ✓ Done (Phase 2.1)

#### 7. Implement Steam Social Integration
- Add `ISteamFriends` callback for `GameRichPresenceJoinRequested_t`
- Auto-trigger IPAM handshake when a user clicks "Join Game" in Steam Overlay
- Surface the "Join Friend" invite from the dashboard

#### 8. Proper Error Recovery
- Detect `bytesRead < 0` from `bridgeReceive` and signal the `Facade`
- Add a watchdog goroutine in `Facade` that restarts the bridge on error (with backoff)
- Flush TUN write queue before `Close()`

### Phase 3: Architecture Improvements

#### 9. Replace `sudo` with netlink (Linux)
See HANDOFF.md §3.1.

#### 10. SteamBridge Struct (Replace Package-Level Vars)
See HANDOFF.md §3.2.

#### 11. Structured Logging
Replace `log.Printf` with `log/slog`; expose packet-count and peer-event telemetry to the dashboard.

#### 12. Host VPN IP Assignment
Assign `10.8.0.1` to the host at bridge start so `GetStatus().localIP` is populated for all node types.

---

## 4. Risk Summary

| Risk | Severity | Status |
|------|----------|--------|
| Panic on Steam SDK load | HIGH | ✓ Resolved (Phase 1) |
| IP offset mismatch | HIGH | ✓ Resolved (Phase 1) |
| IPAM mutex during P2P send | MEDIUM | Open |
| sudo dependency (Linux) | MEDIUM | Open |
| IPv6 silent drop | MEDIUM | Open |
| Host has no VPN IP | LOW | Open |
| Bridge package-level vars | LOW | Open |

---

## 5. Summary Assessment

The core networking stack is stable and the GUI dashboard is functional. The app is now a complete end-to-end product loop: users can start the bridge, observe connected peers, and manage firewall rules from the UI without touching the CLI.

The remaining high-value work is Steam Social Integration (2.2) — without it, `BootstrapPeerID` must be hardcoded, making the UX manual. Error recovery (2.3) is needed before the app is production-reliable.
