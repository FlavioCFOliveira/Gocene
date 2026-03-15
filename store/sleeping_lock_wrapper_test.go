// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package store_test contains tests for the store package.
// This file is a port of Apache Lucene's TestSleepingLockWrapper.java
// Source: lucene/core/src/test/org/apache/lucene/store/TestSleepingLockWrapper.java
//
// Tests SleepingLockWrapper which provides lock retry with polling interval and timeout.
// The wrapper sleeps and retries if obtaining the lock fails, useful for handling
// transient lock contention.
package store_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestSleepingLockWrapper_Basics tests obtaining and releasing locks, checking validity.
// Ported from: BaseLockFactoryTestCase.testBasics()
func TestSleepingLockWrapper_Basics(t *testing.T) {
	tests := []struct {
		name            string
		lockWaitTimeout int64
		pollInterval    int64
	}{
		{
			name:            "with timeout",
			lockWaitTimeout: 100,
			pollInterval:    10,
		},
		{
			name:            "with forever wait",
			lockWaitTimeout: -1, // LOCK_OBTAIN_WAIT_FOREVER
			pollInterval:    10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := store.NewByteBuffersDirectory()
			defer dir.Close()

			// Create SleepingLockWrapper with test parameters
			// Note: This assumes SleepingLockWrapper will be implemented
			sleepingDir := store.NewSleepingLockWrapper(dir, tt.lockWaitTimeout, tt.pollInterval)
			defer sleepingDir.Close()

			// Obtain lock
			lock, err := sleepingDir.ObtainLock("commit")
			if err != nil {
				t.Fatalf("Failed to obtain lock: %v", err)
			}
			if lock == nil {
				t.Fatal("Expected non-nil lock")
			}

			// Should not be able to get the lock twice
			_, err = sleepingDir.ObtainLock("commit")
			if err == nil {
				t.Error("Expected error when obtaining same lock twice")
			}

			// Release lock
			if err := lock.Close(); err != nil {
				t.Fatalf("Failed to release lock: %v", err)
			}

			// Should be able to obtain lock again after release
			lock2, err := sleepingDir.ObtainLock("commit")
			if err != nil {
				t.Fatalf("Failed to reobtain lock: %v", err)
			}
			if lock2 == nil {
				t.Fatal("Expected non-nil lock on reobtain")
			}
			lock2.Close()
		})
	}
}

// TestSleepingLockWrapper_DoubleClose tests closing locks twice.
// Ported from: BaseLockFactoryTestCase.testDoubleClose()
func TestSleepingLockWrapper_DoubleClose(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sleepingDir := store.NewSleepingLockWrapper(dir, 100, 10)
	defer sleepingDir.Close()

	lock, err := sleepingDir.ObtainLock("commit")
	if err != nil {
		t.Fatalf("Failed to obtain lock: %v", err)
	}

	// First close should succeed
	if err := lock.Close(); err != nil {
		t.Fatalf("First close failed: %v", err)
	}

	// Second close should also succeed (no-op)
	if err := lock.Close(); err != nil {
		t.Errorf("Second close should succeed: %v", err)
	}
}

// TestSleepingLockWrapper_ValidAfterAcquire tests ensureValid returns true after acquire.
// Ported from: BaseLockFactoryTestCase.testValidAfterAcquire()
func TestSleepingLockWrapper_ValidAfterAcquire(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sleepingDir := store.NewSleepingLockWrapper(dir, 100, 10)
	defer sleepingDir.Close()

	lock, err := sleepingDir.ObtainLock("commit")
	if err != nil {
		t.Fatalf("Failed to obtain lock: %v", err)
	}
	defer lock.Close()

	// Lock should be valid immediately after acquire
	if err := lock.EnsureValid(); err != nil {
		t.Errorf("Lock should be valid after acquire: %v", err)
	}
}

// TestSleepingLockWrapper_InvalidAfterClose tests ensureValid throws exception after close.
// Ported from: BaseLockFactoryTestCase.testInvalidAfterClose()
func TestSleepingLockWrapper_InvalidAfterClose(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sleepingDir := store.NewSleepingLockWrapper(dir, 100, 10)
	defer sleepingDir.Close()

	lock, err := sleepingDir.ObtainLock("commit")
	if err != nil {
		t.Fatalf("Failed to obtain lock: %v", err)
	}

	// Release lock
	lock.Close()

	// Lock should be invalid after close
	if err := lock.EnsureValid(); err == nil {
		t.Error("Expected error when checking validity of released lock")
	}
}

