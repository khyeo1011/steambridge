//go:build windows

package steam

import (
	"fmt"
	"log"
	"os/exec"
	"steambridge/internal/tap"
)

func setTAPIP(ip uint32, device *tap.Device) error {
	ipString := fmt.Sprintf("%d.%d.%d.%d", ip>>24, (ip>>16)&0xFF, (ip>>8)&0xFF, ip&0xFF)
	ipCmd := exec.Command("netsh", "interface", "ipv4", "set", "address", fmt.Sprintf("name=%s", device.Name()), "static", fmt.Sprintf("address=%s", ipString), "255.255.255.0")
	if err := ipCmd.Run(); err != nil {
		log.Printf("Failed to set ip address to %s", ipString)
		return err
	}
	return nil
}
