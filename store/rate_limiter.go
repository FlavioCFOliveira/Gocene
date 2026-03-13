// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"fmt"
	"time"
)

// RateLimiter is used to limit the rate of IO operations.
//
// This is the Go port of Lucene's org.apache.lucene.store.RateLimiter.
//
// RateLimiter is used to throttle IO operations to a specified
// rate in MB/sec. This is useful for preventing IO operations
// from monopolizing system resources.
type RateLimiter interface {
	// GetMBPerSec returns the current rate limit in MB/sec.
	GetMBPerSec() float64

	// SetMBPerSec sets the rate limit in MB/sec.
	SetMBPerSec(mbPerSec float64)

	// Pause pauses if necessary to ensure the IO rate does not exceed the limit.
	// Returns the number of nanoseconds paused.
	Pause(bytes int64) int64
}

// SimpleRateLimiter is a basic implementation of RateLimiter.
type SimpleRateLimiter struct {
	mbPerSec float64
	minPause time.Duration
	lastNS   int64
	totalNS  int64
}

// NewSimpleRateLimiter creates a new SimpleRateLimiter.
// mbPerSec is the initial rate limit in MB/sec. Use 0.0 for no limit.
func NewSimpleRateLimiter(mbPerSec float64) *SimpleRateLimiter {
	return &SimpleRateLimiter{
		mbPerSec: mbPerSec,
		minPause: time.Millisecond,
		lastNS:   time.Now().UnixNano(),
	}
}

// GetMBPerSec returns the current rate limit.
func (r *SimpleRateLimiter) GetMBPerSec() float64 {
	return r.mbPerSec
}

// SetMBPerSec sets the rate limit.
func (r *SimpleRateLimiter) SetMBPerSec(mbPerSec float64) {
	r.mbPerSec = mbPerSec
}

// Pause pauses to maintain the rate limit.
// Returns the number of nanoseconds paused.
func (r *SimpleRateLimiter) Pause(bytes int64) int64 {
	if r.mbPerSec <= 0 {
		return 0
	}

	now := time.Now().UnixNano()
	elapsedNS := now - r.lastNS
	if elapsedNS < 0 {
		// Time went backwards - adjust
		elapsedNS = 0
	}

	// Calculate expected time for this many bytes
	bytesPerSec := r.mbPerSec * 1024 * 1024
	expectedNS := int64(float64(bytes) / bytesPerSec * 1e9)

	// Calculate pause needed
	pauseNS := expectedNS - elapsedNS
	if pauseNS < int64(r.minPause) {
		pauseNS = 0
	}

	if pauseNS > 0 {
		time.Sleep(time.Duration(pauseNS))
		r.lastNS = now + pauseNS
		r.totalNS += pauseNS
	} else {
		r.lastNS = now
	}

	return pauseNS
}

// GetTotalPauseNS returns the total nanoseconds paused.
func (r *SimpleRateLimiter) GetTotalPauseNS() int64 {
	return r.totalNS
}

// String returns a string representation of this RateLimiter.
func (r *SimpleRateLimiter) String() string {
	return fmt.Sprintf("SimpleRateLimiter(mbPerSec=%.2f)", r.mbPerSec)
}
