// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build !windows

package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"golang.org/x/sys/unix"
)

// lockHeld is the in-process registry of currently-held NativeFSLock paths,
// equivalent to Lucene's static synchronized LOCK_HELD set. A path is present
// in this map while a NativeFSLock for that path is alive in this process.
// Using sync.Map avoids a global mutex on the hot path while still being safe
// for concurrent access from multiple goroutines / IndexWriter instances.
var lockHeld sync.Map // map[string]struct{}

// NativeFSLockFactory creates OS-level advisory locks using fcntl(2) F_SETLK.
// Advisory locks are automatically released by the OS when the process exits,
// which is the primary advantage over SimpleFSLockFactory.
//
// This is the Go port of org.apache.lucene.store.NativeFSLockFactory from
// Apache Lucene 10.4.0. Like the Java original it is a singleton.
type NativeFSLockFactory struct{}

// NewNativeFSLockFactory returns the NativeFSLockFactory singleton.
func NewNativeFSLockFactory() *NativeFSLockFactory {
	return &NativeFSLockFactory{}
}

// ObtainLock obtains a POSIX advisory write lock on the named file inside
// dir's directory. The lock file is created if it does not exist and is never
// deleted, matching Lucene's contract. Two concurrent calls from the same
// process for the same path return an error for the second caller (via
// lockHeld) without reaching the kernel.
func (f *NativeFSLockFactory) ObtainLock(dir Directory, lockName string) (Lock, error) {
	dirPath := nativeFSPath(dir)
	if dirPath == "" {
		return nil, fmt.Errorf("NativeFSLockFactory: directory %T has no filesystem path", dir)
	}

	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return nil, fmt.Errorf("NativeFSLockFactory: create lock dir %q: %w", dirPath, err)
	}

	lockFile := filepath.Join(dirPath, lockName)

	// Create the file so we have a canonical real path; ignore "already exists".
	f2, createErr := os.OpenFile(lockFile, os.O_CREATE|os.O_WRONLY, 0644)
	if createErr == nil {
		_ = f2.Close()
	}

	realPath, err := filepath.EvalSymlinks(lockFile)
	if err != nil {
		if createErr != nil {
			return nil, fmt.Errorf("NativeFSLockFactory: resolve lock path %q: %w (create also failed: %v)", lockFile, err, createErr)
		}
		return nil, fmt.Errorf("NativeFSLockFactory: resolve lock path %q: %w", lockFile, err)
	}

	// In-process guard: reject if this process already holds the lock.
	if _, loaded := lockHeld.LoadOrStore(realPath, struct{}{}); loaded {
		return nil, fmt.Errorf("lock held by this process: %s", realPath)
	}

	// Open the lock file for writing — the file descriptor is kept alive for
	// the duration of the lock (the advisory lock is tied to the fd).
	fh, err := os.OpenFile(realPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		lockHeld.Delete(realPath)
		return nil, fmt.Errorf("NativeFSLockFactory: open lock file %q: %w", realPath, err)
	}

	// Attempt a non-blocking exclusive OFD lock (F_OFD_SETLK).
	// OFD locks are per open-file-description (fd), not per process, making
	// them immune to the POSIX advisory lock inheritance and fork anomalies.
	// Returns EACCES or EAGAIN if another fd already holds the lock.
	flk := unix.Flock_t{
		Type:   unix.F_WRLCK,
		Whence: int16(unix.SEEK_SET),
		Start:  0,
		Len:    0, // 0 = entire file
	}
	if err := unix.FcntlFlock(fh.Fd(), unix.F_OFD_SETLK, &flk); err != nil {
		_ = fh.Close()
		lockHeld.Delete(realPath)
		if errors.Is(err, unix.EACCES) || errors.Is(err, unix.EAGAIN) {
			return nil, fmt.Errorf("lock held by another process: %s", realPath)
		}
		return nil, fmt.Errorf("NativeFSLockFactory: fcntl F_OFD_SETLK %q: %w", realPath, err)
	}

	return &NativeFSLock{
		name:     lockName,
		realPath: realPath,
		fh:       fh,
	}, nil
}

// nativeFSPath extracts the filesystem path from a Directory, walking
// FilterDirectory chains. Returns "" if no filesystem path is reachable.
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

// NativeFSLock is a POSIX advisory write lock backed by an open file descriptor.
// The advisory lock is held for the lifetime of fh; closing fh releases it
// atomically in the kernel — even if the process is killed.
type NativeFSLock struct {
	name     string
	realPath string
	fh       *os.File
	closed   atomic.Bool
	mu       sync.Mutex
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

	// Release OFD lock then close fd. Closing the fd would also release the
	// OFD lock automatically, but explicit unlock makes the intent clear.
	flk := unix.Flock_t{
		Type:   unix.F_UNLCK,
		Whence: int16(unix.SEEK_SET),
		Start:  0,
		Len:    0,
	}
	_ = unix.FcntlFlock(l.fh.Fd(), unix.F_OFD_SETLK, &flk)
	if err := l.fh.Close(); err != nil {
		lockHeld.Delete(l.realPath)
		return fmt.Errorf("NativeFSLock: close %q: %w", l.realPath, err)
	}

	lockHeld.Delete(l.realPath)
	return nil
}

// EnsureValid returns an error if the lock has been released or if the
// advisory lock is no longer valid (e.g. the fd was externally invalidated).
func (l *NativeFSLock) EnsureValid() error {
	if l.closed.Load() {
		return fmt.Errorf("lock %s has been released", l.name)
	}
	if _, ok := lockHeld.Load(l.realPath); !ok {
		return fmt.Errorf("lock path unexpectedly cleared: %s", l.realPath)
	}
	// Probe the fd with a zero-byte stat; surfaces EBADF if the fd is dead.
	if _, err := l.fh.Stat(); err != nil {
		return fmt.Errorf("lock file descriptor invalid: %w", err)
	}
	return nil
}

// IsLocked reports whether the lock is still held.
func (l *NativeFSLock) IsLocked() bool {
	return !l.closed.Load()
}
