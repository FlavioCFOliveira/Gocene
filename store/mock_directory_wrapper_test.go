// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"errors"
	"testing"
)

// newBaseDir builds a fresh ByteBuffersDirectory for tests that need
// a live delegate. ByteBuffersDirectory is chosen because it is
// in-memory and avoids touching the filesystem.
func newBaseDir(t *testing.T) Directory {
	t.Helper()
	return NewByteBuffersDirectory()
}

// writeFile creates a file with the given name and content via the
// wrapper. It is used as a fixture in multiple tests.
func writeFile(t *testing.T, dir Directory, name string, content []byte) {
	t.Helper()
	out, err := dir.CreateOutput(name, IOContextDefault)
	if err != nil {
		t.Fatalf("CreateOutput(%q): %v", name, err)
	}
	if err := out.WriteBytes(content); err != nil {
		t.Fatalf("WriteBytes: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("close output(%q): %v", name, err)
	}
}

// TestMockDirectoryWrapper_HappyPath verifies that with no injection
// enabled the wrapper delegates faithfully to its inner directory.
func TestMockDirectoryWrapper_HappyPath(t *testing.T) {
	t.Parallel()

	base := newBaseDir(t)
	mock := NewMockDirectoryWrapper(base)
	defer func() {
		if err := mock.Close(); err != nil {
			t.Errorf("close: unexpected error %v", err)
		}
	}()

	writeFile(t, mock, "alpha", []byte("hello"))

	in, err := mock.OpenInput("alpha", IOContextDefault)
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	buf := make([]byte, 5)
	if err := in.ReadBytes(buf); err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}
	if string(buf) != "hello" {
		t.Errorf("content: got %q, want %q", buf, "hello")
	}
	if err := in.Close(); err != nil {
		t.Errorf("input close: %v", err)
	}

	if mock.GetOpenFileCount() != 0 {
		t.Errorf("open file count after close: got %d, want 0", mock.GetOpenFileCount())
	}
}

// TestMockDirectoryWrapper_InjectedOpenInputFailure verifies that the
// all-or-nothing failure flag fires on OpenInput.
func TestMockDirectoryWrapper_InjectedOpenInputFailure(t *testing.T) {
	t.Parallel()

	base := newBaseDir(t)
	mock := NewMockDirectoryWrapper(base)
	defer mock.Close()

	writeFile(t, mock, "alpha", []byte("x"))

	mock.SetFailOnOpenInput(true)
	if _, err := mock.OpenInput("alpha", IOContextDefault); err == nil {
		t.Fatal("OpenInput: expected injected error, got nil")
	}
	mock.SetFailOnOpenInput(false)
	in, err := mock.OpenInput("alpha", IOContextDefault)
	if err != nil {
		t.Fatalf("OpenInput after clearing flag: %v", err)
	}
	if err := in.Close(); err != nil {
		t.Errorf("input close: %v", err)
	}
}

// TestMockDirectoryWrapper_InjectedCreateOutputFailure verifies the
// CreateOutput injection knob.
func TestMockDirectoryWrapper_InjectedCreateOutputFailure(t *testing.T) {
	t.Parallel()

	base := newBaseDir(t)
	mock := NewMockDirectoryWrapper(base)
	defer mock.Close()

	mock.SetFailOnCreateOutput(true)
	if _, err := mock.CreateOutput("alpha", IOContextDefault); err == nil {
		t.Fatal("CreateOutput: expected injected error, got nil")
	}
}

// TestMockDirectoryWrapper_InjectedDeleteFileFailure verifies the
// DeleteFile injection knob.
func TestMockDirectoryWrapper_InjectedDeleteFileFailure(t *testing.T) {
	t.Parallel()

	base := newBaseDir(t)
	mock := NewMockDirectoryWrapper(base)
	defer mock.Close()

	writeFile(t, mock, "alpha", []byte("x"))

	mock.SetFailOnDeleteFile(true)
	if err := mock.DeleteFile("alpha"); err == nil {
		t.Fatal("DeleteFile: expected injected error, got nil")
	}
	mock.SetFailOnDeleteFile(false)
	if err := mock.DeleteFile("alpha"); err != nil {
		t.Errorf("DeleteFile after clearing: %v", err)
	}
}

