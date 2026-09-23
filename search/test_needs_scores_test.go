// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestNeedsScores.java
// (Apache Lucene 10.5.0).

package search_test

import (
	"math"
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	testsearch "github.com/FlavioCFOliveira/Gocene/tests/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// needsScoresSetUp renders setUp(); the returned function renders tearDown().
func needsScoresSetUp(t *testing.T) (*search.IndexSearcher, func()) {
	t.Helper()
	dir := newDirectory()
	iw := newRandomIndexWriter(t, dir)
	for i := 0; i < 5; i++ {
		doc := document.NewDocument()
		f, err := document.NewTextField("field", "this is document "+strconv.Itoa(i), false)
		if err != nil {
			t.Fatalf("new TextField: %v", err)
		}
		doc.Add(f)
		mustAddDocument(t, iw, doc)
	}
	reader := mustGetReader(t, iw)
	tearDown := func() {
		if err := util.CloseAll(reader, dir); err != nil {
			t.Errorf("IOUtils.close: %v", err)
		}
	}
	searcher := newSearcher(t, reader)
	// Needed so that the cache doesn't consume weights with ScoreMode.COMPLETE_NO_SCORES for the
	// purpose of populating the cache.
	searcher.SetQueryCache(nil)
	mustClose(t, iw)
	return searcher, tearDown
}

// prohibited clauses in booleanquery don't need scoring
func TestNeedsScoresProhibitedClause(t *testing.T) {
	searcher, tearDown := needsScoresSetUp(t)
	defer tearDown()
	required := search.NewTermQuery(index.NewTerm("field", "this"))
	prohibited := search.NewTermQuery(index.NewTerm("field", "3"))
	bq := search.NewBooleanQueryBuilder()
	bq.Add(newAssertNeedsScores(t, required, search.TOP_SCORES), search.MUST)
	bq.Add(newAssertNeedsScores(t, prohibited, search.COMPLETE_NO_SCORES), search.MUST_NOT)
	if got := mustSearch(t, searcher, bq.Build(), 5).TotalHits.Value; got != 4 { // we exclude 3
		t.Fatalf("totalHits = %d, want 4", got)
	}
}

// nested inside constant score query
func TestNeedsScoresConstantScoreQuery(t *testing.T) {
	searcher, tearDown := needsScoresSetUp(t)
	defer tearDown()
	term := search.NewTermQuery(index.NewTerm("field", "this"))

	// Counting queries and top-score queries that compute the hit count should use
	// COMPLETE_NO_SCORES
	var constantScore search.Query = search.NewConstantScoreQuery(newAssertNeedsScores(t, term, search.COMPLETE_NO_SCORES))
	assertIntEquals(t, 5, mustCount(t, searcher, constantScore))
	manager, err := search.NewTopScoreDocCollectorManager(5, nil, math.MaxInt32)
	if err != nil {
		t.Fatal(err)
	}
	hits, err := search.SearchWithCollectorManager[*search.TopScoreDocCollector, *search.TopDocs](searcher, constantScore, manager)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if hits.TotalHits.Value != 5 {
		t.Fatalf("totalHits = %d, want 5", hits.TotalHits.Value)
	}

	// Queries that support dynamic pruning like top-score or top-doc queries that do not compute
	// the hit count should use TOP_DOCS
	constantScore = search.NewConstantScoreQuery(newAssertNeedsScores(t, term, search.TOP_DOCS))
	if got := mustSearch(t, searcher, constantScore, 5).TotalHits.Value; got != 5 {
		t.Fatalf("totalHits = %d, want 5", got)
	}
	if got := needsScoresSearchSorted(t, searcher, constantScore, 5, search.NewSort(search.FIELD_DOC)).TotalHits.Value; got != 5 {
		t.Fatalf("totalHits = %d, want 5", got)
	}
	if got := needsScoresSearchSorted(t, searcher, constantScore, 5, search.NewSort(search.FIELD_DOC, search.FieldScore)).TotalHits.Value; got != 5 {
		t.Fatalf("totalHits = %d, want 5", got)
	}
}

// when not sorting by score
func TestNeedsScoresSortByField(t *testing.T) {
	searcher, tearDown := needsScoresSetUp(t)
	defer tearDown()
	query := newAssertNeedsScores(t, search.NewMatchAllDocsQuery(), search.TOP_DOCS)
	if got := needsScoresSearchSorted(t, searcher, query, 5, search.INDEXORDER).TotalHits.Value; got != 5 {
		t.Fatalf("totalHits = %d, want 5", got)
	}
}

// when sorting by score
func TestNeedsScoresSortByScore(t *testing.T) {
	searcher, tearDown := needsScoresSetUp(t)
	defer tearDown()
	query := newAssertNeedsScores(t, search.NewMatchAllDocsQuery(), search.TOP_SCORES)
	if got := needsScoresSearchSorted(t, searcher, query, 5, search.RELEVANCE).TotalHits.Value; got != 5 {
		t.Fatalf("totalHits = %d, want 5", got)
	}
}

// needsScoresSearchSorted renders IndexSearcher.search(Query, int, Sort).
func needsScoresSearchSorted(t *testing.T, searcher *search.IndexSearcher, q search.Query, n int, sort *search.Sort) *search.TopFieldDocs {
	t.Helper()
	td, err := searcher.SearchWithSort(q, n, sort, false)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	return td
}

// assertNeedsScores renders the static class AssertNeedsScores: wraps a query,
// checking that the needsScores param passed to Weight.scorer is the expected
// value.
type assertNeedsScores struct {
	search.BaseQuery
	t     *testing.T
	in    search.Query
	value search.ScoreMode
}

func newAssertNeedsScores(t *testing.T, in search.Query, value search.ScoreMode) *assertNeedsScores {
	if in == nil {
		panic("Objects.requireNonNull(in)")
	}
	return &assertNeedsScores{t: t, in: in, value: value}
}

// CreateWeight renders createWeight(IndexSearcher, ScoreMode, float).
func (q *assertNeedsScores) CreateWeight(searcher *search.IndexSearcher, scoreMode search.ScoreMode, boost float32) (search.Weight, error) {
	w, err := q.in.CreateWeight(searcher, scoreMode, boost)
	if err != nil {
		return nil, err
	}
	return &assertNeedsScoresWeight{FilterWeight: search.NewFilterWeight(w), w: w, q: q, scoreMode: scoreMode}, nil
}

// assertNeedsScoresWeight renders the anonymous FilterWeight of createWeight.
type assertNeedsScoresWeight struct {
	*search.FilterWeight
	w         search.Weight
	q         *assertNeedsScores
	scoreMode search.ScoreMode
}

// ScorerSupplier renders the scorerSupplier(LeafReaderContext) override.
func (w *assertNeedsScoresWeight) ScorerSupplier(context *index.LeafReaderContext) (search.ScorerSupplier, error) {
	scorerSupplier, err := w.w.ScorerSupplier(context)
	if err != nil {
		return nil, err
	}
	if scorerSupplier == nil {
		return nil, nil
	}
	scorer, err := scorerSupplier.Get(math.MaxInt64)
	if err != nil {
		return nil, err
	}
	return &assertNeedsScoresSupplier{w: w, scorer: scorer}, nil
}

// Scorer renders the inherited Weight.scorer(LeafReaderContext), which
// dispatches to the scorerSupplier override.
func (w *assertNeedsScoresWeight) Scorer(context *index.LeafReaderContext) (search.Scorer, error) {
	scorerSupplier, err := w.ScorerSupplier(context)
	if err != nil || scorerSupplier == nil {
		return nil, err
	}
	return scorerSupplier.Get(math.MaxInt64)
}

// BulkScorer renders the inherited Weight.bulkScorer(LeafReaderContext),
// which dispatches to the scorerSupplier override.
func (w *assertNeedsScoresWeight) BulkScorer(context *index.LeafReaderContext) (search.BulkScorer, error) {
	scorerSupplier, err := w.ScorerSupplier(context)
	if err != nil || scorerSupplier == nil {
		return nil, err
	}
	if err := scorerSupplier.SetTopLevelScoringClause(); err != nil {
		return nil, err
	}
	return scorerSupplier.BulkScorer()
}

// assertNeedsScoresSupplier renders the anonymous ScorerSupplier of
// scorerSupplier(LeafReaderContext).
type assertNeedsScoresSupplier struct {
	search.BaseScorerSupplier
	w      *assertNeedsScoresWeight
	scorer search.Scorer
}

func (s *assertNeedsScoresSupplier) Get(leadCost int64) (search.Scorer, error) {
	q := s.w.q
	if q.value != s.w.scoreMode {
		q.t.Errorf("query=%v: expected %v, got %v", q.in, q.value, s.w.scoreMode)
	}
	return s.scorer, nil
}

func (s *assertNeedsScoresSupplier) Cost() int64 {
	return s.scorer.Iterator().Cost()
}

func (s *assertNeedsScoresSupplier) BulkScorer() (search.BulkScorer, error) {
	return search.DefaultScorerSupplierBulkScorer(s)
}

// Rewrite renders rewrite(IndexSearcher).
func (q *assertNeedsScores) Rewrite(searcher *search.IndexSearcher) (search.Query, error) {
	in2, err := q.in.Rewrite(searcher)
	if err != nil {
		return nil, err
	}
	if in2 == q.in {
		return q, nil
	}
	return newAssertNeedsScores(q.t, in2, q.value), nil
}

// Visit renders visit(QueryVisitor).
func (q *assertNeedsScores) Visit(visitor search.QueryVisitor) {
	q.in.Visit(visitor)
}

// HashCode renders hashCode(). classHash() is the hash of the class name;
// Java's ScoreMode.hashCode() is the enum's identity hash, rendered by its
// ordinal.
func (q *assertNeedsScores) HashCode() int {
	const prime = 31
	result := int(javaStringHashCode("org.apache.lucene.search.TestNeedsScores$AssertNeedsScores"))
	result = prime*result + q.in.HashCode()
	result = prime*result + int(q.value)
	return result
}

// Equals renders equals(Object): sameClassAs(other) && equalsTo(other).
func (q *assertNeedsScores) Equals(other spi.Query) bool {
	o, ok := other.(*assertNeedsScores)
	return ok && q.in.Equals(o.in) && q.value == o.value
}

// ToString renders toString(String field).
func (q *assertNeedsScores) ToString(field string) string {
	return "asserting(" + testsearch.QueryString(q.in, field) + ")"
}

func (q *assertNeedsScores) String() string { return q.ToString("") }
