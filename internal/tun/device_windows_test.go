//go:build windows

package tun

import (
	"testing"

	"golang.org/x/sys/windows"
)

func TestDeterministicGUIDStable(t *testing.T) {
	seeds := []string{"iface-id-1", "steambridge", ""}
	for _, seed := range seeds {
		a := deterministicGUID(seed)
		b := deterministicGUID(seed)
		if a != b {
			t.Errorf("deterministicGUID(%q) not stable across calls: %v != %v", seed, a, b)
		}
	}
}

// TestDeterministicGUIDKnownAnswer pins the derivation so it stays stable
// across builds and releases — a changed GUID would make Windows treat the
// adapter as a brand-new network (the exact bug this derivation prevents).
func TestDeterministicGUIDKnownAnswer(t *testing.T) {
	want := windows.GUID{
		Data1: 0xebb1bb56,
		Data2: 0xcf70,
		Data3: 0x63f5,
		Data4: [8]byte{0xc0, 0xd2, 0xa4, 0xf6, 0x41, 0xb6, 0x0b, 0x25},
	}
	got := deterministicGUID("steambridge-iface-id")
	if got != want {
		t.Errorf("deterministicGUID(\"steambridge-iface-id\") = %v, want %v", got, want)
	}
}

func TestDeterministicGUIDDistinctSeeds(t *testing.T) {
	seeds := []string{"", "a", "b", "iface-id-1", "iface-id-2", "steambridge"}
	seen := make(map[windows.GUID]string, len(seeds))
	for _, seed := range seeds {
		guid := deterministicGUID(seed)
		if prev, dup := seen[guid]; dup {
			t.Errorf("seeds %q and %q produced the same GUID %v", prev, seed, guid)
		}
		seen[guid] = seed
	}
}

func TestDeterministicGUIDFieldsFilled(t *testing.T) {
	// A zero field is not wrong per se, but for these seeds every field of
	// the SHA-256-derived GUID should be populated; an all-zero field would
	// suggest the derivation stopped reading the digest partway.
	guid := deterministicGUID("steambridge-iface-id")
	if guid.Data1 == 0 || guid.Data2 == 0 || guid.Data3 == 0 || guid.Data4 == [8]byte{} {
		t.Errorf("unexpected zero field in derived GUID: %+v", guid)
	}
}

func TestAdapterGUIDSeedFallback(t *testing.T) {
	if got := adapterGUIDSeed("steambridge", "iface-id-1"); got != "iface-id-1" {
		t.Errorf("adapterGUIDSeed with ifaceID = %q, want %q", got, "iface-id-1")
	}
	if got := adapterGUIDSeed("steambridge", ""); got != "steambridge" {
		t.Errorf("adapterGUIDSeed fallback = %q, want %q", got, "steambridge")
	}
}
