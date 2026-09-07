// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// SpanNearQuery matches spans which are near one another. One can specify slop, the maximum number of
// intervening unmatched positions, as well as whether matches are required to be in-order.
//
// This is the Go port of Lucene's org.apache.lucene.queries.spans.SpanNearQuery.
type SpanNearQuery struct {
	*BaseSpanQuery
	clauses []SpanQuery
	slop    int
	inOrder bool
}

// SpanNearQueryBuilder is a builder for SpanNearQueries.
type SpanNearQueryBuilder struct {
	field    string
	ordered  bool
	clauses  []SpanQuery
	slop     int
}

// NewSpanNearQueryBuilder constructs a new builder.
func NewSpanNearQueryBuilder(field string, ordered bool) *SpanNearQueryBuilder {
	return &SpanNearQueryBuilder{
		field:   field,
		ordered: ordered,
	}
}

// AddClause adds a new clause to the query.
func (b *SpanNearQueryBuilder) AddClause(clause SpanQuery) *SpanNearQueryBuilder {
	if clause.GetField() != b.field {
		panic(fmt.Sprintf("Cannot add clause %v to SpanNearQuery for field %s", clause, b.field))
	}
	b.clauses = append(b.clauses, clause)
	return b
}

// AddGap adds a gap after the previous clause of a defined width.
func (b *SpanNearQueryBuilder) AddGap(width int) *SpanNearQueryBuilder {
	if !b.ordered {
		panic("Gaps can only be added to ordered near queries")
	}
	b.clauses = append(b.clauses, NewSpanGapQuery(b.field, width))
	return b
}

// SetSlop sets the slop for this query.
func (b *SpanNearQueryBuilder) SetSlop(slop int) *SpanNearQueryBuilder {
	b.slop = slop
	return b
}

// Build builds the SpanNearQuery.
func (b *SpanNearQueryBuilder) Build() *SpanNearQuery {
	return NewSpanNearQuery(b.clauses, b.slop, b.ordered)
}

// NewOrderedNearQuery returns a builder for an ordered query on a particular field.
func NewOrderedNearQuery(field string) *SpanNearQueryBuilder {
	return NewSpanNearQueryBuilder(field, true)
}

// NewUnorderedNearQuery returns a builder for an unordered query on a particular field.
func NewUnorderedNearQuery(field string) *SpanNearQueryBuilder {
	return NewSpanNearQueryBuilder(field, false)
}

// NewSpanNearQuery constructs a SpanNearQuery.
func NewSpanNearQuery(clausesIn []SpanQuery, slop int, inOrder bool) *SpanNearQuery {
	var field string
	clauses := make([]SpanQuery, 0, len(clausesIn))

	for _, clause := range clausesIn {
		f := clause.GetField()
		if field == "" {
			field = f
		} else if f != "" && f != field {
			panic("Clauses must have same field.")
		}
		clauses = append(clauses, clause)
	}

	return &SpanNearQuery{
		BaseSpanQuery: NewBaseSpanQuery(field),
		clauses:       clauses,
		slop:          slop,
		inOrder:       inOrder,
	}
}

// GetClauses returns the clauses whose spans are matched.
func (q *SpanNearQuery) GetClauses() []SpanQuery {
	return q.clauses
}

// GetSlop returns the maximum number of intervening unmatched positions permitted.
func (q *SpanNearQuery) GetSlop() int {
	return q.slop
}

// IsInOrder returns true if matches are required to be in-order.
func (q *SpanNearQuery) IsInOrder() bool {
	return q.inOrder
}

// String returns a string representation of the query.
func (q *SpanNearQuery) String(field string) string {
	var sb strings.Builder
	sb.WriteString("spanNear([")
	for i, clause := range q.clauses {
		sb.WriteString(clause.String(field))
		if i < len(q.clauses)-1 {
			sb.WriteString(", ")
		}
	}
	sb.WriteString("], ")
	sb.WriteString(fmt.Sprintf("%d, %v)", q.slop, q.inOrder))
	return sb.String()
}

