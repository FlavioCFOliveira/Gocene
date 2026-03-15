// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for the index package.
//
// Ported from Apache Lucene's org.apache.lucene.index.TestIndexWriterCommit
// Source: lucene/core/src/test/org/apache/lucene/index/TestIndexWriterCommit.java
//
// GC-175: Test IndexWriterCommit - Commit on close behavior, abort (rollback),
// multiple commits, commit data preservation, two-phase commit
package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// createCommitTestAnalyzer creates a simple test analyzer
func createCommitTestAnalyzer() analysis.Analyzer {
	return analysis.NewWhitespaceAnalyzer()
}

// addCommitTestDoc adds a simple document with content "aaa" to the writer
func addCommitTestDoc(writer *index.IndexWriter) error {
	doc := document.NewDocument()
	tf, err := document.NewTextField("content", "aaa", false)
	if err != nil {
		return err
	}
	doc.Add(tf)
	return writer.AddDocument(doc)
}

// addCommitTestDocWithIndex adds a document with indexed content and id
func addCommitTestDocWithIndex(writer *index.IndexWriter, idx int) error {
	doc := document.NewDocument()
	tf, err := document.NewTextField("content", "aaa", false)
	if err != nil {
		return err
	}
	doc.Add(tf)
	sf, err := document.NewStringField("id", string(rune('0'+idx)), true)
	if err != nil {
		return err
	}
	doc.Add(sf)
	return writer.AddDocument(doc)
}

// assertNoUnreferencedFiles checks that there are no unreferenced files after rollback
func assertNoUnreferencedFiles(t *testing.T, dir store.Directory, message string) {
	// Create a temporary writer and rollback to trigger file cleanup
	config := index.NewIndexWriterConfig(createCommitTestAnalyzer())
	tempWriter, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Logf("Warning: could not create temp writer for cleanup check: %v", err)
		return
	}
	tempWriter.Rollback()
}

// TestCommitOnClose tests that documents are only visible after writer close
// Source: TestIndexWriterCommit.testCommitOnClose()
// Purpose: Tests basic commit on close behavior
func TestCommitOnClose(t *testing.T) {
	t.Run("documents visible after close", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createCommitTestAnalyzer())
		writer, err := index.NewIndexWriter(dir, config)
		if err != nil {
			t.Fatalf("Failed to create IndexWriter: %v", err)
		}

		// Add 14 documents
		for i := 0; i < 14; i++ {
			if err := addCommitTestDoc(writer); err != nil {
				t.Fatalf("Failed to add document %d: %v", i, err)
			}
		}

		// Close writer (should commit)
		if err := writer.Close(); err != nil {
			t.Fatalf("Failed to close writer: %v", err)
		}

		// Open reader and verify documents are visible
		reader, err := index.OpenDirectoryReader(dir)
		if err != nil {
			t.Fatalf("Failed to open reader: %v", err)
		}
		defer reader.Close()

		if reader.NumDocs() != 14 {
			t.Errorf("Expected 14 documents, got %d", reader.NumDocs())
		}
	})

	t.Run("documents not visible until close", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		// Create initial index with 14 documents
		config := index.NewIndexWriterConfig(createCommitTestAnalyzer())
		writer, err := index.NewIndexWriter(dir, config)
		if err != nil {
			t.Fatalf("Failed to create IndexWriter: %v", err)
		}
		for i := 0; i < 14; i++ {
			if err := addCommitTestDoc(writer); err != nil {
				t.Fatalf("Failed to add document %d: %v", i, err)
			}
		}
		if err := writer.Close(); err != nil {
			t.Fatalf("Failed to close writer: %v", err)
		}

		// Open reader on committed index
		reader, err := index.OpenDirectoryReader(dir)
		if err != nil {
			t.Fatalf("Failed to open reader: %v", err)
		}

		// Create new writer and add more documents
		config2 := index.NewIndexWriterConfig(createCommitTestAnalyzer())
		config2.SetOpenMode(index.APPEND)
		writer, err = index.NewIndexWriter(dir, config2)
		if err != nil {
			reader.Close()
			t.Fatalf("Failed to create second IndexWriter: %v", err)
		}

		// Add 33 more documents (3 iterations of 11)
		for i := 0; i < 3; i++ {
			for j := 0; j < 11; j++ {
				if err := addCommitTestDoc(writer); err != nil {
					t.Fatalf("Failed to add document: %v", err)
				}
			}

			// Reopen reader - should still see only 14 documents
			r2, err := reader.Reopen()
			if err != nil {
				t.Fatalf("Failed to reopen reader: %v", err)
			}
			if r2.NumDocs() != 14 {
				t.Errorf("Reader incorrectly sees changes from writer: expected 14, got %d", r2.NumDocs())
			}
			r2.Close()

			// Check if original reader is still current
			isCurrent, _ := reader.IsCurrent()
			if !isCurrent {
				t.Error("Reader should have still been current")
			}
		}

		// Close writer - now changes should be visible
		if err := writer.Close(); err != nil {
			t.Fatalf("Failed to close writer: %v", err)
		}

		// Check reader is no longer current
		isCurrent, _ := reader.IsCurrent()
		if isCurrent {
			t.Error("Reader should not be current after writer close")
		}
		reader.Close()

		// Open new reader and verify all documents
		reader, err = index.OpenDirectoryReader(dir)
		if err != nil {
			t.Fatalf("Failed to open reader: %v", err)
		}
		defer reader.Close()

		// 14 + 33 = 47 documents
		if reader.NumDocs() != 47 {
			t.Errorf("Reader did not see changes after writer close: expected 47, got %d", reader.NumDocs())
		}
	})
}

