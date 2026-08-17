package router

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"
)

// fakeTun satisfies tun.TunInterface for tests.
// Read returns scripted packets one at a time; Write records payloads.
type fakeTun struct {
	mu       sync.Mutex
	packets  [][]byte // packets to feed on Read
	written  [][]byte // payloads captured from Write
	readIdx  int
	done     chan struct{} // closed to unblock a pending Read after all packets consumed
	doneOnce sync.Once
	readErr  error // if set, returned instead of io.EOF when packets are exhausted
}

func newFakeTun(packets ...[]byte) *fakeTun {
	return &fakeTun{packets: packets, done: make(chan struct{})}
}

func (f *fakeTun) Read(p []byte) (int, error) {
	f.mu.Lock()
	if f.readIdx < len(f.packets) {
		pkt := f.packets[f.readIdx]
		f.readIdx++
		n := copy(p, pkt)
		f.mu.Unlock()
		return n, nil
	}
	readErr := f.readErr
	f.mu.Unlock()
	// signal the test we've fed all packets, then block until closed
	f.doneOnce.Do(func() { close(f.done) })
	<-f.done
	if readErr != nil {
		return 0, readErr
	}
	return 0, io.EOF
}

func (f *fakeTun) Write(p []byte) (int, error) {
	cp := make([]byte, len(p))
	copy(cp, p)
	f.mu.Lock()
	f.written = append(f.written, cp)
	f.mu.Unlock()
	return len(p), nil
}

func (f *fakeTun) Unblock() error       { f.doneOnce.Do(func() { close(f.done) }); return nil }
func (f *fakeTun) Close() error         { return nil }
func (f *fakeTun) SetIP(_ uint32) error { return nil }
func (f *fakeTun) Name() string         { return "faketun0" }

// fakeSteam satisfies SteamSender for tests.
type fakeSteam struct {
	mu     sync.Mutex
	toPeer []struct {
		id  uint64
		pkt []byte
	}
	toAll [][]byte
}

func (s *fakeSteam) SendToPeer(steamID uint64, frame []byte) {
	cp := make([]byte, len(frame))
	copy(cp, frame)
	s.mu.Lock()
	s.toPeer = append(s.toPeer, struct {
		id  uint64
		pkt []byte
	}{steamID, cp})
	s.mu.Unlock()
}

func (s *fakeSteam) SendToAll(frame []byte) {
	cp := make([]byte, len(frame))
	copy(cp, frame)
	s.mu.Lock()
	s.toAll = append(s.toAll, cp)
	s.mu.Unlock()
}

// makeIPv4 builds a minimal 20-byte IPv4 packet with the given src/dst IPs.
func makeIPv4(srcIP, dstIP uint32) []byte {
	p := make([]byte, 20)
	p[0] = 0x45
	p[9] = 17 // UDP
	p[12] = byte(srcIP >> 24)
	p[13] = byte(srcIP >> 16)
	p[14] = byte(srcIP >> 8)
	p[15] = byte(srcIP)
	p[16] = byte(dstIP >> 24)
	p[17] = byte(dstIP >> 16)
	p[18] = byte(dstIP >> 8)
	p[19] = byte(dstIP)
	return p
}

// ip encodes a.b.c.d as uint32 big-endian.
func ip(a, b, c, d byte) uint32 {
	return uint32(a)<<24 | uint32(b)<<16 | uint32(c)<<8 | uint32(d)
}

// ── Table ────────────────────────────────────────────────────────────────────

func TestTable_UpdateIfAbsentOrSame(t *testing.T) {
	tbl := NewTable()
	addr := ip(10, 8, 0, 2)
	if !tbl.UpdateIfAbsentOrSame(addr, 42) {
		t.Fatal("first claim of an unclaimed IP should apply")
	}
	if !tbl.UpdateIfAbsentOrSame(addr, 42) {
		t.Fatal("same-owner refresh should apply")
	}
	if tbl.UpdateIfAbsentOrSame(addr, 43) {
		t.Fatal("claim by a different SteamID should be refused")
	}
	if got, _ := tbl.Lookup(addr); got != 42 {
		t.Fatalf("entry overwritten: got %v, want 42", got)
	}
}

