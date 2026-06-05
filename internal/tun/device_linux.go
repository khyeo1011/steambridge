//go:build linux

package tun

import (
	"fmt"
	"log"
	"os/exec"
	"steambridge/internal/utils"

	"github.com/songgao/water"
	"github.com/vishvananda/netlink"
)

type Device struct {
	*water.Interface
}

func getWaterConfig(ifaceName string) water.Config {
	if ifaceName == "" {
		return water.Config{
			DeviceType: water.TUN,
		}
	}
	return water.Config{
		DeviceType: water.TUN,
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

func NewTUN(ifaceName string, ifaceID string) (TunInterface, error) {
	config := getWaterConfig(ifaceName)
	iface, err := water.New(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create TAP interface: %w", err)
	}
	if err := setupLink(&Device{iface}); err != nil {
		return nil, fmt.Errorf("failed to configure link: %w", err)
	}
	return &Device{iface}, nil
}

func (d *Device) Read(p []byte) (int, error) {
	return d.Interface.Read(p)
}

func (d *Device) Write(p []byte) (int, error) {
	return d.Interface.Write(p)
}

func (d *Device) Close() error {
	return d.Interface.Close()
}

func (d *Device) SetIP(ip uint32) error {
	ipCmd := exec.Command("sudo", "ip", "addr", "add", fmt.Sprintf("%s/24", utils.IntIPtoString(ip)), "dev", d.Name())
	if err := ipCmd.Run(); err != nil {
		log.Printf("Failed to set ip address to %s", utils.IntIPtoString(ip))
		return err
	}
	upCmd := exec.Command("sudo", "ip", "link", "set", "dev", d.Name(), "up")
	if err := upCmd.Run(); err != nil {
		log.Printf("Failed to set interface %s up", d.Name())
		return err
	}
	return nil
}

func (d *Device) Name() string {
	return d.Interface.Name()
}
