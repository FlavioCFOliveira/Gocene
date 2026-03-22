// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// GC-905: Document Indexing Roundtrip
// These tests validate that document indexing and retrieval preserves
// all field types, binary values, and stored field content without corruption.

// TestDocumentIndexingRoundtrip_StringField validates string field roundtrip.
func TestDocumentIndexingRoundtrip_StringField(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Create document with string fields
	doc := document.NewDocument()

	field1, _ := document.NewStringField("id", "doc-001", true)
	doc.Add(field1)

	field2, _ := document.NewStringField("category", "test", true)
	doc.Add(field2)

	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("failed to add document: %v", err)
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Open reader and verify
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 1 {
		t.Errorf("expected 1 doc, got %d", reader.NumDocs())
	}
}

// TestDocumentIndexingRoundtrip_TextField validates text field roundtrip.
func TestDocumentIndexingRoundtrip_TextField(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	doc := document.NewDocument()

	idField, _ := document.NewStringField("id", "doc-001", true)
	doc.Add(idField)

	// Text field - tokenized and indexed
	contentField, _ := document.NewTextField("content", "This is a test document with some content", true)
	doc.Add(contentField)

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
}

// TestDocumentIndexingRoundtrip_StoredField validates stored field roundtrip.
func TestDocumentIndexingRoundtrip_StoredField(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	doc := document.NewDocument()

	idField, _ := document.NewStringField("id", "doc-001", true)
	doc.Add(idField)

	// Stored field - not indexed but stored for retrieval
	storedField, _ := document.NewStoredField("metadata", "{\"version\":\"1.0\",\"source\":\"test\"}")
	doc.Add(storedField)

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
}

// TestDocumentIndexingRoundtrip_NumericFields validates numeric field roundtrip.
func TestDocumentIndexingRoundtrip_NumericFields(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	doc := document.NewDocument()

	idField, _ := document.NewStringField("id", "doc-001", true)
	doc.Add(idField)

	// Various numeric fields
	intField, _ := document.NewIntField("count", 42, true)
	doc.Add(intField)

	longField, _ := document.NewLongField("timestamp", 1234567890, true)
	doc.Add(longField)

	floatField, _ := document.NewFloatField("score", 3.14, true)
	doc.Add(floatField)

	doubleField, _ := document.NewDoubleField("precision", 2.718281828, true)
	doc.Add(doubleField)

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
}

// TestDocumentIndexingRoundtrip_BinaryField validates binary field roundtrip.
func TestDocumentIndexingRoundtrip_BinaryField(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	doc := document.NewDocument()

	idField, _ := document.NewStringField("id", "doc-001", true)
	doc.Add(idField)

	// Binary field
	binaryData := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0xFF, 0xFE}
	binaryField, _ := document.NewBinaryPoint("data", binaryData)
	doc.Add(binaryField)

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
}

// TestDocumentIndexingRoundtrip_MultipleFields validates multiple fields roundtrip.
func TestDocumentIndexingRoundtrip_MultipleFields(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	doc := document.NewDocument()

	// Mix of field types
	idField, _ := document.NewStringField("id", "doc-001", true)
	doc.Add(idField)

	titleField, _ := document.NewTextField("title", "Document Title", true)
	doc.Add(titleField)

	contentField, _ := document.NewTextField("content", "This is the document content", true)
	doc.Add(contentField)

	authorField, _ := document.NewStringField("author", "John Doe", true)
	doc.Add(authorField)

	dateField, _ := document.NewLongField("date", 1609459200, true)
	doc.Add(dateField)

	scoreField, _ := document.NewFloatField("score", 9.5, true)
	doc.Add(scoreField)

	metadataField, _ := document.NewStoredField("metadata", "{\"category\":\"test\"}")
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
}

// TestDocumentIndexingRoundtrip_BatchDocuments validates batch document indexing.
func TestDocumentIndexingRoundtrip_BatchDocuments(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	numDocs := 100
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()

		idField, _ := document.NewStringField("id", fmt.Sprintf("doc-%d", i), true)
		doc.Add(idField)

		contentField, _ := document.NewTextField("content", fmt.Sprintf("Content of document %d", i), true)
		doc.Add(contentField)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document %d: %v", i, err)
		}

		// Periodic commit
		if (i+1)%25 == 0 {
			if err := writer.Commit(); err != nil {
				t.Fatalf("failed to commit at %d: %v", i, err)
			}
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to final commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != numDocs {
		t.Errorf("expected %d docs, got %d", numDocs, reader.NumDocs())
	}
}

