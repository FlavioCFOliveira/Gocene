// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package search_test contains tests for sloppy phrase query functionality.
//
// Source: TestSloppyPhraseQuery.java, TestSloppyPhraseQuery2.java
// Purpose: Tests sloppy phrase scoring, edit distance, and complex sloppy scenarios
//
// These tests verify that:
// - Sloppy phrase queries match documents with terms within the specified slop distance
// - Higher slop values allow more matches (slop N is a subset of slop N+1)
// - Scoring is consistent and sane (no infinite scores)
// - Phrase queries with holes (position increments) work correctly
package search_test

import (
	"math"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Test result structure for collecting search results
type sloppyPhraseResult struct {
	max       float64
	totalHits int
}

// makeDocument creates a document with a text field using the given content.
// The field omits norms for consistent scoring.
func makeSloppyDocument(docText string) *document.Document {
	doc := document.NewDocument()
	ft := document.NewFieldType()
	ft.SetIndexed(true).
		SetStored(false).
		SetTokenized(true).
		SetOmitNorms(true).
		SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions)
	ft.Freeze()

	field, _ := document.NewField("f", docText, ft)
	doc.Add(field)
	return doc
}

// makePhraseQuery creates a PhraseQuery from space-separated terms.
func makeSloppyPhraseQuery(terms string) *search.PhraseQuery {
	t := strings.Fields(terms)
	return search.NewPhraseQueryWithStrings("f", t...)
}

// checkPhraseQuery executes a phrase query with the given slop and returns the max score.
func checkPhraseQuery(t *testing.T, doc *document.Document, query *search.PhraseQuery, slop int, expectedNumResults int) float64 {
	// Clone the query and set slop
	clonedQuery := query.Clone().(*search.PhraseQuery)
	clonedQuery.SetSlop(slop)

	// Create in-memory directory
	dir := store.NewByteBuffersDirectory()

	// Create analyzer and index writer
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add document
	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}

	// Close writer to flush documents
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	// Create reader and searcher
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)

	// Execute search
	topDocs, err := searcher.Search(clonedQuery, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Verify hit count
	if int(topDocs.TotalHits.Value) != expectedNumResults {
		t.Errorf("slop: %d query: %s doc: %s - Expected %d hits, got %d",
			slop, clonedQuery.String(), doc.String(), expectedNumResults, topDocs.TotalHits.Value)
	}

	// Calculate max score
	var maxScore float64
	for _, scoreDoc := range topDocs.ScoreDocs {
		if float64(scoreDoc.Score) > maxScore {
			maxScore = float64(scoreDoc.Score)
		}
	}

	return maxScore
}

// TestSloppyPhraseQuery_Doc4_Query4 tests DOC_4 and QUERY_4.
// QUERY_4 has a fuzzy (len=1) match to DOC_4, so all slop values > 0 should succeed.
// But only the 3rd sequence of A's in DOC_4 will do.
func TestSloppyPhraseQuery_Doc4_Query4_AllSlopsShouldMatch(t *testing.T) {
	doc4 := makeSloppyDocument("A A X A X B A X B B A A X B A A")
	query4 := makeSloppyPhraseQuery("X A A")

	for slop := 0; slop < 30; slop++ {
		expectedResults := 0
		if slop >= 1 {
			expectedResults = 1
		}
		checkPhraseQuery(t, doc4, query4, slop, expectedResults)
	}
}

// TestSloppyPhraseQuery_Doc1_Query1 tests DOC_1 and QUERY_1.
// QUERY_1 has an exact match to DOC_1, so all slop values should succeed.
// Before LUCENE-1310, a slop value of 1 did not succeed.
func TestSloppyPhraseQuery_Doc1_Query1_AllSlopsShouldMatch(t *testing.T) {
	s1 := "A A A"
	doc1 := makeSloppyDocument("X " + s1 + " Y")
	doc1B := makeSloppyDocument("X " + s1 + " Y N N N N " + s1 + " Z")
	query1 := makeSloppyPhraseQuery(s1)

	for slop := 0; slop < 30; slop++ {
		freq1 := checkPhraseQuery(t, doc1, query1, slop, 1)
		freq2 := checkPhraseQuery(t, doc1B, query1, slop, 1)
		if freq2 <= freq1 {
			t.Errorf("slop=%d freq2=%f should be greater than freq1 %f", slop, freq2, freq1)
		}
	}
}

