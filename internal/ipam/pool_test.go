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

// TestReleaseBySteamID_DoubleReleaseIsNoOp verifies a second release from the
// same SteamID does nothing: the freed IP must land on the free-list exactly
// once, so two subsequent Allocates for new peers get distinct IPs.
func TestReleaseBySteamID_DoubleReleaseIsNoOp(t *testing.T) {
	p := NewPool()
	ipA := p.Allocate(1)
	if got, ok := p.ReleaseBySteamID(1); !ok || got != ipA {
		t.Fatalf("first release: got (%08x, %v), want (%08x, true)", got, ok, ipA)
	}
	if got, ok := p.ReleaseBySteamID(1); ok || got != 0 {
		t.Errorf("second release: got (%08x, %v), want (0, false)", got, ok)
	}
	first := p.Allocate(2)
	second := p.Allocate(3)
	if first != ipA {
		t.Errorf("first Allocate after release: got %08x, want recycled %08x", first, ipA)
	}
	if second == ipA {
		t.Errorf("freed IP %08x handed out twice — free-list holds a duplicate", ipA)
	}
}

// TestAllocate_RecycleFIFO verifies multiple freed IPs are reused in the
// order they were released.
func TestAllocate_RecycleFIFO(t *testing.T) {
	p := NewPool()
	ipA := p.Allocate(1) // 10.8.0.2
	ipB := p.Allocate(2) // 10.8.0.3
	p.ReleaseBySteamID(2)
	p.ReleaseBySteamID(1)

	// Released order was B then A, so reuse must be B first.
	if got := p.Allocate(3); got != ipB {
		t.Errorf("first recycled Allocate: got %08x, want %08x (FIFO)", got, ipB)
	}
	if got := p.Allocate(4); got != ipA {
		t.Errorf("second recycled Allocate: got %08x, want %08x (FIFO)", got, ipA)
	}
}

// TestAllocate_RepeatSteamIDAtCapacity verifies the idempotent path still
// works when the pool is exhausted: an existing leaseholder re-requesting
// gets its own IP back, not 0.
func TestAllocate_RepeatSteamIDAtCapacity(t *testing.T) {
	p := NewPool()
	var first uint32
	for i := 0; i < 253; i++ {
		got := p.Allocate(uint64(i + 1))
		if i == 0 {
			first = got
		}
	}
	if got := p.Allocate(9999); got != 0 {
		t.Fatalf("pool should be exhausted, but Allocate returned %08x", got)
	}
	if got := p.Allocate(1); got != first {
		t.Errorf("repeat Allocate(1) at capacity: got %08x, want existing lease %08x", got, first)
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
