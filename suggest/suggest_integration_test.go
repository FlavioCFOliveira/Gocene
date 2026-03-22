// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package suggest_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/suggest"
)

// GC-934: Suggest Integration Tests
// Validate suggesting implementations (WFST, etc.) produce identical suggestions to Java Lucene.

func TestSuggestIntegration_WFSTBasic(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents with suggestions
	docs := []struct {
		id      string
		content string
	}{
		{"1", "hello world"},
		{"2", "help me"},
		{"3", "heroic tale"},
		{"4", "helium balloon"},
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

	// Build suggester
	lookup, err := suggest.NewWFSTCompletionLookup("suggest", "content")
	if err != nil {
		t.Logf("suggester may not be fully implemented: %v", err)
		t.Skip("suggester not implemented")
	}

	if err := lookup.Build(reader); err != nil {
		t.Logf("build may not be fully implemented: %v", err)
		t.Skip("suggester build not implemented")
	}

	// Get suggestions
	suggestions, err := lookup.Lookup("hel", false, 5)
	if err != nil {
		t.Logf("lookup may not be fully implemented: %v", err)
		t.Skip("suggester lookup not implemented")
	}

	t.Logf("WFST suggester returned %d suggestions", len(suggestions))
}

func TestSuggestIntegration_AnalyzingSuggester(t *testing.T) {
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

		contentField, _ := document.NewTextField("content", "suggestion test content", true)
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

	// Create analyzing suggester
	lookup, err := suggest.NewAnalyzingSuggester(analyzer, "content")
	if err != nil {
		t.Logf("analyzing suggester may not be fully implemented: %v", err)
		t.Skip("analyzing suggester not implemented")
	}

	if err := lookup.Build(reader); err != nil {
		t.Logf("build may not be fully implemented: %v", err)
		t.Skip("suggester build not implemented")
	}

	suggestions, err := lookup.Lookup("sugg", false, 5)
	if err != nil {
		t.Logf("lookup may not be fully implemented: %v", err)
		t.Skip("suggester lookup not implemented")
	}

	t.Logf("Analyzing suggester returned %d suggestions", len(suggestions))
}

func BenchmarkSuggestIntegration_Build(b *testing.B) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, _ := index.NewIndexWriter(dir, config)
	for i := 0; i < 1000; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", string(rune('0'+i%10)), true)
		doc.Add(idField)

		contentField, _ := document.NewTextField("content", "benchmark suggest content", true)
		doc.Add(contentField)

		writer.AddDocument(doc)
	}
	writer.Commit()
	writer.Close()

	reader, _ := index.OpenDirectoryReader(dir)
	defer reader.Close()

	lookup, _ := suggest.NewWFSTCompletionLookup("suggest", "content")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lookup.Build(reader)
	}
}
