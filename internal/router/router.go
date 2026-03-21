package router

import (
	"context"
	"encoding/binary"
	"steambridge/internal/dpi"
	"steambridge/internal/protocol"
	"steambridge/internal/tun"
	"sync"
	"sync/atomic"
)

type SteamSender interface {
	SendToPeer(steamID uint64, frame []byte)
	SendToAll(frame []byte)
}

type Router struct {
	tunDev          *tun.Device
	steam           SteamSender
	table           Table
	allowedPorts    sync.Map
	firewallEnabled atomic.Bool
}

func NewRouter(tap *tun.Device, steam SteamSender) *Router {
	return &Router{
		tunDev:          tap,
		steam:           steam,
		table:           *NewTable(),
		allowedPorts:    sync.Map{},
		firewallEnabled: atomic.Bool{},
	}
}

func (r *Router) HandleIngress(senderID uint64, packet []byte) {
	offset := 0

	// - Raw IPv4 starts with 0x45 (Version 4, IHL 5)
	// - PI/AF headers usually start with 0x00
	if len(packet) > 4 && (packet[0] == 0x00 || packet[0] == 0x02) {
		offset = 4
	}

	if len(packet) < 20+offset {
		return
	}

	if !dpi.IsValidLan(packet[offset:]) {
		return
	}

	if r.firewallEnabled.Load() && !dpi.IsAllowedPort(packet[offset:], &r.allowedPorts) {
		return
	}
	if len(packet) < 60 {
		padded := make([]byte, 60)
		copy(padded, packet)
		packet = padded
	}
	var ip uint32
	ip = binary.BigEndian.Uint32(packet[offset+12 : offset+16])
	r.table.Update(ip, senderID)
	r.tunDev.Write(packet)
}

func (r *Router) StartEgress(ctx context.Context) {
	frame := make([]byte, 2048)

	for {
		n, err := r.tunDev.Read(frame[1:])
		if err != nil {
			return
		}
		if n < 14 {
			continue
		}
		if !dpi.IsValidLan(frame[1:]) {
			continue
		}
		if r.firewallEnabled.Load() && !dpi.IsAllowedPort(frame[1:], &r.allowedPorts) {
			continue
		}
		frame[0] = protocol.PacketTypeData
		// Isolate the actual read bytes
		payload := frame[:n+1]

		var destMAC [6]byte
		copy(destMAC[:], payload[1:7])

		if destMAC == [6]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF} {
			r.steam.SendToAll(payload)
		} else {
			steamID, ok := r.table.Lookup(destMAC)
			if ok {
				r.steam.SendToPeer(steamID, payload)
			} else {
				r.steam.SendToAll(payload)
			}
		}

	}

}

func (r *Router) SetSteamSender(s SteamSender) {
	r.steam = s
}

func (r *Router) GetTap() *tun.Device {
	return r.tunDev
}

func (r *Router) AddPort(port uint16) {
	r.allowedPorts.Store(port, true)
}

func (r *Router) RemovePort(port uint16) {
	r.allowedPorts.Delete(port)
}

func (r *Router) SetFirewall(enabled bool) {
	r.firewallEnabled.Store(enabled)
}
