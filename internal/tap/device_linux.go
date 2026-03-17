//go:build linux

package tap

import (
	"fmt"

	"github.com/songgao/water"
	"github.com/vishvananda/netlink"
)

func getWaterConfig(ifaceName string) water.Config {
	if ifaceName == "" {
		return water.Config{
			DeviceType: water.TAP,
		}
	}
	return water.Config{
		DeviceType: water.TAP,
		PlatformSpecificParams: water.PlatformSpecificParams{
			Name: ifaceName,
		},
	}
}

func setupLink(dev *Device) error {
	link, err := netlink.LinkByName(dev.Name())
	if err != nil {
		return fmt.Errorf("failed to get TAP link: %w", err)
	}

	if err := netlink.LinkSetMTU(link, MAXMTU); err != nil {
		return fmt.Errorf("failed to set MTU: %w", err)
	}

	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("failed to bring interface up: %w", err)
	}
	return nil
}
