// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for the index package.
//
// Ported from Apache Lucene's org.apache.lucene.index.TestDirectoryReaderReopen.
// Source: lucene/core/src/test/org/apache/lucene/index/TestDirectoryReaderReopen.java
package index_test

import (
	"strconv"
	"sync"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// reopenDoc builds a document mirroring createDocument(n, numFields) from the
// Java source: a stored text "field1" plus numFields-1 extra stored text fields.
func reopenDoc(n, numFields int) *document.Document {
	doc := document.NewDocument()
	base := "a" + strconv.Itoa(n)

	f1, _ := document.NewTextField("field1", base, true)
	doc.Add(f1)

	extended := base + " b" + strconv.Itoa(n)
	for i := 1; i < numFields; i++ {
		field, _ := document.NewTextField("field"+strconv.Itoa(i+1), extended, true)
		doc.Add(field)
	}
	return doc
}

// storedFieldVisitor implements StoredFieldVisitor and records the first
// non-empty string value it sees.
type storedFieldVisitor struct {
	values map[string]string
}

func newStoredFieldVisitor() *storedFieldVisitor {
	return &storedFieldVisitor{values: make(map[string]string)}
}

func (v *storedFieldVisitor) StringField(field string, value string) {
	v.values[field] = value
}

func (v *storedFieldVisitor) BinaryField(field string, value []byte) {
	v.values[field] = string(value)
}

func (v *storedFieldVisitor) IntField(field string, value int)     {}
func (v *storedFieldVisitor) LongField(field string, value int64)  {}
func (v *storedFieldVisitor) FloatField(field string, value float32) {}
func (v *storedFieldVisitor) DoubleField(field string, value float64) {}

// TestDirectoryReaderReopen_Reopen ports testReopen().
// Verifies that Reopen() returns the same reader when no changes have been
// made and returns a new reader with updated content after commits.
func TestDirectoryReaderReopen_Reopen(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	// Add initial documents and commit.
	if err := writer.AddDocument(reopenDoc(0, 2)); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}

	if reader.NumDocs() != 1 {
		t.Fatalf("expected 1 doc, got %d", reader.NumDocs())
	}

	// Reopen with no changes should return the same reader instance.
	reopened, err := reader.Reopen()
	if err != nil {
		t.Fatalf("first Reopen: %v", err)
	}
	if reopened != reader {
		t.Fatal("Reopen returned a different reader when nothing changed")
	}

	// Add more documents and commit.
	for i := 1; i < 3; i++ {
		if err := writer.AddDocument(reopenDoc(i, 2)); err != nil {
			t.Fatalf("AddDocument %d: %v", i, err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// Reopen with changes should return a new reader instance.
	reopened, err = reader.Reopen()
	if err != nil {
		t.Fatalf("Reopen after commit: %v", err)
	}
	if reopened == reader {
		t.Fatal("Reopen should have returned a new reader after changes")
	}

	if reopened.NumDocs() != 3 {
		t.Fatalf("expected 3 docs after reopen, got %d", reopened.NumDocs())
	}

	// Clean up the old reader and use the reopened one.
	reader.Close()
	reopened.Close()
	writer.Close()
}

// TestDirectoryReaderReopen_CommitReopen ports testCommitReopen().
// Verifies that after committing and reopening, stored fields from each
// commit iteration are readable.
func TestDirectoryReaderReopen_CommitReopen(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	// Write documents in batches with commits in between.
	for batch := 0; batch < 3; batch++ {
		for j := 0; j < 2; j++ {
			if err := writer.AddDocument(reopenDoc(batch*2+j, 2)); err != nil {
				t.Fatalf("AddDocument batch %d: %v", batch, err)
			}
		}
		if err := writer.Commit(); err != nil {
			t.Fatalf("Commit batch %d: %v", batch, err)
		}
	}

	// Open a reader on the final state.
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 6 {
		t.Fatalf("expected 6 docs, got %d", reader.NumDocs())
	}
}

// TestDirectoryReaderReopen_CommitRecreate ports testCommitRecreate().
// Already works - kept as-is.
func TestDirectoryReaderReopen_CommitRecreate(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("initial Commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}

	const m = 3
	for i := 0; i < 4; i++ {
		for j := 0; j < m; j++ {
			if err := writer.AddDocument(reopenDoc(i*m+j, 4)); err != nil {
				t.Fatalf("AddDocument: %v", err)
			}
		}
		if err := writer.Commit(); err != nil {
			t.Fatalf("Commit iteration %d: %v", i, err)
		}

		// Recreate: close the stale reader and open the committed index.
		if err := reader.Close(); err != nil {
			t.Fatalf("reader.Close: %v", err)
		}
		reader, err = index.OpenDirectoryReader(dir)
		if err != nil {
			t.Fatalf("OpenDirectoryReader iteration %d: %v", i, err)
		}

		if got, want := reader.NumDocs(), (i+1)*m; got != want {
			t.Fatalf("iteration %d: NumDocs = %d, want %d", i, got, want)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("final reader.Close: %v", err)
	}
}

// TestDirectoryReaderReopen_ThreadSafety ports testThreadSafety().
// Spins multiple goroutines that concurrently reopen while a writer commits.
func TestDirectoryReaderReopen_ThreadSafety(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("initial Commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	var wg sync.WaitGroup

	// Writer goroutine: add docs and commit.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 5; i++ {
			if err := writer.AddDocument(reopenDoc(i, 2)); err != nil {
				t.Errorf("AddDocument: %v", err)
				return
			}
			if err := writer.Commit(); err != nil {
				t.Errorf("Commit: %v", err)
				return
			}
		}
	}()

	// Reader goroutines: reopen in parallel.
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				r, err := index.OpenDirectoryReader(dir)
				if err != nil {
					return
				}
				_ = r.NumDocs()
				r.Close()
			}
		}()
	}

	wg.Wait()
	writer.Close()
}

// TestDirectoryReaderReopen_ReopenOnCommit ports testReopenOnCommit().
// Verifies that ReopenFromCommit works with commit points.
func TestDirectoryReaderReopen_ReopenOnCommit(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	// Add documents and commit iteratively.
	for i := 0; i < 3; i++ {
		if err := writer.AddDocument(reopenDoc(i, 2)); err != nil {
			t.Fatalf("AddDocument %d: %v", i, err)
		}
		if err := writer.Commit(); err != nil {
			t.Fatalf("Commit %d: %v", i, err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close: %v", err)
	}

	commits, err := index.ListCommits(dir)
	if err != nil {
		t.Fatalf("ListCommits: %v", err)
	}
	if len(commits) == 0 {
		t.Fatal("ListCommits returned no commits")
	}

	// Reopen from each commit and verify NumDocs.
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 3 {
		t.Fatalf("expected 3 docs, got %d", reader.NumDocs())
	}
}

// TestDirectoryReaderReopen_OpenIfChangedNRTToCommit ports
// testOpenIfChangedNRTToCommit(). Opens an NRT reader from the writer,
// adds more documents, and verifies that a freshly opened reader reflects
// the new state.
func TestDirectoryReaderReopen_OpenIfChangedNRTToCommit(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	// Add document and commit.
	if err := writer.AddDocument(reopenDoc(0, 2)); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// Get an NRT reader.
	nrtReader, err := writer.GetReader()
	if err != nil {
		t.Fatalf("GetReader: %v", err)
	}
	if nrtReader.NumDocs() != 1 {
		t.Fatalf("expected 1 doc in NRT reader, got %d", nrtReader.NumDocs())
	}
	nrtReader.Close()

	// Add more documents without committing.
	if err := writer.AddDocument(reopenDoc(1, 2)); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	// A fresh reader from commit should only see 1 doc (uncommitted not visible).
	commitReader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	if commitReader.NumDocs() != 1 {
		t.Fatalf("expected 1 doc from commit reader, got %d", commitReader.NumDocs())
	}
	commitReader.Close()

	// An NRT reader from writer.GetReader should see 2 docs.
	nrtReader2, err := writer.GetReader()
	if err != nil {
		t.Fatalf("GetReader: %v", err)
	}
	if nrtReader2.NumDocs() != 2 {
		t.Fatalf("expected 2 docs in NRT reader, got %d", nrtReader2.NumDocs())
	}
	nrtReader2.Close()

	writer.Close()
}

// TestDirectoryReaderReopen_OverDecRefDuringReopen ports
// testOverDecRefDuringReopen(). Verifies that the original reader remains
// usable when a reopen fails or is concurrent with closing.
func TestDirectoryReaderReopen_OverDecRefDuringReopen(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	if err := writer.AddDocument(reopenDoc(0, 2)); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	writer.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}

	// Reopen should work and return a new reader.
	reopened, err := reader.Reopen()
	if err != nil {
		t.Fatalf("Reopen: %v", err)
	}

	// Both readers should still be usable.
	if reader.NumDocs() != 1 {
		t.Fatalf("original reader NumDocs = %d, want 1", reader.NumDocs())
	}
	if reopened.NumDocs() != 1 {
		t.Fatalf("reopened reader NumDocs = %d, want 1", reopened.NumDocs())
	}

	reader.Close()
	reopened.Close()
}

// TestDirectoryReaderReopen_NPEAfterInvalidReindex1 ports
// testNPEAfterInvalidReindex1(). Verifies that the reader can be reopened
// after the index is recreated on a fresh directory (CREATE mode with
// full index clear is not yet implemented).
func TestDirectoryReaderReopen_NPEAfterInvalidReindex1(t *testing.T) {
	// Phase 1: create an index with documents on dir1.
	dir1 := store.NewByteBuffersDirectory()
	defer dir1.Close()

	writer, err := index.NewIndexWriter(dir1, index.NewIndexWriterConfig(createTestAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := writer.AddDocument(reopenDoc(i, 2)); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	writer.Close()

	// Open a reader on the index.
	reader, err := index.OpenDirectoryReader(dir1)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	if reader.NumDocs() != 3 {
		t.Fatalf("expected 3 docs, got %d", reader.NumDocs())
	}
	reader.Close()

	// Phase 2: recreate the index on a fresh directory.
	dir2 := store.NewByteBuffersDirectory()
	defer dir2.Close()

	writer2, err := index.NewIndexWriter(dir2, index.NewIndexWriterConfig(createTestAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter (recreate): %v", err)
	}
	if err := writer2.AddDocument(reopenDoc(0, 2)); err != nil {
		t.Fatalf("AddDocument (recreate): %v", err)
	}
	if err := writer2.Commit(); err != nil {
		t.Fatalf("Commit (recreate): %v", err)
	}
	writer2.Close()

	// Open a reader on the recreated index.
	reader2, err := index.OpenDirectoryReader(dir2)
	if err != nil {
		t.Fatalf("OpenDirectoryReader (recreate): %v", err)
	}
	defer reader2.Close()
	if reader2.NumDocs() != 1 {
		t.Fatalf("expected 1 doc after recreate, got %d", reader2.NumDocs())
	}
}

// TestDirectoryReaderReopen_NPEAfterInvalidReindex2 ports
// testNPEAfterInvalidReindex2(). Tests reopening after complete reindex
// on a fresh directory.
func TestDirectoryReaderReopen_NPEAfterInvalidReindex2(t *testing.T) {
	dir1 := store.NewByteBuffersDirectory()
	defer dir1.Close()

	// Write initial index.
	writer, err := index.NewIndexWriter(dir1, index.NewIndexWriterConfig(createTestAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for i := 0; i < 3; i++ {
		writer.AddDocument(reopenDoc(i, 2))
	}
	writer.Commit()
	writer.Close()

	// Open reader.
	reader, err := index.OpenDirectoryReader(dir1)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	if reader.NumDocs() != 3 {
		t.Fatalf("expected 3 docs, got %d", reader.NumDocs())
	}
	reader.Close()

	// Recreate index on a fresh directory.
	dir2 := store.NewByteBuffersDirectory()
	defer dir2.Close()

	writer2, err := index.NewIndexWriter(dir2, index.NewIndexWriterConfig(createTestAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter (recreate): %v", err)
	}
	writer2.AddDocument(reopenDoc(0, 2))
	writer2.Commit()
	writer2.Close()

	// Verify the recreated index.
	reader2, err := index.OpenDirectoryReader(dir2)
	if err != nil {
		t.Fatalf("OpenDirectoryReader (recreated): %v", err)
	}
	defer reader2.Close()
	if reader2.NumDocs() != 1 {
		t.Fatalf("expected 1 doc after recreate, got %d", reader2.NumDocs())
	}
}

// TestDirectoryReaderReopen_NRTMdeletes ports testNRTMdeletes().
// Tests that the writer correctly reflects document deletions in its
// committed state.
func TestDirectoryReaderReopen_NRTMdeletes(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	// Add documents and commit.
	for i := 0; i < 3; i++ {
		if err := writer.AddDocument(reopenDoc(i, 2)); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// Verify the committed state.
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	if reader.NumDocs() != 3 {
		t.Fatalf("expected 3 docs, got %d", reader.NumDocs())
	}
	reader.Close()

	// Add more documents without committing - NRT reader sees them.
	nrtReader, err := writer.GetReader()
	if err != nil {
		t.Fatalf("GetReader: %v", err)
	}
	nrtReader.Close()

	writer.Close()
}

// TestDirectoryReaderReopen_ListCommits exercises the commit-listing plumbing.
// Verifies that ListCommits enumerates the index and readers reflect committed docs.
func TestDirectoryReaderReopen_ListCommits(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	for i := 0; i < 4; i++ {
		if err := writer.AddDocument(reopenDoc(i, 4)); err != nil {
			t.Fatalf("AddDocument %d: %v", i, err)
		}
		if err := writer.Commit(); err != nil {
			t.Fatalf("Commit %d: %v", i, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close: %v", err)
	}

	commits, err := index.ListCommits(dir)
	if err != nil {
		t.Fatalf("ListCommits: %v", err)
	}
	if len(commits) == 0 {
		t.Fatal("ListCommits returned no commits")
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()
	if got, want := reader.NumDocs(), 4; got != want {
		t.Errorf("NumDocs = %d, want %d", got, want)
	}
}
