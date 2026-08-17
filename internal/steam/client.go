package steam

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"net"
	"steambridge/internal/ipam"
	"steambridge/internal/protocol"
	"steambridge/internal/router"
	"steambridge/internal/utils"
	"sync"
	"sync/atomic"
	"time"
)

type RouterInterface interface {
	HandleIngress(senderID uint64, packet []byte)
	SetSteamSender(s router.SteamSender)
	StartEgress(ctx context.Context) error
	AddPort(port uint16)
	RemovePort(port uint16)
	SetFirewall(enabled bool)
	SetIP(ip uint32) error
	GetDevName() string
}

// joinResultTimeout is how long the guest waits for an IPAM offer.
// Raised to 45s so a human host has time to click Accept/Reject.
const joinResultTimeout = 45 * time.Second

// joinRetryInterval is how often the guest retries ActionRequestIP while waiting.
// The host defers P2P acceptance until the user approves, so the first request is
// silently dropped; retries guarantee a fresh packet arrives after acceptance.
const joinRetryInterval = 2 * time.Second

type Client struct {
	router        RouterInterface
	peermutex     sync.RWMutex
	steamIDs      map[uint64]bool
	ipPool        *ipam.Pool
	localIP       atomic.Uint32
	steamID       uint64
	onJoinRequest func(uint64)
	onJoinResult  func(uint64, bool) // fired once per join: true=connected, false=failed/timeout
	onJoinConfirm func(uint64)       // fired on host when a non-friend requests a session
	joinMutex     sync.Mutex
	pendingJoins  map[uint64]bool // hosts we sent ActionRequestIP to but haven't resolved yet
}

func NewClient(router RouterInterface) (*Client, error) {
	if err := LoadLibrary(); err != nil {
		return nil, fmt.Errorf("steam bridge load: %w", err)
	}
	if !bridgeInit() {
		return nil, errors.New("Bridge_Init failed")
	}
	return &Client{
		router:       router,
		steamIDs:     make(map[uint64]bool),
		pendingJoins: make(map[uint64]bool),
		ipPool:       ipam.NewPool(),
		steamID:      bridgeGetLocalSteamID(),
	}, nil
}

// SetJoinHandler registers a callback invoked whenever a join is initiated
// (via Steam "Join Game" or a manual join). Optional; nil is a no-op.
func (c *Client) SetJoinHandler(fn func(uint64)) {
	c.onJoinRequest = fn
}

// SetJoinResultHandler registers a callback invoked exactly once per join attempt
// with the outcome: connected=true on IPAM ACK, false on NACK or timeout.
func (c *Client) SetJoinResultHandler(fn func(uint64, bool)) {
	c.onJoinResult = fn
}

// SetJoinConfirmHandler registers a callback invoked on the host whenever an
// incoming P2P session request comes from a non-friend. The host GUI should
// prompt the user and then call RespondToJoinRequest with the decision.
func (c *Client) SetJoinConfirmHandler(fn func(uint64)) {
	c.onJoinConfirm = fn
}

// resolveJoin fires onJoinResult exactly once per host, idempotently.
// The joinMutex guards pendingJoins so a timeout and a real ACK/NACK cannot
// both fire the callback if they race.
func (c *Client) resolveJoin(host uint64, connected bool) {
	c.joinMutex.Lock()
	if _, ok := c.pendingJoins[host]; !ok {
		c.joinMutex.Unlock()
		return
	}
	delete(c.pendingJoins, host)
	c.joinMutex.Unlock()
	if c.onJoinResult != nil {
		c.onJoinResult(host, connected)
	}
}

// joinTimeout fires a failed result if no ACK/NACK arrives within joinResultTimeout.
func (c *Client) joinTimeout(host uint64) {
	time.Sleep(joinResultTimeout)
	c.resolveJoin(host, false)
}

// joinRetry resends ActionRequestIP every joinRetryInterval until the join resolves.
// Needed because the host defers P2P session acceptance until the user approves, so
// the guest's initial packet is silently dropped; retries flow through once accepted.
func (c *Client) joinRetry(host uint64) {
	ticker := time.NewTicker(joinRetryInterval)
	defer ticker.Stop()
	for range ticker.C {
		c.joinMutex.Lock()
		_, pending := c.pendingJoins[host]
		c.joinMutex.Unlock()
		if !pending {
			return
		}
		c.SendControlMessage(host, protocol.ActionRequestIP, 0)
	}
}

