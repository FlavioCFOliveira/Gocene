// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"math"
	"math/rand"
	"strconv"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// RandomApproximationQuery is the Go port of
// org.apache.lucene.tests.search.RandomApproximationQuery (Apache Lucene
// 10.5.0): a Query that adds random approximations to its scorers.
type RandomApproximationQuery struct {
	search.BaseQuery
	query  search.Query
	random *rand.Rand
}

// NewRandomApproximationQuery renders RandomApproximationQuery(Query, Random).
func NewRandomApproximationQuery(query search.Query, random *rand.Rand) *RandomApproximationQuery {
	return &RandomApproximationQuery{query: query, random: random}
}

// Rewrite renders rewrite(IndexSearcher).
func (q *RandomApproximationQuery) Rewrite(indexSearcher *search.IndexSearcher) (search.Query, error) {
	rewritten, err := q.query.Rewrite(indexSearcher)
	if err != nil {
		return nil, err
	}
	if rewritten != q.query {
		return NewRandomApproximationQuery(rewritten, q.random), nil
	}
	return q, nil
}

// Visit renders visit(QueryVisitor).
func (q *RandomApproximationQuery) Visit(visitor search.QueryVisitor) {
	q.query.Visit(visitor)
}

// Equals renders equals(Object).
func (q *RandomApproximationQuery) Equals(other spi.Query) bool {
	o, ok := other.(*RandomApproximationQuery)
	return ok && q.query.Equals(o.query)
}

// HashCode renders hashCode(): 31 * classHash() + query.hashCode().
func (q *RandomApproximationQuery) HashCode() int {
	return int(31*javaStringHash("org.apache.lucene.tests.search.RandomApproximationQuery") + int32(q.query.HashCode()))
}

// ToString renders toString(String field).
func (q *RandomApproximationQuery) ToString(field string) string {
	return QueryString(q.query, field)
}

func (q *RandomApproximationQuery) String() string { return q.ToString("") }

// CreateWeight renders createWeight(IndexSearcher, ScoreMode, float).
func (q *RandomApproximationQuery) CreateWeight(searcher *search.IndexSearcher, scoreMode search.ScoreMode, boost float32) (search.Weight, error) {
	weight, err := q.query.CreateWeight(searcher, scoreMode, boost)
	if err != nil {
		return nil, err
	}
	return newRandomApproximationWeight(weight, rand.New(rand.NewSource(q.random.Int63()))), nil
}

// javaStringHash renders String.hashCode().
func javaStringHash(s string) int32 {
	var h int32
	for _, c := range utf16Units(s) {
		h = 31*h + int32(c)
	}
	return h
}

func utf16Units(s string) []uint16 {
	var out []uint16
	for _, r := range s {
		if r >= 0x10000 {
			r -= 0x10000
			out = append(out, uint16(0xD800+(r>>10)), uint16(0xDC00+(r&0x3FF)))
		} else {
			out = append(out, uint16(r))
		}
	}
	return out
}

// randomApproximationWeight renders the private static class
// RandomApproximationWeight (a FilterWeight).
type randomApproximationWeight struct {
	*search.FilterWeight
	in     search.Weight
	random *rand.Rand
}

func newRandomApproximationWeight(weight search.Weight, random *rand.Rand) *randomApproximationWeight {
	return &randomApproximationWeight{FilterWeight: search.NewFilterWeight(weight), in: weight, random: random}
}

// ScorerSupplier renders scorerSupplier(LeafReaderContext).
func (w *randomApproximationWeight) ScorerSupplier(context *index.LeafReaderContext) (search.ScorerSupplier, error) {
	scorerSupplier, err := w.in.ScorerSupplier(context)
	if err != nil {
		return nil, err
	}
	if scorerSupplier == nil {
		return nil, nil
	}
	subScorer, err := scorerSupplier.Get(math.MaxInt64)
	if err != nil {
		return nil, err
	}
	scorer := newRandomApproximationScorer(subScorer, rand.New(rand.NewSource(w.random.Int63())))
	return search.NewDefaultScorerSupplier(scorer), nil
}

// Scorer renders the inherited Weight.scorer, which dispatches to the
// scorerSupplier override.
func (w *randomApproximationWeight) Scorer(context *index.LeafReaderContext) (search.Scorer, error) {
	ss, err := w.ScorerSupplier(context)
	if err != nil || ss == nil {
		return nil, err
	}
	return ss.Get(math.MaxInt64)
}

// BulkScorer renders the inherited Weight.bulkScorer, which dispatches to the
// scorerSupplier override.
func (w *randomApproximationWeight) BulkScorer(context *index.LeafReaderContext) (search.BulkScorer, error) {
	ss, err := w.ScorerSupplier(context)
	if err != nil || ss == nil {
		return nil, err
	}
	if err := ss.SetTopLevelScoringClause(); err != nil {
		return nil, err
	}
	return ss.BulkScorer()
}

// Count renders count(LeafReaderContext): -1.
func (w *randomApproximationWeight) Count(context *index.LeafReaderContext) (int, error) {
	return -1, nil
}

// randomApproximationScorer renders the private static class
// RandomApproximationScorer.
type randomApproximationScorer struct {
	search.BaseScorer
	scorer       search.Scorer
	twoPhaseView *RandomTwoPhaseView
	tpi          *search.TwoPhaseIterator
}

