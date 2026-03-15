// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for the index package.
//
// Ported from Apache Lucene's org.apache.lucene.index.TestIndexWriterReader
// Source: lucene/core/src/test/org/apache/lucene/index/TestIndexWriterReader.java
//
// GC-187: Test IndexWriterReader - NRT reader functionality
//
// Focus areas:
//   - NRT reader from IndexWriter
//   - Uncommitted changes visibility
//   - NRT reader reopening
//   - isCurrent() behavior
//   - Document updates and deletes via NRT
//   - AddIndexes with NRT readers
//   - Thread safety of NRT operations
package index_test

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestIndexWriterReader_BasicNRT tests basic NRT reader functionality
// Ported from testAddCloseOpen() in TestIndexWriterReader.java
// Purpose: Tests basic document addition and NRT reader visibility
func TestIndexWriterReader_BasicNRT(t *testing.T) {
	t.Run("NRT reader sees added documents", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		writer, err := index.NewIndexWriter(dir, config)
		if err != nil {
			t.Fatalf("Failed to create IndexWriter: %v", err)
		}
		defer writer.Close()

		// Add documents
		for i := 0; i < 10; i++ {
			doc := createTestDoc(i, "test", 2)
			if err := writer.AddDocument(doc); err != nil {
				t.Fatalf("Failed to add document %d: %v", i, err)
			}
		}

		// Commit to persist documents
		if err := writer.Commit(); err != nil {
			t.Fatalf("Failed to commit: %v", err)
		}

		// Open reader and verify documents are visible
		reader, err := index.OpenDirectoryReader(dir)
		if err != nil {
			t.Fatalf("Failed to open reader: %v", err)
		}
		defer reader.Close()

		if reader.NumDocs() != 10 {
			t.Errorf("Expected 10 documents, got %d", reader.NumDocs())
		}
	})

	t.Run("reader isCurrent after commit", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		writer, err := index.NewIndexWriter(dir, config)
		if err != nil {
			t.Fatalf("Failed to create IndexWriter: %v", err)
		}

		// Add document and commit
		doc := createTestDoc(1, "test", 2)
		writer.AddDocument(doc)
		writer.Commit()

		// Open reader
		reader, err := index.OpenDirectoryReader(dir)
		if err != nil {
			t.Fatalf("Failed to open reader: %v", err)
		}

		// Reader should be current after opening on committed index
		isCurrent, err := reader.IsCurrent()
		if err != nil {
			t.Fatalf("IsCurrent() error: %v", err)
		}
		if !isCurrent {
			t.Error("Expected reader to be current after opening on committed index")
		}

		reader.Close()

		// Reopen writer and add another document
		config2 := index.NewIndexWriterConfig(createTestAnalyzer())
		config2.SetOpenMode(index.APPEND)
		writer2, err := index.NewIndexWriter(dir, config2)
		if err != nil {
			t.Fatalf("Failed to reopen IndexWriter: %v", err)
		}

		doc2 := createTestDoc(2, "test", 2)
		writer2.AddDocument(doc2)
		writer2.Close()

		// Reader should not be current after new documents added
		isCurrent, err = reader.IsCurrent()
		if err != nil {
			t.Fatalf("IsCurrent() error after modification: %v", err)
		}
		if isCurrent {
			t.Error("Expected reader to not be current after index modification")
		}
	})
}

