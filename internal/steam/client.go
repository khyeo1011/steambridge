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
	StartEgress(ctx context.Context)
	AddPort(port uint16)
	RemovePort(port uint16)
	SetFirewall(enabled bool)
	SetIP(ip uint32) error
	GetDevName() string
}

type Client struct {
	router    RouterInterface
	peermutex sync.RWMutex
	steamIDs  map[uint64]bool
	ipPool    *ipam.Pool
	localIP   atomic.Uint32
}

func NewClient(router RouterInterface) (*Client, error) {
	if err := LoadLibrary(); err != nil {
		return nil, fmt.Errorf("steam bridge load: %w", err)
	}
	if !bridgeInit() {
		return nil, errors.New("Bridge_Init failed")
	}
	return &Client{
		router:   router,
		steamIDs: make(map[uint64]bool),
		ipPool:   ipam.NewPool(),
	}, nil
}

func (c *Client) AddPeer(steamID uint64) {
	c.peermutex.Lock()
	defer c.peermutex.Unlock()
	c.steamIDs[steamID] = true
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

func (c *Client) ReadLoop(ctx context.Context) {
	// Allocate a buffer slightly larger than standard Ethernet MTU (1500)
	buffer := make([]byte, 2048)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			bridgeRunCallbacks()

			var remoteSteamID uint64
			bytesRead := bridgeReceive(&buffer[0], len(buffer), &remoteSteamID)

			if bytesRead == 0 {
				time.Sleep(time.Millisecond) // Don't peg the CPU at 100%
				continue
			} else if bytesRead < 0 {
				log.Printf("Bytes received = %d, aborting\n", bytesRead)
				return
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
	return bridgeGetLocalSteamID()
}

func (c *Client) GetLocalIP() uint32 {
	return c.localIP.Load()
}
