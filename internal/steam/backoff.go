package steam

import (
	"context"
	"time"
)

// idleBackoff paces ReadLoop's receive polling.
//
// ISteamNetworkingMessages delivers inbound data through ReceiveMessagesOnChannel,
// which returns nothing when the queue is empty — there is no blocking receive or
// pollable fd in the Steamworks SDK. So ReadLoop still polls. While packets are
// flowing (a running game sends constantly) we want to poll flat-out for low
// latency; once the link goes quiet we want to stop pegging a CPU core.
//
// idleBackoff keeps the loop hot during traffic and grows the inter-poll sleep
// geometrically toward maxIdleSleep during silence.
type idleBackoff struct {
	cur time.Duration
}

const (
	// minIdleSleep is the sleep after the first empty poll following activity.
	minIdleSleep = time.Millisecond
	// maxIdleSleep caps the backoff. It also bounds how long a quiet ReadLoop
	// takes to notice ctx cancellation and how stale bridgeRunCallbacks() gets.
	maxIdleSleep = 32 * time.Millisecond
)

// gotPacket is called after ReadLoop receives a packet. It returns the loop to
// full poll rate so the next quiet spell starts backing off from minIdleSleep.
func (b *idleBackoff) gotPacket() {
	b.cur = minIdleSleep
}

// idle is called when a receive poll found no packet. It sleeps for the current
// backoff interval (aborting early if ctx is cancelled), then grows the interval
// geometrically, capped at maxIdleSleep, for the next call.
func (b *idleBackoff) idle(ctx context.Context) {
	if b.cur < minIdleSleep {
		b.cur = minIdleSleep
	}
	timer := time.NewTimer(b.cur)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-timer.C:
	}
	b.cur = min(maxIdleSleep, b.cur*2)
}
