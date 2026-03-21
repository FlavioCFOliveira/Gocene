package store

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// NRTLockType represents the type of lock for NRT operations.
type NRTLockType int32

const (
	// NRTLockTypeRead represents a read lock.
	NRTLockTypeRead NRTLockType = iota
	// NRTLockTypeWrite represents a write lock.
	NRTLockTypeWrite
	// NRTLockTypeCommit represents a commit lock (exclusive, blocks reads and writes).
	NRTLockTypeCommit
)

// String returns the string representation of the lock type.
func (t NRTLockType) String() string {
	switch t {
	case NRTLockTypeRead:
		return "READ"
	case NRTLockTypeWrite:
		return "WRITE"
	case NRTLockTypeCommit:
		return "COMMIT"
	default:
		return "UNKNOWN"
	}
}

// NRTLockFactory provides locking optimized for Near Real-Time (NRT) operations.
// It supports read/write locks with timeout and retry mechanisms.
// This is the Go port of Lucene's NRT lock factory pattern.
type NRTLockFactory struct {
	mu sync.RWMutex

	// locks holds active locks by name.
	locks map[string]*nrtLockEntry

	// defaultTimeout is the default timeout for lock acquisition.
	defaultTimeout time.Duration

	// defaultRetryDelay is the default delay between retry attempts.
	defaultRetryDelay time.Duration

	// stats holds lock statistics.
	stats NRTLockStats
}

// nrtLockEntry represents a single lock entry.
type nrtLockEntry struct {
	mu sync.Mutex

	// name is the lock name.
	name string

	// readCount is the number of active read locks.
	readCount int32

	// writeLocked indicates if a write lock is held.
	writeLocked bool

	// commitLocked indicates if a commit lock is held.
	commitLocked bool

	// waiters is the number of goroutines waiting for this lock.
	waiters int32

	// owner is the identifier of the current write/commit lock holder.
	owner string
}

// NRTLockStats holds statistics for the lock factory.
type NRTLockStats struct {
	// TotalLocks is the total number of locks created.
	TotalLocks int64

	// TotalAcquires is the total number of lock acquisitions.
	TotalAcquires int64

	// FailedAcquires is the number of failed acquisitions.
	FailedAcquires int64

	// TimedOutAcquires is the number of acquisitions that timed out.
	TimedOutAcquires int64

	// TotalReleases is the total number of lock releases.
	TotalReleases int64

	// CurrentLocks is the current number of active locks.
	CurrentLocks int64

	// MaxConcurrentLocks is the maximum number of concurrent locks.
	MaxConcurrentLocks int64
}

// NewNRTLockFactory creates a new NRTLockFactory.
func NewNRTLockFactory() *NRTLockFactory {
	return &NRTLockFactory{
		locks:             make(map[string]*nrtLockEntry),
		defaultTimeout:    30 * time.Second,
		defaultRetryDelay: 10 * time.Millisecond,
	}
}

// ObtainLock obtains a lock for the specified name.
// For NRT operations, this creates a write lock by default.
func (f *NRTLockFactory) ObtainLock(dir Directory, lockName string) (Lock, error) {
	return f.ObtainLockWithType(lockName, NRTLockTypeWrite, f.defaultTimeout)
}

// ObtainLockWithType obtains a lock with the specified type and timeout.
func (f *NRTLockFactory) ObtainLockWithType(lockName string, lockType NRTLockType, timeout time.Duration) (*NRTLock, error) {
	if lockName == "" {
		return nil, fmt.Errorf("lock name cannot be empty")
	}

	atomic.AddInt64(&f.stats.TotalAcquires, 1)

	entry := f.getOrCreateEntry(lockName)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	acquired := entry.acquire(ctx, lockType)
	if !acquired {
		atomic.AddInt64(&f.stats.TimedOutAcquires, 1)
		atomic.AddInt64(&f.stats.FailedAcquires, 1)
		return nil, fmt.Errorf("failed to acquire %s lock %s within %v", lockType, lockName, timeout)
	}

	atomic.AddInt64(&f.stats.CurrentLocks, 1)

	// Update max concurrent locks
	current := atomic.LoadInt64(&f.stats.CurrentLocks)
	for {
		max := atomic.LoadInt64(&f.stats.MaxConcurrentLocks)
		if current <= max {
			break
		}
		if atomic.CompareAndSwapInt64(&f.stats.MaxConcurrentLocks, max, current) {
			break
		}
	}

	return &NRTLock{
		factory:    f,
		entry:      entry,
		lockType:   lockType,
		acquiredAt: time.Now(),
	}, nil
}

// ObtainReadLock obtains a read lock.
func (f *NRTLockFactory) ObtainReadLock(lockName string) (*NRTLock, error) {
	return f.ObtainLockWithType(lockName, NRTLockTypeRead, f.defaultTimeout)
}

// ObtainWriteLock obtains a write lock.
func (f *NRTLockFactory) ObtainWriteLock(lockName string) (*NRTLock, error) {
	return f.ObtainLockWithType(lockName, NRTLockTypeWrite, f.defaultTimeout)
}

// ObtainCommitLock obtains a commit lock (most exclusive).
func (f *NRTLockFactory) ObtainCommitLock(lockName string) (*NRTLock, error) {
	return f.ObtainLockWithType(lockName, NRTLockTypeCommit, f.defaultTimeout)
}

// getOrCreateEntry gets or creates a lock entry.
func (f *NRTLockFactory) getOrCreateEntry(name string) *nrtLockEntry {
	f.mu.Lock()
	defer f.mu.Unlock()

	entry, exists := f.locks[name]
	if !exists {
		entry = &nrtLockEntry{name: name}
		f.locks[name] = entry
		atomic.AddInt64(&f.stats.TotalLocks, 1)
	}

	return entry
}

