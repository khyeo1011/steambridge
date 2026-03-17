package switchboard

import (
	"context"
	"steambridge/internal/dpi"
	"steambridge/internal/protocol"
	"steambridge/internal/tap"
)

type SteamSender interface {
	SendToPeer(steamID uint64, frame []byte)
	SendToAll(frame []byte)
}

type Router struct {
	tap   *tap.Device
	steam SteamSender
	table *Table
}

func NewRouter(tap *tap.Device, steam SteamSender, table *Table) *Router {
	return &Router{
		tap:   tap,
		steam: steam,
		table: table,
	}
}

func (r *Router) HandleIngress(senderID uint64, frame []byte) {
	if len(frame) < 14 {
		return
	}
	if len(frame) < 60 {
		padded := make([]byte, 60)
		copy(padded, frame)
		frame = padded
	}
	var sourceMAC [6]byte
	copy(sourceMAC[:], frame[6:12])
	r.table.Update(sourceMAC, senderID)
	r.tap.Write(frame)
}

func (r *Router) StartEgress(ctx context.Context) {
	frame := make([]byte, 2048)

	for {
		n, err := r.tap.Read(frame[1:])
		if err != nil {
			return
		}
		if n < 14 {
			continue
		}
		if !dpi.IsValidLan(frame[1:]) {
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
