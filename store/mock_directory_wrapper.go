// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"
)

// MockDirectoryWrapper is a Directory wrapper that can simulate
// various I/O failures for testing purposes. It extends
// [FilterDirectory] to wrap an existing Directory and provides
// mechanisms to inject errors at OpenInput, CreateOutput, DeleteFile,
// Sync, Rename, ListAll and FileLength, as well as track open file
// handles and surface leaks on Close.
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
//   - SetMaxSizeInBytes(max): caps total directory bytes; further writes
//     exceeding the cap return an IO error (disk-full simulation).
//   - SetRandomIOExceptionRate(rate): probability in [0,1] that any
//     write/read operation throws a random IOException.
//   - SetRandomIOExceptionRateOnOpen(rate): probability in [0,1] that any
//     OpenInput or CreateOutput throws a random IOException.
//   - FailOn(failure): register a [Failure] that is evaluated at every
//     potential failure point (writes, opens, sync, etc.).
//   - Crash() / ClearCrash(): simulate a JVM/machine crash by corrupting
//     all unsynced files; further operations are blocked until ClearCrash.
//
// Open-file tracking decrements on the wrapped IndexInput.Close /
// IndexOutput.Close. Close on the wrapper itself returns
// [ErrUnclosedFiles] when any handle is still open at the time the
// wrapper is closed.
//
// MockDirectoryWrapper is intended for tests only.
type MockDirectoryWrapper struct {
	*FilterDirectory

	// --- All-or-nothing per-method failure flags ---
	failOnOpenInput    atomic.Bool
	failOnCreateOutput atomic.Bool
	failOnDeleteFile   atomic.Bool
	failOnSync         atomic.Bool
	failOnListAll      atomic.Bool
	failOnFileLength   atomic.Bool

	// --- Probabilistic failure (randomErrors) ---
	randomErrors atomic.Bool
	errorMessage string
	failureRate  float64
	rng          *rand.Rand

	// --- Max open files limit ---
	maxOpenFiles  int
	openFileCount atomic.Int32
	openFiles     map[any]string // opaque handle -> filename

	// --- Per-file one-shot failures ---
	fileFailures map[string]map[string]struct{}

	// --- Callbacks ---
	openInputCallback    func(name string)
	createOutputCallback func(name string)
	deleteFileCallback   func(name string)

	// --- Disk-full simulation ---
	maxSize     int64 // 0 = unlimited
	maxUsedSize int64

	// --- Disk-usage tracking ---
	trackDiskUsage bool // mirrors MockDirectoryWrapper.setTrackDiskUsage

	// --- Random I/O exception injection ---
	randomIOExceptionRate       float64
	randomIOExceptionRateOnOpen float64

	// --- Crash simulation ---
	crashed         atomic.Bool
	unSyncedFiles   map[string]struct{}
	createdFiles    map[string]struct{}
	openFilesForWrite map[string]struct{}

	// --- Custom failure injection ---
	failures []*Failure

	mu sync.RWMutex
}

// ErrUnclosedFiles is returned by [MockDirectoryWrapper.Close] when
// one or more files are still open against the wrapper at the time
// Close is invoked.
var ErrUnclosedFiles = errors.New("MockDirectoryWrapper: unclosed file handles at Close")

// FakeIOException is a sentinel error type returned by simulated I/O
// failures injected through [MockDirectoryWrapper]. Tests can check for
// it via errors.Is/errors.As to distinguish injected failures from
// real ones. This mirrors Lucene's MockDirectoryWrapper.FakeIOException.
type FakeIOException struct{}

func (FakeIOException) Error() string { return "Fake IOException" }

// Failure is a custom failure-injection object modelled after Lucene's
// MockDirectoryWrapper.Failure. Tests create a Failure, store an eval
// function via SetEval, register it via [MockDirectoryWrapper.FailOn],
// and the eval function is called at every guarded operation (write,
// open, sync, ...).
//
// The DoFail flag gates whether the failure is active; call
// [Failure.SetDoFail] / [Failure.ClearDoFail] to toggle it.
//
// Example:
//
//	f := &store.Failure{}
//	f.SetEval(func(dir *store.MockDirectoryWrapper) error {
//	    return errors.New("custom failure")
//	})
//	f.SetDoFail()
//	mock.FailOn(f)
type Failure struct {
	doFail atomic.Bool
	evalFn func(dir *MockDirectoryWrapper) error
	mu     sync.RWMutex
}

