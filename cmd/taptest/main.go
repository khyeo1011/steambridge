package main

import (
	"fmt"
	"log"

	"steambridge/internal/tap"
)

func main() {
	iface := "steambridge0"
	fmt.Printf("Attempting to open TAP device: %s...\n", iface)

	dev, err := tap.NewDevice(iface)
	if err != nil {
		log.Fatalf("❌ Failed to open TAP: %v", err)
	}
	defer dev.Close()

	fmt.Printf("✅ Successfully bound on %s! Ignoring OS noise and waiting for 3 ICMP Pings...\n", dev.Name())

	buf := make([]byte, 2048)
	packetsRead := 0

	for packetsRead < 3 {
		n, err := dev.Read(buf)
		if err != nil {
			log.Fatalf("❌ Error reading from TAP: %v", err)
		}

		if n < 14 {
			log.Println("Received packet < 14 bytes")
			continue
		}

		frame := buf[:n]
		ethType := uint16(frame[12])<<8 | uint16(frame[13])

		if ethType != 0x0800 || len(frame) < 34 {
			continue // Drop ARP, IPv6, etc.
		}

		srcIP := fmt.Sprintf("%d.%d.%d.%d", frame[26], frame[27], frame[28], frame[29])
		dstIP := fmt.Sprintf("%d.%d.%d.%d", frame[30], frame[31], frame[32], frame[33])

		if frame[23] != 1 {
			continue
		}

		destMAC := fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", frame[0], frame[1], frame[2], frame[3], frame[4], frame[5])
		srcMAC := fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", frame[6], frame[7], frame[8], frame[9], frame[10], frame[11])

		packetsRead++
		fmt.Printf("🏓 PING DETECTED %d/3: %4d bytes | %s (%s) -> %s (%s)\n", packetsRead, n, srcIP, srcMAC, dstIP, destMAC)
	}

	fmt.Println("🎉 Signal isolated! TAP filtering is working.")
}
