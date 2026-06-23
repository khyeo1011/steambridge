# SteamBridge — Phase 2 Handoff

## What Was Done (Phase 1 — `refactor` branch)

All pre-merge stabilisation items from `design.md` are complete. CI is green.

| Fix | File(s) |
|-----|---------|
| Egress dest IP offset corrected (`[13:17]` → `[17:21]`) — packets now route to peers instead of broadcasting everything | `internal/router/router.go` |
| `NewClient` converted from panic to `(*Client, error)` | `internal/steam/client.go` |
| Facade propagates Steam init error; closes TUN on failure | `internal/facade/steambridge_facade.go` |
| `app.go` handles `Start()` error; guards `GetLocalSteamID()` call | `app.go` |
| Removed `RealeaseSteamID` (misspelled, dead) | `internal/ipam/pool.go` |
| DPI tests updated for TUN (raw IP) vs old TAP (Ethernet-framed) data | `internal/dpi/inspector_test.go` |
| CI wintun.dll copied to correct path (`internal/tun/`); added `$ErrorActionPreference = 'Stop'` | `.github/workflows/main.yml` |

---

## What Was Done (Phase 2.1 — `frontend` branch)

GUI dashboard is now functional. The bridge no longer auto-starts; the user controls it via the UI.

| Change | File(s) |
|--------|---------|
| `Table.Snapshot()` — thread-safe copy of IP→SteamID table for GUI reads | `internal/router/table.go` |
| `Router.GetFirewallState()`, `GetAllowedPorts()`, `GetPeers()` — read-only accessors for dashboard polling | `internal/router/router.go` |
| `Client.localIP atomic.Uint32` — stores assigned IP when IPAM ACK is confirmed; exposed via `GetLocalIP()` | `internal/steam/client.go` |
| `Facade.running atomic.Bool` — double-start guard; `IsRunning()`, `GetLocalIP()`, `GetPeerTable()`, `GetFirewallState()`, `GetAllowedPorts()` | `internal/facade/steambridge_facade.go` |
| `StatusPayload` / `PeerInfo` types; new Wails-bound methods: `StartBridge`, `StopBridge`, `GetStatus`, `GetPeers`, `GetFirewallState`, `GetAllowedPorts`, `AddPort`, `RemovePort`, `ToggleFirewall`; removed `domReady` auto-start; removed `Greet` stub | `app.go` |
| Wails JS bindings updated to match new methods and types | `frontend/wailsjs/wailsjs/go/main/App.js`, `App.d.ts` |
| Browser mock (`mockApp` + `isWails()`) — full dashboard usable via `npm run dev` without a live bridge | `frontend/wailsjs/wailsjs/go/main/mock.ts` |
| Dashboard UI: status card, peer table, firewall toggle, port editor; 2s polling loop | `frontend/pages/index.tsx` |
| Minimal flat CSS, easy to restyle once design is finalised | `frontend/pages/index.module.css` |

**SteamID encoding note:** All SteamIDs cross the Wails RPC boundary as `string` (not `uint64`) to avoid JavaScript float64 precision loss on values > 2^53.

---

## Current Architecture

```
Next.js Dashboard
      │  (Wails RPC — 2s poll + immediate mutations)
      ▼
App (Wails) ──► Facade ──► Router ──► DPI + TUN
                       └──► SteamClient ──► steam_bridge.cpp (C++ Steamworks)
                                     └──► IPAM Pool
```

- **Egress** (OS → Steam): `TUN.Read` → DPI validate → table lookup by dest IP → `SendToPeer` / `SendToAll`
- **Ingress** (Steam → OS): `ReadLoop` polls bridge → control (IPAM) or data → DPI validate → table update by src IP → `TUN.Write`
- **IPAM**: guest sends `ActionRequestIP`, host responds `ActionOfferIP`, guest ACKs or NACKs; guest stores confirmed IP in `Client.localIP`
- **TUN framing invariant**: `TUN.Read` returns raw IPv4 (no PI, no Ethernet header). The router prepends a 1-byte `PacketTypeData` tag before sending over Steam.
- **GUI state reads**: read-only snapshot methods on `Router` and `Client`; all guarded with nil checks so they're safe to call before `StartBridge`.

---

## Phase 2 — Feature Completion

### ~~2.1 Real GUI Dashboard~~ ✓ Done

### 2.2 Steam Social Integration (`ISteamFriends`)

Right now `BootstrapPeerID` is hardcoded. Real join-from-Steam-overlay requires:
- Register a `GameRichPresenceJoinRequested_t` callback in the C++ bridge and expose it via `Bridge_GetJoinRequest()` (or a callback channel)
- Poll or receive the callback in `client.ReadLoop` (or a separate goroutine)
- Auto-trigger `ActionRequestIP` when a friend accepts the join invite
- Surface the "Join Friend" Steam invite from the GUI

**Key files:** `cbridge/steam_bridge.cpp`, `cbridge/steam_bridge.h`, `internal/steam/client.go`, `internal/steam/bridge.go`

### 2.3 Proper Error Recovery

Current gaps:
- No reconnect if the Steam P2P session drops mid-game
- No handling if the TUN device is removed (Linux hotplug)
- `Stop()` does not drain in-flight packets before closing

**What to add:**
- Detect `bytesRead < 0` return from `bridgeReceive` as a disconnect signal; attempt re-init
- Add a watchdog goroutine in `Facade` that restarts the bridge on error (with backoff)
- Flush the TUN write queue before `Close()`

---

## Phase 3 — Architecture (lower priority)

These are clean-up items that improve testability and long-term maintainability.

### 3.1 Replace `sudo` in Linux `SetIP` with netlink

`internal/tun/device_linux.go:SetIP` shells out to `sudo ip addr add`. The `netlink` library is already imported for `setupLink`; use `netlink.AddrAdd` directly instead.

```go
func (d *Device) SetIP(ip uint32) error {
    link, err := netlink.LinkByName(d.Name())
    if err != nil { return err }
    addr, err := netlink.ParseAddr(fmt.Sprintf("%s/24", utils.IntIPtoString(ip)))
    if err != nil { return err }
    if err := netlink.AddrAdd(link, addr); err != nil { return err }
    return netlink.LinkSetUp(link)
}
```

### 3.2 Wrap package-level bridge vars in a `SteamBridge` struct

`internal/steam/bridge.go` uses package-level `var` function pointers (`bridgeInit`, `bridgeSend`, …). Wrapping them in a struct makes the API testable and prevents silent overwrite if `LoadLibrary` is called twice.

### 3.3 Structured logging

Replace `log.Printf` calls with `log/slog`. Add packet-count and peer-event telemetry that the GUI dashboard can query.

---

## Known Limitations to Document Before Shipping

- **IPv6 is silently dropped** — `dpi.IsValidLan` returns false for all non-IPv4. Add a package-level comment or add support.
- **Windows requires admin** — Wintun adapter creation needs elevated privileges. Document or add a manifest.
- **Linux requires `CAP_NET_ADMIN`** — either `SetUID` the binary or document the capability requirement.
- **Steam app ID** — `steam_appid.txt` must be present alongside the binary and set to a valid app ID.
- **Host has no VPN IP** — The host node never receives an IPAM offer, so `GetLocalIP()` returns `0.0.0.0` for the host. Host IP assignment is not yet implemented.
