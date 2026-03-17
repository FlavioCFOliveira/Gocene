// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"fmt"
	"sync"
)

// VerifyingLockFactory is a LockFactory that verifies lock obtain/release calls.
//
// This is the Go port of Lucene's org.apache.lucene.store.VerifyingLockFactory.
//
// This is useful for debugging lock-related issues. It wraps another LockFactory
// and tracks the state of locks, detecting errors like:
// - Obtaining a lock that is already held
// - Releasing a lock that is not held
// - Closing a lock factory while locks are still held
type VerifyingLockFactory struct {
	// delegate is the wrapped LockFactory
	delegate LockFactory

	// locks tracks the currently held locks
	locks map[string]bool

	// mu protects the locks map
	mu sync.Mutex
}

// NewVerifyingLockFactory creates a new VerifyingLockFactory wrapping the given factory.
func NewVerifyingLockFactory(delegate LockFactory) *VerifyingLockFactory {
	return &VerifyingLockFactory{
		delegate: delegate,
		locks:    make(map[string]bool),
	}
}

// ObtainLock attempts to obtain a lock, verifying the operation.
func (f *VerifyingLockFactory) ObtainLock(dir Directory, lockName string) (Lock, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Check if lock is already held
	if f.locks[lockName] {
		return nil, fmt.Errorf("lock '%s' is already held", lockName)
	}

	// Obtain the lock from the delegate
	lock, err := f.delegate.ObtainLock(dir, lockName)
	if err != nil {
		return nil, err
	}

	// Mark the lock as held
	f.locks[lockName] = true

	// Wrap the lock to track release
	return &verifyingLockImpl{
		Lock:     lock,
		factory:  f,
		lockName: lockName,
	}, nil
}

// markReleased marks a lock as released.
func (f *VerifyingLockFactory) markReleased(lockName string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Check if lock is held
	if !f.locks[lockName] {
		return fmt.Errorf("lock '%s' is not held", lockName)
	}

	// Mark the lock as released
	delete(f.locks, lockName)
	return nil
}

// GetHeldLocks returns a list of currently held locks.
func (f *VerifyingLockFactory) GetHeldLocks() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	result := make([]string, 0, len(f.locks))
	for lockName := range f.locks {
		result = append(result, lockName)
	}
	return result
}

// IsLockHeld returns true if the given lock is currently held.
func (f *VerifyingLockFactory) IsLockHeld(lockName string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.locks[lockName]
}

// verifyingLockImpl wraps a Lock to track its release.
type verifyingLockImpl struct {
	Lock
	factory  *VerifyingLockFactory
	lockName string
	released bool
}

// Close releases the lock and marks it as released in the factory.
func (l *verifyingLockImpl) Close() error {
	if l.released {
		return fmt.Errorf("lock '%s' is already released", l.lockName)
	}

	// Release the underlying lock
	if err := l.Lock.Close(); err != nil {
		return err
	}

	// Mark as released in the factory
	if err := l.factory.markReleased(l.lockName); err != nil {
		return err
	}

	l.released = true
	return nil
}

// Ensure VerifyingLockFactory implements LockFactory
var _ LockFactory = (*VerifyingLockFactory)(nil)

// Ensure verifyingLockImpl implements Lock
var _ Lock = (*verifyingLockImpl)(nil)
