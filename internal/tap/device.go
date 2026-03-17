package tap

import (
	"fmt"
	"log"

	"github.com/songgao/water"
)

const MAXMTU = 1180

type Device struct {
	*water.Interface
}

func NewDevice(ifaceName string, ifaceID string) (*Device, error) {
	// 1. Get the OS-specific config
	config := getWaterConfig(ifaceName)

	// 2. Create the interface (Shared logic)
	iface, err := water.New(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create TAP interface: %w", err)
	}

	// 3. Apply the OS-specific MTU and Link Up commands
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
	log.Println("Closing TAP interface")
	return d.Interface.Close()
}
