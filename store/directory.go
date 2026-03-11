// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"errors"
	"fmt"
	"io"
)

// ErrFileNotFound is returned when a file does not exist in the directory.
var ErrFileNotFound = errors.New("file not found")

// ErrFileAlreadyExists is returned when attempting to create a file that already exists.
var ErrFileAlreadyExists = errors.New("file already exists")

// ErrFileIsOpen is returned when attempting to delete a file that is currently open.
var ErrFileIsOpen = errors.New("file is currently open")

// ErrIllegalState is returned when the directory is in an illegal state for the operation.
var ErrIllegalState = errors.New("illegal state")

// Directory is the abstract base class for storing and retrieving index files.
//
// A Directory is a flat list of files. Files may be written once, when they
// are created. Once a file is created, it may only be opened for read, or
// deleted. Random access is permitted when reading, but only sequentially
// when writing.
//
// This is the Go port of Lucene's org.apache.lucene.store.Directory.
type Directory interface {
	// ListAll returns the names of all files in this directory.
	// The returned slice is sorted and may be cached.
	ListAll() ([]string, error)

	// FileExists returns true if a file with the given name exists in this directory.
	FileExists(name string) bool

	// OpenInput returns an IndexInput for reading an existing file.
	// Returns ErrFileNotFound if the file does not exist.
	OpenInput(name string, ctx IOContext) (IndexInput, error)

	// CreateOutput returns an IndexOutput for writing a new file.
	// Returns ErrFileAlreadyExists if the file already exists.
	CreateOutput(name string, ctx IOContext) (IndexOutput, error)

	// DeleteFile deletes a file from the directory.
	// Returns ErrFileNotFound if the file does not exist.
	// Returns ErrFileIsOpen if the file is currently open for reading/writing.
	DeleteFile(name string) error

	// ObtainLock attempts to obtain a lock for the specified name.
	// Returns the Lock instance if successful, or an error if the lock
	// could not be obtained.
	ObtainLock(name string) (Lock, error)

	// Close releases all resources associated with this directory.
	// If the directory is already closed, this returns nil.
	Close() error

	// GetDirectory returns the directory itself (for compatibility with Lucene patterns).
	GetDirectory() Directory
}

// BaseDirectory provides common functionality for Directory implementations.
// Embed this struct in concrete Directory implementations to inherit
// common behavior and helper methods.
type BaseDirectory struct {
	// lockFactory is the factory used to create locks
	lockFactory LockFactory

	// isOpen tracks whether the directory is still open
	isOpen bool

	// openFiles tracks files currently open for reading/writing
	openFiles map[string]int
}

// NewBaseDirectory creates a new BaseDirectory with the given LockFactory.
// If lockFactory is nil, a default NativeFSLockFactory is used.
func NewBaseDirectory(lockFactory LockFactory) *BaseDirectory {
	if lockFactory == nil {
		lockFactory = NewNativeFSLockFactory()
	}
	return &BaseDirectory{
		lockFactory: lockFactory,
		isOpen:      true,
		openFiles:   make(map[string]int),
	}
}

// SetLockFactory sets the LockFactory for this directory.
// This must be called before any locks are obtained.
func (d *BaseDirectory) SetLockFactory(lockFactory LockFactory) error {
	if !d.isOpen {
		return ErrIllegalState
	}
	if lockFactory == nil {
		return errors.New("lockFactory cannot be nil")
	}
	d.lockFactory = lockFactory
	return nil
}

// GetLockFactory returns the LockFactory used by this directory.
func (d *BaseDirectory) GetLockFactory() LockFactory {
	return d.lockFactory
}

// EnsureOpen checks if the directory is open and returns an error if not.
func (d *BaseDirectory) EnsureOpen() error {
	if !d.isOpen {
		return fmt.Errorf("%w: directory is closed", ErrIllegalState)
	}
	return nil
}

// IsOpen returns true if the directory is currently open.
func (d *BaseDirectory) IsOpen() bool {
	return d.isOpen
}

// MarkClosed marks the directory as closed.
func (d *BaseDirectory) MarkClosed() {
	d.isOpen = false
}

// Close releases resources and marks the directory as closed.
func (d *BaseDirectory) Close() error {
	d.MarkClosed()
	return nil
}

// ListAll returns error - must be implemented by subclasses.
func (d *BaseDirectory) ListAll() ([]string, error) {
	return nil, errors.New("ListAll not implemented")
}

// FileExists returns false - must be implemented by subclasses.
func (d *BaseDirectory) FileExists(name string) bool {
	return false
}

// OpenInput returns error - must be implemented by subclasses.
func (d *BaseDirectory) OpenInput(name string, ctx IOContext) (IndexInput, error) {
	return nil, errors.New("OpenInput not implemented")
}

// CreateOutput returns error - must be implemented by subclasses.
func (d *BaseDirectory) CreateOutput(name string, ctx IOContext) (IndexOutput, error) {
	return nil, errors.New("CreateOutput not implemented")
}

// DeleteFile returns error - must be implemented by subclasses.
func (d *BaseDirectory) DeleteFile(name string) error {
	return errors.New("DeleteFile not implemented")
}

// ObtainLock obtains a lock using the configured LockFactory.
func (d *BaseDirectory) ObtainLock(name string) (Lock, error) {
	if d.lockFactory == nil {
		return nil, errors.New("no LockFactory configured")
	}
	return d.lockFactory.ObtainLock(d, name)
}

// GetDirectory returns the directory itself.
func (d *BaseDirectory) GetDirectory() Directory {
	return d
}

// AddOpenFile increments the open file count for the given file.
func (d *BaseDirectory) AddOpenFile(name string) {
	d.openFiles[name]++
}

// RemoveOpenFile decrements the open file count for the given file.
func (d *BaseDirectory) RemoveOpenFile(name string) {
	if count, ok := d.openFiles[name]; ok {
		if count <= 1 {
			delete(d.openFiles, name)
		} else {
			d.openFiles[name] = count - 1
		}
	}
}

// IsFileOpen returns true if the file is currently open.
func (d *BaseDirectory) IsFileOpen(name string) bool {
	count, ok := d.openFiles[name]
	return ok && count > 0
}

// GetOpenFileCount returns the number of open handles for a file.
func (d *BaseDirectory) GetOpenFileCount(name string) int {
	return d.openFiles[name]
}

// GetOpenFiles returns a copy of the open files map.
func (d *BaseDirectory) GetOpenFiles() map[string]int {
	result := make(map[string]int, len(d.openFiles))
	for k, v := range d.openFiles {
		result[k] = v
	}
	return result
}

// Closeable is an interface for types that can be closed.
// This is a subset of io.Closer to avoid importing io where not needed.
type Closeable interface {
	Close() error
}

// EnsureClose is a helper to ensure a Closeable is closed, even if panics occur.
// Use with defer: defer EnsureClose(closer, &err)
func EnsureClose(c io.Closer, err *error) {
	if c != nil {
		if closeErr := c.Close(); closeErr != nil && *err == nil {
			*err = closeErr
		}
	}
}
