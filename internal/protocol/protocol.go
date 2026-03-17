package protocol

const (
	PacketTypeData    byte = 0x01
	PacketTypeControl byte = 0x02
)

type ControlMessage struct {
	Action uint8  `json:"action"`
	IP     uint32 `json:"ip"`
}

const (
	ActionRequestIP = 0
	ActionOfferIP   = 1
	ActionAckIP     = 2
)
