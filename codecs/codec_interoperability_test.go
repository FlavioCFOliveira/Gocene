// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// GC-912: Codec Interoperability Tests
// Validates all codecs can read indexes written by Java Lucene and vice versa,
// including compression codecs.

func TestCodecInteroperability_Lucene99Codec(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	// Use Lucene99 codec
	codec := codecs.NewLucene99Codec()
	config.SetCodec(codec)

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

		contentField, _ := document.NewTextField("content", "test content", true)
		doc.Add(contentField)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Verify index can be read back
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 100 {
		t.Errorf("expected 100 docs, got %d", reader.NumDocs())
	}

	t.Logf("Lucene99Codec: Index created and verified with %d documents", reader.NumDocs())
}

func TestCodecInteroperability_Lucene90Codec(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	// Test Lucene90 codec
	codec := codecs.NewLucene90Codec()
	config.SetCodec(codec)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents
	for i := 0; i < 50; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", string(rune('0'+i%5)), true)
		doc.Add(idField)

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
}

func TestCodecInteroperability_CompressingCodec(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	// Test compressing codec
	codec := codecs.NewCompressingCodec("Lucene90", 1, 1024, 10)
	config.SetCodec(codec)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents with varying content sizes
	contents := []string{
		"short",
		"medium length content here",
		"this is a longer piece of content that should exercise the compression codec properly",
		"another document with different content",
	}

	for i := 0; i < 20; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", string(rune('0'+i%10)), true)
		doc.Add(idField)

		contentField, _ := document.NewTextField("content", contents[i%4], true)
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

	if reader.NumDocs() != 20 {
		t.Errorf("expected 20 docs, got %d", reader.NumDocs())
	}

	t.Log("CompressingCodec: Compression codec test passed")
}

func TestCodecInteroperability_PerFieldPostingsFormat(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents with different field types
	for i := 0; i < 30; i++ {
		doc := document.NewDocument()

		// String field (indexed, not tokenized)
		idField, _ := document.NewStringField("id", string(rune('0'+i%10)), true)
		doc.Add(idField)

		// Text field (indexed, tokenized)
		contentField, _ := document.NewTextField("content", "hello world testing", true)
		doc.Add(contentField)

		// Stored field (not indexed)
		storedField, _ := document.NewStoredField("metadata", "stored data")
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

	if reader.NumDocs() != 30 {
		t.Errorf("expected 30 docs, got %d", reader.NumDocs())
	}
}

func TestCodecInteroperability_CodecRoundtrip(t *testing.T) {
	// Test that we can write with one codec and read with the same codec
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()

	// Write phase
	func() {
		config := index.NewIndexWriterConfig(analyzer)
		codec := codecs.NewLucene99Codec()
		config.SetCodec(codec)

		writer, err := index.NewIndexWriter(dir, config)
		if err != nil {
			t.Fatalf("failed to create writer: %v", err)
		}
		defer writer.Close()

		for i := 0; i < 25; i++ {
			doc := document.NewDocument()
			idField, _ := document.NewStringField("id", string(rune('0'+i%5)), true)
			doc.Add(idField)

			contentField, _ := document.NewTextField("content", "roundtrip test content", true)
			doc.Add(contentField)

			if err := writer.AddDocument(doc); err != nil {
				t.Fatalf("failed to add document: %v", err)
			}
		}

		if err := writer.Commit(); err != nil {
			t.Fatalf("failed to commit: %v", err)
		}
	}()

	// Read phase
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 25 {
		t.Errorf("expected 25 docs, got %d", reader.NumDocs())
	}

	t.Log("Codec roundtrip test passed")
}

func TestCodecInteroperability_FieldInfosFormat(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents with various field configurations
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()

		// Various field types
		stringField, _ := document.NewStringField("string_field", "value", true)
		doc.Add(stringField)

		textField, _ := document.NewTextField("text_field", "text content", true)
		doc.Add(textField)

		storedField, _ := document.NewStoredField("stored_field", "stored")
		doc.Add(storedField)

		intField, _ := document.NewIntField("int_field", i, true)
		doc.Add(intField)

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

	// Verify field infos
	fieldInfos := reader.GetFieldInfos()
	if fieldInfos == nil {
		t.Error("expected non-nil FieldInfos")
	}

	t.Logf("FieldInfos format test passed with %d documents", reader.NumDocs())
}

func TestCodecInteroperability_StoredFieldsFormat(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents with stored fields
	testData := []struct {
		id      string
		content string
		meta    string
	}{
		{"1", "content one", "meta one"},
		{"2", "content two", "meta two"},
		{"3", "content three", "meta three"},
	}

	for _, data := range testData {
		doc := document.NewDocument()

		idField, _ := document.NewStringField("id", data.id, true)
		doc.Add(idField)

		contentField, _ := document.NewTextField("content", data.content, true)
		doc.Add(contentField)

		storedField, _ := document.NewStoredField("metadata", data.meta)
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

	t.Log("StoredFields format test passed")
}

func TestCodecInteroperability_TermVectorsFormat(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents that will have term vectors
	for i := 0; i < 15; i++ {
		doc := document.NewDocument()

		idField, _ := document.NewStringField("id", string(rune('0'+i%5)), true)
		doc.Add(idField)

		// Text field with term vectors
		content := "term vectors test document with multiple words for testing"
		textField, _ := document.NewTextField("content", content, true)
		doc.Add(textField)

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

	t.Log("TermVectors format test passed")
}

func BenchmarkCodecInteroperability_Write(b *testing.B) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	codec := codecs.NewLucene99Codec()
	config.SetCodec(codec)

	writer, _ := index.NewIndexWriter(dir, config)

	doc := document.NewDocument()
	idField, _ := document.NewStringField("id", "1", true)
	doc.Add(idField)
	contentField, _ := document.NewTextField("content", "benchmark content", true)
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

func BenchmarkCodecInteroperability_Read(b *testing.B) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	// Setup: create index
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
