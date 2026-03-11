// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"errors"
	"fmt"
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

// NativeFSLockFactory creates locks using the native file system.
// This is the default LockFactory.
//
// It creates file-based locks that are visible to other processes.
type NativeFSLockFactory struct {
	lockDir string
}

// NewNativeFSLockFactory creates a new NativeFSLockFactory.
func NewNativeFSLockFactory() *NativeFSLockFactory {
	return &NativeFSLockFactory{}
}

// ObtainLock obtains a lock using the native file system.
func (f *NativeFSLockFactory) ObtainLock(dir Directory, lockName string) (Lock, error) {
	// This is a simplified implementation
	// A full implementation would use file locking syscalls
	// (flock on Unix, LockFile on Windows)
	return &NativeFSLock{
		BaseLock: NewBaseLock(),
		name:     lockName,
	}, nil
}

// NativeFSLock is a lock implemented using the native file system.
type NativeFSLock struct {
	*BaseLock
	name string
}

// Close releases the lock.
func (l *NativeFSLock) Close() error {
	if !l.IsLocked() {
		return nil
	}
	// In a full implementation, this would release the file lock
	l.MarkReleased()
	return nil
}

// EnsureValid returns an error if the lock is no longer valid.
func (l *NativeFSLock) EnsureValid() error {
	if err := l.VerifyLocked(); err != nil {
		return fmt.Errorf("lock %s is not valid: %w", l.name, err)
	}
	// In a full implementation, this would verify the lock file still exists
	return nil
}

// SingleInstanceLockFactory creates locks that prevent multiple IndexWriters
// in the same JVM (or process) from accessing the same directory.
type SingleInstanceLockFactory struct {
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
