// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// GlobalOrdinalsWithScoreQuery matches "to" documents whose join-field
// ordinals intersect the collected ordinals, propagating the per-ordinal
// scores from a GlobalOrdinalsWithScoreCollector.
//
// Mirrors org.apache.lucene.search.join.GlobalOrdinalsWithScoreQuery.
type GlobalOrdinalsWithScoreQuery struct {
	collector            *GlobalOrdinalsWithScoreCollector
	scoreMode            ScoreMode
	joinField            string
	globalOrds           *index.OrdinalMap
	toQuery              search.Query
	fromQuery            search.Query
	min                  int
	max                  int
	indexReaderContextID interface{}
}

// NewGlobalOrdinalsWithScoreQuery creates a GlobalOrdinalsWithScoreQuery.
func NewGlobalOrdinalsWithScoreQuery(
	collector *GlobalOrdinalsWithScoreCollector,
	scoreMode ScoreMode,
	joinField string,
	globalOrds *index.OrdinalMap,
	toQuery search.Query,
	fromQuery search.Query,
	min, max int,
	indexReaderContextID interface{},
) *GlobalOrdinalsWithScoreQuery {
	return &GlobalOrdinalsWithScoreQuery{
		collector:            collector,
		scoreMode:            scoreMode,
		joinField:            joinField,
		globalOrds:           globalOrds,
		toQuery:              toQuery,
		fromQuery:            fromQuery,
		min:                  min,
		max:                  max,
		indexReaderContextID: indexReaderContextID,
	}
}

// GetJoinField returns the join field name.
func (q *GlobalOrdinalsWithScoreQuery) GetJoinField() string { return q.joinField }

// GetScoreMode returns the join ScoreMode.
func (q *GlobalOrdinalsWithScoreQuery) GetScoreMode() ScoreMode { return q.scoreMode }

// String implements search.Query.
func (q *GlobalOrdinalsWithScoreQuery) String() string {
	return fmt.Sprintf(
		"GlobalOrdinalsWithScoreQuery{joinField=%s,min=%d,max=%d,fromQuery=%v}",
		q.joinField, q.min, q.max, q.fromQuery,
	)
}

// Rewrite implements search.Query.
func (q *GlobalOrdinalsWithScoreQuery) Rewrite(_ search.IndexReader) (search.Query, error) {
	return q, nil
}

// Clone implements search.Query.
func (q *GlobalOrdinalsWithScoreQuery) Clone() search.Query {
	cp := *q
	return &cp
}

// Equals implements search.Query.
func (q *GlobalOrdinalsWithScoreQuery) Equals(other search.Query) bool {
	o, ok := other.(*GlobalOrdinalsWithScoreQuery)
	if !ok {
		return false
	}
	if q.min != o.min || q.max != o.max || q.scoreMode != o.scoreMode {
		return false
	}
	if q.joinField != o.joinField {
		return false
	}
	if q.indexReaderContextID != o.indexReaderContextID {
		return false
	}
	return true
}

// HashCode implements search.Query.
func (q *GlobalOrdinalsWithScoreQuery) HashCode() int {
	h := 31
	for _, c := range q.joinField {
		h = 31*h + int(c)
	}
	h = 31*h + int(q.scoreMode) + q.min + q.max
	return h
}

// CreateWeight implements search.Query.
func (q *GlobalOrdinalsWithScoreQuery) CreateWeight(_ *search.IndexSearcher, _ bool, boost float32) (search.Weight, error) {
	return &globalOrdinalsWithScoreWeight{
		BaseWeight: search.NewBaseWeight(q),
		query:      q,
		boost:      boost,
	}, nil
}

// globalOrdinalsWithScoreWeight is the Weight for GlobalOrdinalsWithScoreQuery.
type globalOrdinalsWithScoreWeight struct {
	*search.BaseWeight
	query *GlobalOrdinalsWithScoreQuery
	boost float32
}

