// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

func newStringField(t *testing.T, name, value string, stored bool) document.IndexableField {
	t.Helper()
	f, err := document.NewStringField(name, value, stored)
	if err != nil {
		t.Fatalf("NewStringField(%q): %v", name, err)
	}
	return f
}

// TestIndexWriterFromReader ports org.apache.lucene.index.TestIndexWriterFromReader.
//
// Every case in the upstream suite opens an IndexWriter from a commit pinned by
// a reader, via IndexWriterConfig.setIndexCommit, and several pull a near-real-time
// reader directly from the writer (DirectoryReader.open(IndexWriter)).
//
// The GetReader / OpenDirectoryReaderFromWriter / IndexWriterConfig.SetIndexCommit
// APIs are now present (rmp #1, #2), so the cases that need only NRT-reader open
// plus latest-commit append (testRightAfterCommit, testFromNonNRTReader) and the
// OpenMode.CREATE rejection (testInvalidOpenMode) run here. The remaining cases
// require writer reopen on an older *pinned* commit (rollback) and closed-reader
// liveness, which are tracked by rmp #118 and stay skipped with that reason.

// testRightAfterCommit ports TestIndexWriterFromReader#testRightAfterCommit:
// pull an NRT reader immediately after the writer has committed.
func TestIndexWriterFromReader_RightAfterCommit(t *testing.T) {
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

	// DirectoryReader.open(w) — near-real-time reader from the writer.
	r, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("OpenDirectoryReaderFromWriter: %v", err)
	}
	if got := r.MaxDoc(); got != 1 {
		t.Fatalf("MaxDoc = %d, want 1", got)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	iwc := index.NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	iwc.SetIndexCommit(r.GetIndexCommit())

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
	iwc.SetIndexCommit(r.GetIndexCommit())

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
	dir := store.NewByteBuffersDirectory()
	defer func() { _ = dir.Close() }()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewStandardAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	if err := w.AddDocument(document.NewDocument()); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	r, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("OpenDirectoryReaderFromWriter: %v", err)
	}
	if err := w.Rollback(); err != nil {
		t.Fatalf("Rollback: %v", err)
	}

	iwc := index.NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	iwc.SetIndexCommit(r.GetIndexCommit())
	_, err = index.NewIndexWriter(dir, iwc)
	const want = "cannot use IndexWriterConfig.setIndexCommit() when index has no commit"
	if err == nil || err.Error() != want {
		t.Fatalf("NewIndexWriter error = %v, want %q", err, want)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("reader Close: %v", err)
	}
}