// TestMockDirectoryWrapper_InjectedSyncFailure verifies the Sync
// injection knob.
func TestMockDirectoryWrapper_InjectedSyncFailure(t *testing.T) {
	t.Parallel()

	base := newBaseDir(t)
	mock := NewMockDirectoryWrapper(base)
	defer mock.Close()

	writeFile(t, mock, "alpha", []byte("x"))

	mock.SetFailOnSync(true)
	if err := mock.Sync([]string{"alpha"}); err == nil {
		t.Fatal("Sync: expected injected error, got nil")
	}
	mock.SetFailOnSync(false)
	if err := mock.Sync([]string{"alpha"}); err != nil {
		t.Errorf("Sync after clearing flag: %v", err)
	}
}

// TestMockDirectoryWrapper_FailOnFileSpecific verifies the
// filename-scoped, one-shot injection: only the matching file +
// method pair fails, and only once.
func TestMockDirectoryWrapper_FailOnFileSpecific(t *testing.T) {
	t.Parallel()

	base := newBaseDir(t)
	mock := NewMockDirectoryWrapper(base)
	defer mock.Close()

	writeFile(t, mock, "alpha", []byte("a"))
	writeFile(t, mock, "beta", []byte("b"))

	mock.SetFailOnFile("alpha", "openInput")

	// First open of alpha must fail (consumes the one-shot).
	if _, err := mock.OpenInput("alpha", IOContextDefault); err == nil {
		t.Fatal("first OpenInput(alpha): expected injected error, got nil")
	}
	// Open of beta must succeed.
	inBeta, err := mock.OpenInput("beta", IOContextDefault)
	if err != nil {
		t.Fatalf("OpenInput(beta): %v", err)
	}
	if err := inBeta.Close(); err != nil {
		t.Errorf("close beta: %v", err)
	}
	// Second open of alpha must now succeed (one-shot consumed).
	inAlpha, err := mock.OpenInput("alpha", IOContextDefault)
	if err != nil {
		t.Fatalf("second OpenInput(alpha): %v", err)
	}
	if err := inAlpha.Close(); err != nil {
		t.Errorf("close alpha: %v", err)
	}
}

// TestMockDirectoryWrapper_FailureRate_Deterministic verifies that
// SetRandomSeed + SetFailureRate yield reproducible injection.
func TestMockDirectoryWrapper_FailureRate_Deterministic(t *testing.T) {
	t.Parallel()

	run := func() int {
		base := newBaseDir(t)
		mock := NewMockDirectoryWrapper(base)
		// Write the fixture BEFORE enabling random injection, so the
		// determinism check focuses on the dice-driven OpenInput path
		// rather than the fixture's CreateOutput.
		writeFile(t, mock, "alpha", []byte("x"))

		mock.SetRandomSeed(42)
		mock.SetFailureRate(0.5)
		mock.SetRandomErrors(true)
		mock.SetMaxOpenFiles(1024) // trigger MaybeThrow path on OpenInput

		failures := 0
		const trials = 200
		for i := 0; i < trials; i++ {
			in, err := mock.OpenInput("alpha", IOContextDefault)
			if err != nil {
				failures++
				continue
			}
			if err := in.Close(); err != nil {
				t.Fatalf("close: %v", err)
			}
		}
		_ = mock.Close()
		return failures
	}

	a := run()
	b := run()
	if a != b {
		t.Errorf("deterministic seed expected same failure count: a=%d b=%d", a, b)
	}
	if a == 0 {
		t.Errorf("expected at least one injected failure at rate=0.5, got 0")
	}
}

