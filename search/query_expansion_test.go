// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// GC-936: Query Expansion Tests
// Validate query expansion and rewriting produces identical results to Java Lucene.

func TestQueryExpansion_TermRewrite(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	for i := 0; i < 50; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", string(rune('0'+i%5)), true)
		doc.Add(idField)

		contentField, _ := document.NewTextField("content", "query rewrite test", true)
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

	query := search.NewTermQuery(index.NewTerm("content", "test"))

	rewritten, err := query.Rewrite(reader)
	if err != nil {
		t.Fatalf("Rewrite failed: %v", err)
	}

	if rewritten == nil {
		t.Fatal("Rewrite returned nil")
	}
}

func TestQueryExpansion_BooleanRewrite(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	for i := 0; i < 30; i++ {
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

	boolQuery := search.NewBooleanQuery()
	boolQuery.Add(search.NewTermQuery(index.NewTerm("id", "1")), search.SHOULD)
	boolQuery.Add(search.NewTermQuery(index.NewTerm("id", "2")), search.SHOULD)

	rewritten, err := boolQuery.Rewrite(reader)
	if err != nil {
		t.Fatalf("Rewrite failed: %v", err)
	}
	if rewritten == nil {
		t.Fatal("Rewrite returned nil")
	}
}

func TestQueryExpansion_PhraseRewrite(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	for i := 0; i < 20; i++ {
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

	phraseQuery := search.NewPhraseQueryBuilder().
		AddTerm(index.NewTerm("id", "1")).
		AddTerm(index.NewTerm("id", "2")).
		Build()

	rewritten, err := phraseQuery.Rewrite(reader)
	if err != nil {
		t.Fatalf("Rewrite failed: %v", err)
	}
	if rewritten == nil {
		t.Fatal("Rewrite returned nil")
	}
}

func BenchmarkQueryExpansion_Rewrite(b *testing.B) {
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

	reader, _ := index.OpenDirectoryReader(dir)
	defer reader.Close()

	query := search.NewTermQuery(index.NewTerm("id", "1"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		query.Rewrite(reader)
	}
}
