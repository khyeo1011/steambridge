package steam

import (
	"context"
	"encoding/binary"
	"steambridge/internal/ipam"
	"steambridge/internal/protocol"
	"steambridge/internal/router"
	"sync"
	"testing"
	"time"
	"unsafe"
)

// fakeRouter satisfies RouterInterface without a real TUN or Steam connection.
type fakeRouter struct {
	mu       sync.Mutex
	ingress  []struct{ id uint64; pkt []byte }
	setIPErr error
	devName  string
}

func (r *fakeRouter) HandleIngress(senderID uint64, packet []byte) {
	cp := make([]byte, len(packet))
	copy(cp, packet)
	r.mu.Lock()
	r.ingress = append(r.ingress, struct{ id uint64; pkt []byte }{senderID, cp})
	r.mu.Unlock()
}
func (r *fakeRouter) SetSteamSender(_ router.SteamSender)     {}
func (r *fakeRouter) StartEgress(_ context.Context) error     { return nil }
func (r *fakeRouter) AddPort(_ uint16)                     {}
func (r *fakeRouter) RemovePort(_ uint16)                  {}
func (r *fakeRouter) SetFirewall(_ bool)                   {}
func (r *fakeRouter) SetIP(_ uint32) error                 { return r.setIPErr }
func (r *fakeRouter) GetDevName() string                   { return r.devName }

// newTestClient builds a Client with DLL vars replaced by fakes.
// Returns the client, the router, and a slice pointer for reliable sends.
func newTestClient(t *testing.T) (*Client, *fakeRouter, *[][]byte) {
	t.Helper()
	sent := &[][]byte{}
	bridgeInit = func() bool { return true }
	bridgeRunCallbacks = func() {}
	bridgeShutdown = func() {}
	bridgeGetLocalSteamID = func() uint64 { return 1 }
	bridgeGetJoinRequest = func() uint64 { return 0 }
	bridgeSetJoinable = func(bool) {}
	bridgeOpenFriendsOverlay = func() {}
	bridgeSend = func(_ uint64, _ *byte, _ int) bool { return true }
	bridgeSendReliable = func(_ uint64, data *byte, size int) bool {
		buf := make([]byte, size)
		copy(buf, unsafe.Slice(data, size))
		*sent = append(*sent, buf)
		return true
	}
	bridgeReceive = func(_ *byte, _ int, _ *uint64) int32 { return 0 }
	bridgeGetSessionRequest = func() uint64 { return 0 }
	bridgeIsFriend = func(_ uint64) bool { return false }
	bridgeAcceptSession = func(_ uint64) {}
	bridgeRejectSession = func(_ uint64) {}

	fr := &fakeRouter{}
	c := &Client{
		router:       fr,
		steamIDs:     make(map[uint64]bool),
		pendingJoins: make(map[uint64]bool),
		ipPool:       ipam.NewPool(),
		steamID:      1,
	}
	return c, fr, sent
}

// makeControlFrame builds a 6-byte control packet.
func makeControlFrame(action uint8, ipAddr uint32) []byte {
	f := make([]byte, 6)
	f[0] = protocol.PacketTypeControl
	f[1] = action
	binary.BigEndian.PutUint32(f[2:], ipAddr)
	return f
}

// makeDataFrame wraps a payload as a data packet.
func makeDataFrame(payload []byte) []byte {
	return append([]byte{protocol.PacketTypeData}, payload...)
}

// feedSessionRequest makes bridgeGetSessionRequest return id once, then 0.
func feedSessionRequest(id uint64) {
	delivered := false
	bridgeGetSessionRequest = func() uint64 {
		if delivered {
			return 0
		}
		delivered = true
		return id
	}
}

// feedPacket replaces bridgeReceive so it returns pkt once, then 0 thereafter.
func feedPacket(pkt []byte, remoteSteamID uint64) {
	delivered := false
	bridgeReceive = func(buffer *byte, bufferSize int, outID *uint64) int32 {
		if delivered {
			return 0
		}
		delivered = true
		*outID = remoteSteamID
		n := copy(unsafe.Slice(buffer, bufferSize), pkt)
		return int32(n)
	}
}

// ── Join / resolveJoin ────────────────────────────────────────────────────────