// TestSloppyPhraseQuery_Doc2_Query1 tests DOC_2 and QUERY_1.
// 6 should be the minimum slop to make QUERY_1 match DOC_2.
// Before LUCENE-1310, 7 was the minimum.
func TestSloppyPhraseQuery_Doc2_Query1_Slop6OrMoreShouldMatch(t *testing.T) {
	s1 := "A A A"
	s2 := "A 1 2 3 A 4 5 6 A"
	doc2 := makeSloppyDocument("X " + s2 + " Y")
	doc2B := makeSloppyDocument("X " + s2 + " Y N N N N " + s2 + " Z")
	query1 := makeSloppyPhraseQuery(s1)

	for slop := 0; slop < 30; slop++ {
		expectedResults := 0
		if slop >= 6 {
			expectedResults = 1
		}
		freq1 := checkPhraseQuery(t, doc2, query1, slop, expectedResults)
		if expectedResults > 0 {
			freq2 := checkPhraseQuery(t, doc2B, query1, slop, 1)
			if freq2 <= freq1 {
				t.Errorf("slop=%d freq2=%f should be greater than freq1 %f", slop, freq2, freq1)
			}
		}
	}
}

// TestSloppyPhraseQuery_Doc2_Query2 tests DOC_2 and QUERY_2.
// QUERY_2 has an exact match to DOC_2, so all slop values should succeed.
// Before LUCENE-1310, 0 succeeds, 1 through 7 fail, and 8 or greater succeeds.
func TestSloppyPhraseQuery_Doc2_Query2_AllSlopsShouldMatch(t *testing.T) {
	s2 := "A 1 2 3 A 4 5 6 A"
	doc2 := makeSloppyDocument("X " + s2 + " Y")
	doc2B := makeSloppyDocument("X " + s2 + " Y N N N N " + s2 + " Z")
	query2 := makeSloppyPhraseQuery(s2)

	for slop := 0; slop < 30; slop++ {
		freq1 := checkPhraseQuery(t, doc2, query2, slop, 1)
		freq2 := checkPhraseQuery(t, doc2B, query2, slop, 1)
		if freq2 <= freq1 {
			t.Errorf("slop=%d freq2=%f should be greater than freq1 %f", slop, freq2, freq1)
		}
	}
}

// TestSloppyPhraseQuery_Doc3_Query1 tests DOC_3 and QUERY_1.
// QUERY_1 has an exact match to DOC_3, so all slop values should succeed.
func TestSloppyPhraseQuery_Doc3_Query1_AllSlopsShouldMatch(t *testing.T) {
	s1 := "A A A"
	doc3 := makeSloppyDocument("X " + s1 + " A Y")
	doc3B := makeSloppyDocument("X " + s1 + " A Y N N N N " + s1 + " A Y")
	query1 := makeSloppyPhraseQuery(s1)

	for slop := 0; slop < 30; slop++ {
		freq1 := checkPhraseQuery(t, doc3, query1, slop, 1)
		freq2 := checkPhraseQuery(t, doc3B, query1, slop, 1)
		if freq2 <= freq1 {
			t.Errorf("slop=%d freq2=%f should be greater than freq1 %f", slop, freq2, freq1)
		}
	}
}

// TestSloppyPhraseQuery_Doc5_Query5 tests LUCENE-3412.
// Any slop should be consistent - DOC_5_4 should always match, DOC_5_3 should never match.
func TestSloppyPhraseQuery_Doc5_Query5_AnySlopShouldBeConsistent(t *testing.T) {
	doc5_3 := makeSloppyDocument("H H H X X X H H H X X X H H H")
	doc5_4 := makeSloppyDocument("H H H H")
	query5_4 := makeSloppyPhraseQuery("H H H H")

	nRepeats := 5
	for slop := 0; slop < 3; slop++ {
		for trial := 0; trial < nRepeats; trial++ {
			// Should steadily always find this one
			checkPhraseQuery(t, doc5_4, query5_4, slop, 1)
		}
		for trial := 0; trial < nRepeats; trial++ {
			// Should steadily never find this one
			checkPhraseQuery(t, doc5_3, query5_4, slop, 0)
		}
	}
}