// Join begins the IPAM handshake with host: add it as a peer and request an IP.
// Shared by the Steam rich-presence path and the manual "Join by Steam ID" path.
func (c *Client) Join(host uint64) {
	c.AddPeer(host)
	c.joinMutex.Lock()
	c.pendingJoins[host] = true
	c.joinMutex.Unlock()
	c.SendControlMessage(host, protocol.ActionRequestIP, 0)
	go c.joinTimeout(host)
	go c.joinRetry(host)
	if c.onJoinRequest != nil {
		c.onJoinRequest(host)
	}
}

// RespondToJoinRequest is called by the host GUI after prompting the user.
// accept=true opens the P2P session and registers the peer; accept=false closes it silently.
func (c *Client) RespondToJoinRequest(host uint64, accept bool) {
	if accept {
		bridgeAcceptSession(host)
		c.AddPeer(host)
	} else {
		bridgeRejectSession(host)
	}
}

// SetJoinable advertises (or hides) this node in friends' "Join Game" menus.
func (c *Client) SetJoinable(joinable bool) {
	bridgeSetJoinable(joinable)
}

// OpenFriendsOverlay opens the Steam friends overlay so the user can pick a friend.
func (c *Client) OpenFriendsOverlay() {
	bridgeOpenFriendsOverlay()
}

func (c *Client) AddPeer(steamID uint64) {
	c.peermutex.Lock()
	defer c.peermutex.Unlock()
	c.steamIDs[steamID] = true
}

func (c *Client) removePeer(steamID uint64) {
	c.peermutex.Lock()
	defer c.peermutex.Unlock()
	delete(c.steamIDs, steamID)
}

// Disconnect broadcasts ActionDisconnect to all known peers so they can clean
// up their side. Carries the local IP so receivers can release the IPAM lease.
func (c *Client) Disconnect() {
	c.peermutex.RLock()
	ids := make([]uint64, 0, len(c.steamIDs))
	for id := range c.steamIDs {
		ids = append(ids, id)
	}
	c.peermutex.RUnlock()

	localIP := c.localIP.Load()
	for _, id := range ids {
		c.SendControlMessage(id, protocol.ActionDisconnect, localIP)
	}
}

func (c *Client) SendToPeer(steamID uint64, frame []byte) {
	if len(frame) == 0 {
		return
	}
	bridgeSend(steamID, &frame[0], len(frame))
}

func (c *Client) SendToPeerReliable(steamID uint64, frame []byte) {
	if len(frame) == 0 {
		return
	}
	bridgeSendReliable(steamID, &frame[0], len(frame))
}

func (c *Client) SendToAll(frame []byte) {
	c.peermutex.RLock()
	defer c.peermutex.RUnlock()

	for steamID := range c.steamIDs {
		bridgeSend(steamID, &frame[0], len(frame))
	}
}