func TestJoin_RegistersPendingAndFiresCallback(t *testing.T) {
	c, _, sent := newTestClient(t)

	var gotHost uint64
	c.SetJoinHandler(func(h uint64) { gotHost = h })

	c.Join(99)

	// pendingJoins must record the host.
	c.joinMutex.Lock()
	_, ok := c.pendingJoins[99]
	c.joinMutex.Unlock()
	if !ok {
		t.Error("Join did not add host to pendingJoins")
	}
	// Must have fired onJoinRequest.
	if gotHost != 99 {
		t.Errorf("onJoinRequest: got %v, want 99", gotHost)
	}
	// Must have sent ActionRequestIP via SendControlMessage.
	if len(*sent) == 0 {
		t.Fatal("expected at least one reliable send")
	}
	frame := (*sent)[0]
	if frame[0] != protocol.PacketTypeControl || frame[1] != protocol.ActionRequestIP {
		t.Errorf("first sent frame wrong: %v", frame)
	}
}

func TestResolveJoin_FiresOnlyOnce(t *testing.T) {
	c, _, _ := newTestClient(t)
	var results []bool
	c.SetJoinResultHandler(func(_ uint64, connected bool) {
		results = append(results, connected)
	})

	c.joinMutex.Lock()
	c.pendingJoins[77] = true
	c.joinMutex.Unlock()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.resolveJoin(77, true)
		}()
	}
	wg.Wait()

	if len(results) != 1 {
		t.Errorf("onJoinResult fired %d times, want exactly 1", len(results))
	}
}

// ── ReadLoop IPAM routing ─────────────────────────────────────────────────────

func runReadLoopOnce(c *Client) {
	ctx, cancel := context.WithCancel(context.Background())
	// Cancel after first pass — we replace bridgeReceive to return 0 after one packet.
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	c.ReadLoop(ctx)
}

func TestReadLoop_ActionRequestIP_AllocatesAndReplies(t *testing.T) {
	c, _, sent := newTestClient(t)
	feedPacket(makeControlFrame(protocol.ActionRequestIP, 0), 55)

	runReadLoopOnce(c)

	// Must have sent back ActionOfferIP.
	var found bool
	for _, f := range *sent {
		if len(f) >= 2 && f[0] == protocol.PacketTypeControl && f[1] == protocol.ActionOfferIP {
			found = true
		}
	}
	if !found {
		t.Errorf("no ActionOfferIP found in sends: %v", *sent)
	}
}

func TestReadLoop_ActionNackIP_ReleasesLease(t *testing.T) {
	c, _, _ := newTestClient(t)
	// Pre-allocate an IP for steamID 55.
	allocatedIP := c.ipPool.Allocate(55)
	// Now feed a NACK for that IP.
	feedPacket(makeControlFrame(protocol.ActionNackIP, allocatedIP), 55)

	runReadLoopOnce(c)

	// After NACK, the IP should be back in the free-list — a new Allocate for
	// a different steamID should reclaim it.
	recycled := c.ipPool.Allocate(99)
	if recycled != allocatedIP {
		t.Errorf("expected released IP %08x to be recycled, got %08x", allocatedIP, recycled)
	}
}

func TestReadLoop_ActionNackIP_IgnoresForgedIP(t *testing.T) {
	c, _, _ := newTestClient(t)
	// Victim 55 holds a lease; attacker 66 sends a NACK naming the victim's IP.
	victimIP := c.ipPool.Allocate(55)
	feedPacket(makeControlFrame(protocol.ActionNackIP, victimIP), 66)

	runReadLoopOnce(c)

	// The victim's lease must be untouched: a fresh Allocate for a new peer
	// must NOT hand out the victim's IP.
	if got := c.ipPool.Allocate(99); got == victimIP {
		t.Errorf("forged NACK from 66 released victim 55's lease %08x", victimIP)
	}
}

func TestReadLoop_ActionNackIP_ResolvesPendingJoin(t *testing.T) {
	c, _, _ := newTestClient(t)
	var gotHost uint64
	var gotConnected = true
	var fired bool
	c.SetJoinResultHandler(func(h uint64, connected bool) {
		fired = true
		gotHost = h
		gotConnected = connected
	})
	c.joinMutex.Lock()
	c.pendingJoins[88] = true
	c.joinMutex.Unlock()

	// Host 88 NACKs the join (e.g. pool exhausted).
	feedPacket(makeControlFrame(protocol.ActionNackIP, 0), 88)
	runReadLoopOnce(c)

	if !fired {
		t.Fatal("onJoinResult did not fire on NACK")
	}
	if gotHost != 88 || gotConnected {
		t.Errorf("onJoinResult: got (%d, %v), want (88, false)", gotHost, gotConnected)
	}
}

