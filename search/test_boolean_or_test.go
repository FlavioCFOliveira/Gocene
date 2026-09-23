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
	"slices"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
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
	q := search.NewBooleanQueryBuilder()
	q.Add(booleanOrT1(), search.SHOULD)
	q.Add(booleanOrT2(), search.SHOULD)
	q.Add(booleanOrC1(), search.SHOULD)
	q.Add(booleanOrC2(), search.SHOULD)
	if got := booleanOrSearchCount(t, s, q.Build()); got != 1 {
		t.Errorf("hits = %d, want 1", got)
	}
}

// TestBooleanOr_ParenthesisMust ports testParenthesisMust:
// (T:files T:deleting) (+C:production +C:optimize).
func TestBooleanOr_ParenthesisMust(t *testing.T) {
	s, cleanup := newBooleanOrSearcher(t)
	defer cleanup()
	q3 := search.NewBooleanQueryBuilder()
	q3.Add(booleanOrT1(), search.SHOULD)
	q3.Add(booleanOrT2(), search.SHOULD)
	q4 := search.NewBooleanQueryBuilder()
	q4.Add(booleanOrC1(), search.MUST)
	q4.Add(booleanOrC2(), search.MUST)
	q2 := search.NewBooleanQueryBuilder()
	q2.Add(q3.Build(), search.SHOULD)
	q2.Add(q4.Build(), search.SHOULD)
	if got := booleanOrSearchCount(t, s, q2.Build()); got != 1 {
		t.Errorf("hits = %d, want 1", got)
	}
}

// TestBooleanOr_ParenthesisMust2 ports testParenthesisMust2:
// (T:files T:deleting) +(C:production C:optimize).
func TestBooleanOr_ParenthesisMust2(t *testing.T) {
	s, cleanup := newBooleanOrSearcher(t)
	defer cleanup()
	q3 := search.NewBooleanQueryBuilder()
	q3.Add(booleanOrT1(), search.SHOULD)
	q3.Add(booleanOrT2(), search.SHOULD)
	q4 := search.NewBooleanQueryBuilder()
	q4.Add(booleanOrC1(), search.SHOULD)
	q4.Add(booleanOrC2(), search.SHOULD)
	q2 := search.NewBooleanQueryBuilder()
	q2.Add(q3.Build(), search.SHOULD)
	q2.Add(q4.Build(), search.MUST)
	if got := booleanOrSearchCount(t, s, q2.Build()); got != 1 {
		t.Errorf("hits = %d, want 1", got)
	}
}