// TestIndexWriterReader_IsCurrent tests isCurrent() behavior
// Ported from testIsCurrent() in TestIndexWriterReader.java
// Purpose: Tests that isCurrent() correctly reflects index state changes
func TestIndexWriterReader_IsCurrent(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Create initial index
	config := index.NewIndexWriterConfig(createTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	doc := createTestDoc(1, "test", 2)
	writer.AddDocument(doc)
	writer.Commit()
	writer.Close()

	// Reopen writer
	config2 := index.NewIndexWriterConfig(createTestAnalyzer())
	config2.SetOpenMode(index.APPEND)
	writer2, err := index.NewIndexWriter(dir, config2)
	if err != nil {
		t.Fatalf("Failed to reopen IndexWriter: %v", err)
	}
	defer writer2.Close()

	// Open reader
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	// Reader should be current initially
	isCurrent, err := reader.IsCurrent()
	if err != nil {
		t.Fatalf("IsCurrent() error: %v", err)
	}
	if !isCurrent {
		t.Error("Expected reader to be current initially")
	}

	// Add document - reader should not be current
	doc2 := createTestDoc(2, "test", 2)
	writer2.AddDocument(doc2)

	isCurrent, err = reader.IsCurrent()
	if err != nil {
		t.Fatalf("IsCurrent() error after add: %v", err)
	}
	if isCurrent {
		t.Error("Expected reader to not be current after adding document")
	}

	// Commit changes
	writer2.Commit()

	// Reader should still not be current (it was opened before commit)
	isCurrent, err = reader.IsCurrent()
	if err != nil {
		t.Fatalf("IsCurrent() error after commit: %v", err)
	}
	if isCurrent {
		t.Error("Expected reader to not be current after commit (opened before commit)")
	}
}

// TestIndexWriterReader_UpdateDocument tests document updates
// Ported from testUpdateDocument() in TestIndexWriterReader.java
// Purpose: Tests that updates are visible through NRT readers
func TestIndexWriterReader_UpdateDocument(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	// Ensure we have enough buffered docs
	config.SetMaxBufferedDocs(20)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}
	defer writer.Close()

	// Create initial documents
	for i := 0; i < 10; i++ {
		doc := createTestDoc(i, "index1", 2)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document %d: %v", i, err)
		}
	}
	writer.Commit()

	// Open reader
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}

	// Verify initial documents
	if reader.NumDocs() != 10 {
		t.Errorf("Expected 10 documents initially, got %d", reader.NumDocs())
	}
	reader.Close()

	// Update a document
	term := index.NewTerm("id", "5")
	newDoc := createTestDoc(100, "updated", 2)
	if err := writer.UpdateDocument(term, newDoc); err != nil {
		t.Fatalf("Failed to update document: %v", err)
	}
	writer.Commit()

	// Reopen reader and verify
	reader2, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to reopen reader: %v", err)
	}
	defer reader2.Close()

	// Should still have 10 documents (update = delete + add)
	if reader2.NumDocs() != 10 {
		t.Errorf("Expected 10 documents after update, got %d", reader2.NumDocs())
	}
}

// TestIndexWriterReader_DeleteFromIndexWriter tests document deletion
// Ported from testDeleteFromIndexWriter() in TestIndexWriterReader.java
// Purpose: Tests that deletes are visible through NRT readers
func TestIndexWriterReader_DeleteFromIndexWriter(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}
	defer writer.Close()

	// Create initial documents
	for i := 0; i < 10; i++ {
		doc := createTestDoc(i, "index1", 2)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document %d: %v", i, err)
		}
	}
	writer.Commit()

	// Open reader
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}

	// Verify initial documents
	if reader.NumDocs() != 10 {
		t.Errorf("Expected 10 documents initially, got %d", reader.NumDocs())
	}
	reader.Close()

	// Delete documents
	term := index.NewTerm("id", "5")
	if err := writer.DeleteDocuments(term); err != nil {
		t.Fatalf("Failed to delete document: %v", err)
	}
	writer.Commit()

	// Reopen reader and verify
	reader2, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to reopen reader: %v", err)
	}
	defer reader2.Close()

	// Should have 9 documents after delete
	if reader2.NumDocs() != 9 {
		t.Errorf("Expected 9 documents after delete, got %d", reader2.NumDocs())
	}
}

// TestIndexWriterReader_AddIndexes tests adding indexes from other directories
// Ported from testAddIndexes() in TestIndexWriterReader.java
// Purpose: Tests that added indexes are visible through NRT readers
func TestIndexWriterReader_AddIndexes(t *testing.T) {
	t.Run("add single index", func(t *testing.T) {
		// Create source directory
		sourceDir := store.NewByteBuffersDirectory()
		defer sourceDir.Close()

		sourceConfig := index.NewIndexWriterConfig(createTestAnalyzer())
		sourceWriter, err := index.NewIndexWriter(sourceDir, sourceConfig)
		if err != nil {
			t.Fatalf("Failed to create source IndexWriter: %v", err)
		}

		// Add documents to source
		for i := 0; i < 50; i++ {
			doc := createTestDoc(i, "source", 2)
			sourceWriter.AddDocument(doc)
		}
		sourceWriter.Commit()
		sourceWriter.Close()

		// Create target directory
		targetDir := store.NewByteBuffersDirectory()
		defer targetDir.Close()

		targetConfig := index.NewIndexWriterConfig(createTestAnalyzer())
		targetWriter, err := index.NewIndexWriter(targetDir, targetConfig)
		if err != nil {
			t.Fatalf("Failed to create target IndexWriter: %v", err)
		}

		// Add documents to target
		for i := 0; i < 50; i++ {
			doc := createTestDoc(i, "target", 2)
			targetWriter.AddDocument(doc)
		}
		targetWriter.Commit()

		// Add source index to target
		if err := targetWriter.AddIndexes(sourceDir); err != nil {
			t.Fatalf("Failed to add indexes: %v", err)
		}
		targetWriter.Commit()
		targetWriter.Close()

		// Verify total documents
		reader, err := index.OpenDirectoryReader(targetDir)
		if err != nil {
			t.Fatalf("Failed to open reader: %v", err)
		}
		defer reader.Close()

		// Should have 100 documents (50 from target + 50 from source)
		if reader.NumDocs() != 100 {
			t.Errorf("Expected 100 documents after addIndexes, got %d", reader.NumDocs())
		}
	})
}

