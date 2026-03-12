// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"fmt"
	"io"
	"os"
	"sync"
)

// IOUtils provides utility methods for I/O operations.
// This is the Go port of Lucene's org.apache.lucene.util.IOUtils.
//
// IOUtils provides helper methods for:
//   - Closing resources safely
//   - Deleting files with exception handling
//   - FSync operations for durability
//   - Applying functions to multiple resources
type IOUtils struct{}

// Close closes the given Closeable, ignoring any errors.
// Use this when you don't care about errors during close.
func Close(c io.Closer) {
	if c != nil {
		_ = c.Close()
	}
}

// CloseWhileHandlingException closes the given Closeable and returns any error.
// The error is wrapped with context about what was being closed.
func CloseWhileHandlingException(c io.Closer, name string) error {
	if c == nil {
		return nil
	}
	if err := c.Close(); err != nil {
		return fmt.Errorf("error closing %s: %w", name, err)
	}
	return nil
}

// CloseAll closes all given Closeables, collecting all errors.
// Returns a single error containing all individual errors, or nil if all closed successfully.
func CloseAll(closeables ...io.Closer) error {
	var errs []error
	for _, c := range closeables {
		if c != nil {
			if err := c.Close(); err != nil {
				errs = append(errs, err)
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors closing resources: %v", errs)
	}
	return nil
}

// CloseAllWhileHandlingException closes all given Closeables, ignoring errors.
// This is useful when you need to close multiple resources and don't want
// one close error to prevent closing the others.
func CloseAllWhileHandlingException(closeables ...io.Closer) {
	for _, c := range closeables {
		Close(c)
	}
}

// DeleteFilesIgnoringExceptions deletes the given files, ignoring any errors.
// This is useful for cleanup operations where you don't care if deletion fails.
func DeleteFilesIgnoringExceptions(files ...string) {
	for _, file := range files {
		_ = os.Remove(file)
	}
}

// DeleteFiles deletes the given files, collecting all errors.
// Returns a single error containing all individual errors, or nil if all deletions succeeded.
func DeleteFiles(files ...string) error {
	var errs []error
	for _, file := range files {
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			errs = append(errs, fmt.Errorf("failed to delete %s: %w", file, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors deleting files: %v", errs)
	}
	return nil
}

// FSync syncs the given file to disk, ensuring all data is written.
// This is important for durability guarantees.
func FSync(file *os.File) error {
	if file == nil {
		return nil
	}
	return file.Sync()
}

// FSyncDirectory syncs the given directory to disk.
// On some systems, this ensures that directory entries are persisted.
func FSyncDirectory(dir string) error {
	// Open the directory
	f, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("failed to open directory %s for fsync: %w", dir, err)
	}
	defer f.Close()

	// Sync the directory
	if err := f.Sync(); err != nil {
		return fmt.Errorf("failed to fsync directory %s: %w", dir, err)
	}
	return nil
}

// ApplyToAll applies the given function to all items, collecting errors.
// Returns a single error containing all individual errors, or nil if all succeeded.
func ApplyToAll[T any](items []T, fn func(T) error) error {
	var errs []error
	for _, item := range items {
		if err := fn(item); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors applying function: %v", errs)
	}
	return nil
}

// CloseChan closes the given channel safely.
// This is useful for closing channels that may be accessed concurrently.
func CloseChan[T any](ch chan T) {
	select {
	case <-ch:
		// Already closed
	default:
		close(ch)
	}
}

// SafeClose closes a resource with a recover to catch panics.
// This is useful when closing resources that might panic.
func SafeClose(c io.Closer) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic while closing: %v", r)
		}
	}()
	if c != nil {
		err = c.Close()
	}
	return
}

// ResourcePool manages a pool of closable resources.
// This is useful for managing resources that need to be closed together.
type ResourcePool struct {
	resources []io.Closer
	mu        sync.Mutex
}

// NewResourcePool creates a new ResourcePool.
func NewResourcePool() *ResourcePool {
	return &ResourcePool{
		resources: make([]io.Closer, 0),
	}
}

// Add adds a resource to the pool.
func (p *ResourcePool) Add(c io.Closer) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.resources = append(p.resources, c)
}

// CloseAll closes all resources in the pool.
// Returns an error if any close fails, but attempts to close all resources.
func (p *ResourcePool) CloseAll() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var errs []error
	for _, c := range p.resources {
		if c != nil {
			if err := c.Close(); err != nil {
				errs = append(errs, err)
			}
		}
	}
	p.resources = p.resources[:0] // Clear the slice

	if len(errs) > 0 {
		return fmt.Errorf("errors closing resources: %v", errs)
	}
	return nil
}

// CloseAllWhileHandlingException closes all resources, ignoring errors.
func (p *ResourcePool) CloseAllWhileHandlingException() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, c := range p.resources {
		Close(c)
	}
	p.resources = p.resources[:0] // Clear the slice
}

// Len returns the number of resources in the pool.
func (p *ResourcePool) Len() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.resources)
}

// EnsureOpen panics if the given closer is nil.
// This is useful for defensive programming.
func EnsureOpen(c io.Closer, name string) {
	if c == nil {
		panic(fmt.Sprintf("%s is closed", name))
	}
}

// CheckClosed returns true if the given error indicates a closed resource.
func CheckClosed(err error) bool {
	if err == nil {
		return false
	}
	// Check for common closed error messages
	errStr := err.Error()
	return contains(errStr, "closed") || contains(errStr, "EOF")
}

// contains checks if s contains substr.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