func TestReadLoop_ActionRequestIP_ExhaustedPool_Nacks(t *testing.T) {
	c, _, sent := newTestClient(t)
	// Exhaust the pool: hosts 2..254.
	for i := 0; i < 253; i++ {
		if c.ipPool.Allocate(uint64(1000+i)) == 0 {
			t.Fatalf("pool exhausted early at lease #%d", i+1)
		}
	}
	feedPacket(makeControlFrame(protocol.ActionRequestIP, 0), 55)

	runReadLoopOnce(c)

	var nacked, offered bool
	for _, f := range *sent {
		if len(f) >= 6 && f[0] == protocol.PacketTypeControl {
			switch f[1] {
			case protocol.ActionNackIP:
				if binary.BigEndian.Uint32(f[2:6]) == 0 {
					nacked = true
				}
			case protocol.ActionOfferIP:
				offered = true
			}
		}
	}
	if !nacked {
		t.Errorf("expected ActionNackIP(0) on exhausted pool, sends: %v", *sent)
	}
	if offered {
		t.Errorf("must not send ActionOfferIP on exhausted pool, sends: %v", *sent)
	}
}

func TestReadLoop_UndersizedControlFrame_NoCrash(t *testing.T) {
	c, _, _ := newTestClient(t)
	// Control frame with only 3 bytes (less than the required 6).
	feedPacket([]byte{protocol.PacketTypeControl, protocol.ActionRequestIP, 0x00}, 55)
	runReadLoopOnce(c) // must not panic
}

func TestReadLoop_NegativeBytesRead_ReturnsError(t *testing.T) {
	c, _, _ := newTestClient(t)
	bridgeReceive = func(_ *byte, _ int, _ *uint64) int32 { return -1 }

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := c.ReadLoop(ctx)
	if err == nil {
		t.Error("expected non-nil error when bridgeReceive returns -1, got nil")
	}
}

func TestReadLoop_DataPacket_ForwardedToRouter(t *testing.T) {
	c, fr, _ := newTestClient(t)
	payload := []byte{0x45, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}
	feedPacket(makeDataFrame(payload), 42)

	runReadLoopOnce(c)

	fr.mu.Lock()
	defer fr.mu.Unlock()
	if len(fr.ingress) != 1 || fr.ingress[0].id != 42 {
		t.Errorf("expected 1 ingress from sender 42, got %v", fr.ingress)
	}
	// Sender must be recorded as a peer.
	c.peermutex.RLock()
	defer c.peermutex.RUnlock()
	if !c.steamIDs[42] {
		t.Error("data sender 42 not recorded as peer")
	}
}

// ── Session-request gating ────────────────────────────────────────────────────

func TestReadLoop_SessionRequest_Friend_AutoAccepts(t *testing.T) {
	c, _, _ := newTestClient(t)
	var acceptedID uint64
	bridgeIsFriend = func(id uint64) bool { return id == 42 }
	bridgeAcceptSession = func(id uint64) { acceptedID = id }
	feedSessionRequest(42)

	var confirmFired bool
	c.SetJoinConfirmHandler(func(_ uint64) { confirmFired = true })

	runReadLoopOnce(c)

	if acceptedID != 42 {
		t.Errorf("bridgeAcceptSession: got %d, want 42", acceptedID)
	}
	if confirmFired {
		t.Error("onJoinConfirm must not fire for a friend")
	}
	c.peermutex.RLock()
	defer c.peermutex.RUnlock()
	if !c.steamIDs[42] {
		t.Error("friend 42 should be added as peer after auto-accept")
	}
}

func TestReadLoop_SessionRequest_Stranger_PromptHost(t *testing.T) {
	c, _, _ := newTestClient(t)
	var acceptedID uint64
	bridgeIsFriend = func(_ uint64) bool { return false }
	bridgeAcceptSession = func(id uint64) { acceptedID = id }
	feedSessionRequest(77)

	var confirmID uint64
	c.SetJoinConfirmHandler(func(id uint64) { confirmID = id })

	runReadLoopOnce(c)

	if confirmID != 77 {
		t.Errorf("onJoinConfirm: got %d, want 77", confirmID)
	}
	if acceptedID != 0 {
		t.Errorf("bridgeAcceptSession must not be called for a stranger, got %d", acceptedID)
	}
}

