// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

func newTestDir(t *testing.T) *store.SimpleFSDirectory {
	t.Helper()
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	t.Cleanup(func() { _ = dir.Close() })
	return dir
}

func writeEmptyIndex(t *testing.T, dir *store.SimpleFSDirectory) {
	t.Helper()
	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	cfg.SetUseCompoundFile(false)
	iw, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	if err := iw.Close(); err != nil {
		t.Fatalf("IndexWriter.Close: %v", err)
	}
}

// TestReaderManager_AcquireRelease verifies that Acquire returns a non-nil
// reader and Release succeeds without error.
func TestReaderManager_AcquireRelease(t *testing.T) {
	dir := newTestDir(t)
	writeEmptyIndex(t, dir)

	rm, err := index.NewReaderManagerFromDir(dir)
	if err != nil {
		t.Fatalf("NewReaderManagerFromDir: %v", err)
	}
	defer func() {
		if err := rm.Close(); err != nil {
			t.Errorf("ReaderManager.Close: %v", err)
		}
	}()

	reader, err := rm.Acquire()
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if reader == nil {
		t.Fatal("Acquire returned nil reader")
	}
	if err := rm.Release(reader); err != nil {
		t.Fatalf("Release: %v", err)
	}
}

// TestReaderManager_MaybeRefreshNoOp verifies that MaybeRefresh returns false
// (no refresh performed, OpenIfChanged not yet ported — backlog #2707).
func TestReaderManager_MaybeRefreshNoOp(t *testing.T) {
	dir := newTestDir(t)
	writeEmptyIndex(t, dir)

	rm, err := index.NewReaderManagerFromDir(dir)
	if err != nil {
		t.Fatalf("NewReaderManagerFromDir: %v", err)
	}
	defer rm.Close() //nolint:errcheck // cleanup only

	refreshed, err := rm.MaybeRefresh()
	if err != nil {
		t.Fatalf("MaybeRefresh: %v", err)
	}
	if refreshed {
		t.Fatal("MaybeRefresh must return false until OpenIfChanged is ported (backlog #2707)")
	}
}

// TestReaderManager_FromReader verifies the NewReaderManagerFromReader path.
func TestReaderManager_FromReader(t *testing.T) {
	dir := newTestDir(t)
	writeEmptyIndex(t, dir)

	dr, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}

	rm := index.NewReaderManagerFromReader(dr)
	defer rm.Close() //nolint:errcheck // cleanup only

	reader, err := rm.Acquire()
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if reader == nil {
		t.Fatal("Acquire returned nil reader")
	}
	if err := rm.Release(reader); err != nil {
		t.Fatalf("Release: %v", err)
	}
}

// TestReaderManager_DoubleClose verifies that Close is idempotent.
func TestReaderManager_DoubleClose(t *testing.T) {
	dir := newTestDir(t)
	writeEmptyIndex(t, dir)

	rm, err := index.NewReaderManagerFromDir(dir)
	if err != nil {
		t.Fatalf("NewReaderManagerFromDir: %v", err)
	}
	if err := rm.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	// Second close must not panic or return an error.
	if err := rm.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}
