// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"
)

// MockDirectoryWrapper is a Directory wrapper that can simulate
// various I/O failures for testing purposes. It extends
// [FilterDirectory] to wrap an existing Directory and provides
// mechanisms to inject errors at OpenInput, CreateOutput, DeleteFile,
// Sync, ListAll and FileLength, as well as track open file handles
// and surface leaks on Close.
//
// This is the Go port of Lucene 10.4.0's
// org.apache.lucene.tests.store.MockDirectoryWrapper.
//
// Injection knobs (all default-off):
//
//   - SetFailOnOpenInput / SetFailOnCreateOutput / SetFailOnDeleteFile /
//     SetFailOnSync / SetFailOnListAll / SetFailOnFileLength: per-method
//     all-or-nothing failure flag.
//   - SetFailOnFile(name): per-method, filename-scoped injection. The
//     wrapper fails the next operation against name on each registered
//     method (OpenInput / CreateOutput / DeleteFile / Sync).
//   - SetFailureRate(rate): probability in [0,1] that any guarded
//     operation will fail. Uses an internal *rand.Rand whose seed is
//     reproducible via SetRandomSeed for deterministic tests.
//
// Open-file tracking decrements on the wrapped IndexInput.Close /
// IndexOutput.Close. Close on the wrapper itself returns
// [ErrUnclosedFiles] when any handle is still open at the time the
// wrapper is closed.
//
// Example:
//
//	base, _ := store.NewSimpleFSDirectory(t.TempDir())
//	mock := store.NewMockDirectoryWrapper(base)
//	mock.SetFailOnFile("segments_1", "openInput")
//	if _, err := mock.OpenInput("segments_1", store.IOContextDefault); err == nil {
//	    t.Fatal("expected failure")
//	}
//
// MockDirectoryWrapper is intended for tests only.
type MockDirectoryWrapper struct {
	*FilterDirectory

	// All-or-nothing per-method failure flags. Kept for back-compat
	// with the pre-Sprint 116 skeleton.
	failOnOpenInput    atomic.Bool
	failOnCreateOutput atomic.Bool
	failOnDeleteFile   atomic.Bool
	failOnSync         atomic.Bool
	failOnListAll      atomic.Bool
	failOnFileLength   atomic.Bool

	// randomErrors enables probabilistic failure on every guarded
	// operation according to failureRate.
	randomErrors atomic.Bool

	// errorMessage is the human-readable message returned by all
	// injected failures.
	errorMessage string

	// failureRate is the probability of random errors (0.0-1.0).
	// Legacy field name "errorRate" preserved through getter/setter.
	failureRate float64

	// rng drives the probabilistic injection. Always non-nil after
	// construction; seedable via SetRandomSeed.
	rng *rand.Rand

	// maxOpenFiles soft-limits the count of open files; exceeding
	// the limit raises an injected error on OpenInput / CreateOutput.
	maxOpenFiles int

	// openFileCount tracks currently open files (incremented on
	// OpenInput/CreateOutput, decremented on the wrapped Close).
	openFileCount atomic.Int32

	// openFiles is the set of file names currently open, keyed by
	// the wrapped handle (an opaque interface value), so multiple
	// open handles on the same file are tracked independently.
	openFiles map[any]string

	// fileFailures maps filename → set of method names that must
	// fail on the next call. A method name is removed once the
	// matching failure fires (matching Lucene's one-shot semantics).
	fileFailures map[string]map[string]struct{}

	// callbacks for operations, retained for back-compat.
	openInputCallback    func(name string)
	createOutputCallback func(name string)
	deleteFileCallback   func(name string)

	// mu protects non-atomic fields.
	mu sync.RWMutex
}

// ErrUnclosedFiles is returned by [MockDirectoryWrapper.Close] when
// one or more files are still open against the wrapper at the time
// Close is invoked.
var ErrUnclosedFiles = errors.New("MockDirectoryWrapper: unclosed file handles at Close")

