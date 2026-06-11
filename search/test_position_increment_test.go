// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//
//	lucene/core/src/test/org/apache/lucene/search/TestPositionIncrement.java
//
// This test verifies that the indexing pipeline honours per-token position
// increments and that PhraseQuery / MultiPhraseQuery respect the gaps they
// create. A custom analyzer emits the canned tokens {1,2,3,4,5} with position
// increments {1,2,1,0,1}, producing absolute positions {0,2,3,3,4}. The body
// confirms the stored positions via the postings enum and then asserts the exact
// hit counts the upstream test asserts for every phrase shape, including the
// zero-gap (stacked) tokens 3 and 4.
package search_test

import (
	"io"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"

	_ "github.com/FlavioCFOliveira/Gocene/codecs"
)

// TestPositionIncrement_TestCrazy mirrors TestPositionIncrement.testSetPosition.
func TestPositionIncrement_TestCrazy(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(&cannedPositionAnalyzer{}))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	d := document.NewDocument()
	f, err := document.NewTextField("field", "bogus", true)
	if err != nil {
		t.Fatalf("NewTextField: %v", err)
	}
	d.Add(f)
	if err := w.AddDocument(d); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("writer.Close: %v", err)
	}
	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer func() { _ = r.Close(); _ = dir.Close() }()
	searcher := search.NewIndexSearcher(r)

	// The first token ("1") should be at position 0.
	if pos := firstPosition(t, r, "field", "1"); pos != 0 {
		t.Errorf("position of token \"1\" = %d, want 0", pos)
	}
	// The second token ("2") should be at position 2 (increment 2 from 0).
	if pos := firstPosition(t, r, "field", "2"); pos != 2 {
		t.Errorf("position of token \"2\" = %d, want 2", pos)
	}

	mkTerm := func(text string) *index.Term { return index.NewTerm("field", text) }

	// "1" "2" adjacent (positions 0,1) -> no match (real gap is 0->2).
	assertPhraseHits(t, searcher, search.NewPhraseQuery("field", mkTerm("1"), mkTerm("2")), 0)

	// Builder with implicit consecutive positions -> still no match.
	assertPhraseHits(t, searcher, buildPhrase(mkTerm("1"), mkTerm("2")), 0)

	// Builder with explicit positions 0,1 -> no match.
	assertPhraseHits(t, searcher, buildPhraseAt([]termPos{{mkTerm("1"), 0}, {mkTerm("2"), 1}}), 0)

	// Builder with the correct positions 0,2 -> matches.
	assertPhraseHits(t, searcher, buildPhraseAt([]termPos{{mkTerm("1"), 0}, {mkTerm("2"), 2}}), 1)

	// "2" "3" adjacent (positions 2,3) -> matches.
	assertPhraseHits(t, searcher, search.NewPhraseQuery("field", mkTerm("2"), mkTerm("3")), 1)

	// "3" "4" adjacent -> no match: both are at position 3 (increment 0).
	assertPhraseHits(t, searcher, search.NewPhraseQuery("field", mkTerm("3"), mkTerm("4")), 0)

	// "3" "4" both at position 0 (relative) -> matches the stacked tokens.
	assertPhraseHits(t, searcher, buildPhraseAt([]termPos{{mkTerm("3"), 0}, {mkTerm("4"), 0}}), 1)

	// "3" "9" both at position 0 -> no match: "9" does not exist even though "3"
	// shares the position with "4".
	assertPhraseHits(t, searcher, buildPhraseAt([]termPos{{mkTerm("3"), 0}, {mkTerm("9"), 0}}), 0)

	// MultiPhraseQuery {3,9} at position 0 -> matches because "3" exists at that
	// stacked position.
	mqb := search.NewMultiPhraseQueryBuilder()
	mqb.AddTermsAtPosition([]*index.Term{mkTerm("3"), mkTerm("9")}, 0)
	assertPhraseHits(t, searcher, mqb.Build(), 1)

	// Remaining adjacency checks following the absolute positions {0,2,3,3,4}.
	assertPhraseHits(t, searcher, search.NewPhraseQuery("field", mkTerm("2"), mkTerm("4")), 1)
	assertPhraseHits(t, searcher, search.NewPhraseQuery("field", mkTerm("3"), mkTerm("5")), 1)
	assertPhraseHits(t, searcher, search.NewPhraseQuery("field", mkTerm("4"), mkTerm("5")), 1)
	assertPhraseHits(t, searcher, search.NewPhraseQuery("field", mkTerm("2"), mkTerm("5")), 0)
}

