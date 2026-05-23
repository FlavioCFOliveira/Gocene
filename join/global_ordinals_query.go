// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// GlobalOrdinalsQuery matches "to" documents whose join-field ordinals
// intersect with the set of ordinals collected from matching "from" documents.
//
// Mirrors org.apache.lucene.search.join.GlobalOrdinalsQuery.
type GlobalOrdinalsQuery struct {
	// foundOrds is the bitset of global ordinals collected from the "from" side.
	foundOrds *util.LongBitSet
	// joinField is the field shared between both sides.
	joinField string
	// globalOrds provides segment-to-global ordinal mapping; nil for single-segment.
	globalOrds *index.OrdinalMap
	// toQuery approximates the candidate "to" documents.
	toQuery search.Query
	// fromQuery is stored for equality/hashcode only.
	fromQuery search.Query
	// indexReaderContextID is the reader context identity at collection time.
	indexReaderContextID interface{}
}

// NewGlobalOrdinalsQuery creates a GlobalOrdinalsQuery.
func NewGlobalOrdinalsQuery(
	foundOrds *util.LongBitSet,
	joinField string,
	globalOrds *index.OrdinalMap,
	toQuery search.Query,
	fromQuery search.Query,
	indexReaderContextID interface{},
) *GlobalOrdinalsQuery {
	return &GlobalOrdinalsQuery{
		foundOrds:            foundOrds,
		joinField:            joinField,
		globalOrds:           globalOrds,
		toQuery:              toQuery,
		fromQuery:            fromQuery,
		indexReaderContextID: indexReaderContextID,
	}
}

// GetFoundOrds returns the bitset of collected global ordinals.
func (q *GlobalOrdinalsQuery) GetFoundOrds() *util.LongBitSet { return q.foundOrds }

// GetJoinField returns the join field name.
func (q *GlobalOrdinalsQuery) GetJoinField() string { return q.joinField }

// GetToQuery returns the approximation query for "to" documents.
func (q *GlobalOrdinalsQuery) GetToQuery() search.Query { return q.toQuery }

// String implements search.Query.
func (q *GlobalOrdinalsQuery) String() string {
	return fmt.Sprintf("GlobalOrdinalsQuery{joinField=%s}", q.joinField)
}

// Rewrite implements search.Query.
func (q *GlobalOrdinalsQuery) Rewrite(_ search.IndexReader) (search.Query, error) {
	return q, nil
}

// Clone implements search.Query.
func (q *GlobalOrdinalsQuery) Clone() search.Query {
	cp := *q
	return &cp
}

// Equals implements search.Query.
func (q *GlobalOrdinalsQuery) Equals(other search.Query) bool {
	o, ok := other.(*GlobalOrdinalsQuery)
	if !ok {
		return false
	}
	if q.joinField != o.joinField {
		return false
	}
	if q.indexReaderContextID != o.indexReaderContextID {
		return false
	}
	fromEq := q.fromQuery == o.fromQuery ||
		(q.fromQuery != nil && o.fromQuery != nil && q.fromQuery.Equals(o.fromQuery))
	toEq := q.toQuery == o.toQuery ||
		(q.toQuery != nil && o.toQuery != nil && q.toQuery.Equals(o.toQuery))
	return fromEq && toEq
}

// HashCode implements search.Query.
func (q *GlobalOrdinalsQuery) HashCode() int {
	h := 31
	for _, c := range q.joinField {
		h = 31*h + int(c)
	}
	if q.toQuery != nil {
		h = 31*h + q.toQuery.HashCode()
	}
	if q.fromQuery != nil {
		h = 31*h + q.fromQuery.HashCode()
	}
	return h
}

// CreateWeight implements search.Query.
func (q *GlobalOrdinalsQuery) CreateWeight(_ *search.IndexSearcher, _ bool, boost float32) (search.Weight, error) {
	return &globalOrdinalsQueryWeight{
		BaseWeight: search.NewBaseWeight(q),
		query:      q,
		boost:      boost,
	}, nil
}

// globalOrdinalsQueryWeight is the Weight for GlobalOrdinalsQuery.
type globalOrdinalsQueryWeight struct {
	*search.BaseWeight
	query *GlobalOrdinalsQuery
	boost float32
}

func (w *globalOrdinalsQueryWeight) Scorer(ctx *index.LeafReaderContext) (search.Scorer, error) {
	return newGlobalOrdinalsStubScorer(), nil
}

func (w *globalOrdinalsQueryWeight) ScorerSupplier(_ *index.LeafReaderContext) (search.ScorerSupplier, error) {
	return nil, nil
}

func (w *globalOrdinalsQueryWeight) BulkScorer(_ *index.LeafReaderContext) (search.BulkScorer, error) {
	return nil, nil
}

func (w *globalOrdinalsQueryWeight) IsCacheable(_ *index.LeafReaderContext) bool {
	// Disabled: holds a bitset of matching ordinals from a top-level reader.
	return false
}

func (w *globalOrdinalsQueryWeight) Explain(_ *index.LeafReaderContext, _ int) (search.Explanation, error) {
	return search.NewExplanation(false, 0, "GlobalOrdinalsQueryWeight stub"), nil
}

func (w *globalOrdinalsQueryWeight) Count(_ *index.LeafReaderContext) (int, error) { return -1, nil }

func (w *globalOrdinalsQueryWeight) Matches(_ *index.LeafReaderContext, _ int) (search.Matches, error) {
	return nil, nil
}

// globalOrdinalsStubScorer is a no-match placeholder.
type globalOrdinalsStubScorer struct {
	*search.BaseDocIdSetIterator
}

func newGlobalOrdinalsStubScorer() *globalOrdinalsStubScorer {
	return &globalOrdinalsStubScorer{BaseDocIdSetIterator: &search.BaseDocIdSetIterator{}}
}

func (s *globalOrdinalsStubScorer) Score() float32        { return 0 }
func (s *globalOrdinalsStubScorer) GetMaxScore(_ int) float32 { return 0 }
func (s *globalOrdinalsStubScorer) DocID() int               { return search.NO_MORE_DOCS }
func (s *globalOrdinalsStubScorer) NextDoc() (int, error)    { return search.NO_MORE_DOCS, nil }
func (s *globalOrdinalsStubScorer) Advance(_ int) (int, error) {
	return search.NO_MORE_DOCS, nil
}
func (s *globalOrdinalsStubScorer) Cost() int64      { return 0 }
func (s *globalOrdinalsStubScorer) DocIDRunEnd() int { return search.NO_MORE_DOCS }

var _ search.Query  = (*GlobalOrdinalsQuery)(nil)
var _ search.Scorer = (*globalOrdinalsStubScorer)(nil)
