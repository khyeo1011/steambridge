package dpi

import (
	"encoding/binary"
	"sync"
)

func IsReliable(packet []byte) bool {
	if len(packet) < 1 {
		return true
	}

	// The first 4 bits of the IP header indicate the version (IPv4 or IPv6)
	version := packet[0] >> 4

	if version == 4 && len(packet) >= 10 {
		// IPv4 Protocol field is at offset 9
		if packet[9] == 17 { // 17 = UDP
			return false
		} else {
			return true
		}
	}

	if version == 6 && len(packet) >= 7 {
		// IPv6 Next Header field is at offset 6
		if packet[6] == 17 { // 17 = UDP
			return false
		} else {
			return true
		}
	}

	return true
}

func IsValidLan(packet []byte) bool {
	if len(packet) < 1 {
		return false
	}
	version := packet[0] >> 4

	if version == 4 {
		if len(packet) < 20 {
			return false
		}

		// IPv4 Destination IP is at offset 16
		dstIP := packet[16:20]

		is10 := dstIP[0] == 10
		is172 := dstIP[0] == 172 && dstIP[1] >= 16 && dstIP[1] <= 31
		is192 := dstIP[0] == 192 && dstIP[1] == 168

		isBroadcast := dstIP[0] == 255 && dstIP[1] == 255 && dstIP[2] == 255 && dstIP[3] == 255

		return is10 || is172 || is192 || isBroadcast
	}

	// Drop everything else (IPv6, etc.) to keep the tunnel quiet
	return false
}

func IsAllowedPort(packet []byte, allowedPorts *sync.Map) bool {
	if len(packet) < 1 {
		return false
	}

	version := packet[0] >> 4

	if version != 4 {
		return false
	}

	if len(packet) < 20 {
		return false
	}

	// Calculate the actual length of the IPv4 header
	ipHeaderLen := int(packet[0]&0x0F) * 4

	// Protocol is at offset 9
	protocolInfo := packet[9]
	if protocolInfo == 1 { // 1 = ICMP
		return true
	}
	if protocolInfo != 6 && protocolInfo != 17 { // 6 = TCP; 17 = UDP
		return false
	}

	// Ensure packet is large enough to contain the Layer 4 ports
	if len(packet) < ipHeaderLen+4 {
		return false
	}

	// Ports are at the very beginning of the Layer 4 header
	sourcePort := binary.BigEndian.Uint16(packet[ipHeaderLen : ipHeaderLen+2])
	destPort := binary.BigEndian.Uint16(packet[ipHeaderLen+2 : ipHeaderLen+4])

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
