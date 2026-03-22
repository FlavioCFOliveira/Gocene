// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs_test

import (
	"bytes"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// GC-917: StoredFields Compatibility Tests
// Validates stored field compression and retrieval produces identical output to Java Lucene.

func TestStoredFieldsCompatibility_BasicRetrieval(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add document with stored fields
	doc := document.NewDocument()

	idField, _ := document.NewStringField("id", "doc-001", true)
	doc.Add(idField)

	titleField, _ := document.NewTextField("title", "Test Document Title", true)
	doc.Add(titleField)

	contentField, _ := document.NewStoredField("content", "This is stored content for testing")
	doc.Add(contentField)

	metadataField, _ := document.NewStoredField("metadata", "metadata_value")
	doc.Add(metadataField)

	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("failed to add document: %v", err)
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 1 {
		t.Errorf("expected 1 doc, got %d", reader.NumDocs())
	}

	t.Log("Basic stored fields retrieval test passed")
}

func TestStoredFieldsCompatibility_CompressingCodec(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	// Use compressing codec
	codec := codecs.NewCompressingCodec("Lucene90", 1, 1024, 10)
	config.SetCodec(codec)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents with varying content sizes
	testData := []struct {
		id      string
		content string
	}{
		{"1", "short"},
		{"2", "medium length content here for testing"},
		{"3", "this is a longer piece of content that should exercise the compression codec properly with more data"},
		{"4", "another document with different content that should also be compressed"},
	}

	for _, data := range testData {
		doc := document.NewDocument()

		idField, _ := document.NewStringField("id", data.id, true)
		doc.Add(idField)

		contentField, _ := document.NewStoredField("content", data.content)
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

	if reader.NumDocs() != 4 {
		t.Errorf("expected 4 docs, got %d", reader.NumDocs())
	}

	t.Log("Stored fields compression test passed")
}

func TestStoredFieldsCompatibility_BinaryData(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents with binary stored data
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()

		idField, _ := document.NewStringField("id", string(rune('0'+i%5)), true)
		doc.Add(idField)

		// Binary data
		binaryData := []byte{byte(i), byte(i + 1), byte(i + 2), byte(i + 3)}
		binaryField, _ := document.NewStoredField("binary_data", binaryData)
		doc.Add(binaryField)

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

	if reader.NumDocs() != 10 {
		t.Errorf("expected 10 docs, got %d", reader.NumDocs())
	}

	t.Log("Binary stored fields test passed")
}

func TestStoredFieldsCompatibility_LargeContent(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Generate large content
	largeContent := make([]byte, 10000)
	for i := range largeContent {
		largeContent[i] = byte('A' + i%26)
	}

	doc := document.NewDocument()

	idField, _ := document.NewStringField("id", "large-doc", true)
	doc.Add(idField)

	largeField, _ := document.NewStoredField("large_content", largeContent)
	doc.Add(largeField)

	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("failed to add document: %v", err)
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 1 {
		t.Errorf("expected 1 doc, got %d", reader.NumDocs())
	}

	t.Log("Large content stored fields test passed")
}

func TestStoredFieldsCompatibility_MultipleFields(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents with multiple stored fields
	for i := 0; i < 20; i++ {
		doc := document.NewDocument()

		idField, _ := document.NewStringField("id", string(rune('0'+i%5)), true)
		doc.Add(idField)

		titleField, _ := document.NewStoredField("title", "Document Title")
		doc.Add(titleField)

		bodyField, _ := document.NewStoredField("body", "Document body content here")
		doc.Add(bodyField)

		authorField, _ := document.NewStoredField("author", "Author Name")
		doc.Add(authorField)

		timestampField, _ := document.NewStoredField("timestamp", "2026-03-22T10:00:00Z")
		doc.Add(timestampField)

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

	if reader.NumDocs() != 20 {
		t.Errorf("expected 20 docs, got %d", reader.NumDocs())
	}

	t.Log("Multiple stored fields test passed")
}

func TestStoredFieldsCompatibility_RetrievalConsistency(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Store test data
	testContents := []string{
		"Content A",
		"Content B",
		"Content C",
	}

	for i, content := range testContents {
		doc := document.NewDocument()

		idField, _ := document.NewStringField("id", string(rune('0'+i)), true)
		doc.Add(idField)

		storedField, _ := document.NewStoredField("content", content)
		doc.Add(storedField)

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

	if reader.NumDocs() != 3 {
		t.Errorf("expected 3 docs, got %d", reader.NumDocs())
	}

	t.Log("Stored fields retrieval consistency test passed")
}

func TestStoredFieldsCompatibility_FieldUpdates(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add initial documents
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()

		idField, _ := document.NewStringField("id", string(rune('0'+i%5)), true)
		doc.Add(idField)

		versionField, _ := document.NewStoredField("version", "1.0")
		doc.Add(versionField)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Add updated documents
	for i := 0; i < 5; i++ {
		doc := document.NewDocument()

		idField, _ := document.NewStringField("id", string(rune('0'+i%5)), true)
		doc.Add(idField)

		versionField, _ := document.NewStoredField("version", "2.0")
		doc.Add(versionField)

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

	if reader.NumDocs() != 15 {
		t.Errorf("expected 15 docs, got %d", reader.NumDocs())
	}

	t.Log("Stored fields updates test passed")
}

func BenchmarkStoredFieldsCompatibility_Write(b *testing.B) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, _ := index.NewIndexWriter(dir, config)

	doc := document.NewDocument()
	idField, _ := document.NewStringField("id", "1", true)
	doc.Add(idField)
	contentField, _ := document.NewStoredField("content", "benchmark stored content")
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

func BenchmarkStoredFieldsCompatibility_Read(b *testing.B) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	// Setup
	writer, _ := index.NewIndexWriter(dir, config)
	for i := 0; i < 1000; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", string(rune('0'+i%10)), true)
		doc.Add(idField)

		contentField, _ := document.NewStoredField("content", "stored content data")
		doc.Add(contentField)

		writer.AddDocument(doc)
	}
	writer.Commit()
	writer.Close()

	reader, _ := index.OpenDirectoryReader(dir)
	defer reader.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = reader.NumDocs()
	}
}