// SetEval sets the callback function that is evaluated at each guarded
// operation. If it returns a non-nil error, the operation fails.
func (f *Failure) SetEval(fn func(dir *MockDirectoryWrapper) error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.evalFn = fn
}

// SetDoFail activates this failure.
func (f *Failure) SetDoFail() { f.doFail.Store(true) }

// ClearDoFail deactivates this failure.
func (f *Failure) ClearDoFail() { f.doFail.Store(false) }

// DoFail returns whether this failure is active.
func (f *Failure) DoFail() bool { return f.doFail.Load() }

// Reset deactivates the failure and returns the receiver (for chaining).
func (f *Failure) Reset() *Failure {
	f.ClearDoFail()
	return f
}

// Eval calls the stored eval function. It is called by
// [MockDirectoryWrapper.maybeThrowDeterministicException] at every
// guarded operation.
func (f *Failure) Eval(dir *MockDirectoryWrapper) error {
	f.mu.RLock()
	fn := f.evalFn
	f.mu.RUnlock()
	if fn == nil {
		return nil
	}
	return fn(dir)
}

// NewMockDirectoryWrapper creates a new MockDirectoryWrapper wrapping
// the given directory. The wrapper starts with no failure injection
// enabled and a deterministic *rand.Rand seeded from a fixed value
// (call [MockDirectoryWrapper.SetRandomSeed] before any guarded
// operation to override).
func NewMockDirectoryWrapper(in Directory) *MockDirectoryWrapper {
	return &MockDirectoryWrapper{
		FilterDirectory:   NewFilterDirectory(in),
		errorMessage:      "simulated I/O error",
		failureRate:       0.0,
		maxOpenFiles:      0, // 0 = unlimited
		rng:               rand.New(rand.NewSource(0xDEADBEEF)),
		openFiles:         make(map[any]string),
		fileFailures:      make(map[string]map[string]struct{}),
		unSyncedFiles:     make(map[string]struct{}),
		createdFiles:      make(map[string]struct{}),
		openFilesForWrite: make(map[string]struct{}),
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

// --- Disk-full simulation ------------------------------------------

// SetMaxSizeInBytes sets the maximum total directory size in bytes.
// A value of 0 (or negative) means unlimited. When the total size of
// all files in the directory exceeds this limit, write operations
// return an error.
func (m *MockDirectoryWrapper) SetMaxSizeInBytes(max int64) {
	if max < 0 {
		max = 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.maxSize = max
}

// GetMaxSizeInBytes returns the maximum directory size in bytes (0 = unlimited).
func (m *MockDirectoryWrapper) GetMaxSizeInBytes() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.maxSize
}

// SetTrackDiskUsage enables or disables tracking of the peak directory size.
// When enabled, every closed output updates maxUsedSize. This is the Go
// analogue of Lucene's MockDirectoryWrapper.setTrackDiskUsage(boolean).
// Tracking is disabled by default.
func (m *MockDirectoryWrapper) SetTrackDiskUsage(track bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.trackDiskUsage = track
}

// GetTrackDiskUsage reports whether peak-directory-size tracking is enabled.
func (m *MockDirectoryWrapper) GetTrackDiskUsage() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.trackDiskUsage
}

// GetMaxUsedSizeInBytes returns the peak storage used (bytes) in this directory.
func (m *MockDirectoryWrapper) GetMaxUsedSizeInBytes() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.maxUsedSize
}

// ResetMaxUsedSizeInBytes resets the peak-used-size tracker to the
// current total size of the directory.
func (m *MockDirectoryWrapper) ResetMaxUsedSizeInBytes() error {
	sz, err := m.sizeInBytes()
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.maxUsedSize = sz
	m.mu.Unlock()
	return nil
}

// sizeInBytes returns the total size in bytes of all files in the
// underlying directory.
func (m *MockDirectoryWrapper) sizeInBytes() (int64, error) {
	names, err := m.FilterDirectory.ListAll()
	if err != nil {
		return 0, err
	}
	var total int64
	for _, name := range names {
		sz, err := m.FilterDirectory.FileLength(name)
		if err != nil {
			return 0, err
		}
		total += sz
	}
	return total, nil
}

// --- Random I/O exception injection ---------------------------------

