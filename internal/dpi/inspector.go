package dpi

import (
	"encoding/binary"
)

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

func IsValidLan(frame []byte) bool {
	if len(frame) < 14 {
		return false
	}

	etherType := binary.BigEndian.Uint16(frame[12:14])

	switch etherType {
	case 0x0806:
		return true

	case 0x0800:
		if len(frame) < 34 {
			return false
		}

		dstIP := frame[30:34]

		is10 := dstIP[0] == 10
		is172 := dstIP[0] == 172 && dstIP[1] >= 16 && dstIP[1] <= 31
		is192 := dstIP[0] == 192 && dstIP[1] == 168

		isBroadcast := dstIP[0] == 255 && dstIP[1] == 255 && dstIP[2] == 255 && dstIP[3] == 255

		return is10 || is172 || is192 || isBroadcast

	default:
		// Drop everything else (IPv6, STP, LLDP, etc.) to keep the tunnel quiet
		return false
	}
}
