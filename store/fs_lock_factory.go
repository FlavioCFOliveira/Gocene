// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import "fmt"

// FSLockFactory is the base abstraction for filesystem-based LockFactory
// implementations. It enforces that the passed Directory is (or wraps) an
// FSDirectory.
//
// This is the Go port of org.apache.lucene.store.FSLockFactory from Apache
// Lucene 10.4.0. Concrete subclasses (e.g. SimpleFSLockFactory) embed
// FSLockFactory and provide ObtainFSLock.
type FSLockFactory struct {
	obtainFSLock func(dir *FSDirectory, lockName string) (Lock, error)
}

// NewFSLockFactory constructs an FSLockFactory whose obtainFSLock callback
// is supplied by the embedding factory. Embedders should pass their concrete
// lock-obtention logic so that ObtainLock can dispatch through them.
func NewFSLockFactory(obtainFSLock func(dir *FSDirectory, lockName string) (Lock, error)) *FSLockFactory {
	return &FSLockFactory{obtainFSLock: obtainFSLock}
}

// ObtainLock validates that dir is an FSDirectory (possibly wrapped) and
// delegates to the embedder's obtainFSLock callback.
//
// Mirrors Lucene's FSLockFactory.obtainLock which throws
// UnsupportedOperationException when the Directory is not an FSDirectory.
func (f *FSLockFactory) ObtainLock(dir Directory, lockName string) (Lock, error) {
	fs := unwrapFSDirectory(dir)
	if fs == nil {
		return nil, fmt.Errorf("FSLockFactory can only be used with FSDirectory subclasses, got: %T", dir)
	}
	if f.obtainFSLock == nil {
		return nil, fmt.Errorf("FSLockFactory missing obtainFSLock callback")
	}
	return f.obtainFSLock(fs, lockName)
}

// unwrapFSDirectory walks FilterDirectory chains looking for an FSDirectory.
// Returns nil when no FSDirectory is reachable.
func unwrapFSDirectory(dir Directory) *FSDirectory {
	for {
		switch d := dir.(type) {
		case *FSDirectory:
			return d
		case *SimpleFSDirectory:
			return d.FSDirectory
		case *NIOFSDirectory:
			return d.FSDirectory
		case *MMapDirectory:
			return d.FSDirectory
		case *FilterDirectory:
			dir = d.GetDelegate()
		default:
			return nil
		}
	}
}

// DefaultFSLockFactory returns the default FSLockFactory for this platform,
// matching Lucene's FSLockFactory.getDefault. Currently always returns the
// NativeFSLockFactory singleton.
func DefaultFSLockFactory() LockFactory {
	return NewNativeFSLockFactory()
}
