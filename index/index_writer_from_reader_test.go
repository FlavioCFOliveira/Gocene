// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestIndexWriterFromReader ports org.apache.lucene.index.TestIndexWriterFromReader.
//
// Every case in the upstream suite opens an IndexWriter from a commit pinned by
// a reader, via IndexWriterConfig.setIndexCommit, and several pull a near-real-time
// reader directly from the writer (DirectoryReader.open(IndexWriter)).
//
// Both capabilities are absent in Gocene: IndexWriterConfig exposes no
// SetIndexCommit (index/index_writer_config.go), and no DirectoryReader.open
// variant accepts an IndexWriter. The whole suite is therefore non-runnable and
// each case is skipped, preserving the ported logic for when those APIs land.

// testRightAfterCommit ports TestIndexWriterFromReader#testRightAfterCommit:
// pull an NRT reader immediately after the writer has committed.
func TestIndexWriterFromReader_RightAfterCommit(t *testing.T) {
	t.Skip("blocked: IndexWriterConfig has no SetIndexCommit and DirectoryReader.open does not accept an IndexWriter (NRT reader)")

	dir := store.NewByteBuffersDirectory()
	defer func() { _ = dir.Close() }()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewStandardAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	if err := w.AddDocument(document.NewDocument()); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// DirectoryReader.open(w) — NRT reader from the writer. No Gocene equivalent.
	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	if got := r.MaxDoc(); got != 1 {
		t.Fatalf("MaxDoc = %d, want 1", got)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	iwc := index.NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	// iwc.SetIndexCommit(r.GetIndexCommit()) — no such method on IndexWriterConfig.

	w2, err := index.NewIndexWriter(dir, iwc)
	if err != nil {
		t.Fatalf("NewIndexWriter (w2): %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("reader Close: %v", err)
	}

	if got := w2.GetDocStats().MaxDoc; got != 1 {
		t.Fatalf("w2 maxDoc = %d, want 1", got)
	}
	if err := w2.AddDocument(document.NewDocument()); err != nil {
		t.Fatalf("AddDocument (w2): %v", err)
	}
	if got := w2.GetDocStats().MaxDoc; got != 2 {
		t.Fatalf("w2 maxDoc = %d, want 2", got)
	}
	if err := w2.Close(); err != nil {
		t.Fatalf("Close (w2): %v", err)
	}

	r2, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader (r2): %v", err)
	}
	if got := r2.MaxDoc(); got != 2 {
		t.Fatalf("r2 MaxDoc = %d, want 2", got)
	}
	if err := r2.Close(); err != nil {
		t.Fatalf("Close (r2): %v", err)
	}
}

// testFromNonNRTReader ports TestIndexWriterFromReader#testFromNonNRTReader:
// open a new writer from a commit pinned by a non-NRT directory reader.
func TestIndexWriterFromReader_FromNonNRTReader(t *testing.T) {
	t.Skip("blocked: IndexWriterConfig has no SetIndexCommit (index/index_writer_config.go)")

	dir := store.NewByteBuffersDirectory()
	defer func() { _ = dir.Close() }()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewStandardAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	if err := w.AddDocument(document.NewDocument()); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	if got := r.MaxDoc(); got != 1 {
		t.Fatalf("MaxDoc = %d, want 1", got)
	}
	iwc := index.NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	// iwc.SetIndexCommit(r.GetIndexCommit()) — no such method.

	w2, err := index.NewIndexWriter(dir, iwc)
	if err != nil {
		t.Fatalf("NewIndexWriter (w2): %v", err)
	}
	if got := r.MaxDoc(); got != 1 {
		t.Fatalf("MaxDoc = %d, want 1", got)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("reader Close: %v", err)
	}

	if got := w2.GetDocStats().MaxDoc; got != 1 {
		t.Fatalf("w2 maxDoc = %d, want 1", got)
	}
	if err := w2.AddDocument(document.NewDocument()); err != nil {
		t.Fatalf("AddDocument (w2): %v", err)
	}
	if got := w2.GetDocStats().MaxDoc; got != 2 {
		t.Fatalf("w2 maxDoc = %d, want 2", got)
	}
	if err := w2.Close(); err != nil {
		t.Fatalf("Close (w2): %v", err)
	}

	r2, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader (r2): %v", err)
	}
	if got := r2.MaxDoc(); got != 2 {
		t.Fatalf("r2 MaxDoc = %d, want 2", got)
	}
	if err := r2.Close(); err != nil {
		t.Fatalf("Close (r2): %v", err)
	}
}

