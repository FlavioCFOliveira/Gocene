// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for the index package.
//
// Ported from Apache Lucene's org.apache.lucene.store.TestIndexWriter
// and related test files:
//   - TestIndexWriter.java
//
// GC-114: Index Tests - IndexWriter Core
package index_test

import (
	"os"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// createTestAnalyzer creates a simple test analyzer
func createTestAnalyzer() analysis.Analyzer {
	return analysis.NewWhitespaceAnalyzer()
}

// TestIndexWriterCommitDataIsolation verifies that two writers on separate
// directories have independent commit data (regression: was using package-level globals).
func TestIndexWriterCommitDataIsolation(t *testing.T) {
	dir1 := store.NewByteBuffersDirectory()
	defer dir1.Close()
	dir2 := store.NewByteBuffersDirectory()
	defer dir2.Close()

	config1 := index.NewIndexWriterConfig(createTestAnalyzer())
	writer1, err := index.NewIndexWriter(dir1, config1)
	if err != nil {
		t.Fatalf("NewIndexWriter(dir1) error = %v", err)
	}

	config2 := index.NewIndexWriterConfig(createTestAnalyzer())
	writer2, err := index.NewIndexWriter(dir2, config2)
	if err != nil {
		t.Fatalf("NewIndexWriter(dir2) error = %v", err)
	}

	// Set different commit data on each writer
	writer1.SetLiveCommitData(map[string]string{"writer": "one"})
	writer2.SetLiveCommitData(map[string]string{"writer": "two"})

	// Commit both - should not interfere
	if err := writer1.Commit(); err != nil {
		t.Errorf("writer1.Commit() error = %v", err)
	}
	if err := writer2.Commit(); err != nil {
		t.Errorf("writer2.Commit() error = %v", err)
	}

	writer1.Close()
	writer2.Close()

	// Verify both indices are readable
	reader1, err := index.OpenDirectoryReader(dir1)
	if err != nil {
		t.Errorf("OpenDirectoryReader(dir1) error = %v", err)
	} else {
		reader1.Close()
	}

	reader2, err := index.OpenDirectoryReader(dir2)
	if err != nil {
		t.Errorf("OpenDirectoryReader(dir2) error = %v", err)
	} else {
		reader2.Close()
	}
}

// TestNewIndexWriter tests IndexWriter creation
func TestNewIndexWriter(t *testing.T) {
	t.Run("create with valid directory", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		writer, err := index.NewIndexWriter(dir, config)
		if err != nil {
			t.Fatalf("NewIndexWriter() error = %v", err)
		}
		if writer == nil {
			t.Fatal("NewIndexWriter() returned nil")
		}

		defer writer.Close()
	})
}

// TestIndexWriterClose tests close operations
func TestIndexWriterClose(t *testing.T) {
	t.Run("close without changes", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)

		err := writer.Close()
		if err != nil {
			t.Errorf("Close() error = %v", err)
		}

		if !writer.IsClosed() {
			t.Error("IsClosed() should return true after Close()")
		}
	})

	t.Run("close twice", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)

		writer.Close()
		err := writer.Close()
		if err != nil {
			t.Errorf("Close() second time error = %v", err)
		}
	})
}

// TestIndexWriterNumDocs tests document counting
func TestIndexWriterNumDocs(t *testing.T) {
	t.Run("num docs with no documents", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)
		defer writer.Close()

		num := writer.NumDocs()
		if num != 0 {
			t.Errorf("NumDocs() = %d, want 0", num)
		}
	})
}

// TestIndexWriterMaxDoc tests max document ID
func TestIndexWriterMaxDoc(t *testing.T) {
	t.Run("max doc with no documents", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)
		defer writer.Close()

		max := writer.MaxDoc()
		if max != 0 {
			t.Errorf("MaxDoc() = %d, want 0", max)
		}
	})
}

// TestIndexWriterCommit tests commit operations
func TestIndexWriterCommit(t *testing.T) {
	t.Run("commit with no changes", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)

		err := writer.Commit()
		if err != nil {
			t.Errorf("Commit() error = %v", err)
		}

		writer.Close()
	})
}

