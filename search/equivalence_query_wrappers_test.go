// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Go ports of the test-framework query wrappers used by the search-equivalence
// and block-max suites of Apache Lucene 10.4.0:
//   lucene/test-framework/src/java/org/apache/lucene/tests/search/RandomApproximationQuery.java
//   lucene/test-framework/src/java/org/apache/lucene/tests/search/AssertingQuery.java
//
// RandomApproximationQuery wraps a query so that its scorer iterates through a
// two-phase view that introduces false positives (doc ids that precede the next
// real match) which are then rejected by TwoPhaseIterator.matches(). Because the
// two-phase view ultimately yields exactly the same matches and scores as the
// wrapped query, a RandomApproximationQuery is result- and score-equivalent to
// its inner query — which is the property the equivalence tests rely on, while
// still exercising the two-phase/approximation machinery.
//
// assertingQuery is a faithful-but-minimal port of AssertingQuery: it is an
// opaque wrapper that delegates weight creation to its inner query but reports a
// distinct equals/hashCode, which is exactly what TestSimpleSearchEquivalence
// needs to keep BoostQuery from merging an inner and outer boost.

package search_test

import (
	"math/rand"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// ── RandomApproximationQuery ────────────────────────────────────────────────

type randomApproximationQuery struct {
	*search.BaseQuery
	query search.Query
	rng   *rand.Rand
}

func newRandomApproximationQuery(query search.Query, rng *rand.Rand) *randomApproximationQuery {
	return &randomApproximationQuery{
		BaseQuery: &search.BaseQuery{},
		query:     query,
		rng:       rng,
	}
}

func (q *randomApproximationQuery) Rewrite(reader search.IndexReader) (search.Query, error) {
	rewritten, err := q.query.Rewrite(reader)
	if err != nil {
		return nil, err
	}
	if rewritten != q.query {
		return newRandomApproximationQuery(rewritten, q.rng), nil
	}
	return q, nil
}

func (q *randomApproximationQuery) Clone() search.Query {
	return newRandomApproximationQuery(q.query.Clone(), q.rng)
}

func (q *randomApproximationQuery) Equals(other search.Query) bool {
	o, ok := other.(*randomApproximationQuery)
	return ok && q.query.Equals(o.query)
}

func (q *randomApproximationQuery) HashCode() int {
	return 31*0x52414e44 + q.query.HashCode()
}

func (q *randomApproximationQuery) CreateWeight(searcher *search.IndexSearcher, needsScores bool, boost float32) (search.Weight, error) {
	mode := search.COMPLETE_NO_SCORES
	if needsScores {
		mode = search.COMPLETE
	}
	return q.CreateWeightScoreMode(searcher, mode, boost)
}

func (q *randomApproximationQuery) CreateWeightScoreMode(searcher *search.IndexSearcher, scoreMode search.ScoreMode, boost float32) (search.Weight, error) {
	inner, err := searcher.CreateWeight(q.query, scoreMode, boost)
	if err != nil {
		return nil, err
	}
	if inner == nil {
		return nil, nil
	}
	return &randomApproximationWeight{Weight: inner, rng: rand.New(rand.NewSource(q.rng.Int63()))}, nil //nolint:gosec // deterministic test seed
}

// randomApproximationWeight delegates everything to the inner weight but wraps
// the scorer with a two-phase approximation view.
type randomApproximationWeight struct {
	search.Weight
	rng *rand.Rand
}

func (w *randomApproximationWeight) Scorer(ctx *index.LeafReaderContext) (search.Scorer, error) {
	inner, err := w.Weight.Scorer(ctx)
	if err != nil || inner == nil {
		return nil, err
	}
	return newRandomApproximationScorer(inner, rand.New(rand.NewSource(w.rng.Int63()))), nil //nolint:gosec // deterministic test seed
}

func (w *randomApproximationWeight) ScorerSupplier(ctx *index.LeafReaderContext) (search.ScorerSupplier, error) {
	scorer, err := w.Scorer(ctx)
	if err != nil || scorer == nil {
		return nil, err
	}
	return &fixedScorerSupplier{scorer: scorer}, nil
}

func (w *randomApproximationWeight) BulkScorer(ctx *index.LeafReaderContext) (search.BulkScorer, error) {
	scorer, err := w.Scorer(ctx)
	if err != nil || scorer == nil {
		return nil, err
	}
	return search.NewDefaultBulkScorer(scorer), nil
}

func (w *randomApproximationWeight) Count(_ *index.LeafReaderContext) (int, error) { return -1, nil }

// fixedScorerSupplier returns an already-built scorer.
type fixedScorerSupplier struct {
	scorer search.Scorer
}

func (s *fixedScorerSupplier) Get(_ int64) (search.Scorer, error) { return s.scorer, nil }
func (s *fixedScorerSupplier) Cost() int64                        { return s.scorer.Cost() }
func (s *fixedScorerSupplier) SetTopLevelScoringClause()          {}
func (s *fixedScorerSupplier) BulkScorer() (search.BulkScorer, error) {
	return search.NewDefaultBulkScorer(s.scorer), nil
}

// randomApproximationScorer wraps the inner scorer behind a two-phase iterator
// that admits false positives, then verifies them via matches().
type randomApproximationScorer struct {
	search.Scorer
	approx   *randomApproximation
	twoPhase *search.TwoPhaseIterator
	disi     search.DocIdSetIterator
	lastDoc  int
}

func newRandomApproximationScorer(inner search.Scorer, rng *rand.Rand) *randomApproximationScorer {
	s := &randomApproximationScorer{Scorer: inner, lastDoc: -1}
	s.approx = &randomApproximation{rng: rng, disi: inner, doc: -1}
	s.twoPhase = search.NewTwoPhaseIteratorWithMatchCost(s.approx, s.matches, rng.Float32()*200)
	s.disi = search.NewTwoPhaseIteratorAsDocIdSetIterator(s.twoPhase)
	return s
}

// matches verifies a candidate produced by the approximation: it is a real match
// exactly when the approximation lands on the inner scorer's current doc.
func (s *randomApproximationScorer) matches() (bool, error) {
	cur := s.approx.DocID()
	if cur == -1 || cur == search.NO_MORE_DOCS {
		return false, nil
	}
	s.lastDoc = cur
	return cur == s.Scorer.DocID(), nil
}

func (s *randomApproximationScorer) TwoPhaseIterator() *search.TwoPhaseIterator { return s.twoPhase }
func (s *randomApproximationScorer) DocID() int                                 { return s.approx.DocID() }
func (s *randomApproximationScorer) NextDoc() (int, error)                      { return s.disi.NextDoc() }
func (s *randomApproximationScorer) Advance(target int) (int, error)            { return s.disi.Advance(target) }
func (s *randomApproximationScorer) Cost() int64                                { return s.disi.Cost() }
func (s *randomApproximationScorer) DocIDRunEnd() int                           { return s.disi.DocIDRunEnd() }
func (s *randomApproximationScorer) Score() float32                             { return s.Scorer.Score() }
func (s *randomApproximationScorer) GetMaxScore(upTo int) float32               { return s.Scorer.GetMaxScore(upTo) }
func (s *randomApproximationScorer) AdvanceShallow(target int) (int, error) {
	if s.Scorer.DocID() > target {
		target = s.Scorer.DocID()
	}
	return s.Scorer.AdvanceShallow(target)
}

// randomApproximation drives the inner scorer's iterator and returns doc ids that
// are a (random) lower bound of the next real match, mirroring the upstream
// RandomApproximation.
type randomApproximation struct {
	rng  *rand.Rand
	disi search.DocIdSetIterator
	doc  int
}

func (a *randomApproximation) DocID() int { return a.doc }

func (a *randomApproximation) NextDoc() (int, error) { return a.Advance(a.doc + 1) }

func (a *randomApproximation) Advance(target int) (int, error) {
	if a.disi.DocID() < target {
		if _, err := a.disi.Advance(target); err != nil {
			return search.NO_MORE_DOCS, err
		}
	if a.disi.DocID() == search.NO_MORE_DOCS {
		a.doc = search.NO_MORE_DOCS
		return a.doc, nil
	}
	// Return a random doc in [target, disi.docID()] — a false positive unless it
	// equals disi.docID().
	span := a.disi.DocID() - target
	if span <= 0 {
		a.doc = a.disi.DocID()
	} else {
		a.doc = target + a.rng.Intn(span+1)
	}
	return a.doc, nil
}

}
func (a *randomApproximation) Cost() int64      { return a.disi.Cost() }
func (a *randomApproximation) DocIDRunEnd() int { return a.doc + 1 }

// ── AssertingQuery (minimal delegating wrapper) ─────────────────────────────

type assertingQuery struct {
	*search.BaseQuery
	in search.Query
}

func newAssertingQuery(in search.Query) *assertingQuery {
	return &assertingQuery{BaseQuery: &search.BaseQuery{}, in: in}
}

func (q *assertingQuery) Rewrite(reader search.IndexReader) (search.Query, error) {
	rewritten, err := q.in.Rewrite(reader)
	if err != nil {
		return nil, err
	}
	if rewritten == q.in {
		return q, nil
	}
	return newAssertingQuery(rewritten), nil
}

func (q *assertingQuery) Clone() search.Query { return newAssertingQuery(q.in.Clone()) }

func (q *assertingQuery) Equals(other search.Query) bool {
	o, ok := other.(*assertingQuery)
	return ok && q.in.Equals(o.in)
}

// HashCode is deliberately distinct from the inner query's so that BoostQuery
// does not recognise it as the same query and merge boosts.
func (q *assertingQuery) HashCode() int { return -q.in.HashCode() }

func (q *assertingQuery) CreateWeight(searcher *search.IndexSearcher, needsScores bool, boost float32) (search.Weight, error) {
	return q.in.CreateWeight(searcher, needsScores, boost)
}

func (q *assertingQuery) CreateWeightScoreMode(searcher *search.IndexSearcher, scoreMode search.ScoreMode, boost float32) (search.Weight, error) {
	return searcher.CreateWeight(q.in, scoreMode, boost)
}