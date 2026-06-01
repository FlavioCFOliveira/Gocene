// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package function

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// FunctionQuery returns a score for each document based on a [ValueSource].
// It is the Go port of org.apache.lucene.queries.function.FunctionQuery.
//
// Gocene deviation: Lucene caches the per-search Context inside a generated
// FunctionWeight inner class; here the equivalent state lives in
// [functionWeight] which embeds search.BaseWeight to satisfy the
// search.Weight contract.
type FunctionQuery struct {
	function ValueSource
}

// NewFunctionQuery returns a FunctionQuery scoring documents via the
// supplied ValueSource.
func NewFunctionQuery(function ValueSource) *FunctionQuery {
	return &FunctionQuery{function: function}
}

// GetValueSource returns the wrapped ValueSource.
func (q *FunctionQuery) GetValueSource() ValueSource { return q.function }

// Rewrite returns the query itself; FunctionQuery does not simplify further.
func (q *FunctionQuery) Rewrite(_ search.IndexReader) (search.Query, error) { return q, nil }

// Clone returns a defensive copy of the query.
func (q *FunctionQuery) Clone() search.Query { return &FunctionQuery{function: q.function} }

// Equals checks structural equality with another query.
func (q *FunctionQuery) Equals(other search.Query) bool {
	o, ok := other.(*FunctionQuery)
	if !ok || o == nil {
		return false
	}
	return q.function.Equals(o.function)
}

// HashCode returns a deterministic hash combining the class identity and
// the wrapped ValueSource hash.
func (q *FunctionQuery) HashCode() int {
	// classHash() ^ func.hashCode() in Lucene.
	h := int(q.function.HashCode())
	return h ^ 0x46_75_6e_51 // "FunQ" magic constant for parity within Gocene.
}

// CreateWeight returns a search.Weight that scores documents via the
// wrapped ValueSource.
func (q *FunctionQuery) CreateWeight(searcher *search.IndexSearcher, needsScores bool, boost float32) (search.Weight, error) {
	w := &functionWeight{
		BaseWeight: search.NewBaseWeight(q),
		query:      q,
		searcher:   searcher,
		boost:      boost,
		ctx:        NewContext(),
	}
	w.ctx.Put(SearcherKey, searcher)
	if err := q.function.CreateWeight(w.ctx, searcher); err != nil {
		return nil, err
	}
	return w, nil
}

// String renders a stable description used in toString() output.
func (q *FunctionQuery) String() string { return q.function.Description() }

// Visit implements the QueryVisitor pattern; FunctionQuery is a leaf.
func (q *FunctionQuery) Visit(visitor search.QueryVisitor) { visitor.VisitLeaf(q) }

// functionWeight is the search.Weight implementation backing FunctionQuery.
type functionWeight struct {
	*search.BaseWeight
	query    *FunctionQuery
	searcher *search.IndexSearcher
	boost    float32
	ctx      Context
}

// IsCacheable reports whether this weight can be cached. FunctionQuery
// declares itself non-cacheable to mirror Lucene's default.
func (w *functionWeight) IsCacheable(_ *index.LeafReaderContext) bool { return false }

// Scorer returns a FunctionAllScorer for the supplied leaf.
func (w *functionWeight) Scorer(ctx *index.LeafReaderContext) (search.Scorer, error) {
	vals, err := w.query.function.GetValues(w.ctx, ctx)
	if err != nil {
		return nil, err
	}
	return newFunctionAllScorer(w, ctx, vals, w.boost), nil
}

// ScorerSupplier wraps Scorer in a default supplier.
func (w *functionWeight) ScorerSupplier(ctx *index.LeafReaderContext) (search.ScorerSupplier, error) {
	scorer, err := w.Scorer(ctx)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return search.NewScorerSupplierAdapter(scorer), nil
}

// BulkScorer delegates to the default per-doc bulk scorer.
func (w *functionWeight) BulkScorer(ctx *index.LeafReaderContext) (search.BulkScorer, error) {
	scorer, err := w.Scorer(ctx)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return search.NewDefaultBulkScorer(scorer), nil
}

