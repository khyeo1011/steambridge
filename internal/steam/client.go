package steam

import (
	"context"
	"encoding/binary"
	"log"
	"net"
	"steambridge/internal/ipam"
	"steambridge/internal/protocol"
	"steambridge/internal/switchboard"
	"sync"
	"time"
)

type Client struct {
	router    *switchboard.Router
	peermutex sync.RWMutex
	steamIDs  map[uint64]bool
	ipPool    *ipam.Pool
}

func NewClient(router *switchboard.Router) *Client {
	err := LoadLibrary()
	if err != nil {
		panic(err)
	}

	if !bridgeInit() {
		panic("Bridge_Init failed")
	}

	return &Client{
		router:   router,
		steamIDs: make(map[uint64]bool),
		ipPool:   ipam.NewPool(),
	}
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

	// sendType := 0
	// Let TCP handle reliability
	// if reliable {
	// 	sendType = 1
	// }

	bridgeSend(steamID, &frame[0], len(frame))
}

func (c *Client) SendToPeerReliable(steamID uint64, frame []byte) {
	if len(frame) == 0 {
		return
	}
	bridgeSendReliable(steamID, &frame[0], len(frame))
}

func (c *Client) SendToAll(frame []byte) {
	// sendType := 0
	// Let TCP handle reliability
	// if reliable {
	// 	sendType = 1
	// }

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
					log.Printf("Assigned IP %s to %v", ipam.IntIPtoString(assignedIP), remoteSteamID)
				case protocol.ActionOfferIP:
					err := setTAPIP(msg.IP, c.router.GetTap())
					if err != nil {
						c.SendControlMessage(remoteSteamID, protocol.ActionNackIP, 0)
						continue
					}
					assigned := false
					for i := 0; i < 3; i++ {
						iface, err := net.InterfaceByName(c.router.GetTap().Name())
						addrs, err := iface.Addrs()
						if err != nil {
							log.Printf("Error getting addresses")
							continue
						}
						for _, addr := range addrs {
							if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
								if ipnet.IP.String() == ipam.IntIPtoString(msg.IP) {
									log.Printf("Received IP %s from %v", ipam.IntIPtoString(msg.IP), remoteSteamID)
									c.SendControlMessage(remoteSteamID, protocol.ActionAckIP, msg.IP)
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
					log.Printf("Received ACK for IP %s from %v", ipam.IntIPtoString(msg.IP), remoteSteamID)
				case protocol.ActionNackIP:
					if msg.IP != 0 {
						c.ipPool.Release(msg.IP)
						log.Printf("Releasing IP %s", ipam.IntIPtoString(msg.IP))
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
