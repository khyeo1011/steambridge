//go:build linux

package steam

import (
	"fmt"
	"log"
	"os/exec"
	"steambridge/internal/tap"
)

func setTAPIP(ip uint32, device *tap.Device) error {
	ipString := fmt.Sprintf("%d.%d.%d.%d", ip>>24, (ip>>16)&0xFF, (ip>>8)&0xFF, ip&0xFF)
	ipCmd := exec.Command("sudo", "ip", "addr", "add", fmt.Sprintf("%s/24", ipString), "dev", device.Name())
	if err := ipCmd.Run(); err != nil {
		log.Printf("Failed to set ip address to %s", ipString)
		return err
	}
	upCmd := exec.Command("sudo", "ip", "link", "set", "dev", device.Name(), "up")
	if err := upCmd.Run(); err != nil {
		log.Printf("Failed to set interface %s up", device.Name())
		return err
	}
	return nil
}