// testAfterCommitThenIndex ports TestIndexWriterFromReader#testAfterCommitThenIndex:
// an NRT reader becomes stale once the writer commits past its commit point.
func TestIndexWriterFromReader_AfterCommitThenIndex(t *testing.T) {
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
	if err := w.AddDocument(document.NewDocument()); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	r, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("OpenDirectoryReaderFromWriter: %v", err)
	}
	if got := r.MaxDoc(); got != 2 {
		t.Fatalf("MaxDoc = %d, want 2", got)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	iwc := index.NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	iwc.SetIndexCommit(r.GetIndexCommit())
	_, err = index.NewIndexWriter(dir, iwc)
	if err == nil || !strings.Contains(err.Error(), "the provided reader is stale") {
		t.Fatalf("NewIndexWriter error = %v, want stale reader", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("reader Close: %v", err)
	}
}

// testNRTRollback ports TestIndexWriterFromReader#testNRTRollback:
// after a commit and a further add, a pre-add NRT reader is stale.
func TestIndexWriterFromReader_NRTRollback(t *testing.T) {
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

	r, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("OpenDirectoryReaderFromWriter: %v", err)
	}
	if got := r.MaxDoc(); got != 1 {
		t.Fatalf("MaxDoc = %d, want 1", got)
	}
	if err := w.AddDocument(document.NewDocument()); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if got := w.GetDocStats().MaxDoc; got != 2 {
		t.Fatalf("writer MaxDoc = %d, want 2", got)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	iwc := index.NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	iwc.SetIndexCommit(r.GetIndexCommit())
	_, err = index.NewIndexWriter(dir, iwc)
	if err == nil || !strings.Contains(err.Error(), "the provided reader is stale") {
		t.Fatalf("NewIndexWriter error = %v, want stale reader", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("reader Close: %v", err)
	}
}

// testRandom ports TestIndexWriterFromReader#testRandom: a randomized sequence of
// adds, deletes, NRT reopens, rollbacks, and commits cross-checked against
// reader/writer doc counts. The full upstream random test requires
// RandomIndexWriter / MockDirectoryWrapper infrastructure that Gocene has not
// yet ported; it stays blocked on that unrelated gap.
func TestIndexWriterFromReader_Random(t *testing.T) {
	t.Fatal("blocked by rmp #118-follow-up: full random test needs RandomIndexWriter and MockDirectoryWrapper infrastructure; commit-pinning/rollback itself is implemented")
}

// testConsistentFieldNumbers ports TestIndexWriterFromReader#testConsistentFieldNumbers:
// field numbers stay consistent when a writer resumes from a pinned commit.
func TestIndexWriterFromReader_ConsistentFieldNumbers(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer func() { _ = dir.Close() }()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewStandardAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	// Empty first commit.
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	doc := document.NewDocument()
	f0, err := document.NewStringField("f0", "foo", false)
	if err != nil {
		t.Fatalf("NewStringField f0: %v", err)
	}
	doc.Add(f0)
	if err := w.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	r, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("OpenDirectoryReaderFromWriter: %v", err)
	}
	if got := r.MaxDoc(); got != 1 {
		t.Fatalf("r MaxDoc = %d, want 1", got)
	}

	doc2 := document.NewDocument()
	f1, err := document.NewStringField("f1", "foo", false)
	if err != nil {
		t.Fatalf("NewStringField f1: %v", err)
	}
	doc2.Add(f1)
	if err := w.AddDocument(doc2); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	r2, err := index.OpenIfChangedFromWriter(r, w)
	if err != nil {
		t.Fatalf("OpenIfChangedFromWriter: %v", err)
	}
	if r2 == nil {
		t.Fatal("OpenIfChangedFromWriter returned nil, want new reader")
	}
	if got := r2.MaxDoc(); got != 2 {
		t.Fatalf("r2 MaxDoc = %d, want 2", got)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("r Close: %v", err)
	}
	if err := w.Rollback(); err != nil {
		t.Fatalf("Rollback: %v", err)
	}

	iwc := index.NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	iwc.SetIndexCommit(r2.GetIndexCommit())
	w2, err := index.NewIndexWriter(dir, iwc)
	if err != nil {
		t.Fatalf("NewIndexWriter (w2): %v", err)
	}
	if err := r2.Close(); err != nil {
		t.Fatalf("r2 Close: %v", err)
	}

	doc3 := document.NewDocument()
	f1b, err := document.NewStringField("f1", "foo", false)
	if err != nil {
		t.Fatalf("NewStringField f1b: %v", err)
	}
	doc3.Add(f1b)
	f0b, err := document.NewStringField("f0", "foo", false)
	if err != nil {
		t.Fatalf("NewStringField f0b: %v", err)
	}
	doc3.Add(f0b)
	if err := w2.AddDocument(doc3); err != nil {
		t.Fatalf("AddDocument (w2): %v", err)
	}
	if err := w2.Close(); err != nil {
		t.Fatalf("Close (w2): %v", err)
	}
}

// testInvalidOpenMode ports TestIndexWriterFromReader#testInvalidOpenMode:
// setIndexCommit combined with OpenMode.CREATE must be rejected.
func TestIndexWriterFromReader_InvalidOpenMode(t *testing.T) {
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

	r, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("OpenDirectoryReaderFromWriter: %v", err)
	}
	if got := r.MaxDoc(); got != 1 {
		t.Fatalf("MaxDoc = %d, want 1", got)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	iwc := index.NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	iwc.SetOpenMode(index.CREATE)
	iwc.SetIndexCommit(r.GetIndexCommit())

	_, err = index.NewIndexWriter(dir, iwc)
	const want = "cannot use IndexWriterConfig.setIndexCommit() with OpenMode.CREATE"
	if err == nil || err.Error() != want {
		t.Fatalf("NewIndexWriter error = %v, want %q", err, want)
	}

	if err := r.Close(); err != nil {
		t.Fatalf("reader Close: %v", err)
	}
}

// testOnClosedReader ports TestIndexWriterFromReader#testOnClosedReader:
// pinning a commit from an already-closed reader must fail.
func TestIndexWriterFromReader_OnClosedReader(t *testing.T) {
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

	r, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("OpenDirectoryReaderFromWriter: %v", err)
	}
	if got := r.MaxDoc(); got != 1 {
		t.Fatalf("MaxDoc = %d, want 1", got)
	}
	commit := r.GetIndexCommit()
	if err := r.Close(); err != nil {
		t.Fatalf("reader Close: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	iwc := index.NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	iwc.SetIndexCommit(commit)
	_, err = index.NewIndexWriter(dir, iwc)
	var ace *index.AlreadyClosedException
	if err == nil || !errors.As(err, &ace) {
		t.Fatalf("NewIndexWriter error = %v, want AlreadyClosedException", err)
	}
}

// testStaleNRTReader ports TestIndexWriterFromReader#testStaleNRTReader:
// a writer reopened from a stale NRT reader's commit sees the pinned doc count.
func TestIndexWriterFromReader_StaleNRTReader(t *testing.T) {
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

	r, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("OpenDirectoryReaderFromWriter: %v", err)
	}
	if got := r.MaxDoc(); got != 1 {
		t.Fatalf("r MaxDoc = %d, want 1", got)
	}
	if err := w.AddDocument(document.NewDocument()); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	r2, err := index.OpenIfChangedFromWriter(r, w)
	if err != nil {
		t.Fatalf("OpenIfChangedFromWriter: %v", err)
	}
	if r2 == nil {
		t.Fatal("OpenIfChangedFromWriter returned nil, want new reader")
	}
	if got := r2.MaxDoc(); got != 2 {
		t.Fatalf("r2 MaxDoc = %d, want 2", got)
	}
	if err := r2.Close(); err != nil {
		t.Fatalf("r2 Close: %v", err)
	}
	if err := w.Rollback(); err != nil {
		t.Fatalf("Rollback: %v", err)
	}

	iwc := index.NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	iwc.SetIndexCommit(r.GetIndexCommit())
	w2, err := index.NewIndexWriter(dir, iwc)
	if err != nil {
		t.Fatalf("NewIndexWriter (w2): %v", err)
	}
	if got := w2.GetDocStats().NumDocs; got != 1 {
		t.Fatalf("w2 NumDocs = %d, want 1", got)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("r Close: %v", err)
	}

	r3, err := index.OpenDirectoryReaderFromWriter(w2)
	if err != nil {
		t.Fatalf("OpenDirectoryReaderFromWriter (r3): %v", err)
	}
	if got := r3.NumDocs(); got != 1 {
		t.Fatalf("r3 NumDocs = %d, want 1", got)
	}
	if err := r3.Close(); err != nil {
		t.Fatalf("r3 Close: %v", err)
	}

	if err := w2.AddDocument(document.NewDocument()); err != nil {
		t.Fatalf("AddDocument (w2): %v", err)
	}
	r4, err := index.OpenIfChangedFromWriter(r3, w2)
	if err != nil {
		t.Fatalf("OpenIfChangedFromWriter (r4): %v", err)
	}
	if r4 == nil {
		t.Fatal("OpenIfChangedFromWriter returned nil, want new reader")
	}
	if got := r4.NumDocs(); got != 2 {
		t.Fatalf("r4 NumDocs = %d, want 2", got)
	}
	if err := r4.Close(); err != nil {
		t.Fatalf("r4 Close: %v", err)
	}
	if err := w2.Close(); err != nil {
		t.Fatalf("Close (w2): %v", err)
	}
}

// testAfterRollback ports TestIndexWriterFromReader#testAfterRollback:
// after a rollback, a writer reopened from the NRT reader's commit keeps its docs.
func TestIndexWriterFromReader_AfterRollback(t *testing.T) {
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
	if err := w.AddDocument(document.NewDocument()); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	r, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("OpenDirectoryReaderFromWriter: %v", err)
	}
	if got := r.MaxDoc(); got != 2 {
		t.Fatalf("r MaxDoc = %d, want 2", got)
	}
	if err := w.Rollback(); err != nil {
		t.Fatalf("Rollback: %v", err)
	}

	iwc := index.NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	iwc.SetIndexCommit(r.GetIndexCommit())
	w2, err := index.NewIndexWriter(dir, iwc)
	if err != nil {
		t.Fatalf("NewIndexWriter (w2): %v", err)
	}
	if got := w2.GetDocStats().NumDocs; got != 2 {
		t.Fatalf("w2 NumDocs = %d, want 2", got)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("r Close: %v", err)
	}
	if err := w2.Close(); err != nil {
		t.Fatalf("Close (w2): %v", err)
	}

	r2, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader (r2): %v", err)
	}
	if got := r2.NumDocs(); got != 2 {
		t.Fatalf("r2 NumDocs = %d, want 2", got)
	}
	if err := r2.Close(); err != nil {
		t.Fatalf("r2 Close: %v", err)
	}
}

