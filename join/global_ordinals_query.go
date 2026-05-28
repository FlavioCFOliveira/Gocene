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

// sortedDocValuesReader is a local interface so join does not import a
// concrete index type and avoids creating an import cycle.
type sortedDocValuesReader interface {
	GetSortedDocValues(field string) (index.SortedDocValues, error)
}

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

// getSortedDocValues retrieves SortedDocValues from a LeafReader if the
// concrete reader type exposes GetSortedDocValues.  Returns nil (no match)
// when the reader does not support the field or the assertion fails.
func getSortedDocValues(ctx *index.LeafReaderContext, field string) (index.SortedDocValues, error) {
	if ctx == nil {
		return nil, nil
	}
	r := ctx.LeafReader()
	if r == nil {
		return nil, nil
	}
	if dv, ok := r.(sortedDocValuesReader); ok {
		return dv.GetSortedDocValues(field)
	}
	return nil, nil
}

func (w *globalOrdinalsQueryWeight) Scorer(ctx *index.LeafReaderContext) (search.Scorer, error) {
	values, err := getSortedDocValues(ctx, w.query.joinField)
	if err != nil {
		return nil, fmt.Errorf("globalOrdinalsQuery: get SortedDocValues: %w", err)
	}
	if values == nil {
		return nil, nil
	}
	return newGlobalOrdinalsScorer(
		w.boost,
		w.query.foundOrds,
		w.query.globalOrds,
		values,
		ctx.Ord(),
	), nil
}

// newGlobalOrdinalsScorer builds a scorer that matches "to" documents whose
// join-field ordinal is present in foundOrds.
//
// Deviation from Lucene 10.4.0: the Java implementation uses an approximation
// DISI derived from toQuery.createWeight; here we iterate all docs that have a
// SortedDocValues value for the field (any doc with segOrd >= 0) and then
// filter via the two-phase check.  The toQuery approximation optimisation is
// deferred until IndexSearcher wiring is complete.
func newGlobalOrdinalsScorer(
	boost float32,
	foundOrds *util.LongBitSet,
	globalOrds *index.OrdinalMap,
	values index.SortedDocValues,
	segmentOrd int,
) *globalOrdinalsScorer {
	var segToGlobal []int64
	if globalOrds != nil {
		segToGlobal = globalOrds.GetGlobalOrds(segmentOrd)
	}
	return &globalOrdinalsScorer{
		boost:        boost,
		foundOrds:    foundOrds,
		values:       values,
		segToGlobal:  segToGlobal,
		currentScore: boost, // constant-score
		currentDoc:   -1,
	}
}

// globalOrdinalsScorer iterates matching "to" documents by advancing the
// SortedDocValues iterator and checking the foundOrds bitset.
type globalOrdinalsScorer struct {
	boost        float32
	foundOrds    *util.LongBitSet
	values       index.SortedDocValues
	segToGlobal  []int64
	currentScore float32
	currentDoc   int
}

// matches mirrors the Lucene TwoPhaseIterator.matches callback.
//
// Migrated to AdvanceExact + OrdValue (rmp #4709). matches is called
// in monotonic doc order by the TwoPhase iterator.
func (s *globalOrdinalsScorer) matches(docID int) (bool, error) {
	ok, err := s.values.AdvanceExact(docID)
	if err != nil || !ok {
		return false, err
	}
	segOrd, err := s.values.OrdValue()
	if err != nil || segOrd < 0 {
		return false, err
	}
	globalOrd := int64(segOrd)
	if s.segToGlobal != nil && segOrd < len(s.segToGlobal) {
		globalOrd = s.segToGlobal[segOrd]
	}
	return s.foundOrds.Get(globalOrd), nil
}

func (s *globalOrdinalsScorer) Score() float32            { return s.currentScore }
func (s *globalOrdinalsScorer) GetMaxScore(_ int) float32 { return s.boost }
func (s *globalOrdinalsScorer) DocID() int                { return s.currentDoc }

func (s *globalOrdinalsScorer) NextDoc() (int, error) {
	for {
		// Advance the doc-values iterator to the next doc that has a value.
		doc, err := s.values.Advance(s.currentDoc + 1)
		if err != nil {
			return search.NO_MORE_DOCS, err
		}
		if doc == search.NO_MORE_DOCS {
			s.currentDoc = search.NO_MORE_DOCS
			return search.NO_MORE_DOCS, nil
		}
		// Migrated to OrdValue (rmp #4709): Advance positions the
		// iterator at doc, so OrdValue reads the ord at the current
		// position without seeking again.
		segOrd, err := s.values.OrdValue()
		if err != nil || segOrd < 0 {
			s.currentDoc = doc
			continue
		}
		globalOrd := int64(segOrd)
		if s.segToGlobal != nil && segOrd < len(s.segToGlobal) {
			globalOrd = s.segToGlobal[segOrd]
		}
		if s.foundOrds.Get(globalOrd) {
			s.currentDoc = doc
			return doc, nil
		}
		s.currentDoc = doc
	}
}

func (s *globalOrdinalsScorer) Advance(target int) (int, error) {
	s.currentDoc = target - 1
	return s.NextDoc()
}

func (s *globalOrdinalsScorer) Cost() int64      { return s.foundOrds.Length() }
func (s *globalOrdinalsScorer) DocIDRunEnd() int { return s.currentDoc + 1 }

var _ search.Scorer = (*globalOrdinalsScorer)(nil)

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

var _ search.Query = (*GlobalOrdinalsQuery)(nil)
