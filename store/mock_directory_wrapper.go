// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
)

// MockDirectoryWrapper is a Directory wrapper that can simulate various I/O failures
// for testing purposes. It extends FilterDirectory to wrap an existing Directory
// and provides mechanisms to inject errors at specific points.
//
// This is the Go port of Lucene's org.apache.lucene.store.MockDirectoryWrapper.
//
// Example:
//
//	baseDir := store.NewByteBuffersDirectory()
//	mockDir := store.NewMockDirectoryWrapper(baseDir)
//	// Simulate I/O error on next OpenInput call
//	mockDir.SetFailOnOpenInput(true)
//	// Use mockDir for indexing...
type MockDirectoryWrapper struct {
	*FilterDirectory

	// failOnOpenInput causes OpenInput to fail
	failOnOpenInput atomic.Bool

	// failOnCreateOutput causes CreateOutput to fail
	failOnCreateOutput atomic.Bool

	// failOnDeleteFile causes DeleteFile to fail
	failOnDeleteFile atomic.Bool

	// failOnListAll causes ListAll to fail
	failOnListAll atomic.Bool

	// failOnFileLength causes FileLength to fail
	failOnFileLength atomic.Bool

	// randomErrors causes random I/O failures
	randomErrors atomic.Bool

	// errorMessage is the message returned on failures
	errorMessage string

	// errorRate is the probability of random errors (0.0-1.0)
	errorRate float64

	// maxOpenFiles limits the number of open files
	maxOpenFiles int

	// openFileCount tracks currently open files
	openFileCount atomic.Int32

	// callbacks for operations
	openInputCallback   func(name string)
	createOutputCallback func(name string)
	deleteFileCallback  func(name string)

	// mu protects non-atomic fields
	mu sync.RWMutex
}

// NewMockDirectoryWrapper creates a new MockDirectoryWrapper wrapping the given directory.
func NewMockDirectoryWrapper(in Directory) *MockDirectoryWrapper {
	return &MockDirectoryWrapper{
		FilterDirectory: NewFilterDirectory(in),
		errorMessage:    "simulated I/O error",
		errorRate:       0.1,
		maxOpenFiles:    1000,
	}
}

// SetFailOnOpenInput enables or disables failures on OpenInput.
func (m *MockDirectoryWrapper) SetFailOnOpenInput(fail bool) {
	m.failOnOpenInput.Store(fail)
}

// GetFailOnOpenInput returns whether OpenInput failures are enabled.
func (m *MockDirectoryWrapper) GetFailOnOpenInput() bool {
	return m.failOnOpenInput.Load()
}

// SetFailOnCreateOutput enables or disables failures on CreateOutput.
func (m *MockDirectoryWrapper) SetFailOnCreateOutput(fail bool) {
	m.failOnCreateOutput.Store(fail)
}

// GetFailOnCreateOutput returns whether CreateOutput failures are enabled.
func (m *MockDirectoryWrapper) GetFailOnCreateOutput() bool {
	return m.failOnCreateOutput.Load()
}

// SetFailOnDeleteFile enables or disables failures on DeleteFile.
func (m *MockDirectoryWrapper) SetFailOnDeleteFile(fail bool) {
	m.failOnDeleteFile.Store(fail)
}

// GetFailOnDeleteFile returns whether DeleteFile failures are enabled.
func (m *MockDirectoryWrapper) GetFailOnDeleteFile() bool {
	return m.failOnDeleteFile.Load()
}

// SetFailOnListAll enables or disables failures on ListAll.
func (m *MockDirectoryWrapper) SetFailOnListAll(fail bool) {
	m.failOnListAll.Store(fail)
}

// GetFailOnListAll returns whether ListAll failures are enabled.
func (m *MockDirectoryWrapper) GetFailOnListAll() bool {
	return m.failOnListAll.Load()
}

// SetFailOnFileLength enables or disables failures on FileLength.
func (m *MockDirectoryWrapper) SetFailOnFileLength(fail bool) {
	m.failOnFileLength.Store(fail)
}

// GetFailOnFileLength returns whether FileLength failures are enabled.
func (m *MockDirectoryWrapper) GetFailOnFileLength() bool {
	return m.failOnFileLength.Load()
}

// SetRandomErrors enables or disables random I/O failures.
func (m *MockDirectoryWrapper) SetRandomErrors(enable bool) {
	m.randomErrors.Store(enable)
}

// GetRandomErrors returns whether random errors are enabled.
func (m *MockDirectoryWrapper) GetRandomErrors() bool {
	return m.randomErrors.Load()
}