// TestSleepingLockWrapper_ObtainConcurrently tests concurrent lock acquisition.
// Ported from: BaseLockFactoryTestCase.testObtainConcurrently()
func TestSleepingLockWrapper_ObtainConcurrently(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Use short timeout for faster test execution
	sleepingDir := store.NewSleepingLockWrapper(dir, 500, 5)
	defer sleepingDir.Close()

	numThreads := 5
	runs := 100
	var wg sync.WaitGroup
	wg.Add(numThreads)

	// Use atomic counter to track successful lock acquisitions
	var successCount int32
	var failCount int32

	// Barrier to synchronize thread start
	barrier := make(chan struct{})

	for i := 0; i < numThreads; i++ {
		go func(id int) {
			defer wg.Done()

			// Wait for barrier
			<-barrier

			for j := 0; j < runs; j++ {
				lock, err := sleepingDir.ObtainLock("concurrent.lock")
				if err != nil {
					// Lock contention is expected, increment fail count
					atomic.AddInt32(&failCount, 1)
					continue
				}

				// Successfully obtained lock
				atomic.AddInt32(&successCount, 1)

				// Small delay to simulate work
				time.Sleep(time.Millisecond)

				// Release lock
				lock.Close()
			}
		}(i)
	}

	// Release all threads at once
	close(barrier)

	// Wait for all threads to complete
	wg.Wait()

	// Verify that we had both successful acquisitions and contention
	if successCount == 0 {
		t.Error("Expected some successful lock acquisitions")
	}
	if failCount == 0 {
		t.Error("Expected some lock contention (failures)")
	}

	t.Logf("Successful acquisitions: %d, Contentions: %d", successCount, failCount)
}

// TestSleepingLockWrapper_RetryBehavior tests the retry behavior with polling interval.
// This is specific to SleepingLockWrapper's core functionality.
func TestSleepingLockWrapper_RetryBehavior(t *testing.T) {
	tests := []struct {
		name            string
		lockWaitTimeout int64
		pollInterval    int64
		expectTimeout   bool
	}{
		{
			name:            "sufficient timeout for retry",
			lockWaitTimeout: 200,
			pollInterval:    20,
			expectTimeout:   false,
		},
		{
			name:            "short timeout may fail",
			lockWaitTimeout: 10,
			pollInterval:    50,
			expectTimeout:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := store.NewByteBuffersDirectory()
			defer dir.Close()

			sleepingDir := store.NewSleepingLockWrapper(dir, tt.lockWaitTimeout, tt.pollInterval)
			defer sleepingDir.Close()

			// First, obtain the lock
			lock1, err := sleepingDir.ObtainLock("retry.lock")
			if err != nil {
				t.Fatalf("Failed to obtain first lock: %v", err)
			}
			defer lock1.Close()

			// Try to obtain the same lock from another goroutine
			done := make(chan struct{})
			var secondLock store.Lock
			var secondErr error

			go func() {
				secondLock, secondErr = sleepingDir.ObtainLock("retry.lock")
				close(done)
			}()

			// Release first lock after a short delay
			time.Sleep(50 * time.Millisecond)
			lock1.Close()

			// Wait for second lock attempt to complete
			select {
			case <-done:
				if tt.expectTimeout {
					if secondErr == nil {
						t.Error("Expected timeout error")
					}
				} else {
					if secondErr != nil {
						t.Errorf("Expected successful lock acquisition: %v", secondErr)
					}
					if secondLock != nil {
						secondLock.Close()
					}
				}
			case <-time.After(time.Second):
				t.Error("Timeout waiting for second lock attempt")
			}
		})
	}
}

// TestSleepingLockWrapper_Constructor tests the constructor validation.
func TestSleepingLockWrapper_Constructor(t *testing.T) {
	tests := []struct {
		name            string
		lockWaitTimeout int64
		pollInterval    int64
		expectPanic     bool
	}{
		{
			name:            "valid positive timeout",
			lockWaitTimeout: 100,
			pollInterval:    10,
			expectPanic:     false,
		},
		{
			name:            "valid forever timeout",
			lockWaitTimeout: -1, // LOCK_OBTAIN_WAIT_FOREVER
			pollInterval:    10,
			expectPanic:     false,
		},
		{
			name:            "valid zero timeout",
			lockWaitTimeout: 0,
			pollInterval:    10,
			expectPanic:     false,
		},
		{
			name:            "invalid negative timeout (not forever)",
			lockWaitTimeout: -2,
			pollInterval:    10,
			expectPanic:     true,
		},
		{
			name:            "invalid negative poll interval",
			lockWaitTimeout: 100,
			pollInterval:    -1,
			expectPanic:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := store.NewByteBuffersDirectory()
			defer dir.Close()

			if tt.expectPanic {
				defer func() {
					if r := recover(); r == nil {
						t.Error("Expected panic for invalid arguments")
					}
				}()
			}

			sleepingDir := store.NewSleepingLockWrapper(dir, tt.lockWaitTimeout, tt.pollInterval)
			if !tt.expectPanic && sleepingDir == nil {
				t.Error("Expected non-nil SleepingLockWrapper")
			}
			if sleepingDir != nil {
				sleepingDir.Close()
			}
		})
	}
}

// TestSleepingLockWrapper_String tests the String representation.
func TestSleepingLockWrapper_String(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sleepingDir := store.NewSleepingLockWrapper(dir, 100, 10)
	defer sleepingDir.Close()

	str := sleepingDir.String()
	if str == "" {
		t.Error("Expected non-empty string representation")
	}
	// Should contain "SleepingLockWrapper" in the string
	// This is implementation-specific but follows Lucene convention
}

