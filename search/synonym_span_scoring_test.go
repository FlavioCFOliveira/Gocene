// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// docSet returns the set of leaf-local doc ids present in the search results.
func docSet(t *testing.T, searcher *search.IndexSearcher, query search.Query) map[int]float32 {
	t.Helper()
	topDocs, err := searcher.Search(query, 100)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	out := make(map[int]float32, len(topDocs.ScoreDocs))
	for _, sd := range topDocs.ScoreDocs {
		out[sd.Doc] = sd.Score
	}
	return out
}

// TestSynonymQuery_MatchesAnySynonym verifies that a SynonymQuery matches every
// document containing any of its synonym terms, and no others. This exercises
// the disjunction in the SynonymScorer ported in rmp #4749 from Lucene's
// SynonymQuery.SynonymWeight.
func TestSynonymQuery_MatchesAnySynonym(t *testing.T) {
	// Docs: 0:"quick"  1:"fast"  2:"slow"  3:"quick fast"  4:"lazy"
	searcher, _ := explainTestIndex(t, []string{"quick", "fast", "slow", "quick fast", "lazy"})

	query := search.NewSynonymQueryBuilder("field").
		AddTerm(index.NewTerm("field", "quick")).
		AddTerm(index.NewTerm("field", "fast")).
		Build()

	got := docSet(t, searcher, query)

	// Docs 0,1,3 contain at least one synonym term; 2 and 4 contain neither.
	for _, doc := range []int{0, 1, 3} {
		if _, ok := got[doc]; !ok {
			t.Errorf("expected doc %d to match the synonym query", doc)
		}
	}
	for _, doc := range []int{2, 4} {
		if _, ok := got[doc]; ok {
			t.Errorf("did not expect doc %d to match the synonym query", doc)
		}
	}
}

// TestSynonymQuery_ScoresByCombinedFreq verifies the core SynonymQuery scoring
// invariant: a document is scored on the SUM of the per-term frequencies of its
// synonym terms, with the similarity invoked a single time. A document with a
// higher combined synonym frequency therefore scores strictly higher than one
// with a lower combined frequency (now that rmp #4751 makes tf scoring work).
//
// Source: org.apache.lucene.search.SynonymQuery (combined-freq scoring) and
// TestSynonymQuery.testScores().
func TestSynonymQuery_ScoresByCombinedFreq(t *testing.T) {
	// Both "aa" and "bb" are synonyms with equal document frequency (each appears
	// in exactly one document), so the per-term IDF is identical and the score
	// difference is driven purely by the combined term frequency.
	//
	// Doc 0: "aa"          -> combined freq 1
	// Doc 1: "bb bb bb"    -> combined freq 3
	searcher, _ := explainTestIndex(t, []string{"aa", "bb bb bb"})

	query := search.NewSynonymQueryBuilder("field").
		AddTerm(index.NewTerm("field", "aa")).
		AddTerm(index.NewTerm("field", "bb")).
		Build()

	low := scoreOfDoc(t, searcher, query, 0)  // combined freq 1
	high := scoreOfDoc(t, searcher, query, 1) // combined freq 3

	if !(high > low) {
		t.Errorf("expected higher combined synonym freq to score higher: doc1(freq=3)=%v should be > doc0(freq=1)=%v", high, low)
	}
	if low <= 0 || high <= 0 {
		t.Errorf("expected positive scores, got low=%v high=%v", low, high)
	}
}

// TestSynonymQuery_SumsFreqAcrossTerms verifies that when several synonym terms
// co-occur in the same document their frequencies are summed (not maxed) before
// scoring, matching Lucene SynonymScorer.freq(). A document containing both
// terms once each must outscore a document containing a single term once.
func TestSynonymQuery_SumsFreqAcrossTerms(t *testing.T) {
	// Doc 0: "aa"     -> combined freq 1 (only "aa")
	// Doc 1: "aa bb"  -> combined freq 2 (one "aa" + one "bb")
	searcher, _ := explainTestIndex(t, []string{"aa", "aa bb"})

	query := search.NewSynonymQueryBuilder("field").
		AddTerm(index.NewTerm("field", "aa")).
		AddTerm(index.NewTerm("field", "bb")).
		Build()

	single := scoreOfDoc(t, searcher, query, 0) // freq 1
	both := scoreOfDoc(t, searcher, query, 1)   // freq 2

	if !(both > single) {
		t.Errorf("expected summed synonym freq (doc1, freq=2)=%v to outscore single term (doc0, freq=1)=%v", both, single)
	}
}

