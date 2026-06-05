package dpi

import (
	"testing"
)

func TestIsReliable(t *testing.T) {
	tests := []struct {
		name     string
		frame    []byte
		expected bool // true = Reliable (TCP/ICMP/Unknown), false = Unreliable (UDP)
	}{
		{
			name:     "Too Short Frame",
			frame:    []byte{0xFF, 0xFF, 0xFF, 0xFF}, // Only 4 bytes
			expected: true,                           // Safe fallback
		},
		{
			name: "IPv4 UDP",
			// Raw IPv4 packet (TUN device — no Ethernet header). Protocol at offset 9.
			frame: []byte{
				0x45,       // version=4, IHL=5
				0x00,       // DSCP+ECN
				0x00, 0x1c, // total length
				0x00, 0x00, // identification
				0x00, 0x00, // flags + fragment offset
				0x40,       // TTL
				17,         // protocol = UDP
			},
			expected: false,
		},
		{
			name: "IPv4 TCP",
			// Raw IPv4 packet. Protocol at offset 9.
			frame: []byte{
				0x45,
				0x00,
				0x00, 0x28,
				0x00, 0x00,
				0x00, 0x00,
				0x40,
				6, // protocol = TCP
			},
			expected: true,
		},
		{
			name: "IPv6 UDP",
			// Raw IPv6 packet (TUN device — no Ethernet header). Next Header at offset 6.
			frame: []byte{
				0x60,       // version=6, TC=0, flow=0
				0x00, 0x00, 0x00, // TC + flow label
				0x00, 0x08, // payload length
				17,         // next header = UDP
			},
			expected: false,
		},
		{
			name: "ARP Broadcast",
			frame: []byte{
				0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0, 0, 0, 0, 0, 0,
				0x08, 0x06, // EtherType ARP
				0, 0, 0, 0, 0, 0,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsReliable(tt.frame)
			if result != tt.expected {
				t.Errorf("IsReliable() = %v, expected %v", result, tt.expected)
			}
		})
	}
}