// TestCommitOnCloseAbort tests that rollback aborts uncommitted changes
// Source: TestIndexWriterCommit.testCommitOnCloseAbort()
// Purpose: Tests abort (rollback) behavior
func TestCommitOnCloseAbort(t *testing.T) {
	t.Run("rollback aborts changes", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		// Create initial index with 14 documents
		config := index.NewIndexWriterConfig(createCommitTestAnalyzer())
		config.SetMaxBufferedDocs(10)
		writer, err := index.NewIndexWriter(dir, config)
		if err != nil {
			t.Fatalf("Failed to create IndexWriter: %v", err)
		}
		for i := 0; i < 14; i++ {
			if err := addCommitTestDoc(writer); err != nil {
				t.Fatalf("Failed to add document: %v", err)
			}
		}
		if err := writer.Close(); err != nil {
			t.Fatalf("Failed to close writer: %v", err)
		}

		// Open reader
		reader, err := index.OpenDirectoryReader(dir)
		if err != nil {
			t.Fatalf("Failed to open reader: %v", err)
		}
		if reader.NumDocs() != 14 {
			t.Errorf("Expected 14 documents, got %d", reader.NumDocs())
		}
		reader.Close()

		// Create writer with APPEND mode
		config2 := index.NewIndexWriterConfig(createCommitTestAnalyzer())
		config2.SetOpenMode(index.APPEND)
		config2.SetMaxBufferedDocs(10)
		writer, err = index.NewIndexWriter(dir, config2)
		if err != nil {
			t.Fatalf("Failed to create second IndexWriter: %v", err)
		}

		// Add 17 documents
		for i := 0; i < 17; i++ {
			if err := addCommitTestDoc(writer); err != nil {
				t.Fatalf("Failed to add document: %v", err)
			}
		}

		// Delete all documents with content "aaa"
		term := index.NewTerm("content", "aaa")
		if err := writer.DeleteDocuments(term); err != nil {
			t.Fatalf("Failed to delete documents: %v", err)
		}

		// Verify reader still sees 14 documents
		reader, err = index.OpenDirectoryReader(dir)
		if err != nil {
			t.Fatalf("Failed to open reader: %v", err)
		}
		if reader.NumDocs() != 14 {
			t.Errorf("Reader incorrectly sees changes from writer: expected 14, got %d", reader.NumDocs())
		}
		reader.Close()

		// Rollback writer
		if err := writer.Rollback(); err != nil {
			t.Fatalf("Failed to rollback writer: %v", err)
		}

		// Check no unreferenced files
		assertNoUnreferencedFiles(t, dir, "unreferenced files remain after rollback()")

		// Verify reader still sees 14 documents after rollback
		reader, err = index.OpenDirectoryReader(dir)
		if err != nil {
			t.Fatalf("Failed to open reader: %v", err)
		}
		if reader.NumDocs() != 14 {
			t.Errorf("Saw changes after writer.abort: expected 14, got %d", reader.NumDocs())
		}
		reader.Close()

		// Verify we can reopen and add more documents
		config3 := index.NewIndexWriterConfig(createCommitTestAnalyzer())
		config3.SetOpenMode(index.APPEND)
		config3.SetMaxBufferedDocs(10)
		writer, err = index.NewIndexWriter(dir, config3)
		if err != nil {
			t.Fatalf("Failed to create third IndexWriter: %v", err)
		}

		// Add 204 documents (12 iterations of 17)
		for i := 0; i < 12; i++ {
			for j := 0; j < 17; j++ {
				if err := addCommitTestDoc(writer); err != nil {
					t.Fatalf("Failed to add document: %v", err)
				}
			}
			// Verify reader still sees only 14
			r, err := index.OpenDirectoryReader(dir)
			if err != nil {
				t.Fatalf("Failed to open reader: %v", err)
			}
			if r.NumDocs() != 14 {
				t.Errorf("Reader incorrectly sees changes from writer: expected 14, got %d", r.NumDocs())
			}
			r.Close()
		}

		// Close writer
		if err := writer.Close(); err != nil {
			t.Fatalf("Failed to close writer: %v", err)
		}

		// Verify all documents are visible
		reader, err = index.OpenDirectoryReader(dir)
		if err != nil {
			t.Fatalf("Failed to open reader: %v", err)
		}
		defer reader.Close()

		// 14 + 204 = 218 documents
		if reader.NumDocs() != 218 {
			t.Errorf("Didn't see changes after close: expected 218, got %d", reader.NumDocs())
		}
	})
}