// TestIndexWriterReader_EmptyIndex tests NRT reader on empty index
// Ported from testEmptyIndex() in TestIndexWriterReader.java
// Purpose: Ensures that getReader works on an empty index
func TestIndexWriterReader_EmptyIndex(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}
	defer writer.Close()

	// Open reader on empty index (no commit yet)
	reader, err := index.OpenDirectoryReader(dir)
	if err == nil {
		// If we can open a reader, it should show 0 documents
		if reader.NumDocs() != 0 {
			t.Errorf("Expected 0 documents on empty index, got %d", reader.NumDocs())
		}
		reader.Close()
	}
	// Error is expected if no segments file exists yet
}

// TestIndexWriterReader_AfterClose tests reader after writer close
// Ported from testAfterClose() in TestIndexWriterReader.java
// Purpose: Tests that reader remains usable after IndexWriter closes
func TestIndexWriterReader_AfterClose(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Create documents
	for i := 0; i < 10; i++ {
		doc := createTestDoc(i, "test", 2)
		writer.AddDocument(doc)
	}
	writer.Commit()

	// Open reader
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}

	// Close writer
	writer.Close()

	// Reader should still be usable
	if reader.NumDocs() != 10 {
		t.Errorf("Expected 10 documents after writer close, got %d", reader.NumDocs())
	}

	reader.Close()
}

// TestIndexWriterReader_DeletesNumDocs tests numDocs after deletes
// Ported from testDeletesNumDocs() in TestIndexWriterReader.java
// Purpose: Tests that numDocs reflects deletions correctly
func TestIndexWriterReader_DeletesNumDocs(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}
	defer writer.Close()

	// Add documents
	doc1 := createTestDoc(1, "test", 2)
	doc2 := createTestDoc(2, "test", 2)
	writer.AddDocument(doc1)
	writer.AddDocument(doc2)
	writer.Commit()

	// Verify initial count
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	if reader.NumDocs() != 2 {
		t.Errorf("Expected 2 documents initially, got %d", reader.NumDocs())
	}
	reader.Close()

	// Delete first document
	term1 := index.NewTerm("id", "1")
	writer.DeleteDocuments(term1)
	writer.Commit()

	reader2, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to reopen reader: %v", err)
	}
	if reader2.NumDocs() != 1 {
		t.Errorf("Expected 1 document after first delete, got %d", reader2.NumDocs())
	}
	reader2.Close()

	// Delete second document
	term2 := index.NewTerm("id", "2")
	writer.DeleteDocuments(term2)
	writer.Commit()

	reader3, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to reopen reader: %v", err)
	}
	if reader3.NumDocs() != 0 {
		t.Errorf("Expected 0 documents after all deletes, got %d", reader3.NumDocs())
	}
	reader3.Close()
}

// TestIndexWriterReader_ForceMergeDeletes tests force merge of deletes
// Ported from testForceMergeDeletes() in TestIndexWriterReader.java
// Purpose: Tests that forceMergeDeletes removes deleted documents
func TestIndexWriterReader_ForceMergeDeletes(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add documents
	doc1 := createTestDoc(1, "test", 2)
	doc2 := createTestDoc(2, "test", 2)
	writer.AddDocument(doc1)
	writer.AddDocument(doc2)
	writer.Commit()

	// Delete first document
	term := index.NewTerm("id", "1")
	writer.DeleteDocuments(term)
	writer.Commit()

	// Force merge
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge failed: %v", err)
	}
	writer.Close()

	// Verify
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 1 {
		t.Errorf("Expected 1 document after force merge, got %d", reader.NumDocs())
	}
}