// NewMockDirectoryWrapper creates a new MockDirectoryWrapper wrapping
// the given directory. The wrapper starts with no failure injection
// enabled and a deterministic *rand.Rand seeded from a fixed value
// (call [MockDirectoryWrapper.SetRandomSeed] before any guarded
// operation to override).
func NewMockDirectoryWrapper(in Directory) *MockDirectoryWrapper {
	return &MockDirectoryWrapper{
		FilterDirectory: NewFilterDirectory(in),
		errorMessage:    "simulated I/O error",
		failureRate:     0.0,
		maxOpenFiles:    0, // 0 = unlimited
		rng:             rand.New(rand.NewSource(0xDEADBEEF)),
		openFiles:       make(map[any]string),
		fileFailures:    make(map[string]map[string]struct{}),
	}
}

// --- Configuration (all-or-nothing flags) ---------------------------

// SetFailOnOpenInput enables or disables failures on OpenInput.
func (m *MockDirectoryWrapper) SetFailOnOpenInput(fail bool) { m.failOnOpenInput.Store(fail) }

// GetFailOnOpenInput returns whether OpenInput failures are enabled.
func (m *MockDirectoryWrapper) GetFailOnOpenInput() bool { return m.failOnOpenInput.Load() }

// SetFailOnCreateOutput enables or disables failures on CreateOutput.
func (m *MockDirectoryWrapper) SetFailOnCreateOutput(fail bool) { m.failOnCreateOutput.Store(fail) }

// GetFailOnCreateOutput returns whether CreateOutput failures are enabled.
func (m *MockDirectoryWrapper) GetFailOnCreateOutput() bool { return m.failOnCreateOutput.Load() }

// SetFailOnDeleteFile enables or disables failures on DeleteFile.
func (m *MockDirectoryWrapper) SetFailOnDeleteFile(fail bool) { m.failOnDeleteFile.Store(fail) }

// GetFailOnDeleteFile returns whether DeleteFile failures are enabled.
func (m *MockDirectoryWrapper) GetFailOnDeleteFile() bool { return m.failOnDeleteFile.Load() }

// SetFailOnSync enables or disables failures on Sync.
func (m *MockDirectoryWrapper) SetFailOnSync(fail bool) { m.failOnSync.Store(fail) }

// GetFailOnSync returns whether Sync failures are enabled.
func (m *MockDirectoryWrapper) GetFailOnSync() bool { return m.failOnSync.Load() }

// SetFailOnListAll enables or disables failures on ListAll.
func (m *MockDirectoryWrapper) SetFailOnListAll(fail bool) { m.failOnListAll.Store(fail) }

// GetFailOnListAll returns whether ListAll failures are enabled.
func (m *MockDirectoryWrapper) GetFailOnListAll() bool { return m.failOnListAll.Load() }

// SetFailOnFileLength enables or disables failures on FileLength.
func (m *MockDirectoryWrapper) SetFailOnFileLength(fail bool) { m.failOnFileLength.Store(fail) }

// GetFailOnFileLength returns whether FileLength failures are enabled.
func (m *MockDirectoryWrapper) GetFailOnFileLength() bool { return m.failOnFileLength.Load() }

// SetRandomErrors enables or disables probabilistic failure injection
// against the configured failure rate. Set the rate with
// [MockDirectoryWrapper.SetFailureRate].
func (m *MockDirectoryWrapper) SetRandomErrors(enable bool) { m.randomErrors.Store(enable) }

// GetRandomErrors returns whether random errors are enabled.
func (m *MockDirectoryWrapper) GetRandomErrors() bool { return m.randomErrors.Load() }

// SetErrorMessage sets the error message returned on injected failures.
func (m *MockDirectoryWrapper) SetErrorMessage(msg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errorMessage = msg
}

// GetErrorMessage returns the current injected-error message.
func (m *MockDirectoryWrapper) GetErrorMessage() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.errorMessage
}

// SetFailureRate sets the probability of random errors (0.0-1.0).
// Values outside the range are clamped.
func (m *MockDirectoryWrapper) SetFailureRate(rate float64) {
	if rate < 0 {
		rate = 0
	}
	if rate > 1 {
		rate = 1
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failureRate = rate
}

// GetFailureRate returns the current random-error rate in [0,1].
func (m *MockDirectoryWrapper) GetFailureRate() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.failureRate
}

