// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

// SimpleFSLockFactory is a LockFactory that uses Files.createFile-style
// exclusive create semantics: a lock file is created on ObtainLock and
// deleted on Close.
//
// This is the Go port of org.apache.lucene.store.SimpleFSLockFactory from
// Apache Lucene 10.4.0. The main downside relative to NativeFSLockFactory is
// that the lock may persist if the process exits abnormally; callers must
// then delete the lock file manually after confirming no writer is running.
//
// SimpleFSLockFactoryInstance is the singleton recommended by Lucene; it is
// safe to share across goroutines.
type SimpleFSLockFactory struct {
	*FSLockFactory
}

// SimpleFSLockFactoryInstance is the package-level singleton, equivalent to
// Lucene's SimpleFSLockFactory.INSTANCE.
var SimpleFSLockFactoryInstance = NewSimpleFSLockFactory()

// NewSimpleFSLockFactory constructs a SimpleFSLockFactory. Prefer
// SimpleFSLockFactoryInstance when sharing across the package.
func NewSimpleFSLockFactory() *SimpleFSLockFactory {
	f := &SimpleFSLockFactory{}
	f.FSLockFactory = NewFSLockFactory(f.obtain)
	return f
}

// obtain implements the FSLockFactory ObtainFSLock callback.
func (f *SimpleFSLockFactory) obtain(dir *FSDirectory, lockName string) (Lock, error) {
	lockDir := dir.GetPath()
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		return nil, err
	}
	lockFile := filepath.Join(lockDir, lockName)

	// O_CREATE|O_EXCL fails with EEXIST when the file already exists, which is
	// the documented Lucene semantic.
	fh, err := os.OpenFile(lockFile, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o644)
	if err != nil {
		if errors.Is(err, fs.ErrExist) || errors.Is(err, fs.ErrPermission) {
			return nil, NewLockObtainFailedException(
				fmt.Sprintf("Lock held elsewhere: %s", lockFile), err)
		}
		return nil, err
	}
	// We only need the file's existence; close the handle but retain the path.
	if err := fh.Close(); err != nil {
		return nil, err
	}

	info, err := os.Stat(lockFile)
	if err != nil {
		return nil, err
	}
	return &simpleFSLock{
		path:    lockFile,
		modTime: info.ModTime().UnixNano(),
	}, nil
}

// simpleFSLock is the Lock returned by SimpleFSLockFactory. It validates the
// lock file's existence and creation time on EnsureValid and deletes the
// file on Close.
type simpleFSLock struct {
	mu      sync.Mutex
	path    string
	modTime int64
	closed  bool
}

// EnsureValid returns an error if the lock has been released or if the
// backing file has been externally altered (deleted and recreated).
func (l *simpleFSLock) EnsureValid() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return NewAlreadyClosedException(fmt.Sprintf("Lock instance already released: %s", l.path), nil)
	}
	info, err := os.Stat(l.path)
	if err != nil {
		return NewAlreadyClosedException(fmt.Sprintf("Lock file missing: %s", l.path), err)
	}
	if info.ModTime().UnixNano() != l.modTime {
		return NewAlreadyClosedException(
			fmt.Sprintf("Underlying file changed by an external force (lock=%s)", l.path), nil)
	}
	return nil
}

// Close releases the lock by deleting the backing file. Best-effort
// validation is performed first; failures are wrapped in
// LockReleaseFailedException to match Lucene's contract.
func (l *simpleFSLock) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return nil
	}
	defer func() { l.closed = true }()

	// EnsureValid would need re-entrant lock; replicate inline.
	info, statErr := os.Stat(l.path)
	if statErr != nil {
		return NewLockReleaseFailedException(
			"Lock file cannot be safely removed. Manual intervention is recommended.", statErr)
	}
	if info.ModTime().UnixNano() != l.modTime {
		return NewLockReleaseFailedException(
			"Underlying lock file changed by an external force; manual intervention recommended.", nil)
	}

	if err := os.Remove(l.path); err != nil {
		return NewLockReleaseFailedException(
			"Unable to remove lock file. Manual intervention is recommended.", err)
	}
	return nil
}

// IsLocked returns true while the backing lock file is still owned by this
// instance.
func (l *simpleFSLock) IsLocked() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return !l.closed
}

// Compile-time assertion that simpleFSLock satisfies Lock.
var _ Lock = (*simpleFSLock)(nil)
