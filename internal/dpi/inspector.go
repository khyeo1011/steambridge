package dpi

func IsReliable(frame []byte) bool {
	if len(frame) < 14 {
		return true
	}
	var ethernetType uint16 = uint16(frame[12])<<8 | uint16(frame[13])
	if ethernetType == 0x0800 && len(frame) >= 24 {
		if uint8(frame[23]) == 17 {
			return false
		} else {
			return true
		}
	}
	if ethernetType == 0x86DD && len(frame) >= 21 {
		if uint8(frame[20]) == 17 {
			return false
		} else {
			return true
		}
	}
	return true
}