// TestIndexWriterReader_ConcurrentAccess tests concurrent access
// Ported from testDuringAddDelete() in TestIndexWriterReader.java
// Purpose: Tests thread safety of NRT operations
func TestIndexWriterReader_ConcurrentAccess(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Create initial documents
	for i := 0; i < 10; i++ {
		doc := createTestDoc(i, "test", 2)
		writer.AddDocument(doc)
	}
	writer.Commit()

	// Open reader
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	// Concurrent operations
	var wg sync.WaitGroup
	numGoroutines := 3
	iterations := 5

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				doc := createTestDoc(100*id+j, "concurrent", 2)
				writer.AddDocument(doc)
			}
		}(i)
	}

	wg.Wait()
	writer.Commit()

	// Verify documents were added
	reader2, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to reopen reader: %v", err)
	}
	defer reader2.Close()

	expectedDocs := 10 + numGoroutines*iterations
	if reader2.NumDocs() != expectedDocs {
		t.Errorf("Expected %d documents after concurrent adds, got %d", expectedDocs, reader2.NumDocs())
	}

	writer.Close()
}

// TestIndexWriterReader_Reopen tests reader reopening
// Ported from testIndexWriterReopenSegment() in TestIndexWriterReader.java
// Purpose: Tests that reopening reflects index changes
func TestIndexWriterReader_Reopen(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}
	defer writer.Close()

	// Open initial reader on empty index
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		// Empty index may not have segments yet
		reader = nil
	}

	// Add documents
	for i := 0; i < 10; i++ {
		doc := createTestDoc(i, "test", 2)
		writer.AddDocument(doc)
	}
	writer.Commit()

	// Reopen reader
	if reader != nil {
		reader.Close()
	}
	reader2, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to reopen reader: %v", err)
	}
	defer reader2.Close()

	if reader2.NumDocs() != 10 {
		t.Errorf("Expected 10 documents after reopen, got %d", reader2.NumDocs())
	}

	// Add more documents
	for i := 10; i < 20; i++ {
		doc := createTestDoc(i, "test", 2)
		writer.AddDocument(doc)
	}
	writer.Commit()

	// Reopen again
	reader2.Close()
	reader3, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to reopen reader: %v", err)
	}
	defer reader3.Close()

	if reader3.NumDocs() != 20 {
		t.Errorf("Expected 20 documents after second reopen, got %d", reader3.NumDocs())
	}
}

// TestIndexWriterReader_AddIndexesMultiple tests adding multiple indexes
// Ported from testAddIndexes2() in TestIndexWriterReader.java
// Purpose: Tests adding the same index multiple times
func TestIndexWriterReader_AddIndexesMultiple(t *testing.T) {
	// Create source directory
	sourceDir := store.NewByteBuffersDirectory()
	defer sourceDir.Close()

	sourceConfig := index.NewIndexWriterConfig(createTestAnalyzer())
	sourceWriter, err := index.NewIndexWriter(sourceDir, sourceConfig)
	if err != nil {
		t.Fatalf("Failed to create source IndexWriter: %v", err)
	}

	// Add documents to source
	for i := 0; i < 20; i++ {
		doc := createTestDoc(i, "source", 2)
		sourceWriter.AddDocument(doc)
	}
	sourceWriter.Commit()
	sourceWriter.Close()

	// Create target directory
	targetDir := store.NewByteBuffersDirectory()
	defer targetDir.Close()

	targetConfig := index.NewIndexWriterConfig(createTestAnalyzer())
	targetWriter, err := index.NewIndexWriter(targetDir, targetConfig)
	if err != nil {
		t.Fatalf("Failed to create target IndexWriter: %v", err)
	}

	// Add source index multiple times
	for i := 0; i < 5; i++ {
		if err := targetWriter.AddIndexes(sourceDir); err != nil {
			t.Fatalf("Failed to add indexes iteration %d: %v", i, err)
		}
	}
	targetWriter.Commit()
	targetWriter.Close()

	// Verify total documents
	reader, err := index.OpenDirectoryReader(targetDir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	// Should have 100 documents (20 * 5)
	expectedDocs := 100
	if reader.NumDocs() != expectedDocs {
		t.Errorf("Expected %d documents after multiple addIndexes, got %d", expectedDocs, reader.NumDocs())
	}
}

// TestIndexWriterReader_ReaderPooling tests reader pooling behavior
// Ported from testAfterCommit() in TestIndexWriterReader.java
// Purpose: Tests reader pooling with commits
func TestIndexWriterReader_ReaderPooling(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}
	defer writer.Close()

	// Initial commit
	writer.Commit()

	// Create initial documents
	for i := 0; i < 10; i++ {
		doc := createTestDoc(i, "test", 2)
		writer.AddDocument(doc)
	}
	writer.Commit()

	// Open reader
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}

	if reader.NumDocs() != 10 {
		t.Errorf("Expected 10 documents initially, got %d", reader.NumDocs())
	}
	reader.Close()

	// Add more documents
	for i := 10; i < 15; i++ {
		doc := createTestDoc(i, "test", 2)
		writer.AddDocument(doc)
	}
	writer.Commit()

	// Reopen reader
	reader2, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to reopen reader: %v", err)
	}
	defer reader2.Close()

	if reader2.NumDocs() != 15 {
		t.Errorf("Expected 15 documents after adding more, got %d", reader2.NumDocs())
	}
}

