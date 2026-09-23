// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestBooleanOr.java
// (Apache Lucene 10.5.0).

package search_test

import (
	"math"
	"slices"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

const (
	booleanOrFieldT = "T"
	booleanOrFieldC = "C"
)

var (
	booleanOrT1 = search.NewTermQuery(index.NewTerm(booleanOrFieldT, "files"))
	booleanOrT2 = search.NewTermQuery(index.NewTerm(booleanOrFieldT, "deleting"))
	booleanOrC1 = search.NewTermQuery(index.NewTerm(booleanOrFieldC, "production"))
	booleanOrC2 = search.NewTermQuery(index.NewTerm(booleanOrFieldC, "optimize"))
)

// booleanOrSearch renders the private search(Query).
func booleanOrSearch(t *testing.T, searcher *search.IndexSearcher, q search.Query) int64 {
	t.Helper()
	queryUtilsCheckSearcher(t, q, searcher)
	return mustSearch(t, searcher, q, 1000).TotalHits.Value
}

func booleanOrAssertOne(t *testing.T, got int64) {
	t.Helper()
	if got != 1 {
		t.Fatalf("expected 1, got %d", got)
	}
}

func TestBooleanOrElements(t *testing.T) {
	searcher, tearDown := booleanOrSetUp(t)
	defer tearDown()
	booleanOrAssertOne(t, booleanOrSearch(t, searcher, booleanOrT1))
	booleanOrAssertOne(t, booleanOrSearch(t, searcher, booleanOrT2))
	booleanOrAssertOne(t, booleanOrSearch(t, searcher, booleanOrC1))
	booleanOrAssertOne(t, booleanOrSearch(t, searcher, booleanOrC2))
}

// T:files T:deleting C:production C:optimize it works.
func TestBooleanOrFlat(t *testing.T) {
	searcher, tearDown := booleanOrSetUp(t)
	defer tearDown()
	q := search.NewBooleanQueryBuilder()
	q.AddClause(search.NewBooleanClause(booleanOrT1, search.SHOULD))
	q.AddClause(search.NewBooleanClause(booleanOrT2, search.SHOULD))
	q.AddClause(search.NewBooleanClause(booleanOrC1, search.SHOULD))
	q.AddClause(search.NewBooleanClause(booleanOrC2, search.SHOULD))
	booleanOrAssertOne(t, booleanOrSearch(t, searcher, q.Build()))
}

// (T:files T:deleting) (+C:production +C:optimize) it works.
func TestBooleanOrParenthesisMust(t *testing.T) {
	searcher, tearDown := booleanOrSetUp(t)
	defer tearDown()
	q3 := search.NewBooleanQueryBuilder()
	q3.AddClause(search.NewBooleanClause(booleanOrT1, search.SHOULD))
	q3.AddClause(search.NewBooleanClause(booleanOrT2, search.SHOULD))
	q4 := search.NewBooleanQueryBuilder()
	q4.AddClause(search.NewBooleanClause(booleanOrC1, search.MUST))
	q4.AddClause(search.NewBooleanClause(booleanOrC2, search.MUST))
	q2 := search.NewBooleanQueryBuilder()
	q2.Add(q3.Build(), search.SHOULD)
	q2.Add(q4.Build(), search.SHOULD)
	booleanOrAssertOne(t, booleanOrSearch(t, searcher, q2.Build()))
}

// (T:files T:deleting) +(C:production C:optimize) not working. results NO HIT.
func TestBooleanOrParenthesisMust2(t *testing.T) {
	searcher, tearDown := booleanOrSetUp(t)
	defer tearDown()
	q3 := search.NewBooleanQueryBuilder()
	q3.AddClause(search.NewBooleanClause(booleanOrT1, search.SHOULD))
	q3.AddClause(search.NewBooleanClause(booleanOrT2, search.SHOULD))
	q4 := search.NewBooleanQueryBuilder()
	q4.AddClause(search.NewBooleanClause(booleanOrC1, search.SHOULD))
	q4.AddClause(search.NewBooleanClause(booleanOrC2, search.SHOULD))
	q2 := search.NewBooleanQueryBuilder()
	q2.Add(q3.Build(), search.SHOULD)
	q2.Add(q4.Build(), search.MUST)
	booleanOrAssertOne(t, booleanOrSearch(t, searcher, q2.Build()))
}

// (T:files T:deleting) (C:production C:optimize) not working. results NO HIT.
func TestBooleanOrParenthesisShould(t *testing.T) {
	searcher, tearDown := booleanOrSetUp(t)
	defer tearDown()
	q3 := search.NewBooleanQueryBuilder()
	q3.AddClause(search.NewBooleanClause(booleanOrT1, search.SHOULD))
	q3.AddClause(search.NewBooleanClause(booleanOrT2, search.SHOULD))
	q4 := search.NewBooleanQueryBuilder()
	q4.AddClause(search.NewBooleanClause(booleanOrC1, search.SHOULD))
	q4.AddClause(search.NewBooleanClause(booleanOrC2, search.SHOULD))
	q2 := search.NewBooleanQueryBuilder()
	q2.Add(q3.Build(), search.SHOULD)
	q2.Add(q4.Build(), search.SHOULD)
	booleanOrAssertOne(t, booleanOrSearch(t, searcher, q2.Build()))
}

// booleanOrSetUp renders setUp(); the returned function renders tearDown().
func booleanOrSetUp(t *testing.T) (*search.IndexSearcher, func()) {
	t.Helper()
	dir := newDirectory()
	writer := newRandomIndexWriter(t, dir)
	d := document.NewDocument()
	d.Add(newField(t, booleanOrFieldT, "Optimize not deleting all files", document.TextFieldTypeStored))
	d.Add(newField(t, booleanOrFieldC, "Deleted When I run an optimize in our production environment.", document.TextFieldTypeStored))
	mustAddDocument(t, writer, d)
	reader := mustGetReader(t, writer)
	searcher := newSearcher(t, reader)
	mustClose(t, writer)
	return searcher, func() {
		mustClose(t, reader, dir)
	}
}

// booleanOrMaxCollector renders the anonymous SimpleCollector of
// testBooleanScorerMax.
type booleanOrMaxCollector struct {
	search.BaseSimpleCollector
	search.BaseLeafCollector
	t    *testing.T
	hits *util.FixedBitSet
	end  *int
}

func (c *booleanOrMaxCollector) Collect(doc int) error {
	if !(doc < *c.end) {
		c.t.Errorf("collected doc=%d beyond max=%d", doc, *c.end)
	}
	c.hits.Set(doc)
	return nil
}

func (c *booleanOrMaxCollector) ScoreMode() search.ScoreMode { return search.COMPLETE_NO_SCORES }

func (c *booleanOrMaxCollector) SetScorer(scorer search.Scorable) error { return nil }

func (c *booleanOrMaxCollector) CollectRange(min, max int) error {
	return search.DefaultCollectRange(c, min, max)
}

func (c *booleanOrMaxCollector) CollectStream(stream search.DocIdStream) error {
	return search.DefaultCollectStream(c, stream)
}

func (c *booleanOrMaxCollector) CompetitiveIterator() (search.DocIdSetIterator, error) {
	return c.BaseLeafCollector.CompetitiveIterator()
}

func (c *booleanOrMaxCollector) Finish() error { return c.BaseLeafCollector.Finish() }

func TestBooleanOrBooleanScorerMax(t *testing.T) {
	_, tearDown := booleanOrSetUp(t)
	defer tearDown()
	dir := newDirectory()
	riw := newRandomIndexWriterWithConfig(t, dir, newIndexWriterConfigWithAnalyzer(testanalysis.NewMockAnalyzerRandom(random())))

	docCount := atLeast(10000)

	for i := 0; i < docCount; i++ {
		doc := document.NewDocument()
		doc.Add(newField(t, "field", "a", document.TextFieldTypeNotStored))
		mustAddDocument(t, riw, doc)
	}

	if err := riw.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	r := mustGetReader(t, riw)
	mustClose(t, riw)

	s := newSearcher(t, r)
	bq := search.NewBooleanQueryBuilder()
	bq.Add(search.NewTermQuery(index.NewTerm("field", "a")), search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm("field", "a")), search.SHOULD)

	w := mustCreateWeight(t, s, mustRewrite(t, s, bq.Build()), search.COMPLETE, 1)

	leaves := mustLeaves(t, s.GetIndexReader())
	assertIntEquals(t, 1, len(leaves))
	scorer, err := w.BulkScorer(leaves[0])
	if err != nil {
		t.Fatalf("bulkScorer: %v", err)
	}

	hits, err := util.NewFixedBitSet(docCount)
	if err != nil {
		t.Fatal(err)
	}
	end := 0
	c := &booleanOrMaxCollector{t: t, hits: hits, end: &end}
	c.Outer = c

	for end < docCount {
		min := end
		inc := nextInt(1, 1000)
		end += inc
		max := end
		if _, err := scorer.Score(c, nil, min, max); err != nil {
			t.Fatalf("score: %v", err)
		}
	}

	assertIntEquals(t, docCount, hits.Cardinality())
	mustClose(t, r, dir)
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
func TestBooleanOrSubScorerNextIsNotMatch(t *testing.T) {
	_, tearDown := booleanOrSetUp(t)
	defer tearDown()
	optionalScorers := []search.Scorer{
		newBooleanOrIntScorer(100000, 1000001, 9999999),
		newBooleanOrIntScorer(4000, 1000051),
		newBooleanOrIntScorer(5000, 100000, 9999998, 9999999),
	}
	random().Shuffle(len(optionalScorers), func(i, j int) {
		optionalScorers[i], optionalScorers[j] = optionalScorers[j], optionalScorers[i]
	})
	scorer, err := search.NewBooleanScorer(optionalScorers, 1, random().Intn(2) == 0)
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
