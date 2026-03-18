# SteamBridge

[!NOTE]
Still very WIP and only has a CLI, that will be hard to set up without instructions.

SteamBridge is a high-performance, custom Layer 2/Layer 4 virtual tunneling application written in Go. It routes raw Ethernet frames over the Steam P2P network (via the Steamworks SDK), effectively turning the Steam backbone into a zero-configuration, secure Virtual Private LAN for gaming, or if you wish, personal use.



## 🚀 Core Features

* **Memory-Safe Hot Path:** SteamBridge is engineered to route packets without allocating structs or copying payloads unnecessarily. It uses exact slice windowing and direct bitwise operations to jump headers and inspect frames.

###  Layer 4 Deep Packet Inspection (DPI) & Firewall
* **Stateless Port Filtering:** Features a custom DPI engine that parses variable-length IPv4 headers on the fly. It utilizes a bidirectional, thread-safe map to instantly allow or drop game traffic without the overhead of connection tracking, while allowing critical network infrastructure like ARP or ICMP to pass through
* **Lock-Free Hot Toggle:** Includes a firewall kill-switch, allowing the access control engine to be toggled at runtime

### IPAM (IP Address Management)
* **Byte-Frugal Handshakes:** Custom protocol for an ultra-lightweight 6-byte binary control protocol
* **Thread-Safe Leasing:** The Host operates a mutex-locked IP pool utilizing bitwise math to generate and manage leased addresses dynamically.
* **Deterministic OS Execution:** Automatically executes cross-platform OS network commands (`netsh` on Windows, `ip addr` on Linux) to configure the local TAP adapter, utilizing a polling state machine to guarantee interface readiness before acknowledging leases.

##  Architecture
SteamBridge operates by creating a local virtual TAP adapter. Egress traffic from the OS is intercepted, inspected by the DPI engine, multiplexed, and transmitted via `ISteamNetworkingSockets`. Ingress traffic from remote Steam peers is demultiplexed, verified, and written directly back into the local OS network stack. 

## 🗺️ Roadmap
- [X] **TAP device configuration:** Automatically set up the tap device(assuming installation is complete)
- [X] **Allow for a direct P2P send or BroadCast**
- [X] **Dynamic IP address management:** Have the host allocate IP addresses 
- [ ] **Steam Social Integration:** Leveraging the `ISteamFriends` API and `GameRichPresenceJoinRequested_t` callbacks to allow users to dynamically trigger IPAM handshakes by simply clicking "Join Game" in the Steam Overlay.
- [ ] **GUI Dashboard:** Transitioning from the CLI to a graphical interface (Wails/Fyne) for seamless peer discovery and visual session management.
- _brainstorm more ideas that seem good_