// TestMockDirectoryWrapper_UnclosedHandleSurfacesOnClose verifies the
// open-handle leak check on Close.
func TestMockDirectoryWrapper_UnclosedHandleSurfacesOnClose(t *testing.T) {
	t.Parallel()

	base := newBaseDir(t)
	mock := NewMockDirectoryWrapper(base)

	writeFile(t, mock, "alpha", []byte("x"))

	in, err := mock.OpenInput("alpha", IOContextDefault)
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}

	// Deliberately do NOT close in before closing the wrapper.
	err = mock.Close()
	if err == nil {
		t.Fatal("expected ErrUnclosedFiles, got nil")
	}
	if !errors.Is(err, ErrUnclosedFiles) {
		t.Errorf("expected wrapped ErrUnclosedFiles, got %v", err)
	}

	// Close the input afterwards just for hygiene; double-close on
	// the wrapper must not panic and must not re-report leaks.
	if cerr := in.Close(); cerr != nil {
		t.Errorf("late close input: %v", cerr)
	}
}

// TestMockDirectoryWrapper_OpenFileCountTracksClose verifies the
// open-file counter increments on Open and decrements on Close.
func TestMockDirectoryWrapper_OpenFileCountTracksClose(t *testing.T) {
	t.Parallel()

	base := newBaseDir(t)
	mock := NewMockDirectoryWrapper(base)
	defer mock.Close()

	writeFile(t, mock, "alpha", []byte("x"))

	if got := mock.GetOpenFileCount(); got != 0 {
		t.Fatalf("initial open count: got %d, want 0", got)
	}

	in1, _ := mock.OpenInput("alpha", IOContextDefault)
	in2, _ := mock.OpenInput("alpha", IOContextDefault)
	if got := mock.GetOpenFileCount(); got != 2 {
		t.Fatalf("after two opens: got %d, want 2", got)
	}

	_ = in1.Close()
	if got := mock.GetOpenFileCount(); got != 1 {
		t.Fatalf("after one close: got %d, want 1", got)
	}
	_ = in2.Close()
	if got := mock.GetOpenFileCount(); got != 0 {
		t.Fatalf("after both closes: got %d, want 0", got)
	}

	// Double-close on a wrapped input must be a no-op (no panic, no
	// negative counter).
	_ = in1.Close()
	if got := mock.GetOpenFileCount(); got != 0 {
		t.Errorf("after double-close: got %d, want 0", got)
	}
}

// TestMockDirectoryWrapper_MaxOpenFilesEnforced verifies that
// SetMaxOpenFiles caps the concurrent open count.
func TestMockDirectoryWrapper_MaxOpenFilesEnforced(t *testing.T) {
	t.Parallel()

	base := newBaseDir(t)
	mock := NewMockDirectoryWrapper(base)
	defer mock.Close()

	writeFile(t, mock, "alpha", []byte("x"))
	mock.SetMaxOpenFiles(1)

	in1, err := mock.OpenInput("alpha", IOContextDefault)
	if err != nil {
		t.Fatalf("first OpenInput: %v", err)
	}
	if _, err := mock.OpenInput("alpha", IOContextDefault); err == nil {
		t.Fatal("second OpenInput at limit: expected error, got nil")
	}
	_ = in1.Close()
	in2, err := mock.OpenInput("alpha", IOContextDefault)
	if err != nil {
		t.Fatalf("OpenInput after release: %v", err)
	}
	_ = in2.Close()
}

// TestMockDirectoryWrapper_ClearErrorsResets verifies that
// ClearErrors removes every injection flag and filename-scoped entry.
func TestMockDirectoryWrapper_ClearErrorsResets(t *testing.T) {
	t.Parallel()

	base := newBaseDir(t)
	mock := NewMockDirectoryWrapper(base)
	defer mock.Close()

	writeFile(t, mock, "alpha", []byte("x"))

	mock.SetFailOnOpenInput(true)
	mock.SetFailOnFile("alpha", "openInput")
	mock.ClearErrors()

	in, err := mock.OpenInput("alpha", IOContextDefault)
	if err != nil {
		t.Fatalf("OpenInput after ClearErrors: unexpected error %v", err)
	}
	_ = in.Close()
}