// testWithNoFirstCommit ports TestIndexWriterFromReader#testWithNoFirstCommit:
// pinning a commit from a reader of an index with no commit must fail.
func TestIndexWriterFromReader_WithNoFirstCommit(t *testing.T) {
	t.Skip("blocked: IndexWriterConfig has no SetIndexCommit and DirectoryReader.open does not accept an IndexWriter (NRT reader)")
}

// testAfterCommitThenIndex ports TestIndexWriterFromReader#testAfterCommitThenIndex:
// an NRT reader becomes stale once the writer commits past its commit point.
func TestIndexWriterFromReader_AfterCommitThenIndex(t *testing.T) {
	t.Skip("blocked: IndexWriterConfig has no SetIndexCommit and DirectoryReader.open does not accept an IndexWriter (NRT reader)")
}

// testNRTRollback ports TestIndexWriterFromReader#testNRTRollback:
// after a commit and a further add, a pre-add NRT reader is stale.
func TestIndexWriterFromReader_NRTRollback(t *testing.T) {
	t.Skip("blocked: IndexWriterConfig has no SetIndexCommit and DirectoryReader.open does not accept an IndexWriter (NRT reader)")
}

// testRandom ports TestIndexWriterFromReader#testRandom: a randomized sequence of
// adds, deletes, NRT reopens, rollbacks, and commits cross-checked against
// reader/writer doc counts.
func TestIndexWriterFromReader_Random(t *testing.T) {
	t.Skip("blocked: IndexWriterConfig has no SetIndexCommit and DirectoryReader.open does not accept an IndexWriter (NRT reader)")
}

// testConsistentFieldNumbers ports TestIndexWriterFromReader#testConsistentFieldNumbers:
// field numbers stay consistent when a writer resumes from a pinned commit.
func TestIndexWriterFromReader_ConsistentFieldNumbers(t *testing.T) {
	t.Skip("blocked: IndexWriterConfig has no SetIndexCommit and DirectoryReader.open does not accept an IndexWriter (NRT reader)")
}

// testInvalidOpenMode ports TestIndexWriterFromReader#testInvalidOpenMode:
// setIndexCommit combined with OpenMode.CREATE must be rejected.
func TestIndexWriterFromReader_InvalidOpenMode(t *testing.T) {
	t.Skip("blocked: IndexWriterConfig has no SetIndexCommit and DirectoryReader.open does not accept an IndexWriter (NRT reader)")
}

// testOnClosedReader ports TestIndexWriterFromReader#testOnClosedReader:
// pinning a commit from an already-closed reader must fail.
func TestIndexWriterFromReader_OnClosedReader(t *testing.T) {
	t.Skip("blocked: IndexWriterConfig has no SetIndexCommit and DirectoryReader.open does not accept an IndexWriter (NRT reader)")
}

// testStaleNRTReader ports TestIndexWriterFromReader#testStaleNRTReader:
// a writer reopened from a stale NRT reader's commit sees the pinned doc count.
func TestIndexWriterFromReader_StaleNRTReader(t *testing.T) {
	t.Skip("blocked: IndexWriterConfig has no SetIndexCommit and DirectoryReader.open does not accept an IndexWriter (NRT reader)")
}

// testAfterRollback ports TestIndexWriterFromReader#testAfterRollback:
// after a rollback, a writer reopened from the NRT reader's commit keeps its docs.
func TestIndexWriterFromReader_AfterRollback(t *testing.T) {
	t.Skip("blocked: IndexWriterConfig has no SetIndexCommit and DirectoryReader.open does not accept an IndexWriter (NRT reader)")
}

// testAfterCommitThenIndexKeepCommits ports
// TestIndexWriterFromReader#testAfterCommitThenIndexKeepCommits: with a
// keep-all-commits deletion policy, an NRT reader is never stale.
func TestIndexWriterFromReader_AfterCommitThenIndexKeepCommits(t *testing.T) {
	t.Skip("blocked: IndexWriterConfig has no SetIndexCommit and DirectoryReader.open does not accept an IndexWriter (NRT reader)")
}
