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
	"time"

	"golang.org/x/sys/windows"
)

// lockHeld is the in-process registry of currently-held NativeFSLock paths.
var lockHeld sync.Map // map[string]struct{}

// NativeFSLockFactory creates OS-level advisory locks using LockFileEx on Windows.
// Like the Java original it is a singleton.
type NativeFSLockFactory struct{}

// NewNativeFSLockFactory returns the NativeFSLockFactory singleton.
func NewNativeFSLockFactory() *NativeFSLockFactory {
	return &NativeFSLockFactory{}
}

// ObtainLock obtains an exclusive OS lock on the named file inside dir's directory.
// The lock file is created if it does not exist and is never deleted, matching
// Lucene's contract.
func (f *NativeFSLockFactory) ObtainLock(dir Directory, lockName string) (Lock, error) {
	dirPath := nativeFSPath(dir)
	if dirPath == "" {
		return nil, fmt.Errorf("NativeFSLockFactory: directory %T has no filesystem path", dir)
	}

	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return nil, fmt.Errorf("NativeFSLockFactory: create lock dir %q: %w", dirPath, err)
	}

	lockFile := filepath.Join(dirPath, lockName)

	// Open the lock file (create if it doesn't exist).
	fh, err := os.OpenFile(lockFile, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("NativeFSLockFactory: open lock file %q: %w", lockFile, err)
	}

	// Resolve the canonical path for the in-process registry.
	realPath, err := filepath.Abs(lockFile)
	if err != nil {
		_ = fh.Close()
		return nil, fmt.Errorf("NativeFSLockFactory: resolve lock path %q: %w", lockFile, err)
	}

	// In-process guard: reject if this process already holds the lock.
	if _, loaded := lockHeld.LoadOrStore(realPath, struct{}{}); loaded {
		_ = fh.Close()
		return nil, NewLockObtainFailedException(fmt.Sprintf("lock held by this process: %s", realPath), nil)
	}

	// Capture the lock file's modification time as a best-effort check.
	info, err := os.Stat(realPath)
	if err != nil {
		_ = fh.Close()
		lockHeld.Delete(realPath)
		return nil, fmt.Errorf("NativeFSLockFactory: stat lock file %q: %w", realPath, err)
	}
	creationTime := info.ModTime()

	// Attempt a non-blocking exclusive lock using LockFileEx.
	// LOCKFILE_EXCLUSIVE_LOCK | LOCKFILE_FAIL_IMMEDIATELY
	handle := windows.Handle(fh.Fd())
	err = windows.LockFileEx(handle, windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &windows.Overlapped{})
	if err != nil {
		_ = fh.Close()
		lockHeld.Delete(realPath)
		if err == windows.ERROR_LOCK_VIOLATION {
			return nil, NewLockObtainFailedException(fmt.Sprintf("lock held by another process: %s", realPath), nil)
		}
		return nil, fmt.Errorf("NativeFSLockFactory: LockFileEx %q: %w", realPath, err)
	}

	return &NativeFSLock{
		name:         lockName,
		realPath:     realPath,
		fh:           fh,
		creationTime: creationTime,
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

// NativeFSLock is a Windows advisory write lock backed by a file handle.
type NativeFSLock struct {
	name         string
	realPath     string
	fh           *os.File
	closed       atomic.Bool
	mu           sync.Mutex
	creationTime time.Time
}

// Close releases the advisory lock and removes the path from the in-process
// registry. Idempotent: subsequent calls return nil.
func (l *NativeFSLock) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed.Load() {
		return nil
	}
	l.closed.Store(true)

	// Unlock the file before closing the handle.
	handle := windows.Handle(l.fh.Fd())
	err := windows.UnlockFileEx(handle, 0, 1, 0, &windows.Overlapped{})
	if err != nil {
		// Log or handle unlock error, but proceed to close the handle.
	}

	if err := l.fh.Close(); err != nil {
		lockHeld.Delete(l.realPath)
		return fmt.Errorf("NativeFSLock: close %q: %w", l.realPath, err)
	}

	lockHeld.Delete(l.realPath)
	return nil
}

// EnsureValid returns an error if the lock has been released or if the
// advisory lock is no longer valid.
func (l *NativeFSLock) EnsureValid() error {
	if l.closed.Load() {
		return fmt.Errorf("lock %s has been released", l.name)
	}
	if _, ok := lockHeld.Load(l.realPath); !ok {
		return fmt.Errorf("lock path unexpectedly cleared: %s", l.realPath)
	}
	// Probe the fd with a zero-byte stat; surfaces EBADF if the fd is dead.
	info, err := l.fh.Stat()
	if err != nil {
		return fmt.Errorf("lock file descriptor invalid: %w", err)
	}
	// Check if the lock file size is 0.
	if info.Size() != 0 {
		return fmt.Errorf("unexpected lock file size: %d, (lock=%s)", info.Size(), l.name)
	}
	// Verify the modification time hasn't changed.
	if !info.ModTime().Equal(l.creationTime) {
		return fmt.Errorf("underlying file changed by an external force at %v, (lock=%s)", info.ModTime(), l.name)
	}
	return nil
}

// IsLocked reports whether the lock is still held.
func (l *NativeFSLock) IsLocked() bool {
	return !l.closed.Load()
}
