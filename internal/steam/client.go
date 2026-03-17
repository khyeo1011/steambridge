package steam

import (
	"context"
	"encoding/binary"
	"log"
	"steambridge/internal/protocol"
	"steambridge/internal/switchboard"
	"sync"
	"time"
)

type Client struct {
	router    *switchboard.Router
	peermutex sync.RWMutex
	steamIDs  map[uint64]bool
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
				if len(packetCopy) < 14 {
					log.Printf("Warning: Dropping undersized control frame (%d bytes)", len(packetCopy))
					continue
				}
				msg := protocol.ControlMessage{
					Action:  packetCopy[1],
					IP:      binary.BigEndian.Uint32(packetCopy[2:6]),
					SteamID: binary.BigEndian.Uint64(packetCopy[6:14]),
				}
				switch msg.Action {
				case protocol.ActionRequestIP:
					// TODO
				case protocol.ActionOfferIP:
					// TODO
				case protocol.ActionAckIP:
					// TODO
				default:
					// Invalid
					log.Printf("Warning: Unknown control action '%s' from %v", msg.Action, remoteSteamID)
				}
			default:
				// Invalid
			}

		}
	}
}

func (c *Client) SendControlMessage(steamID uint64, action uint8, ip uint32) {
	frame := make([]byte, 14)
	frame[0] = protocol.PacketTypeControl
	msg := protocol.ControlMessage{
		Action:  action,
		IP:      ip,
		SteamID: steamID,
	}
	frame[1] = byte(msg.Action)
	binary.BigEndian.PutUint32(frame[2:6], msg.IP)
	binary.BigEndian.PutUint64(frame[6:14], msg.SteamID)
	c.SendToPeer(steamID, frame)
}

func (c *Client) Close() {
	bridgeShutdown()
}
