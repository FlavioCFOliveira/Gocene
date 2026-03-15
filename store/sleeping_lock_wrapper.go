// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"fmt"
	"time"
)

// SleepingLockWrapper is a Directory that wraps another, and that sleeps and retries
// if obtaining the lock fails.
//
// This is the Go port of Lucene's org.apache.lucene.store.SleepingLockWrapper.
//
// Note: This is not a good idea for production use, but is provided for compatibility
// with Lucene's implementation.
type SleepingLockWrapper struct {
	*FilterDirectory
	lockWaitTimeout int64
	pollInterval    int64
}

// LOCK_OBTAIN_WAIT_FOREVER is a special value for lockWaitTimeout that means
// to retry forever to obtain the lock.
const LOCK_OBTAIN_WAIT_FOREVER int64 = -1

// DEFAULT_POLL_INTERVAL is the default poll interval in milliseconds.
const DEFAULT_POLL_INTERVAL int64 = 1000

// NewSleepingLockWrapper creates a new SleepingLockWrapper with the specified timeout and poll interval.
//
// Parameters:
//   - delegate: underlying directory to wrap
//   - lockWaitTimeout: length of time to wait in milliseconds, or LOCK_OBTAIN_WAIT_FOREVER to retry forever
//   - pollInterval: poll once per this interval in milliseconds until lockWaitTimeout is exceeded
//
// Panics if lockWaitTimeout is negative (but not LOCK_OBTAIN_WAIT_FOREVER) or if pollInterval is negative.
func NewSleepingLockWrapper(delegate Directory, lockWaitTimeout, pollInterval int64) *SleepingLockWrapper {
	if lockWaitTimeout < 0 && lockWaitTimeout != LOCK_OBTAIN_WAIT_FOREVER {
		panic(fmt.Sprintf("lockWaitTimeout should be LOCK_OBTAIN_WAIT_FOREVER or a non-negative number (got %d)", lockWaitTimeout))
	}
	if pollInterval < 0 {
		panic(fmt.Sprintf("pollInterval must be a non-negative number (got %d)", pollInterval))
	}

	return &SleepingLockWrapper{
		FilterDirectory: NewFilterDirectory(delegate),
		lockWaitTimeout: lockWaitTimeout,
		pollInterval:    pollInterval,
	}
}

// NewSleepingLockWrapperWithDefaultPollInterval creates a new SleepingLockWrapper with the default poll interval.
//
// Parameters:
//   - delegate: underlying directory to wrap
//   - lockWaitTimeout: length of time to wait in milliseconds, or LOCK_OBTAIN_WAIT_FOREVER to retry forever
func NewSleepingLockWrapperWithDefaultPollInterval(delegate Directory, lockWaitTimeout int64) *SleepingLockWrapper {
	return NewSleepingLockWrapper(delegate, lockWaitTimeout, DEFAULT_POLL_INTERVAL)
}

// ObtainLock attempts to obtain a lock with retry logic.
// It will sleep for pollInterval milliseconds between attempts until either:
//   - The lock is successfully obtained
//   - The lockWaitTimeout is exceeded (unless it's LOCK_OBTAIN_WAIT_FOREVER)
//
// If the timeout is exceeded, returns an error wrapping the original failure reason.
func (s *SleepingLockWrapper) ObtainLock(name string) (Lock, error) {
	var failureReason error
	var maxSleepCount int64
	if s.lockWaitTimeout != LOCK_OBTAIN_WAIT_FOREVER {
		maxSleepCount = s.lockWaitTimeout / s.pollInterval
	}
	var sleepCount int64

	for {
		lock, err := s.FilterDirectory.ObtainLock(name)
		if err == nil {
			return lock, nil
		}

		// Store the first failure reason
		if failureReason == nil {
			failureReason = err
		}

		// Sleep before retry
		time.Sleep(time.Duration(s.pollInterval) * time.Millisecond)

		// Check if we've exceeded the timeout
		if s.lockWaitTimeout != LOCK_OBTAIN_WAIT_FOREVER {
			sleepCount++
			if sleepCount >= maxSleepCount {
				return nil, fmt.Errorf("Lock obtain timed out: %s: %w", s.String(), failureReason)
			}
		}
	}
}

// String returns a string representation of this SleepingLockWrapper.
func (s *SleepingLockWrapper) String() string {
	return fmt.Sprintf("SleepingLockWrapper(%v)", s.GetDelegate())
}
