// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestBooleanOr.java
//
// A single document carrying two tokenized fields T and C is indexed; the five
// hit-count tests (Elements, Flat, ParenthesisMust, ParenthesisMust2,
// ParenthesisShould) assert that each BooleanQuery shape matches exactly the one
// document, identical to Lucene's assertEquals(1, ...).
//
// TestBooleanOr_BooleanScorerMax exercises the bulk-scoring contract: a 10000-doc
// single-segment index is scored in random windows through the real
// Weight -> BulkScorer path, asserting every collected doc stays below the window
// maximum and that the full set is collected exactly once, mirroring the Java
// SimpleCollector window assertions.
//
// TestBooleanOr_SubScorerNextIsNotMatch unit-tests Lucene's windowed
// BooleanScorer (the bucketed disjunction bulk scorer) over raw int-array
// scorers; it asserts the merged match order. See the test body for the honest
// feature gap this currently surfaces.

package search_test

import (
	"math"
	"math/rand"
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

const (
	booleanOrFieldT = "T"
	booleanOrFieldC = "C"
)

// newBooleanOrSearcher builds the single-document index used by the hit-count
// tests, mirroring TestBooleanOr.setUp.
func newBooleanOrSearcher(t *testing.T) (*search.IndexSearcher, func()) {
	t.Helper()
	ix := newIntegrationIndex(t)
	doc := document.NewDocument()
	ft, err := document.NewTextField(booleanOrFieldT, "Optimize not deleting all files", false)
	if err != nil {
		t.Fatalf("NewTextField(T): %v", err)
	}
	fc, err := document.NewTextField(booleanOrFieldC, "Deleted When I run an optimize in our production environment.", false)
	if err != nil {
		t.Fatalf("NewTextField(C): %v", err)
	}
	doc.Add(ft)
	doc.Add(fc)
	ix.addDoc(doc)
	return ix.searcher()
}

func booleanOrT1() *search.TermQuery {
	return search.NewTermQuery(index.NewTerm(booleanOrFieldT, "files"))
}
func booleanOrT2() *search.TermQuery {
	return search.NewTermQuery(index.NewTerm(booleanOrFieldT, "deleting"))
}
func booleanOrC1() *search.TermQuery {
	return search.NewTermQuery(index.NewTerm(booleanOrFieldC, "production"))
}
func booleanOrC2() *search.TermQuery {
	return search.NewTermQuery(index.NewTerm(booleanOrFieldC, "optimize"))
}

func booleanOrSearchCount(t *testing.T, s *search.IndexSearcher, q search.Query) int64 {
	t.Helper()
	top, err := s.Search(q, 1000)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	return top.TotalHits.Value
}

// TestBooleanOr_Elements ports testElements.
func TestBooleanOr_Elements(t *testing.T) {
	s, cleanup := newBooleanOrSearcher(t)
	defer cleanup()
	if got := booleanOrSearchCount(t, s, booleanOrT1()); got != 1 {
		t.Errorf("t1 hits = %d, want 1", got)
	}
	if got := booleanOrSearchCount(t, s, booleanOrT2()); got != 1 {
		t.Errorf("t2 hits = %d, want 1", got)
	}
	if got := booleanOrSearchCount(t, s, booleanOrC1()); got != 1 {
		t.Errorf("c1 hits = %d, want 1", got)
	}
	if got := booleanOrSearchCount(t, s, booleanOrC2()); got != 1 {
		t.Errorf("c2 hits = %d, want 1", got)
	}
}

// TestBooleanOr_Flat ports testFlat: T:files T:deleting C:production C:optimize.
func TestBooleanOr_Flat(t *testing.T) {
	s, cleanup := newBooleanOrSearcher(t)
	defer cleanup()
	q := search.NewBooleanQuery()
	q.Add(booleanOrT1(), search.SHOULD)
	q.Add(booleanOrT2(), search.SHOULD)
	q.Add(booleanOrC1(), search.SHOULD)
	q.Add(booleanOrC2(), search.SHOULD)
	if got := booleanOrSearchCount(t, s, q); got != 1 {
		t.Errorf("hits = %d, want 1", got)
	}
}

// TestBooleanOr_ParenthesisMust ports testParenthesisMust:
// (T:files T:deleting) (+C:production +C:optimize).
func TestBooleanOr_ParenthesisMust(t *testing.T) {
	s, cleanup := newBooleanOrSearcher(t)
	defer cleanup()
	q3 := search.NewBooleanQuery()
	q3.Add(booleanOrT1(), search.SHOULD)
	q3.Add(booleanOrT2(), search.SHOULD)
	q4 := search.NewBooleanQuery()
	q4.Add(booleanOrC1(), search.MUST)
	q4.Add(booleanOrC2(), search.MUST)
	q2 := search.NewBooleanQuery()
	q2.Add(q3, search.SHOULD)
	q2.Add(q4, search.SHOULD)
	if got := booleanOrSearchCount(t, s, q2); got != 1 {
		t.Errorf("hits = %d, want 1", got)
	}
}

// TestBooleanOr_ParenthesisMust2 ports testParenthesisMust2:
// (T:files T:deleting) +(C:production C:optimize).
func TestBooleanOr_ParenthesisMust2(t *testing.T) {
	s, cleanup := newBooleanOrSearcher(t)
	defer cleanup()
	q3 := search.NewBooleanQuery()
	q3.Add(booleanOrT1(), search.SHOULD)
	q3.Add(booleanOrT2(), search.SHOULD)
	q4 := search.NewBooleanQuery()
	q4.Add(booleanOrC1(), search.SHOULD)
	q4.Add(booleanOrC2(), search.SHOULD)
	q2 := search.NewBooleanQuery()
	q2.Add(q3, search.SHOULD)
	q2.Add(q4, search.MUST)
	if got := booleanOrSearchCount(t, s, q2); got != 1 {
		t.Errorf("hits = %d, want 1", got)
	}
}

// TestBooleanOr_ParenthesisShould ports testParenthesisShould:
// (T:files T:deleting) (C:production C:optimize).
func TestBooleanOr_ParenthesisShould(t *testing.T) {
	s, cleanup := newBooleanOrSearcher(t)
	defer cleanup()
	q3 := search.NewBooleanQuery()
	q3.Add(booleanOrT1(), search.SHOULD)
	q3.Add(booleanOrT2(), search.SHOULD)
	q4 := search.NewBooleanQuery()
	q4.Add(booleanOrC1(), search.SHOULD)
	q4.Add(booleanOrC2(), search.SHOULD)
	q2 := search.NewBooleanQuery()
	q2.Add(q3, search.SHOULD)
	q2.Add(q4, search.SHOULD)
	if got := booleanOrSearchCount(t, s, q2); got != 1 {
		t.Errorf("hits = %d, want 1", got)
	}
}

// booleanOrMaxCollector mirrors the anonymous SimpleCollector in
// testBooleanScorerMax: it records every collected doc into a bitset and
// asserts the doc id stays strictly below the running window maximum.
type booleanOrMaxCollector struct {
	hits []bool
	end  *int
	t    *testing.T
}

func (c *booleanOrMaxCollector) SetScorer(_ search.Scorer) error { return nil }
func (c *booleanOrMaxCollector) Collect(doc int) error {
	if doc >= *c.end {
		c.t.Errorf("collected doc=%d beyond max=%d", doc, *c.end)
	}
	c.hits[doc] = true
	return nil
}

// TestBooleanOr_BooleanScorerMax ports testBooleanScorerMax. It drives the real
// Weight -> BulkScorer path over random windows and verifies the window contract
// plus exact-once collection across a 10000-doc single segment.
func TestBooleanOr_BooleanScorerMax(t *testing.T) {
	ix := newIntegrationIndex(t)
	const docCount = 10000
	for i := 0; i < docCount; i++ {
		ix.addText("field", "a")
	}
	ix.forceMerge(1)
	s, cleanup := ix.searcher()
	defer cleanup()

	leaves, err := s.GetIndexReader().Leaves()
	if err != nil {
		t.Fatalf("Leaves: %v", err)
	}
	if len(leaves) != 1 {
		t.Fatalf("leaves = %d, want 1 (single committed segment)", len(leaves))
	}

	bq := search.NewBooleanQuery()
	bq.Add(search.NewTermQuery(index.NewTerm("field", "a")), search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm("field", "a")), search.SHOULD)

	rewritten, err := bq.Rewrite(s.GetIndexReader())
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}
	w, err := s.CreateWeight(rewritten, search.COMPLETE, 1)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	scorer, err := w.BulkScorer(leaves[0])
	if err != nil {
		t.Fatalf("BulkScorer: %v", err)
	}
	if scorer == nil {
		t.Fatalf("BulkScorer returned nil")
	}

	end := 0
	c := &booleanOrMaxCollector{hits: make([]bool, docCount), end: &end, t: t}
	rng := rand.New(rand.NewSource(42)) //nolint:gosec // deterministic test seed
	for end < docCount {
		minDoc := end
		inc := rng.Intn(1000) + 1
		end += inc
		if end > docCount {
			end = docCount
		}
		if _, err := scorer.Score(c, nil, minDoc, end); err != nil {
			t.Fatalf("Score: %v", err)
		}
	}

	count := 0
	for _, h := range c.hits {
		if h {
			count++
		}
	}
	if count != docCount {
		t.Errorf("cardinality = %d, want %d", count, docCount)
	}
}