// CreateWeight creates a SpanNearWeight for this query.
func (q *SpanNearQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (SpanWeight, error) {
	subWeights := make([]SpanWeight, 0, len(q.clauses))
	for _, cq := range q.clauses {
		sw, err := cq.CreateWeight(searcher, needsScores, boost)
		if err != nil {
			return nil, err
		}
		subWeights = append(subWeights, sw)
	}

	// Note: getTermStates is not implemented in BaseSpanQuery or SpanWeight in the current port.
	// Lucene's SpanNearQuery uses it to optimize. We pass nil for now as per the port state.
	return NewSpanNearWeight(subWeights, searcher, nil, boost), nil
}

// Rewrite rewrites the query to a simpler form.
func (q *SpanNearQuery) Rewrite(searcher *IndexSearcher) (Query, error) {
	actuallyRewritten := false
	rewrittenClauses := make([]SpanQuery, 0, len(q.clauses))

	for _, c := range q.clauses {
		rewritten, err := c.Rewrite(searcher)
		if err != nil {
			return nil, err
		}
		sq, ok := rewritten.(SpanQuery)
		if !ok {
			return nil, fmt.Errorf("rewritten clause must be a SpanQuery")
		}
		if sq != c {
			actuallyRewritten = true
		}
		rewrittenClauses = append(rewrittenClauses, sq)
	}

	if actuallyRewritten {
		newQ := *q
		newQ.clauses = rewrittenClauses
		return &newQ, nil
	}

	return q, nil
}

// Visit visits the query.
func (q *SpanNearQuery) Visit(visitor QueryVisitor) {
	if !visitor.AcceptField(q.GetField()) {
		return
	}
	v := visitor.GetSubVisitor(BooleanClauseOccurMust, q)
	for _, clause := range q.clauses {
		clause.Visit(v)
	}
}

// Equals checks if this query equals another.
func (q *SpanNearQuery) Equals(other Query) bool {
	if other == nil {
		return false
	}
	o, ok := other.(*SpanNearQuery)
	if !ok {
		return false
	}
	if q.inOrder != o.inOrder || q.slop != o.slop {
		return false
	}
	if len(q.clauses) != len(o.clauses) {
		return false
	}
	for i := range q.clauses {
		if !q.clauses[i].Equals(o.clauses[i]) {
			return false
		}
	}
	return true
}

// HashCode returns a hash code for this query.
func (q *SpanNearQuery) HashCode() int {
	result := 17 // classHash() placeholder
	h := 0
	for _, c := range q.clauses {
		h = 31*h + c.HashCode()
	}
	result ^= h
	result += q.slop
	fac := 1 + 4
	if q.inOrder {
		fac = 1 + 8
	}
	return fac * result
}

// SpanNearWeight is the weight for a SpanNearQuery.
type SpanNearWeight struct {
	*SpanWeight
	subWeights []SpanWeight
}

// NewSpanNearWeight creates a new SpanNearWeight.
func NewSpanNearWeight(subWeights []SpanWeight, searcher *IndexSearcher, terms map[index.Term]*index.TermStates, boost float32) *SpanNearWeight {
	return &SpanNearWeight{
		SpanWeight:  NewSpanWeight(nil, nil),
		subWeights: subWeights,
	}
}

// ExtractTermStates extracts term states from all sub-weights.
func (sw *SpanNearWeight) ExtractTermStates(contexts map[index.Term]*index.TermStates) {
	for _, w := range sw.subWeights {
		w.ExtractTermStates(contexts)
	}
}

// GetSpans returns the spans for the near query.
func (sw *SpanNearWeight) GetSpans(context *index.LeafReaderContext, requiredPostings int) (Spans, error) {
	terms := context.Reader().Terms(sw.SpanWeight.SpanQuery.GetField())
	if terms == nil {
		return nil, nil // field does not exist
	}

	subSpans := make([]Spans, 0, len(sw.subWeights))
	for _, w := range sw.subWeights {
		subSpan, err := w.GetSpans(context, requiredPostings)
		if err != nil {
			return nil, err
		}
		if subSpan != nil {
			subSpans = append(subSpans, subSpan)
		} else {
			return nil, nil // all required
		}
	}

	if !sw.SpanWeight.SpanQuery.(*SpanNearQuery).inOrder {
		return nil, fmt.Errorf("NearSpansUnordered not implemented")
	}
	return nil, fmt.Errorf("NearSpansOrdered not implemented")
}

// IsCacheable returns true if all sub-weights are cacheable.
func (sw *SpanNearWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	for _, w := range sw.subWeights {
		if !w.IsCacheable(ctx) {
			return false
		}
	}
	return true
}

// ScorerSupplier creates a scorer supplier for the near query.
func (sw *SpanNearWeight) ScorerSupplier(context *index.LeafReaderContext) (ScorerSupplier, error) {
	spans, err := sw.GetSpans(context, 1) // Postings.POSITIONS = 1
	if err != nil {
		return nil, err
	}
	if spans == nil {
		return nil, nil
	}

	return nil, fmt.Errorf("SpanScorer not implemented")
}

// SpanGapQuery is a special kind of span query that matches a gap of a certain width.
type SpanGapQuery struct {
	*BaseSpanQuery
	width int
}

// NewSpanGapQuery creates a new SpanGapQuery.
func NewSpanGapQuery(field string, width int) *SpanGapQuery {
	return &SpanGapQuery{
		BaseSpanQuery: NewBaseSpanQuery(field),
		width:         width,
	}
}

// Visit visits the gap query.
func (q *SpanGapQuery) Visit(visitor QueryVisitor) {
	visitor.VisitLeaf(q)
}

// String returns a string representation of the gap query.
func (q *SpanGapQuery) String(field string) string {
	return fmt.Sprintf("SpanGap(%s:%d)", field, q.width)
}

// CreateWeight creates a weight for the gap query.
func (q *SpanGapQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return NewSpanGapWeight(searcher, boost), nil
}

// SpanGapWeight is the weight for a SpanGapQuery.
type SpanGapWeight struct {
	*SpanWeight
}

// NewSpanGapWeight creates a new SpanGapWeight.
func NewSpanGapWeight(searcher *IndexSearcher, boost float32) *SpanGapWeight {
	return &SpanGapWeight{
		SpanWeight: NewSpanWeight(nil, nil),
	}
}

// ExtractTermStates does nothing for gap queries.
func (w *SpanGapWeight) ExtractTermStates(contexts map[index.Term]*index.TermStates) {}

// GetSpans returns gap spans.
func (w *SpanGapWeight) GetSpans(ctx *index.LeafReaderContext, requiredPostings int) (Spans, error) {
	q := w.SpanWeight.SpanQuery.(*SpanGapQuery)
	return &GapSpans{width: q.width}, nil
}

// IsCacheable returns true for gap queries.
func (w *SpanGapWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return true
}

// GapSpans is the spans implementation for gaps.
type GapSpans struct {
	doc   int
	pos   int
	width int
}

func (s *GapSpans) NextStartPosition() (int, error) {
	s.pos++
	return s.pos, nil
}

func (s *GapSpans) SkipToPosition(position int) (int, error) {
	s.pos = position
	return s.pos, nil
}

func (s *GapSpans) StartPosition() int {
	return s.pos
}

func (s *GapSpans) EndPosition() int {
	return s.pos + s.width
}

func (s *GapSpans) Width() int {
	return s.width
}

func (s *GapSpans) Collect(collector SpanCollector) error {
	return nil
}

func (s *GapSpans) DocID() int {
	return s.doc
}

func (s *GapSpans) NextDoc() (int, error) {
	s.pos = -1
	s.doc++
	return s.doc, nil
}

func (s *GapSpans) Advance(target int) (int, error) {
	s.pos = -1
	s.doc = target
	return s.doc, nil
}

func (s *GapSpans) Cost() int64 {
	return 0
}

func (s *GapSpans) PositionsCost() float32 {
	return 0
}
