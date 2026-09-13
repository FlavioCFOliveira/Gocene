// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.5.0:
//   lucene/queries/src/java/org/apache/lucene/queries/spans/SpanContainQuery.java

package spans

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// SpanContainQuery is the abstract base for SpanContainingQuery and
// SpanWithinQuery. It holds two sub-queries, big and little, which must share
// the same field.
//
// Mirrors org.apache.lucene.queries.spans.SpanContainQuery (abstract,
// package-private).
//
// Deviations from Java:
//   - Java's inner class SpanContainWeight is a package-level type here, since
//     Go has no inner classes; the enclosing query is passed explicitly.
//   - Java's abstract equals()/hashCode() pair uses sameClassAs(); Go has no
//     class identity on an embedded struct, so each concrete subclass declares
//     its own Equals with the type assertion and delegates to equalsTo.
type SpanContainQuery struct {
	search.BaseQuery
	Big    SpanQuery
	Little SpanQuery
}

// NewSpanContainQuery constructs a SpanContainQuery.
// Both big and little must have the same field.
//
// Mirrors SpanContainQuery(SpanQuery, SpanQuery).
func NewSpanContainQuery(big, little SpanQuery) (*SpanContainQuery, error) {
	if big == nil || little == nil {
		return nil, fmt.Errorf("SpanContainQuery: big and little must not be nil")
	}
	if big.GetField() != little.GetField() {
		return nil, fmt.Errorf("big and little not same field")
	}
	return &SpanContainQuery{Big: big, Little: little}, nil
}

// GetField returns the field shared by big and little.
func (q *SpanContainQuery) GetField() string { return q.Big.GetField() }

// GetBig returns the big span query.
func (q *SpanContainQuery) GetBig() SpanQuery { return q.Big }

// GetLittle returns the little span query.
func (q *SpanContainQuery) GetLittle() SpanQuery { return q.Little }

// toStringWithName mirrors SpanContainQuery.toString(String field, String name).
func (q *SpanContainQuery) toStringWithName(field, name string) string {
	return fmt.Sprintf("%s(%s, %s)", name, spanQueryToString(q.Big, field), spanQueryToString(q.Little, field))
}

// rewriteSubQueries rewrites big and little and reports whether either changed.
// It carries the body of SpanContainQuery.rewrite(IndexSearcher) up to the
// point where Java clones the concrete subclass, which Go cannot do from the
// embedded base struct: the caller performs the clone.
func (q *SpanContainQuery) rewriteSubQueries(searcher *search.IndexSearcher) (SpanQuery, SpanQuery, bool, error) {
	rewrittenBigQuery, err := q.Big.Rewrite(searcher)
	if err != nil {
		return nil, nil, false, err
	}
	rewrittenBig, ok := rewrittenBigQuery.(SpanQuery)
	if !ok {
		return nil, nil, false, fmt.Errorf("SpanContainQuery.Rewrite: big rewrote to non-SpanQuery %T", rewrittenBigQuery)
	}
	rewrittenLittleQuery, err := q.Little.Rewrite(searcher)
	if err != nil {
		return nil, nil, false, err
	}
	rewrittenLittle, ok := rewrittenLittleQuery.(SpanQuery)
	if !ok {
		return nil, nil, false, fmt.Errorf("SpanContainQuery.Rewrite: little rewrote to non-SpanQuery %T", rewrittenLittleQuery)
	}
	changed := q.Big != rewrittenBig || q.Little != rewrittenLittle
	return rewrittenBig, rewrittenLittle, changed, nil
}

// Visit mirrors SpanContainQuery.visit(QueryVisitor).
func (q *SpanContainQuery) Visit(visitor search.QueryVisitor) {
	if visitor.AcceptField(q.GetField()) {
		v := visitor.GetSubVisitor(search.MUST, q)
		visitSpanQuery(q.Big, v)
		visitSpanQuery(q.Little, v)
	}
}

// equalsTo mirrors SpanContainQuery.equalsTo(SpanContainQuery).
func (q *SpanContainQuery) equalsTo(other *SpanContainQuery) bool {
	return q.Big.Equals(other.Big) && q.Little.Equals(other.Little)
}

// hashCodeWithClassHash mirrors SpanContainQuery.hashCode(), whose seed
// classHash() is the hash of the *concrete* subclass. Go cannot recover that
// from the embedded base struct, so each concrete subclass passes its own
// stable class discriminator — the convention the port already uses for
// Java's classHash() (see search/sorted_numeric_doc_values_range_query.go).
func (q *SpanContainQuery) hashCodeWithClassHash(classHash int) int {
	h := rotateLeft32(classHash, 1)
	h ^= q.Big.HashCode()
	h = rotateLeft32(h, 1)
	h ^= q.Little.HashCode()
	return h
}

// SpanContainWeight is the weight shared by SpanContainingQuery and
// SpanWithinQuery.
//
// Mirrors the inner class SpanContainQuery.SpanContainWeight (abstract).
type SpanContainWeight struct {
	*SpanWeight
	bigWeight    *SpanWeight
	littleWeight *SpanWeight
}

// NewSpanContainWeight constructs a SpanContainWeight for the enclosing query.
//
// Mirrors SpanContainWeight(IndexSearcher, Map<Term,TermStates>, SpanWeight,
// SpanWeight, float). getSpans carries the abstract SpanWeight.getSpans that
// the concrete subclass supplies, and isCacheable its isCacheable override.
func NewSpanContainWeight(
	query search.Query,
	field string,
	simScorer search.SimScorer,
	bigWeight, littleWeight *SpanWeight,
	getSpans func(*index.LeafReaderContext, Postings) (Spans, error),
	isCacheable func(*index.LeafReaderContext) bool,
) *SpanContainWeight {
	w := &SpanContainWeight{
		bigWeight:    bigWeight,
		littleWeight: littleWeight,
	}
	w.SpanWeight = NewSpanWeight(query, SpanWeightConfig{
		Field:     field,
		SimScorer: simScorer,
		GetSpans:  getSpans,
		// extractTermStates(contexts) { bigWeight.extractTermStates(contexts);
		//                               littleWeight.extractTermStates(contexts); }
		ExtractStates: func(contexts map[string]*index.TermStates) {
			bigWeight.ExtractTermStates(contexts)
			littleWeight.ExtractTermStates(contexts)
		},
		IsCacheable: isCacheable,
	})
	return w
}

// PrepareConjunction returns the big and little Spans for the given leaf, or
// nil when either of them is absent.
//
// Mirrors SpanContainWeight.prepareConjunction(LeafReaderContext, Postings).
func (w *SpanContainWeight) PrepareConjunction(ctx *index.LeafReaderContext, postings Postings) ([]Spans, error) {
	bigSpans, err := w.bigWeight.GetSpans(ctx, postings)
	if err != nil {
		return nil, err
	}
	if bigSpans == nil {
		return nil, nil
	}
	littleSpans, err := w.littleWeight.GetSpans(ctx, postings)
	if err != nil {
		return nil, err
	}
	if littleSpans == nil {
		return nil, nil
	}
	return []Spans{bigSpans, littleSpans}, nil
}

var _ spi.Query = (*SpanContainQuery)(nil)
