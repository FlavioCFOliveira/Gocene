// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/intervals/IntervalQuery.java

package intervals

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// IntervalQuery retrieves documents containing intervals produced by an IntervalsSource
// and scores them using an IntervalScoreFunction.
//
// Mirrors org.apache.lucene.queries.intervals.IntervalQuery.
//
// Deviations from Java:
//   - Weight/ScorerSupplier/Matches deferred; CreateWeight returns a stub weight.
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

// Visit visits the query.
func (q *IntervalQuery) Visit(visitor search.QueryVisitor) {
	if visitor.AcceptField(q.field) {
		q.source.Visit(q.field, visitor)
	}
}

// Clone returns a shallow copy.
func (q *IntervalQuery) Clone() search.Query {
	cp := *q
	return &cp
}

// Equals reports structural equality.
func (q *IntervalQuery) Equals(other search.Query) bool {
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

// CreateWeight creates a Weight for this query.
// Full scoring Weight with interval iteration is used when needsScores is true.
func (q *IntervalQuery) CreateWeight(searcher *search.IndexSearcher, needsScores bool, boost float32) (search.Weight, error) {
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
	sc, err := supplier.Get(0)
	if err != nil || sc == nil {
		return search.NoMatchExplanation("no matching intervals"), nil
	}
	advanced, err := sc.Advance(doc)
	if err != nil {
		return nil, err
	}
	if advanced != doc {
		return search.NoMatchExplanation("no matching intervals"), nil
	}
	is, ok := sc.(*IntervalScorer)
	if !ok {
		return search.NoMatchExplanation("no matching intervals"), nil
	}
	freq, err := is.Freq()
	if err != nil {
		return nil, err
	}
	return w.query.scoreFunction.Explain(w.query.String(), w.boost, freq), nil
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
	return search.NewScorerSupplierAdapter(scorer), nil
}

// IsCacheable returns true.
func (w *intervalWeight) IsCacheable(ctx *index.LeafReaderContext) bool { return true }

var _ search.Weight = (*intervalWeight)(nil)