// TestSloppyPhraseQuery_SlopWithHoles tests LUCENE-3215.
// Tests phrase queries with position increments (holes).
func TestSloppyPhraseQuery_SlopWithHoles(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Create field type with omit norms
	ft := document.NewFieldType()
	ft.SetIndexed(true).
		SetStored(false).
		SetTokenized(true).
		SetOmitNorms(true).
		SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions)
	ft.Freeze()

	// Add documents
	docs := []string{
		"drug drug",
		"drug druggy drug",
		"drug druggy druggy drug",
		"drug druggy drug druggy drug",
	}

	for _, docText := range docs {
		doc := document.NewDocument()
		field, _ := document.NewField("lyrics", docText, ft)
		doc.Add(field)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)

	// Build phrase query with holes: "drug" at position 1, "drug" at position 4
	// This is like "drug the drug" (2 missing words between)
	builder := search.NewPhraseQueryBuilder()
	builder.AddTermAtPosition(index.NewTerm("lyrics", "drug"), 1)
	builder.AddTermAtPosition(index.NewTerm("lyrics", "drug"), 4)

	// Test with slop 0 - should match only "drug drug" (exactly 3 positions apart)
	pq := builder.Build()
	topDocs, _ := searcher.Search(pq, 4)
	if topDocs.TotalHits.Value != 1 {
		t.Errorf("Expected 1 hit with slop 0, got %d", topDocs.TotalHits.Value)
	}

	// Test with slop 1 - should match 3 documents
	builder.SetSlop(1)
	pq = builder.Build()
	topDocs, _ = searcher.Search(pq, 4)
	if topDocs.TotalHits.Value != 3 {
		t.Errorf("Expected 3 hits with slop 1, got %d", topDocs.TotalHits.Value)
	}

	// Test with slop 2 - should match all 4 documents
	builder.SetSlop(2)
	pq = builder.Build()
	topDocs, _ = searcher.Search(pq, 4)
	if topDocs.TotalHits.Value != 4 {
		t.Errorf("Expected 4 hits with slop 2, got %d", topDocs.TotalHits.Value)
	}
}

// TestSloppyPhraseQuery_InfiniteFreq1 tests LUCENE-3215.
// Verifies that no scores are infinite with specific document pattern.
func TestSloppyPhraseQuery_InfiniteFreq1(t *testing.T) {
	document := "drug druggy drug drug drug"

	dir := store.NewByteBuffersDirectory()
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	doc := document.NewDocument()
	ft := document.NewFieldType()
	ft.SetIndexed(true).
		SetTokenized(true).
		SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions)
	ft.Freeze()
	field, _ := document.NewField("lyrics", document, ft)
	doc.Add(field)

	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)

	// Build phrase query: "drug" at position 1, "drug" at position 3, slop 1
	builder := search.NewPhraseQueryBuilder()
	builder.AddTermAtPosition(index.NewTerm("lyrics", "drug"), 1)
	builder.AddTermAtPosition(index.NewTerm("lyrics", "drug"), 3)
	builder.SetSlop(1)
	pq := builder.Build()

	// Execute search and verify no infinite scores
	topDocs, err := searcher.Search(pq, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	for _, scoreDoc := range topDocs.ScoreDocs {
		if math.IsInf(float64(scoreDoc.Score), 0) {
			t.Errorf("Score is infinite for doc %d", scoreDoc.Doc)
		}
	}
}