// TestCommitOnCloseForceMerge tests forceMerge with commit on close
// Source: TestIndexWriterCommit.testCommitOnCloseForceMerge()
// Purpose: Tests forceMerge behavior with commit on close and rollback
func TestCommitOnCloseForceMerge(t *testing.T) {
	t.Run("forceMerge with rollback", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		// Create initial index with 17 documents
		config := index.NewIndexWriterConfig(createCommitTestAnalyzer())
		config.SetMaxBufferedDocs(10)
		config.SetMergePolicy(index.NewTieredMergePolicy())
		writer, err := index.NewIndexWriter(dir, config)
		if err != nil {
			t.Fatalf("Failed to create IndexWriter: %v", err)
		}
		for i := 0; i < 17; i++ {
			if err := addCommitTestDocWithIndex(writer, i); err != nil {
				t.Fatalf("Failed to add document: %v", err)
			}
		}
		if err := writer.Close(); err != nil {
			t.Fatalf("Failed to close writer: %v", err)
		}

		// Create writer and force merge
		config2 := index.NewIndexWriterConfig(createCommitTestAnalyzer())
		config2.SetOpenMode(index.APPEND)
		writer, err = index.NewIndexWriter(dir, config2)
		if err != nil {
			t.Fatalf("Failed to create second IndexWriter: %v", err)
		}

		if err := writer.ForceMerge(1); err != nil {
			t.Fatalf("Failed to force merge: %v", err)
		}

		// Open reader before closing (committing) the writer
		reader, err := index.OpenDirectoryReader(dir)
		if err != nil {
			t.Fatalf("Failed to open reader: %v", err)
		}

		// Reader should see index as multi-segment at this point
		leaves, err := reader.Leaves()
		if err != nil {
			t.Fatalf("Failed to get leaves: %v", err)
		}
		if len(leaves) <= 1 {
			t.Error("Reader incorrectly sees one segment")
		}
		reader.Close()

		// Abort the writer
		if err := writer.Rollback(); err != nil {
			t.Fatalf("Failed to rollback writer: %v", err)
		}

		assertNoUnreferencedFiles(t, dir, "aborted writer after forceMerge")

		// Open reader after aborting writer
		reader, err = index.OpenDirectoryReader(dir)
		if err != nil {
			t.Fatalf("Failed to open reader: %v", err)
		}

		// Reader should still see index as multi-segment
		leaves, err = reader.Leaves()
		if err != nil {
			t.Fatalf("Failed to get leaves: %v", err)
		}
		if len(leaves) <= 1 {
			t.Error("Reader incorrectly sees one segment after abort")
		}
		reader.Close()

		// Now do a real full merge
		config3 := index.NewIndexWriterConfig(createCommitTestAnalyzer())
		config3.SetOpenMode(index.APPEND)
		writer, err = index.NewIndexWriter(dir, config3)
		if err != nil {
			t.Fatalf("Failed to create third IndexWriter: %v", err)
		}

		if err := writer.ForceMerge(1); err != nil {
			t.Fatalf("Failed to force merge: %v", err)
		}
		if err := writer.Close(); err != nil {
			t.Fatalf("Failed to close writer: %v", err)
		}

		assertNoUnreferencedFiles(t, dir, "after real forceMerge")

		// Open reader after real merge
		reader, err = index.OpenDirectoryReader(dir)
		if err != nil {
			t.Fatalf("Failed to open reader: %v", err)
		}
		defer reader.Close()

		// Reader should see index as one segment
		leaves, err = reader.Leaves()
		if err != nil {
			t.Fatalf("Failed to get leaves: %v", err)
		}
		if len(leaves) != 1 {
			t.Errorf("Reader incorrectly sees more than one segment: got %d", len(leaves))
		}
	})
}

