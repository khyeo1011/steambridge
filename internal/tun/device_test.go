package tun

import (
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if runtime.GOOS == "windows" {
		exePath, _ := os.Executable()
		exeDir := filepath.Dir(exePath)

		_, filename, _, _ := runtime.Caller(0)
		rootDir := filepath.Join(filepath.Dir(filename), "..", "..")
		sourceDLL := filepath.Join(rootDir, "wintun.dll")

		destDLL := filepath.Join(exeDir, "wintun.dll")
		if sourceFile, err := os.Open(sourceDLL); err == nil {
			if destFile, err := os.Create(destDLL); err == nil {
				io.Copy(destFile, sourceFile)
				destFile.Close()
			}
			sourceFile.Close()
		}
	}

	os.Exit(m.Run())
}
func setupTestDevice(t *testing.T, ifaceName string) (TunInterface, func()) {
	t.Helper()

	if os.Getenv("SKIP_HW_TESTS") != "" {
		t.Skip("Skipping hardware tests")
	}

	dir, err := os.Getwd()
	t.Logf("CWD : %s", dir)

	ifaceID := "sb_" + ifaceName

	t.Logf("Attempting to create TUN device: %s", ifaceName)
	dev, err := NewTUN(ifaceName, ifaceID)
	if err != nil {
		t.Fatalf("Failed to create device: %v", err)
	}

	teardown := func() {
		if err := dev.Close(); err != nil {
			t.Errorf("Failed to close device %s gracefully: %v", ifaceName, err)
		}
	}

	return dev, teardown
}

func TestDevice_Name(t *testing.T) {
	ifaceName := "tun_test_name"
	dev, teardown := setupTestDevice(t, ifaceName)
	defer teardown()

	if dev.Name() != ifaceName {
		t.Errorf("Expected interface name %s, got %s", ifaceName, dev.Name())
	}

	// Verify the OS recognizes the interface
	osIface, err := net.InterfaceByName(dev.Name())
	if err != nil {
		t.Fatalf("OS could not find interface %s: %v", dev.Name(), err)
	}
	t.Logf("OS successfully registered interface: %s", osIface.Name)
}

func TestDevice_SetIP(t *testing.T) {
	ifaceName := "tun_test_ip"
	dev, teardown := setupTestDevice(t, ifaceName)
	defer teardown()

	// Using IP 10.0.0.1 (0x0A000001 in uint32)
	ipInt := uint32(0x0A000001)
	expectedIPStr := "10.0.0.1"

	err := dev.SetIP(ipInt)
	if err != nil {
		t.Fatalf("Failed to set IP: %v", err)
	}

	// Give the OS a moment to apply the IP configuration
	time.Sleep(1 * time.Second)

	osIface, err := net.InterfaceByName(dev.Name())
	if err != nil {
		t.Fatalf("Failed to re-fetch interface after setting IP: %v", err)
	}

	addrs, err := osIface.Addrs()
	if err != nil {
		t.Fatalf("Failed to get addresses for interface %s: %v", dev.Name(), err)
	}

	ipFound := false
	for _, addr := range addrs {
		ip, _, err := net.ParseCIDR(addr.String())
		if err == nil && ip.String() == expectedIPStr {
			ipFound = true
			t.Logf("Successfully verified IP %s is assigned", expectedIPStr)
			break
		}
	}

	if !ipFound {
		t.Errorf("Expected IP %s on interface %s, found: %v", expectedIPStr, dev.Name(), addrs)
	}
}

func TestDevice_Write(t *testing.T) {
	ifaceName := "tun_test_wr"
	dev, teardown := setupTestDevice(t, ifaceName)
	defer teardown()

	// Create a dummy IPv4 packet (20 bytes header minimum)
	dummyPacket := []byte{
		0x45, 0x00, 0x00, 0x14, // Version, IHL, TOS, Total Length
		0x00, 0x00, 0x40, 0x00, // Identification, Flags, Fragment Offset
		0x40, 0x01, 0x00, 0x00, // TTL, Protocol (ICMP), Header Checksum
		0x0a, 0x00, 0x00, 0x01, // Source IP (10.0.0.1)
		0x0a, 0x00, 0x00, 0x02, // Destination IP (10.0.0.2)
	}

	n, err := dev.Write(dummyPacket)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(dummyPacket) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(dummyPacket), n)
	}
}

func TestDevice_Read(t *testing.T) {
	ifaceName := "tun_test_rd"
	dev, teardown := setupTestDevice(t, ifaceName)
	defer teardown()

	readResult := make(chan error)
	go func() {
		buf := make([]byte, MAXMTU)
		_, err := dev.Read(buf)
		readResult <- err
	}()

	select {
	case err := <-readResult:
		if err != nil {
			t.Fatalf("Read returned with error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Log("Read successfully blocked (timeout reached without error)")
	}
}
