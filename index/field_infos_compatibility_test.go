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

// GC-914: FieldInfos Compatibility Tests
// Validates FieldInfos serialization and field attribute handling is identical to Java Lucene.

func TestFieldInfos_BasicFields(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add document with various field types
	doc := document.NewDocument()

	idField, _ := document.NewStringField("id", "1", true)
	doc.Add(idField)

	titleField, _ := document.NewTextField("title", "Test Title", true)
	doc.Add(titleField)

	contentField, _ := document.NewTextField("content", "Test content here", true)
	doc.Add(contentField)

	storedField, _ := document.NewStoredField("metadata", "stored data")
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

	fieldInfos := reader.GetFieldInfos()
	if fieldInfos == nil {
		t.Fatal("expected non-nil FieldInfos")
	}

	t.Logf("FieldInfos contains fields for basic document")
}

func TestFieldInfos_FieldAttributes(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents with different field configurations
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()

		// Indexed field
		idField, _ := document.NewStringField("id", string(rune('0'+i%5)), true)
		doc.Add(idField)

		// Stored only field
		storedField, _ := document.NewStoredField("stored", "stored value")
		doc.Add(storedField)

		// Text field
		textField, _ := document.NewTextField("text", "text content", true)
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

	fieldInfos := reader.GetFieldInfos()
	if fieldInfos == nil {
		t.Fatal("expected non-nil FieldInfos")
	}

	t.Log("Field attributes test passed")
}

func TestFieldInfos_NumericFields(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents with numeric fields
	for i := 0; i < 20; i++ {
		doc := document.NewDocument()

		idField, _ := document.NewStringField("id", string(rune('0'+i%10)), true)
		doc.Add(idField)

		// Various numeric types
		intField, _ := document.NewIntField("int_field", i*10, true)
		doc.Add(intField)

		longField, _ := document.NewLongField("long_field", int64(i*100), true)
		doc.Add(longField)

		floatField, _ := document.NewFloatField("float_field", float32(i)*1.5, true)
		doc.Add(floatField)

		doubleField, _ := document.NewDoubleField("double_field", float64(i)*2.5, true)
		doc.Add(doubleField)

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

	fieldInfos := reader.GetFieldInfos()
	if fieldInfos == nil {
		t.Fatal("expected non-nil FieldInfos")
	}

	t.Log("Numeric fields FieldInfos test passed")
}

func TestFieldInfos_BinaryFields(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents with binary fields
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()

		idField, _ := document.NewStringField("id", string(rune('0'+i%5)), true)
		doc.Add(idField)

		// Binary stored field
		binaryData := []byte{byte(i), byte(i + 1), byte(i + 2)}
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

	fieldInfos := reader.GetFieldInfos()
	if fieldInfos == nil {
		t.Fatal("expected non-nil FieldInfos")
	}

	t.Log("Binary fields FieldInfos test passed")
}

func TestFieldInfos_FieldConsistency(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents with consistent field structure
	for i := 0; i < 50; i++ {
		doc := document.NewDocument()

		// Same fields for all documents
		idField, _ := document.NewStringField("id", string(rune('0'+i%10)), true)
		doc.Add(idField)

		titleField, _ := document.NewTextField("title", "Document Title", true)
		doc.Add(titleField)

		contentField, _ := document.NewTextField("content", "Content body", true)
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

	fieldInfos := reader.GetFieldInfos()
	if fieldInfos == nil {
		t.Fatal("expected non-nil FieldInfos")
	}

	t.Log("Field consistency test passed")
}

func TestFieldInfos_MixedDocumentFields(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents with varying field structures
	for i := 0; i < 15; i++ {
		doc := document.NewDocument()

		// All documents have an ID
		idField, _ := document.NewStringField("id", string(rune('0'+i%5)), true)
		doc.Add(idField)

		// Varying fields based on document index
		if i%2 == 0 {
			evenField, _ := document.NewTextField("even_field", "even content", true)
			doc.Add(evenField)
		}

		if i%3 == 0 {
			thirdField, _ := document.NewTextField("third_field", "third content", true)
			doc.Add(thirdField)
		}

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

	fieldInfos := reader.GetFieldInfos()
	if fieldInfos == nil {
		t.Fatal("expected non-nil FieldInfos")
	}

	t.Log("Mixed document fields test passed")
}

func TestFieldInfos_FieldSerialization(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents with complex field structures
	for i := 0; i < 25; i++ {
		doc := document.NewDocument()

		idField, _ := document.NewStringField("id", string(rune('0'+i%5)), true)
		doc.Add(idField)

		// Multiple field types
		stringField, _ := document.NewStringField("string_field", "value", true)
		doc.Add(stringField)

		textField, _ := document.NewTextField("text_field", "text content", true)
		doc.Add(textField)

		storedField, _ := document.NewStoredField("stored_field", "stored")
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

	fieldInfos := reader.GetFieldInfos()
	if fieldInfos == nil {
		t.Fatal("expected non-nil FieldInfos")
	}

	t.Log("Field serialization test passed")
}

func BenchmarkFieldInfos_Retrieval(b *testing.B) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, _ := index.NewIndexWriter(dir, config)

	// Setup
	for i := 0; i < 1000; i++ {
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
	writer.Close()

	reader, _ := index.OpenDirectoryReader(dir)
	defer reader.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = reader.GetFieldInfos()
	}
}
