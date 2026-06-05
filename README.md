# SteamBridge

> [!IMPORTANT]
> **Current Status:** The project is in progress. See the [Roadmap](#-roadmap) for what's done and what's next.

SteamBridge is a high-performance, custom Layer 2/Layer 4 virtual tunneling application written in Go. It routes raw Ethernet frames over the Steam P2P network (via the Steamworks SDK), effectively turning the Steam backbone into a zero-configuration, secure Virtual Private LAN for gaming.

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
| `cbridge/` | C++ Steamworks shim (`ISteamNetworkingSockets` P2P) |
| `frontend/` | Wails-rendered Next.js dashboard (stub) |

### Data Flow

**Egress (OS --> Remote Peer):**
1. `TUN.Read()` grabs raw packet from OS
2. `DPI.IsValidLan()` validates IPv4 source/destination is RFC1918
3. `DPI.IsAllowedPort()` checks TCP/UDP ports against firewall allowlist
4. Router looks up destination IP in NAT table for SteamID
5. `Client.SendToPeer()` or `SendToAll()` transmits via Steamworks P2P

**Ingress (Remote Peer --> OS):**
1. `Client.ReadLoop()` polls `Bridge_Receive()` for incoming P2P packets
2. Control messages (IPAM handshake) handled in ReadLoop switch
3. Data packets validated by DPI, source IP updated in NAT table, written to TUN device

---

## Core Features

### Layer 4 Deep Packet Inspection (DPI) & Firewall
- **Stateless Port Filtering** — parses variable-length IPv4 headers on the fly, uses a thread-safe `sync.Map` for instant port lookups without connection tracking
- **Infrastructure Passthrough** — ARP, ICMP, and other non-TCP/UDP traffic always allowed
- **Lock-Free Toggle** — firewall can be enabled/disabled at runtime via `atomic.Bool`

### IP Address Management (IPAM)
- **6-Byte Binary Control Protocol** — ultra-lightweight handshake: `RequestIP -> OfferIP -> AckIP/NackIP`
- **Thread-Safe Lease Pool** — mutex-locked IP pool, assigns `10.8.0.x` addresses dynamically
- **Deterministic Assignment** — polling with retry loop guarantees interface readiness before acknowledging

---

## 🗺️ Roadmap

### Phase 1: Stabilize Core (In Progress)
- [X] TUN device configuration (Windows + Linux)
- [X] Direct P2P send and broadcast support
- [X] Dynamic IP address management
- [X] Refactor: interface-based TUN abstraction, stateless DPI, clean Router
- [ ] **Fix panic on Steam SDK load failure** — return error instead of `panic()`
- [ ] **Fix IP offset invariant** — document/verify framing between ingress and egress paths
- [ ] **Fix IPAM lock scope** — release mutex before blocking P2P send

### Phase 2: Feature Completion
- [ ] **GUI Dashboard** — real-time peer list, IP assignments, firewall controls
- [ ] **Steam Social Integration** — auto-trigger IPAM handshake via `ISteamFriends` "Join Game" callback
- [ ] **Proper error recovery** — graceful shutdown on Steam P2P disconnect, TUN device removal

### Phase 3: Architecture Improvements
- [ ] **Platform abstraction layer** — replace `exec.Command("netsh"/"sudo")` with native Go (`netlink`)
- [ ] **SteamBridge struct** — replace package-level function pointers with testable struct
- [ ] **IPv6 support** — currently silently dropped
- [ ] **Logging & observability** — structured logging, packet counters, connection telemetry

---

## ⚠️ Known Issues

| Issue | Severity | Details |
|-------|----------|---------|
| Panic on Steam SDK load | HIGH | `NewClient()` panics if `LoadLibrary()` or `Bridge_Init()` fails — should return error |
| IP offset fragility | HIGH | Egress offset `payload[13:17]` is hardcoded to TUN framing; breaks if driver layout changes |
| IPAM deadlock risk | MEDIUM | `Pool.Allocate()` holds mutex during blocking P2P send |
| Linux requires sudo | MEDIUM | `device_linux.go:74` uses `sudo ip addr add` — requires passwordless sudoers entry |
| IPv6 silently dropped | MEDIUM | DPI and Router both reject IPv6; no documentation of this invariant |