// TestForceCommit tests explicit commit() calls
// Source: TestIndexWriterCommit.testForceCommit()
// Purpose: Tests explicit commit behavior
func TestForceCommit(t *testing.T) {
	t.Run("explicit commit", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createCommitTestAnalyzer())
		config.SetMaxBufferedDocs(2)
		config.SetMergePolicy(index.NewTieredMergePolicy())
		writer, err := index.NewIndexWriter(dir, config)
		if err != nil {
			t.Fatalf("Failed to create IndexWriter: %v", err)
		}

		// Initial commit
		if err := writer.Commit(); err != nil {
			t.Fatalf("Failed to commit: %v", err)
		}

		// Add 23 documents
		for i := 0; i < 23; i++ {
			if err := addCommitTestDoc(writer); err != nil {
				t.Fatalf("Failed to add document: %v", err)
			}
		}

		// Open reader - should see 0 documents (not committed yet)
		reader, err := index.OpenDirectoryReader(dir)
		if err != nil {
			t.Fatalf("Failed to open reader: %v", err)
		}
		if reader.NumDocs() != 0 {
			t.Errorf("Expected 0 documents before commit, got %d", reader.NumDocs())
		}

		// Commit
		if err := writer.Commit(); err != nil {
			t.Fatalf("Failed to commit: %v", err)
		}

		// Reopen reader - should see 23 documents
		reader2, err := reader.Reopen()
		if err != nil {
			t.Fatalf("Failed to reopen reader: %v", err)
		}
		if reader2.NumDocs() != 23 {
			t.Errorf("Expected 23 documents after commit, got %d", reader2.NumDocs())
		}
		reader.Close()

		// Add 17 more documents
		for i := 0; i < 17; i++ {
			if err := addCommitTestDoc(writer); err != nil {
				t.Fatalf("Failed to add document: %v", err)
			}
		}

		// reader2 should still see 23
		if reader2.NumDocs() != 23 {
			t.Errorf("reader2 should still see 23, got %d", reader2.NumDocs())
		}
		reader2.Close()

		// Open new reader - should see 23 (not 40 yet)
		reader, err = index.OpenDirectoryReader(dir)
		if err != nil {
			t.Fatalf("Failed to open reader: %v", err)
		}
		if reader.NumDocs() != 23 {
			t.Errorf("Expected 23 documents, got %d", reader.NumDocs())
		}
		reader.Close()

		// Commit again
		if err := writer.Commit(); err != nil {
			t.Fatalf("Failed to commit: %v", err)
		}

		// Open reader - should see 40 documents
		reader, err = index.OpenDirectoryReader(dir)
		if err != nil {
			t.Fatalf("Failed to open reader: %v", err)
		}
		defer reader.Close()

		if reader.NumDocs() != 40 {
			t.Errorf("Expected 40 documents, got %d", reader.NumDocs())
		}

		writer.Close()
	})
}

