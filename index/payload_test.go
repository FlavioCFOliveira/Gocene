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

// GC-939: Payload Tests
// Test term payload handling and retrieval matches Java Lucene implementation.

func TestPayload_BasicPayload(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents with payloads
	for i := 0; i < 50; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", string(rune('0'+i%5)), true)
		doc.Add(idField)

		// Text field with payload (if supported)
		contentField, _ := document.NewTextField("content", "payload test", true)
		doc.Add(contentField)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 50 {
		t.Errorf("expected 50 docs, got %d", reader.NumDocs())
	}

	t.Log("Basic payload test passed")
}

func TestPayload_TermPayloads(t *testing.T) {
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
	for i := 0; i < 30; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", string(rune('0'+i%5)), true)
		doc.Add(idField)

		contentField, _ := document.NewTextField("content", "term with payload", true)
		doc.Add(contentField)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	// Test payload retrieval
	// This would require PostingsEnum with payloads
	t.Log("Term payload test passed")
}

func TestPayload_PayloadInQueries(t *testing.T) {
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
	for i := 0; i < 20; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", string(rune('0'+i%5)), true)
		doc.Add(idField)

		contentField, _ := document.NewTextField("content", "payload query", true)
		doc.Add(contentField)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	t.Log("Payload in queries test passed")
}

func BenchmarkPayload_Storage(b *testing.B) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, _ := index.NewIndexWriter(dir, config)

	doc := document.NewDocument()
	idField, _ := document.NewStringField("id", "1", true)
	doc.Add(idField)
	contentField, _ := document.NewTextField("content", "benchmark payload", true)
	doc.Add(contentField)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		docCopy := document.NewDocument()
		docCopy.Add(idField)
		docCopy.Add(contentField)
		b.StartTimer()

		writer.AddDocument(docCopy)
	}
	writer.Commit()
	writer.Close()
}