// TestSloppyPhraseQuery_InfiniteFreq2 tests LUCENE-3215 with a longer document.
// Verifies that no scores are infinite with complex document pattern.
func TestSloppyPhraseQuery_InfiniteFreq2(t *testing.T) {
	document := "So much fun to be had in my head " +
		"No more sunshine " +
		"So much fun just lying in my bed " +
		"No more sunshine " +
		"I can't face the sunlight and the dirt outside " +
		"Wanna stay in 666 where this darkness don't lie " +
		"Drug drug druggy " +
		"Got a feeling sweet like honey " +
		"Drug drug druggy " +
		"Need sensation like my baby " +
		"Show me your scars you're so aware " +
		"I'm not barbaric I just care " +
		"Drug drug drug " +
		"I need a reflection to prove I exist " +
		"No more sunshine " +
		"I am a victim of designer blitz " +
		"No more sunshine " +
		"Dance like a robot when you're chained at the knee " +
		"The C.I.A say you're all they'll ever need " +
		"Drug drug druggy " +
		"Got a feeling sweet like honey " +
		"Drug drug druggy " +
		"Need sensation like my baby " +
		"Snort your lines you're so aware " +
		"I'm not barbaric I just care " +
		"Drug drug druggy " +
		"Got a feeling sweet like honey " +
		"Drug drug druggy " +
		"Need sensation like my baby"

	dir := store.NewByteBuffersDirectory()
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	doc := document.NewDocument()
	ft := document.NewFieldType()
	ft.SetIndexed(true).
		SetTokenized(true).
		SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions)
	ft.Freeze()
	field, _ := document.NewField("lyrics", document, ft)
	doc.Add(field)

	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)

	// Build phrase query: "drug" at position 1, "drug" at position 4, slop 5
	builder := search.NewPhraseQueryBuilder()
	builder.AddTermAtPosition(index.NewTerm("lyrics", "drug"), 1)
	builder.AddTermAtPosition(index.NewTerm("lyrics", "drug"), 4)
	builder.SetSlop(5)
	pq := builder.Build()

	// Execute search and verify no infinite scores
	topDocs, err := searcher.Search(pq, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	for _, scoreDoc := range topDocs.ScoreDocs {
		if math.IsInf(float64(scoreDoc.Score), 0) {
			t.Errorf("Score is infinite for doc %d", scoreDoc.Doc)
		}
	}
}

// assertSubsetOf verifies that all documents matched by q1 are also matched by q2.
// This is used to test that slop N is a subset of slop N+1.
func assertSubsetOf(t *testing.T, searcher *search.IndexSearcher, q1, q2 search.Query) {
	topDocs1, err := searcher.Search(q1, 1000)
	if err != nil {
		t.Fatalf("Search failed for q1: %v", err)
	}

	topDocs2, err := searcher.Search(q2, 1000)
	if err != nil {
		t.Fatalf("Search failed for q2: %v", err)
	}

	// Build set of doc IDs from q2
	docSet := make(map[int]bool)
	for _, scoreDoc := range topDocs2.ScoreDocs {
		docSet[scoreDoc.Doc] = true
	}

	// Verify all docs from q1 are in q2
	for _, scoreDoc := range topDocs1.ScoreDocs {
		if !docSet[scoreDoc.Doc] {
			t.Errorf("Document %d matched by q1 but not by q2", scoreDoc.Doc)
		}
	}
}

// createTestIndex creates a test index with random documents for subset testing.
func createTestIndex(t *testing.T) (store.Directory, *search.IndexSearcher, func()) {
	dir := store.NewByteBuffersDirectory()
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add some test documents with terms a-z
	docTexts := []string{
		"a b c d e",
		"a b c",
		"b c d",
		"a c e",
		"a b d e",
		"c d e f",
		"a b c d",
		"b c d e",
		"a c d e",
		"a b c e",
	}

	for _, text := range docTexts {
		doc := document.NewDocument()
		ft := document.NewFieldType()
		ft.SetIndexed(true).
			SetTokenized(true).
			SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions)
		ft.Freeze()
		field, _ := document.NewField("field", text, ft)
		doc.Add(field)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}

	searcher := search.NewIndexSearcher(reader)

	cleanup := func() {
		reader.Close()
	}

	return dir, searcher, cleanup
}