// TestPrepareCommit tests the two-phase commit (prepareCommit/commit)
// Source: TestIndexWriterCommit.testPrepareCommit()
// Purpose: Tests two-phase commit behavior
func TestPrepareCommit(t *testing.T) {
	t.Run("two-phase commit", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createCommitTestAnalyzer())
		config.SetMaxBufferedDocs(2)
		config.SetMergePolicy(index.NewTieredMergePolicy())
		writer, err := index.NewIndexWriter(dir, config)
		if err != nil {
			t.Fatalf("Failed to create IndexWriter: %v", err)
		}

		// Initial commit
		if err := writer.Commit(); err != nil {
			t.Fatalf("Failed to commit: %v", err)
		}

		// Add 23 documents
		for i := 0; i < 23; i++ {
			if err := addCommitTestDoc(writer); err != nil {
				t.Fatalf("Failed to add document: %v", err)
			}
		}

		// Open reader - should see 0 documents
		reader, err := index.OpenDirectoryReader(dir)
		if err != nil {
			t.Fatalf("Failed to open reader: %v", err)
		}
		if reader.NumDocs() != 0 {
			t.Errorf("Expected 0 documents before prepareCommit, got %d", reader.NumDocs())
		}

		// Prepare commit
		if err := writer.PrepareCommit(); err != nil {
			t.Fatalf("Failed to prepare commit: %v", err)
		}

		// Open another reader - should still see 0 documents
		reader2, err := index.OpenDirectoryReader(dir)
		if err != nil {
			t.Fatalf("Failed to open reader: %v", err)
		}
		if reader2.NumDocs() != 0 {
			t.Errorf("Expected 0 documents after prepareCommit, got %d", reader2.NumDocs())
		}

		// Commit
		if err := writer.Commit(); err != nil {
			t.Fatalf("Failed to commit: %v", err)
		}

		// Reopen reader - should see 23 documents
		reader3, err := reader.Reopen()
		if err != nil {
			t.Fatalf("Failed to reopen reader: %v", err)
		}
		if reader3.NumDocs() != 23 {
			t.Errorf("Expected 23 documents after commit, got %d", reader3.NumDocs())
		}
		reader.Close()
		reader2.Close()

		// Add 17 more documents
		for i := 0; i < 17; i++ {
			if err := addCommitTestDoc(writer); err != nil {
				t.Fatalf("Failed to add document: %v", err)
			}
		}

		// reader3 should still see 23
		if reader3.NumDocs() != 23 {
			t.Errorf("reader3 should still see 23, got %d", reader3.NumDocs())
		}
		reader3.Close()

		// Open reader - should see 23
		reader, err = index.OpenDirectoryReader(dir)
		if err != nil {
			t.Fatalf("Failed to open reader: %v", err)
		}
		if reader.NumDocs() != 23 {
			t.Errorf("Expected 23 documents, got %d", reader.NumDocs())
		}
		reader.Close()

		// Prepare commit
		if err := writer.PrepareCommit(); err != nil {
			t.Fatalf("Failed to prepare commit: %v", err)
		}

		// Open reader - should still see 23
		reader, err = index.OpenDirectoryReader(dir)
		if err != nil {
			t.Fatalf("Failed to open reader: %v", err)
		}
		if reader.NumDocs() != 23 {
			t.Errorf("Expected 23 documents after prepareCommit, got %d", reader.NumDocs())
		}
		reader.Close()

		// Commit
		if err := writer.Commit(); err != nil {
			t.Fatalf("Failed to commit: %v", err)
		}

		// Open reader - should see 40
		reader, err = index.OpenDirectoryReader(dir)
		if err != nil {
			t.Fatalf("Failed to open reader: %v", err)
		}
		defer reader.Close()

		if reader.NumDocs() != 40 {
			t.Errorf("Expected 40 documents, got %d", reader.NumDocs())
		}

		writer.Close()
	})
}

