// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.sandbox.search.TestQueryProfilerTimer.
package search

import (
	"sync/atomic"
	"testing"
)

// TestQueryProfilerTimer_TimingInterval mirrors testTimingInterval.
// Verifies that nanoTime() is called at most O(log N) times for N calls,
// specifically 3356 times for 100_000 calls.
func TestQueryProfilerTimer_TimingInterval(t *testing.T) {
	var nanoTimeCallCounter atomic.Int64
	var timeVal int64 = 50

	timer := newQueryProfilerTimerWithClock(func() int64 {
		nanoTimeCallCounter.Add(1)
		timeVal++
		return timeVal
	})

	for i := 0; i < 100_000; i++ {
		timer.Start()
		timer.Stop()
		if i < 256 {
			// For the first 256 calls, nanoTime() is called once for Start and
			// once for Stop.
			expected := int64((i + 1) * 2)
			if got := nanoTimeCallCounter.Load(); got != expected {
				t.Errorf("after %d calls: nanoTimeCallCounter = %d; want %d", i+1, got, expected)
			}
		}
	}
	// Total nanoTime calls should be well below 100_000; Java expects 3356.
	if got := nanoTimeCallCounter.Load(); got != 3356 {
		t.Errorf("total nanoTime calls = %d; want 3356", got)
	}
}

// TestQueryProfilerTimer_Extrapolate mirrors testExtrapolate.
// Verifies that GetApproximateTiming() extrapolates correctly so that
// timing == count * elapsed_per_call even when some calls are not timed.
func TestQueryProfilerTimer_Extrapolate(t *testing.T) {
	var timeVal int64 = 50
	timer := newQueryProfilerTimerWithClock(func() int64 {
		timeVal += 42
		return timeVal
	})

	timer.Start()
	timer.Stop()
	timerStartTime := timer.GetEarliestTimerStartTime()

	for i := 2; i < 100_000; i++ {
		timer.Start()
		timer.Stop()
		if got := timer.GetCount(); got != int64(i) {
			t.Fatalf("GetCount() = %d; want %d", got, i)
		}
		if got := timer.GetEarliestTimerStartTime(); got != timerStartTime {
			t.Errorf("GetEarliestTimerStartTime() changed: got %d, want %d", got, timerStartTime)
		}
		want := int64(i) * 42
		if got := timer.GetApproximateTiming(); got != want {
			t.Errorf("GetApproximateTiming() = %d; want %d (i=%d)", got, want, i)
			break
		}
	}
}
