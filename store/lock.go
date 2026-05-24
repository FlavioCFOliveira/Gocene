// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"errors"
	"fmt"
	"sync"
)

// Lock represents a lock obtained by a LockFactory.
//
// This is the Go port of Lucene's org.apache.lucene.store.Lock.
type Lock interface {
	// Close releases the lock. If the lock is already released, this returns nil.
	Close() error

	// EnsureValid returns an error if the lock is no longer valid.
	// This should be called periodically to verify the lock is still held.
	EnsureValid() error

	// IsLocked returns true if the lock is still held.
	IsLocked() bool
}

// LockFactory is a factory for creating locks.
//
// This is the Go port of Lucene's org.apache.lucene.store.LockFactory.
type LockFactory interface {
	// ObtainLock attempts to obtain a lock for the specified name.
	// Returns the Lock instance if successful, or an error if the lock
	// could not be obtained.
	ObtainLock(dir Directory, lockName string) (Lock, error)
}

// BaseLock provides common functionality for Lock implementations.
type BaseLock struct {
	locked bool
}

// NewBaseLock creates a new BaseLock.
func NewBaseLock() *BaseLock {
	return &BaseLock{locked: true}
}

// IsLocked returns true if the lock is still held.
func (l *BaseLock) IsLocked() bool {
	return l.locked
}

// MarkReleased marks the lock as released.
func (l *BaseLock) MarkReleased() {
	l.locked = false
}

// VerifyLocked returns an error if the lock is not held.
func (l *BaseLock) VerifyLocked() error {
	if !l.locked {
		return errors.New("lock is not held")
	}
	return nil
}

// SingleInstanceLockFactory creates locks that prevent multiple IndexWriters
// in the same JVM (or process) from accessing the same directory.
type SingleInstanceLockFactory struct {
	mu    sync.Mutex
	locks map[string]Lock
}

// NewSingleInstanceLockFactory creates a new SingleInstanceLockFactory.
func NewSingleInstanceLockFactory() *SingleInstanceLockFactory {
	return &SingleInstanceLockFactory{
		locks: make(map[string]Lock),
	}
}

// ObtainLock obtains a lock.
func (f *SingleInstanceLockFactory) ObtainLock(dir Directory, lockName string) (Lock, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.locks[lockName]; exists {
		return nil, fmt.Errorf("lock %s is already held", lockName)
	}
	lock := &SingleInstanceLock{
		BaseLock: NewBaseLock(),
		factory:  f,
		name:     lockName,
	}
	f.locks[lockName] = lock
	return lock, nil
}

// releaseLock releases a lock.
func (f *SingleInstanceLockFactory) releaseLock(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.locks, name)
}

// SingleInstanceLock is a lock that works within a single process.
type SingleInstanceLock struct {
	*BaseLock
	factory *SingleInstanceLockFactory
	name    string
}

// Close releases the lock.
func (l *SingleInstanceLock) Close() error {
	if !l.IsLocked() {
		return nil
	}
	l.factory.releaseLock(l.name)
	l.MarkReleased()
	return nil
}

// EnsureValid returns an error if the lock is no longer valid.
func (l *SingleInstanceLock) EnsureValid() error {
	if err := l.VerifyLocked(); err != nil {
		return fmt.Errorf("lock %s is not valid: %w", l.name, err)
	}
	return nil
}

// NoLockFactory is a LockFactory that does not create any locks.
// This is useful for read-only operations or in single-threaded environments.
type NoLockFactory struct{}

// NewNoLockFactory creates a new NoLockFactory.
func NewNoLockFactory() *NoLockFactory {
	return &NoLockFactory{}
}

// ObtainLock returns a no-op lock.
func (f *NoLockFactory) ObtainLock(dir Directory, lockName string) (Lock, error) {
	return &NoOpLock{}, nil
}

// NoOpLock is a lock that does nothing.
type NoOpLock struct{}

// Close does nothing.
func (l *NoOpLock) Close() error { return nil }

// EnsureValid always returns nil.
func (l *NoOpLock) EnsureValid() error { return nil }

// IsLocked always returns false.
func (l *NoOpLock) IsLocked() bool { return false }

// MockLock is a lock implementation for testing.
// It tracks whether it was created and can be released.
type MockLock struct {
	*BaseLock
	name string
}

// NewMockLock creates a new MockLock for testing.
func NewMockLock() *MockLock {
	return &MockLock{
		BaseLock: NewBaseLock(),
		name:     "mock",
	}
}

// Close releases the lock.
func (l *MockLock) Close() error {
	if !l.IsLocked() {
		return nil
	}
	l.MarkReleased()
	return nil
}

// EnsureValid returns an error if the lock is no longer valid.
func (l *MockLock) EnsureValid() error {
	return l.VerifyLocked()
}
