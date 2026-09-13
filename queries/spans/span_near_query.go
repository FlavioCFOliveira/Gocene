// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/spans/SpanNearQuery.java

package spans

import (
	"fmt"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// SpanNearQuery matches spans which are near one another, with a configurable
// slop (maximum unmatched positions between them) and an optional in-order
// constraint.
//
// Mirrors org.apache.lucene.queries.spans.SpanNearQuery.
//
// Deviations from Java:
//   - Java SpanNearQuery is Cloneable; Gocene Clone() method returns a deep
//     copy without Java reflection.
//   - The Builder inner class is a package-level struct SpanNearBuilder.
//   - SpanGapQuery and GapSpans are package-private types here.
//   - SimScorer construction deferred (same as SpanTermQuery).
type SpanNearQuery struct {
	search.BaseQuery
	clauses []SpanQuery
	slop    int
	inOrder bool
	field   string
}

// SpanNearBuilder builds a SpanNearQuery fluently.
type SpanNearBuilder struct {
	field   string
	ordered bool
	clauses []spanClause
	slop    int
}

type spanClause struct {
	query SpanQuery
	gap   int // > 0 means this is a gap
}

// NewOrderedNearQuery creates a builder for an ordered SpanNearQuery.
func NewOrderedNearQuery(field string) *SpanNearBuilder {
	return &SpanNearBuilder{field: field, ordered: true}
}

// NewUnorderedNearQuery creates a builder for an unordered SpanNearQuery.
func NewUnorderedNearQuery(field string) *SpanNearBuilder {
	return &SpanNearBuilder{field: field, ordered: false}
}

// AddClause adds a clause.
//
// Mirrors SpanNearQuery.Builder.addClause(SpanQuery).
func (b *SpanNearBuilder) AddClause(q SpanQuery) *SpanNearBuilder {
	b.clauses = append(b.clauses, spanClause{query: q})
	return b
}

// AddGap adds a positional gap (ordered queries only).
func (b *SpanNearBuilder) AddGap(width int) *SpanNearBuilder {
	b.clauses = append(b.clauses, spanClause{gap: width})
	return b
}

// SetSlop sets the maximum positional slop.
func (b *SpanNearBuilder) SetSlop(slop int) *SpanNearBuilder {
	b.slop = slop
	return b
}

// Build constructs the SpanNearQuery.
func (b *SpanNearBuilder) Build() *SpanNearQuery {
	clauses := make([]SpanQuery, 0, len(b.clauses))
	for _, c := range b.clauses {
		if c.query != nil {
			clauses = append(clauses, c.query)
		}
	}
	return newSpanNearQuery(clauses, b.slop, b.ordered, b.field)
}

// newSpanNearQuery is the internal constructor.
func newSpanNearQuery(clauses []SpanQuery, slop int, inOrder bool, field string) *SpanNearQuery {
	if field == "" && len(clauses) > 0 {
		field = clauses[0].GetField()
	}
	return &SpanNearQuery{
		clauses: clauses,
		slop:    slop,
		inOrder: inOrder,
		field:   field,
	}
}

// NewSpanNearQuery constructs a SpanNearQuery. It matches spans matching a
// span from each clause, with up to slop total unmatched positions between
// them.
//
// When inOrder is true, the spans from each clause must be in the same order as
// in clauses and must be non-overlapping. When inOrder is false, the spans from
// each clause need not be ordered and may overlap.
//
// Mirrors SpanNearQuery(SpanQuery[], int, boolean).
func NewSpanNearQuery(clausesIn []SpanQuery, slop int, inOrder bool) (*SpanNearQuery, error) {
	q := &SpanNearQuery{
		clauses: make([]SpanQuery, 0, len(clausesIn)),
		slop:    slop,
		inOrder: inOrder,
	}
	for _, clause := range clausesIn {
		if q.field == "" { // check field
			q.field = clause.GetField()
		} else if clause.GetField() != "" && clause.GetField() != q.field {
			return nil, fmt.Errorf("Clauses must have same field.")
		}
		q.clauses = append(q.clauses, clause)
	}
	return q, nil
}

// NewSpanNearQueryFromTerms constructs a SpanNearQuery directly from term
// clauses. It is the SpanTermQuery-typed spelling of Java's
// SpanNearQuery(SpanQuery[], int, boolean) constructor, kept because the
// package's own tests build near queries from terms.
func NewSpanNearQueryFromTerms(clauses []*SpanTermQuery, slop int, inOrder bool) *SpanNearQuery {
	widened := make([]SpanQuery, len(clauses))
	for i, c := range clauses {
		widened[i] = c
	}
	return newSpanNearQuery(widened, slop, inOrder, "")
}

// GetField returns the field targeted by this query.
func (q *SpanNearQuery) GetField() string { return q.field }

// GetClauses returns the clauses whose spans are matched.
//
// Mirrors SpanNearQuery.getClauses().
func (q *SpanNearQuery) GetClauses() []SpanQuery {
	out := make([]SpanQuery, len(q.clauses))
	copy(out, q.clauses)
	return out
}

// GetSlop returns the maximum positional distance between sub-spans.
func (q *SpanNearQuery) GetSlop() int { return q.slop }

// IsInOrder returns whether sub-spans must appear in order.
func (q *SpanNearQuery) IsInOrder() bool { return q.inOrder }

// Visit walks the query tree.
func (q *SpanNearQuery) Visit(visitor search.QueryVisitor) {
	if !visitor.AcceptField(q.field) {
		return
	}
	subVisitor := visitor.GetSubVisitor(search.MUST, q)
	for _, c := range q.clauses {
		visitSpanQuery(c, subVisitor)
	}
}

// Clone returns a copy carrying the same clauses.
func (q *SpanNearQuery) Clone() search.Query {
	clauses := make([]SpanQuery, len(q.clauses))
	copy(clauses, q.clauses)
	return newSpanNearQuery(clauses, q.slop, q.inOrder, q.field)
}

// Equals reports structural equality.
func (q *SpanNearQuery) Equals(other spi.Query) bool {
	o, ok := other.(*SpanNearQuery)
	if !ok {
		return false
	}
	if q.inOrder != o.inOrder || q.slop != o.slop || q.field != o.field {
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

// HashCode returns a hash code.
func (q *SpanNearQuery) HashCode() int {
	h := 0
	for _, b := range q.field {
		h = h*31 + int(b)
	}
	for _, c := range q.clauses {
		h = h*31 + c.HashCode()
	}
	h = h*31 + q.slop
	if q.inOrder {
		h = h*31 + 8
	} else {
		h = h*31 + 4
	}
	return h
}

// ToString mirrors SpanNearQuery.toString(String field).
func (q *SpanNearQuery) ToString(field string) string {
	parts := make([]string, len(q.clauses))
	for i, c := range q.clauses {
		parts[i] = spanQueryToString(c, field)
	}
	return fmt.Sprintf("spanNear([%s], %d, %v)", strings.Join(parts, ", "), q.slop, q.inOrder)
}

// String renders Query.toString(), whose Java body is toString("").
func (q *SpanNearQuery) String() string { return q.ToString("") }

// CreateWeight creates a Weight for this query.
func (q *SpanNearQuery) CreateWeight(searcher *search.IndexSearcher, scoreMode search.ScoreMode, boost float32) (search.Weight, error) {
	return q.createSpanWeight(searcher, scoreMode, boost)
}

// CreateSpanWeight creates a SpanWeight for this query.
func (q *SpanNearQuery) CreateSpanWeight(searcher *search.IndexSearcher, scoreMode search.ScoreMode, boost float32) (*SpanWeight, error) {
	return q.createSpanWeight(searcher, scoreMode, boost)
}

func (q *SpanNearQuery) createSpanWeight(searcher *search.IndexSearcher, scoreMode search.ScoreMode, boost float32) (*SpanWeight, error) {
	subWeights := make([]*SpanWeight, len(q.clauses))
	for i, clause := range q.clauses {
		sw, err := clause.CreateSpanWeight(searcher, scoreMode, boost)
		if err != nil {
			return nil, err
		}
		subWeights[i] = sw
	}

	inOrder := q.inOrder
	slop := q.slop
	field := q.field
	allCacheable := func(ctx *index.LeafReaderContext) bool {
		for _, sw := range subWeights {
			if !sw.IsCacheable(ctx) {
				return false
			}
		}
		return true
	}

	return NewSpanWeight(q, SpanWeightConfig{
		Field:     field,
		SimScorer: nil,
		GetSpans: func(ctx *index.LeafReaderContext, postings Postings) (Spans, error) {
			return getSpansNear(ctx, subWeights, field, slop, inOrder, postings)
		},
		ExtractStates: func(terms map[string]*index.TermStates) {
			for _, sw := range subWeights {
				sw.ExtractTermStates(terms)
			}
		},
		IsCacheable: allCacheable,
	}), nil
}

// getSpansNear builds either NearSpansOrdered or NearSpansUnordered.
func getSpansNear(ctx *index.LeafReaderContext, subWeights []*SpanWeight, field string, slop int, inOrder bool, postings Postings) (Spans, error) {
	if ctx == nil {
		return nil, nil
	}
	lr := ctx.LeafReader()
	if lr == nil {
		return nil, nil
	}
	// Check the field has terms.
	terms, err := lr.Terms(field)
	if err != nil {
		return nil, err
	}
	if terms == nil {
		return nil, nil
	}

	subSpans := make([]Spans, 0, len(subWeights))
	for _, sw := range subWeights {
		sp, err := sw.GetSpans(ctx, postings)
		if err != nil {
			return nil, err
		}
		if sp == nil {
			return nil, nil // all sub-spans required
		}
		subSpans = append(subSpans, sp)
	}

	if inOrder {
		return NewNearSpansOrdered(slop, subSpans)
	}
	return NewNearSpansUnordered(slop, subSpans)
}

// GapSpans is a Spans implementation that represents a positional gap.
// It advances through positions mechanically without any real postings.
//
// Mirrors org.apache.lucene.queries.spans.SpanNearQuery.GapSpans (package-private).
type GapSpans struct {
	BaseSpans
	doc   int
	pos   int
	width int
}

// NewGapSpans constructs a GapSpans with the given width.
func NewGapSpans(width int) *GapSpans {
	return &GapSpans{doc: -1, pos: -1, width: width}
}

// SkipToPosition advances to the given position.
func (s *GapSpans) SkipToPosition(position int) (int, error) {
	s.pos = position
	return s.pos, nil
}

// NextStartPosition increments position by one.
func (s *GapSpans) NextStartPosition() (int, error) {
	s.pos++
	return s.pos, nil
}

// StartPosition returns the current position.
func (s *GapSpans) StartPosition() int { return s.pos }

// EndPosition returns position + width.
func (s *GapSpans) EndPosition() int { return s.pos + s.width }

// Width returns the gap width.
func (s *GapSpans) Width() int { return s.width }

// Collect is a no-op for gap spans.
func (s *GapSpans) Collect(_ SpanCollector) error { return nil }

// DocID returns the current document ID.
func (s *GapSpans) DocID() int { return s.doc }

// NextDoc increments the document ID.
func (s *GapSpans) NextDoc() (int, error) { s.pos = -1; s.doc++; return s.doc, nil }

// Advance advances to the target document.
func (s *GapSpans) Advance(target int) (int, error) { s.pos = -1; s.doc = target; return s.doc, nil }

// Cost returns 0 (gap spans have no real cost).
func (s *GapSpans) Cost() int64 { return 0 }

// DocIDRunEnd returns the conservative upper bound.
func (s *GapSpans) DocIDRunEnd() (int, error) { return s.doc + 1, nil }

// PositionsCost returns 0.
func (s *GapSpans) PositionsCost() float32 { return 0 }

// AsTwoPhaseIterator returns nil.
func (s *GapSpans) AsTwoPhaseIterator() *search.TwoPhaseIterator { return nil }

var _ Spans = (*GapSpans)(nil)
var _ search.Query = (*SpanNearQuery)(nil)

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene
// 10.5.0, which every subclass inherits unless it overrides it.
func (s *GapSpans) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(s, upTo, bitSet, offset)
}
