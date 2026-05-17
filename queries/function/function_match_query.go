// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package function

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// DefaultMatchCost is the default per-iteration TwoPhaseIterator match
// cost used by [FunctionMatchQuery], mirroring Lucene's 100f.
const DefaultFunctionMatchCost float32 = 100

// DoublePredicate is the Go counterpart to java.util.function.DoublePredicate.
type DoublePredicate func(float64) bool

// FunctionMatchQuery retrieves every document whose [DoubleValuesSource]
// value satisfies a predicate. It linearly scans the index and is best
// composed with other clauses that restrict the document set.
//
// Go port of org.apache.lucene.queries.function.FunctionMatchQuery.
type FunctionMatchQuery struct {
	source    DoubleValuesSource
	filter    DoublePredicate
	matchCost float32 // not used in Equals/HashCode
	// filterKey is used by Equals/HashCode so two queries that share the
	// same logical filter compare equal; callers may set it to a stable
	// identifier when the closure itself does not (e.g. via NewFunctionMatchQueryKeyed).
	filterKey string
}

// NewFunctionMatchQuery builds a FunctionMatchQuery with the default match cost.
func NewFunctionMatchQuery(source DoubleValuesSource, filter DoublePredicate) *FunctionMatchQuery {
	return NewFunctionMatchQueryWithCost(source, filter, DefaultFunctionMatchCost)
}

// NewFunctionMatchQueryWithCost builds a FunctionMatchQuery with a
// caller-supplied per-iteration match cost.
func NewFunctionMatchQueryWithCost(source DoubleValuesSource, filter DoublePredicate, matchCost float32) *FunctionMatchQuery {
	return &FunctionMatchQuery{source: source, filter: filter, matchCost: matchCost}
}

// WithFilterKey returns a copy of q whose filterKey participates in
// equality/hashing, giving callers a stable identity for closure-typed
// predicates. The returned query is otherwise identical to the receiver.
func (q *FunctionMatchQuery) WithFilterKey(key string) *FunctionMatchQuery {
	cp := *q
	cp.filterKey = key
	return &cp
}

// Source returns the wrapped DoubleValuesSource.
func (q *FunctionMatchQuery) Source() DoubleValuesSource { return q.source }

// MatchCost returns the per-doc TwoPhaseIterator cost.
func (q *FunctionMatchQuery) MatchCost() float32 { return q.matchCost }

// String renders the canonical Lucene-style description.
func (q *FunctionMatchQuery) String() string {
	return fmt.Sprintf("FunctionMatchQuery(%s)", q.source.Description())
}

// Rewrite rewrites the underlying DoubleValuesSource.
func (q *FunctionMatchQuery) Rewrite(_ search.IndexReader) (search.Query, error) {
	return q, nil
}

// Clone returns a defensive copy.
func (q *FunctionMatchQuery) Clone() search.Query {
	cp := *q
	return &cp
}

// Equals reports value equality. Filters compare by filterKey when set;
// otherwise filter equality is impossible to determine from arbitrary
// closures and the comparison conservatively returns false (matching
// Java's reference-equality semantics for arbitrary DoublePredicate
// instances).
func (q *FunctionMatchQuery) Equals(other search.Query) bool {
	o, ok := other.(*FunctionMatchQuery)
	if !ok || o == nil {
		return false
	}
	if !q.source.Equals(o.source) {
		return false
	}
	if q.filterKey == "" && o.filterKey == "" {
		// Closure identity is opaque; default to reference equality.
		return fmt.Sprintf("%p", q.filter) == fmt.Sprintf("%p", o.filter)
	}
	return q.filterKey == o.filterKey
}

// HashCode mirrors Objects.hash(source, filter).
func (q *FunctionMatchQuery) HashCode() int {
	h := int32(q.source.HashCode())
	if q.filterKey != "" {
		h = combineHash(h, hashString(q.filterKey))
	} else {
		h = combineHash(h, hashString(fmt.Sprintf("%p", q.filter)))
	}
	return int(h)
}

// Visit implements the QueryVisitor contract; leaf query.
func (q *FunctionMatchQuery) Visit(visitor search.QueryVisitor) { visitor.VisitLeaf(q) }