// SetErrorRate is a legacy alias for [MockDirectoryWrapper.SetFailureRate].
// Retained for back-compat with the pre-Sprint 116 skeleton.
func (m *MockDirectoryWrapper) SetErrorRate(rate float64) { m.SetFailureRate(rate) }

// GetErrorRate is a legacy alias for [MockDirectoryWrapper.GetFailureRate].
func (m *MockDirectoryWrapper) GetErrorRate() float64 { return m.GetFailureRate() }

// SetRandomSeed reseeds the internal *rand.Rand. Use to make
// probabilistic failure injection deterministic across runs.
func (m *MockDirectoryWrapper) SetRandomSeed(seed int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rng = rand.New(rand.NewSource(seed))
}

// SetMaxOpenFiles sets the maximum number of concurrently open files.
// A value of 0 means unlimited. Exceeding the limit causes the next
// OpenInput / CreateOutput to return an injected error.
func (m *MockDirectoryWrapper) SetMaxOpenFiles(max int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.maxOpenFiles = max
}

// GetMaxOpenFiles returns the maximum-open-files limit (0 = unlimited).
func (m *MockDirectoryWrapper) GetMaxOpenFiles() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.maxOpenFiles
}

// SetOpenInputCallback sets a callback invoked before OpenInput.
func (m *MockDirectoryWrapper) SetOpenInputCallback(fn func(name string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.openInputCallback = fn
}

// SetCreateOutputCallback sets a callback invoked before CreateOutput.
func (m *MockDirectoryWrapper) SetCreateOutputCallback(fn func(name string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createOutputCallback = fn
}

// SetDeleteFileCallback sets a callback invoked before DeleteFile.
func (m *MockDirectoryWrapper) SetDeleteFileCallback(fn func(name string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deleteFileCallback = fn
}

// SetFailOnFile registers a one-shot, per-method, filename-scoped
// failure. The next call to the named method against the named file
// will return an injected error; the entry is then consumed.
//
// Supported method names: "openInput", "createOutput", "deleteFile",
// "sync". Unknown method names are ignored.
//
// Multiple registrations for the same (file, method) pair are
// idempotent (still one-shot).
func (m *MockDirectoryWrapper) SetFailOnFile(name, method string) {
	switch method {
	case "openInput", "createOutput", "deleteFile", "sync":
	default:
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	set, ok := m.fileFailures[name]
	if !ok {
		set = make(map[string]struct{})
		m.fileFailures[name] = set
	}
	set[method] = struct{}{}
}

// ClearErrors clears all injection flags and filename-scoped failures.
func (m *MockDirectoryWrapper) ClearErrors() {
	m.failOnOpenInput.Store(false)
	m.failOnCreateOutput.Store(false)
	m.failOnDeleteFile.Store(false)
	m.failOnSync.Store(false)
	m.failOnListAll.Store(false)
	m.failOnFileLength.Store(false)
	m.randomErrors.Store(false)
	m.mu.Lock()
	m.fileFailures = make(map[string]map[string]struct{})
	m.mu.Unlock()
}

// MaybeThrowIOException probabilistically returns an injected error
// when random injection is enabled. Useful for tests that drive the
// dice manually.
func (m *MockDirectoryWrapper) MaybeThrowIOException() error {
	if !m.randomErrors.Load() {
		return nil
	}
	m.mu.Lock()
	roll := m.rng.Float64()
	rate := m.failureRate
	m.mu.Unlock()
	if roll < rate {
		return m.createError()
	}
	return nil
}

// --- Open-file tracking ---------------------------------------------

// GetOpenFileCount returns the number of currently open file handles.
func (m *MockDirectoryWrapper) GetOpenFileCount() int32 { return m.openFileCount.Load() }

// GetOpenFiles returns a snapshot of the file names currently open.
// Duplicates are possible when the same file is opened multiple times.
func (m *MockDirectoryWrapper) GetOpenFiles() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.openFiles))
	for _, name := range m.openFiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (m *MockDirectoryWrapper) registerOpen(handle any, name string) {
	m.mu.Lock()
	m.openFiles[handle] = name
	m.mu.Unlock()
	m.openFileCount.Add(1)
}

func (m *MockDirectoryWrapper) registerClose(handle any) {
	m.mu.Lock()
	_, present := m.openFiles[handle]
	if present {
		delete(m.openFiles, handle)
	}
	m.mu.Unlock()
	if present {
		m.openFileCount.Add(-1)
	}
}

// --- Guarded operations ---------------------------------------------

// OpenInput opens an input stream against the wrapped directory,
// honouring every configured injection knob.
func (m *MockDirectoryWrapper) OpenInput(name string, ctx IOContext) (IndexInput, error) {
	if err := m.injectFor("openInput", name); err != nil {
		return nil, err
	}
	if m.failOnOpenInput.Load() {
		return nil, m.createError()
	}
	if err := m.checkMaxOpenFiles(); err != nil {
		return nil, err
	}

	m.mu.RLock()
	cb := m.openInputCallback
	m.mu.RUnlock()
	if cb != nil {
		cb(name)
	}

	in, err := m.FilterDirectory.OpenInput(name, ctx)
	if err != nil {
		return nil, err
	}
	wrapped := newMockDirIndexInput(m, in, name)
	m.registerOpen(wrapped, name)
	return wrapped, nil
}

// CreateOutput creates an output stream against the wrapped
// directory, honouring every configured injection knob.
func (m *MockDirectoryWrapper) CreateOutput(name string, ctx IOContext) (IndexOutput, error) {
	if err := m.injectFor("createOutput", name); err != nil {
		return nil, err
	}
	if m.failOnCreateOutput.Load() {
		return nil, m.createError()
	}
	if err := m.checkMaxOpenFiles(); err != nil {
		return nil, err
	}

	m.mu.RLock()
	cb := m.createOutputCallback
	m.mu.RUnlock()
	if cb != nil {
		cb(name)
	}

	out, err := m.FilterDirectory.CreateOutput(name, ctx)
	if err != nil {
		return nil, err
	}
	wrapped := newMockDirIndexOutput(m, out, name)
	m.registerOpen(wrapped, name)
	return wrapped, nil
}

// DeleteFile deletes a file against the wrapped directory, honouring
// every configured injection knob.
func (m *MockDirectoryWrapper) DeleteFile(name string) error {
	if err := m.injectFor("deleteFile", name); err != nil {
		return err
	}
	if m.failOnDeleteFile.Load() {
		return m.createError()
	}

	m.mu.RLock()
	cb := m.deleteFileCallback
	m.mu.RUnlock()
	if cb != nil {
		cb(name)
	}

	return m.FilterDirectory.DeleteFile(name)
}

// Sync fsyncs the listed names on the underlying directory if it
// supports a Sync method. Honours per-name and global Sync injection
// flags.
//
// If the wrapped directory has no Sync method, Sync returns nil
// (matching the Directory contract for in-memory directories), but
// the injection flags are still honoured.
func (m *MockDirectoryWrapper) Sync(names []string) error {
	for _, name := range names {
		if err := m.injectFor("sync", name); err != nil {
			return err
		}
	}
	if m.failOnSync.Load() {
		return m.createError()
	}
	type syncer interface {
		Sync(names []string) error
	}
	if s, ok := m.FilterDirectory.GetDelegate().(syncer); ok {
		return s.Sync(names)
	}
	return nil
}

// ListAll lists files in the wrapped directory.
func (m *MockDirectoryWrapper) ListAll() ([]string, error) {
	if m.failOnListAll.Load() {
		return nil, m.createError()
	}
	if err := m.MaybeThrowIOException(); err != nil {
		return nil, err
	}
	return m.FilterDirectory.ListAll()
}

// FileLength returns the length of a file in the wrapped directory.
func (m *MockDirectoryWrapper) FileLength(name string) (int64, error) {
	if m.failOnFileLength.Load() {
		return 0, m.createError()
	}
	if err := m.MaybeThrowIOException(); err != nil {
		return 0, err
	}
	return m.FilterDirectory.FileLength(name)
}

// Close releases the underlying directory and returns
// [ErrUnclosedFiles] (wrapping the underlying Close error if any) when
// any file handle is still open against this wrapper.
//
// Close is idempotent: subsequent calls after the first return nil.
func (m *MockDirectoryWrapper) Close() error {
	leaks := m.GetOpenFiles()
	underErr := m.FilterDirectory.Close()
	if len(leaks) > 0 {
		if underErr != nil {
			return fmt.Errorf("%w: %v (delegate close: %v)", ErrUnclosedFiles, leaks, underErr)
		}
		return fmt.Errorf("%w: %v", ErrUnclosedFiles, leaks)
	}
	return underErr
}

// --- Internal helpers ----------------------------------------------

// injectFor consumes a one-shot file-scoped failure entry if one is
// registered for (name, method).
func (m *MockDirectoryWrapper) injectFor(method, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	set, ok := m.fileFailures[name]
	if !ok {
		return nil
	}
	if _, hit := set[method]; !hit {
		return nil
	}
	delete(set, method)
	if len(set) == 0 {
		delete(m.fileFailures, name)
	}
	return m.createErrorLocked(name, method)
}

// checkMaxOpenFiles returns an injected error when the open-file
// count is at or above the configured limit. A limit of 0 disables
// the check.
func (m *MockDirectoryWrapper) checkMaxOpenFiles() error {
	m.mu.RLock()
	limit := m.maxOpenFiles
	m.mu.RUnlock()
	if limit <= 0 {
		return nil
	}
	if int(m.openFileCount.Load()) >= limit {
		return fmt.Errorf("%s: maxOpenFiles=%d reached", m.GetErrorMessage(), limit)
	}
	return m.MaybeThrowIOException()
}

// createError builds the canonical injected error.
func (m *MockDirectoryWrapper) createError() error {
	return errors.New(m.GetErrorMessage())
}

// createErrorLocked builds a contextual injected error including the
// failing file name and method. Caller must hold m.mu.
func (m *MockDirectoryWrapper) createErrorLocked(name, method string) error {
	return fmt.Errorf("%s (injected on %s for %q)", m.errorMessage, method, name)
}

// String returns a debug-friendly representation of this wrapper.
func (m *MockDirectoryWrapper) String() string {
	return fmt.Sprintf("MockDirectoryWrapper(delegate=%v, openFiles=%d)",
		m.FilterDirectory.GetDelegate(), m.GetOpenFileCount())
}

// --- Wrapped IndexInput / IndexOutput -------------------------------

// mockDirIndexInput wraps an [IndexInput] so the wrapper's open-file
// counter decrements when Close is invoked. All other methods
// delegate verbatim to the inner input.
type mockDirIndexInput struct {
	IndexInput
	owner  *MockDirectoryWrapper
	name   string
	closed atomic.Bool
}

func newMockDirIndexInput(owner *MockDirectoryWrapper, in IndexInput, name string) *mockDirIndexInput {
	return &mockDirIndexInput{IndexInput: in, owner: owner, name: name}
}

// Close releases the inner input and updates the wrapper's open-file
// tracker. Idempotent: a double Close is a no-op past the first call.
func (m *mockDirIndexInput) Close() error {
	if !m.closed.CompareAndSwap(false, true) {
		return nil
	}
	m.owner.registerClose(m)
	return m.IndexInput.Close()
}

// mockDirIndexOutput wraps an [IndexOutput] so the wrapper's open-file
// counter decrements when Close is invoked.
type mockDirIndexOutput struct {
	IndexOutput
	owner  *MockDirectoryWrapper
	name   string
	closed atomic.Bool
}

func newMockDirIndexOutput(owner *MockDirectoryWrapper, out IndexOutput, name string) *mockDirIndexOutput {
	return &mockDirIndexOutput{IndexOutput: out, owner: owner, name: name}
}

// Close releases the inner output and updates the wrapper's open-file
// tracker. Idempotent.
func (m *mockDirIndexOutput) Close() error {
	if !m.closed.CompareAndSwap(false, true) {
		return nil
	}
	m.owner.registerClose(m)
	return m.IndexOutput.Close()
}
