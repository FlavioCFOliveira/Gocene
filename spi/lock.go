// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

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
