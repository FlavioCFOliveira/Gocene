// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestDirectoryReader_IsCurrent tests DirectoryReader.IsCurrent()
// Ported from Apache Lucene's org.apache.lucene.index.TestDirectoryReader.testIsCurrent()
func TestDirectoryReader_IsCurrent(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Initial index creation
	config := index.NewIndexWriterConfig(createTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	doc := &testDocument{fields: []interface{}{}}
	err = writer.AddDocument(doc)
	if err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}
	writer.Close()

	// Open reader
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open DirectoryReader: %v", err)
	}
	defer reader.Close()

	// isCurrent should be true initially
	isCurrent, err := reader.IsCurrent()
	if err != nil {
		t.Fatalf("IsCurrent() error: %v", err)
	}
	if !isCurrent {
		t.Error("Expected reader.IsCurrent() to be true")
	}

	// Modify index
	config2 := index.NewIndexWriterConfig(createTestAnalyzer())
	config2.SetOpenMode(index.APPEND)
	writer2, err := index.NewIndexWriter(dir, config2)
	if err != nil {
		t.Fatalf("Failed to open IndexWriter for append: %v", err)
	}
	err = writer2.AddDocument(doc)
	if err != nil {
		t.Fatalf("Failed to add second document: %v", err)
	}
	writer2.Commit()
	writer2.Close()

	// isCurrent should be false after modification
	isCurrent, err = reader.IsCurrent()
	if err != nil {
		t.Fatalf("IsCurrent() error after mod: %v", err)
	}
	if isCurrent {
		t.Error("Expected reader.IsCurrent() to be false after index modification")
	}
}

// TestDirectoryReader_Basic tests basic DirectoryReader functionality
func TestDirectoryReader_Basic(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	doc := &testDocument{fields: []interface{}{}}
	writer.AddDocument(doc)
	writer.AddDocument(doc)
	writer.Commit()
	writer.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open DirectoryReader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 2 {
		t.Errorf("Expected 2 documents, got %d", reader.NumDocs())
	}

	if reader.MaxDoc() != 2 {
		t.Errorf("Expected MaxDoc 2, got %d", reader.MaxDoc())
	}
}

// TestSegmentReader_Basic tests basic SegmentReader functionality
// Ported from Apache Lucene's org.apache.lucene.index.TestSegmentReader.test()
func TestSegmentReader_Basic(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	doc := &testDocument{fields: []interface{}{}}
	writer.AddDocument(doc)
	writer.Commit()
	writer.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open DirectoryReader: %v", err)
	}
	defer reader.Close()

	segmentReaders := reader.GetSegmentReaders()
	if len(segmentReaders) == 0 {
		t.Fatal("Expected at least one segment reader")
	}

	sr := segmentReaders[0]
	if sr.NumDocs() != 1 {
		t.Errorf("Expected 1 doc in segment, got %d", sr.NumDocs())
	}

	if sr.GetSegmentCommitInfo() == nil {
		t.Error("Expected SegmentCommitInfo to be present")
	}
}

// TestFilterDirectoryReader tests basic FilterDirectoryReader functionality
func TestFilterDirectoryReader(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	doc := &testDocument{fields: []interface{}{}}
	writer.AddDocument(doc)
	writer.Commit()
	writer.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open DirectoryReader: %v", err)
	}
	defer reader.Close()

	wrapped := index.NewFilterDirectoryReader(reader)
	if wrapped.NumDocs() != 1 {
		t.Errorf("Expected 1 doc in wrapped reader, got %d", wrapped.NumDocs())
	}

	if wrapped.GetDelegate() != reader {
		t.Error("Expected GetDelegate() to return the original reader")
	}
}
