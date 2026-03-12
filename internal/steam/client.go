package steam

import (
	"steambridge/internal/switchboard"
	"sync"
	"time"

	"github.com/BenLubar/steamworks"
	"github.com/BenLubar/steamworks/steamnet"
)

type Client struct {
	router    *switchboard.Router
	peermutex sync.RWMutex
	steamIDs  map[uint64]bool
}

func NewClient(router *switchboard.Router) *Client {
	err := steamworks.InitClient(true)
	if err != nil {
		panic(err)
	}
	return &Client{
		router:   router,
		steamIDs: make(map[uint64]bool),
	}
}

func (c *Client) SendToPeer(steamID uint64, frame []byte, reliable bool) {
	c.peermutex.Lock()
	defer c.peermutex.Unlock()
	sendType := steamnet.Unreliable
	if reliable {
		sendType = steamnet.Reliable
	}

	target := steamworks.SteamID(steamID)

	steamnet.SendPacket(target, frame, sendType, 0)
}

func (c *Client) SendToAll(frame []byte, reliable bool) {
	sendType := steamnet.Unreliable
	if reliable {
		sendType = steamnet.Reliable
	}

	c.peermutex.RLock()
	defer c.peermutex.RUnlock()

	for steamID := range c.steamIDs {
		target := steamworks.SteamID(steamID)
		steamnet.SendPacket(target, frame, sendType, 0)
	}
}

func (c *Client) ReadLoop() {
	for {
		packet, steamID := steamnet.ReadPacket(0)

		if len(packet) == 0 {
			time.Sleep(time.Millisecond)
			continue
		}
		c.router.HandleIngress(uint64(steamID), packet)
	}
}