// SetRandomIOExceptionRate sets the probability (0.0-1.0) that any
// write/read operation on this directory will throw a random IOException.
func (m *MockDirectoryWrapper) SetRandomIOExceptionRate(rate float64) {
	if rate < 0 {
		rate = 0
	}
	if rate > 1 {
		rate = 1
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.randomIOExceptionRate = rate
}

// GetRandomIOExceptionRate returns the current random I/O exception rate.
func (m *MockDirectoryWrapper) GetRandomIOExceptionRate() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.randomIOExceptionRate
}

// SetRandomIOExceptionRateOnOpen sets the probability (0.0-1.0) that
// OpenInput or CreateOutput throw a random IOException.
func (m *MockDirectoryWrapper) SetRandomIOExceptionRateOnOpen(rate float64) {
	if rate < 0 {
		rate = 0
	}
	if rate > 1 {
		rate = 1
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.randomIOExceptionRateOnOpen = rate
}

// GetRandomIOExceptionRateOnOpen returns the current random I/O
// exception rate on open operations.
func (m *MockDirectoryWrapper) GetRandomIOExceptionRateOnOpen() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.randomIOExceptionRateOnOpen
}

// maybeThrowIOException randomly throws an IOException based on the
// configured randomIOExceptionRate (excluding open operations).
func (m *MockDirectoryWrapper) maybeThrowIOException(name string) error {
	m.mu.RLock()
	rate := m.randomIOExceptionRate
	rng := m.rng
	m.mu.RUnlock()
	if rate <= 0 || rng == nil {
		return nil
	}
	m.mu.Lock()
	roll := rng.Float64()
	m.mu.Unlock()
	if roll < rate {
		return FakeIOException{}
	}
	return nil
}

