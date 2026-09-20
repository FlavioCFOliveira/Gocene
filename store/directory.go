// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/FlavioCFOliveira/Gocene/spi"
)

// ErrFileNotFound is an alias for spi.ErrFileNotFound.
var ErrFileNotFound = spi.ErrFileNotFound

// ErrFileAlreadyExists is an alias for spi.ErrFileAlreadyExists.
var ErrFileAlreadyExists = spi.ErrFileAlreadyExists

// ErrFileIsOpen is an alias for spi.ErrFileIsOpen.
var ErrFileIsOpen = spi.ErrFileIsOpen

// ErrIllegalState is an alias for spi.ErrIllegalState.
var ErrIllegalState = spi.ErrIllegalState

// Directory is an alias for spi.Directory.
type Directory = spi.Directory

// IOContext is an alias for spi.IOContext.
type IOContext = spi.IOContext

// IOContextType is an alias for spi.IOContextType.
type IOContextType = spi.IOContextType

// Lock is an alias for spi.Lock.
type Lock = spi.Lock

// LockFactory is an alias for spi.LockFactory.
type LockFactory = spi.LockFactory

// MergeInfo is an alias for spi.MergeInfo.
type MergeInfo = spi.MergeInfo

// FlushInfo is an alias for spi.FlushInfo.
type FlushInfo = spi.FlushInfo

// IOContext constants.
const (
	ContextRead      = spi.ContextRead
	ContextWrite     = spi.ContextWrite
	ContextMerge     = spi.ContextMerge
	ContextFlush     = spi.ContextFlush
	ContextReadOnce  = spi.ContextReadOnce
)

// IOContext values.
var (
	IOContextRead     = spi.IOContextRead
	IOContextWrite    = spi.IOContextWrite
	IOContextReadOnce = spi.IOContextReadOnce
	IOContextDefault  = spi.IOContextDefault
)

// BaseDirectory provides common functionality for Directory implementations.
// Embed this struct in concrete Directory implementations to inherit
// common behavior and helper methods.
type BaseDirectory struct {
	// lockFactory is the factory used to create locks
	lockFactory LockFactory

	// isOpen tracks whether the directory is still open.
	// atomic.Bool ensures reads and writes are safe without a mutex,
	// preventing data races between IsOpen()/EnsureOpen() and Close().
	isOpen atomic.Bool

	// openFiles tracks files currently open for reading/writing
	openFiles map[string]int
	openMu    sync.RWMutex
}

// NewBaseDirectory creates a new BaseDirectory with the given LockFactory.
// If lockFactory is nil, a default NativeFSLockFactory is used.
func NewBaseDirectory(lockFactory LockFactory) *BaseDirectory {
	if lockFactory == nil {
		lockFactory = NewNativeFSLockFactory()
	}
	d := &BaseDirectory{
		lockFactory: lockFactory,
		openFiles:   make(map[string]int),
	}
	d.isOpen.Store(true)
	return d
}

// SetLockFactory sets the LockFactory for this directory.
// This must be called before any locks are obtained.
func (d *BaseDirectory) SetLockFactory(lockFactory LockFactory) error {
	if !d.isOpen.Load() {
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
	if !d.isOpen.Load() {
		return fmt.Errorf("%w: directory is closed", ErrIllegalState)
	}
	return nil
}

// IsOpen returns true if the directory is currently open.
func (d *BaseDirectory) IsOpen() bool {
	return d.isOpen.Load()
}

// MarkClosed marks the directory as closed.
func (d *BaseDirectory) MarkClosed() {
	d.isOpen.Store(false)
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

// FileLength returns error - must be implemented by subclasses.
func (d *BaseDirectory) FileLength(name string) (int64, error) {
	return 0, errors.New("FileLength not implemented")
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

// Rename renames a file from from to to.
func (d *BaseDirectory) Rename(from, to string) error {
	return errors.New("Rename not implemented in BaseDirectory")
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
	d.openMu.Lock()
	defer d.openMu.Unlock()
	d.openFiles[name]++
}

// RemoveOpenFile decrements the open file count for the given file.
func (d *BaseDirectory) RemoveOpenFile(name string) {
	d.openMu.Lock()
	defer d.openMu.Unlock()
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
	d.openMu.RLock()
	defer d.openMu.RUnlock()
	count, ok := d.openFiles[name]
	return ok && count > 0
}

// GetOpenFileCount returns the number of open handles for a file.
func (d *BaseDirectory) GetOpenFileCount(name string) int {
	d.openMu.RLock()
	defer d.openMu.RUnlock()
	return d.openFiles[name]
}

// GetOpenFiles returns a copy of the open files map.
func (d *BaseDirectory) GetOpenFiles() map[string]int {
	d.openMu.RLock()
	defer d.openMu.RUnlock()
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
