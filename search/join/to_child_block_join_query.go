// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// ToChildBlockJoinQuery is just like ToParentBlockJoinQuery, except this query
// joins in reverse: you provide a Query matching parent documents and it joins
// down to child documents.
//
// Mirrors org.apache.lucene.search.join.ToChildBlockJoinQuery (Apache Lucene
// 10.5.0).
//
// lucene.experimental
type ToChildBlockJoinQuery struct {
	parentsFilter BitSetProducer
	parentQuery   search.Query
}

// illegalAdvanceOnParent mirrors ToChildBlockJoinQuery.ILLEGAL_ADVANCE_ON_PARENT.
const illegalAdvanceOnParent = "Expect to be advanced on child docs only. got docID="

// NewToChildBlockJoinQuery creates a ToChildBlockJoinQuery; parentQuery is the
// Query that matches parent documents and parentsFilter the filter
// identifying the parent documents.
//
// Mirrors ToChildBlockJoinQuery(Query, BitSetProducer).
func NewToChildBlockJoinQuery(parentQuery search.Query, parentsFilter BitSetProducer) *ToChildBlockJoinQuery {
	return &ToChildBlockJoinQuery{parentQuery: parentQuery, parentsFilter: parentsFilter}
}

// Visit renders visit(QueryVisitor): visitor.visitLeaf(this).
func (q *ToChildBlockJoinQuery) Visit(visitor search.QueryVisitor) {
	visitor.VisitLeaf(q)
}

// CreateWeight renders createWeight(IndexSearcher, ScoreMode, float).
func (q *ToChildBlockJoinQuery) CreateWeight(searcher *search.IndexSearcher, scoreMode search.ScoreMode, boost float32) (search.Weight, error) {
	parentWeight, err := q.parentQuery.CreateWeight(searcher, scoreMode, boost)
	if err != nil {
		return nil, err
	}
	return NewToChildBlockJoinWeight(q, parentWeight, q.parentsFilter, scoreMode.NeedsScores()), nil
}

// GetParentQuery returns our parent query.
func (q *ToChildBlockJoinQuery) GetParentQuery() search.Query {
	return q.parentQuery
}

// Rewrite renders rewrite(IndexSearcher).
func (q *ToChildBlockJoinQuery) Rewrite(indexSearcher *search.IndexSearcher) (search.Query, error) {
	parentRewrite, err := q.parentQuery.Rewrite(indexSearcher)
	if err != nil {
		return nil, err
	}
	if parentRewrite != q.parentQuery {
		return NewToChildBlockJoinQuery(parentRewrite, q.parentsFilter), nil
	}
	// super.rewrite(indexSearcher): Query.rewrite returns this.
	return q, nil
}

// ToString renders toString(String): the parent query is rendered with
// Query.toString(), i.e. toString("").
func (q *ToChildBlockJoinQuery) ToString(field string) string {
	return "ToChildBlockJoinQuery (" + joinQueryToString(q.parentQuery, "") + ")"
}

// String renders Query.toString(): toString("").
func (q *ToChildBlockJoinQuery) String() string {
	return q.ToString("")
}

// Equals renders equals(Object): sameClassAs(other) && parentQuery.equals &&
// parentsFilter.equals.
func (q *ToChildBlockJoinQuery) Equals(other spi.Query) bool {
	o, ok := other.(*ToChildBlockJoinQuery)
	if !ok {
		return false
	}
	return q.parentQuery.Equals(o.parentQuery) && bitSetProducerEquals(q.parentsFilter, o.parentsFilter)
}

// HashCode renders hashCode().
//
// PORT NOTE: the seed renders Query.classHash(), which Apache Lucene 10.5.0
// derives from the JVM class identity and which therefore has no reproducible
// Go counterpart; a per-type constant seed is the idiom this package uses (see
// ParentChildrenBlockJoinQuery.HashCode).
func (q *ToChildBlockJoinQuery) HashCode() int {
	const prime = 31
	hash := 19
	hash = prime*hash + q.parentQuery.HashCode()
	hash = prime*hash + bitSetProducerHashCode(q.parentsFilter)
	return hash
}

// bitSetProducerEquals renders parentsFilter.equals(other.parentsFilter):
// the implementation's equals when it declares one, identity otherwise
// (Object.equals).
func bitSetProducerEquals(a, b BitSetProducer) bool {
	if e, ok := a.(interface{ Equals(interface{}) bool }); ok {
		return e.Equals(b)
	}
	return a == b
}

// bitSetProducerHashCode renders parentsFilter.hashCode(): the
// implementation's hashCode when it declares one, a constant otherwise.
func bitSetProducerHashCode(p BitSetProducer) int {
	if h, ok := p.(interface{ HashCode() int }); ok {
		return h.HashCode()
	}
	return 0
}

