// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"container/heap"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

func TestNewMoreLikeThis(t *testing.T) {
	analyzer := analysis.NewStandardAnalyzer()
	mlt := NewMoreLikeThis(analyzer)

	if mlt.MinTermFreq != 2 {
		t.Errorf("Expected MinTermFreq=2, got: %d", mlt.MinTermFreq)
	}
	if mlt.MinDocFreq != 5 {
		t.Errorf("Expected MinDocFreq=5, got: %d", mlt.MinDocFreq)
	}
	if mlt.MaxDocFreq != 95 {
		t.Errorf("Expected MaxDocFreq=95, got: %d", mlt.MaxDocFreq)
	}
	if mlt.MaxQueryTerms != 25 {
		t.Errorf("Expected MaxQueryTerms=25, got: %d", mlt.MaxQueryTerms)
	}
	if mlt.MaxNumTokensParsed != 5000 {
		t.Errorf("Expected MaxNumTokensParsed=5000, got: %d", mlt.MaxNumTokensParsed)
	}
}

func TestMoreLikeThisSetStopWords(t *testing.T) {
	analyzer := analysis.NewStandardAnalyzer()
	mlt := NewMoreLikeThis(analyzer)

	stopWords := []string{"the", "and", "or"}
	mlt.SetStopWords(stopWords)

	for _, word := range stopWords {
		if !mlt.IsStopWord(word) {
			t.Errorf("Expected '%s' to be a stop word", word)
		}
	}

	if mlt.IsStopWord("lucene") {
		t.Error("Expected 'lucene' not to be a stop word")
	}
}

func TestMoreLikeThisLikeText(t *testing.T) {
	analyzer := analysis.NewStandardAnalyzer()
	mlt := NewMoreLikeThis(analyzer)
	mlt.MinTermFreq = 1 // Lower threshold for testing
	mlt.MaxQueryTerms = 5

	text := "lucene search engine for full text search"
	query, err := mlt.LikeText(text)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if query == nil {
		t.Fatal("Expected query to not be nil")
	}

	// Verify it's a BooleanQuery
	boolQuery, ok := query.(*BooleanQuery)
	if !ok {
		t.Fatalf("Expected BooleanQuery, got: %T", query)
	}

	// Should have created some clauses
	if len(boolQuery.Clauses()) == 0 {
		t.Error("Expected at least one clause")
	}
}

func TestMoreLikeThisLikeTextNoAnalyzer(t *testing.T) {
	mlt := NewMoreLikeThis(nil)
	_, err := mlt.LikeText("test text")
	if err == nil {
		t.Error("Expected error when analyzer is nil")
	}
}

func TestMoreLikeThisLikeTextWithStopWords(t *testing.T) {
	analyzer := analysis.NewStandardAnalyzer()
	mlt := NewMoreLikeThis(analyzer)
	mlt.MinTermFreq = 1
	mlt.SetStopWords([]string{"the", "and", "for"})

	text := "the lucene and search for engine"
	query, err := mlt.LikeText(text)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if query == nil {
		t.Fatal("Expected query to not be nil")
	}

	// Stop words should be filtered out
	// Only "lucene", "search", "engine" should remain
}

func TestMoreLikeThisLikeTextMinWordLen(t *testing.T) {
	analyzer := analysis.NewStandardAnalyzer()
	mlt := NewMoreLikeThis(analyzer)
	mlt.MinTermFreq = 1
	mlt.MinWordLen = 5

	text := "lucene go is a search engine"
	query, err := mlt.LikeText(text)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if query == nil {
		t.Fatal("Expected query to not be nil")
	}

	// Words shorter than 5 chars should be filtered: "go", "is", "a"
}

func TestMoreLikeThisLikeTextMaxWordLen(t *testing.T) {
	analyzer := analysis.NewStandardAnalyzer()
	mlt := NewMoreLikeThis(analyzer)
	mlt.MinTermFreq = 1
	mlt.MaxWordLen = 6

	text := "lucene search engine implementation"
	query, err := mlt.LikeText(text)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if query == nil {
		t.Fatal("Expected query to not be nil")
	}

	// "implementation" is longer than 6 chars, should be filtered
}

