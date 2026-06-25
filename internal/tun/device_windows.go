//go:build windows

package tun

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"steambridge/internal/utils"

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
	adapter *wintun.Adapter
	session wintun.Session
	name    string
}

func NewTUN(ifaceName string, ifaceID string) (TunInterface, error) {
	wintunGUID, _ := windows.GenerateGUID()
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

	dev := &Device{
		adapter: adapter,
		session: session,
		name:    ifaceName,
	}

	return dev, nil
}

func (d *Device) Read(p []byte) (int, error) {
	for {
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
			windows.WaitForSingleObject(d.session.ReadWaitEvent(), windows.INFINITE)
			continue
		default:
			return 0, fmt.Errorf("wintun read error: %w", err)
		}
	}
}

func (d *Device) Write(p []byte) (int, error) {
	packet, err := d.session.AllocateSendPacket(len(p))
	if err != nil {
		return 0, fmt.Errorf("failed to allocate send packet: %w", err)
	}

	copy(packet, p)
	d.session.SendPacket(packet)
	return len(p), nil
}

func (d *Device) Close() error {
	d.session.End()
	if d.adapter != nil {
		return d.adapter.Close()
	}
	wintun.Uninstall()
	return nil
}

func (d *Device) Name() string {
	return d.name
}

func (d *Device) SetIP(ip uint32) error {
	cmd := exec.Command("netsh", "interface", "ip", "set", "address",
		fmt.Sprintf("name=%s", d.Name()), "static", utils.IntIPtoString(ip), "255.255.255.0")
	return cmd.Run()
}