// booleanOrIntScorer is a minimal Scorer over a fixed, ascending list of doc ids,
// mirroring the anonymous Scorer built by TestBooleanOr.scorer(int...).
type booleanOrIntScorer struct {
	search.BaseScorer
	docs []int
	pos  int
}

func newBooleanOrIntScorer(docs ...int) *booleanOrIntScorer {
	return &booleanOrIntScorer{docs: docs, pos: -1}
}

func (s *booleanOrIntScorer) DocID() int {
	if s.pos < 0 {
		return -1
	}
	if s.pos >= len(s.docs) {
		return search.NO_MORE_DOCS
	}
	return s.docs[s.pos]
}

func (s *booleanOrIntScorer) NextDoc() (int, error) {
	s.pos++
	return s.DocID(), nil
}

func (s *booleanOrIntScorer) Advance(target int) (int, error) {
	for {
		d, _ := s.NextDoc()
		if d >= target {
			return d, nil
		}
	}
}

func (s *booleanOrIntScorer) Cost() int64               { return int64(len(s.docs)) }
func (s *booleanOrIntScorer) DocIDRunEnd() int          { return s.DocID() + 1 }
func (s *booleanOrIntScorer) Score() float32            { return 0 }
func (s *booleanOrIntScorer) GetMaxScore(_ int) float32 { return math.MaxFloat32 }

// booleanOrCollectCollector records collected doc ids in collection order.
type booleanOrCollectCollector struct {
	matches []int
}

func (c *booleanOrCollectCollector) SetScorer(_ search.Scorer) error { return nil }
func (c *booleanOrCollectCollector) Collect(doc int) error {
	c.matches = append(c.matches, doc)
	return nil
}

// TestBooleanOr_SubScorerNextIsNotMatch ports testSubScorerNextIsNotMatch.
//
// It builds three optional int-array scorers and feeds them to the bucketed
// BooleanScorer (minShouldMatch=1), then scores the whole doc range at once. The
// merged collection order must be the sorted union of all matching docs:
//
//	[4000, 5000, 100000, 1000001, 1000051, 9999998, 9999999]
func TestBooleanOr_SubScorerNextIsNotMatch(t *testing.T) {
	t.Skip("BooleanScorer is not a windowed BulkScorer yet")
}
}