// TestSynonymQuery_SingleTermMatches verifies the degenerate single-term synonym
// query still matches and scores. Lucene optimizes this to a TermScorer; the
// Gocene SynonymScorer handles it directly with identical combined-freq
// semantics (a single sub-iterator).
func TestSynonymQuery_SingleTermMatches(t *testing.T) {
	// Docs: 0:"apple"  1:"apple apple"  2:"banana"
	searcher, _ := explainTestIndex(t, []string{"apple", "apple apple", "banana"})

	query := search.NewSynonymQueryBuilder("field").
		AddTerm(index.NewTerm("field", "apple")).
		Build()

	got := docSet(t, searcher, query)
	if _, ok := got[0]; !ok {
		t.Error("expected doc 0 to match single-term synonym query")
	}
	if _, ok := got[1]; !ok {
		t.Error("expected doc 1 to match single-term synonym query")
	}
	if _, ok := got[2]; ok {
		t.Error("did not expect doc 2 (banana) to match")
	}

	// Doc 1 has freq 2, doc 0 has freq 1 -> doc 1 scores higher.
	if !(got[1] > got[0]) {
		t.Errorf("expected doc1(freq=2)=%v to score higher than doc0(freq=1)=%v", got[1], got[0])
	}
}

// TestSpanTermQuery_MatchesRightDocs verifies that a SpanTermQuery matches
// exactly the documents containing the term, exercising the TermSpans/SpanScorer
// path ported in rmp #4749.
func TestSpanTermQuery_MatchesRightDocs(t *testing.T) {
	// Docs: 0:"the quick brown fox"  1:"lazy dog"  2:"the quick cat"
	searcher, _ := explainTestIndex(t, []string{"the quick brown fox", "lazy dog", "the quick cat"})

	query := search.NewSpanTermQuery(index.NewTerm("field", "quick"))

	got := docSet(t, searcher, query)
	if _, ok := got[0]; !ok {
		t.Error("expected doc 0 to match span term 'quick'")
	}
	if _, ok := got[2]; !ok {
		t.Error("expected doc 2 to match span term 'quick'")
	}
	if _, ok := got[1]; ok {
		t.Error("did not expect doc 1 (lazy dog) to match span term 'quick'")
	}
}

// TestSpanTermQuery_Positions verifies that the postings-backed Spans
// (NewTermSpans) reports the term's positions faithfully: one zero-width span
// per occurrence with endPosition == startPosition + 1, matching Lucene's
// TermSpans. It drives the Spans directly via the SpanTermWeight so the position
// stream can be inspected.
func TestSpanTermQuery_Positions(t *testing.T) {
	// Doc 0: "a quick brown quick fox" -> "quick" at positions 1 and 3.
	searcher, leaf := explainTestIndex(t, []string{"a quick brown quick fox"})

	query := search.NewSpanTermQuery(index.NewTerm("field", "quick"))
	weight, err := query.CreateWeight(searcher, true, 1.0)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	stw, ok := weight.(*search.SpanTermWeight)
	if !ok {
		t.Fatalf("expected *search.SpanTermWeight, got %T", weight)
	}

	spans, err := stw.GetSpans(leaf, index.PostingsFlagPositions)
	if err != nil {
		t.Fatalf("GetSpans: %v", err)
	}
	if spans == nil {
		t.Fatal("expected non-nil Spans for an existing term")
	}

	doc, err := spans.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc: %v", err)
	}
	if doc != 0 {
		t.Fatalf("expected first span doc 0, got %d", doc)
	}

	var positions []int
	for {
		pos, err := spans.NextStartPosition()
		if err != nil {
			t.Fatalf("NextStartPosition: %v", err)
		}
		if pos == -1 {
			break
		}
		// endPosition must be startPosition + 1 (zero-width term span).
		if got := spans.EndPosition(); got != pos+1 {
			t.Errorf("at start %d: endPosition = %d, want %d", pos, got, pos+1)
		}
		if got := spans.Width(); got != 0 {
			t.Errorf("at start %d: width = %d, want 0", pos, got)
		}
		positions = append(positions, pos)
	}

	want := []int{1, 3}
	if len(positions) != len(want) {
		t.Fatalf("positions = %v, want %v", positions, want)
	}
	for i, p := range want {
		if positions[i] != p {
			t.Fatalf("positions = %v, want %v", positions, want)
		}
	}
}

// TestSpanTermQuery_ScoresByFreq verifies the SpanScorer scores a document with
// more term occurrences higher, since each zero-width occurrence contributes 1.0
// to the sloppy frequency (Lucene SpanScorer.setFreqCurrentDoc with
// TermSpans.width()==0) and tf scoring is monotone in freq (rmp #4751).
func TestSpanTermQuery_ScoresByFreq(t *testing.T) {
	// Doc 0: "quick"             -> freq 1
	// Doc 1: "quick quick quick" -> freq 3
	searcher, _ := explainTestIndex(t, []string{"quick", "quick quick quick"})

	query := search.NewSpanTermQuery(index.NewTerm("field", "quick"))

	got := docSet(t, searcher, query)
	if _, ok := got[0]; !ok {
		t.Fatal("expected doc 0 to match")
	}
	if _, ok := got[1]; !ok {
		t.Fatal("expected doc 1 to match")
	}
	if !(got[1] > got[0]) {
		t.Errorf("expected doc1(freq=3)=%v to score higher than doc0(freq=1)=%v", got[1], got[0])
	}
}