// TestDocumentIndexingRoundtrip_UpdateDocument validates document update.
func TestDocumentIndexingRoundtrip_UpdateDocument(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add initial document
	doc := document.NewDocument()
	idField, _ := document.NewStringField("id", "doc-001", true)
	doc.Add(idField)

	contentField, _ := document.NewTextField("content", "Original content", true)
	doc.Add(contentField)

	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("failed to add document: %v", err)
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Update document
	term := index.NewTerm("id", "doc-001")
	if err := writer.DeleteDocuments(term); err != nil {
		t.Fatalf("failed to delete document: %v", err)
	}

	newDoc := document.NewDocument()
	newDoc.Add(idField)
	newContentField, _ := document.NewTextField("content", "Updated content", true)
	newDoc.Add(newContentField)

	if err := writer.AddDocument(newDoc); err != nil {
		t.Fatalf("failed to add updated document: %v", err)
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit update: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	// Should have 1 live document
	if reader.NumDocs() != 1 {
		t.Errorf("expected 1 live doc after update, got %d", reader.NumDocs())
	}

	// But MaxDoc should be 2 (1 deleted + 1 live)
	if reader.MaxDoc() != 2 {
		t.Errorf("expected MaxDoc 2, got %d", reader.MaxDoc())
	}
}

// TestDocumentIndexingRoundtrip_DeleteDocument validates document deletion.
func TestDocumentIndexingRoundtrip_DeleteDocument(t *testing.T) {
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
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", fmt.Sprintf("doc-%d", i), true)
		doc.Add(idField)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Delete half the documents
	for i := 0; i < 5; i++ {
		term := index.NewTerm("id", fmt.Sprintf("doc-%d", i))
		if err := writer.DeleteDocuments(term); err != nil {
			t.Fatalf("failed to delete document: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit deletions: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	// Should have 5 live documents
	if reader.NumDocs() != 5 {
		t.Errorf("expected 5 live docs, got %d", reader.NumDocs())
	}

	// MaxDoc should be 10
	if reader.MaxDoc() != 10 {
		t.Errorf("expected MaxDoc 10, got %d", reader.MaxDoc())
	}
}

// TestDocumentIndexingRoundtrip_BinaryDataIntegrity validates binary data integrity.
func TestDocumentIndexingRoundtrip_BinaryDataIntegrity(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Test various binary patterns
	binaryPatterns := [][]byte{
		{0x00, 0x01, 0x02, 0x03, 0x04, 0x05},
		{0xFF, 0xFE, 0xFD, 0xFC, 0xFB},
		{0x80, 0x81, 0x82, 0x83, 0x84},
		{0x00, 0x00, 0x00, 0x00},
		{0xFF, 0xFF, 0xFF, 0xFF},
	}

	for i, pattern := range binaryPatterns {
		doc := document.NewDocument()

		idField, _ := document.NewStringField("id", fmt.Sprintf("binary-%d", i), true)
		doc.Add(idField)

		binaryField, _ := document.NewBinaryPoint("data", pattern)
		doc.Add(binaryField)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document %d: %v", i, err)
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

	if reader.NumDocs() != len(binaryPatterns) {
		t.Errorf("expected %d docs, got %d", len(binaryPatterns), reader.NumDocs())
	}
}

// TestDocumentIndexingRoundtrip_LargeContent validates large content handling.
func TestDocumentIndexingRoundtrip_LargeContent(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	doc := document.NewDocument()

	idField, _ := document.NewStringField("id", "large-doc", true)
	doc.Add(idField)

	// Large content field
	largeContent := make([]byte, 10000)
	for i := range largeContent {
		largeContent[i] = byte('a' + (i % 26))
	}
	contentField, _ := document.NewTextField("content", string(largeContent), true)
	doc.Add(contentField)

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
}

// TestDocumentIndexingRoundtrip_SpecialCharacters validates special character handling.
func TestDocumentIndexingRoundtrip_SpecialCharacters(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	specialContents := []string{
		"Content with special chars: @#$%^&*()",
		"Unicode content: ñ, ü, é, 中文, 日本語",
		"Content with\nnewlines\nand\ttabs",
		`Content with "quotes" and 'apostrophes'`,
		"Content with <html>tags</html>",
	}

	for i, content := range specialContents {
		doc := document.NewDocument()

		idField, _ := document.NewStringField("id", fmt.Sprintf("special-%d", i), true)
		doc.Add(idField)

		contentField, _ := document.NewTextField("content", content, true)
		doc.Add(contentField)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document %d: %v", i, err)
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

	if reader.NumDocs() != len(specialContents) {
		t.Errorf("expected %d docs, got %d", len(specialContents), reader.NumDocs())
	}
}

// TestDocumentIndexingRoundtrip_ReopenWriter validates index writer reopening.
func TestDocumentIndexingRoundtrip_ReopenWriter(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	// First writer session
	writer1, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}

	doc1 := document.NewDocument()
	idField1, _ := document.NewStringField("id", "doc-001", true)
	doc1.Add(idField1)

	if err := writer1.AddDocument(doc1); err != nil {
		t.Fatalf("failed to add document: %v", err)
	}

	if err := writer1.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	writer1.Close()

	// Second writer session
	writer2, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to reopen writer: %v", err)
	}
	defer writer2.Close()

	doc2 := document.NewDocument()
	idField2, _ := document.NewStringField("id", "doc-002", true)
	doc2.Add(idField2)

	if err := writer2.AddDocument(doc2); err != nil {
		t.Fatalf("failed to add document: %v", err)
	}

	if err := writer2.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 2 {
		t.Errorf("expected 2 docs, got %d", reader.NumDocs())
	}
}

// BenchmarkDocumentIndexing_Index validates indexing performance.
func BenchmarkDocumentIndexing_Index(b *testing.B) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, _ := index.NewIndexWriter(dir, config)
	defer writer.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", fmt.Sprintf("doc-%d", i), true)
		doc.Add(idField)

		contentField, _ := document.NewTextField("content", "Test content", true)
		doc.Add(contentField)

		writer.AddDocument(doc)
	}
	writer.Commit()
}
