// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"fmt"
	"sync"
)

// TrackingDirectoryWrapper wraps a Directory and tracks all file creation
// and deletion operations. This is useful for testing and debugging.
//
// This is the Go port of Lucene's org.apache.lucene.store.TrackingDirectoryWrapper.
//
// TrackingDirectoryWrapper tracks:
//   - Files created (createdFiles map)
//   - Files deleted (deletedFiles map)
//   - Current file sizes (fileSizes map)
//   - Total bytes written and deleted
//
// Example:
//
//	baseDir := store.NewByteBuffersDirectory()
//	trackingDir := store.NewTrackingDirectoryWrapper(baseDir)
//	// ... use trackingDir for indexing ...
//	created := trackingDir.GetCreatedFiles()
//	deleted := trackingDir.GetDeletedFiles()
//	totalWritten := trackingDir.GetTotalBytesWritten()
type TrackingDirectoryWrapper struct {
	*FilterDirectory

	// createdFiles tracks all files that have been created
	createdFiles map[string]int64

	// deletedFiles tracks all files that have been deleted
	deletedFiles map[string]int64

	// fileSizes tracks current file sizes
	fileSizes map[string]int64

	// totalBytesWritten is the total number of bytes written
	totalBytesWritten int64

	// totalBytesDeleted is the total number of bytes deleted
	totalBytesDeleted int64

	// mu protects all tracking fields
	mu sync.RWMutex
}

// NewTrackingDirectoryWrapper creates a new TrackingDirectoryWrapper.
func NewTrackingDirectoryWrapper(in Directory) *TrackingDirectoryWrapper {
	return &TrackingDirectoryWrapper{
		FilterDirectory:   NewFilterDirectory(in),
		createdFiles:      make(map[string]int64),
		deletedFiles:      make(map[string]int64),
		fileSizes:         make(map[string]int64),
		totalBytesWritten: 0,
		totalBytesDeleted: 0,
	}
}

// CreateOutput creates an output and tracks the creation.
func (d *TrackingDirectoryWrapper) CreateOutput(name string, ctx IOContext) (IndexOutput, error) {
	out, err := d.FilterDirectory.CreateOutput(name, ctx)
	if err != nil {
		return nil, err
	}

	d.mu.Lock()
	d.createdFiles[name] = 0 // Will be updated when closed
	d.mu.Unlock()

	// Wrap the output to track bytes written
	return &trackingIndexOutput{
		IndexOutput: out,
		name:        name,
		dir:         d,
	}, nil
}

// DeleteFile deletes a file and tracks the deletion.
func (d *TrackingDirectoryWrapper) DeleteFile(name string) error {
	// Get file size before deletion
	size, _ := d.FilterDirectory.FileLength(name)

	err := d.FilterDirectory.DeleteFile(name)
	if err != nil {
		return err
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// Track deletion
	d.deletedFiles[name] = size
	d.totalBytesDeleted += size
	delete(d.fileSizes, name)
	delete(d.createdFiles, name)

	return nil
}

// GetCreatedFiles returns a copy of the created files map.
func (d *TrackingDirectoryWrapper) GetCreatedFiles() map[string]int64 {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make(map[string]int64, len(d.createdFiles))
	for k, v := range d.createdFiles {
		result[k] = v
	}
	return result
}

// GetDeletedFiles returns a copy of the deleted files map.
func (d *TrackingDirectoryWrapper) GetDeletedFiles() map[string]int64 {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make(map[string]int64, len(d.deletedFiles))
	for k, v := range d.deletedFiles {
		result[k] = v
	}
	return result
}

// GetCreatedFileNames returns a slice of created file names.
func (d *TrackingDirectoryWrapper) GetCreatedFileNames() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	names := make([]string, 0, len(d.createdFiles))
	for name := range d.createdFiles {
		names = append(names, name)
	}
	return names
}

// GetDeletedFileNames returns a slice of deleted file names.
func (d *TrackingDirectoryWrapper) GetDeletedFileNames() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	names := make([]string, 0, len(d.deletedFiles))
	for name := range d.deletedFiles {
		names = append(names, name)
	}
	return names
}

// GetTotalBytesWritten returns the total bytes written.
func (d *TrackingDirectoryWrapper) GetTotalBytesWritten() int64 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.totalBytesWritten
}

// GetTotalBytesDeleted returns the total bytes deleted.
func (d *TrackingDirectoryWrapper) GetTotalBytesDeleted() int64 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.totalBytesDeleted
}

// GetFileSize returns the tracked size of a file.
func (d *TrackingDirectoryWrapper) GetFileSize(name string) int64 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.fileSizes[name]
}

// HasCreatedFile returns true if the file was created through this wrapper.
func (d *TrackingDirectoryWrapper) HasCreatedFile(name string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	_, ok := d.createdFiles[name]
	return ok
}

// HasDeletedFile returns true if the file was deleted through this wrapper.
func (d *TrackingDirectoryWrapper) HasDeletedFile(name string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	_, ok := d.deletedFiles[name]
	return ok
}

// GetCreatedFileCount returns the number of files created.
func (d *TrackingDirectoryWrapper) GetCreatedFileCount() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.createdFiles)
}

// GetDeletedFileCount returns the number of files deleted.
func (d *TrackingDirectoryWrapper) GetDeletedFileCount() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.deletedFiles)
}

// Clear clears all tracking information.
func (d *TrackingDirectoryWrapper) Clear() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.createdFiles = make(map[string]int64)
	d.deletedFiles = make(map[string]int64)
	d.fileSizes = make(map[string]int64)
	d.totalBytesWritten = 0
	d.totalBytesDeleted = 0
}

// String returns a string representation of this TrackingDirectoryWrapper.
func (d *TrackingDirectoryWrapper) String() string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return fmt.Sprintf("TrackingDirectoryWrapper(created=%d, deleted=%d, written=%d, deletedBytes=%d)",
		len(d.createdFiles), len(d.deletedFiles), d.totalBytesWritten, d.totalBytesDeleted)
}

// recordWrite records bytes written for a file.
func (d *TrackingDirectoryWrapper) recordWrite(name string, bytes int64) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.totalBytesWritten += bytes
	d.createdFiles[name] += bytes
	d.fileSizes[name] = d.createdFiles[name]
}

// trackingIndexOutput wraps an IndexOutput to track bytes written.
type trackingIndexOutput struct {
	IndexOutput
	name string
	dir  *TrackingDirectoryWrapper
}

// WriteByte writes a byte and tracks it.
func (t *trackingIndexOutput) WriteByte(b byte) error {
	err := t.IndexOutput.WriteByte(b)
	if err == nil {
		t.dir.recordWrite(t.name, 1)
	}
	return err
}

// WriteBytes writes bytes and tracks them.
func (t *trackingIndexOutput) WriteBytes(b []byte) error {
	err := t.IndexOutput.WriteBytes(b)
	if err == nil {
		t.dir.recordWrite(t.name, int64(len(b)))
	}
	return err
}

// WriteBytesN writes exactly n bytes and tracks them.
func (t *trackingIndexOutput) WriteBytesN(b []byte, n int) error {
	err := t.IndexOutput.WriteBytesN(b, n)
	if err == nil {
		t.dir.recordWrite(t.name, int64(n))
	}
	return err
}

// Close closes the output and updates the final size.
func (t *trackingIndexOutput) Close() error {
	return t.IndexOutput.Close()
}
