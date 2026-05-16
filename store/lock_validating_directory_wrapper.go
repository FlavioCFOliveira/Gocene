// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

// LockValidatingDirectoryWrapper makes a best-effort check that a provided
// Lock is valid before any destructive filesystem operation.
//
// This is the Go port of
// org.apache.lucene.store.LockValidatingDirectoryWrapper from Apache Lucene
// 10.4.0. It wraps a Directory and, on every mutating operation
// (DeleteFile / CreateOutput / Rename / and the package-level helpers
// CopyFrom / Sync / SyncMetaData when those exist), first invokes
// writeLock.EnsureValid before forwarding to the wrapped Directory.
type LockValidatingDirectoryWrapper struct {
	*FilterDirectory
	writeLock Lock
}

// NewLockValidatingDirectoryWrapper wraps the given Directory and arms it
// with writeLock so destructive operations validate the lock first.
func NewLockValidatingDirectoryWrapper(in Directory, writeLock Lock) *LockValidatingDirectoryWrapper {
	return &LockValidatingDirectoryWrapper{
		FilterDirectory: NewFilterDirectory(in),
		writeLock:       writeLock,
	}
}

// DeleteFile validates the write lock then forwards to the wrapped directory.
func (d *LockValidatingDirectoryWrapper) DeleteFile(name string) error {
	if err := d.writeLock.EnsureValid(); err != nil {
		return err
	}
	return d.FilterDirectory.DeleteFile(name)
}

// CreateOutput validates the write lock then forwards to the wrapped
// directory.
func (d *LockValidatingDirectoryWrapper) CreateOutput(name string, ctx IOContext) (IndexOutput, error) {
	if err := d.writeLock.EnsureValid(); err != nil {
		return nil, err
	}
	return d.FilterDirectory.CreateOutput(name, ctx)
}

// Compile-time assertion that LockValidatingDirectoryWrapper satisfies
// Directory.
var _ Directory = (*LockValidatingDirectoryWrapper)(nil)