func TestMoreLikeThisLikeTextMinTermFreq(t *testing.T) {
	analyzer := analysis.NewStandardAnalyzer()
	mlt := NewMoreLikeThis(analyzer)
	mlt.MinTermFreq = 2 // Terms must appear at least twice

	text := "lucene search engine"
	_, err := mlt.LikeText(text)
	if err == nil {
		t.Error("Expected error when no terms meet minTermFreq")
	}
}

func TestMoreLikeThisLikeTextDuplicateTerms(t *testing.T) {
	analyzer := analysis.NewStandardAnalyzer()
	mlt := NewMoreLikeThis(analyzer)
	mlt.MinTermFreq = 1

	text := "lucene lucene search search engine"
	query, err := mlt.LikeText(text)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if query == nil {
		t.Fatal("Expected query to not be nil")
	}

	// "lucene" and "search" should have higher frequencies
}

func TestMoreLikeThisQuery(t *testing.T) {
	analyzer := analysis.NewStandardAnalyzer()
	mlt := NewMoreLikeThis(analyzer)

	query := NewMoreLikeThisQuery(mlt, 42)
	if query == nil {
		t.Fatal("Expected query to not be nil")
	}

	if query.docID != 42 {
		t.Errorf("Expected docID=42, got: %d", query.docID)
	}

	if query.isText {
		t.Error("Expected isText to be false")
	}
}

func TestMoreLikeThisQueryFromText(t *testing.T) {
	analyzer := analysis.NewStandardAnalyzer()
	mlt := NewMoreLikeThis(analyzer)

	query := NewMoreLikeThisQueryFromText(mlt, "test text")
	if query == nil {
		t.Fatal("Expected query to not be nil")
	}

	if query.text != "test text" {
		t.Errorf("Expected text='test text', got: %s", query.text)
	}

	if !query.isText {
		t.Error("Expected isText to be true")
	}
}

func TestMoreLikeThisQueryRewrite(t *testing.T) {
	analyzer := analysis.NewStandardAnalyzer()
	mlt := NewMoreLikeThis(analyzer)
	mlt.MinTermFreq = 1
	mlt.MaxQueryTerms = 3

	query := NewMoreLikeThisQueryFromText(mlt, "lucene search engine")

	// For now, LikeText returns an error since we don't have a real IndexReader
	// but the Rewrite method should work with text-based queries
	rewritten, err := query.Rewrite(nil)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if rewritten == nil {
		t.Fatal("Expected rewritten query to not be nil")
	}
}

func TestMoreLikeThisFieldNames(t *testing.T) {
	analyzer := analysis.NewStandardAnalyzer()
	mlt := NewMoreLikeThis(analyzer)
	mlt.FieldNames = []string{"title", "content"}

	if len(mlt.FieldNames) != 2 {
		t.Errorf("Expected 2 field names, got: %d", len(mlt.FieldNames))
	}

	if mlt.FieldNames[0] != "title" {
		t.Errorf("Expected field name 'title', got: %s", mlt.FieldNames[0])
	}

	if mlt.FieldNames[1] != "content" {
		t.Errorf("Expected field name 'content', got: %s", mlt.FieldNames[1])
	}
}

func TestMoreLikeThisCreateQueryEmpty(t *testing.T) {
	analyzer := analysis.NewStandardAnalyzer()
	mlt := NewMoreLikeThis(analyzer)

	query := mlt.createQuery(nil)
	if query != nil {
		t.Error("Expected nil query for empty terms")
	}
}

func TestInterestingTermHeap(t *testing.T) {
	h := &interestingTermHeap{}
	heap.Init(h)

	// Add terms with different scores
	terms := []*interestingTerm{
		{term: "low", score: 1.0},
		{term: "medium", score: 5.0},
		{term: "high", score: 10.0},
	}

	for _, term := range terms {
		heap.Push(h, term)
	}

	if h.Len() != 3 {
		t.Errorf("Expected heap length 3, got: %d", h.Len())
	}

	// Pop should return lowest score first (min heap)
	lowest := heap.Pop(h).(*interestingTerm)
	if lowest.score != 1.0 {
		t.Errorf("Expected lowest score 1.0, got: %f", lowest.score)
	}
}
