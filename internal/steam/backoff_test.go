package steam

import (
	"context"
	"testing"
	"time"
)

// The first idle() after gotPacket() (or on a fresh backoff) must sleep about
// minIdleSleep — not zero, not the max.
func TestIdleBackoff_FirstIdleSleepsMinimum(t *testing.T) {
	var b idleBackoff
	start := time.Now()
	b.idle(context.Background())
	elapsed := time.Since(start)

	if elapsed < minIdleSleep {
		t.Errorf("first idle slept %v, want >= %v", elapsed, minIdleSleep)
	}
	if elapsed > 4*minIdleSleep {
		t.Errorf("first idle slept %v, want close to %v", elapsed, minIdleSleep)
	}
}

// Consecutive idle() calls without a packet must grow the interval and cap at
// maxIdleSleep — never longer.
func TestIdleBackoff_GrowsAndCaps(t *testing.T) {
	var b idleBackoff
	for i := 0; i < 12; i++ {
		start := time.Now()
		b.idle(context.Background())
		elapsed := time.Since(start)
		if elapsed > maxIdleSleep+10*time.Millisecond {
			t.Fatalf("idle #%d slept %v, exceeds cap %v", i, elapsed, maxIdleSleep)
		}
	}

	// After many idles the interval must have grown well past the minimum.
	start := time.Now()
	b.idle(context.Background())
	if elapsed := time.Since(start); elapsed < 4*minIdleSleep {
		t.Errorf("after 12 idles, interval is %v — expected it to have backed off", elapsed)
	}
}

// gotPacket() resets the backoff: the next idle() drops back to the minimum.
func TestIdleBackoff_GotPacketResets(t *testing.T) {
	var b idleBackoff
	for i := 0; i < 10; i++ {
		b.idle(context.Background())
	}
	b.gotPacket()

	start := time.Now()
	b.idle(context.Background())
	if elapsed := time.Since(start); elapsed > 4*minIdleSleep {
		t.Errorf("idle after gotPacket slept %v, want back down near %v", elapsed, minIdleSleep)
	}
}

// A cancelled context must abort the sleep promptly rather than blocking for the
// full (possibly 32ms) interval — this is what keeps Stop() responsive.
func TestIdleBackoff_CancelledContextReturnsPromptly(t *testing.T) {
	var b idleBackoff
	// Grow the interval to the cap first.
	for i := 0; i < 12; i++ {
		b.idle(context.Background())
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	b.idle(ctx)
	if elapsed := time.Since(start); elapsed > 5*time.Millisecond {
		t.Errorf("idle with cancelled ctx blocked for %v, want near-immediate return", elapsed)
	}
}