package switchboard

import (
	"steambridge/internal/dpi"
	"steambridge/internal/tap"
)

type SteamSender interface {
	SendToPeer(steamID uint64, frame []byte, reliable bool)
	SendToAll(frame []byte, reliable bool)
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
	var sourceMAC [6]byte
	copy(sourceMAC[:], frame[6:12])
	r.table.Update(sourceMAC, senderID)
	r.tap.Write(frame)
}

func (r *Router) StartEgress() {
	frame := make([]byte, 2048)

	for {
		n, err := r.tap.Read(frame)
		if err != nil {
			return
		}
		if n < 14 {
			continue
		}

		// Isolate the actual read bytes
		payload := frame[:n]
		reliable := dpi.IsReliable(payload)

		var destMAC [6]byte
		copy(destMAC[:], payload[0:6])

		if destMAC == [6]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF} {
			r.steam.SendToAll(payload, reliable)
		} else {
			steamID, ok := r.table.Lookup(destMAC)
			if ok {
				r.steam.SendToPeer(steamID, payload, reliable)
			} else {
				r.steam.SendToAll(payload, reliable)
			}
		}
	}
}

func (r *Router) SetSteamSender(s SteamSender) {
	r.steam = s
}
