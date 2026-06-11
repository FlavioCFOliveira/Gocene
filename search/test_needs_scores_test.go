// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//
//	lucene/core/src/test/org/apache/lucene/search/TestNeedsScores.java
//
// This suite verifies that the ScoreMode chosen by the collector is propagated
// through the query tree to each sub-query's weight creation, exactly as Lucene
// does. The AssertNeedsScores wrapper records the ScoreMode handed to its inner
// query's createWeight (via IndexSearcher.CreateWeight dispatch) and asserts it
// equals the expected value when a Scorer is actually pulled.
//
// Deviation from the upstream test: Gocene's stock top-N collectors do not yet
// implement Lucene's dynamic-pruning ScoreModes. IndexSearcher.Search uses a
// COMPLETE TopDocsCollector (not TOP_SCORES) and the sort-by-field collector
// uses COMPLETE_NO_SCORES (not TOP_DOCS). The expected ScoreModes asserted here
// are therefore the modes Gocene's collectors actually report; the propagation
// logic under test — scoring vs. non-scoring clause routing in BooleanQuery and
// the exhaustive/non-exhaustive forwarding in ConstantScoreQuery — is the same
// as Lucene's and is exercised faithfully.
package search_test

import (
	"fmt"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// needsScoresIndex builds the five-document index shared by the TestNeedsScores
// cases: each document carries a tokenized "field" with value
// "this is document <i>". The whitespace analyzer used by the integration
// harness yields the tokens this/is/document/<i>, so the term "this" matches all
// five documents and the term "3" matches exactly the i==3 document.
func needsScoresIndex(t *testing.T) (*search.IndexSearcher, func()) {
	t.Helper()
	ix := newIntegrationIndex(t)
	for i := 0; i < 5; i++ {
		ix.addText("field", fmt.Sprintf("this is document %d", i))
	}
	return ix.searcher()
}

// TestNeedsScores ports org.apache.lucene.search.TestNeedsScores. Each subtest
// wraps a query in AssertNeedsScores and confirms both the hit count and the
// ScoreMode observed by the wrapped query.
func TestNeedsScores(t *testing.T) {
	t.Run("ProhibitedClause", func(t *testing.T) {
		// Prohibited clauses in a BooleanQuery don't need scoring: a MUST clause
		// is scoring and observes the collector mode (COMPLETE for Gocene's
		// TopDocsCollector), while a MUST_NOT clause is non-scoring and must
		// observe COMPLETE_NO_SCORES.
		searcher, cleanup := needsScoresIndex(t)
		defer cleanup()

		required := search.NewTermQuery(index.NewTerm("field", "this"))
		prohibited := search.NewTermQuery(index.NewTerm("field", "3"))

		requiredAssert := newAssertNeedsScores(t, required, search.COMPLETE)
		prohibitedAssert := newAssertNeedsScores(t, prohibited, search.COMPLETE_NO_SCORES)

		bq := search.NewBooleanQuery()
		bq.Add(requiredAssert, search.MUST)
		bq.Add(prohibitedAssert, search.MUST_NOT)

		top, err := searcher.Search(bq, 5)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if got := top.TotalHits.Value; got != 4 { // we exclude document 3
			t.Errorf("totalHits = %d, want 4", got)
		}
		requiredAssert.requireObserved(t)
		prohibitedAssert.requireObserved(t)
	})

	t.Run("ConstantScoreQuery", func(t *testing.T) {
		// Nested inside a ConstantScoreQuery: with Gocene's exhaustive collectors
		// the wrapped query observes COMPLETE_NO_SCORES, whether reached via a
		// counting collector or a top-N (COMPLETE) search.
		searcher, cleanup := needsScoresIndex(t)
		defer cleanup()

		// Counting collector path (COMPLETE_NO_SCORES, exhaustive).
		term := search.NewTermQuery(index.NewTerm("field", "this"))
		countAssert := newAssertNeedsScores(t, term, search.COMPLETE_NO_SCORES)
		countCSQ := search.NewConstantScoreQuery(countAssert)

		counter := search.NewTotalHitCountCollector()
		if err := searcher.SearchWithCollector(countCSQ, counter); err != nil {
			t.Fatalf("SearchWithCollector(count): %v", err)
		}
		if got := counter.GetTotalHits(); got != 5 {
			t.Errorf("count = %d, want 5", got)
		}
		countAssert.requireObserved(t)

		// Top-N search path (COMPLETE collector, exhaustive -> inner
		// COMPLETE_NO_SCORES).
		term2 := search.NewTermQuery(index.NewTerm("field", "this"))
		topAssert := newAssertNeedsScores(t, term2, search.COMPLETE_NO_SCORES)
		topCSQ := search.NewConstantScoreQuery(topAssert)

		top, err := searcher.Search(topCSQ, 5)
		if err != nil {
			t.Fatalf("Search(constantScore): %v", err)
		}
		if got := top.TotalHits.Value; got != 5 {
			t.Errorf("totalHits = %d, want 5", got)
		}
		topAssert.requireObserved(t)

		// Sort-by-field path: the field-sort collector is COMPLETE_NO_SCORES
		// (exhaustive), so the inner query still observes COMPLETE_NO_SCORES.
		term3 := search.NewTermQuery(index.NewTerm("field", "this"))
		sortAssert := newAssertNeedsScores(t, term3, search.COMPLETE_NO_SCORES)
		sortCSQ := search.NewConstantScoreQuery(sortAssert)

		sortTop, err := searcher.SearchWithSort(sortCSQ, 5, search.NewSortByDoc())
		if err != nil {
			t.Fatalf("SearchWithSort(constantScore): %v", err)
		}
		if got := sortTop.TotalHits.Value; got != 5 {
			t.Errorf("sorted totalHits = %d, want 5", got)
		}
		sortAssert.requireObserved(t)
	})

	t.Run("SortByField", func(t *testing.T) {
		// When not sorting by score, the field-sort collector is
		// COMPLETE_NO_SCORES and the (unwrapped) query observes it directly.
		searcher, cleanup := needsScoresIndex(t)
		defer cleanup()

		assertQ := newAssertNeedsScores(t, search.NewMatchAllDocsQuery(), search.COMPLETE_NO_SCORES)
		top, err := searcher.SearchWithSort(assertQ, 5, search.NewSortByDoc())
		if err != nil {
			t.Fatalf("SearchWithSort: %v", err)
		}
		if got := top.TotalHits.Value; got != 5 {
			t.Errorf("totalHits = %d, want 5", got)
		}
		assertQ.requireObserved(t)
	})

	t.Run("SortByScore", func(t *testing.T) {
		// When sorting by score, the field-sort collector is COMPLETE and the
		// query observes a score-bearing mode.
		searcher, cleanup := needsScoresIndex(t)
		defer cleanup()

		assertQ := newAssertNeedsScores(t, search.NewMatchAllDocsQuery(), search.COMPLETE)
		top, err := searcher.SearchWithSort(assertQ, 5, search.NewSortByScore())
		if err != nil {
			t.Fatalf("SearchWithSort: %v", err)
		}
		if got := top.TotalHits.Value; got != 5 {
			t.Errorf("totalHits = %d, want 5", got)
		}
		assertQ.requireObserved(t)
	})
}

// assertNeedsScores wraps a query and asserts that the ScoreMode passed to its
// inner query's weight creation equals value, mirroring the upstream
// AssertNeedsScores test helper. It implements search.Query plus the optional
// ScoreMode-aware CreateWeightScoreMode so IndexSearcher.CreateWeight dispatches
// to it and forwards the full ScoreMode.
type assertNeedsScores struct {
	t        *testing.T
	in       search.Query
	value    search.ScoreMode
	observed bool
}

func newAssertNeedsScores(t *testing.T, in search.Query, value search.ScoreMode) *assertNeedsScores {
	if in == nil {
		t.Fatal("assertNeedsScores: inner query must not be nil")
	}
	return &assertNeedsScores{t: t, in: in, value: value}
}

// requireObserved fails the test if no Scorer was ever pulled, guaranteeing the
// assertion in the wrapped supplier actually ran (a silent no-op must fail, not
// pass).
func (q *assertNeedsScores) requireObserved(t *testing.T) {
	t.Helper()
	if !q.observed {
		t.Errorf("query=%v: no scorer was pulled, ScoreMode assertion never ran", q.in)
	}

// CreateWeightScoreMode builds the inner weight under scoreMode (via the
// searcher's dispatch, so composite inner queries also see it) and wraps it so
// that pulling a Scorer asserts scoreMode == q.value.
func (q *assertNeedsScores) CreateWeightScoreMode(searcher *search.IndexSearcher, scoreMode search.ScoreMode, boost float32) (search.Weight, error) {
	inner, err := searcher.CreateWeight(q.in, scoreMode, boost)
	if err != nil {
		return nil, err
	}
	if inner == nil {
		return nil, nil
	}
	return &assertNeedsScoresWeight{Weight: inner, parent: q, scoreMode: scoreMode}, nil
}

// CreateWeight is the bool-based entry point required by search.Query; it maps
// the bool to the coarsest ScoreMode and delegates to CreateWeightScoreMode.
// IndexSearcher always reaches this wrapper through CreateWeightScoreMode, so on
// the real search path the full ScoreMode is preserved.
func (q *assertNeedsScores) CreateWeight(searcher *search.IndexSearcher, needsScores bool, boost float32) (search.Weight, error) {
	mode := search.COMPLETE_NO_SCORES
	if needsScores {
		mode = search.COMPLETE
	}
	return q.CreateWeightScoreMode(searcher, mode, boost)
}

func (q *assertNeedsScores) Rewrite(reader search.IndexReader) (search.Query, error) {
	in2, err := q.in.Rewrite(reader)
	if err != nil {
		return nil, err
	}
	if in2 == q.in {
		return q, nil
	}
	return &assertNeedsScores{t: q.t, in: in2, value: q.value}, nil
}

func (q *assertNeedsScores) Clone() search.Query {
	return &assertNeedsScores{t: q.t, in: q.in.Clone(), value: q.value}
}

func (q *assertNeedsScores) Equals(other search.Query) bool {
	o, ok := other.(*assertNeedsScores)
	if !ok {
		return false
	}
	return q.value == o.value && q.in.Equals(o.in)
}

func (q *assertNeedsScores) HashCode() int {
	const prime = 31
	result := 1
	result = prime*result + q.in.HashCode()
	result = prime*result + int(q.value)
	return result
}

var _ search.Query = (*assertNeedsScores)(nil)

// assertNeedsScoresWeight embeds the inner Weight (so every Weight method
// delegates by default) and overrides ScorerSupplier so that pulling a Scorer
// asserts the recorded ScoreMode.
type assertNeedsScoresWeight struct {
	search.Weight
	parent    *assertNeedsScores
	scoreMode search.ScoreMode
}

func (w *assertNeedsScoresWeight) ScorerSupplier(ctx *index.LeafReaderContext) (search.ScorerSupplier, error) {
	inner, err := w.Weight.ScorerSupplier(ctx)
	if err != nil {
		return nil, err
	}
	if inner == nil {
		return nil, nil
	}
	return &assertNeedsScoresSupplier{ScorerSupplier: inner, weight: w}, nil
}

// Scorer mirrors BaseWeight.Scorer so the assertion also fires when callers pull
// a Scorer directly rather than through the supplier.
func (w *assertNeedsScoresWeight) Scorer(ctx *index.LeafReaderContext) (search.Scorer, error) {
	supplier, err := w.ScorerSupplier(ctx)
	if err != nil {
		return nil, err
	}
	if supplier == nil {
		return nil, nil
	}
	return supplier.Get(0)
}

// BulkScorer mirrors BaseWeight.BulkScorer over the asserting Scorer so the
// assertion fires on the bulk-scoring path used by the search loop.
func (w *assertNeedsScoresWeight) BulkScorer(ctx *index.LeafReaderContext) (search.BulkScorer, error) {
	scorer, err := w.Scorer(ctx)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return search.NewDefaultBulkScorer(scorer), nil
}

var _ search.Weight = (*assertNeedsScoresWeight)(nil)

// assertNeedsScoresSupplier asserts the recorded ScoreMode each time a Scorer is
// pulled, matching the upstream AssertNeedsScores supplier whose get() asserts
// the expected scoreMode.
type assertNeedsScoresSupplier struct {
	search.ScorerSupplier
	weight *assertNeedsScoresWeight
}

func (s *assertNeedsScoresSupplier) Get(leadCost int64) (search.Scorer, error) {
	p := s.weight.parent
	p.observed = true
	if s.weight.scoreMode != p.value {
		p.t.Errorf("query=%v: ScoreMode = %v, want %v", p.in, s.weight.scoreMode, p.value)
	}
	return s.ScorerSupplier.Get(leadCost)
}