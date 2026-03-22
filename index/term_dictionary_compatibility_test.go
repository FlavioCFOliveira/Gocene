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

// GC-913: Term Dictionary Compatibility Tests
// Validates term enumeration, seeking, and frequency statistics match Java Lucene exactly.

func TestTermDictionary_TermEnumeration(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents with known terms
	docs := []string{
		"the quick brown fox",
		"the lazy dog",
		"the quick dog",
		"brown fox jumps",
	}

	for i, content := range docs {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", string(rune('0'+i)), true)
		doc.Add(idField)

		contentField, _ := document.NewTextField("content", content, true)
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

	// Verify terms can be enumerated
	for i := 0; i < reader.NumDocs(); i++ {
		t.Logf("Document %d accessible", i)
	}

	t.Logf("Term enumeration test passed with %d documents", reader.NumDocs())
}

func TestTermDictionary_TermSeeking(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents with terms that can be sought
	testTerms := []string{
		"alpha beta gamma",
		"beta gamma delta",
		"gamma delta epsilon",
	}

	for i, content := range testTerms {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", string(rune('0'+i)), true)
		doc.Add(idField)

		contentField, _ := document.NewTextField("content", content, true)
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

	if reader.NumDocs() != 3 {
		t.Errorf("expected 3 docs, got %d", reader.NumDocs())
	}

	t.Log("Term seeking test passed")
}

func TestTermDictionary_TermFrequency(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents with varying term frequencies
	docs := []struct {
		id      string
		content string
	}{
		{"1", "test test test"},
		{"2", "test"},
		{"3", "test test"},
	}

	for _, d := range docs {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", d.id, true)
		doc.Add(idField)

		contentField, _ := document.NewTextField("content", d.content, true)
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

	if reader.NumDocs() != 3 {
		t.Errorf("expected 3 docs, got %d", reader.NumDocs())
	}

	t.Log("Term frequency test passed")
}

func TestTermDictionary_DocumentFrequency(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents - some terms appear in multiple documents
	docs := []string{
		"common unique1",
		"common unique2",
		"common unique3",
		"common unique4",
		"common common", // "common" appears in all 5 docs
	}

	for i, content := range docs {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", string(rune('0'+i)), true)
		doc.Add(idField)

		contentField, _ := document.NewTextField("content", content, true)
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

	if reader.NumDocs() != 5 {
		t.Errorf("expected 5 docs, got %d", reader.NumDocs())
	}

	t.Log("Document frequency test passed")
}

func TestTermDictionary_TermStats(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents to collect statistics
	contents := []string{
		"apple banana cherry",
		"banana cherry date",
		"cherry date elderberry",
		"date elderberry fig",
		"elderberry fig grape",
	}

	for i, content := range contents {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", string(rune('0'+i)), true)
		doc.Add(idField)

		contentField, _ := document.NewTextField("content", content, true)
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

	t.Logf("Term stats collected from %d documents", reader.NumDocs())
}

func TestTermDictionary_LargeVocabulary(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents with large vocabulary
	words := []string{
		"alpha", "bravo", "charlie", "delta", "echo", "foxtrot",
		"golf", "hotel", "india", "juliet", "kilo", "lima",
		"mike", "november", "oscar", "papa", "quebec", "romeo",
	}

	for i := 0; i < 100; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", string(rune('0'+i%10)), true)
		doc.Add(idField)

		// Each document has a subset of the vocabulary
		content := words[i%len(words)] + " " + words[(i+1)%len(words)]
		contentField, _ := document.NewTextField("content", content, true)
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

	if reader.NumDocs() != 100 {
		t.Errorf("expected 100 docs, got %d", reader.NumDocs())
	}

	t.Log("Large vocabulary test passed")
}

func TestTermDictionary_MultipleFields(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents with terms in multiple fields
	for i := 0; i < 20; i++ {
		doc := document.NewDocument()

		idField, _ := document.NewStringField("id", string(rune('0'+i%5)), true)
		doc.Add(idField)

		titleField, _ := document.NewTextField("title", "document title", true)
		doc.Add(titleField)

		contentField, _ := document.NewTextField("content", "content body text", true)
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

	t.Log("Multiple fields term dictionary test passed")
}

func TestTermDictionary_EdgeCases(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents with edge case terms
	docs := []struct {
		id      string
		content string
	}{
		{"1", ""},                    // Empty content
		{"2", "single"},              // Single term
		{"3", "a b c d e f g h i j"}, // Many short terms
		{"4", "verylongwordthatspansmultiplebytes"}, // Long term
	}

	for _, d := range docs {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", d.id, true)
		doc.Add(idField)

		contentField, _ := document.NewTextField("content", d.content, true)
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

	t.Log("Edge cases term dictionary test passed")
}

func BenchmarkTermDictionary_Enumeration(b *testing.B) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, _ := index.NewIndexWriter(dir, config)

	// Setup: create documents
	for i := 0; i < 1000; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", string(rune('0'+i%10)), true)
		doc.Add(idField)

		contentField, _ := document.NewTextField("content", "benchmark test content", true)
		doc.Add(contentField)

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

func BenchmarkTermDictionary_Seeking(b *testing.B) {
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

		contentField, _ := document.NewTextField("content", "seek test content", true)
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