// testAfterCommitThenIndexKeepCommits ports
// TestIndexWriterFromReader#testAfterCommitThenIndexKeepCommits: with a
// keep-all-commits deletion policy, an NRT reader is never stale.
func TestIndexWriterFromReader_AfterCommitThenIndexKeepCommits(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer func() { _ = dir.Close() }()

	iwc := index.NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	iwc.SetIndexDeletionPolicy(index.NewKeepAllDeletionPolicy())
	w, err := index.NewIndexWriter(dir, iwc)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	if err := w.AddDocument(document.NewDocument()); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := w.AddDocument(document.NewDocument()); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	r, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("OpenDirectoryReaderFromWriter: %v", err)
	}
	if got := r.MaxDoc(); got != 2 {
		t.Fatalf("r MaxDoc = %d, want 2", got)
	}
	if err := w.AddDocument(document.NewDocument()); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	r2, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("OpenDirectoryReaderFromWriter (r2): %v", err)
	}
	if got := r2.MaxDoc(); got != 3 {
		t.Fatalf("r2 MaxDoc = %d, want 3", got)
	}
	if err := r2.Close(); err != nil {
		t.Fatalf("r2 Close: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	iwc2 := index.NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	iwc2.SetIndexCommit(r.GetIndexCommit())
	w2, err := index.NewIndexWriter(dir, iwc2)
	if err != nil {
		t.Fatalf("NewIndexWriter (w2): %v", err)
	}
	if got := w2.GetDocStats().MaxDoc; got != 2 {
		t.Fatalf("w2 MaxDoc = %d, want 2", got)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("r Close: %v", err)
	}
	if err := w2.Close(); err != nil {
		t.Fatalf("Close (w2): %v", err)
	}
}