// maybeThrowIOExceptionOnOpen randomly throws an IOException based on
// the configured randomIOExceptionRateOnOpen. This is intended for
// OpenInput and CreateOutput.
func (m *MockDirectoryWrapper) maybeThrowIOExceptionOnOpen(name string) error {
	m.mu.RLock()
	rate := m.randomIOExceptionRateOnOpen
	rng := m.rng
	m.mu.RUnlock()
	if rate <= 0 || rng == nil {
		return nil
	}
	m.mu.Lock()
	roll := rng.Float64()
	m.mu.Unlock()
	if roll < rate {
		return FakeIOException{}
	}
	return nil
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

// --- Crash simulation -----------------------------------------------

// Crash simulates a JVM/machine crash by force-closing all open handles
// and corrupting all unsynced files (deleting and zeroing them).
// After a crash, most operations are blocked until [ClearCrash] is
// called.
func (m *MockDirectoryWrapper) Crash() error {
	m.mu.Lock()
	// Snapshot all unsynced files to corrupt
	toCorrupt := make([]string, 0, len(m.unSyncedFiles))
	for name := range m.unSyncedFiles {
		toCorrupt = append(toCorrupt, name)
	}
	// Close all open file handles
	for handle := range m.openFiles {
		if c, ok := handle.(Closable); ok {
			_ = c.Close()
		}
	}
	m.openFiles = make(map[any]string)
	m.openFilesForWrite = make(map[string]struct{})
	m.mu.Unlock()

	// Corrupt unsynced files (outside the lock to avoid deadlock)
	if len(toCorrupt) > 0 {
		m.corruptFiles(toCorrupt)
	}

	m.mu.Lock()
	m.unSyncedFiles = make(map[string]struct{})
	m.mu.Unlock()
	m.crashed.Store(true)
	return nil
}

// ClearCrash resets the crashed state, allowing operations again.
func (m *MockDirectoryWrapper) ClearCrash() {
	m.crashed.Store(false)
}

// isCrashed returns true if the directory is in crashed state.
func (m *MockDirectoryWrapper) isCrashed() bool {
	return m.crashed.Load()
}

// --- File corruption ------------------------------------------------

// corruptFiles corrupts the given files by applying a random damage
// pattern to each one (delete, zero, truncate, bit-flip, etc.).
// This mirrors Lucene's _corruptFiles logic.
func (m *MockDirectoryWrapper) corruptFiles(files []string) {
	if len(files) == 0 {
		return
	}
	sort.Strings(files)
	for _, name := range files {
		damage := m.rng.Intn(6)

		switch damage {
		case 0:
			// Delete the file entirely
			_ = m.FilterDirectory.DeleteFile(name)

		case 1:
			// Zero out file entirely
			length, err := m.FilterDirectory.FileLength(name)
			if err != nil {
				continue
			}
			_ = m.FilterDirectory.DeleteFile(name)
			out, err := m.FilterDirectory.CreateOutput(name, IOContextDefault)
			if err != nil {
				continue
			}
			zeroes := make([]byte, 256)
			var upto int64
			for upto < length {
				limit := int(math.Min(float64(length-upto), 256))
				_ = out.WriteBytes(zeroes[:limit])
				upto += int64(limit)
			}
			_ = out.Close()

		case 2:
			// Partially truncate the file (keep first half)
			length, err := m.FilterDirectory.FileLength(name)
			if err != nil || length <= 1 {
				continue
			}
			half := length / 2
			in, err := m.FilterDirectory.OpenInput(name, IOContextDefault)
			if err != nil {
				continue
			}
			// Read first half via ReadBytesN
			data, err := in.ReadBytesN(int(half))
			if err != nil {
				_ = in.Close()
				continue
			}
			_ = in.Close()
			// Rewrite with only first half
			_ = m.FilterDirectory.DeleteFile(name)
			out, err := m.FilterDirectory.CreateOutput(name, IOContextDefault)
			if err != nil {
				continue
			}
			_ = out.WriteBytes(data)
			_ = out.Close()

		case 3:
			// File survives intact (no damage)

		case 4:
			// Corrupt one random bit
			length, err := m.FilterDirectory.FileLength(name)
			if err != nil || length <= 0 {
				continue
			}
			byteToCorrupt := m.rng.Int63n(length)
			in, err := m.FilterDirectory.OpenInput(name, IOContextDefault)
			if err != nil {
				continue
			}
			data, err := in.ReadBytesN(int(length))
			if err != nil {
				_ = in.Close()
				continue
			}
			_ = in.Close()
			// Flip one bit in the chosen byte
			bitToFlip := m.rng.Intn(8)
			data[byteToCorrupt] ^= (1 << bitToFlip)
			// Rewrite
			_ = m.FilterDirectory.DeleteFile(name)
			out, err := m.FilterDirectory.CreateOutput(name, IOContextDefault)
			if err != nil {
				continue
			}
			_ = out.WriteBytes(data)
			_ = out.Close()

		case 5:
			// Fully truncate (zero bytes)
			_ = m.FilterDirectory.DeleteFile(name)
			out, err := m.FilterDirectory.CreateOutput(name, IOContextDefault)
			if err != nil {
				continue
			}
			_ = out.Close()
		}
	}
}

// --- Custom Failure mechanism (Lucene's Failure/FailOn) -------------

// FailOn registers a Failure object that is evaluated at every
// potential failure point in the wrapper. The Failure's Eval
// function is called each time, giving it an opportunity to throw
// an IOException.
func (m *MockDirectoryWrapper) FailOn(f *Failure) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failures = append(m.failures, f)
}