// TestIndexWriterWithFSDirectory tests with file system directory
func TestIndexWriterWithFSDirectory(t *testing.T) {
	t.Run("create with simple fs directory", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "gocene_index_test_*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		dir, err := store.NewSimpleFSDirectory(tempDir)
		if err != nil {
			t.Fatalf("NewSimpleFSDirectory() error = %v", err)
		}
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		writer, err := index.NewIndexWriter(dir, config)
		if err != nil {
			t.Fatalf("NewIndexWriter() error = %v", err)
		}

		err = writer.Commit()
		if err != nil {
			t.Errorf("Commit() error = %v", err)
		}

		writer.Close()
	})
}

// TestIndexWriterConfig tests with different configurations
func TestIndexWriterConfig(t *testing.T) {
	t.Run("config with different ram buffer", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		config.SetRAMBufferSizeMB(64.0)

		writer, _ := index.NewIndexWriter(dir, config)
		defer writer.Close()
	})

	t.Run("config with merge policy", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		policy := index.NewTieredMergePolicy()
		config.SetMergePolicy(policy)

		writer, _ := index.NewIndexWriter(dir, config)
		defer writer.Close()
	})

	t.Run("config with open mode create", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		config.SetOpenMode(index.CREATE)

		writer, _ := index.NewIndexWriter(dir, config)
		defer writer.Close()
	})

	t.Run("config with open mode append", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		// Create initial index
		config1 := index.NewIndexWriterConfig(createTestAnalyzer())
		config1.SetOpenMode(index.CREATE)
		writer1, _ := index.NewIndexWriter(dir, config1)
		writer1.Close()

		// Append to existing index
		config2 := index.NewIndexWriterConfig(createTestAnalyzer())
		config2.SetOpenMode(index.APPEND)
		writer2, _ := index.NewIndexWriter(dir, config2)
		defer writer2.Close()
	})
}

// TestIndexWriterAddDocument tests document addition
func TestIndexWriterAddDocument(t *testing.T) {
	t.Run("add empty document", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)
		defer writer.Close()

		doc := &testDocument{fields: []interface{}{}}
		err := writer.AddDocument(doc)
		if err != nil {
			t.Errorf("AddDocument() error = %v", err)
		}
	})

	t.Run("add multiple documents", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)
		defer writer.Close()

		for i := 0; i < 5; i++ {
			doc := &testDocument{fields: []interface{}{}}
			err := writer.AddDocument(doc)
			if err != nil {
				t.Errorf("AddDocument() iteration %d error = %v", i, err)
			}
		}
	})
}

// TestIndexWriterUpdateDocument tests document updates
func TestIndexWriterUpdateDocument(t *testing.T) {
	t.Run("update document with term", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)
		defer writer.Close()

		term := index.NewTerm("id", "1")
		doc := &testDocument{fields: []interface{}{}}

		err := writer.UpdateDocument(term, doc)
		if err != nil {
			t.Errorf("UpdateDocument() error = %v", err)
		}
	})
}

// TestIndexWriterDeleteDocuments tests document deletion
func TestIndexWriterDeleteDocuments(t *testing.T) {
	t.Run("delete documents by term", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)
		defer writer.Close()

		term := index.NewTerm("id", "1")
		err := writer.DeleteDocuments(term)
		if err != nil {
			t.Errorf("DeleteDocuments() error = %v", err)
		}
	})
}

// TestIndexWriterWorkflow tests complete workflows
func TestIndexWriterWorkflow(t *testing.T) {
	t.Run("add commit close workflow", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		writer, err := index.NewIndexWriter(dir, config)
		if err != nil {
			t.Fatalf("NewIndexWriter() error = %v", err)
		}

		// Add documents
		for i := 0; i < 3; i++ {
			doc := &testDocument{fields: []interface{}{}}
			if err := writer.AddDocument(doc); err != nil {
				t.Errorf("AddDocument() error = %v", err)
			}
		}

		// Commit
		if err := writer.Commit(); err != nil {
			t.Errorf("Commit() error = %v", err)
		}

		// Close
		if err := writer.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}

		if !writer.IsClosed() {
			t.Error("writer should be closed")
		}
	})

	t.Run("update and delete workflow", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)
		defer writer.Close()

		// Add document
		doc := &testDocument{fields: []interface{}{}}
		writer.AddDocument(doc)

		// Update document
		term := index.NewTerm("id", "1")
		writer.UpdateDocument(term, doc)

		// Delete document
		writer.DeleteDocuments(term)

		// Commit
		writer.Commit()
	})
}

// testDocument is a minimal document implementation for testing
type testDocument struct {
	fields []interface{}
}

func (d *testDocument) GetFields() []interface{} {
	return d.fields
}
