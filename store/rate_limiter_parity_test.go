// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Source: lucene/core/src/test/org/apache/lucene/store/TestRateLimiter.java
// Extends the legacy rate_limiter_test.go with cases for
// GetMinPauseCheckBytes parity introduced in Sprint 11.

package store

import "testing"

func minPauseExpected(mbPerSec float64) int64 {
	return int64((float64(5) / 1000.0) * mbPerSec * 1024 * 1024)
}

func TestRateLimiter_GetMinPauseCheckBytes(t *testing.T) {
	r := NewSimpleRateLimiter(10.0)
	if got, want := r.GetMinPauseCheckBytes(), minPauseExpected(10.0); got != want {
		t.Fatalf("GetMinPauseCheckBytes(mbPerSec=10) = %d, want %d", got, want)
	}
	r.SetMBPerSec(100.0)
	if got, want := r.GetMinPauseCheckBytes(), minPauseExpected(100.0); got != want {
		t.Fatalf("GetMinPauseCheckBytes(mbPerSec=100) = %d, want %d", got, want)
	}
}

func TestRateLimiter_GetMinPauseCheckBytes_ZeroRate(t *testing.T) {
	r := NewSimpleRateLimiter(0.0)
	if got := r.GetMinPauseCheckBytes(); got != 0 {
		t.Fatalf("GetMinPauseCheckBytes(mbPerSec=0) = %d, want 0", got)
	}
}