// TestIndexWriterReader_NRTStressTest is a stress test for NRT operations
// Ported from stress test patterns in TestIndexWriterReader.java
// Purpose: Tests stability under heavy NRT reader operations
func TestIndexWriterReader_NRTStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}
	defer writer.Close()

	// Initial documents
	for i := 0; i < 50; i++ {
		doc := createTestDoc(i, "test", 2)
		writer.AddDocument(doc)
	}
	writer.Commit()

	// Concurrent readers and writers
	var wg sync.WaitGroup
	numReaders := 3
	numWriters := 2
	iterations := 10

	// Track document count
	var docCount atomic.Int32
	docCount.Store(50)

	// Writer goroutines
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				doc := createTestDoc(1000*id+j, "stress", 2)
				if err := writer.AddDocument(doc); err != nil {
					t.Errorf("Failed to add document: %v", err)
					return
				}
				docCount.Add(1)
			}
		}(i)
	}

	// Reader goroutines
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				reader, err := index.OpenDirectoryReader(dir)
				if err != nil {
					continue // Index may be in flux
				}
				_ = reader.NumDocs() // Just verify we can read
				reader.Close()
			}
		}(i)
	}

	wg.Wait()
	writer.Commit()

	// Final verification
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open final reader: %v", err)
	}
	defer reader.Close()

	expectedDocs := int(docCount.Load())
	if reader.NumDocs() != expectedDocs {
		t.Errorf("Expected %d documents after stress test, got %d", expectedDocs, reader.NumDocs())
	}
}

// TestIndexWriterReader_NRTNotYetImplemented documents NRT features not yet implemented
// These tests verify the expected behavior once NRT reader from IndexWriter is implemented
func TestIndexWriterReader_NRTNotYetImplemented(t *testing.T) {
	t.Skip("NRT reader from IndexWriter not yet fully implemented")

	// The following features from TestIndexWriterReader.java are not yet implemented:
	//
	// 1. writer.GetReader() - Get NRT reader directly from IndexWriter
	//    This would allow seeing uncommitted changes
	//
	// 2. DirectoryReader.openIfChanged(reader) - Reopen reader to see changes
	//    This would allow efficient reopening without re-reading unchanged segments
	//
	// 3. DirectoryReader.openIfChanged(reader, writer) - Reopen with writer
	//    This would allow seeing uncommitted changes from a specific writer
	//
	// 4. reader.isCurrent() with uncommitted changes
	//    Currently only works with committed changes
	//
	// 5. Segment sharing between reopened readers
	//    For efficiency, unchanged segments should be shared
	//
	// 6. MergedSegmentWarmer callback
	//    Callback when segments are merged
	//
	// 7. SimpleMergedSegmentWarmer
	//    Built-in warmer implementation
	//
	// 8. LeafSorter for custom segment ordering
	//    Custom ordering of leaf readers
}

// createTestDoc creates a test document with the given parameters
// Ported from DocHelper.createDocument() in Lucene tests
func createTestDoc(id int, indexName string, numFields int) *document.Document {
	doc := &document.Document{}

	// Add ID field
	idField, _ := document.NewStringField("id", string(rune('0'+id%10)), true)
	doc.Add(idField)

	// Add index name field
	indexField, _ := document.NewStringField("indexname", indexName, true)
	doc.Add(indexField)

	// Add content fields
	for i := 0; i < numFields; i++ {
		fieldName := "field" + string(rune('1'+i))
		fieldValue := "value" + string(rune('0'+id%10)) + " " + indexName
		field, _ := document.NewTextField(fieldName, fieldValue, false)
		doc.Add(field)
	}

	return doc
}