// TestSloppyPhraseQuery2_IncreasingSloppiness tests that "A B"~N is a subset of "A B"~N+1.
func TestSloppyPhraseQuery2_IncreasingSloppiness(t *testing.T) {
	_, searcher, cleanup := createTestIndex(t)
	defer cleanup()

	for i := 0; i < 10; i++ {
		q1 := search.NewPhraseQueryWithSlop(i, "field",
			index.NewTerm("field", "a"),
			index.NewTerm("field", "b"))
		q2 := search.NewPhraseQueryWithSlop(i+1, "field",
			index.NewTerm("field", "a"),
			index.NewTerm("field", "b"))
		assertSubsetOf(t, searcher, q1, q2)
	}
}

// TestSloppyPhraseQuery2_IncreasingSloppinessWithHoles tests that "A B"~N with position
// increments is a subset of "A B"~N+1.
func TestSloppyPhraseQuery2_IncreasingSloppinessWithHoles(t *testing.T) {
	_, searcher, cleanup := createTestIndex(t)
	defer cleanup()

	for i := 0; i < 10; i++ {
		builder1 := search.NewPhraseQueryBuilder()
		builder1.AddTermAtPosition(index.NewTerm("field", "a"), 0)
		builder1.AddTermAtPosition(index.NewTerm("field", "b"), 2)
		builder1.SetSlop(i)
		q1 := builder1.Build()

		builder2 := search.NewPhraseQueryBuilder()
		builder2.AddTermAtPosition(index.NewTerm("field", "a"), 0)
		builder2.AddTermAtPosition(index.NewTerm("field", "b"), 2)
		builder2.SetSlop(i + 1)
		q2 := builder2.Build()

		assertSubsetOf(t, searcher, q1, q2)
	}
}

// TestSloppyPhraseQuery2_IncreasingSloppiness3 tests that "A B C"~N is a subset of "A B C"~N+1.
func TestSloppyPhraseQuery2_IncreasingSloppiness3(t *testing.T) {
	_, searcher, cleanup := createTestIndex(t)
	defer cleanup()

	for i := 0; i < 10; i++ {
		q1 := search.NewPhraseQueryWithSlop(i, "field",
			index.NewTerm("field", "a"),
			index.NewTerm("field", "b"),
			index.NewTerm("field", "c"))
		q2 := search.NewPhraseQueryWithSlop(i+1, "field",
			index.NewTerm("field", "a"),
			index.NewTerm("field", "b"),
			index.NewTerm("field", "c"))
		assertSubsetOf(t, searcher, q1, q2)
	}
}

// TestSloppyPhraseQuery2_IncreasingSloppiness3WithHoles tests that "A B C"~N with position
// increments is a subset of "A B C"~N+1.
func TestSloppyPhraseQuery2_IncreasingSloppiness3WithHoles(t *testing.T) {
	_, searcher, cleanup := createTestIndex(t)
	defer cleanup()

	for i := 0; i < 10; i++ {
		builder1 := search.NewPhraseQueryBuilder()
		builder1.AddTermAtPosition(index.NewTerm("field", "a"), 0)
		builder1.AddTermAtPosition(index.NewTerm("field", "b"), 2)
		builder1.AddTermAtPosition(index.NewTerm("field", "c"), 4)
		builder1.SetSlop(i)
		q1 := builder1.Build()

		builder2 := search.NewPhraseQueryBuilder()
		builder2.AddTermAtPosition(index.NewTerm("field", "a"), 0)
		builder2.AddTermAtPosition(index.NewTerm("field", "b"), 2)
		builder2.AddTermAtPosition(index.NewTerm("field", "c"), 4)
		builder2.SetSlop(i + 1)
		q2 := builder2.Build()

		assertSubsetOf(t, searcher, q1, q2)
	}
}

// TestSloppyPhraseQuery2_RepetitiveIncreasingSloppiness tests that "A A"~N is a subset of "A A"~N+1.
func TestSloppyPhraseQuery2_RepetitiveIncreasingSloppiness(t *testing.T) {
	_, searcher, cleanup := createTestIndex(t)
	defer cleanup()

	term := index.NewTerm("field", "a")
	for i := 0; i < 10; i++ {
		q1 := search.NewPhraseQueryWithSlop(i, "field", term, term)
		q2 := search.NewPhraseQueryWithSlop(i+1, "field", term, term)
		assertSubsetOf(t, searcher, q1, q2)
	}
}

