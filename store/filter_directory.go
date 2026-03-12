// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"fmt"
)

// FilterDirectory is a directory implementation that delegates all operations
// to another directory, allowing subclasses to override specific methods.
// This is the decorator pattern applied to Directory.
//
// This is the Go port of Lucene's org.apache.lucene.store.FilterDirectory.
//
// FilterDirectory is useful for:
//   - Adding monitoring or instrumentation to directory operations
//   - Implementing caching layers
//   - Adding encryption or compression
//   - Testing and mocking
//
// Example:
//
//	type MyFilterDirectory struct {
//	    *FilterDirectory
//	}
//
//	func NewMyFilterDirectory(in Directory) *MyFilterDirectory {
//	    return &MyFilterDirectory{
//	        FilterDirectory: NewFilterDirectory(in),
//	    }
//	}
//
//	func (d *MyFilterDirectory) OpenInput(name string, ctx IOContext) (IndexInput, error) {
//	    // Custom logic here
//	    return d.FilterDirectory.OpenInput(name, ctx)
//	}
type FilterDirectory struct {
	in Directory
}

// NewFilterDirectory creates a new FilterDirectory wrapping the given directory.
func NewFilterDirectory(in Directory) *FilterDirectory {
	return &FilterDirectory{in: in}
}

// GetDelegate returns the wrapped directory.
func (d *FilterDirectory) GetDelegate() Directory {
	return d.in
}

// SetDelegate sets the wrapped directory.
func (d *FilterDirectory) SetDelegate(in Directory) {
	d.in = in
}

// ListAll delegates to the wrapped directory.
func (d *FilterDirectory) ListAll() ([]string, error) {
	return d.in.ListAll()
}

// FileExists delegates to the wrapped directory.
func (d *FilterDirectory) FileExists(name string) bool {
	return d.in.FileExists(name)
}

// FileLength delegates to the wrapped directory.
func (d *FilterDirectory) FileLength(name string) (int64, error) {
	return d.in.FileLength(name)
}

// OpenInput delegates to the wrapped directory.
func (d *FilterDirectory) OpenInput(name string, ctx IOContext) (IndexInput, error) {
	return d.in.OpenInput(name, ctx)
}

// CreateOutput delegates to the wrapped directory.
func (d *FilterDirectory) CreateOutput(name string, ctx IOContext) (IndexOutput, error) {
	return d.in.CreateOutput(name, ctx)
}

// DeleteFile delegates to the wrapped directory.
func (d *FilterDirectory) DeleteFile(name string) error {
	return d.in.DeleteFile(name)
}

// ObtainLock delegates to the wrapped directory.
func (d *FilterDirectory) ObtainLock(name string) (Lock, error) {
	return d.in.ObtainLock(name)
}

// Close delegates to the wrapped directory.
func (d *FilterDirectory) Close() error {
	return d.in.Close()
}

// GetDirectory returns this FilterDirectory.
func (d *FilterDirectory) GetDirectory() Directory {
	return d
}

// EnsureOpen checks if the wrapped directory is open.
func (d *FilterDirectory) EnsureOpen() error {
	if d.in == nil {
		return fmt.Errorf("delegate directory is nil")
	}
	return nil
}

// IsOpen returns true if the wrapped directory is open.
func (d *FilterDirectory) IsOpen() bool {
	// Try to check if the underlying directory is open
	// by checking if it has an IsOpen method
	type opener interface {
		IsOpen() bool
	}
	if o, ok := d.in.(opener); ok {
		return o.IsOpen()
	}
	return d.in != nil
}

// String returns a string representation of this FilterDirectory.
func (d *FilterDirectory) String() string {
	return fmt.Sprintf("FilterDirectory(delegate=%v)", d.in)
}