func (c *Client) ReadLoop(ctx context.Context) error {
	// Allocate a buffer slightly larger than standard Ethernet MTU (1500)
	buffer := make([]byte, 2048)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			bridgeRunCallbacks()

			// A friend clicked "Join Game" in Steam; start the handshake with them.
			if host := bridgeGetJoinRequest(); host != 0 {
				log.Printf("Steam join requested for host %v", host)
				c.Join(host)
			}

			// Drain pending P2P session requests queued by C++ (OnP2PSessionRequest
			// no longer auto-accepts; Go owns the accept/reject policy).
			for {
				peer := bridgeGetSessionRequest()
				if peer == 0 {
					break
				}
				if bridgeIsFriend(peer) {
					bridgeAcceptSession(peer)
					c.AddPeer(peer)
				} else if c.onJoinConfirm != nil {
					c.onJoinConfirm(peer)
				}
			}

			var remoteSteamID uint64
			bytesRead := bridgeReceive(&buffer[0], len(buffer), &remoteSteamID)

			if bytesRead == 0 {
				time.Sleep(time.Millisecond) // Don't peg the CPU at 100%
				continue
			} else if bytesRead < 0 {
				return fmt.Errorf("bridge receive returned %d: Steam P2P session dropped", bytesRead)
			}
			log.Printf("Steam Received %d bytes from %v", bytesRead, remoteSteamID)
			packetCopy := make([]byte, bytesRead)
			copy(packetCopy, buffer[:bytesRead])

			switch packetCopy[0] {
			case protocol.PacketTypeData:
				// Layer 2 Ethernet Frame, pass to TAP
				// Go slicing is actually really efficient here because it creates a header instead of copying
				c.router.HandleIngress(remoteSteamID, packetCopy[1:])

				c.peermutex.Lock()
				c.steamIDs[remoteSteamID] = true
				c.peermutex.Unlock()
			case protocol.PacketTypeControl:
				// IPAM Control Message for IP address assignment.
				if len(packetCopy) < 6 {
					log.Printf("Warning: Dropping undersized control frame (%d bytes)", len(packetCopy))
					continue
				}
				msg := protocol.ControlMessage{
					Action: packetCopy[1],
					IP:     binary.BigEndian.Uint32(packetCopy[2:6]),
				}
				switch msg.Action {
				case protocol.ActionRequestIP:
					assignedIP := c.ipPool.Allocate(remoteSteamID)
					c.SendControlMessage(remoteSteamID, protocol.ActionOfferIP, assignedIP)
					log.Printf("Assigned IP %s to %v", utils.IntIPtoString(assignedIP), remoteSteamID)
				case protocol.ActionOfferIP:
					err := c.router.SetIP(msg.IP)
					if err != nil {
						c.SendControlMessage(remoteSteamID, protocol.ActionNackIP, 0)
						c.resolveJoin(remoteSteamID, false)
						continue
					}
					assigned := false
					for i := 0; i < 3; i++ {
						iface, err := net.InterfaceByName(c.router.GetDevName())
						addrs, err := iface.Addrs()
						if err != nil {
							log.Printf("Error getting addresses")
							continue
						}
						for _, addr := range addrs {
							if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
								if ipnet.IP.String() == utils.IntIPtoString(msg.IP) {
									log.Printf("Received IP %s from %v", utils.IntIPtoString(msg.IP), remoteSteamID)
									c.SendControlMessage(remoteSteamID, protocol.ActionAckIP, msg.IP)
									c.localIP.Store(msg.IP)
									c.resolveJoin(remoteSteamID, true)
									assigned = true
									break
								}
							}
						}
						if assigned {
							break
						}
						time.Sleep(time.Second)
					}
					if !assigned {
						c.SendControlMessage(remoteSteamID, protocol.ActionNackIP, msg.IP)
						c.resolveJoin(remoteSteamID, false)
					}
				case protocol.ActionAckIP:
					log.Printf("Received ACK for IP %s from %v", utils.IntIPtoString(msg.IP), remoteSteamID)
				case protocol.ActionNackIP:
					if msg.IP != 0 {
						c.ipPool.Release(msg.IP)
						log.Printf("Releasing IP %s", utils.IntIPtoString(msg.IP))
					} else {
						log.Printf("Peceived 0 as nack op")
					}
				case protocol.ActionDisconnect:
					log.Printf("Peer %d disconnected; releasing IP %s", remoteSteamID, utils.IntIPtoString(msg.IP))
					c.ipPool.Release(msg.IP)
					c.removePeer(remoteSteamID)
					// resolveJoin is a no-op if they were never in pendingJoins.
					c.resolveJoin(remoteSteamID, false)

				default:
					// Invalid
					log.Printf("Warning: Unknown control action '%d' from %v", msg.Action, remoteSteamID)
				}
			default:
				// Invalid
			}

		}
	}
}

func (c *Client) SendControlMessage(steamID uint64, action uint8, ip uint32) {
	frame := make([]byte, 6)
	frame[0] = protocol.PacketTypeControl
	frame[1] = byte(action)
	binary.BigEndian.PutUint32(frame[2:6], ip)
	c.SendToPeerReliable(steamID, frame)
}

func (c *Client) Close() {
	bridgeShutdown()
}

func (c *Client) GetLocalSteamID() uint64 {
	return c.steamID
}

func (c *Client) GetLocalIP() uint32 {
	return c.localIP.Load()
}

// SetLocalIP records this node's VPN IP. Used by the facade for host
// self-assignment; guests get theirs recorded via the ActionOfferIP flow.
func (c *Client) SetLocalIP(ip uint32) {
	c.localIP.Store(ip)
}