func TestTable_UpdateLookupDelete(t *testing.T) {
	tbl := NewTable()
	tbl.Update(ip(10, 8, 0, 2), 42)
	got, ok := tbl.Lookup(ip(10, 8, 0, 2))
	if !ok || got != 42 {
		t.Fatalf("Lookup after Update: got (%v, %v), want (42, true)", got, ok)
	}
	tbl.Delete(ip(10, 8, 0, 2))
	if _, ok := tbl.Lookup(ip(10, 8, 0, 2)); ok {
		t.Fatal("entry still present after Delete")
	}
}

func TestTable_Snapshot(t *testing.T) {
	tbl := NewTable()
	tbl.Update(ip(10, 8, 0, 2), 1)
	tbl.Update(ip(10, 8, 0, 3), 2)
	snap := tbl.Snapshot()
	if len(snap) != 2 || snap[ip(10, 8, 0, 2)] != 1 || snap[ip(10, 8, 0, 3)] != 2 {
		t.Fatalf("unexpected snapshot: %v", snap)
	}
	// Mutations to snapshot must not affect the table.
	snap[ip(10, 8, 0, 99)] = 99
	if _, ok := tbl.Lookup(ip(10, 8, 0, 99)); ok {
		t.Fatal("snapshot mutation leaked into table")
	}
}

// ── HandleIngress ─────────────────────────────────────────────────────────────

func TestHandleIngress_ValidLanWritesTun(t *testing.T) {
	tun := newFakeTun()
	steam := &fakeSteam{}
	r := NewRouter(tun, steam)

	pkt := makeIPv4(ip(10, 8, 0, 3), ip(10, 8, 0, 2))
	senderID := uint64(99)
	r.HandleIngress(senderID, pkt)

	tun.mu.Lock()
	defer tun.mu.Unlock()
	if len(tun.written) != 1 {
		t.Fatalf("expected 1 write to TUN, got %d", len(tun.written))
	}
	// Table must record sender's source IP.
	sid, ok := r.table.Lookup(ip(10, 8, 0, 3))
	if !ok || sid != senderID {
		t.Errorf("table: got (%v,%v), want (%v,true)", sid, ok, senderID)
	}
}

func TestHandleIngress_TooShortDropped(t *testing.T) {
	tun := newFakeTun()
	r := NewRouter(tun, &fakeSteam{})
	r.HandleIngress(1, []byte{0x45, 0, 0}) // only 3 bytes
	tun.mu.Lock()
	defer tun.mu.Unlock()
	if len(tun.written) != 0 {
		t.Fatal("short packet should have been dropped")
	}
}

func TestHandleIngress_InvalidLanDropped(t *testing.T) {
	tun := newFakeTun()
	r := NewRouter(tun, &fakeSteam{})
	// Public IP destination — should be dropped.
	pkt := makeIPv4(ip(10, 8, 0, 3), ip(8, 8, 8, 8))
	r.HandleIngress(1, pkt)
	tun.mu.Lock()
	defer tun.mu.Unlock()
	if len(tun.written) != 0 {
		t.Fatal("non-LAN packet should have been dropped")
	}
}

