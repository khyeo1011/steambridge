# SteamBridge

> [!IMPORTANT]
> **Current Status:** Core data path, IPAM, the GUI dashboard, and Steam social join are working and under test.
> Remaining work is platform-native networking, IPv6, reconnect, and observability — see the [Roadmap](#-roadmap) and [Known Issues](#️-known-issues).

SteamBridge is a high-performance, custom Layer 3 virtual tunneling application written in Go. It routes raw IP frames over the Steam P2P network (via the Steamworks SDK), effectively turning the Steam backbone into a zero-configuration, secure Virtual Private LAN for gaming.

---

## Architecture

```
+----------+     +-----------+     +--------------+     +----------------+
|  Wails   | --> |  Facade  | --> |   Router     | --> |  Steam Client  |
|  (Go+JS) |     |  (Lifecycle) | (Firewall +   |     |  (C++ Bridge)  |
+----------+     +-----------+     |  NAT Table)  |     +----------------+
                                   +--------------+            |
                                          |                    v
                                   +--------------+     +----------------+
                                   |   DPI Engine |     |  Steam P2P     |
                                   |  (L3/L4 Filter)| --> |  Network       |
                                   +--------------+     +----------------+
                                          |
                                   +--------------+
                                   |   TUN Device | (wintun / water)
                                   +--------------+
```

### Core Modules

| Package | Responsibility |
|---------|---------------|
| `internal/facade` | Lifecycle orchestration — starts/stops all subsystems |
| `internal/router` | Packet multiplexing, firewall rules, IP-to-SteamID NAT table |
| `internal/steam` | C++ Steamworks SDK bridge via `purego` dynamic loading |
| `internal/tun` | OS-level virtual NIC abstraction (Wintun on Windows, Water on Linux) |
| `internal/dpi` | Stateless Layer 3/4 packet inspection — validates RFC1918 sources, port filtering |
| `internal/ipam` | IP lease pool — assigns `10.8.0.x` addresses to connected peers |
| `internal/protocol` | 6-byte binary control protocol for IPAM handshake |
| `cbridge/` | C++ Steamworks shim (`ISteamNetworkingMessages` P2P, `ISteamFriends` rich presence / join callbacks) |
| `frontend/` | Wails-rendered Next.js dashboard — live status, peer table, firewall and join controls |

### Data Flow

**Egress (OS --> Remote Peer):**
1. `TUN.Read()` grabs raw packet from OS
2. `DPI.IsValidLan()` validates IPv4 source/destination is RFC1918
3. `DPI.IsAllowedPort()` checks TCP/UDP ports against firewall allowlist
4. Router looks up destination IP in NAT table for SteamID
5. `Client.SendToPeer()` or `SendToAll()` transmits via Steamworks P2P

**Ingress (Remote Peer --> OS):**
1. `Client.ReadLoop()` polls `Bridge_Receive()` for incoming P2P packets — adaptive backoff keeps the loop hot under load and idles down to 32 ms between polls when quiet
2. Control messages (IPAM handshake) handled in ReadLoop switch
3. Data packets validated by DPI, source IP updated in NAT table, written to TUN device

---

## Core Features

### Layer 4 Deep Packet Inspection (DPI) & Firewall
- **Stateless Port Filtering** — parses variable-length IPv4 headers on the fly, uses a thread-safe `sync.Map` for instant port lookups without connection tracking
- **Infrastructure Passthrough** — ARP, ICMP, and other non-TCP/UDP traffic always allowed
- **Lock-Free Toggle** — firewall can be enabled/disabled at runtime via `atomic.Bool`

### IP Address Management (IPAM)
- **6-Byte Binary Control Protocol** — ultra-lightweight handshake: `RequestIP -> OfferIP -> AckIP/NackIP`, plus `Disconnect` for graceful teardown
- **Thread-Safe Lease Pool** — mutex-locked IP pool, assigns `10.8.0.x` addresses dynamically; the host self-assigns `10.8.0.1`
- **Deterministic Assignment** — polling with retry loop guarantees interface readiness before acknowledging
- **Authenticated Lease Release** — leases are freed only for the authenticated P2P sender, never the (forgeable) IP in the payload
- **Routing-Table Anti-Poisoning** — a peer can only claim an IP not already bound to another SteamID

### Steam Social Integration
- **Rich Presence** — a running host advertises a `connect` key so friends see **Join Game** in their Steam friends list
- **Join Callbacks** — `GameRichPresenceJoinRequested` auto-triggers the IPAM handshake; no manual SteamID entry
- **Friends Overlay** — one-click access to the Steam overlay for picking a peer
- **Host Gatekeeping** — non-friend join requests surface an Accept/Reject prompt on the host before a session is allowed

### GUI Dashboard
- **Wails Desktop App** — Go engine + Next.js UI, with a browser mock mode for frontend development
- **Live Status** — bridge state, local SteamID / VPN IP, and connected-peer table polled every 2s
- **Runtime Firewall Control** — toggle the firewall and add/remove allowed ports without restarting
- **Join Flow** — join by SteamID or overlay, with per-connection status (pending / awaiting / connected / failed)

### Lifecycle & Error Recovery
- **Struct-Based Facade** — `facade.Facade` orchestrates TUN, router, and Steam client; idempotent `Start`/`Stop` guarded by `atomic.Bool`
- **Abnormal-Exit Detection** — if an engine goroutine dies (e.g. Steam P2P drop), the bridge tears itself down once and fires a disconnect callback to the UI/CLI
- **Graceful Shutdown** — `Stop` broadcasts `Disconnect`, cancels the engine context, unblocks the TUN reader, and waits for goroutines before freeing resources

---

## Building & Running

**Prerequisites:** Go 1.23+, Node.js, CMake, a C++ toolchain, the [Wails CLI](https://wails.io),
and a running Steam client. The Steamworks SDK ships encrypted (`cbridge/steamworks_sdk.zip.gpg`)
and must be decrypted with the project passphrase before building the bridge:

```bash
cd cbridge && gpg --decrypt --output sdk.zip steamworks_sdk.zip.gpg && unzip -q sdk.zip
```

```bash
# C++ Steamworks bridge -> libsteam_bridge.so
cd cbridge && bash build.sh

# Desktop app (engine + dashboard)
bash scripts/build.sh          # wraps `wails build` + copies runtime .so files

# Headless CLI (Linux dev harness)
sudo ./scripts/setup_bridge.sh <REMOTE_STEAM_ID>
```

## Testing

```bash
sudo go test ./internal/...                       # Go engine (TUN tests need root)
cd cbridge && cmake -B build-tests -DSTEAMBRIDGE_BUILD_TESTS=ON \
  && cmake --build build-tests --target steam_bridge_tests \
  && ctest --test-dir build-tests --output-on-failure   # C++ bridge, fake SDK
cd frontend && npm test                            # dashboard (vitest)
```

CI (`.github/workflows`) runs the Go and C++ suites plus Linux/Windows Wails builds on every push.

---

## 🗺️ Roadmap

### Phase 1: Stabilize Core (Done)
Hardened the data path against oversized/fragmented packets, PI-header leaks, routing-table
poisoning, and forged-IP lease release; fixed facade lifecycle races and IPAM offer-retry
panics. Test suites now cover every Go package, the C++ bridge (`ctest` against a fake SDK),
and the frontend (`vitest`), all wired into CI.

### Phase 2: Feature Completion
- [x] **GUI Dashboard** — live peer table, IP assignments, runtime firewall controls
- [x] **Steam Social Integration** — rich-presence `connect` key + `GameRichPresenceJoinRequested` auto-triggers the IPAM handshake; host Accept/Reject gate for non-friends
- [x] **Error recovery** — abnormal-exit detection tears down the bridge and notifies the UI/CLI; graceful ordered shutdown
- [ ] **Reconnect** — no automatic re-join after a transient Steam P2P drop

### Phase 3: Architecture Improvements
- [x] **Steam networking API migration** — legacy P2P `ISteamNetworking` → `ISteamNetworkingMessages`; `ReadLoop`'s flat 1 ms poll replaced with adaptive backoff (#58)
- [ ] **Platform abstraction layer** — IP config still shells out to `sudo ip` (Linux) / `netsh` (Windows); replace with native Go (`netlink`)
- [x] **Testable facade** — package-level function pointers replaced by the `facade.Facade` struct (the `internal/steam` bridge still uses `purego`-bound function vars)
- [ ] **IPv6 support** — currently silently dropped by the DPI layer
- [ ] **Logging & observability** — still `log.Printf`; no structured logging, packet counters, or connection telemetry

---

## ⚠️ Known Issues

- **IPv6 is dropped.** `dpi.IsValidLan` only passes IPv4 RFC1918 traffic; all IPv6 frames are silently discarded.
- **Privileged IP configuration.** Linux needs passwordless `sudo` for `ip addr`/`ip link`; Windows needs an elevated process for `netsh`.
- **"Join Game" needs a real shared App ID.** The bundled test App ID (`480`) will not surface the Steam "Join Game" button; friends must join by SteamID until a real app ID is configured.
- **Abrupt peer loss lags.** If a guest process is killed (not stopped gracefully), the host only notices once the underlying Steam P2P session times out.
- **Linux TUN tests require root.** `internal/tun` tests fail with `operation not permitted` unless run as `sudo go test ./internal/...` (as CI does).