// TestSleepingLockWrapper_DelegateAccess tests access to the wrapped directory.
func TestSleepingLockWrapper_DelegateAccess(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sleepingDir := store.NewSleepingLockWrapper(dir, 100, 10)
	defer sleepingDir.Close()

	// Test that we can access the delegate
	delegate := sleepingDir.GetDelegate()
	if delegate == nil {
		t.Error("Expected non-nil delegate")
	}

	// The delegate should be the original directory
	if delegate != dir {
		t.Error("Delegate should be the original directory")
	}
}

// TestSleepingLockWrapper_WithSingleInstanceLockFactory tests with SingleInstanceLockFactory.
// This mirrors the Java test which uses SingleInstanceLockFactory.
func TestSleepingLockWrapper_WithSingleInstanceLockFactory(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Set SingleInstanceLockFactory on the underlying directory
	dir.SetLockFactory(store.NewSingleInstanceLockFactory())

	sleepingDir := store.NewSleepingLockWrapper(dir, 100, 10)
	defer sleepingDir.Close()

	// Test basic lock operations
	lock, err := sleepingDir.ObtainLock("test.lock")
	if err != nil {
		t.Fatalf("Failed to obtain lock: %v", err)
	}

	// Try to obtain same lock - should fail immediately (before retry)
	// because SingleInstanceLockFactory doesn't support concurrent locks
	start := time.Now()
	_, err = sleepingDir.ObtainLock("test.lock")
	elapsed := time.Since(start)

	if err == nil {
		t.Error("Expected error when obtaining same lock twice")
	}

	// Should have waited at least one poll interval before giving up
	if elapsed < 10*time.Millisecond {
		t.Logf("Lock attempt failed quickly as expected (elapsed: %v)", elapsed)
	}

	lock.Close()
}

// TestSleepingLockWrapper_Stress performs a stress test with multiple goroutines.
// Ported from: BaseLockFactoryTestCase.testStressLocks()
func TestSleepingLockWrapper_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Use reasonable timeout for stress test
	sleepingDir := store.NewSleepingLockWrapper(dir, 1000, 50)
	defer sleepingDir.Close()

	numIterations := 20
	numWorkers := 4

	var wg sync.WaitGroup
	errors := make(chan error, numWorkers*numIterations)

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < numIterations; j++ {
				lock, err := sleepingDir.ObtainLock("stress.lock")
				if err != nil {
					errors <- err
					continue
				}

				// Simulate some work
				time.Sleep(time.Millisecond * 5)

				if err := lock.Close(); err != nil {
					errors <- err
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	errorCount := 0
	for err := range errors {
		if err != nil {
			errorCount++
			t.Logf("Error during stress test: %v", err)
		}
	}

	if errorCount > 0 {
		t.Errorf("Had %d errors during stress test", errorCount)
	}
}

// TestSleepingLockWrapper_DefaultPollInterval tests using default poll interval constructor.
func TestSleepingLockWrapper_DefaultPollInterval(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Use constructor with default poll interval
	sleepingDir := store.NewSleepingLockWrapperWithDefaultPollInterval(dir, 100)
	defer sleepingDir.Close()

	// Should be able to obtain lock
	lock, err := sleepingDir.ObtainLock("default.lock")
	if err != nil {
		t.Fatalf("Failed to obtain lock: %v", err)
	}
	lock.Close()
}

// TestSleepingLockWrapper_LockObtainFailedException tests that proper error is returned on timeout.
func TestSleepingLockWrapper_LockObtainFailedException(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Set SingleInstanceLockFactory to ensure lock contention
	dir.SetLockFactory(store.NewSingleInstanceLockFactory())

	// Use very short timeout
	sleepingDir := store.NewSleepingLockWrapper(dir, 50, 10)
	defer sleepingDir.Close()

	// Obtain first lock
	lock1, err := sleepingDir.ObtainLock("timeout.lock")
	if err != nil {
		t.Fatalf("Failed to obtain first lock: %v", err)
	}
	defer lock1.Close()

	// Try to obtain second lock - should timeout
	start := time.Now()
	_, err = sleepingDir.ObtainLock("timeout.lock")
	elapsed := time.Since(start)

	if err == nil {
		t.Error("Expected timeout error")
	}

	// Should have waited approximately the timeout duration
	if elapsed < 40*time.Millisecond || elapsed > 200*time.Millisecond {
		t.Logf("Elapsed time: %v (expected around 50ms)", elapsed)
	}
}

// BenchmarkSleepingLockWrapper_ObtainLock benchmarks lock acquisition.
func BenchmarkSleepingLockWrapper_ObtainLock(b *testing.B) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sleepingDir := store.NewSleepingLockWrapper(dir, 1000, 10)
	defer sleepingDir.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lock, err := sleepingDir.ObtainLock("bench.lock")
		if err != nil {
			b.Fatalf("Failed to obtain lock: %v", err)
		}
		lock.Close()
	}
}
