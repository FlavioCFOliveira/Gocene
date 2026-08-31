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

const (
	minPauseCheckMsec = 25
	minPauseNs        = 2 * 1000 * 1000   // 2ms
	maxPauseNs        = 250 * 1000 * 1000 // 250ms
)

// MergeRateLimiter limits the rate of merge I/O.
// This is the Go port of org.apache.lucene.index.MergeRateLimiter.
type MergeRateLimiter struct {
	// mbPerSec is the target MB per second
	mbPerSec atomic.Uint64 // stored as math.Float64bits

	// minPauseCheckBytes is the minimum bytes between pause checks
	minPauseCheckBytes atomic.Int64

	// totalBytesWritten is the total bytes written
	totalBytesWritten atomic.Int64

	// lastNS is the last nano time for rate calculation
	lastNS atomic.Int64

	// mergeProgress tracks the progress of the merge and handles pausing
	mergeProgress *OneMergeProgress

	// mu protects configuration updates to ensure atomicity between mbPerSec and minPauseCheckBytes
	mu sync.Mutex
}

var _ store.RateLimiter = (*MergeRateLimiter)(nil)

// NewMergeRateLimiter creates a new MergeRateLimiter.
func NewMergeRateLimiter(mergeProgress *OneMergeProgress) *MergeRateLimiter {
	r := &MergeRateLimiter{
		mergeProgress: mergeProgress,
	}
	r.SetMBPerSec(math.Inf(1))
	return r
}

// SetMBPerSec sets the rate limit in MB/sec.
func (r *MergeRateLimiter) SetMBPerSec(mbPerSec float64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if mbPerSec < 0.0 {
		panic(fmt.Sprintf("mbPerSec must be positive; got: %f", mbPerSec))
	}

	r.mbPerSec.Store(math.Float64bits(mbPerSec))

	// minPauseCheckBytes = min(1MB, (minPauseCheckMsec / 1000.0) * mbPerSec * 1MB)
	checkBytes := (float64(minPauseCheckMsec) / 1000.0) * mbPerSec * 1024 * 1024
	minPauseCheckBytes := int64(checkBytes)
	if minPauseCheckBytes > 1024*1024 {
		minPauseCheckBytes = 1024 * 1024
	}
	r.minPauseCheckBytes.Store(minPauseCheckBytes)

	r.mergeProgress.Wakeup()
}

// GetMBPerSec returns the current rate limit in MB/sec.
func (r *MergeRateLimiter) GetMBPerSec() float64 {
	return math.Float64frombits(r.mbPerSec.Load())
}

// GetTotalBytesWritten returns the total bytes written during the merge.
func (r *MergeRateLimiter) GetTotalBytesWritten() int64 {
	return r.totalBytesWritten.Load()
}

// Pause pauses if necessary to ensure the IO rate does not exceed the limit.
// Returns the number of nanoseconds paused.
func (r *MergeRateLimiter) Pause(bytes int64) int64 {
	r.totalBytesWritten.Add(bytes)

	var paused int64
	for {
		delta, err := r.maybePause(bytes)
		if err != nil {
			// In Lucene, MergeAbortedException is thrown.
			// We return 0 and let the caller handle the abort via OneMergeProgress.
			return paused
		}
		if delta < 0 {
			break
		}
		paused += delta
	}

	return paused
}

// GetTotalStoppedNS returns the cumulative nanoseconds the merge was STOPPED.
func (r *MergeRateLimiter) GetTotalStoppedNS() int64 {
	return r.mergeProgress.GetPauseTimes()[STOPPED]
}

// GetTotalPausedNS returns the cumulative nanoseconds the merge was PAUSED.
func (r *MergeRateLimiter) GetTotalPausedNS() int64 {
	return r.mergeProgress.GetPauseTimes()[PAUSED]
}

func (r *MergeRateLimiter) maybePause(bytes int64) (int64, error) {
	if r.mergeProgress.IsAborted() {
		return 0, fmt.Errorf("merge aborted")
	}

	rate := r.GetMBPerSec()
	var secondsToPause float64
	if rate == 0 {
		secondsToPause = math.Inf(1)
	} else {
		secondsToPause = (float64(bytes) / 1024.0 / 1024.0) / rate
	}

	var curPauseNS int64

	// Atomic update of lastNS
	for {
		last := r.lastNS.Load()
		curNS := time.Now().UnixNano()
		targetNS := last + int64(1e9*secondsToPause)

		// Handle Infinity
		if math.IsInf(secondsToPause, 1) {
			targetNS = curNS + math.MaxInt64
		}

		diff := targetNS - curNS
		if diff <= minPauseNs {
			// Reset timeline to now
			if r.lastNS.CompareAndSwap(last, curNS) {
				curPauseNS = 0
				break
			}
		} else {
			// Keep timeline, will pause
			if r.lastNS.CompareAndSwap(last, last) { // No-op but matches logic
				curPauseNS = diff
				break
			}
			// Actually, in Java: return last.
			// So we don't change lastNS.
			curPauseNS = diff
			break
		}
	}

	if curPauseNS == 0 {
		return -1, nil
	}

	if curPauseNS > maxPauseNs {
		curPauseNS = maxPauseNs
	}

	start := time.Now().UnixNano()

	reason := PAUSED
	if rate == 0 {
		reason = STOPPED
	}

	err := r.mergeProgress.PauseNanos(curPauseNS, reason, func() bool {
		return r.GetMBPerSec() == rate
	})
	if err != nil {
		return 0, err
	}

	return time.Now().UnixNano() - start, nil
}

// GetMinPauseCheckBytes returns how many bytes a caller should accumulate before invoking Pause.
func (r *MergeRateLimiter) GetMinPauseCheckBytes() int64 {
	return r.minPauseCheckBytes.Load()
}
