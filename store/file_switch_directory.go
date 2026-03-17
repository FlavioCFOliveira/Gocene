// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"path/filepath"
	"strings"
)

// FileSwitchDirectory is a Directory implementation that switches between
// different directories based on file extensions.
//
// This is the Go port of Lucene's org.apache.lucene.store.FileSwitchDirectory.
//
// This is useful for storing different file types in different locations,
// for example, storing large term dictionary files on SSD and other files on HDD.
type FileSwitchDirectory struct {
	*BaseDirectory

	// primaryDir is the primary directory for files not matching extensions
	primaryDir Directory

	// secondaryDir is the secondary directory for files matching extensions
	secondaryDir Directory

	// extensions is the set of file extensions that go to secondaryDir
	extensions map[string]bool
}

// NewFileSwitchDirectory creates a new FileSwitchDirectory.
// Files with extensions in the extensions set are stored in secondaryDir,
// all other files are stored in primaryDir.
func NewFileSwitchDirectory(primaryDir, secondaryDir Directory, extensions []string) *FileSwitchDirectory {
	extMap := make(map[string]bool)
	for _, ext := range extensions {
		// Normalize extension to include the dot
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		extMap[strings.ToLower(ext)] = true
	}

	return &FileSwitchDirectory{
		BaseDirectory: NewBaseDirectory(nil),
		primaryDir:    primaryDir,
		secondaryDir:  secondaryDir,
		extensions:    extMap,
	}
}

// getDirectory returns the appropriate directory for the given file name.
func (d *FileSwitchDirectory) getDirectory(name string) Directory {
	ext := strings.ToLower(filepath.Ext(name))
	if d.extensions[ext] {
		return d.secondaryDir
	}
	return d.primaryDir
}

// ListAll returns the names of all files in both directories.
func (d *FileSwitchDirectory) ListAll() ([]string, error) {
	primaryFiles, err := d.primaryDir.ListAll()
	if err != nil {
		return nil, err
	}

	secondaryFiles, err := d.secondaryDir.ListAll()
	if err != nil {
		return nil, err
	}

	// Combine and sort
	allFiles := make([]string, 0, len(primaryFiles)+len(secondaryFiles))
	allFiles = append(allFiles, primaryFiles...)
	allFiles = append(allFiles, secondaryFiles...)

	return allFiles, nil
}

// FileExists returns true if a file with the given name exists in either directory.
func (d *FileSwitchDirectory) FileExists(name string) bool {
	return d.getDirectory(name).FileExists(name)
}

// FileLength returns the length of a file in bytes.
func (d *FileSwitchDirectory) FileLength(name string) (int64, error) {
	return d.getDirectory(name).FileLength(name)
}

// OpenInput returns an IndexInput for reading an existing file.
func (d *FileSwitchDirectory) OpenInput(name string, ctx IOContext) (IndexInput, error) {
	return d.getDirectory(name).OpenInput(name, ctx)
}

// CreateOutput returns an IndexOutput for writing a new file.
func (d *FileSwitchDirectory) CreateOutput(name string, ctx IOContext) (IndexOutput, error) {
	return d.getDirectory(name).CreateOutput(name, ctx)
}

// DeleteFile deletes a file from the appropriate directory.
func (d *FileSwitchDirectory) DeleteFile(name string) error {
	return d.getDirectory(name).DeleteFile(name)
}

// ObtainLock attempts to obtain a lock from the primary directory.
func (d *FileSwitchDirectory) ObtainLock(name string) (Lock, error) {
	return d.primaryDir.ObtainLock(name)
}

// Close releases resources associated with both directories.
func (d *FileSwitchDirectory) Close() error {
	var primaryErr, secondaryErr error

	if d.primaryDir != nil {
		primaryErr = d.primaryDir.Close()
	}
	if d.secondaryDir != nil {
		secondaryErr = d.secondaryDir.Close()
	}

	if primaryErr != nil {
		return primaryErr
	}
	return secondaryErr
}

// GetDirectory returns the directory itself.
func (d *FileSwitchDirectory) GetDirectory() Directory {
	return d
}

// GetPrimaryDirectory returns the primary directory.
func (d *FileSwitchDirectory) GetPrimaryDirectory() Directory {
	return d.primaryDir
}

// GetSecondaryDirectory returns the secondary directory.
func (d *FileSwitchDirectory) GetSecondaryDirectory() Directory {
	return d.secondaryDir
}

// GetExtensions returns the set of file extensions that go to secondaryDir.
func (d *FileSwitchDirectory) GetExtensions() map[string]bool {
	return d.extensions
}

// Ensure FileSwitchDirectory implements Directory
var _ Directory = (*FileSwitchDirectory)(nil)