var _ search.Query = (*ToChildBlockJoinQuery)(nil)

// ToChildBlockJoinWeight renders the private static class
// ToChildBlockJoinQuery.ToChildBlockJoinWeight, which extends FilterWeight
// over the parent weight.
type ToChildBlockJoinWeight struct {
	joinQuery     search.Query
	in            search.Weight
	parentsFilter BitSetProducer
	doScores      bool
}

// NewToChildBlockJoinWeight renders ToChildBlockJoinWeight(Query, Weight,
// BitSetProducer, boolean).
func NewToChildBlockJoinWeight(joinQuery search.Query, parentWeight search.Weight, parentsFilter BitSetProducer, doScores bool) *ToChildBlockJoinWeight {
	return &ToChildBlockJoinWeight{joinQuery: joinQuery, in: parentWeight, parentsFilter: parentsFilter, doScores: doScores}
}

// GetQuery renders the inherited Weight.getQuery(): the join query FilterWeight
// was constructed with.
func (w *ToChildBlockJoinWeight) GetQuery() search.Query { return w.joinQuery }

// ScorerSupplier renders scorerSupplier(LeafReaderContext).
//
// NOTE: acceptDocs applies (and is checked) only in the child document space.
func (w *ToChildBlockJoinWeight) ScorerSupplier(readerContext *index.LeafReaderContext) (search.ScorerSupplier, error) {
	parentScorer, err := w.in.Scorer(readerContext)
	if err != nil {
		return nil, err
	}
	if parentScorer == nil {
		// No matches
		return nil, nil
	}

	// NOTE: this doesn't take acceptDocs into account, the responsibility
	// to not match deleted docs is on the scorer
	parents, err := w.parentsFilter.GetBitSet(readerContext)
	if err != nil {
		return nil, err
	}
	if parents == nil {
		// No parents
		return nil, nil
	}

	scorer := NewToChildBlockJoinScorer(parentScorer, parents, w.doScores)
	return search.NewDefaultScorerSupplier(scorer), nil
}

// Scorer renders the inherited Weight.scorer(LeafReaderContext):
// scorerSupplier(context).get(Long.MAX_VALUE).
func (w *ToChildBlockJoinWeight) Scorer(context *index.LeafReaderContext) (search.Scorer, error) {
	ss, err := w.ScorerSupplier(context)
	if err != nil || ss == nil {
		return nil, err
	}
	return ss.Get(math.MaxInt64)
}

// BulkScorer renders the inherited Weight.bulkScorer(LeafReaderContext).
func (w *ToChildBlockJoinWeight) BulkScorer(context *index.LeafReaderContext) (search.BulkScorer, error) {
	ss, err := w.ScorerSupplier(context)
	if err != nil || ss == nil {
		return nil, err
	}
	return search.DefaultScorerSupplierBulkScorer(ss)
}

// Explain renders explain(LeafReaderContext, int).
func (w *ToChildBlockJoinWeight) Explain(context *index.LeafReaderContext, doc int) (search.Explanation, error) {
	s, err := w.Scorer(context)
	if err != nil {
		return nil, err
	}
	if s != nil {
		scorer := s.(*ToChildBlockJoinScorer)
		advanced, err := scorer.Iterator().Advance(doc)
		if err != nil {
			return nil, err
		}
		if advanced == doc {
			parentDoc := scorer.GetParentDoc()
			score, err := scorer.Score()
			if err != nil {
				return nil, err
			}
			parentExplanation, err := w.in.Explain(context, parentDoc)
			if err != nil {
				return nil, err
			}
			return search.MatchExplanationWithDetails(score,
				fmt.Sprintf("Score based on parent document %d", parentDoc+context.DocBase), parentExplanation), nil
		}
	}
	return search.NoMatchExplanation("Not a match"), nil
}

// Count renders count(LeafReaderContext): -1.
func (w *ToChildBlockJoinWeight) Count(context *index.LeafReaderContext) (int, error) {
	return -1, nil
}

// IsCacheable renders the inherited FilterWeight.isCacheable(LeafReaderContext):
// in.isCacheable(ctx).
func (w *ToChildBlockJoinWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return w.in.IsCacheable(ctx)
}

// Matches renders the inherited FilterWeight.matches(LeafReaderContext, int):
// in.matches(context, doc).
func (w *ToChildBlockJoinWeight) Matches(context *index.LeafReaderContext, doc int) (search.Matches, error) {
	return w.in.Matches(context, doc)
}

var _ search.Weight = (*ToChildBlockJoinWeight)(nil)