// firstPosition reads the first position of the first document containing term.
// It reads from the single segment reader directly (the index has one segment),
// mirroring the upstream MultiTerms.getTermPostingsEnum over a leaf reader; the
// segment-level postings enum carries the indexed positions.
func firstPosition(t *testing.T, r *index.DirectoryReader, field, term string) int {
	t.Helper()
	segs := r.GetSegmentReaders()
	if len(segs) != 1 {
		t.Fatalf("expected a single segment, got %d", len(segs))
	}
	terms, err := segs[0].Terms(field)
	if err != nil {
		t.Fatalf("Terms(%q): %v", field, err)
	}
	if terms == nil {
		t.Fatalf("Terms(%q) returned nil", field)
	}
	te, err := terms.GetIterator()
	if err != nil {
		t.Fatalf("GetIterator: %v", err)
	}
	found, err := te.SeekExact(index.NewTerm(field, term))
	if err != nil || !found {
		t.Fatalf("SeekExact(%q): found=%v err=%v", term, found, err)
	}
	pe, err := te.Postings(index.PostingsFlagPositions)
	if err != nil {
		t.Fatalf("Postings: %v", err)
	}
	if _, err := pe.NextDoc(); err != nil {
		t.Fatalf("NextDoc: %v", err)
	}
	pos, err := pe.NextPosition()
	if err != nil {
		t.Fatalf("NextPosition: %v", err)
	}
	return pos
}

// assertPhraseHits runs a query and asserts the exact number of hits.
func assertPhraseHits(t *testing.T, s *search.IndexSearcher, q search.Query, want int) {
	t.Helper()
	top, err := s.Search(q, 1000)
	if err != nil {
		t.Fatalf("Search(%v): %v", q, err)
	}
	if got := len(top.ScoreDocs); got != want {
		t.Errorf("query %v: hits = %d, want %d", q, got, want)
	}

// termPos pairs a term with the explicit query position it occupies.
type termPos struct {
	term *index.Term
	pos  int
}

// buildPhrase builds a PhraseQuery with implicit consecutive positions.
func buildPhrase(terms ...*index.Term) search.Query {
	b := search.NewPhraseQueryBuilder()
	for _, term := range terms {
		b.AddTerm(term)
	}
	return b.Build()
}

// buildPhraseAt builds a PhraseQuery with explicit per-term positions.
func buildPhraseAt(tps []termPos) search.Query {
	b := search.NewPhraseQueryBuilder()
	for _, tp := range tps {
		b.AddTermAtPosition(tp.term, tp.pos)
	}
	return b.Build()
}

// cannedPositionAnalyzer is an Analyzer whose TokenStream always yields the
// fixed token/position sequence from the upstream test, ignoring the reader.
type cannedPositionAnalyzer struct{}

func (a *cannedPositionAnalyzer) TokenStream(_ string, _ io.Reader) (analysis.TokenStream, error) {
	return newCannedPositionTokenizer(), nil
}

func (a *cannedPositionAnalyzer) Close() error { return nil }

var _ analysis.Analyzer = (*cannedPositionAnalyzer)(nil)

// cannedPositionTokenizer emits the tokens {1,2,3,4,5} with the position
// increments {1,2,1,0,1}, mirroring the anonymous Tokenizer in the upstream
// test. It is a Tokenizer (not just a TokenStream) because the indexing pipeline
// resets and drives the analyzer's tokenizer source.
type cannedPositionTokenizer struct {
	*analysis.BaseTokenizer

	termAttr    analysis.CharTermAttribute
	offsetAttr  analysis.OffsetAttribute
	posIncrAttr analysis.PositionIncrementAttribute

	i int
}

var (
	cannedTokens     = []string{"1", "2", "3", "4", "5"}
	cannedIncrements = []int{1, 2, 1, 0, 1}
)

func newCannedPositionTokenizer() *cannedPositionTokenizer {
	tk := &cannedPositionTokenizer{
		BaseTokenizer: analysis.NewBaseTokenizer(),
		termAttr:      analysis.NewCharTermAttribute(),
		offsetAttr:    analysis.NewOffsetAttribute(),
		posIncrAttr:   analysis.NewPositionIncrementAttribute(),
	}
	tk.AddAttribute(tk.termAttr)
	tk.AddAttribute(tk.offsetAttr)
	tk.AddAttribute(tk.posIncrAttr)
	return tk
}

func (tk *cannedPositionTokenizer) IncrementToken() (bool, error) {
	if tk.i == len(cannedTokens) {
		return false, nil
	}
	tk.ClearAttributes()
	tk.termAttr.SetValue(cannedTokens[tk.i])
	tk.offsetAttr.SetStartOffset(tk.i)
	tk.offsetAttr.SetEndOffset(tk.i)
	tk.posIncrAttr.SetPositionIncrement(cannedIncrements[tk.i])
	tk.i++
	return true, nil
}

func (tk *cannedPositionTokenizer) Reset() error {
	if err := tk.BaseTokenizer.Reset(); err != nil {
		return err
	}
	tk.i = 0
	return nil
}

var _ analysis.Tokenizer = (*cannedPositionTokenizer)(nil)