func TestRespondToJoinRequest_Accept(t *testing.T) {
	c, _, _ := newTestClient(t)
	var acceptedID uint64
	bridgeAcceptSession = func(id uint64) { acceptedID = id }

	c.RespondToJoinRequest(55, true)

	if acceptedID != 55 {
		t.Errorf("bridgeAcceptSession: got %d, want 55", acceptedID)
	}
	c.peermutex.RLock()
	defer c.peermutex.RUnlock()
	if !c.steamIDs[55] {
		t.Error("accepted peer 55 should be added as peer")
	}
}

func TestRespondToJoinRequest_Reject(t *testing.T) {
	c, _, _ := newTestClient(t)
	var rejectedID uint64
	bridgeRejectSession = func(id uint64) { rejectedID = id }

	c.RespondToJoinRequest(55, false)

	if rejectedID != 55 {
		t.Errorf("bridgeRejectSession: got %d, want 55", rejectedID)
	}
	c.peermutex.RLock()
	defer c.peermutex.RUnlock()
	if c.steamIDs[55] {
		t.Error("rejected peer 55 must not be added as peer")
	}
}

// ── Disconnect ────────────────────────────────────────────────────────────────

func TestDisconnect_BroadcastsToAllPeers(t *testing.T) {
	c, _, sent := newTestClient(t)
	// Register two peers.
	c.AddPeer(10)
	c.AddPeer(20)
	// Give the client a local IP so the disconnect frame carries it.
	c.localIP.Store(0x0A080001) // 10.8.0.1

	c.Disconnect()

	// Each peer must have received exactly one ActionDisconnect reliable frame.
	peerFrames := map[uint64]int{}
	for _, f := range *sent {
		if len(f) >= 2 && f[0] == protocol.PacketTypeControl && f[1] == protocol.ActionDisconnect {
			// The destination steamID isn't in the frame itself; count total frames.
			peerFrames[0]++
		}
	}
	if peerFrames[0] != 2 {
		t.Errorf("expected 2 ActionDisconnect frames (one per peer), got %d: %v", peerFrames[0], *sent)
	}
}

func TestReadLoop_ActionDisconnect_RemovesPeerAndReleasesIP(t *testing.T) {
	c, _, _ := newTestClient(t)

	// Pre-allocate an IP for the peer that will disconnect.
	const disconnectingPeer = uint64(55)
	allocatedIP := c.ipPool.Allocate(disconnectingPeer)

	// Register the peer.
	c.AddPeer(disconnectingPeer)

	// Feed an ActionDisconnect frame from that peer.
	feedPacket(makeControlFrame(protocol.ActionDisconnect, allocatedIP), disconnectingPeer)

	runReadLoopOnce(c)

	// The IP must be released (recyclable by a new allocation).
	recycled := c.ipPool.Allocate(99)
	if recycled != allocatedIP {
		t.Errorf("expected released IP %08x to be recycled, got %08x", allocatedIP, recycled)
	}

	// The peer must be removed from steamIDs.
	c.peermutex.RLock()
	defer c.peermutex.RUnlock()
	if c.steamIDs[disconnectingPeer] {
		t.Errorf("peer %d should have been removed after ActionDisconnect", disconnectingPeer)
	}
}

func TestReadLoop_DoubleDisconnect_ReleasesOnce(t *testing.T) {
	c, _, _ := newTestClient(t)
	const peer = uint64(55)
	allocatedIP := c.ipPool.Allocate(peer)
	c.AddPeer(peer)

	// The same peer sends ActionDisconnect twice (e.g. a retransmit).
	feedPacket(makeControlFrame(protocol.ActionDisconnect, allocatedIP), peer)
	runReadLoopOnce(c)
	feedPacket(makeControlFrame(protocol.ActionDisconnect, allocatedIP), peer)
	runReadLoopOnce(c)

	// The freed IP must be on the free-list exactly once: the first new
	// allocation recycles it, the second must get a fresh IP.
	if got := c.ipPool.Allocate(98); got != allocatedIP {
		t.Errorf("first Allocate after disconnect: got %08x, want recycled %08x", got, allocatedIP)
	}
	if got := c.ipPool.Allocate(99); got == allocatedIP {
		t.Errorf("IP %08x handed out twice — double disconnect double-released the lease", allocatedIP)
	}
}