// TestBooleanOr_ParenthesisShould ports testParenthesisShould:
// (T:files T:deleting) (C:production C:optimize).
func TestBooleanOr_ParenthesisShould(t *testing.T) {
	s, cleanup := newBooleanOrSearcher(t)
	defer cleanup()
	q3 := search.NewBooleanQueryBuilder()
	q3.Add(booleanOrT1(), search.SHOULD)
	q3.Add(booleanOrT2(), search.SHOULD)
	q4 := search.NewBooleanQueryBuilder()
	q4.Add(booleanOrC1(), search.SHOULD)
	q4.Add(booleanOrC2(), search.SHOULD)
	q2 := search.NewBooleanQueryBuilder()
	q2.Add(q3.Build(), search.SHOULD)
	q2.Add(q4.Build(), search.SHOULD)
	if got := booleanOrSearchCount(t, s, q2.Build()); got != 1 {
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

func (c *booleanOrMaxCollector) SetScorer(_ search.Scorable) error { return nil }
func (c *booleanOrMaxCollector) Collect(doc int) error {
	if doc >= *c.end {
		c.t.Errorf("collected doc=%d beyond max=%d", doc, *c.end)
	}
	c.hits[doc] = true
	return nil
}

// CollectRange carries the default body Lucene gives LeafCollector.CollectRange.
func (c *booleanOrMaxCollector) CollectRange(min int, max int) error {
	return search.DefaultCollectRange(c, min, max)
}

// CollectStream carries the default body Lucene gives LeafCollector.CollectStream.
func (c *booleanOrMaxCollector) CollectStream(stream search.DocIdStream) error {
	return search.DefaultCollectStream(c, stream)
}

// CompetitiveIterator carries the default body Lucene gives LeafCollector.CompetitiveIterator.
func (c *booleanOrMaxCollector) CompetitiveIterator() (search.DocIdSetIterator, error) {
	return nil, nil
}

// Finish carries the default body Lucene gives LeafCollector.Finish.
func (c *booleanOrMaxCollector) Finish() error {
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

	bq := search.NewBooleanQueryBuilder()
	bq.Add(search.NewTermQuery(index.NewTerm("field", "a")), search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm("field", "a")), search.SHOULD)

	rewritten, err := bq.Build().Rewrite(s)
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

// Iterator returns the double itself: it iterates its own documents.
func (s *booleanOrIntScorer) Iterator() search.DocIdSetIterator { return s }

func (s *booleanOrIntScorer) Cost() int64                        { return int64(len(s.docs)) }
func (s *booleanOrIntScorer) DocIDRunEnd() (int, error)          { return s.DocID() + 1, nil }
func (s *booleanOrIntScorer) Score() (float32, error)            { return 0, nil }
func (s *booleanOrIntScorer) GetMaxScore(_ int) (float32, error) { return math.MaxFloat32, nil }

// booleanOrCollectCollector renders the anonymous LeafCollector of
// testSubScorerNextIsNotMatch: setScorer is a no-op and collect records the
// doc; the other members keep LeafCollector's default bodies.
type booleanOrCollectCollector struct {
	*search.BaseLeafCollector
	matches []int
}

func (c *booleanOrCollectCollector) SetScorer(_ search.Scorable) error { return nil }

func (c *booleanOrCollectCollector) Collect(doc int) error {
	c.matches = append(c.matches, doc)
	return nil
}

func (c *booleanOrCollectCollector) CollectRange(min, max int) error {
	return search.DefaultCollectRange(c, min, max)
}

func (c *booleanOrCollectCollector) CollectStream(stream search.DocIdStream) error {
	return search.DefaultCollectStream(c, stream)
}

// Make sure that BooleanScorer keeps working even if the sub clauses return
// next matching docs which are less than the actual next match.
func TestBooleanOr_SubScorerNextIsNotMatch(t *testing.T) {
	optionalScorers := []search.Scorer{
		newBooleanOrIntScorer(100000, 1000001, 9999999),
		newBooleanOrIntScorer(4000, 1000051),
		newBooleanOrIntScorer(5000, 100000, 9999998, 9999999),
	}
	rand.Shuffle(len(optionalScorers), func(i, j int) {
		optionalScorers[i], optionalScorers[j] = optionalScorers[j], optionalScorers[i]
	})
	scorer, err := search.NewBooleanScorer(optionalScorers, 1, rand.Intn(2) == 0)
	if err != nil {
		t.Fatalf("new BooleanScorer: %v", err)
	}
	collector := &booleanOrCollectCollector{BaseLeafCollector: search.NewBaseLeafCollector()}
	if _, err := scorer.Score(collector, nil, 0, util.NO_MORE_DOCS); err != nil {
		t.Fatalf("score: %v", err)
	}
	want := []int{4000, 5000, 100000, 1000001, 1000051, 9999998, 9999999}
	if !slices.Equal(want, collector.matches) {
		t.Fatalf("expected %v, got %v", want, collector.matches)
	}
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene
// 10.5.0, which every subclass inherits unless it overrides it.
func (s *booleanOrIntScorer) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(s, upTo, bitSet, offset)
}

// NextDocsAndScores carries the default body Lucene gives Scorer.NextDocsAndScores.
func (s *booleanOrIntScorer) NextDocsAndScores(upTo int, liveDocs util.Bits, buffer *search.DocAndFloatFeatureBuffer) error {
	return search.DefaultNextDocsAndScores(s, upTo, liveDocs, buffer)
}
