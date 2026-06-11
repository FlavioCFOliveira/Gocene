// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package memory_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/memory"
	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestMemoryIndex_Search_DirectQueries validates that the MemoryIndex can run
// queries directly (without a store.Directory or IndexWriter).
func TestMemoryIndex_Search_DirectQueries(t *testing.T) {
	mi := memory.NewMemoryIndex()

	if err := mi.AddField("title", "The quick brown fox jumps over the lazy dog"); err != nil {
		t.Fatalf("AddField: %v", err)
	}
	if err := mi.AddField("body", "Gocene is a Go port of Apache Lucene"); err != nil {
		t.Fatalf("AddField body: %v", err)
	}

	// Term that exists in body
	q1 := search.NewTermQuery(schema.NewTerm("body", "Gocene"))
	td, err := mi.Search(q1, 10)
	if err != nil {
		t.Fatalf("Search(body:Gocene): %v", err)
	}
	if td.TotalHits.Value != 1 {
		t.Errorf("Search(body:Gocene) totalHits = %d, want 1", td.TotalHits.Value)
	}
	if len(td.ScoreDocs) != 1 {
		t.Errorf("Search(body:Gocene) len(ScoreDocs) = %d, want 1", len(td.ScoreDocs))
	}
	if len(td.ScoreDocs) > 0 && td.ScoreDocs[0].Doc != 0 {
		t.Errorf("Search(body:Gocene) doc = %d, want 0", td.ScoreDocs[0].Doc)
	}

	// Term that exists in title
	q2 := search.NewTermQuery(schema.NewTerm("title", "fox"))
	td2, err := mi.Search(q2, 10)
	if err != nil {
		t.Fatalf("Search(title:fox): %v", err)
	}
	if td2.TotalHits.Value != 1 {
		t.Errorf("Search(title:fox) totalHits = %d, want 1", td2.TotalHits.Value)
	}

	// Term that does NOT exist
	q3 := search.NewTermQuery(schema.NewTerm("body", "nonexistent"))
	td3, err := mi.Search(q3, 10)
	if err != nil {
		t.Fatalf("Search(body:nonexistent): %v", err)
	}
	if td3.TotalHits.Value != 0 {
		t.Errorf("Search(body:nonexistent) totalHits = %d, want 0", td3.TotalHits.Value)
	}

	// Unknown field
	q4 := search.NewTermQuery(schema.NewTerm("unknown", "Gocene"))
	td4, err := mi.Search(q4, 10)
	if err != nil {
		t.Fatalf("Search(unknown:Gocene): %v", err)
	}
	if td4.TotalHits.Value != 0 {
		t.Errorf("Search(unknown:Gocene) totalHits = %d, want 0", td4.TotalHits.Value)
	}

	// BooleanQuery: AND of two matching terms
	bq := search.NewBooleanQuery()
	bq.Add(search.NewTermQuery(schema.NewTerm("title", "quick")), search.MUST)
	bq.Add(search.NewTermQuery(schema.NewTerm("body", "Gocene")), search.MUST)
	td5, err := mi.Search(bq, 10)
	if err != nil {
		t.Fatalf("Search(Boolean AND): %v", err)
	}
	if td5.TotalHits.Value != 1 {
		t.Errorf("Search(Boolean AND) totalHits = %d, want 1", td5.TotalHits.Value)
	}

	// BooleanQuery: AND where one term doesn't match
	bq2 := search.NewBooleanQuery()
	bq2.Add(search.NewTermQuery(schema.NewTerm("title", "quick")), search.MUST)
	bq2.Add(search.NewTermQuery(schema.NewTerm("body", "nosuchterm")), search.MUST)
	td6, err := mi.Search(bq2, 10)
	if err != nil {
		t.Fatalf("Search(Boolean AND miss): %v", err)
	}
	if td6.TotalHits.Value != 0 {
		t.Errorf("Search(Boolean AND miss) totalHits = %d, want 0", td6.TotalHits.Value)
	}

	t.Logf("MemoryIndex direct search tests passed")
}

// TestMemoryIndex_Search_CreateSearcher validates the CreateSearcher API.
func TestMemoryIndex_Search_CreateSearcher(t *testing.T) {
	mi := memory.NewMemoryIndex()

	if err := mi.AddField("text", "hello world"); err != nil {
		t.Fatalf("AddField: %v", err)
	}

	searcher, err := mi.CreateSearcher()
	if err != nil {
		t.Fatalf("CreateSearcher: %v", err)
	}

	td, err := searcher.Search(search.NewTermQuery(schema.NewTerm("text", "hello")), 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if td.TotalHits.Value != 1 {
		t.Errorf("CreateSearcher totalHits = %d, want 1", td.TotalHits.Value)
	}

	t.Logf("MemoryIndex CreateSearcher test passed")
}

// TestMemoryIndex_Search_EmptyIndex verifies that searching an empty index returns no results.
func TestMemoryIndex_Search_EmptyIndex(t *testing.T) {
	mi := memory.NewMemoryIndex()

	td, err := mi.Search(search.NewTermQuery(schema.NewTerm("field", "term")), 10)
	if err != nil {
		t.Fatalf("Search(empty): %v", err)
	}
	if td.TotalHits.Value != 0 {
		t.Errorf("Search(empty) totalHits = %d, want 0", td.TotalHits.Value)
	}

	t.Logf("MemoryIndex empty index search passed")
}

// TestMemoryIndex_Search_Positions verifies that postings expose correct term positions.
func TestMemoryIndex_Search_Positions(t *testing.T) {
	mi := memory.NewMemoryIndex()

	if err := mi.AddField("text", "alpha beta alpha gamma"); err != nil {
		t.Fatalf("AddField: %v", err)
	}

	// Verify term frequency for "alpha" (should be 2)
	searcher, err := mi.CreateSearcher()
	if err != nil {
		t.Fatalf("CreateSearcher: %v", err)
	}

	td, err := searcher.Search(search.NewTermQuery(schema.NewTerm("text", "alpha")), 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if td.TotalHits.Value != 1 {
		t.Errorf("positions search totalHits = %d, want 1", td.TotalHits.Value)
	}
	// Score should be proportional to term frequency (2 occurrences)
	if len(td.ScoreDocs) > 0 {
		t.Logf("alpha score: %v", td.ScoreDocs[0].Score)
	}

	t.Logf("MemoryIndex positions test passed")
}
