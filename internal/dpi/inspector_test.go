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
			// 14 byte Eth Header (0x0800 at offset 12) + IP Header (Protocol 17 at offset 23)
			frame: []byte{
				0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, // 12 bytes MACs
				0x08, 0x00, // EtherType IPv4
				0, 0, 0, 0, 0, 0, 0, 0, 0, // 9 bytes IP header start
				17, // Protocol UDP (Offset 23)
			},
			expected: false,
		},
		{
			name: "IPv4 TCP",
			frame: []byte{
				0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
				0x08, 0x00,
				0, 0, 0, 0, 0, 0, 0, 0, 0,
				6, // Protocol TCP
			},
			expected: true,
		},
		{
			name: "IPv6 UDP",
			// 14 byte Eth Header (0x86DD at offset 12) + IP Header (Next Header 17 at offset 20)
			frame: []byte{
				0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
				0x86, 0xDD, // EtherType IPv6
				0, 0, 0, 0, 0, 0, // 6 bytes IP header start
				17, // Next Header UDP (Offset 20)
				0,  // Padding for length check
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
