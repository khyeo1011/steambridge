//go:build windows

package tun

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"steambridge/internal/utils"
	"sync/atomic"

	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wintun"
)

func init() {
	// wintun loads its DLL with LoadLibraryEx(APPLICATION_DIR|SYSTEM32), which
	// only looks in the exe directory and System32. In wails dev mode the exe is
	// in a temp dir, so wintun.dll can't be found. SetDllDirectory redirects
	// LOAD_LIBRARY_SEARCH_APPLICATION_DIR to the given path, fixing the search.
	for _, dir := range []string{exeDir(), cwdDir()} {
		if dir == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, "wintun.dll")); err == nil {
			windows.SetDllDirectory(dir) //nolint:errcheck
			return
		}
	}
}

func exeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Dir(exe)
}

func cwdDir() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return wd
}

type Device struct {
	adapter    *wintun.Adapter
	session    wintun.Session
	name       string
	closeEvent windows.Handle
	closed     atomic.Bool
}

// deterministicGUID derives a stable windows.GUID from seed so the same
// logical interface keeps the same adapter identity across runs, preventing
// Windows from accumulating "Network 2, 3, 4..." profiles.
func deterministicGUID(seed string) windows.GUID {
	sum := sha256.Sum256([]byte(seed))
	guid := windows.GUID{
		Data1: binary.LittleEndian.Uint32(sum[0:4]),
		Data2: binary.LittleEndian.Uint16(sum[4:6]),
		Data3: binary.LittleEndian.Uint16(sum[6:8]),
	}
	copy(guid.Data4[:], sum[8:16])
	return guid
}

// adapterGUIDSeed picks the seed for the adapter GUID: the stable ifaceID
// when available, falling back to the interface name otherwise.
func adapterGUIDSeed(ifaceName, ifaceID string) string {
	if ifaceID != "" {
		return ifaceID
	}
	return ifaceName
}

func NewTUN(ifaceName string, ifaceID string) (TunInterface, error) {
	wintunGUID := deterministicGUID(adapterGUIDSeed(ifaceName, ifaceID))
	adapter, err := wintun.CreateAdapter(ifaceName, "SteamBridge", &wintunGUID)
	if err != nil {
		// Fallback to Open if it already exists
		adapter, err = wintun.OpenAdapter(ifaceName)
		if err != nil {
			return nil, fmt.Errorf("failed to create/open wintun adapter: %w", err)
		}
	}

	session, err := adapter.StartSession(0x400000)
	if err != nil {
		adapter.Close()
		return nil, fmt.Errorf("failed to start wintun session: %w", err)
	}

	// Manual-reset event used by Unblock() to wake a blocked Read.
	closeEvent, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		session.End()
		adapter.Close()
		return nil, fmt.Errorf("failed to create close event: %w", err)
	}

	dev := &Device{
		adapter:    adapter,
		session:    session,
		name:       ifaceName,
		closeEvent: closeEvent,
	}

	return dev, nil
}

func (d *Device) Read(p []byte) (int, error) {
	for {
		if d.closed.Load() {
			return 0, io.EOF
		}

		packet, err := d.session.ReceivePacket()
		if err == nil {
			n := copy(p, packet)
			d.session.ReleaseReceivePacket(packet)
			return n, nil
		}

		switch err {
		case windows.ERROR_HANDLE_EOF:
			return 0, io.EOF
		case windows.ERROR_NO_MORE_ITEMS:
			// Wait for either new data or an Unblock() signal.
			windows.WaitForMultipleObjects(
				[]windows.Handle{d.closeEvent, d.session.ReadWaitEvent()},
				false,
				windows.INFINITE,
			) //nolint:errcheck
			continue
		default:
			return 0, fmt.Errorf("wintun read error: %w", err)
		}
	}
}

func (d *Device) Write(p []byte) (int, error) {
	if d.closed.Load() {
		return 0, io.EOF
	}
	packet, err := d.session.AllocateSendPacket(len(p))
	if err != nil {
		return 0, fmt.Errorf("failed to allocate send packet: %w", err)
	}

	copy(packet, p)
	d.session.SendPacket(packet)
	return len(p), nil
}

// Unblock wakes any goroutine blocked in Read without freeing device resources.
// Call Close() after all goroutines have exited.
func (d *Device) Unblock() error {
	d.closed.Store(true)
	return windows.SetEvent(d.closeEvent)
}

// Close frees the wintun session and adapter. Must only be called after all
// goroutines using Read/Write have returned (i.e. after wg.Wait()).
func (d *Device) Close() error {
	d.session.End()
	if d.closeEvent != 0 {
		windows.CloseHandle(d.closeEvent) //nolint:errcheck
		d.closeEvent = 0
	}
	if d.adapter != nil {
		return d.adapter.Close()
	}
	return nil
}

func (d *Device) Name() string {
	return d.name
}

func (d *Device) SetIP(ip uint32) error {
	cmd := exec.Command("netsh", "interface", "ip", "set", "address",
		fmt.Sprintf("name=%s", d.Name()), "static", utils.IntIPtoString(ip), "255.255.255.0")
	if err := cmd.Run(); err != nil {
		return err
	}

	// Cap the adapter MTU so packets fit within Steam's unreliable P2P limit,
	// matching the Linux setup (see setupLink in device_linux.go).
	cmd = exec.Command("netsh", "interface", "ipv4", "set", "subinterface",
		d.Name(), fmt.Sprintf("mtu=%d", MAXMTU), "store=active")
	return cmd.Run()
}