// TestSloppyPhraseQuery2_RepetitiveIncreasingSloppinessWithHoles tests that "A A"~N with
// position increments is a subset of "A A"~N+1.
func TestSloppyPhraseQuery2_RepetitiveIncreasingSloppinessWithHoles(t *testing.T) {
	_, searcher, cleanup := createTestIndex(t)
	defer cleanup()

	term := index.NewTerm("field", "a")
	for i := 0; i < 10; i++ {
		builder1 := search.NewPhraseQueryBuilder()
		builder1.AddTermAtPosition(term, 0)
		builder1.AddTermAtPosition(term, 2)
		builder1.SetSlop(i)
		q1 := builder1.Build()

		builder2 := search.NewPhraseQueryBuilder()
		builder2.AddTermAtPosition(term, 0)
		builder2.AddTermAtPosition(term, 2)
		builder2.SetSlop(i + 1)
		q2 := builder2.Build()

		assertSubsetOf(t, searcher, q1, q2)
	}
}

// TestSloppyPhraseQuery2_RepetitiveIncreasingSloppiness3 tests that "A A A"~N is a subset of "A A A"~N+1.
func TestSloppyPhraseQuery2_RepetitiveIncreasingSloppiness3(t *testing.T) {
	_, searcher, cleanup := createTestIndex(t)
	defer cleanup()

	term := index.NewTerm("field", "a")
	for i := 0; i < 10; i++ {
		q1 := search.NewPhraseQueryWithSlop(i, "field", term, term, term)
		q2 := search.NewPhraseQueryWithSlop(i+1, "field", term, term, term)
		assertSubsetOf(t, searcher, q1, q2)
	}
}

// TestSloppyPhraseQuery2_RepetitiveIncreasingSloppiness3WithHoles tests that "A A A"~N with
// position increments is a subset of "A A A"~N+1.
func TestSloppyPhraseQuery2_RepetitiveIncreasingSloppiness3WithHoles(t *testing.T) {
	_, searcher, cleanup := createTestIndex(t)
	defer cleanup()

	term := index.NewTerm("field", "a")
	for i := 0; i < 10; i++ {
		builder1 := search.NewPhraseQueryBuilder()
		builder1.AddTermAtPosition(term, 0)
		builder1.AddTermAtPosition(term, 2)
		builder1.AddTermAtPosition(term, 4)
		builder1.SetSlop(i)
		q1 := builder1.Build()

		builder2 := search.NewPhraseQueryBuilder()
		builder2.AddTermAtPosition(term, 0)
		builder2.AddTermAtPosition(term, 2)
		builder2.AddTermAtPosition(term, 4)
		builder2.SetSlop(i + 1)
		q2 := builder2.Build()

		assertSubsetOf(t, searcher, q1, q2)
	}
}

// PhraseQueryBuilder provides a builder pattern for constructing PhraseQueries.
// This is a helper type for test construction.
type PhraseQueryBuilder struct {
	field     string
	terms     []*index.Term
	positions []int
	slop      int
}

// NewPhraseQueryBuilder creates a new PhraseQueryBuilder.
func NewPhraseQueryBuilder() *PhraseQueryBuilder {
	return &PhraseQueryBuilder{
		terms:     make([]*index.Term, 0),
		positions: make([]int, 0),
		slop:      0,
	}
}

// AddTermAtPosition adds a term at the specified position.
func (b *PhraseQueryBuilder) AddTermAtPosition(term *index.Term, position int) *PhraseQueryBuilder {
	b.terms = append(b.terms, term)
	b.positions = append(b.positions, position)
	if b.field == "" {
		b.field = term.Field
	}
	return b
}

// SetSlop sets the slop value.
func (b *PhraseQueryBuilder) SetSlop(slop int) *PhraseQueryBuilder {
	b.slop = slop
	return b
}

// Build creates the PhraseQuery.
func (b *PhraseQueryBuilder) Build() *search.PhraseQuery {
	// For now, create a simple phrase query and set slop
	// In a full implementation, this would handle positions
	query := search.NewPhraseQuery(b.field, b.terms...)
	query.SetSlop(b.slop)
	return query
}
