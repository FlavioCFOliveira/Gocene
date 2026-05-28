// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Test file: search/search_after_score_doc_test.go
// Source: lucene/core/src/test/org/apache/lucene/search/TestSearchAfter.java
//
//	(and IndexSearcher.searchAfter semantics, IndexSearcher.java lines 582-596)
//
// Purpose: Tests IndexSearcher.SearchAfter(after, query, n) for score-based
// cursor pagination, where results are the top-n documents that sort strictly
// AFTER the given ScoreDoc in the (score desc, docID asc) ordering. These tests
// cover the score-only variant (no Sort); the Sort-based variant is scaffolded
// separately in search_after_test.go.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// assertSorted verifies that the given results are in the (score desc, docID
// asc) ordering that TopScoreDocCollector guarantees, and reports the position
// of the first violation if any.
func assertSorted(t *testing.T, label string, docs []*search.ScoreDoc) {
	t.Helper()
	for i := 1; i < len(docs); i++ {
		prev, cur := docs[i-1], docs[i]
		if cur.Score > prev.Score {
			t.Fatalf("%s: result %d (doc=%d score=%g) outscores predecessor %d (doc=%d score=%g): not score-descending",
				label, i, cur.Doc, cur.Score, i-1, prev.Doc, prev.Score)
		}
		if cur.Score == prev.Score && cur.Doc <= prev.Doc {
			t.Fatalf("%s: result %d (doc=%d) does not follow predecessor %d (doc=%d) in docID-ascending tie-break",
				label, i, cur.Doc, i-1, prev.Doc)
		}
	}
}

// pageThrough walks the entire result set using repeated SearchAfter calls with
// the given page size and returns the concatenation of all pages. It also
// asserts, on the fly, that no document is ever returned twice.
func pageThrough(t *testing.T, searcher *search.IndexSearcher, query search.Query, pageSize int) []*search.ScoreDoc {
	t.Helper()
	var collected []*search.ScoreDoc
	seen := make(map[int]bool)

	var after *search.ScoreDoc
	for {
		page, err := searcher.SearchAfter(after, query, pageSize)
		if err != nil {
			t.Fatalf("SearchAfter(after=%v, pageSize=%d) failed: %v", after, pageSize, err)
		}
		if len(page.ScoreDocs) == 0 {
			break
		}
		assertSorted(t, "page", page.ScoreDocs)
		for _, sd := range page.ScoreDocs {
			if seen[sd.Doc] {
				t.Fatalf("document %d returned on more than one page (pagination overlap)", sd.Doc)
			}
			seen[sd.Doc] = true
			collected = append(collected, sd)
		}
		after = page.ScoreDocs[len(page.ScoreDocs)-1]
	}
	return collected
}

// TestSearchAfterScoreDoc_FirstPageMatchesSearch verifies criterion (a):
// SearchAfter with a nil cursor returns exactly the same top-n as Search.
//
// Source: IndexSearcher.search(query, n) delegates to searchAfter(null, query, n).
func TestSearchAfterScoreDoc_FirstPageMatchesSearch(t *testing.T) {
	_, reader, cleanup := setupTestIndexN(t, 30)
	defer cleanup()

	searcher := search.NewIndexSearcher(reader)
	query := search.NewMatchAllDocsQuery()

	const n = 10
	want, err := searcher.Search(query, n)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	got, err := searcher.SearchAfter(nil, query, n)
	if err != nil {
		t.Fatalf("SearchAfter(nil) failed: %v", err)
	}

	if got.TotalHits.Value != want.TotalHits.Value {
		t.Errorf("TotalHits: SearchAfter=%d, Search=%d", got.TotalHits.Value, want.TotalHits.Value)
	}
	if len(got.ScoreDocs) != len(want.ScoreDocs) {
		t.Fatalf("len(ScoreDocs): SearchAfter=%d, Search=%d", len(got.ScoreDocs), len(want.ScoreDocs))
	}
	for i := range want.ScoreDocs {
		if got.ScoreDocs[i].Doc != want.ScoreDocs[i].Doc || got.ScoreDocs[i].Score != want.ScoreDocs[i].Score {
			t.Errorf("result %d differs: SearchAfter{doc=%d,score=%g} Search{doc=%d,score=%g}",
				i, got.ScoreDocs[i].Doc, got.ScoreDocs[i].Score,
				want.ScoreDocs[i].Doc, want.ScoreDocs[i].Score)
		}
	}
}

