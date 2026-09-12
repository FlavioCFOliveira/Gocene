// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

import "errors"

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

	// FileLength returns the length of a file in bytes.
	// Returns ErrFileNotFound if the file does not exist.
	FileLength(name string) (int64, error)

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

	// Rename renames a file from from to to.
	// Returns ErrFileNotFound if the from file does not exist.
	Rename(from, to string) error

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