func newRandomApproximationScorer(scorer search.Scorer, random *rand.Rand) *randomApproximationScorer {
	s := &randomApproximationScorer{scorer: scorer}
	s.twoPhaseView = NewRandomTwoPhaseView(random, scorer.Iterator())
	s.tpi = s.twoPhaseView.TwoPhaseIterator
	return s
}

func (s *randomApproximationScorer) TwoPhaseIterator() *search.TwoPhaseIterator { return s.tpi }

func (s *randomApproximationScorer) Score() (float32, error) { return s.scorer.Score() }

func (s *randomApproximationScorer) AdvanceShallow(target int) (int, error) {
	if s.scorer.DocID() > target && s.twoPhaseView.Approximation().DocID() != s.scorer.DocID() {
		// The random approximation can return doc ids that are not present in the underlying
		// scorer. These additional doc ids are always *before* the next matching doc so we
		// cannot use them to shallow advance the main scorer which is already ahead.
		target = s.scorer.DocID()
	}
	return s.scorer.AdvanceShallow(target)
}

func (s *randomApproximationScorer) GetMaxScore(upTo int) (float32, error) {
	return s.scorer.GetMaxScore(upTo)
}

func (s *randomApproximationScorer) DocID() int { return s.twoPhaseView.Approximation().DocID() }

func (s *randomApproximationScorer) Iterator() search.DocIdSetIterator {
	return search.AsDocIdSetIterator(s.tpi)
}

func (s *randomApproximationScorer) NextDocsAndScores(upTo int, liveDocs util.Bits, buffer *search.DocAndFloatFeatureBuffer) error {
	return search.DefaultNextDocsAndScores(s, upTo, liveDocs, buffer)
}

// RandomTwoPhaseView renders the public static class
// RandomApproximationQuery.RandomTwoPhaseView: a wrapper around a
// DocIdSetIterator that matches the same documents, but introduces false
// positives that need to be verified via TwoPhaseIterator.matches().
type RandomTwoPhaseView struct {
	*search.TwoPhaseIterator
	disi            search.DocIdSetIterator
	approximation   search.DocIdSetIterator
	lastDoc         int
	randomMatchCost float32
}

// NewRandomTwoPhaseView renders RandomTwoPhaseView(Random, DocIdSetIterator).
func NewRandomTwoPhaseView(random *rand.Rand, disi search.DocIdSetIterator) *RandomTwoPhaseView {
	v := &RandomTwoPhaseView{disi: disi, lastDoc: -1}
	v.approximation = newRandomApproximation(random, disi)
	v.TwoPhaseIterator = search.NewTwoPhaseIterator(v.approximation, v)
	v.randomMatchCost = random.Float32() * 200 // between 0 and 200
	return v
}

// Matches renders matches().
func (v *RandomTwoPhaseView) Matches() (bool, error) {
	doc := v.approximation.DocID()
	if doc == -1 || doc == search.NO_MORE_DOCS {
		panic(util.NewAssertionError("matches() should not be called on doc ID " + strconv.Itoa(doc)))
	}
	if v.lastDoc == doc {
		panic(util.NewAssertionError("matches() has been called twice on doc ID " + strconv.Itoa(doc)))
	}
	v.lastDoc = doc
	return doc == v.disi.DocID(), nil
}

// MatchCost renders matchCost().
func (v *RandomTwoPhaseView) MatchCost() float32 { return v.randomMatchCost }

// DocIDRunEnd renders the docIDRunEnd() override.
func (v *RandomTwoPhaseView) DocIDRunEnd() (int, error) {
	if v.approximation.DocID() == v.disi.DocID() {
		return v.disi.DocIDRunEnd()
	}
	return v.approximation.DocID(), nil // super.docIDRunEnd()
}

// randomApproximation renders the private static class RandomApproximation.
type randomApproximation struct {
	search.AbstractDocIdSetIterator
	random *rand.Rand
	disi   search.DocIdSetIterator
}

func newRandomApproximation(random *rand.Rand, disi search.DocIdSetIterator) *randomApproximation {
	return &randomApproximation{AbstractDocIdSetIterator: search.AbstractDocIdSetIterator{Doc: -1}, random: random, disi: disi}
}

func (a *randomApproximation) NextDoc() (int, error) { return a.Advance(a.Doc + 1) }

func (a *randomApproximation) Advance(target int) (int, error) {
	if a.disi.DocID() < target {
		if _, err := a.disi.Advance(target); err != nil {
			return 0, err
		}
	}
	if a.disi.DocID() == search.NO_MORE_DOCS {
		a.Doc = search.NO_MORE_DOCS
		return a.Doc, nil
	}
	// RandomNumbers.randomIntBetween(random, target, disi.docID())
	a.Doc = target + a.random.Intn(a.disi.DocID()-target+1)
	return a.Doc, nil
}

func (a *randomApproximation) Cost() int64 { return a.disi.Cost() }

func (a *randomApproximation) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(a, upTo, bitSet, offset)
}

func (a *randomApproximation) DocIDRunEnd() (int, error) { return util.DefaultDocIDRunEnd(a) }

var _ search.Scorer = (*randomApproximationScorer)(nil)
