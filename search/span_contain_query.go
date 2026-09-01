// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// SpanContainQuery is the base for queries that match one span containing another.
// This is the Go port of Lucene's org.apache.lucene.queries.spans.SpanContainQuery.
type SpanContainQuery struct {
	BaseSpanQuery
	big    SpanQuery
	little SpanQuery
}

// NewSpanContainQuery constructs a SpanContainQuery.
func NewSpanContainQuery(big, little SpanQuery) *SpanContainQuery {
	if big.GetField() != little.GetField() {
		panic("big and little not same field")
	}
	return &SpanContainQuery{
		BaseSpanQuery: *NewBaseSpanQuery(big.GetField()),
		big:           big,
		little:        little,
	}
}

// GetBig returns the outer span query.
func (q *SpanContainQuery) GetBig() SpanQuery {
	return q.big
}

// GetLittle returns the inner span query.
func (q *SpanContainQuery) GetLittle() SpanQuery {
	return q.little
}

// Rewrite rewrites this query to a more primitive form.
func (q *SpanContainQuery) Rewrite(reader IndexReader) (Query, error) {
	rewrittenBig, err := q.big.Rewrite(reader)
	if err != nil {
		return nil, err
	}
	rewrittenLittle, err := q.little.Rewrite(reader)
	if err != nil {
		return nil, err
	}

	rb := rewrittenBig.(SpanQuery)
	rl := rewrittenLittle.(SpanQuery)

	if rb != q.big || rl != q.little {
		return &SpanContainQuery{
			BaseSpanQuery: *NewBaseSpanQuery(rb.GetField()),
			big:           rb,
			little:        rl,
		}, nil
	}
	return q, nil
}

// SpanContainWeight is the base weight for span containment queries.
type SpanContainWeight struct {
	*SpanWeight
	bigWeight    SpanWeight
	littleWeight SpanWeight
}

func (w *SpanContainWeight) extractTermStates(contexts map[index.Term]*index.TermStates) {
	w.bigWeight.ExtractTermStates(contexts)
	w.littleWeight.ExtractTermStates(contexts)
}

func (q *SpanContainQuery) Equals(other Query) bool {
	if other == nil {
		return false
	}
	if o, ok := other.(*SpanContainQuery); ok {
		return q.big.Equals(o.big) && q.little.Equals(o.little)
	}
	return false
}

func (q *SpanContainQuery) HashCode() int {
	return q.big.HashCode() ^ q.little.HashCode()
}

func (q *SpanContainQuery) String(field string) string {
	return fmt.Sprintf("SpanContainQuery(%s, %s)", q.big.String(field), q.little.String(field))
}

var _ SpanQuery = (*SpanContainQuery)(nil)
