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
	tunDev          tun.TunInterface
	steam           SteamSender
	table           Table
	allowedPorts    sync.Map
	firewallEnabled atomic.Bool
}

func NewRouter(tun tun.TunInterface, steam SteamSender) *Router {
	return &Router{
		tunDev:          tun,
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
	packet := make([]byte, 2048)

	for {
		n, err := r.tunDev.Read(packet[1:])
		if err != nil {
			return
		}
		if !dpi.IsValidLan(packet[1:]) {
			continue
		}
		if r.firewallEnabled.Load() && !dpi.IsAllowedPort(packet[1:], &r.allowedPorts) {
			continue
		}
		packet[0] = protocol.PacketTypeData
		// Isolate the actual read bytes
		payload := packet[:n+1]

		// payload[0] = PacketTypeData tag, payload[1:] = raw IPv4 packet.
		// IPv4 destination address is at header offset 16 (bytes 16–19).
		const tagLen = 1
		if len(payload) < tagLen+20 {
			continue
		}
		var destIP uint32
		destIP = binary.BigEndian.Uint32(payload[tagLen+16 : tagLen+20])

		if destIP == 0xFFFFFFFF {
			r.steam.SendToAll(payload)
		} else {
			steamID, ok := r.table.Lookup(destIP)
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

func (r *Router) AddPort(port uint16) {
	r.allowedPorts.Store(port, true)
}

func (r *Router) RemovePort(port uint16) {
	r.allowedPorts.Delete(port)
}

func (r *Router) SetFirewall(enabled bool) {
	r.firewallEnabled.Store(enabled)
}

func (r *Router) SetIP(ip uint32) error {
	return r.tunDev.SetIP(ip)
}

func (r *Router) GetDevName() string {
	return r.tunDev.Name()
}

func (r *Router) GetFirewallState() bool {
	return r.firewallEnabled.Load()
}

func (r *Router) GetAllowedPorts() []uint16 {
	var ports []uint16
	r.allowedPorts.Range(func(key, _ any) bool {
		ports = append(ports, key.(uint16))
		return true
	})
	return ports
}

func (r *Router) GetPeers() map[uint32]uint64 {
	return r.table.Snapshot()
}