// Explain delegates to the underlying FunctionValues and wraps the result
// with the boost detail, following Lucene's AllScorer.explain.
func (w *functionWeight) Explain(ctx *index.LeafReaderContext, doc int) (search.Explanation, error) {
	vals, err := w.query.function.GetValues(w.ctx, ctx)
	if err != nil {
		return nil, err
	}
	val, err := vals.FloatVal(doc)
	if err != nil {
		return nil, err
	}
	desc, err := vals.ToString(doc)
	if err != nil {
		return nil, err
	}
	score := w.boost * normaliseScore(val)
	root := search.NewExplanation(true, score,
		fmt.Sprintf("FunctionQuery(%s), product of:", w.query.function.Description()))
	root.AddDetail(search.NewExplanation(true, val, desc))
	root.AddDetail(search.NewExplanation(true, w.boost, "boost"))
	return root, nil
}

// Matches returns nil to mirror Lucene's lack of match-info support.
func (w *functionWeight) Matches(_ *index.LeafReaderContext, _ int) (search.Matches, error) {
	return nil, nil
}

// Count is unknown for arbitrary ValueSources; returning -1 matches Lucene.
func (w *functionWeight) Count(_ *index.LeafReaderContext) (int, error) { return -1, nil }

// normaliseScore reproduces FunctionQuery.AllScorer.score(): NaN and
// negative values collapse to 0 (the test `val >= 0 == false` is true for
// any negative or NaN value because comparisons with NaN return false).
func normaliseScore(v float32) float32 {
	if !(v >= 0) {
		return 0
	}
	return v
}

// functionAllScorer iterates over every doc in the leaf, scoring by
// FunctionValues.FloatVal times the boost.
type functionAllScorer struct {
	weight *functionWeight
	leaf   *index.LeafReaderContext
	iter   search.DocIdSetIterator
	vals   FunctionValues
	boost  float32
	maxDoc int
}

func newFunctionAllScorer(w *functionWeight, leaf *index.LeafReaderContext, vals FunctionValues, boost float32) *functionAllScorer {
	maxDoc := 0
	if leaf != nil {
		if lr := leaf.LeafReader(); lr != nil {
			maxDoc = lr.MaxDoc()
		}
	}
	return &functionAllScorer{
		weight: w,
		leaf:   leaf,
		iter:   search.NewRangeDocIdSetIterator(0, maxDoc),
		vals:   vals,
		boost:  boost,
		maxDoc: maxDoc,
	}
}

func (s *functionAllScorer) DocID() int                      { return s.iter.DocID() }
func (s *functionAllScorer) NextDoc() (int, error)           { return s.iter.NextDoc() }
func (s *functionAllScorer) Advance(target int) (int, error) { return s.iter.Advance(target) }
func (s *functionAllScorer) Cost() int64                     { return s.iter.Cost() }
func (s *functionAllScorer) DocIDRunEnd() int                { return s.iter.DocIDRunEnd() }
func (s *functionAllScorer) GetMaxScore(_ int) float32       { return float32(math.Inf(1)) }

// AdvanceShallow returns search.NO_MORE_DOCS, the default defined by
// org.apache.lucene.search.Scorer#advanceShallow. This scorer does not expose
// per-block impact information.
func (s *functionAllScorer) AdvanceShallow(target int) (int, error) {
	return search.NO_MORE_DOCS, nil
}

// Score returns boost * floatVal(docID), with negatives/NaN collapsed to 0.
func (s *functionAllScorer) Score() float32 {
	val, err := s.vals.FloatVal(s.iter.DocID())
	if err != nil {
		return 0
	}
	return s.boost * normaliseScore(val)
}

// Ensure interface conformance.
var (
	_ search.Query  = (*FunctionQuery)(nil)
	_ search.Weight = (*functionWeight)(nil)
	_ search.Scorer = (*functionAllScorer)(nil)
)