func TestHandleIngress_PIHeaderStripped(t *testing.T) {
	// Packets starting with 0x00 or 0x02 have a 4-byte PI/AF header before the
	// IP packet; it must be removed before writing to TUN.
	tests := []struct {
		name   string
		header []byte
	}{
		{name: "Linux PI header (0x00)", header: []byte{0x00, 0x00, 0x08, 0x00}},
		{name: "utun AF header (0x02)", header: []byte{0x02, 0x00, 0x00, 0x02}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tun := newFakeTun()
			r := NewRouter(tun, &fakeSteam{})
			pkt := makeIPv4(ip(10, 8, 0, 3), ip(10, 8, 0, 2))
			piPkt := append(append([]byte{}, tt.header...), pkt...)
			r.HandleIngress(1, piPkt)
			tun.mu.Lock()
			defer tun.mu.Unlock()
			if len(tun.written) != 1 {
				t.Fatal("PI-header packet should have been written to TUN")
			}
			if !bytes.Equal(tun.written[0], pkt) {
				t.Errorf("expected PI header stripped, got %x", tun.written[0])
			}
		})
	}
}

func TestHandleIngress_WritesPacketUnpadded(t *testing.T) {
	// L3 TUN packets must be written as-is: no Ethernet-style minimum-frame padding.
	tun := newFakeTun()
	r := NewRouter(tun, &fakeSteam{})
	pkt := makeIPv4(ip(10, 8, 0, 3), ip(10, 8, 0, 2)) // exactly 20 bytes
	r.HandleIngress(1, pkt)
	tun.mu.Lock()
	defer tun.mu.Unlock()
	if len(tun.written) == 0 {
		t.Fatal("expected write to TUN")
	}
	if !bytes.Equal(tun.written[0], pkt) {
		t.Errorf("expected packet written unmodified (%d bytes), got %d bytes", len(pkt), len(tun.written[0]))
	}
}

func TestHandleIngress_RefusesTablePoisoning(t *testing.T) {
	tun := newFakeTun()
	r := NewRouter(tun, &fakeSteam{})
	srcIP := ip(10, 8, 0, 3)
	pkt := makeIPv4(srcIP, ip(10, 8, 0, 2))

	r.HandleIngress(77, pkt) // legitimate owner claims 10.8.0.3
	r.HandleIngress(66, pkt) // different peer tries to claim the same IP

	sid, ok := r.table.Lookup(srcIP)
	if !ok || sid != 77 {
		t.Fatalf("table entry hijacked: got (%v,%v), want (77,true)", sid, ok)
	}
	tun.mu.Lock()
	defer tun.mu.Unlock()
	if len(tun.written) != 1 {
		t.Errorf("spoofed packet should have been dropped, got %d writes", len(tun.written))
	}
}

func TestHandleIngress_SameOwnerRefreshAllowed(t *testing.T) {
	tun := newFakeTun()
	r := NewRouter(tun, &fakeSteam{})
	pkt := makeIPv4(ip(10, 8, 0, 3), ip(10, 8, 0, 2))

	r.HandleIngress(77, pkt)
	r.HandleIngress(77, pkt) // same peer refreshes its own entry

	tun.mu.Lock()
	defer tun.mu.Unlock()
	if len(tun.written) != 2 {
		t.Errorf("same-owner refresh should pass, got %d writes", len(tun.written))
	}
}

// ── Firewall ──────────────────────────────────────────────────────────────────

func TestFirewall_StateRoundTrip(t *testing.T) {
	r := NewRouter(newFakeTun(), &fakeSteam{})
	if r.GetFirewallState() {
		t.Fatal("firewall should be disabled by default")
	}
	r.SetFirewall(true)
	if !r.GetFirewallState() {
		t.Fatal("firewall should be enabled after SetFirewall(true)")
	}
}

func TestFirewall_PortRoundTrip(t *testing.T) {
	r := NewRouter(newFakeTun(), &fakeSteam{})
	r.AddPort(80)
	r.AddPort(443)
	ports := r.GetAllowedPorts()
	have := map[uint16]bool{}
	for _, p := range ports {
		have[p] = true
	}
	if !have[80] || !have[443] {
		t.Fatalf("expected ports 80 and 443, got %v", ports)
	}
	r.RemovePort(80)
	for _, p := range r.GetAllowedPorts() {
		if p == 80 {
			t.Fatal("port 80 still present after RemovePort")
		}
	}
}

