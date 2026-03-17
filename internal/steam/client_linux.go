//go:build linux

package steam

import (
	"fmt"
	"log"
	"os/exec"
	"steambridge/internal/ipam"
	"steambridge/internal/tap"
)

func setTAPIP(ip uint32, device *tap.Device) error {
	ipCmd := exec.Command("sudo", "ip", "addr", "add", fmt.Sprintf("%s/24", ipam.IntIPtoString(ip)), "dev", device.Name())
	if err := ipCmd.Run(); err != nil {
		log.Printf("Failed to set ip address to %s", ipam.IntIPtoString(ip))
		return err
	}
	upCmd := exec.Command("sudo", "ip", "link", "set", "dev", device.Name(), "up")
	if err := upCmd.Run(); err != nil {
		log.Printf("Failed to set interface %s up", device.Name())
		return err
	}
	return nil
}