// TestSearchAfterScoreDoc_TwoPagesNonOverlapping verifies criterion (b):
// two successive pages are non-overlapping and, concatenated, are globally
// ordered and identical to a single Search over the same span.
//
// Source: TestSearchAfter.assertPage() — paged results must equal the
// corresponding slice of the full result set.
func TestSearchAfterScoreDoc_TwoPagesNonOverlapping(t *testing.T) {
	_, reader, cleanup := setupTestIndexN(t, 30)
	defer cleanup()

	searcher := search.NewIndexSearcher(reader)
	query := search.NewMatchAllDocsQuery()

	const pageSize = 10
	full, err := searcher.Search(query, 2*pageSize)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(full.ScoreDocs) != 2*pageSize {
		t.Fatalf("setup: expected %d docs in full result, got %d", 2*pageSize, len(full.ScoreDocs))
	}

	page1, err := searcher.SearchAfter(nil, query, pageSize)
	if err != nil {
		t.Fatalf("SearchAfter page1 failed: %v", err)
	}
	if len(page1.ScoreDocs) != pageSize {
		t.Fatalf("page1: expected %d docs, got %d", pageSize, len(page1.ScoreDocs))
	}

	page2, err := searcher.SearchAfter(page1.ScoreDocs[pageSize-1], query, pageSize)
	if err != nil {
		t.Fatalf("SearchAfter page2 failed: %v", err)
	}
	if len(page2.ScoreDocs) != pageSize {
		t.Fatalf("page2: expected %d docs, got %d", pageSize, len(page2.ScoreDocs))
	}

	// Non-overlap.
	inPage1 := make(map[int]bool, pageSize)
	for _, sd := range page1.ScoreDocs {
		inPage1[sd.Doc] = true
	}
	for _, sd := range page2.ScoreDocs {
		if inPage1[sd.Doc] {
			t.Errorf("doc %d appears on both page1 and page2", sd.Doc)
		}
	}

	// Concatenation equals the single 2*pageSize Search, in order.
	combined := append(append([]*search.ScoreDoc{}, page1.ScoreDocs...), page2.ScoreDocs...)
	assertSorted(t, "combined", combined)
	for i := range full.ScoreDocs {
		if combined[i].Doc != full.ScoreDocs[i].Doc || combined[i].Score != full.ScoreDocs[i].Score {
			t.Errorf("position %d: paged{doc=%d,score=%g} != full{doc=%d,score=%g}",
				i, combined[i].Doc, combined[i].Score, full.ScoreDocs[i].Doc, full.ScoreDocs[i].Score)
		}
	}
}

// TestSearchAfterScoreDoc_PageConsistencyVariedSizes verifies criterion (b) more
// strongly: paging through the whole result set with several page sizes always
// reproduces, exactly and without overlap, the single full Search ordering.
//
// Source: TestSearchAfter.assertQuery() — pageSize is varied and every page
// must line up with the non-paginated results.
func TestSearchAfterScoreDoc_PageConsistencyVariedSizes(t *testing.T) {
	_, reader, cleanup := setupTestIndexN(t, 30)
	defer cleanup()

	searcher := search.NewIndexSearcher(reader)
	query := search.NewMatchAllDocsQuery()

	full, err := searcher.Search(query, 30)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	for _, pageSize := range []int{1, 3, 7, 30, 50} {
		paged := pageThrough(t, searcher, query, pageSize)
		if len(paged) != len(full.ScoreDocs) {
			t.Fatalf("pageSize=%d: paged returned %d docs, full has %d", pageSize, len(paged), len(full.ScoreDocs))
		}
		for i := range full.ScoreDocs {
			if paged[i].Doc != full.ScoreDocs[i].Doc || paged[i].Score != full.ScoreDocs[i].Score {
				t.Errorf("pageSize=%d position %d: paged{doc=%d,score=%g} != full{doc=%d,score=%g}",
					pageSize, i, paged[i].Doc, paged[i].Score, full.ScoreDocs[i].Doc, full.ScoreDocs[i].Score)
			}
		}
	}
}