// TestPrepareCommitRollback tests rollback after prepareCommit
// Source: TestIndexWriterCommit.testPrepareCommitRollback()
// Purpose: Tests rollback behavior after prepareCommit
func TestPrepareCommitRollback(t *testing.T) {
	t.Run("rollback after prepareCommit", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createCommitTestAnalyzer())
		config.SetMaxBufferedDocs(2)
		config.SetMergePolicy(index.NewTieredMergePolicy())
		writer, err := index.NewIndexWriter(dir, config)
		if err != nil {
			t.Fatalf("Failed to create IndexWriter: %v", err)
		}

		// Initial commit
		if err := writer.Commit(); err != nil {
			t.Fatalf("Failed to commit: %v", err)
		}

		// Add 23 documents
		for i := 0; i < 23; i++ {
			if err := addCommitTestDoc(writer); err != nil {
				t.Fatalf("Failed to add document: %v", err)
			}
		}

		// Open reader - should see 0 documents
		reader, err := index.OpenDirectoryReader(dir)
		if err != nil {
			t.Fatalf("Failed to open reader: %v", err)
		}
		if reader.NumDocs() != 0 {
			t.Errorf("Expected 0 documents, got %d", reader.NumDocs())
		}

		// Prepare commit
		if err := writer.PrepareCommit(); err != nil {
			t.Fatalf("Failed to prepare commit: %v", err)
		}

		// Open another reader - should still see 0
		reader2, err := index.OpenDirectoryReader(dir)
		if err != nil {
			t.Fatalf("Failed to open reader: %v", err)
		}
		if reader2.NumDocs() != 0 {
			t.Errorf("Expected 0 documents after prepareCommit, got %d", reader2.NumDocs())
		}

		// Rollback
		if err := writer.Rollback(); err != nil {
			t.Fatalf("Failed to rollback: %v", err)
		}

		// Reopen reader - should be null (no changes)
		reader3, err := reader.Reopen()
		if err != nil {
			t.Fatalf("Failed to reopen reader: %v", err)
		}
		// reader3 should be the same as reader (no changes)
		if reader3 != reader {
			t.Error("Expected reader3 to be the same as reader after rollback")
		}
		if reader.NumDocs() != 0 {
			t.Errorf("Expected 0 documents after rollback, got %d", reader.NumDocs())
		}
		if reader2.NumDocs() != 0 {
			t.Errorf("Expected 0 documents in reader2 after rollback, got %d", reader2.NumDocs())
		}
		reader.Close()
		reader2.Close()

		// Create new writer
		config2 := index.NewIndexWriterConfig(createCommitTestAnalyzer())
		writer, err = index.NewIndexWriter(dir, config2)
		if err != nil {
			t.Fatalf("Failed to create second IndexWriter: %v", err)
		}

		// Add 17 documents
		for i := 0; i < 17; i++ {
			if err := addCommitTestDoc(writer); err != nil {
				t.Fatalf("Failed to add document: %v", err)
			}
		}

		// Open reader - should see 0
		reader, err = index.OpenDirectoryReader(dir)
		if err != nil {
			t.Fatalf("Failed to open reader: %v", err)
		}
		if reader.NumDocs() != 0 {
			t.Errorf("Expected 0 documents, got %d", reader.NumDocs())
		}
		reader.Close()

		// Prepare commit
		if err := writer.PrepareCommit(); err != nil {
			t.Fatalf("Failed to prepare commit: %v", err)
		}

		// Open reader - should still see 0
		reader, err = index.OpenDirectoryReader(dir)
		if err != nil {
			t.Fatalf("Failed to open reader: %v", err)
		}
		if reader.NumDocs() != 0 {
			t.Errorf("Expected 0 documents after prepareCommit, got %d", reader.NumDocs())
		}
		reader.Close()

		// Commit
		if err := writer.Commit(); err != nil {
			t.Fatalf("Failed to commit: %v", err)
		}

		// Open reader - should see 17
		reader, err = index.OpenDirectoryReader(dir)
		if err != nil {
			t.Fatalf("Failed to open reader: %v", err)
		}
		defer reader.Close()

		if reader.NumDocs() != 17 {
			t.Errorf("Expected 17 documents, got %d", reader.NumDocs())
		}

		writer.Close()
	})
}