func TestGetAllowedPorts_SortedAscending(t *testing.T) {
	r := NewRouter(newFakeTun(), &fakeSteam{})
	for _, p := range []uint16{8080, 22, 443, 80, 27015} {
		r.AddPort(p)
	}
	ports := r.GetAllowedPorts()
	for i := 1; i < len(ports); i++ {
		if ports[i-1] > ports[i] {
			t.Fatalf("ports not sorted ascending: %v", ports)
		}
	}
	if len(ports) != 5 {
		t.Fatalf("expected 5 ports, got %v", ports)
	}
}

func TestFirewall_BlocksIngressOnDisallowedPort(t *testing.T) {
	tun := newFakeTun()
	r := NewRouter(tun, &fakeSteam{})
	r.SetFirewall(true)
	r.AddPort(443)

	// UDP packet src=10.8.0.3 dst=10.8.0.2 on port 9999 (not allowed).
	pkt := make([]byte, 24)
	pkt[0] = 0x45
	pkt[9] = 17 // UDP
	pkt[12], pkt[13], pkt[14], pkt[15] = 10, 8, 0, 3
	pkt[16], pkt[17], pkt[18], pkt[19] = 10, 8, 0, 2
	pkt[20], pkt[21] = 0x27, 0x0F // src port 9999
	pkt[22], pkt[23] = 0x27, 0x0F // dst port 9999
	r.HandleIngress(1, pkt)

	tun.mu.Lock()
	defer tun.mu.Unlock()
	if len(tun.written) != 0 {
		t.Fatal("firewall should have blocked packet on disallowed port")
	}
}

// ── StartEgress ───────────────────────────────────────────────────────────────

// buildEgressPkt constructs a raw packet as StartEgress reads from TUN:
// no PacketTypeData tag yet (router adds it), just raw IPv4.
func buildEgressIPv4(dstIP uint32) []byte {
	pkt := make([]byte, 20)
	pkt[0] = 0x45
	pkt[9] = 17
	pkt[12], pkt[13], pkt[14], pkt[15] = 10, 8, 0, 1
	pkt[16] = byte(dstIP >> 24)
	pkt[17] = byte(dstIP >> 16)
	pkt[18] = byte(dstIP >> 8)
	pkt[19] = byte(dstIP)
	return pkt
}

// runEgress feeds the given packets, waits for them to be consumed, cancels
// the ctx, and returns whatever error StartEgress returned.
func runEgress(r *Router, pkts ...[]byte) error {
	ft := r.tunDev.(*fakeTun)
	ft.mu.Lock()
	ft.packets = append(ft.packets, pkts...)
	ft.readIdx = 0
	ft.done = make(chan struct{})
	ft.doneOnce = sync.Once{}
	ft.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- r.StartEgress(ctx) }()
	<-ft.done // all scripted packets fed
	cancel()  // signal shutdown
	return <-errCh
}

