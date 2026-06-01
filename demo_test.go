// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/TestDemo.java

package gocene

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestDemo mirrors testDemo (Lucene 10.4.0).
// It indexes one document and verifies retrieval via TermQuery and PhraseQuery
// against an in-memory ByteBuffersDirectory, mirroring the FSDirectory variant
// in the original but without the filesystem dependency.
func TestDemo(t *testing.T) {
	const longTerm = "longtermlongtermlongtermlongtermlongtermlongtermlongtermlong" +
		"termlongtermlongtermlongtermlongtermlongtermlongtermlongterm" +
		"longtermlongtermlongterm"
	const text = "This is the text to be indexed. " + longTerm

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewStandardAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	iw, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	doc := document.NewDocument()
	f, err := document.NewTextField("fieldname", text, true)
	if err != nil {
		t.Fatalf("NewTextField: %v", err)
	}
	doc.Add(f)

	if err := iw.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := iw.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := iw.Close(); err != nil {
		t.Fatalf("writer Close: %v", err)
	}

	// Now search the index.
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)

	// TermQuery for the long term — must find exactly 1 hit.
	longTermQuery := search.NewTermQuery(index.NewTerm("fieldname", longTerm))
	longTermHits, err := searcher.Search(longTermQuery, 1)
	if err != nil {
		t.Fatalf("Search(longTerm): %v", err)
	}
	if longTermHits.TotalHits.Value != 1 {
		t.Errorf("Search(longTerm): got %d hits, want 1", longTermHits.TotalHits.Value)
	}

	// TermQuery for "text" — must find exactly 1 hit.
	textQuery := search.NewTermQuery(index.NewTerm("fieldname", "text"))
	hits, err := searcher.Search(textQuery, 1)
	if err != nil {
		t.Fatalf("Search(text): %v", err)
	}
	if hits.TotalHits.Value != 1 {
		t.Fatalf("Search(text): got %d hits, want 1", hits.TotalHits.Value)
	}

	// Retrieve the stored field from the matching document.
	if len(hits.ScoreDocs) == 0 {
		t.Fatal("expected at least one ScoreDoc")
	}
	hitDoc, err := searcher.Doc(hits.ScoreDocs[0].Doc)
	if err != nil {
		t.Fatalf("Doc(%d): %v", hits.ScoreDocs[0].Doc, err)
	}
	field := hitDoc.Get("fieldname")
	if field == nil {
		t.Fatal("retrieved doc missing fieldname")
	}
	if field.StringValue() != text {
		t.Errorf("stored text mismatch:\n  got  %q\n  want %q", field.StringValue(), text)
	}

	// PhraseQuery "to be" — must find exactly 1 hit.
	phraseQuery := search.NewPhraseQueryWithStrings("fieldname", "to", "be")
	phraseHits, err := searcher.Search(phraseQuery, 1)
	if err != nil {
		t.Fatalf("Search(phrase): %v", err)
	}
	if phraseHits.TotalHits.Value != 1 {
		t.Errorf("Search(phrase): got %d hits, want 1", phraseHits.TotalHits.Value)
	}
}