// releaseLock releases a lock.
func (f *NRTLockFactory) releaseLock(lock *NRTLock) {
	if lock == nil || lock.entry == nil {
		return
	}

	lock.entry.release(lock.lockType)
	atomic.AddInt64(&f.stats.TotalReleases, 1)
	atomic.AddInt64(&f.stats.CurrentLocks, -1)
}

// SetDefaultTimeout sets the default timeout for lock acquisition.
func (f *NRTLockFactory) SetDefaultTimeout(timeout time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.defaultTimeout = timeout
}

// GetDefaultTimeout returns the default timeout.
func (f *NRTLockFactory) GetDefaultTimeout() time.Duration {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return f.defaultTimeout
}

// SetDefaultRetryDelay sets the default retry delay.
func (f *NRTLockFactory) SetDefaultRetryDelay(delay time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.defaultRetryDelay = delay
}

// GetDefaultRetryDelay returns the default retry delay.
func (f *NRTLockFactory) GetDefaultRetryDelay() time.Duration {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return f.defaultRetryDelay
}

// GetStats returns lock statistics.
func (f *NRTLockFactory) GetStats() NRTLockStats {
	return NRTLockStats{
		TotalLocks:         atomic.LoadInt64(&f.stats.TotalLocks),
		TotalAcquires:      atomic.LoadInt64(&f.stats.TotalAcquires),
		FailedAcquires:     atomic.LoadInt64(&f.stats.FailedAcquires),
		TimedOutAcquires:   atomic.LoadInt64(&f.stats.TimedOutAcquires),
		TotalReleases:      atomic.LoadInt64(&f.stats.TotalReleases),
		CurrentLocks:       atomic.LoadInt64(&f.stats.CurrentLocks),
		MaxConcurrentLocks: atomic.LoadInt64(&f.stats.MaxConcurrentLocks),
	}
}

// ResetStats resets lock statistics.
func (f *NRTLockFactory) ResetStats() {
	f.stats = NRTLockStats{}
}

// acquire attempts to acquire the lock.
func (e *nrtLockEntry) acquire(ctx context.Context, lockType NRTLockType) bool {
	atomic.AddInt32(&e.waiters, 1)
	defer atomic.AddInt32(&e.waiters, -1)

	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()

	for {
		e.mu.Lock()

		canAcquire := e.canAcquire(lockType)
		if canAcquire {
			e.grantLock(lockType)
			e.mu.Unlock()
			return true
		}

		e.mu.Unlock()

		// Wait with context
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			// Retry
		}
	}
}

// canAcquire checks if the lock can be acquired.
func (e *nrtLockEntry) canAcquire(lockType NRTLockType) bool {
	switch lockType {
	case NRTLockTypeRead:
		// Can acquire read lock if no write or commit lock is held
		return !e.writeLocked && !e.commitLocked
	case NRTLockTypeWrite:
		// Can acquire write lock if no other locks are held
		return e.readCount == 0 && !e.writeLocked && !e.commitLocked
	case NRTLockTypeCommit:
		// Can acquire commit lock if no other locks are held
		return e.readCount == 0 && !e.writeLocked && !e.commitLocked
	}
	return false
}

// grantLock grants the lock.
func (e *nrtLockEntry) grantLock(lockType NRTLockType) {
	switch lockType {
	case NRTLockTypeRead:
		e.readCount++
	case NRTLockTypeWrite:
		e.writeLocked = true
	case NRTLockTypeCommit:
		e.commitLocked = true
	}
}

// release releases the lock.
func (e *nrtLockEntry) release(lockType NRTLockType) {
	e.mu.Lock()
	defer e.mu.Unlock()

	switch lockType {
	case NRTLockTypeRead:
		if e.readCount > 0 {
			e.readCount--
		}
	case NRTLockTypeWrite:
		e.writeLocked = false
		e.owner = ""
	case NRTLockTypeCommit:
		e.commitLocked = false
		e.owner = ""
	}
}

// NRTLock represents a lock obtained from NRTLockFactory.
type NRTLock struct {
	// factory is the factory that created this lock.
	factory *NRTLockFactory

	// entry is the lock entry.
	entry *nrtLockEntry

	// lockType is the type of lock.
	lockType NRTLockType

	// acquiredAt is when the lock was acquired.
	acquiredAt time.Time

	// released indicates if the lock has been released.
	released bool
}

// Close releases the lock.
func (l *NRTLock) Close() error {
	if l.released {
		return nil
	}

	l.factory.releaseLock(l)
	l.released = true

	return nil
}

// EnsureValid returns an error if the lock is no longer valid.
func (l *NRTLock) EnsureValid() error {
	if l.released {
		return fmt.Errorf("lock %s has been released", l.entry.name)
	}

	return nil
}

// IsLocked returns true if the lock is still held.
func (l *NRTLock) IsLocked() bool {
	return !l.released
}

// GetType returns the lock type.
func (l *NRTLock) GetType() NRTLockType {
	return l.lockType
}

// GetName returns the lock name.
func (l *NRTLock) GetName() string {
	return l.entry.name
}

// GetDuration returns how long the lock has been held.
func (l *NRTLock) GetDuration() time.Duration {
	return time.Since(l.acquiredAt)
}

// String returns a string representation of the lock.
func (l *NRTLock) String() string {
	return fmt.Sprintf("NRTLock{name=%s, type=%s, held=%v}",
		l.entry.name, l.lockType, l.GetDuration())
}
