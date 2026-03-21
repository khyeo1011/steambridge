package tun

import (
	"os"
	"testing"
	"time"
)

func TestNewDevice(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping TAP hardware creation test in CI environment")
	}
	ifaceName := "steambridge0"
	ifaceID := "sb_test0" // Use a temporary name for testing

	t.Logf("Attempting to create TAP device: %s", ifaceID)
	dev, err := NewDevice(ifaceName, ifaceID)
	if err != nil {
		t.Fatalf("Failed to create device: %v", err)
	}

	t.Log("Successfully created TAP device! Keeping it open for 5 seconds...")

	// Keep it open briefly so you can verify it exists in your OS
	time.Sleep(5 * time.Second)

	if err := dev.Close(); err != nil {
		t.Errorf("Failed to close device gracefully: %v", err)
	}
	t.Log("Test complete and device closed.")
}
