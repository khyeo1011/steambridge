//go:build linux

package tun

import (
	"fmt"
	"log"
	"os/exec"
	"steambridge/internal/utils"
	"sync"

	"github.com/songgao/water"
	"github.com/vishvananda/netlink"
)

type Device struct {
	*water.Interface
	closeOnce sync.Once
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
	// Clean up a leftover interface from a prior crash so water.New doesn't fail.
	if ifaceName != "" {
		if link, err := netlink.LinkByName(ifaceName); err == nil {
			if delErr := netlink.LinkDel(link); delErr != nil {
				log.Printf("tun: pre-create cleanup of %s failed: %v", ifaceName, delErr)
			}
		}
	}

	config := getWaterConfig(ifaceName)
	iface, err := water.New(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create TAP interface: %w", err)
	}
	dev := &Device{Interface: iface}
	if err := setupLink(dev); err != nil {
		dev.Interface.Close() //nolint:errcheck
		return nil, fmt.Errorf("failed to configure link: %w", err)
	}
	return dev, nil
}

func (d *Device) Read(p []byte) (int, error) {
	return d.Interface.Read(p)
}

func (d *Device) Write(p []byte) (int, error) {
	return d.Interface.Write(p)
}

// closeFD closes the underlying file descriptor exactly once.
// Closing the fd causes any blocked Read to return an error immediately.
func (d *Device) closeFD() {
	d.closeOnce.Do(func() {
		d.Interface.Close() //nolint:errcheck
	})
}

// Unblock wakes any goroutine blocked in Read without removing the interface.
// Call Close() after all goroutines have exited to also delete the link.
func (d *Device) Unblock() error {
	d.closeFD()
	return nil
}

// Close closes the fd and removes the network link so a subsequent NewTUN
// for the same interface name succeeds.
func (d *Device) Close() error {
	d.closeFD()
	link, err := netlink.LinkByName(d.Name())
	if err != nil {
		// Interface may already be gone; log and move on.
		log.Printf("tun: LinkByName(%s) on close: %v", d.Name(), err)
		return nil
	}
	if err := netlink.LinkDel(link); err != nil {
		log.Printf("tun: LinkDel(%s): %v", d.Name(), err)
	}
	return nil
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
