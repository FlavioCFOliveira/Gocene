// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestSloppyPhraseQuery.java
//
// The PhraseQuery Weight/Scorer (exact and sloppy, with explicit positions /
// holes) is implemented (rmp #8), so these cases run end-to-end against a real
// IndexWriter+IndexSearcher. Per the no-skip policy there are no skipped cases.
//
// Deviations from the reference, immaterial to the assertions:
//   - MockAnalyzer(WHITESPACE) is replaced by the WhitespaceAnalyzer.
//   - omitNorms is not configured; the assertions compare raw phrase freq and
//     check score finiteness, neither of which depends on norms.
//   - The MaxFreqCollector is reproduced inline by iterating the leaf scorer
//     and reading the (exact/sloppy) PhraseScorer phrase frequency.

package search

import (
	"math"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

const (
	sloppyS1 = "A A A"
	sloppyS2 = "A 1 2 3 A 4 5 6 A"
)

// sloppyDocText mirrors the makeDocument-wrapped corpus strings.
var (
	sloppyDOC1   = "X " + sloppyS1 + " Y"
	sloppyDOC2   = "X " + sloppyS2 + " Y"
	sloppyDOC3   = "X " + sloppyS1 + " A Y"
	sloppyDOC1B  = "X " + sloppyS1 + " Y N N N N " + sloppyS1 + " Z"
	sloppyDOC2B  = "X " + sloppyS2 + " Y N N N N " + sloppyS2 + " Z"
	sloppyDOC3B  = "X " + sloppyS1 + " A Y N N N N " + sloppyS1 + " A Y"
	sloppyDOC4   = "A A X A X B A X B B A A X B A A"
	sloppyDOC5_3 = "H H H X X X H H H X X X H H H"
	sloppyDOC5_4 = "H H H H"
)

// sloppyPhraseTerms splits a whitespace phrase into field terms (positions are
// implicitly 0,1,2,…), matching makePhraseQuery.
func sloppyPhraseTerms(field, phrase string) []*index.Term {
	parts := strings.Fields(phrase)
	terms := make([]*index.Term, len(parts))
	for i, p := range parts {
		terms[i] = index.NewTerm(field, p)
	}
	return terms
}

// indexOneDoc builds a single-document index on field "f" and returns a
// searcher plus cleanup. ForceMerge is deliberately not used (rmp #114).
func indexOneDoc(t *testing.T, field, text string) (*IndexSearcher, func()) {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	doc := document.NewDocument()
	f, err := document.NewTextField(field, text, false)
	if err != nil {
		t.Fatalf("NewTextField: %v", err)
	}
	doc.Add(f)
	if err := w.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
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
	return NewIndexSearcher(reader), func() {
		reader.Close()
		dir.Close()
	}
}

// phraseScorerStats reproduces MaxFreqCollector: it iterates the leaf scorer for
// q over the searcher's segments, returning the number of matching documents
// and the maximum (sloppy) phrase frequency observed.
func phraseScorerStats(t *testing.T, s *IndexSearcher, q *PhraseQuery) (totalHits int, maxFreq float32) {
	t.Helper()
	weight, err := q.CreateWeight(s, true, 1.0)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	if weight == nil {
		return 0, 0
	}
	dr, ok := s.GetIndexReader().(*index.DirectoryReader)
	if !ok {
		t.Fatalf("expected *index.DirectoryReader")
	}
	docBase := 0
	for ord, sr := range dr.GetSegmentReaders() {
		ctx := index.NewLeafReaderContext(sr, nil, ord, docBase)
		scorer, err := weight.Scorer(ctx)
		if err != nil {
			t.Fatalf("Scorer: %v", err)
		}
		if scorer != nil {
			for {
				doc, err := scorer.NextDoc()
				if err != nil {
					t.Fatalf("NextDoc: %v", err)
				}
				if doc < 0 || doc >= sr.MaxDoc() {
					break
				}
				totalHits++
				if pfs, ok := scorer.(phraseFreqScorer); ok {
					if f := pfs.PhraseFreq(); f > maxFreq {
						maxFreq = f
					}
				}
			}
		}
		docBase += sr.MaxDoc()
	}
	return totalHits, maxFreq
}

// checkPhraseQuery indexes docText, runs the phrase query (rebuilt with slop),
// asserts the hit count, and returns the maximum phrase frequency, mirroring
// the reference checkPhraseQuery helper.
func checkPhraseQuery(t *testing.T, docText string, terms []*index.Term, positions []int, slop, expected int) float32 {
	t.Helper()
	b := NewPhraseQueryBuilder()
	for i, term := range terms {
		b.AddTermAtPosition(term, positions[i])
	}
	b.SetSlop(slop)
	q := b.Build()

	s, cleanup := indexOneDoc(t, "f", docText)
	defer cleanup()

	got, maxFreq := phraseScorerStats(t, s, q)
	if got != expected {
		t.Errorf("slop=%d query=%v doc=%q: hits=%d, want %d", slop, q, docText, got, expected)
	}
	return maxFreq
}

// consecutivePositions returns [0,1,…,n-1].
func consecutivePositions(n int) []int {
	p := make([]int, n)
	for i := range p {
		p[i] = i
	}
	return p
}

func TestSloppyPhraseQuery_Doc4Query4(t *testing.T) {
	terms := sloppyPhraseTerms("f", "X A A")
	pos := consecutivePositions(len(terms))
	for slop := 0; slop < 30; slop++ {
		want := 0
		if slop >= 1 {
			want = 1
		}
		checkPhraseQuery(t, sloppyDOC4, terms, pos, slop, want)
	}
}

func TestSloppyPhraseQuery_Doc1Query1(t *testing.T) {
	terms := sloppyPhraseTerms("f", sloppyS1)
	pos := consecutivePositions(len(terms))
	for slop := 0; slop < 30; slop++ {
		freq1 := checkPhraseQuery(t, sloppyDOC1, terms, pos, slop, 1)
		freq2 := checkPhraseQuery(t, sloppyDOC1B, terms, pos, slop, 1)
		if !(freq2 > freq1) {
			t.Errorf("slop=%d freq2=%v should be greater than freq1=%v", slop, freq2, freq1)
		}
	}
}

func TestSloppyPhraseQuery_Doc2Query1(t *testing.T) {
	terms := sloppyPhraseTerms("f", sloppyS1)
	pos := consecutivePositions(len(terms))
	for slop := 0; slop < 30; slop++ {
		want := 0
		if slop >= 6 {
			want = 1
		}
		freq1 := checkPhraseQuery(t, sloppyDOC2, terms, pos, slop, want)
		if want > 0 {
			freq2 := checkPhraseQuery(t, sloppyDOC2B, terms, pos, slop, 1)
			if !(freq2 > freq1) {
				t.Errorf("slop=%d freq2=%v should be greater than freq1=%v", slop, freq2, freq1)
			}
		}
	}
}

func TestSloppyPhraseQuery_Doc2Query2(t *testing.T) {
	terms := sloppyPhraseTerms("f", sloppyS2)
	pos := consecutivePositions(len(terms))
	for slop := 0; slop < 30; slop++ {
		freq1 := checkPhraseQuery(t, sloppyDOC2, terms, pos, slop, 1)
		freq2 := checkPhraseQuery(t, sloppyDOC2B, terms, pos, slop, 1)
		if !(freq2 > freq1) {
			t.Errorf("slop=%d freq2=%v should be greater than freq1=%v", slop, freq2, freq1)
		}
	}
}

func TestSloppyPhraseQuery_Doc3Query1(t *testing.T) {
	terms := sloppyPhraseTerms("f", sloppyS1)
	pos := consecutivePositions(len(terms))
	for slop := 0; slop < 30; slop++ {
		freq1 := checkPhraseQuery(t, sloppyDOC3, terms, pos, slop, 1)
		freq2 := checkPhraseQuery(t, sloppyDOC3B, terms, pos, slop, 1)
		if !(freq2 > freq1) {
			t.Errorf("slop=%d freq2=%v should be greater than freq1=%v", slop, freq2, freq1)
		}
	}
}

func TestSloppyPhraseQuery_Doc5Query5(t *testing.T) {
	terms := sloppyPhraseTerms("f", sloppyDOC5_4) // QUERY_5_4 == "H H H H"
	pos := consecutivePositions(len(terms))
	const nRepeats = 5
	for slop := 0; slop < 3; slop++ {
		for trial := 0; trial < nRepeats; trial++ {
			checkPhraseQuery(t, sloppyDOC5_4, terms, pos, slop, 1)
		}
		for trial := 0; trial < nRepeats; trial++ {
			checkPhraseQuery(t, sloppyDOC5_3, terms, pos, slop, 0)
		}
	}
}

// TestSloppyPhraseQuery_SlopWithHoles ports testSlopWithHoles (LUCENE-3215):
// a phrase with explicit positions 1 and 4 (a hole of two) over four docs.
func TestSloppyPhraseQuery_SlopWithHoles(t *testing.T) {
	docs := []string{
		"drug drug",
		"drug druggy drug",
		"drug druggy druggy drug",
		"drug druggy drug druggy drug",
	}
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for _, text := range docs {
		doc := document.NewDocument()
		f, err := document.NewTextField("lyrics", text, false)
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

	build := func(slop int) *PhraseQuery {
		b := NewPhraseQueryBuilder()
		b.AddTermAtPosition(index.NewTerm("lyrics", "drug"), 1)
		b.AddTermAtPosition(index.NewTerm("lyrics", "drug"), 4)
		b.SetSlop(slop)
		return b.Build()
	}
	for slop, want := range map[int]int{0: 1, 1: 3, 2: 4} {
		top, err := s.Search(build(slop), 4)
		if err != nil {
			t.Fatalf("search slop=%d: %v", slop, err)
		}
		if got := int(top.TotalHits.Value); got != want {
			t.Errorf("slop=%d: hits=%d, want %d", slop, got, want)
		}
	}
}

// TestSloppyPhraseQuery_InfiniteFreq1 ports testInfiniteFreq1 (LUCENE-3215):
// repeated terms must not yield infinite scores.
func TestSloppyPhraseQuery_InfiniteFreq1(t *testing.T) {
	s, cleanup := indexOneDoc(t, "lyrics", "drug druggy drug drug drug")
	defer cleanup()
	b := NewPhraseQueryBuilder()
	b.AddTermAtPosition(index.NewTerm("lyrics", "drug"), 1)
	b.AddTermAtPosition(index.NewTerm("lyrics", "drug"), 3)
	b.SetSlop(1)
	assertSaneScoring(t, s, b.Build())
}

// TestSloppyPhraseQuery_InfiniteFreq2 ports testInfiniteFreq2 (LUCENE-3215).
func TestSloppyPhraseQuery_InfiniteFreq2(t *testing.T) {
	document := strings.Join([]string{
		"So much fun to be had in my head", "No more sunshine",
		"So much fun just lying in my bed", "No more sunshine",
		"I can't face the sunlight and the dirt outside",
		"Wanna stay in 666 where this darkness don't lie",
		"Drug drug druggy", "Got a feeling sweet like honey",
		"Drug drug druggy", "Need sensation like my baby",
		"Show me your scars you're so aware", "I'm not barbaric I just care",
		"Drug drug drug", "I need a reflection to prove I exist",
		"No more sunshine", "I am a victim of designer blitz",
		"No more sunshine", "Dance like a robot when you're chained at the knee",
		"The C.I.A say you're all they'll ever need",
		"Drug drug druggy", "Got a feeling sweet like honey",
		"Drug drug druggy", "Need sensation like my baby",
		"Snort your lines you're so aware", "I'm not barbaric I just care",
		"Drug drug druggy", "Got a feeling sweet like honey",
		"Drug drug druggy", "Need sensation like my baby",
	}, " ")
	s, cleanup := indexOneDoc(t, "lyrics", document)
	defer cleanup()
	b := NewPhraseQueryBuilder()
	b.AddTermAtPosition(index.NewTerm("lyrics", "drug"), 1)
	b.AddTermAtPosition(index.NewTerm("lyrics", "drug"), 4)
	b.SetSlop(5)
	assertSaneScoring(t, s, b.Build())
}

// assertSaneScoring checks that no matching document scores to infinity,
// mirroring the reference assertSaneScoring (minus QueryUtils.check).
func assertSaneScoring(t *testing.T, s *IndexSearcher, q *PhraseQuery) {
	t.Helper()
	weight, err := q.CreateWeight(s, true, 1.0)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	if weight == nil {
		return
	}
	dr := s.GetIndexReader().(*index.DirectoryReader)
	docBase := 0
	for ord, sr := range dr.GetSegmentReaders() {
		ctx := index.NewLeafReaderContext(sr, nil, ord, docBase)
		scorer, err := weight.Scorer(ctx)
		if err != nil {
			t.Fatalf("Scorer: %v", err)
		}
		if scorer != nil {
			for {
				doc, err := scorer.NextDoc()
				if err != nil {
					t.Fatalf("NextDoc: %v", err)
				}
				if doc < 0 || doc >= sr.MaxDoc() {
					break
				}
				score := scorer.Score()
				if math.IsInf(float64(score), 0) {
					t.Errorf("doc=%d scored to infinity", doc+docBase)
				}
			}
		docBase += sr.MaxDoc()
	}
}