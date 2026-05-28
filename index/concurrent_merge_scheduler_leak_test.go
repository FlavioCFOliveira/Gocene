// Copyright 2026 Gocene. All rights reserved.

package index

import (
	"runtime"
	"testing"
	"time"
)

// TestConcurrentMergeScheduler_AwaitMergesNoGoroutineLeakOnTimeout verifies that
// the merge-watch goroutine does not leak when the wait times out. A non-zero
// running-merge count is simulated so the watcher never observes a drain; the
// wait must hit the timeout and, because the watcher is cancelled when the wait
// returns, the goroutine must have exited by the time the call returns.
func TestConcurrentMergeScheduler_AwaitMergesNoGoroutineLeakOnTimeout(t *testing.T) {
	s := NewConcurrentMergeScheduler()

	// Simulate a still-running merge so GetRunningMergeCount never reaches zero.
	// This is the situation that previously left the watcher goroutine blocked
	// after the timeout fired.
	s.IncrementRunningMerges()

	// settle reports whether the goroutine count drops to at most want within a
	// short window. The watcher is cancelled synchronously when the wait returns,
	// so this mainly absorbs scheduler latency in tearing the goroutine down.
	settle := func(want int) (int, bool) {
		deadline := time.Now().Add(2 * time.Second)
		var got int
		for time.Now().Before(deadline) {
			got = runtime.NumGoroutine()
			if got <= want {
				return got, true
			}
			time.Sleep(5 * time.Millisecond)
		}
		return got, false
	}

	runtime.GC()
	baseline := runtime.NumGoroutine()

	// The count is stuck at 1, so this must hit the timeout path.
	start := time.Now()
	err := s.awaitMergesOrTimeout(20 * time.Millisecond)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error from awaitMergesOrTimeout, got nil")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("awaitMergesOrTimeout took %v, expected it to return shortly after the 20ms timeout", elapsed)
	}

	// The watcher goroutine must be gone now that the wait returned (its context
	// was cancelled). Allow a brief settle for runtime teardown only.
	if got, ok := settle(baseline); !ok {
		t.Fatalf("watcher goroutine leaked after timeout: %d goroutines, want <= %d (baseline)", got, baseline)
	}

	// Now let the merge "finish" and confirm the success path also returns
	// without leaking a goroutine.
	s.DecrementRunningMerges()
	runtime.GC()
	baseline = runtime.NumGoroutine()

	if err := s.awaitMergesOrTimeout(2 * time.Second); err != nil {
		t.Fatalf("expected nil error on success path, got %v", err)
	}
	if got, ok := settle(baseline); !ok {
		t.Fatalf("watcher goroutine leaked after success: %d goroutines, want <= %d (baseline)", got, baseline)
	}
}
