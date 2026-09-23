// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// MatchNoDocsQuery is a query that matches no documents.
type MatchNoDocsQuery struct {
	BaseQuery
	reason string
}

// MatchNoDocsQueryInstance is a singleton instance with a blank reason.
var MatchNoDocsQueryInstance = NewMatchNoDocsQuery("")

// NewMatchNoDocsQuery creates a new MatchNoDocsQuery.
// Provides a reason explaining why this query was used.
//
// NOTE: All instances of this class are equal, even if they were constructed with distinct
// reasons.
func NewMatchNoDocsQuery(reason string) *MatchNoDocsQuery {
	return &MatchNoDocsQuery{
		reason: reason,
	}
}

func (q *MatchNoDocsQuery) Equals(other spi.Query) bool {
	_, ok := other.(*MatchNoDocsQuery)
	return ok
}

func (q *MatchNoDocsQuery) HashCode() int {
	// Return a constant hash code for this class, mirroring Java's classHash().
	return 0
}

func (q *MatchNoDocsQuery) CreateWeight(searcher *IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error) {
	return &matchNoDocsWeight{
		BaseWeight: NewBaseWeight(q),
	}, nil
}

func (q *MatchNoDocsQuery) Visit(visitor QueryVisitor) {
	visitor.VisitLeaf(q)
}

func (q *MatchNoDocsQuery) String() string {
	return fmt.Sprintf("MatchNoDocsQuery(%q)", q.reason)
}

type matchNoDocsWeight struct {
	*BaseWeight
}

func (w *matchNoDocsWeight) Explain(context *index.LeafReaderContext, doc int) (Explanation, error) {
	q := w.GetQuery().(*MatchNoDocsQuery)
	return NoMatchExplanation(q.reason), nil
}

func (w *matchNoDocsWeight) ScorerSupplier(context *index.LeafReaderContext) (ScorerSupplier, error) {
	return nil, nil
}

func (w *matchNoDocsWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return true
}

func (w *matchNoDocsWeight) Count(context *index.LeafReaderContext) (int, error) {
	return 0, nil
}

// Ensure MatchNoDocsQuery implements Query.
var _ Query = (*MatchNoDocsQuery)(nil)

// Ensure matchNoDocsWeight implements Weight.
var _ Weight = (*matchNoDocsWeight)(nil)

// Rewrite renders the inherited Query.rewrite(IndexSearcher), which returns
// this; MatchNoDocsQuery does not override it.
func (q *MatchNoDocsQuery) Rewrite(searcher *IndexSearcher) (Query, error) {
	return q, nil
}

// Scorer renders the final Weight.scorer(LeafReaderContext): the scorer of
// scorerSupplier(context).get(Long.MAX_VALUE), or nil when no document
// matches. It is restated because the embedded BaseWeight.Scorer would call
// BaseWeight.ScorerSupplier, not this type's override.
func (w *matchNoDocsWeight) Scorer(context *index.LeafReaderContext) (Scorer, error) {
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
func (w *matchNoDocsWeight) BulkScorer(context *index.LeafReaderContext) (BulkScorer, error) {
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
