// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"fmt"
	"sync"
	"time"
)

// RateLimiter is used to limit the rate of IO operations.
//
// This is the Go port of org.apache.lucene.store.RateLimiter from Apache
// Lucene 10.4.0. Implementations are typically shared across multiple
// IndexInputs or IndexOutputs (for example all the inputs involved in a
// merge). Those inputs/outputs should call Pause whenever they have read or
// written more than GetMinPauseCheckBytes bytes.
type RateLimiter interface {
	// GetMBPerSec returns the current rate limit in MB/sec.
	GetMBPerSec() float64

	// SetMBPerSec sets the rate limit in MB/sec.
	SetMBPerSec(mbPerSec float64)

	// Pause pauses if necessary to ensure the IO rate does not exceed the
	// limit. Returns the number of nanoseconds paused.
	Pause(bytes int64) int64

	// GetMinPauseCheckBytes returns how many bytes a caller should accumulate
	// before invoking Pause. The value may change over time; callers should
	// refresh their local copy whenever they cross the previous threshold.
	GetMinPauseCheckBytes() int64
}

// simpleRateLimiterMinPauseCheckMS is the minimum number of milliseconds
// represented by GetMinPauseCheckBytes, matching Lucene's
// MIN_PAUSE_CHECK_MSEC constant.
const simpleRateLimiterMinPauseCheckMS = 5

// SimpleRateLimiter is a basic, thread-safe implementation of RateLimiter
// matching Lucene's SimpleRateLimiter inner class.
type SimpleRateLimiter struct {
	mu sync.Mutex

	mbPerSec           float64
	minPauseCheckBytes int64
	minPause           time.Duration
	lastNS             int64
	totalNS            int64
}

// NewSimpleRateLimiter creates a new SimpleRateLimiter. mbPerSec is the
// initial rate limit; passing 0.0 disables the limit.
func NewSimpleRateLimiter(mbPerSec float64) *SimpleRateLimiter {
	r := &SimpleRateLimiter{
		minPause: time.Millisecond,
		lastNS:   time.Now().UnixNano(),
	}
	r.SetMBPerSec(mbPerSec)
	return r
}

// GetMBPerSec returns the current rate limit.
func (r *SimpleRateLimiter) GetMBPerSec() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.mbPerSec
}

// SetMBPerSec sets the rate limit and recomputes the pause-check threshold.
func (r *SimpleRateLimiter) SetMBPerSec(mbPerSec float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.mbPerSec = mbPerSec
	r.minPauseCheckBytes = int64((float64(simpleRateLimiterMinPauseCheckMS) / 1000.0) * mbPerSec * 1024 * 1024)
}

// GetMinPauseCheckBytes returns the current pause-check threshold in bytes,
// matching Lucene's getMinPauseCheckBytes.
func (r *SimpleRateLimiter) GetMinPauseCheckBytes() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.minPauseCheckBytes
}

// Pause pauses, if necessary, to keep the instantaneous IO rate at or below
// the target. Returns the number of nanoseconds paused. Thread-safe.
//
// Per Lucene's contract the caller should only invoke Pause when their
// accumulated bytes exceed GetMinPauseCheckBytes; otherwise the sleep is
// likely to be much longer than expected.
func (r *SimpleRateLimiter) Pause(bytes int64) int64 {
	startNS := time.Now().UnixNano()

	r.mu.Lock()
	if r.mbPerSec <= 0 {
		r.lastNS = startNS
		r.mu.Unlock()
		return 0
	}

	secondsToPause := (float64(bytes) / 1024.0 / 1024.0) / r.mbPerSec
	targetNS := r.lastNS + int64(1e9*secondsToPause)
	if startNS >= targetNS {
		// We are already past the target; enforce instantaneous rate, not
		// averaged-over-history rate.
		r.lastNS = startNS
		r.mu.Unlock()
		return 0
	}
	r.lastNS = targetNS
	r.mu.Unlock()

	// Sleep loop because time.Sleep may return early. We reload curNS each
	// iteration to handle short sleeps.
	curNS := startNS
	for {
		pauseNS := targetNS - curNS
		if pauseNS <= 0 {
			break
		}
		time.Sleep(time.Duration(pauseNS))
		curNS = time.Now().UnixNano()
	}

	delta := curNS - startNS

	r.mu.Lock()
	r.totalNS += delta
	r.mu.Unlock()

	return delta
}

// GetTotalPauseNS returns the cumulative nanoseconds paused across all calls.
func (r *SimpleRateLimiter) GetTotalPauseNS() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.totalNS
}

// String returns a string representation of this RateLimiter.
func (r *SimpleRateLimiter) String() string {
	return fmt.Sprintf("SimpleRateLimiter(mbPerSec=%.2f)", r.mbPerSec)
}
