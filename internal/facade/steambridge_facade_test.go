package facade

import "testing"

// newStoppedFacade returns a facade that was never started: router and client
// are nil, running is false. This mirrors the state the UI can reach while the
// bridge is stopped.
func newStoppedFacade() *Facade {
	return NewFacade(Config{IfaceName: "sbtest", IfaceID: "test"})
}

// TestNilGuardsWhileStopped ensures every UI-reachable method is safe to call
// before Start (regression test for nil-pointer panics in AddPort, RemovePort,
// SetFirewall, and GetLocalSteamID).
func TestNilGuardsWhileStopped(t *testing.T) {
	f := newStoppedFacade()

	f.AddPort(8080)
	f.RemovePort(8080)
	f.SetFirewall(true)
	f.SetFirewall(false)

	if got := f.GetLocalSteamID(); got != 0 {
		t.Errorf("GetLocalSteamID() = %d, want 0 while stopped", got)
	}
	if got := f.GetLocalIP(); got != 0 {
		t.Errorf("GetLocalIP() = %d, want 0 while stopped", got)
	}
	if got := f.GetPeerTable(); got != nil {
		t.Errorf("GetPeerTable() = %v, want nil while stopped", got)
	}
	if got := f.GetFirewallState(); got != false {
		t.Errorf("GetFirewallState() = %v, want false while stopped", got)
	}
	if got := f.GetAllowedPorts(); got != nil {
		t.Errorf("GetAllowedPorts() = %v, want nil while stopped", got)
	}
	if f.IsRunning() {
		t.Error("IsRunning() = true, want false before Start")
	}
}

// TestJoinAndRespondRequireRunning verifies the running-state guards error out
// instead of dereferencing a nil client.
func TestJoinAndRespondRequireRunning(t *testing.T) {
	f := newStoppedFacade()

	if err := f.Join(123); err == nil {
		t.Error("Join() = nil error, want error while stopped")
	}
	if err := f.RespondToJoinRequest(123, true); err == nil {
		t.Error("RespondToJoinRequest() = nil error, want error while stopped")
	}
	// OpenFriendsOverlay already guards nil client; must not panic.
	f.OpenFriendsOverlay()
}

// TestStopIdempotentWhileStopped ensures Stop on a never-started facade is a
// safe no-op, including when called repeatedly.
func TestStopIdempotentWhileStopped(t *testing.T) {
	f := newStoppedFacade()

	for i := 0; i < 3; i++ {
		if err := f.Stop(); err != nil {
			t.Errorf("Stop() call %d = %v, want nil", i+1, err)
		}
	}
	if f.IsRunning() {
		t.Error("IsRunning() = true after Stop")
	}
}

// TestHostIPConstant pins the self-assigned host IP to the subnet root
// 10.8.0.1 (the IPAM pool starts handing out addresses at .2).
func TestHostIPConstant(t *testing.T) {
	want := uint32(10)<<24 | uint32(8)<<16 | uint32(0)<<8 | 1
	if hostIP != want {
		t.Errorf("hostIP = %#x, want %#x (10.8.0.1)", hostIP, want)
	}
}
