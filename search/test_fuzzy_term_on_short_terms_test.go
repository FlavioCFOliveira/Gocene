// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestFuzzyTermOnShortTerms.java
//
// FuzzyQuery now expands against the terms dictionary (rmp #9), so this runs
// end-to-end. It proves the LUCENE-7439 behavior that short terms match within
// the edit distance bound.
//
// Deviations, immaterial to the assertions (hit counts): MockTokenizer(SIMPLE)
// is replaced by the WhitespaceAnalyzer (the corpus tokens are already single
// lowercase words); Lucene's IndexSearcher.count(q) is reproduced with
// Search(q, n).TotalHits.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

const fuzzyShortField = "field"

func fuzzyCountHits(t *testing.T, docs []string, q *FuzzyQuery, expected int) {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for _, s := range docs {
		doc := document.NewDocument()
		f, err := document.NewTextField(fuzzyShortField, s, true)
		if err != nil {
			t.Fatalf("NewTextField: %v", err)
		}
		doc.Add(f)
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	s := NewIndexSearcher(reader)
	top, err := s.Search(q, len(docs)+10)
	if err != nil {
		t.Fatalf("Search(%s): %v", q.String(fuzzyShortField), err)
	}
	if got := int(top.TotalHits.Value); got != expected {
		t.Errorf("%s: hits=%d, want %d", q.String(fuzzyShortField), got, expected)
	}
}

// TestFuzzyTermOnShortTerms_FuzzyTermOnShortTerms mirrors testFuzzyTermOnShortTerms:
// the edit-distance bound must allow short terms to match (LUCENE-7439).
func TestFuzzyTermOnShortTerms_FuzzyTermOnShortTerms(t *testing.T) {
	fz := func(text string, maxEdits int) *FuzzyQuery {
		return NewFuzzyQueryWithMaxEdits(index.NewTerm(fuzzyShortField, text), maxEdits)
	}

	// these work
	fuzzyCountHits(t, []string{"abc"}, fz("ab", 1), 1)
	fuzzyCountHits(t, []string{"ab"}, fz("abc", 1), 1)

	fuzzyCountHits(t, []string{"abcde"}, fz("abc", 2), 1)
	fuzzyCountHits(t, []string{"abc"}, fz("abcde", 2), 1)

	// LUCENE-7439: these now work as well:
	fuzzyCountHits(t, []string{"ab"}, fz("a", 1), 1)
	fuzzyCountHits(t, []string{"a"}, fz("ab", 1), 1)

	fuzzyCountHits(t, []string{"abc"}, fz("a", 2), 1)
	fuzzyCountHits(t, []string{"a"}, fz("abc", 2), 1)

	fuzzyCountHits(t, []string{"abcd"}, fz("ab", 2), 1)
	fuzzyCountHits(t, []string{"ab"}, fz("abcd", 2), 1)
}
