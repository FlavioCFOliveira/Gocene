// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package testutil

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestRamCopyOf copies a committed index and verifies the copied directory is
// readable and contains the same documents.
func TestRamCopyOf(t *testing.T) {
	src := store.NewByteBuffersDirectory()
	defer func() { _ = src.Close() }()

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(src, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	doc := document.NewDocument()
	field, err := document.NewTextField("content", "hello world", false)
	if err != nil {
		t.Fatalf("NewTextField: %v", err)
	}
	doc.Add(field)
	if err := w.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	dst, err := RamCopyOf(src)
	if err != nil {
		t.Fatalf("RamCopyOf: %v", err)
	}
	defer func() { _ = dst.Close() }()

	r, err := index.OpenDirectoryReader(dst)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer func() { _ = r.Close() }()
	if r.NumDocs() != 1 {
		t.Fatalf("NumDocs = %d, want 1", r.NumDocs())
	}
}

// TestWrapDirectory verifies that WrapDirectory produces a functional
// MockDirectoryWrapper that can track open files.
func TestWrapDirectory(t *testing.T) {
	base := store.NewByteBuffersDirectory()
	defer func() { _ = base.Close() }()

	wrapped := WrapDirectory(base)
	wrapped.SetAssertNoUnrefencedFilesOnClose(true)
	defer func() {
		if err := wrapped.Close(); err != nil {
			t.Fatalf("Close wrapped directory: %v", err)
		}
	}()

	out, err := wrapped.CreateOutput("test.txt", store.IOContextDefault)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	if err := out.WriteBytes([]byte("payload")); err != nil {
		t.Fatalf("WriteBytes: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close output: %v", err)
	}
}