// CreateWeight returns a constant-score Weight backed by the predicate.
func (q *FunctionMatchQuery) CreateWeight(searcher *search.IndexSearcher, _ bool, boost float32) (search.Weight, error) {
	rewritten, err := q.source.Rewrite(searcher)
	if err != nil {
		return nil, err
	}
	return &functionMatchWeight{
		BaseWeight: search.NewBaseWeight(q),
		query:      q,
		source:     rewritten,
		boost:      boost,
	}, nil
}

// functionMatchWeight is the constant-score Weight implementation for
// FunctionMatchQuery.
type functionMatchWeight struct {
	*search.BaseWeight
	query  *FunctionMatchQuery
	source DoubleValuesSource
	boost  float32
}

func (w *functionMatchWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return w.source.IsCacheable(ctx)
}

func (w *functionMatchWeight) Scorer(ctx *index.LeafReaderContext) (search.Scorer, error) {
	values, err := w.source.GetValues(ctx, nil)
	if err != nil {
		return nil, err
	}
	maxDoc := 0
	if leaf := ctx.LeafReader(); leaf != nil {
		maxDoc = leaf.MaxDoc()
	}
	return &functionMatchScorer{
		boost:  w.boost,
		iter:   search.NewRangeDocIdSetIterator(0, maxDoc),
		values: values,
		filter: w.query.filter,
		cost:   int64(maxDoc),
	}, nil
}

func (w *functionMatchWeight) ScorerSupplier(ctx *index.LeafReaderContext) (search.ScorerSupplier, error) {
	scorer, err := w.Scorer(ctx)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return search.NewScorerSupplierAdapter(scorer), nil
}

func (w *functionMatchWeight) BulkScorer(ctx *index.LeafReaderContext) (search.BulkScorer, error) {
	scorer, err := w.Scorer(ctx)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return search.NewDefaultBulkScorer(scorer), nil
}

func (w *functionMatchWeight) Matches(_ *index.LeafReaderContext, _ int) (search.Matches, error) {
	return nil, nil
}
func (w *functionMatchWeight) Count(_ *index.LeafReaderContext) (int, error) { return -1, nil }

// functionMatchScorer is the constant-score scorer for FunctionMatchQuery.
type functionMatchScorer struct {
	boost  float32
	iter   *search.RangeDocIdSetIterator
	values DoubleValues
	filter DoublePredicate
	doc    int
	cost   int64
}

func (s *functionMatchScorer) DocID() int { return s.doc }

func (s *functionMatchScorer) NextDoc() (int, error) {
	for {
		doc, err := s.iter.NextDoc()
		if err != nil {
			return search.NO_MORE_DOCS, err
		}
		if doc == search.NO_MORE_DOCS {
			s.doc = doc
			return doc, nil
		}
		ok, err := s.matches(doc)
		if err != nil {
			return search.NO_MORE_DOCS, err
		}
		if ok {
			s.doc = doc
			return doc, nil
		}
	}
}

func (s *functionMatchScorer) Advance(target int) (int, error) {
	doc, err := s.iter.Advance(target)
	if err != nil {
		return search.NO_MORE_DOCS, err
	}
	if doc == search.NO_MORE_DOCS {
		s.doc = doc
		return doc, nil
	}
	ok, err := s.matches(doc)
	if err != nil {
		return search.NO_MORE_DOCS, err
	}
	if ok {
		s.doc = doc
		return doc, nil
	}
	return s.NextDoc()
}

func (s *functionMatchScorer) Cost() int64             { return s.cost }
func (s *functionMatchScorer) DocIDRunEnd() int        { return s.doc + 1 }
func (s *functionMatchScorer) Score() float32          { return s.boost }
func (s *functionMatchScorer) GetMaxScore(int) float32 { return s.boost }

func (s *functionMatchScorer) matches(doc int) (bool, error) {
	ok, err := s.values.AdvanceExact(doc)
	if err != nil || !ok {
		return false, err
	}
	v, err := s.values.DoubleValue()
	if err != nil {
		return false, err
	}
	return s.filter(v), nil
}

var (
	_ search.Query  = (*FunctionMatchQuery)(nil)
	_ search.Weight = (*functionMatchWeight)(nil)
	_ search.Scorer = (*functionMatchScorer)(nil)
)
