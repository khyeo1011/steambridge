//go:build windows

package steam

import (
	"fmt"
	"log"
	"os/exec"
	"steambridge/internal/tun"
	"steambridge/internal/utils"
)

func setTAPIP(ip uint32, device *tun.Device) error {
	ipCmd := exec.Command("netsh", "interface", "ipv4", "set", "address", fmt.Sprintf("name=%s", device.Name()), "static", fmt.Sprintf("address=%s", utils.IntIPtoString(ip)), "255.255.255.0")
	if err := ipCmd.Run(); err != nil {
		log.Printf("Failed to set ip address to %s", utils.IntIPtoString(ip))
		return err
	}
	return nil
}