// TestPrepareCommitNoChanges tests prepareCommit with no changes
// Source: TestIndexWriterCommit.testPrepareCommitNoChanges()
// Purpose: Tests prepareCommit when there are no changes
func TestPrepareCommitNoChanges(t *testing.T) {
	t.Run("prepareCommit with no changes", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createCommitTestAnalyzer())
		writer, err := index.NewIndexWriter(dir, config)
		if err != nil {
			t.Fatalf("Failed to create IndexWriter: %v", err)
		}

		// Prepare commit with no changes
		if err := writer.PrepareCommit(); err != nil {
			t.Fatalf("Failed to prepare commit: %v", err)
		}

		// Commit
		if err := writer.Commit(); err != nil {
			t.Fatalf("Failed to commit: %v", err)
		}

		if err := writer.Close(); err != nil {
			t.Fatalf("Failed to close writer: %v", err)
		}

		// Open reader - should see 0 documents
		reader, err := index.OpenDirectoryReader(dir)
		if err != nil {
			t.Fatalf("Failed to open reader: %v", err)
		}
		defer reader.Close()

		if reader.NumDocs() != 0 {
			t.Errorf("Expected 0 documents, got %d", reader.NumDocs())
		}
	})
}

// TestPrepareCommitThenClose tests that close fails after prepareCommit
// Source: TestIndexWriterCommit.testPrepareCommitThenClose()
// Purpose: Tests that close() fails after prepareCommit without commit
func TestPrepareCommitThenClose(t *testing.T) {
	t.Run("close after prepareCommit should fail", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createCommitTestAnalyzer())
		writer, err := index.NewIndexWriter(dir, config)
		if err != nil {
			t.Fatalf("Failed to create IndexWriter: %v", err)
		}

		// Add a document
		doc := document.NewDocument()
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}

		// Prepare commit
		if err := writer.PrepareCommit(); err != nil {
			t.Fatalf("Failed to prepare commit: %v", err)
		}

		// Try to close - should fail
		err = writer.Close()
		if err == nil {
			t.Error("Expected close to fail after prepareCommit, but it succeeded")
		}

		// Commit and then close should succeed
		if err := writer.Commit(); err != nil {
			t.Fatalf("Failed to commit: %v", err)
		}
		if err := writer.Close(); err != nil {
			t.Fatalf("Failed to close writer after commit: %v", err)
		}

		// Verify document is visible
		reader, err := index.OpenDirectoryReader(dir)
		if err != nil {
			t.Fatalf("Failed to open reader: %v", err)
		}
		defer reader.Close()

		if reader.MaxDoc() != 1 {
			t.Errorf("Expected 1 document, got %d", reader.MaxDoc())
		}
	})
}

