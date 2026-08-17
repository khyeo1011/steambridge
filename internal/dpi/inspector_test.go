package dpi

import (
	"sync"
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
				0x40, // TTL
				17,   // protocol = UDP
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
				0x60,             // version=6, TC=0, flow=0
				0x00, 0x00, 0x00, // TC + flow label
				0x00, 0x08, // payload length
				17, // next header = UDP
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

func TestIsValidLan(t *testing.T) {
	tests := []struct {
		name     string
		packet   []byte
		expected bool
	}{
		{name: "Empty", packet: []byte{}, expected: false},
		{name: "TooShort IPv4", packet: append([]byte{0x45}, make([]byte, 18)...), expected: false},
		{
			name: "10.x.x.x accept",
			packet: func() []byte {
				p := make([]byte, 20)
				p[0] = 0x45
				p[16], p[17], p[18], p[19] = 10, 5, 0, 1
				return p
			}(),
			expected: true,
		},
		{
			name: "172.16.x.x accept",
			packet: func() []byte {
				p := make([]byte, 20)
				p[0] = 0x45
				p[16], p[17], p[18], p[19] = 172, 16, 0, 1
				return p
			}(),
			expected: true,
		},
		{
			name: "172.31.x.x accept",
			packet: func() []byte {
				p := make([]byte, 20)
				p[0] = 0x45
				p[16], p[17], p[18], p[19] = 172, 31, 0, 1
				return p
			}(),
			expected: true,
		},
		{
			name: "172.15.x.x reject (not in 172.16-31 range)",
			packet: func() []byte {
				p := make([]byte, 20)
				p[0] = 0x45
				p[16], p[17], p[18], p[19] = 172, 15, 0, 1
				return p
			}(),
			expected: false,
		},
		{
			name: "192.168.x.x accept",
			packet: func() []byte {
				p := make([]byte, 20)
				p[0] = 0x45
				p[16], p[17], p[18], p[19] = 192, 168, 1, 1
				return p
			}(),
			expected: true,
		},
		{
			name: "255.255.255.255 broadcast accept",
			packet: func() []byte {
				p := make([]byte, 20)
				p[0] = 0x45
				p[16], p[17], p[18], p[19] = 255, 255, 255, 255
				return p
			}(),
			expected: true,
		},
		{
			name: "8.8.8.8 public reject",
			packet: func() []byte {
				p := make([]byte, 20)
				p[0] = 0x45
				p[16], p[17], p[18], p[19] = 8, 8, 8, 8
				return p
			}(),
			expected: false,
		},
		{
			name:     "IPv6 reject",
			packet:   []byte{0x60, 0, 0, 0, 0, 0, 17, 64},
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidLan(tt.packet); got != tt.expected {
				t.Errorf("IsValidLan() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func makeAllowedPorts(ports ...uint16) *sync.Map {
	m := &sync.Map{}
	for _, p := range ports {
		m.Store(p, true)
	}
	return m
}

func makeIPv4Packet(proto byte, ihl byte, srcPort, dstPort uint16) []byte {
	headerLen := int(ihl) * 4
	p := make([]byte, headerLen+4)
	p[0] = (4 << 4) | ihl
	p[9] = proto
	p[headerLen] = byte(srcPort >> 8)
	p[headerLen+1] = byte(srcPort)
	p[headerLen+2] = byte(dstPort >> 8)
	p[headerLen+3] = byte(dstPort)
	return p
}

func TestIsAllowedPort(t *testing.T) {
	tests := []struct {
		name     string
		packet   []byte
		ports    *sync.Map
		expected bool
	}{
		{name: "Empty", packet: []byte{}, ports: makeAllowedPorts(), expected: false},
		{name: "TooShort", packet: []byte{0x45, 0, 0, 0, 0, 0, 0, 0, 0, 6}, ports: makeAllowedPorts(80), expected: false},
		{name: "IPv6 reject", packet: []byte{0x60, 0, 0, 0, 0, 0, 17, 0, 0, 0, 0, 0}, ports: makeAllowedPorts(80), expected: false},
		{
			name:     "ICMP always allowed",
			packet:   func() []byte { p := make([]byte, 24); p[0] = 0x45; p[9] = 1; return p }(),
			ports:    makeAllowedPorts(),
			expected: true,
		},
		{
			name:     "Unknown protocol reject",
			packet:   func() []byte { p := make([]byte, 24); p[0] = 0x45; p[9] = 50; return p }(),
			ports:    makeAllowedPorts(80),
			expected: false,
		},
		{
			name:     "TCP dest port allowed",
			packet:   makeIPv4Packet(6, 5, 12345, 80),
			ports:    makeAllowedPorts(80),
			expected: true,
		},
		{
			name:     "TCP src port allowed",
			packet:   makeIPv4Packet(6, 5, 80, 12345),
			ports:    makeAllowedPorts(80),
			expected: true,
		},
		{
			name:     "UDP port not in list reject",
			packet:   makeIPv4Packet(17, 5, 12345, 9999),
			ports:    makeAllowedPorts(80, 443),
			expected: false,
		},
		{
			name:     "UDP dest port allowed",
			packet:   makeIPv4Packet(17, 5, 12345, 443),
			ports:    makeAllowedPorts(443),
			expected: true,
		},
		{
			// IHL=15 means header is 60 bytes; packet only has 20 bytes — must not panic.
			name:     "IHL too large for packet no panic",
			packet:   func() []byte { p := make([]byte, 20); p[0] = (4 << 4) | 15; p[9] = 6; return p }(),
			ports:    makeAllowedPorts(80),
			expected: false,
		},
		{
			// IHL=4 encodes a 16-byte header, below the IPv4 minimum of 20.
			name:     "IHL below minimum reject",
			packet:   func() []byte { p := makeIPv4Packet(6, 5, 80, 80); p[0] = (4 << 4) | 4; return p }(),
			ports:    makeAllowedPorts(80),
			expected: false,
		},
		{
			// Fragment offset != 0: no L4 header present, must reject even if the
			// bytes at the port offsets happen to match an allowed port.
			name: "Non-first fragment reject",
			packet: func() []byte {
				p := makeIPv4Packet(17, 5, 443, 443)
				p[6], p[7] = 0x00, 0x05 // fragment offset = 5
				return p
			}(),
			ports:    makeAllowedPorts(443),
			expected: false,
		},
		{
			// MF flag set but offset 0: first fragment carries the L4 header.
			name: "First fragment with MF flag allowed",
			packet: func() []byte {
				p := makeIPv4Packet(17, 5, 443, 443)
				p[6] = 0x20 // MF flag, offset 0
				return p
			}(),
			ports:    makeAllowedPorts(443),
			expected: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAllowedPort(tt.packet, tt.ports); got != tt.expected {
				t.Errorf("IsAllowedPort() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func FuzzIsValidLan(f *testing.F) {
	f.Add([]byte{0x45, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 10, 0, 0, 1})
	f.Add([]byte{0x60})
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, p []byte) {
		IsValidLan(p) // must not panic
	})
}

func FuzzIsAllowedPort(f *testing.F) {
	f.Add(makeIPv4Packet(6, 5, 80, 443))
	f.Add([]byte{0x4F, 0, 0, 0, 0, 0, 0, 0, 0, 6}) // IHL=15, truncated
	f.Add([]byte{})
	ports := makeAllowedPorts(80, 443)
	f.Fuzz(func(t *testing.T, p []byte) {
		IsAllowedPort(p, ports) // must not panic
	})
}