// maybeThrowDeterministicException iterates through all registered
// Failure objects and calls their Eval function. If any returns an
// error, the iteration stops and the error is returned.
func (m *MockDirectoryWrapper) maybeThrowDeterministicException() error {
	m.mu.RLock()
	failures := m.failures
	m.mu.RUnlock()
	for _, f := range failures {
		if f.doFail.Load() {
			if err := f.Eval(m); err != nil {
				return err
			}
		}
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
	if m.isCrashed() {
		return nil, fmt.Errorf("cannot openInput after crash (%s)", name)
	}
	if err := m.maybeThrowIOExceptionOnOpen(name); err != nil {
		return nil, err
	}
	if err := m.maybeThrowDeterministicException(); err != nil {
		return nil, err
	}
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
	if m.isCrashed() {
		return nil, fmt.Errorf("cannot createOutput after crash (%s)", name)
	}
	if err := m.maybeThrowIOExceptionOnOpen(name); err != nil {
		return nil, err
	}
	if err := m.maybeThrowDeterministicException(); err != nil {
		return nil, err
	}
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

	m.mu.Lock()
	m.createdFiles[name] = struct{}{}
	m.unSyncedFiles[name] = struct{}{}
	m.openFilesForWrite[name] = struct{}{}
	m.mu.Unlock()
	return wrapped, nil
}

// DeleteFile deletes a file against the wrapped directory, honouring
// every configured injection knob.
func (m *MockDirectoryWrapper) DeleteFile(name string) error {
	if m.isCrashed() {
		return fmt.Errorf("cannot delete after crash (%s)", name)
	}
	if err := m.maybeThrowDeterministicException(); err != nil {
		return err
	}
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

	m.mu.Lock()
	delete(m.createdFiles, name)
	delete(m.unSyncedFiles, name)
	m.mu.Unlock()

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
	if m.isCrashed() {
		return fmt.Errorf("cannot sync after crash")
	}
	for _, name := range names {
		if err := m.maybeThrowDeterministicException(); err != nil {
			return err
		}
		if err := m.injectFor("sync", name); err != nil {
			return err
		}
	}
	if m.failOnSync.Load() {
		return m.createError()
	}
	for _, name := range names {
		if err := m.maybeThrowIOException(name); err != nil {
			return err
		}
	}

	type syncer interface {
		Sync(names []string) error
	}
	if s, ok := m.FilterDirectory.GetDelegate().(syncer); ok {
		if err := s.Sync(names); err != nil {
			return err
		}
	}
	m.mu.Lock()
	for _, name := range names {
		delete(m.unSyncedFiles, name)
	}
	m.mu.Unlock()
	return nil
}

// Rename renames a file from source to dest. Honours crash and
// deterministic failure injection.
func (m *MockDirectoryWrapper) Rename(source string, dest string) error {
	if m.isCrashed() {
		return fmt.Errorf("cannot rename after crash (%s -> %s)", source, dest)
	}
	if err := m.maybeThrowDeterministicException(); err != nil {
		return err
	}

	type renamer interface {
		Rename(source, dest string) error
	}
	r, ok := m.FilterDirectory.GetDelegate().(renamer)
	if !ok {
		return fmt.Errorf("MockDirectoryWrapper: delegate %T does not support Rename", m.FilterDirectory.GetDelegate())
	}

	// Check for open files
	m.mu.RLock()
	_, srcOpen := m.openFilesForWrite[source]
	_, dstOpen := m.openFilesForWrite[dest]
	m.mu.RUnlock()
	_ = srcOpen
	_ = dstOpen

	if err := r.Rename(source, dest); err != nil {
		return err
	}

	m.mu.Lock()
	if _, ok := m.unSyncedFiles[source]; ok {
		delete(m.unSyncedFiles, source)
		m.unSyncedFiles[dest] = struct{}{}
	}
	delete(m.createdFiles, source)
	m.createdFiles[dest] = struct{}{}
	m.mu.Unlock()
	return nil
}

// syncMetaData syncs metadata on the underlying directory.
func (m *MockDirectoryWrapper) syncMetaData() error {
	if m.isCrashed() {
		return fmt.Errorf("cannot sync metadata after crash")
	}
	if err := m.maybeThrowDeterministicException(); err != nil {
		return err
	}

	type metaSyncer interface {
		SyncMetaData() error
	}
	if s, ok := m.FilterDirectory.GetDelegate().(metaSyncer); ok {
		return s.SyncMetaData()
	}
	return nil
}

// ListAll lists files in the wrapped directory.
func (m *MockDirectoryWrapper) ListAll() ([]string, error) {
	if err := m.maybeThrowDeterministicException(); err != nil {
		return nil, err
	}
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
	if err := m.maybeThrowDeterministicException(); err != nil {
		return 0, err
	}
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

// checkDiskFull checks whether the directory has exceeded its maxSize
// limit. If so, returns an error simulating a disk-full condition.
func (m *MockDirectoryWrapper) checkDiskFull() error {
	m.mu.RLock()
	max := m.maxSize
	m.mu.RUnlock()
	if max <= 0 {
		return nil
	}
	current, err := m.sizeInBytes()
	if err != nil {
		return err
	}
	if current >= max {
		return m.createError()
	}
	return nil
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

// mockDirIndexOutput wraps an [IndexOutput] so the wrapper can
// intercept writes for disk-full simulation, random I/O exception
// injection, open-file tracking, and max-used-size tracking.
//
// The inner IndexOutput is stored as a named field (not embedded)
// so that all DataOutput methods are implemented explicitly with
// injection checks before delegating.
type mockDirIndexOutput struct {
	inner  IndexOutput
	owner  *MockDirectoryWrapper
	name   string
	closed atomic.Bool
}

func newMockDirIndexOutput(owner *MockDirectoryWrapper, out IndexOutput, name string) *mockDirIndexOutput {
	return &mockDirIndexOutput{inner: out, owner: owner, name: name}
}

// GetName returns the file name of this output.
func (m *mockDirIndexOutput) GetName() string { return m.inner.GetName() }

// GetFilePointer returns the current position in the output.
func (m *mockDirIndexOutput) GetFilePointer() int64 { return m.inner.GetFilePointer() }

// SetPosition sets the file pointer. Not all implementations support this.
func (m *mockDirIndexOutput) SetPosition(pos int64) error { return m.inner.SetPosition(pos) }

// Length returns the total length of the file.
func (m *mockDirIndexOutput) Length() int64 { return m.inner.Length() }

// Close releases the inner output, updates the wrapper's open-file
// tracker, and records the final file size for maxUsedSize tracking.
// Idempotent.
func (m *mockDirIndexOutput) Close() error {
	if !m.closed.CompareAndSwap(false, true) {
		return nil
	}
	m.owner.registerClose(m)
	m.owner.mu.Lock()
	delete(m.owner.openFilesForWrite, m.name)
	m.owner.mu.Unlock()

	// Update maxUsedSize on close (mirrors Lucene's MockRAMOutputStream).
	m.owner.mu.RLock()
	track := m.owner.trackDiskUsage
	m.owner.mu.RUnlock()
	if track {
		if sz, err := m.owner.sizeInBytes(); err == nil {
			m.owner.mu.Lock()
			if sz > m.owner.maxUsedSize {
				m.owner.maxUsedSize = sz
			}
			m.owner.mu.Unlock()
		}
	}
	return m.inner.Close()
}

// WriteByte writes a single byte, checking for disk-full and random
// I/O exceptions first.
func (m *mockDirIndexOutput) WriteByte(b byte) error {
	if err := m.preWrite(); err != nil {
		return err
	}
	return m.inner.WriteByte(b)
}

// WriteBytes writes all bytes from b, checking for disk-full and
// random I/O exceptions first.
func (m *mockDirIndexOutput) WriteBytes(b []byte) error {
	if err := m.preWrite(); err != nil {
		return err
	}
	return m.inner.WriteBytes(b)
}

// WriteBytesN writes exactly len bytes from b, checking for disk-full
// and random I/O exceptions first.
func (m *mockDirIndexOutput) WriteBytesN(b []byte, n int) error {
	if err := m.preWrite(); err != nil {
		return err
	}
	return m.inner.WriteBytesN(b, n)
}

// WriteShort writes a 16-bit value, checking for disk-full first.
func (m *mockDirIndexOutput) WriteShort(i int16) error {
	if err := m.preWrite(); err != nil {
		return err
	}
	return m.inner.WriteShort(i)
}

// WriteInt writes a 32-bit value, checking for disk-full first.
func (m *mockDirIndexOutput) WriteInt(i int32) error {
	if err := m.preWrite(); err != nil {
		return err
	}
	return m.inner.WriteInt(i)
}

// WriteLong writes a 64-bit value, checking for disk-full first.
func (m *mockDirIndexOutput) WriteLong(i int64) error {
	if err := m.preWrite(); err != nil {
		return err
	}
	return m.inner.WriteLong(i)
}

// WriteString writes a string, checking for disk-full first.
func (m *mockDirIndexOutput) WriteString(s string) error {
	if err := m.preWrite(); err != nil {
		return err
	}
	return m.inner.WriteString(s)
}

// WriteVInt writes a variable-length integer, checking for disk-full first.
func (m *mockDirIndexOutput) WriteVInt(i int32) error {
	if err := m.preWrite(); err != nil {
		return err
	}
	return WriteVInt(m.inner, i)
}

// WriteVLong writes a variable-length long, checking for disk-full first.
func (m *mockDirIndexOutput) WriteVLong(i int64) error {
	if err := m.preWrite(); err != nil {
		return err
	}
	return WriteVLong(m.inner, i)
}

// preWrite is called before every write operation. It checks for
// crash state, deterministic failures, disk-full (maxSize), and
// random I/O exceptions.
func (m *mockDirIndexOutput) preWrite() error {
	if m.owner.isCrashed() {
		return fmt.Errorf("cannot write after crash (%s)", m.name)
	}
	if err := m.owner.maybeThrowDeterministicException(); err != nil {
		return err
	}
	if err := m.owner.maybeThrowIOException(m.name); err != nil {
		return err
	}
	if err := m.owner.checkDiskFull(); err != nil {
		return err
	}
	return nil
}
