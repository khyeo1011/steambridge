package steam

import (
	"context"
	"log"
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

func (c *Client) SendToPeer(steamID uint64, frame []byte, reliable bool) {
	if len(frame) == 0 {
		return
	}

	sendType := 0
	// Let TCP handle reliability
	// if reliable {
	// 	sendType = 1
	// }

	bridgeSend(steamID, &frame[0], len(frame), sendType)
}

func (c *Client) SendToAll(frame []byte, reliable bool) {
	sendType := 0
	// Let TCP handle reliability
	// if reliable {
	// 	sendType = 1
	// }

	c.peermutex.RLock()
	defer c.peermutex.RUnlock()

	for steamID := range c.steamIDs {
		bridgeSend(steamID, &frame[0], len(frame), sendType)
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

			c.router.HandleIngress(remoteSteamID, packetCopy)

			c.peermutex.Lock()
			c.steamIDs[remoteSteamID] = true
			c.peermutex.Unlock()
		}
	}
}

func (c *Client) Close() {
	bridgeShutdown()
}