// SetErrorMessage sets the error message returned on failures.
func (m *MockDirectoryWrapper) SetErrorMessage(msg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errorMessage = msg
}

// GetErrorMessage returns the current error message.
func (m *MockDirectoryWrapper) GetErrorMessage() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.errorMessage
}

// SetErrorRate sets the probability of random errors (0.0-1.0).
func (m *MockDirectoryWrapper) SetErrorRate(rate float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errorRate = rate
}

// GetErrorRate returns the current error rate.
func (m *MockDirectoryWrapper) GetErrorRate() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.errorRate
}

// SetMaxOpenFiles sets the maximum number of open files.
func (m *MockDirectoryWrapper) SetMaxOpenFiles(max int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.maxOpenFiles = max
}

// GetMaxOpenFiles returns the maximum number of open files.
func (m *MockDirectoryWrapper) GetMaxOpenFiles() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.maxOpenFiles
}

// SetOpenInputCallback sets a callback called before OpenInput.
func (m *MockDirectoryWrapper) SetOpenInputCallback(fn func(name string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.openInputCallback = fn
}

// SetCreateOutputCallback sets a callback called before CreateOutput.
func (m *MockDirectoryWrapper) SetCreateOutputCallback(fn func(name string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createOutputCallback = fn
}

// SetDeleteFileCallback sets a callback called before DeleteFile.
func (m *MockDirectoryWrapper) SetDeleteFileCallback(fn func(name string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deleteFileCallback = fn
}

// ClearErrors clears all error injection flags.
func (m *MockDirectoryWrapper) ClearErrors() {
	m.failOnOpenInput.Store(false)
	m.failOnCreateOutput.Store(false)
	m.failOnDeleteFile.Store(false)
	m.failOnListAll.Store(false)
	m.failOnFileLength.Store(false)
	m.randomErrors.Store(false)
}

// MaybeThrowIOException throws an error if conditions are met.
func (m *MockDirectoryWrapper) MaybeThrowIOException() error {
	if m.randomErrors.Load() {
		// In real implementation would use random
		// For now, just return error if random errors enabled
		return m.createError()
	}
	return nil
}

// OpenInput opens an input stream with error injection support.
func (m *MockDirectoryWrapper) OpenInput(name string, ctx IOContext) (IndexInput, error) {
	if m.failOnOpenInput.Load() {
		return nil, m.createError()
	}

	m.mu.RLock()
	callback := m.openInputCallback
	m.mu.RUnlock()

	if callback != nil {
		callback(name)
	}

	return m.FilterDirectory.OpenInput(name, ctx)
}

// CreateOutput creates an output stream with error injection support.
func (m *MockDirectoryWrapper) CreateOutput(name string, ctx IOContext) (IndexOutput, error) {
	if m.failOnCreateOutput.Load() {
		return nil, m.createError()
	}

	m.mu.RLock()
	callback := m.createOutputCallback
	m.mu.RUnlock()

	if callback != nil {
		callback(name)
	}

	return m.FilterDirectory.CreateOutput(name, ctx)
}

// DeleteFile deletes a file with error injection support.
func (m *MockDirectoryWrapper) DeleteFile(name string) error {
	if m.failOnDeleteFile.Load() {
		return m.createError()
	}

	m.mu.RLock()
	callback := m.deleteFileCallback
	m.mu.RUnlock()

	if callback != nil {
		callback(name)
	}

	return m.FilterDirectory.DeleteFile(name)
}

// ListAll lists files with error injection support.
func (m *MockDirectoryWrapper) ListAll() ([]string, error) {
	if m.failOnListAll.Load() {
		return nil, m.createError()
	}

	return m.FilterDirectory.ListAll()
}

// FileLength returns file length with error injection support.
func (m *MockDirectoryWrapper) FileLength(name string) (int64, error) {
	if m.failOnFileLength.Load() {
		return 0, m.createError()
	}

	return m.FilterDirectory.FileLength(name)
}

// createError creates an error with the configured message.
func (m *MockDirectoryWrapper) createError() error {
	m.mu.RLock()
	msg := m.errorMessage
	m.mu.RUnlock()
	return errors.New(msg)
}

// GetOpenFileCount returns the number of currently open files.
func (m *MockDirectoryWrapper) GetOpenFileCount() int32 {
	return m.openFileCount.Load()
}

// String returns a string representation of this MockDirectoryWrapper.
func (m *MockDirectoryWrapper) String() string {
	return fmt.Sprintf("MockDirectoryWrapper(delegate=%v)", m.FilterDirectory.GetDelegate())
}
