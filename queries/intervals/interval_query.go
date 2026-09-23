// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/intervals/IntervalQuery.java

package intervals

import (
	"fmt"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// IntervalQuery retrieves documents containing intervals produced by an IntervalsSource
// and scores them using an IntervalScoreFunction.
//
// Mirrors org.apache.lucene.queries.intervals.IntervalQuery.
type IntervalQuery struct {
	search.BaseQuery
	field         string
	source        IntervalsSource
	scoreFunction IntervalScoreFunction
}

// NewIntervalQuery creates an IntervalQuery with the default saturation scoring function.
func NewIntervalQuery(field string, source IntervalsSource) *IntervalQuery {
	sf, _ := NewSaturationFunction(1) // pivot=1; error impossible for valid constant
	return &IntervalQuery{field: field, source: source, scoreFunction: sf}
}

// NewIntervalQueryWithPivot creates an IntervalQuery with a saturation scoring function.
func NewIntervalQueryWithPivot(field string, source IntervalsSource, pivot float32) (*IntervalQuery, error) {
	sf, err := NewSaturationFunction(pivot)
	if err != nil {
		return nil, err
	}
	return &IntervalQuery{field: field, source: source, scoreFunction: sf}, nil
}

// NewIntervalQueryWithSigmoid creates an IntervalQuery with a sigmoid scoring function.
func NewIntervalQueryWithSigmoid(field string, source IntervalsSource, pivot, exp float32) (*IntervalQuery, error) {
	sf, err := NewSigmoidFunction(pivot, exp)
	if err != nil {
		return nil, err
	}
	return &IntervalQuery{field: field, source: source, scoreFunction: sf}, nil
}

// GetField returns the field this query targets.
func (q *IntervalQuery) GetField() string { return q.field }

// GetSource returns the underlying IntervalsSource.
func (q *IntervalQuery) GetSource() IntervalsSource { return q.source }

// Rewrite renders the Query.rewrite(IndexSearcher) IntervalQuery inherits
// without overriding it: the query is already primitive and returns itself.
func (q *IntervalQuery) Rewrite(searcher *search.IndexSearcher) (search.Query, error) {
	return q, nil
}

// Visit visits the query.
func (q *IntervalQuery) Visit(visitor search.QueryVisitor) {
	if visitor.AcceptField(q.field) {
		q.source.Visit(q.field, visitor)
	}
}

// Equals reports structural equality.
func (q *IntervalQuery) Equals(other spi.Query) bool {
	o, ok := other.(*IntervalQuery)
	if !ok {
		return false
	}
	return q.field == o.field && q.source.Equals(o.source)
}

// HashCode returns a hash code.
func (q *IntervalQuery) HashCode() int {
	return hashString(q.field)*31 + q.source.HashCode()
}

// String returns a human-readable representation.
func (q *IntervalQuery) String() string {
	return fmt.Sprintf("%s:%s", q.field, q.source.String())
}

// CreateWeight mirrors IntervalQuery.createWeight(IndexSearcher, ScoreMode, float):
// return new IntervalWeight(this, boost).
func (q *IntervalQuery) CreateWeight(searcher *search.IndexSearcher, scoreMode search.ScoreMode, boost float32) (search.Weight, error) {
	return &intervalWeight{BaseWeight: search.NewBaseWeight(q), query: q, boost: boost}, nil
}

// intervalWeight is the Weight for an IntervalQuery.
type intervalWeight struct {
	*search.BaseWeight
	query *IntervalQuery
	boost float32
}

// Explain returns an explanation for the given document.
func (w *intervalWeight) Explain(ctx *index.LeafReaderContext, doc int) (search.Explanation, error) {
	supplier, err := w.ScorerSupplier(ctx)
	if err != nil || supplier == nil {
		return search.NoMatchExplanation("no matching intervals"), nil
	}
	// Weight.scorer(LeafReaderContext) passes Long.MAX_VALUE as the lead cost.
	sc, err := supplier.Get(1<<63 - 1)
	if err != nil {
		return nil, err
	}
	if is, ok := sc.(*IntervalScorer); ok {
		newDoc, err := is.Iterator().Advance(doc)
		if err != nil {
			return nil, err
		}
		if newDoc == doc {
			freq, err := is.Freq()
			if err != nil {
				return nil, err
			}
			return w.query.scoreFunction.Explain(w.query.String(), w.boost, freq), nil
		}
	}
	return search.NoMatchExplanation("no matching intervals"), nil
}

// ScorerSupplier creates a ScorerSupplier for the given leaf context.
func (w *intervalWeight) ScorerSupplier(ctx *index.LeafReaderContext) (search.ScorerSupplier, error) {
	intervals, err := w.query.source.Intervals(w.query.field, ctx)
	if err != nil {
		return nil, err
	}
	if intervals == nil {
		return nil, nil
	}
	scorer := NewIntervalScorer(intervals, w.query.source.MinExtent(), w.boost, w.query.scoreFunction)
	return search.NewDefaultScorerSupplier(scorer), nil
}

// Matches returns a Matches instance for the given document.
//
// Mirrors IntervalWeight.matches(LeafReaderContext, int): it hands
// MatchesUtils.forField a supplier that calls intervalsSource.matches(field,
// context, doc) and, when that yields an iterator, wraps it in a
// FilterMatchesIterator whose getQuery() reports a fresh IntervalQuery over the
// same field and source.
func (w *intervalWeight) Matches(ctx *index.LeafReaderContext, doc int) (search.Matches, error) {
	field := w.query.field
	source := w.query.source
	return search.MatchesUtils.ForField(field, func() (search.MatchesIterator, error) {
		mi, err := source.Matches(field, ctx, doc)
		if err != nil {
			return nil, err
		}
		if mi == nil {
			return nil, nil
		}
		return &intervalQueryMatchesIterator{
			FilterMatchesIterator: search.NewFilterMatchesIterator(mi),
			field:                 field,
			source:                source,
		}, nil
	})
}

// intervalQueryMatchesIterator renders the anonymous FilterMatchesIterator
// subclass created by IntervalWeight.matches, which overrides getQuery().
type intervalQueryMatchesIterator struct {
	*search.FilterMatchesIterator
	field  string
	source IntervalsSource
}

// GetQuery returns a new IntervalQuery over the weight's field and source,
// mirroring the overridden getQuery() of the Java anonymous class.
func (m *intervalQueryMatchesIterator) GetQuery() search.Query {
	return NewIntervalQuery(m.field, m.source)
}

// IsCacheable returns true.
func (w *intervalWeight) IsCacheable(ctx *index.LeafReaderContext) bool { return true }

// Count returns -1 to signal no sub-linear count is available.
func (w *intervalWeight) Count(_ *index.LeafReaderContext) (int, error) { return -1, nil }

var _ search.Weight = (*intervalWeight)(nil)
var _ search.MatchesIterator = (*intervalQueryMatchesIterator)(nil)

// Scorer renders the final Weight.scorer(LeafReaderContext): the scorer of
// scorerSupplier(context).get(Long.MAX_VALUE), or nil when no document
// matches. It is restated because the embedded BaseWeight.Scorer would call
// BaseWeight.ScorerSupplier, not this type's override.
func (w *intervalWeight) Scorer(context *index.LeafReaderContext) (search.Scorer, error) {
	scorerSupplier, err := w.ScorerSupplier(context)
	if err != nil || scorerSupplier == nil {
		return nil, err
	}
	return scorerSupplier.Get(math.MaxInt64)
}

// BulkScorer renders the final Weight.bulkScorer(LeafReaderContext):
// scorerSupplier(context), marked as the top-level scoring clause, supplies
// the bulk scorer; nil when no document matches. It is restated because the
// embedded BaseWeight.BulkScorer would call BaseWeight.ScorerSupplier, not
// this type's override.
func (w *intervalWeight) BulkScorer(context *index.LeafReaderContext) (search.BulkScorer, error) {
	scorerSupplier, err := w.ScorerSupplier(context)
	if err != nil || scorerSupplier == nil {
		// No docs match
		return nil, err
	}
	if err := scorerSupplier.SetTopLevelScoringClause(); err != nil {
		return nil, err
	}
	return scorerSupplier.BulkScorer()
}
