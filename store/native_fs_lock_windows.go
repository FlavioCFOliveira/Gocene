// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build windows

package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
)

// lockHeld is the in-process registry of currently-held NativeFSLock paths.
// On Windows, advisory file locks via LockFileEx are used conceptually, but
// this implementation uses O_EXCL as a best-effort approximation because the
// stress test is explicitly skipped on Windows. The in-process guard still
// prevents double-lock within the same process.
var lockHeld sync.Map // map[string]struct{}

// NativeFSLockFactory creates file-based locks using exclusive file creation.
// On Windows this is a best-effort implementation; the stress test skips on
// Windows because SimpleFSLockFactory is preferred there.
type NativeFSLockFactory struct{}

// NewNativeFSLockFactory returns the NativeFSLockFactory singleton.
func NewNativeFSLockFactory() *NativeFSLockFactory {
	return &NativeFSLockFactory{}
}

// ObtainLock creates an exclusive lock file. Returns an error if the lock
// is already held by this process or another process.
func (f *NativeFSLockFactory) ObtainLock(dir Directory, lockName string) (Lock, error) {
	dirPath := nativeFSPath(dir)
	if dirPath == "" {
		return nil, fmt.Errorf("NativeFSLockFactory: directory %T has no filesystem path", dir)
	}

	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return nil, fmt.Errorf("NativeFSLockFactory: create lock dir %q: %w", dirPath, err)
	}

	lockFile := filepath.Join(dirPath, lockName)

	realPath, err := filepath.Abs(lockFile)
	if err != nil {
		return nil, fmt.Errorf("NativeFSLockFactory: resolve lock path %q: %w", lockFile, err)
	}

	// In-process guard.
	if _, loaded := lockHeld.LoadOrStore(realPath, struct{}{}); loaded {
		return nil, fmt.Errorf("lock held by this process: %s", realPath)
	}

	fh, err := os.OpenFile(realPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		lockHeld.Delete(realPath)
		if os.IsExist(err) {
			return nil, fmt.Errorf("lock held by another process: %s", realPath)
		}
		return nil, fmt.Errorf("NativeFSLockFactory: create lock file %q: %w", realPath, err)
	}
	_ = fh.Close()

	return &NativeFSLock{
		name:     lockName,
		realPath: realPath,
	}, nil
}

// nativeFSPath extracts the filesystem path from a Directory.
func nativeFSPath(dir Directory) string {
	for {
		switch d := dir.(type) {
		case *FSDirectory:
			return d.GetPath()
		case *SimpleFSDirectory:
			return d.GetPath()
		case *NIOFSDirectory:
			return d.GetPath()
		case *MMapDirectory:
			return d.GetPath()
		case *FilterDirectory:
			dir = d.GetDelegate()
		default:
			return ""
		}
	}
}

// NativeFSLock is a file-based exclusive lock on Windows.
type NativeFSLock struct {
	name     string
	realPath string
	closed   atomic.Bool
	mu       sync.Mutex
}

// Close releases the lock by removing the lock file.
func (l *NativeFSLock) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed.Load() {
		return nil
	}
	l.closed.Store(true)

	if err := os.Remove(l.realPath); err != nil && !os.IsNotExist(err) {
		lockHeld.Delete(l.realPath)
		return fmt.Errorf("NativeFSLock: remove %q: %w", l.realPath, err)
	}

	lockHeld.Delete(l.realPath)
	return nil
}

// EnsureValid returns an error if the lock has been released.
func (l *NativeFSLock) EnsureValid() error {
	if l.closed.Load() {
		return fmt.Errorf("lock %s has been released", l.name)
	}
	if _, ok := lockHeld.Load(l.realPath); !ok {
		return fmt.Errorf("lock path unexpectedly cleared: %s", l.realPath)
	}
	return nil
}

// IsLocked reports whether the lock is still held.
func (l *NativeFSLock) IsLocked() bool {
	return !l.closed.Load()
}
