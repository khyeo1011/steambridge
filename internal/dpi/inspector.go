package dpi

import (
	"encoding/binary"
	"sync"
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

func IsAllowedPort(frame []byte, allowedPorts *sync.Map) bool {
	if len(frame) < 14 {
		return false
	}

	etherType := binary.BigEndian.Uint16(frame[12:14])

	if etherType == 0x0806 {
		return true
	}

	if etherType != 0x0800 {
		return false
	}

	if len(frame) < 34 {
		return false
	}
	ipHeaderInfo := frame[14]
	ipHeaderLen := (ipHeaderInfo & 0x0F) * 4

	// 6 = TCP; 17 = UDP
	protocolInfo := frame[14+9]
	if protocolInfo != 6 && protocolInfo != 17 {
		return false
	}
	level4Offset := 14 + ipHeaderLen

	if len(frame) < int(level4Offset)+4 {
		return false
	}

	sourcePort := binary.BigEndian.Uint16(frame[level4Offset : level4Offset+2])
	destPort := binary.BigEndian.Uint16(frame[level4Offset+2 : level4Offset+4])
	srcRes, srcOk := allowedPorts.Load(sourcePort)
	if srcOk && srcRes.(bool) {
		return true
	}

	dstRes, dstOk := allowedPorts.Load(destPort)
	if dstOk && dstRes.(bool) {
		return true
	}

	return false
}