// TestSearchAfterScoreDoc_CrossSegmentTieBreak verifies criterion (c):
// with equal scores across segments (MatchAllDocsQuery), pagination ties break
// by global docID, and a page boundary that falls inside a later segment still
// produces strictly increasing, non-overlapping docIDs.
//
// Source: TopScoreDocCollector tie-break (HitQueue favours lower docIDs) plus
// IndexSearcher cross-segment docBase accumulation.
func TestSearchAfterScoreDoc_CrossSegmentTieBreak(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Two segments: 10 docs then 20 docs, all matching MatchAllDocsQuery with
	// equal score, so ordering is purely by global docID 0..29.
	for i := 0; i < 10; i++ {
		if err := writer.AddDocument(document.NewDocument()); err != nil {
			t.Fatalf("AddDocument failed: %v", err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}
	for i := 0; i < 20; i++ {
		if err := writer.AddDocument(document.NewDocument()); err != nil {
			t.Fatalf("AddDocument failed: %v", err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader failed: %v", err)
	}
	defer func() {
		reader.Close()
		dir.Close()
	}()

	if reader.MaxDoc() != 30 {
		t.Fatalf("expected 30 docs across 2 segments, got %d", reader.MaxDoc())
	}

	searcher := search.NewIndexSearcher(reader)
	query := search.NewMatchAllDocsQuery()

	// Page size 12 forces the first boundary at docID 11 (inside segment 1)
	// and the second page to span the segment boundary at docID 10.
	const pageSize = 12
	paged := pageThrough(t, searcher, query, pageSize)

	if len(paged) != 30 {
		t.Fatalf("expected 30 paged docs, got %d", len(paged))
	}
	for i, sd := range paged {
		if sd.Doc != i {
			t.Errorf("position %d: expected global docID %d, got %d", i, i, sd.Doc)
		}
	}
}

// TestSearchAfterScoreDoc_BeyondEndIsEmpty verifies criterion (d):
// requesting a page after the very last result returns no documents (but still
// reports the full total hit count).
//
// Source: TopScoreDocCollector with an after boundary past the last hit
// collects nothing competitive.
func TestSearchAfterScoreDoc_BeyondEndIsEmpty(t *testing.T) {
	_, reader, cleanup := setupTestIndexN(t, 30)
	defer cleanup()

	searcher := search.NewIndexSearcher(reader)
	query := search.NewMatchAllDocsQuery()

	all, err := searcher.Search(query, 30)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	last := all.ScoreDocs[len(all.ScoreDocs)-1]

	beyond, err := searcher.SearchAfter(last, query, 10)
	if err != nil {
		t.Fatalf("SearchAfter(last) failed: %v", err)
	}
	if len(beyond.ScoreDocs) != 0 {
		t.Errorf("expected empty page after last result, got %d docs", len(beyond.ScoreDocs))
	}
	if beyond.TotalHits.Value != 30 {
		t.Errorf("expected total hits 30 even on empty page, got %d", beyond.TotalHits.Value)
	}
}

// TestSearchAfterScoreDoc_ScoreDimensionBoundary proves the pagination boundary
// uses the full (score desc, docID asc) key, not docID alone. It uses a
// TermQuery over documents whose term frequencies differ, so scores differ; if
// the boundary ignored score, pages would overlap or misorder. The expectation
// is derived empirically from the actual full Search ordering rather than from
// any assumed score formula.
//
// Source: TopScoreDocCollector.collect — the after test is
// (score > afterScore || (score == afterScore && doc <= afterDoc)).
func TestSearchAfterScoreDoc_ScoreDimensionBoundary(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Each document repeats the query term a different number of times, so
	// term frequencies (and thus BM25 scores) vary across documents.
	for i := 0; i < 20; i++ {
		doc := document.NewDocument()
		text := ""
		for r := 0; r <= i%5; r++ { // 1..5 occurrences, cycling
			if text != "" {
				text += " "
			}
			text += "alpha"
		}
		text += " filler"
		field, ferr := document.NewTextField("content", text, true)
		if ferr != nil {
			t.Fatalf("NewTextField failed: %v", ferr)
		}
		doc.Add(field)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument failed: %v", err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader failed: %v", err)
	}
	defer func() {
		reader.Close()
		dir.Close()
	}()

	searcher := search.NewIndexSearcher(reader)
	query := search.NewTermQuery(index.NewTerm("content", "alpha"))

	full, err := searcher.Search(query, reader.MaxDoc())
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(full.ScoreDocs) == 0 {
		t.Fatal("setup: TermQuery matched no documents")
	}
	assertSorted(t, "full", full.ScoreDocs)

	// Confirm scores actually vary; otherwise this test would degenerate into
	// the equal-score tie-break case and not exercise the score dimension.
	scoreVaries := false
	for i := 1; i < len(full.ScoreDocs); i++ {
		if full.ScoreDocs[i].Score != full.ScoreDocs[0].Score {
			scoreVaries = true
			break
		}
	}
	if !scoreVaries {
		t.Fatalf("setup: expected varying scores from differing term frequencies, all scores were %g",
			full.ScoreDocs[0].Score)
	}

	// Paging through with a small page size must reproduce the full ordering
	// exactly and without overlap. With varying scores this can only hold if
	// the after boundary honours the score dimension.
	for _, pageSize := range []int{1, 2, 5} {
		paged := pageThrough(t, searcher, query, pageSize)
		if len(paged) != len(full.ScoreDocs) {
			t.Fatalf("pageSize=%d: paged %d docs, full %d", pageSize, len(paged), len(full.ScoreDocs))
		}
		for i := range full.ScoreDocs {
			if paged[i].Doc != full.ScoreDocs[i].Doc || paged[i].Score != full.ScoreDocs[i].Score {
				t.Errorf("pageSize=%d position %d: paged{doc=%d,score=%g} != full{doc=%d,score=%g}",
					pageSize, i, paged[i].Doc, paged[i].Score, full.ScoreDocs[i].Doc, full.ScoreDocs[i].Score)
			}
		}
	}
}

// TestSearchAfterScoreDoc_InvalidArguments verifies the error contract matching
// Lucene 10.4.0: n<=0 is rejected (TopScoreDocCollectorManager requires
// numHits>0) and after.Doc beyond the reader's maxDoc is rejected.
//
// Source: IndexSearcher.searchAfter (IndexSearcher.java lines 582-596).
func TestSearchAfterScoreDoc_InvalidArguments(t *testing.T) {
	_, reader, cleanup := setupTestIndexN(t, 30)
	defer cleanup()

	searcher := search.NewIndexSearcher(reader)
	query := search.NewMatchAllDocsQuery()

	if _, err := searcher.SearchAfter(nil, query, 0); err == nil {
		t.Error("expected error for n=0, got nil")
	}
	if _, err := searcher.SearchAfter(nil, query, -5); err == nil {
		t.Error("expected error for n<0, got nil")
	}

	// after.Doc == maxDoc (30) is out of range; limit is max(1, maxDoc)=30.
	tooFar := search.NewScoreDoc(reader.MaxDoc(), 1.0, 0)
	if _, err := searcher.SearchAfter(tooFar, query, 10); err == nil {
		t.Error("expected error for after.Doc >= limit, got nil")
	}

	// after.Doc == maxDoc-1 is the last valid doc and must be accepted.
	lastValid := search.NewScoreDoc(reader.MaxDoc()-1, 1.0, 0)
	if _, err := searcher.SearchAfter(lastValid, query, 10); err != nil {
		t.Errorf("after.Doc = maxDoc-1 should be valid, got error: %v", err)
	}
}
