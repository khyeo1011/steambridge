//go:build windows

package tap

import (
	"fmt"
	"os/exec"

	"github.com/songgao/water"
)

func getWaterConfig(ifaceName string, ifaceID string) water.Config {
	if ifaceID == "" {
		return water.Config{
			DeviceType: water.TAP,
		}
	}
	return water.Config{
		DeviceType: water.TAP,
		PlatformSpecificParams: water.PlatformSpecificParams{
			ComponentID: ifaceID,
		},
	}
}

func setupLink(dev *Device) error {
	// Enforce 1280 MTU via netsh
	mtuCmd := exec.Command("netsh", "interface", "ipv4", "set", "subinterface", dev.Name(), fmt.Sprintf("mtu=%d", MAXMTU), "store=persistent")
	if err := mtuCmd.Run(); err != nil {
		return fmt.Errorf("netsh mtu failed: %w", err)
	}

	// Bring interface up via netsh
	upCmd := exec.Command("netsh", "interface", "set", "interface", dev.Name(), "admin=enable")
	if err := upCmd.Run(); err != nil {
		return fmt.Errorf("netsh enable failed: %w", err)
	}

	return nil
}