// TestStartEgress_ReturnsNilOnEOF verifies that a TUN EOF (normal close) maps to nil.
func TestStartEgress_ReturnsNilOnEOF(t *testing.T) {
	ft := newFakeTun()
	r := NewRouter(ft, &fakeSteam{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- r.StartEgress(ctx) }()
	<-ft.done                                 // fakeTun blocks until unblocked
	ft.doneOnce.Do(func() { close(ft.done) }) // trigger EOF path (already closed by done signal)
	// Unblock via close event
	ft.Unblock() //nolint:errcheck

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("expected nil on EOF, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("StartEgress did not return after Unblock")
	}
}

// TestStartEgress_ReturnsErrOnReadError verifies unexpected TUN errors are propagated.
func TestStartEgress_ReturnsErrOnReadError(t *testing.T) {
	sentinelErr := errors.New("tun hardware failure")
	ft := newFakeTun()
	ft.readErr = sentinelErr
	r := NewRouter(ft, &fakeSteam{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- r.StartEgress(ctx) }()
	ft.Unblock() //nolint:errcheck

	select {
	case err := <-errCh:
		if !errors.Is(err, sentinelErr) {
			t.Errorf("expected %v, got %v", sentinelErr, err)
		}
	case <-time.After(time.Second):
		t.Fatal("StartEgress did not return after injected error")
	}
}

func TestStartEgress_TableHitSendsToPeer(t *testing.T) {
	ft := newFakeTun()
	steam := &fakeSteam{}
	r := NewRouter(ft, steam)
	destIP := ip(10, 8, 0, 3)
	r.table.Update(destIP, 77) // peer 77 is at 10.8.0.3

	runEgress(r, buildEgressIPv4(destIP))

	steam.mu.Lock()
	defer steam.mu.Unlock()
	if len(steam.toPeer) != 1 || steam.toPeer[0].id != 77 {
		t.Fatalf("expected SendToPeer(77), got toPeer=%v toAll=%v", steam.toPeer, steam.toAll)
	}
}

func TestStartEgress_TableMissSendsToAll(t *testing.T) {
	ft := newFakeTun()
	steam := &fakeSteam{}
	r := NewRouter(ft, steam)
	// No table entry for 10.8.0.9

	runEgress(r, buildEgressIPv4(ip(10, 8, 0, 9)))

	steam.mu.Lock()
	defer steam.mu.Unlock()
	if len(steam.toAll) != 1 {
		t.Fatalf("expected SendToAll on table miss, got toPeer=%v toAll=%v", steam.toPeer, steam.toAll)
	}
}

func TestStartEgress_ShortReadSkipped(t *testing.T) {
	ft := newFakeTun()
	steam := &fakeSteam{}
	r := NewRouter(ft, steam)

	// 10 bytes: no complete IPv4 header, must be skipped.
	runEgress(r, buildEgressIPv4(ip(10, 8, 0, 3))[:10])

	steam.mu.Lock()
	defer steam.mu.Unlock()
	if len(steam.toPeer) != 0 || len(steam.toAll) != 0 {
		t.Fatalf("short read should be skipped, got toPeer=%v toAll=%v", steam.toPeer, steam.toAll)
	}
}

func TestStartEgress_ValidatesOnlyReadBytes(t *testing.T) {
	// A packet must be judged on its own bytes, not on stale bytes left in the
	// read buffer by a previous, longer packet.
	ft := newFakeTun()
	steam := &fakeSteam{}
	r := NewRouter(ft, steam)
	r.SetFirewall(true)
	r.AddPort(443)

	// First packet: 24 bytes, UDP on allowed port 443 — passes the firewall.
	allowed := buildEgressIPv4(ip(10, 8, 0, 3))
	allowed = append(allowed, 0x01, 0xBB, 0x01, 0xBB) // src/dst port 443

	// Second packet: 20 bytes, UDP with no L4 header. With stale bytes from the
	// first packet the old code saw port 443 and let it through.
	headerOnly := buildEgressIPv4(ip(10, 8, 0, 3))

	runEgress(r, allowed, headerOnly)

	steam.mu.Lock()
	defer steam.mu.Unlock()
	total := len(steam.toPeer) + len(steam.toAll)
	if total != 1 {
		t.Fatalf("expected only the first packet to pass the firewall, got %d sends", total)
	}
}

func TestStartEgress_BroadcastSendsToAll(t *testing.T) {
	ft := newFakeTun()
	steam := &fakeSteam{}
	r := NewRouter(ft, steam)

	runEgress(r, buildEgressIPv4(0xFFFFFFFF))

	steam.mu.Lock()
	defer steam.mu.Unlock()
	if len(steam.toAll) != 1 {
		t.Fatalf("expected SendToAll for broadcast, got toPeer=%v toAll=%v", steam.toPeer, steam.toAll)
	}
}