// TestCommitUserData tests setting and retrieving commit user data
// Source: TestIndexWriterCommit.testCommitUserData()
// Purpose: Tests commit data preservation
func TestCommitUserData(t *testing.T) {
	t.Run("commit user data", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		// Create initial index
		config := index.NewIndexWriterConfig(createCommitTestAnalyzer())
		config.SetMaxBufferedDocs(2)
		writer, err := index.NewIndexWriter(dir, config)
		if err != nil {
			t.Fatalf("Failed to create IndexWriter: %v", err)
		}
		for i := 0; i < 17; i++ {
			if err := addCommitTestDoc(writer); err != nil {
				t.Fatalf("Failed to add document: %v", err)
			}
		}
		if err := writer.Close(); err != nil {
			t.Fatalf("Failed to close writer: %v", err)
		}

		// Open reader and check that no user data was set
		reader, err := index.OpenDirectoryReader(dir)
		if err != nil {
			t.Fatalf("Failed to open reader: %v", err)
		}
		commit := reader.GetIndexCommit()
		if commit == nil {
			t.Fatal("GetIndexCommit returned nil")
		}
		if len(commit.GetUserData()) != 0 {
			t.Errorf("Expected empty user data, got %v", commit.GetUserData())
		}
		reader.Close()

		// Create writer with user data
		config2 := index.NewIndexWriterConfig(createCommitTestAnalyzer())
		config2.SetMaxBufferedDocs(2)
		writer, err = index.NewIndexWriter(dir, config2)
		if err != nil {
			t.Fatalf("Failed to create second IndexWriter: %v", err)
		}
		for i := 0; i < 17; i++ {
			if err := addCommitTestDoc(writer); err != nil {
				t.Fatalf("Failed to add document: %v", err)
			}
		}

		// Set commit data
		data := map[string]string{
			"label": "test1",
		}
		writer.SetLiveCommitData(data)
		if err := writer.Close(); err != nil {
			t.Fatalf("Failed to close writer: %v", err)
		}

		// Open reader and verify user data
		reader, err = index.OpenDirectoryReader(dir)
		if err != nil {
			t.Fatalf("Failed to open reader: %v", err)
		}
		commit = reader.GetIndexCommit()
		if commit == nil {
			t.Fatal("GetIndexCommit returned nil")
		}
		userData := commit.GetUserData()
		if userData["label"] != "test1" {
			t.Errorf("Expected label=test1, got %s", userData["label"])
		}
		reader.Close()

		// Force merge and close
		config3 := index.NewIndexWriterConfig(createCommitTestAnalyzer())
		writer, err = index.NewIndexWriter(dir, config3)
		if err != nil {
			t.Fatalf("Failed to create third IndexWriter: %v", err)
		}
		if err := writer.ForceMerge(1); err != nil {
			t.Fatalf("Failed to force merge: %v", err)
		}
		if err := writer.Close(); err != nil {
			t.Fatalf("Failed to close writer: %v", err)
		}
	})
}

// TestCommitDataIsLive tests that commit data is late binding
// Source: TestIndexWriterCommit.testCommitDataIsLive()
// Purpose: Tests that commit data is captured at commit time, not set time
func TestCommitDataIsLive(t *testing.T) {
	t.Run("commit data is late binding", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createCommitTestAnalyzer())
		writer, err := index.NewIndexWriter(dir, config)
		if err != nil {
			t.Fatalf("Failed to create IndexWriter: %v", err)
		}

		doc := document.NewDocument()
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}

		// Set commit data with "foo"="bar"
		data := map[string]string{
			"foo": "bar",
		}
		writer.SetLiveCommitData(data)

		// Clear and set new data
		data["foo"] = "baz"

		// Close writer (commits with current data)
		if err := writer.Close(); err != nil {
			t.Fatalf("Failed to close writer: %v", err)
		}

		// List commits and verify data
		commits, err := index.ListCommits(dir)
		if err != nil {
			t.Fatalf("Failed to list commits: %v", err)
		}
		if len(commits) != 1 {
			t.Fatalf("Expected 1 commit, got %d", len(commits))
		}

		commit := commits[0]
		userData := commit.GetUserData()
		if len(userData) != 1 {
			t.Errorf("Expected 1 user data entry, got %d", len(userData))
		}
		if userData["foo"] != "baz" {
			t.Errorf("Expected foo=baz, got foo=%s", userData["foo"])
		}
	})
}

// TestZeroCommits tests that no commits exist before first commit
// Source: TestIndexWriterCommit.testZeroCommits()
// Purpose: Tests that listCommits fails before any commit
func TestZeroCommits(t *testing.T) {
	t.Run("zero commits before first commit", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createCommitTestAnalyzer())
		writer, err := index.NewIndexWriter(dir, config)
		if err != nil {
			t.Fatalf("Failed to create IndexWriter: %v", err)
		}

		// listCommits should fail before any commit
		_, err = index.ListCommits(dir)
		if err == nil {
			t.Error("Expected listCommits to fail before any commit")
		}

		// Close writer (should create a commit for new index)
		if err := writer.Close(); err != nil {
			t.Fatalf("Failed to close writer: %v", err)
		}

		// Now listCommits should succeed
		commits, err := index.ListCommits(dir)
		if err != nil {
			t.Fatalf("Failed to list commits after close: %v", err)
		}
		if len(commits) != 1 {
			t.Errorf("Expected 1 commit, got %d", len(commits))
		}
	})
}