func (w *globalOrdinalsWithScoreWeight) Scorer(ctx *index.LeafReaderContext) (search.Scorer, error) {
	values, err := getSortedDocValues(ctx, w.query.joinField)
	if err != nil {
		return nil, fmt.Errorf("globalOrdinalsWithScoreQuery: get SortedDocValues: %w", err)
	}
	if values == nil {
		return nil, nil
	}
	collector := w.query.collector
	globalOrds := w.query.globalOrds
	boost := w.boost
	var segToGlobal []int64
	if globalOrds != nil && ctx != nil {
		segToGlobal = globalOrds.GetGlobalOrds(ctx.Ord())
	}

	return &globalOrdinalsWithScoreScorer{
		values:      values,
		collector:   collector,
		segToGlobal: segToGlobal,
		boost:       boost,
		currentDoc:  -1,
	}, nil
}

func (w *globalOrdinalsWithScoreWeight) ScorerSupplier(_ *index.LeafReaderContext) (search.ScorerSupplier, error) {
	return nil, nil
}

func (w *globalOrdinalsWithScoreWeight) BulkScorer(_ *index.LeafReaderContext) (search.BulkScorer, error) {
	return nil, nil
}

func (w *globalOrdinalsWithScoreWeight) IsCacheable(_ *index.LeafReaderContext) bool {
	// Disabled: holds per-ordinal scores from a top-level collector.
	return false
}

func (w *globalOrdinalsWithScoreWeight) Explain(_ *index.LeafReaderContext, _ int) (search.Explanation, error) {
	return search.NewExplanation(false, 0, "GlobalOrdinalsWithScoreWeight stub"), nil
}

func (w *globalOrdinalsWithScoreWeight) Count(_ *index.LeafReaderContext) (int, error) {
	return -1, nil
}

func (w *globalOrdinalsWithScoreWeight) Matches(_ *index.LeafReaderContext, _ int) (search.Matches, error) {
	return nil, nil
}

// globalOrdinalsWithScoreScorer iterates matching "to" documents and returns
// the per-ordinal score from the GlobalOrdinalsWithScoreCollector.
type globalOrdinalsWithScoreScorer struct {
	values      index.SortedDocValues
	collector   *GlobalOrdinalsWithScoreCollector
	segToGlobal []int64
	boost       float32
	currentDoc  int
	curScore    float32
}

func (s *globalOrdinalsWithScoreScorer) advance(target int) (int, error) {
	for {
		doc, err := s.values.Advance(target)
		if err != nil || doc == search.NO_MORE_DOCS {
			s.currentDoc = search.NO_MORE_DOCS
			return search.NO_MORE_DOCS, err
		}
		segOrd, err := s.values.GetOrd(doc)
		if err != nil || segOrd < 0 {
			target = doc + 1
			continue
		}
		globalOrd := segOrd
		if s.segToGlobal != nil && segOrd < len(s.segToGlobal) {
			globalOrd = int(s.segToGlobal[segOrd])
		}
		if s.collector.Match(globalOrd) {
			s.currentDoc = doc
			s.curScore = s.collector.Score(globalOrd) * s.boost
			return doc, nil
		}
		target = doc + 1
	}
}

func (s *globalOrdinalsWithScoreScorer) Score() float32            { return s.curScore }
func (s *globalOrdinalsWithScoreScorer) GetMaxScore(_ int) float32 { return float32(math.Inf(1)) }
func (s *globalOrdinalsWithScoreScorer) DocID() int                { return s.currentDoc }
func (s *globalOrdinalsWithScoreScorer) NextDoc() (int, error) {
	return s.advance(s.currentDoc + 1)
}
func (s *globalOrdinalsWithScoreScorer) Advance(target int) (int, error) { return s.advance(target) }
func (s *globalOrdinalsWithScoreScorer) Cost() int64                     { return 0 }
func (s *globalOrdinalsWithScoreScorer) DocIDRunEnd() int                { return s.currentDoc + 1 }

var _ search.Query = (*GlobalOrdinalsWithScoreQuery)(nil)
var _ search.Scorer = (*globalOrdinalsWithScoreScorer)(nil)
