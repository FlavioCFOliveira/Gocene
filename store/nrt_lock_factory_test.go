package store

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestNewNRTLockFactory(t *testing.T) {
	factory := NewNRTLockFactory()
	if factory == nil {
		t.Fatal("NewNRTLockFactory() returned nil")
	}

	// Check default values
	if factory.GetDefaultTimeout() != 30*time.Second {
		t.Errorf("GetDefaultTimeout() = %v, want 30s", factory.GetDefaultTimeout())
	}

	if factory.GetDefaultRetryDelay() != 10*time.Millisecond {
		t.Errorf("GetDefaultRetryDelay() = %v, want 10ms", factory.GetDefaultRetryDelay())
	}
}

func TestNRTLockType_String(t *testing.T) {
	tests := []struct {
		lockType NRTLockType
		want     string
	}{
		{NRTLockTypeRead, "READ"},
		{NRTLockTypeWrite, "WRITE"},
		{NRTLockTypeCommit, "COMMIT"},
		{NRTLockType(999), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.lockType.String()
			if got != tt.want {
				t.Errorf("NRTLockType.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNRTLockFactory_ObtainLock(t *testing.T) {
	factory := NewNRTLockFactory()
	dir := NewByteBuffersDirectory()

	// Test obtaining a lock
	lock, err := factory.ObtainLock(dir, "test-lock")
	if err != nil {
		t.Fatalf("ObtainLock() error = %v", err)
	}
	if lock == nil {
		t.Fatal("ObtainLock() returned nil lock")
	}

	// Verify lock is held
	if !lock.IsLocked() {
		t.Error("lock.IsLocked() = false, want true")
	}

	// Test EnsureValid
	if err := lock.EnsureValid(); err != nil {
		t.Errorf("EnsureValid() error = %v", err)
	}

	// Release lock
	if err := lock.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	if lock.IsLocked() {
		t.Error("lock.IsLocked() = true after Close, want false")
	}

	// EnsureValid should fail after release
	if err := lock.EnsureValid(); err == nil {
		t.Error("EnsureValid() expected error after release")
	}
}

func TestNRTLockFactory_ObtainLock_EmptyName(t *testing.T) {
	factory := NewNRTLockFactory()
	dir := NewByteBuffersDirectory()

	_, err := factory.ObtainLock(dir, "")
	if err == nil {
		t.Error("ObtainLock() with empty name expected error")
	}
}

func TestNRTLockFactory_MultipleReadLocks(t *testing.T) {
	factory := NewNRTLockFactory()

	// Obtain multiple read locks
	lock1, err := factory.ObtainReadLock("test-lock")
	if err != nil {
		t.Fatalf("First ObtainReadLock() error = %v", err)
	}

	lock2, err := factory.ObtainReadLock("test-lock")
	if err != nil {
		t.Fatalf("Second ObtainReadLock() error = %v", err)
	}

	// Both should be held
	if !lock1.IsLocked() {
		t.Error("lock1 should be held")
	}
	if !lock2.IsLocked() {
		t.Error("lock2 should be held")
	}

	// Release locks
	lock1.Close()
	lock2.Close()
}

func TestNRTLockFactory_ReadBlocksWrite(t *testing.T) {
	factory := NewNRTLockFactory()
	factory.SetDefaultTimeout(100 * time.Millisecond)

	// Obtain read lock
	readLock, err := factory.ObtainReadLock("test-lock")
	if err != nil {
		t.Fatalf("ObtainReadLock() error = %v", err)
	}
	defer readLock.Close()

	// Try to obtain write lock - should timeout
	_, err = factory.ObtainWriteLock("test-lock")
	if err == nil {
		t.Error("ObtainWriteLock() expected timeout error")
	}
}

func TestNRTLockFactory_WriteBlocksRead(t *testing.T) {
	factory := NewNRTLockFactory()
	factory.SetDefaultTimeout(100 * time.Millisecond)

	// Obtain write lock
	writeLock, err := factory.ObtainWriteLock("test-lock")
	if err != nil {
		t.Fatalf("ObtainWriteLock() error = %v", err)
	}
	defer writeLock.Close()

	// Try to obtain read lock - should timeout
	_, err = factory.ObtainReadLock("test-lock")
	if err == nil {
		t.Error("ObtainReadLock() expected timeout error")
	}
}

func TestNRTLockFactory_WriteBlocksWrite(t *testing.T) {
	factory := NewNRTLockFactory()
	factory.SetDefaultTimeout(100 * time.Millisecond)

	// Obtain write lock
	writeLock1, err := factory.ObtainWriteLock("test-lock")
	if err != nil {
		t.Fatalf("First ObtainWriteLock() error = %v", err)
	}
	defer writeLock1.Close()

	// Try to obtain another write lock - should timeout
	_, err = factory.ObtainWriteLock("test-lock")
	if err == nil {
		t.Error("Second ObtainWriteLock() expected timeout error")
	}
}

func TestNRTLockFactory_CommitBlocksAll(t *testing.T) {
	factory := NewNRTLockFactory()
	factory.SetDefaultTimeout(50 * time.Millisecond)

	// Obtain commit lock
	commitLock, err := factory.ObtainCommitLock("test-lock")
	if err != nil {
		t.Fatalf("ObtainCommitLock() error = %v", err)
	}
	defer commitLock.Close()

	// Try to obtain read lock - should timeout
	_, err = factory.ObtainReadLock("test-lock")
	if err == nil {
		t.Error("ObtainReadLock() expected timeout error with commit lock")
	}

	// Try to obtain write lock - should timeout
	_, err = factory.ObtainWriteLock("test-lock")
	if err == nil {
		t.Error("ObtainWriteLock() expected timeout error with commit lock")
	}
}

func TestNRTLockFactory_ConcurrentReads(t *testing.T) {
	factory := NewNRTLockFactory()

	const numReaders = 10
	var wg sync.WaitGroup
	wg.Add(numReaders)

	for i := 0; i < numReaders; i++ {
		go func(id int) {
			defer wg.Done()

			lock, err := factory.ObtainReadLock("concurrent-lock")
			if err != nil {
				t.Errorf("Reader %d: ObtainReadLock() error = %v", id, err)
				return
			}
			defer lock.Close()

			time.Sleep(10 * time.Millisecond)
		}(i)
	}

	wg.Wait()
}

func TestNRTLockFactory_ConcurrentWrites(t *testing.T) {
	factory := NewNRTLockFactory()

	const numWriters = 5
	var wg sync.WaitGroup
	wg.Add(numWriters)

	acquired := make(chan bool, numWriters)

	for i := 0; i < numWriters; i++ {
		go func(id int) {
			defer wg.Done()

			lock, err := factory.ObtainWriteLock("concurrent-write-lock")
			if err != nil {
				return
			}
			defer lock.Close()

			acquired <- true
			time.Sleep(20 * time.Millisecond)
		}(i)
	}

	wg.Wait()
	close(acquired)

	// All writers should have acquired the lock (sequentially)
	count := 0
	for range acquired {
		count++
	}
	if count != numWriters {
		t.Errorf("Expected %d writers to acquire lock, got %d", numWriters, count)
	}
}

func TestNRTLockFactory_Stats(t *testing.T) {
	factory := NewNRTLockFactory()
	dir := NewByteBuffersDirectory()

	// Initial stats
	stats := factory.GetStats()
	if stats.TotalLocks != 0 {
		t.Errorf("Initial TotalLocks = %v, want 0", stats.TotalLocks)
	}

	// Create some locks
	for i := 0; i < 5; i++ {
		lock, err := factory.ObtainLock(dir, "stats-test-lock")
		if err != nil {
			t.Fatalf("ObtainLock() error = %v", err)
		}
		lock.Close()
	}

	// Check stats
	stats = factory.GetStats()
	if stats.TotalLocks < 1 {
		t.Errorf("TotalLocks = %v, want >= 1", stats.TotalLocks)
	}
	if stats.TotalAcquires < 5 {
		t.Errorf("TotalAcquires = %v, want >= 5", stats.TotalAcquires)
	}
	if stats.TotalReleases < 5 {
		t.Errorf("TotalReleases = %v, want >= 5", stats.TotalReleases)
	}
}

func TestNRTLockFactory_ResetStats(t *testing.T) {
	factory := NewNRTLockFactory()
	dir := NewByteBuffersDirectory()

	// Create some activity
	lock, _ := factory.ObtainLock(dir, "reset-test")
	lock.Close()

	// Reset stats
	factory.ResetStats()

	// Verify reset
	stats := factory.GetStats()
	if stats.TotalLocks != 0 {
		t.Errorf("TotalLocks after reset = %v, want 0", stats.TotalLocks)
	}
}

func TestNRTLockFactory_SettersAndGetters(t *testing.T) {
	factory := NewNRTLockFactory()

	// Test default timeout
	factory.SetDefaultTimeout(5 * time.Second)
	if got := factory.GetDefaultTimeout(); got != 5*time.Second {
		t.Errorf("GetDefaultTimeout() = %v, want 5s", got)
	}

	// Test default retry delay
	factory.SetDefaultRetryDelay(50 * time.Millisecond)
	if got := factory.GetDefaultRetryDelay(); got != 50*time.Millisecond {
		t.Errorf("GetDefaultRetryDelay() = %v, want 50ms", got)
	}
}

func TestNRTLock_Duration(t *testing.T) {
	factory := NewNRTLockFactory()

	lock, err := factory.ObtainReadLock("duration-test")
	if err != nil {
		t.Fatalf("ObtainReadLock() error = %v", err)
	}
	defer lock.Close()

	// Wait a bit
	time.Sleep(50 * time.Millisecond)

	duration := lock.GetDuration()
	if duration < 50*time.Millisecond {
		t.Errorf("GetDuration() = %v, want >= 50ms", duration)
	}
}

func TestNRTLock_String(t *testing.T) {
	factory := NewNRTLockFactory()

	lock, err := factory.ObtainReadLock("string-test")
	if err != nil {
		t.Fatalf("ObtainReadLock() error = %v", err)
	}
	defer lock.Close()

	str := lock.String()
	if !containsSubstring(str, "string-test") {
		t.Error("String() should contain lock name")
	}
	if !containsSubstring(str, "READ") {
		t.Error("String() should contain lock type")
	}
}

func TestNRTLock_DoubleClose(t *testing.T) {
	factory := NewNRTLockFactory()

	lock, err := factory.ObtainReadLock("double-close-test")
	if err != nil {
		t.Fatalf("ObtainReadLock() error = %v", err)
	}

	// First close
	if err := lock.Close(); err != nil {
		t.Errorf("First Close() error = %v", err)
	}

	// Second close should succeed (idempotent)
	if err := lock.Close(); err != nil {
		t.Errorf("Second Close() error = %v", err)
	}
}

func TestNRTLockFactory_DifferentLockNames(t *testing.T) {
	factory := NewNRTLockFactory()

	// Locks with different names should not conflict
	lock1, err := factory.ObtainWriteLock("lock-1")
	if err != nil {
		t.Fatalf("ObtainWriteLock(lock-1) error = %v", err)
	}
	defer lock1.Close()

	lock2, err := factory.ObtainWriteLock("lock-2")
	if err != nil {
		t.Fatalf("ObtainWriteLock(lock-2) error = %v", err)
	}
	defer lock2.Close()

	// Both should be held
	if !lock1.IsLocked() {
		t.Error("lock1 should be held")
	}
	if !lock2.IsLocked() {
		t.Error("lock2 should be held")
	}
}

func TestNRTLockFactory_MaxConcurrentLocks(t *testing.T) {
	factory := NewNRTLockFactory()

	// Create multiple concurrent locks with different names
	const numLocks = 5
	locks := make([]*NRTLock, numLocks)

	for i := 0; i < numLocks; i++ {
		lock, err := factory.ObtainWriteLock(fmt.Sprintf("concurrent-lock-%d", i))
		if err != nil {
			t.Fatalf("ObtainWriteLock() error = %v", err)
		}
		locks[i] = lock
	}

	// Check stats
	stats := factory.GetStats()
	if stats.MaxConcurrentLocks != int64(numLocks) {
		t.Errorf("MaxConcurrentLocks = %v, want %v", stats.MaxConcurrentLocks, numLocks)
	}

	// Release all locks
	for _, lock := range locks {
		lock.Close()
	}
}

func TestNRTLockFactory_TimeoutStats(t *testing.T) {
	factory := NewNRTLockFactory()
	factory.SetDefaultTimeout(10 * time.Millisecond)

	// Obtain a lock
	lock, _ := factory.ObtainWriteLock("timeout-stat-lock")
	defer lock.Close()

	// Try to obtain another (should timeout)
	_, _ = factory.ObtainWriteLock("timeout-stat-lock")

	stats := factory.GetStats()
	if stats.TimedOutAcquires < 1 {
		t.Errorf("TimedOutAcquires = %v, want >= 1", stats.TimedOutAcquires)
	}
	if stats.FailedAcquires < 1 {
		t.Errorf("FailedAcquires = %v, want >= 1", stats.FailedAcquires)
	}
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
