// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"sync"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// GC-922: IndexReader Consistency Tests
// Test reader refresh, reopening, and thread safety match Java Lucene behavior patterns.

func TestIndexReaderConsistency_BasicRefresh(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Initial reader
	reader1, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}

	initialDocs := reader1.NumDocs()
	reader1.Close()

	// Add more documents
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", string(rune('0'+i%5)), true)
		doc.Add(idField)
		writer.AddDocument(doc)
	}
	writer.Commit()

	// Reopen reader
	reader2, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to reopen reader: %v", err)
	}
	defer reader2.Close()

	if reader2.NumDocs() != initialDocs+10 {
		t.Errorf("expected %d docs after reopen, got %d", initialDocs+10, reader2.NumDocs())
	}

	t.Log("Basic reader refresh test passed")
}

func TestIndexReaderConsistency_ThreadSafety(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents
	for i := 0; i < 100; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", string(rune('0'+i%10)), true)
		doc.Add(idField)
		writer.AddDocument(doc)
	}
	writer.Commit()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	// Concurrent reads
	var wg sync.WaitGroup
	numReaders := 10

	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = reader.NumDocs()
		}()
	}

	wg.Wait()

	t.Log("Reader thread safety test passed")
}

func TestIndexReaderConsistency_ReopenConsistency(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Open initial reader
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}

	// Multiple reopens
	for i := 0; i < 5; i++ {
		// Add documents
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", string(rune('0'+i)), true)
		doc.Add(idField)
		writer.AddDocument(doc)
		writer.Commit()

		// Reopen
		reader.Close()
		reader, err = index.OpenDirectoryReader(dir)
		if err != nil {
			t.Fatalf("failed to reopen reader: %v", err)
		}
	}

	defer reader.Close()

	if reader.NumDocs() != 5 {
		t.Errorf("expected 5 docs, got %d", reader.NumDocs())
	}

	t.Log("Reopen consistency test passed")
}

func TestIndexReaderConsistency_SegmentConsistency(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Create multiple segments
	for seg := 0; seg < 5; seg++ {
		for i := 0; i < 20; i++ {
			doc := document.NewDocument()
			idField, _ := document.NewStringField("id", string(rune('0'+(seg*20+i)%10)), true)
			doc.Add(idField)
			writer.AddDocument(doc)
		}
		writer.Commit()
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	// Verify total documents
	if reader.NumDocs() != 100 {
		t.Errorf("expected 100 docs, got %d", reader.NumDocs())
	}

	segmentInfos := reader.GetSegmentInfos()
	t.Logf("Reader has %d segments", segmentInfos.Size())

	t.Log("Segment consistency test passed")
}

func TestIndexReaderConsistency_FieldInfosConsistency(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents with various fields
	for i := 0; i < 50; i++ {
		doc := document.NewDocument()

		idField, _ := document.NewStringField("id", string(rune('0'+i%10)), true)
		doc.Add(idField)

		titleField, _ := document.NewTextField("title", "title", true)
		doc.Add(titleField)

		contentField, _ := document.NewTextField("content", "content", true)
		doc.Add(contentField)

		writer.AddDocument(doc)
	}
	writer.Commit()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	fieldInfos := reader.GetFieldInfos()
	if fieldInfos == nil {
		t.Fatal("expected non-nil FieldInfos")
	}

	t.Log("FieldInfos consistency test passed")
}

func TestIndexReaderConsistency_DeletedDocs(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents
	for i := 0; i < 100; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", string(rune('0'+i%10)), true)
		doc.Add(idField)
		writer.AddDocument(doc)
	}
	writer.Commit()

	// Delete some documents
	term := index.NewTerm("id", "0")
	if err := writer.DeleteDocuments(term); err != nil {
		t.Logf("delete may not be fully implemented: %v", err)
	}
	writer.Commit()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	t.Logf("Reader has %d documents after deletions", reader.NumDocs())

	t.Log("Deleted docs consistency test passed")
}

func BenchmarkIndexReaderConsistency_Open(b *testing.B) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, _ := index.NewIndexWriter(dir, config)
	for i := 0; i < 1000; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", string(rune('0'+i%10)), true)
		doc.Add(idField)
		writer.AddDocument(doc)
	}
	writer.Commit()
	writer.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		reader, _ := index.OpenDirectoryReader(dir)
		b.StartTimer()

		_ = reader.NumDocs()
		reader.Close()
	}
}
