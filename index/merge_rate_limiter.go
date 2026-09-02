//go:build ignore

// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/FlavioCFOliveira/Gocene/store"
)

var _ store.RateLimiter = (*MergeRateLimiter)(nil)

const (
	minPauseCheckMsec = 25
	minPauseNS        = 2 * time.Millisecond
	maxPauseNS        = 250 * time.Millisecond
)

// MergeRateLimiter is the RateLimiter that IndexWriter assigns to each running merge,
// to give MergeSchedulers ionice like control.
//
// This is the Go port of org.apache.lucene.index.MergeRateLimiter.
type MergeRateLimiter struct {
	mu sync.Mutex

	mbPerSec           float64
	minPauseCheckBytes int64

	lastNS atomic.Int64

	totalBytesWritten atomic.Int64

	mergeProgress *OneMergeProgress
}

// NewMergeRateLimiter creates a new MergeRateLimiter.
func NewMergeRateLimiter(mergeProgress *OneMergeProgress) *MergeRateLimiter {
	r := &MergeRateLimiter{
		mergeProgress: mergeProgress,
	}
	// Initially no IO limit
	r.SetMBPerSec(math.Inf(1))
	return r
}

// SetMBPerSec sets the rate limit in MB/sec.
func (r *MergeRateLimiter) SetMBPerSec(mbPerSec float64) {
	if mbPerSec < 0.0 {
		panic(fmt.Sprintf("mbPerSec must be positive; got: %f", mbPerSec))
	}

	r.mu.Lock()
	r.mbPerSec = mbPerSec
	// NOTE: math.Inf(1) casts to MaxInt64 in the calculation
	r.minPauseCheckBytes = int64(math.Min(1024*1024, (float64(minPauseCheckMsec)/1000.0)*mbPerSec*1024*1024))
	r.mu.Unlock()

	r.mergeProgress.Wakeup()
}

// GetMBPerSec returns the current rate limit in MB/sec.
func (r *MergeRateLimiter) GetMBPerSec() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.mbPerSec
}

// GetTotalBytesWritten returns total bytes written by this merge.
func (r *MergeRateLimiter) GetTotalBytesWritten() int64 {
	return r.totalBytesWritten.Load()
}

// Pause pauses if necessary to ensure the IO rate does not exceed the limit.
// Returns the number of nanoseconds paused.
func (r *MergeRateLimiter) Pause(bytes int64) int64 {
	r.totalBytesWritten.Add(bytes)

	var paused int64
	for {
		delta := r.maybePause(bytes)
		if delta < 0 {
			break
		}
		paused += delta
	}

	return paused
}

// GetTotalStoppedNS returns total NS merge was stopped.
func (r *MergeRateLimiter) GetTotalStoppedNS() int64 {
	return r.mergeProgress.GetPauseTimes()[STOPPED]
}

// GetTotalPausedNS returns total NS merge was paused to rate limit IO.
func (r *MergeRateLimiter) GetTotalPausedNS() int64 {
	return r.mergeProgress.GetPauseTimes()[PAUSED]
}

// GetMinPauseCheckBytes returns how many bytes a caller should accumulate before invoking Pause.
func (r *MergeRateLimiter) GetMinPauseCheckBytes() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.minPauseCheckBytes
}

func (r *MergeRateLimiter) maybePause(bytes int64) int64 {
	if r.mergeProgress.IsAborted() {
		// In Go, we handle the abort by returning a specific value or error.
		// Lucene throws MergeAbortedException.
		// Since Pause return int64, and maybePause is internal,
		// we need to signal the abort to the caller.
		// We'll use a sentinel or just panic if we want to match the exception,
		// but the standard Go way is returning error.
		// However, the RateLimiter interface does not return error.
		// Looking at OneMergeProgress.PauseNanos, it returns error.
		// Let's assume we'll handle abort via the loop check or a panic for simplicity
		// if it's a critical failure, but better to use a sentinel if the interface allows.
		// Actually, the Java code throws MergeAbortedException which is a checked exception.
		// In Go, if we can't change the interface, we might have to panic or
		// return a sentinel. But Pause is part of RateLimiter interface.
		// Wait, MergeRateLimiter's Pause method in Java throws MergeAbortedException.
		// Our store.RateLimiter interface DOES NOT return an error.
		// This is a discrepancy.
		panic(fmt.Errorf("merge aborted"))
	}

	r.mu.Lock()
	rate := r.mbPerSec
	r.mu.Unlock()

	secondsToPause := (float64(bytes) / 1024.0 / 1024.0) / rate

	var curPauseNS int64

	// Implement AtomicLong.updateAndGet logic
	for {
		last := r.lastNS.Load()
		curNS := time.Now().UnixNano()
		targetNS := last + int64(1e9*secondsToPause)
		diff := targetNS - curNS

		if diff <= minPauseNS.Nanoseconds() {
			curPauseNS = 0
			if r.lastNS.CompareAndSwap(last, curNS) {
				break
			}
		} else {
			curPauseNS = diff
			if r.lastNS.CompareAndSwap(last, last) { // No-op to just break loop if we don't update
				break
			}
		}
	}

	if curPauseNS == 0 {
		return -1
	}

	pauseAmount := curPauseNS
	if pauseAmount > maxPauseNS.Nanoseconds() {
		pauseAmount = maxPauseNS.Nanoseconds()
	}

	start := time.Now().UnixNano()

	reason := PAUSED
	if rate == 0.0 {
		reason = STOPPED
	}

	err := r.mergeProgress.PauseNanos(pauseAmount, reason, func() bool {
		r.mu.Lock()
		defer r.mu.Unlock()
		return rate == r.mbPerSec
	})

	if err != nil {
		// Handle abort/interruption
		panic(err)
	}

	return time.Now().UnixNano() - start
}
