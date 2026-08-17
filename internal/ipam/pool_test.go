package ipam

import (
	"testing"
)

// 10.8.0.x helper: mirrors NewPool baseIP logic
func ip(host uint32) uint32 {
	return (10 << 24) | (8 << 16) | host
}

func TestAllocate_Idempotent(t *testing.T) {
	p := NewPool()
	a1 := p.Allocate(100)
	a2 := p.Allocate(100)
	if a1 != a2 {
		t.Fatalf("same SteamID got different IPs: %08x vs %08x", a1, a2)
	}
}

func TestAllocate_Sequential(t *testing.T) {
	p := NewPool()
	got := make([]uint32, 3)
	for i := 0; i < 3; i++ {
		got[i] = p.Allocate(uint64(i + 1))
	}
	for i, g := range got {
		want := ip(uint32(2 + i)) // 10.8.0.2, .3, .4
		if g != want {
			t.Errorf("SteamID %d: got %08x, want %08x", i+1, g, want)
		}
	}
}

func TestReleaseBySteamID_RemovesLease(t *testing.T) {
	p := NewPool()
	ipA := p.Allocate(42)
	released, ok := p.ReleaseBySteamID(42)
	if !ok || released != ipA {
		t.Fatalf("ReleaseBySteamID(42): got (%08x, %v), want (%08x, true)", released, ok, ipA)
	}
	// Drain the free-list so the recycled IP is claimed by a different SteamID.
	// If the lease for 42 were still present, Allocate(42) would return ipA
	// via the idempotent path, but since it's now held by 99, we'd get a collision.
	_ = p.Allocate(99)
	second := p.Allocate(42)
	if second == ipA {
		t.Errorf("SteamID 42 still has lease for %08x after ReleaseBySteamID", ipA)
	}
}

func TestReleaseBySteamID_NoLease(t *testing.T) {
	p := NewPool()
	if released, ok := p.ReleaseBySteamID(1234); ok || released != 0 {
		t.Errorf("ReleaseBySteamID with no lease: got (%08x, %v), want (0, false)", released, ok)
	}
}

// TestAllocate_Recycle verifies that a released IP is reused by the next
// Allocate call, rather than consuming a new host number. This is the
// behaviour that requires the free-list fix in pool.go.
func TestAllocate_Recycle(t *testing.T) {
	p := NewPool()
	ipA := p.Allocate(1)  // 10.8.0.2
	_ = p.Allocate(2)     // 10.8.0.3
	p.ReleaseBySteamID(1) // free 10.8.0.2

	// A third distinct SteamID should reuse the freed IP (10.8.0.2),
	// not allocate a new one (10.8.0.4).
	ipC := p.Allocate(3)
	if ipC != ipA {
		t.Errorf("expected freed IP %08x to be recycled, got %08x instead", ipA, ipC)
	}
}

// TestAllocate_Exhaustion verifies the pool stays inside the /24: the last
// usable host is 10.8.0.254, the next Allocate fails with 0 (never handing
// out the .255 broadcast or walking into 10.8.1.x), and releasing a lease
// makes allocation possible again.
func TestAllocate_Exhaustion(t *testing.T) {
	p := NewPool()
	var last uint32
	// Hosts 2..254 inclusive = 253 usable leases.
	for i := 0; i < 253; i++ {
		last = p.Allocate(uint64(i + 1))
		if last == 0 {
			t.Fatalf("Allocate failed early at lease #%d", i+1)
		}
	}
	if last != ip(254) {
		t.Errorf("last lease: got %08x, want %08x (10.8.0.254)", last, ip(254))
	}
	if got := p.Allocate(9999); got != 0 {
		t.Errorf("Allocate on exhausted pool: got %08x, want 0", got)
	}
	// Releasing a lease frees capacity again.
	freed, ok := p.ReleaseBySteamID(1)
	if !ok {
		t.Fatal("ReleaseBySteamID(1) failed on a held lease")
	}
	if got := p.Allocate(9999); got != freed {
		t.Errorf("Allocate after release: got %08x, want recycled %08x", got, freed)
	}
}
