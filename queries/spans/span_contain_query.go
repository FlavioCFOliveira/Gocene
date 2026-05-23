// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/spans/SpanContainQuery.java

package spans

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// SpanContainQuery is the abstract base for SpanContainingQuery and SpanWithinQuery.
// It holds two sub-queries: big and little, and ensures they share the same field.
//
// Mirrors org.apache.lucene.queries.spans.SpanContainQuery (abstract, package-private).
//
// Deviations from Java:
//   - Java inner class SpanContainWeight is represented as SpanContainWeight in Go.
//   - Rewrite delegates to the embedded BaseQuery when no rewrite is needed.
type SpanContainQuery struct {
	search.BaseQuery
	Big    search.SpanQuery
	Little search.SpanQuery
}

// NewSpanContainQuery constructs a SpanContainQuery.
// Both big and little must have the same field.
func NewSpanContainQuery(big, little search.SpanQuery) (*SpanContainQuery, error) {
	if big == nil || little == nil {
		return nil, fmt.Errorf("SpanContainQuery: big and little must not be nil")
	}
	if big.GetField() != little.GetField() {
		return nil, fmt.Errorf("SpanContainQuery: big (%s) and little (%s) have different fields",
			big.GetField(), little.GetField())
	}
	return &SpanContainQuery{Big: big, Little: little}, nil
}

// GetField returns the field shared by big and little.
func (q *SpanContainQuery) GetField() string { return q.Big.GetField() }

// GetBig returns the big span query.
func (q *SpanContainQuery) GetBig() search.SpanQuery { return q.Big }

// GetLittle returns the little span query.
func (q *SpanContainQuery) GetLittle() search.SpanQuery { return q.Little }

// Rewrite rewrites big and little; if either changed returns a clone with new sub-queries.
func (q *SpanContainQuery) Rewrite(reader search.IndexReader) (search.Query, error) {
	rewrittenBig, err := q.Big.Rewrite(reader)
	if err != nil {
		return nil, err
	}
	rewrittenLittle, err := q.Little.Rewrite(reader)
	if err != nil {
		return nil, err
	}
	newBig, okBig := rewrittenBig.(search.SpanQuery)
	newLittle, okLittle := rewrittenLittle.(search.SpanQuery)
	if !okBig {
		return nil, fmt.Errorf("SpanContainQuery.Rewrite: big rewrite returned non-SpanQuery %T", rewrittenBig)
	}
	if !okLittle {
		return nil, fmt.Errorf("SpanContainQuery.Rewrite: little rewrite returned non-SpanQuery %T", rewrittenLittle)
	}
	if q.Big != newBig || q.Little != newLittle {
		clone := &SpanContainQuery{Big: newBig, Little: newLittle}
		return clone, nil
	}
	return q, nil
}

// Equals reports whether this query equals other.
func (q *SpanContainQuery) Equals(other search.Query) bool {
	o, ok := other.(*SpanContainQuery)
	if !ok {
		return false
	}
	return q.Big.Equals(o.Big) && q.Little.Equals(o.Little)
}

// HashCode returns a hash code.
func (q *SpanContainQuery) HashCode() int {
	h := 17
	h = h<<1 ^ q.Big.HashCode()
	h = h<<1 ^ q.Little.HashCode()
	return h
}

// stringContain is a helper used by SpanContainingQuery.String and SpanWithinQuery.String.
func stringContain(field, name string, big, little search.SpanQuery) string {
	return fmt.Sprintf("%s(%s, %s)", name, big.String(field), little.String(field))
}
