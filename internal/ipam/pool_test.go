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

func TestRelease_RemovesLease(t *testing.T) {
	p := NewPool()
	ipA := p.Allocate(42)
	p.Release(ipA)
	// Drain the free-list so the recycled IP is claimed by a different SteamID.
	// If the lease for 42 were still present, Allocate(42) would return ipA
	// via the idempotent path, but since it's now held by 99, we'd get a collision.
	_ = p.Allocate(99)
	second := p.Allocate(42)
	if second == ipA {
		t.Errorf("SteamID 42 still has lease for %08x after Release", ipA)
	}
}

// TestAllocate_Recycle verifies that a released IP is reused by the next
// Allocate call, rather than consuming a new host number. This is the
// behaviour that requires the free-list fix in pool.go.
func TestAllocate_Recycle(t *testing.T) {
	p := NewPool()
	ipA := p.Allocate(1) // 10.8.0.2
	_ = p.Allocate(2)    // 10.8.0.3
	p.Release(ipA)       // free 10.8.0.2

	// A third distinct SteamID should reuse the freed IP (10.8.0.2),
	// not allocate a new one (10.8.0.4).
	ipC := p.Allocate(3)
	if ipC != ipA {
		t.Errorf("expected freed IP %08x to be recycled, got %08x instead", ipA, ipC)
	}
}
