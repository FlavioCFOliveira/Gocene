// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"testing"
	"time"
)

// TestRateLimiter tests the RateLimiter implementation.
// Ported from: org.apache.lucene.store.TestRateLimiter
func TestRateLimiter(t *testing.T) {
	t.Run("new rate limiter", func(t *testing.T) {
		rl := NewSimpleRateLimiter(10.0)
		if rl == nil {
			t.Fatal("NewSimpleRateLimiter() returned nil")
		}
		if rl.GetMBPerSec() != 10.0 {
			t.Errorf("GetMBPerSec() = %f, want 10.0", rl.GetMBPerSec())
		}
	})

	t.Run("set mb per sec", func(t *testing.T) {
		rl := NewSimpleRateLimiter(10.0)
		rl.SetMBPerSec(20.0)
		if rl.GetMBPerSec() != 20.0 {
			t.Errorf("GetMBPerSec() = %f, want 20.0", rl.GetMBPerSec())
		}
	})

	t.Run("pause with no limit", func(t *testing.T) {
		rl := NewSimpleRateLimiter(0.0)
		start := time.Now()
		paused := rl.Pause(1024 * 1024) // 1MB
		duration := time.Since(start)

		if paused != 0 {
			t.Errorf("Pause() = %d, want 0 for unlimited", paused)
		}
		if duration > time.Millisecond {
			t.Error("Pause() took too long for unlimited rate")
		}
	})

	t.Run("pause with limit", func(t *testing.T) {
		rl := NewSimpleRateLimiter(1.0) // 1 MB/sec
		start := time.Now()
		paused := rl.Pause(1024 * 1024) // 1MB - should take ~1 second
		duration := time.Since(start)

		// Should have paused approximately 1 second (with some tolerance)
		if paused < int64(900*time.Millisecond) {
			t.Errorf("Pause() = %d ns, expected at least 900ms", paused)
		}
		if duration < 900*time.Millisecond {
			t.Error("Duration too short for rate limiting")
		}
	})

	t.Run("pause small bytes", func(t *testing.T) {
		rl := NewSimpleRateLimiter(100.0) // 100 MB/sec
		start := time.Now()
		paused := rl.Pause(1024) // 1KB - should be very fast
		duration := time.Since(start)

		if paused > int64(100*time.Millisecond) {
			t.Errorf("Pause() = %d ns, expected small pause for small data", paused)
		}
		if duration > 100*time.Millisecond {
			t.Error("Duration too long for small data")
		}
	})

	t.Run("pause accumulates", func(t *testing.T) {
		rl := NewSimpleRateLimiter(10.0)

		// First pause
		rl.Pause(1024 * 1024)
		total1 := rl.GetTotalPauseNS()

		// Second pause
		rl.Pause(1024 * 1024)
		total2 := rl.GetTotalPauseNS()

		if total2 <= total1 {
			t.Error("Total pause should increase after second pause")
		}
	})

	t.Run("string representation", func(t *testing.T) {
		rl := NewSimpleRateLimiter(5.5)
		s := rl.String()
		if s == "" {
			t.Error("String() returned empty string")
		}
		// Should contain the rate
		t.Logf("RateLimiter string: %s", s)
	})
}
