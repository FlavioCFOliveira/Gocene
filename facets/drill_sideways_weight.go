// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// DrillSidewaysWeight is the Weight implementation for DrillSidewaysQuery.
// It manages the scoring and matching for drill-sideways queries.
type DrillSidewaysWeight struct {
	*search.BaseWeight

	// query is the parent DrillSidewaysQuery
	query *DrillSidewaysQuery

	// baseWeight is the Weight for the base query
	baseWeight search.Weight

	// drillDownWeights are the Weights for each drill-down dimension
	drillDownWeights map[string]search.Weight

	// searcher is the index searcher
	searcher *search.IndexSearcher
}

// NewDrillSidewaysWeight creates a new DrillSidewaysWeight.
func NewDrillSidewaysWeight(query *DrillSidewaysQuery, searcher *search.IndexSearcher, needsScores bool, boost float32) (*DrillSidewaysWeight, error) {
	w := &DrillSidewaysWeight{
		BaseWeight:       search.NewBaseWeight(query),
		query:            query,
		drillDownWeights: make(map[string]search.Weight),
		searcher:         searcher,
	}

	// Create weight for base query
	if query.BaseQuery != nil {
		baseWeight, err := query.BaseQuery.CreateWeight(searcher, needsScores, boost)
		if err != nil {
			return nil, fmt.Errorf("creating base weight: %w", err)
		}
		w.baseWeight = baseWeight
	}

	// Create weights for drill-down queries
	for dim, drillQuery := range query.DrillDownQueries {
		drillWeight, err := drillQuery.CreateWeight(searcher, needsScores, boost)
		if err != nil {
			return nil, fmt.Errorf("creating drill-down weight for %s: %w", dim, err)
		}
		w.drillDownWeights[dim] = drillWeight
	}

	return w, nil
}

// IsCacheable returns true if this weight can be cached.
func (w *DrillSidewaysWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	if w.baseWeight != nil && !w.baseWeight.IsCacheable(ctx) {
		return false
	}
	for _, drillWeight := range w.drillDownWeights {
		if !drillWeight.IsCacheable(ctx) {
			return false
		}
	}
	return true
}

// Scorer creates a DrillSidewaysScorer for the given context.
func (w *DrillSidewaysWeight) Scorer(ctx *index.LeafReaderContext) (search.Scorer, error) {
	// Create base scorer
	var baseScorer search.Scorer
	var err error
	if w.baseWeight != nil {
		baseScorer, err = w.baseWeight.Scorer(ctx)
		if err != nil {
			return nil, fmt.Errorf("creating base scorer: %w", err)
		}
	}

	// Create drill-down scorers
	drillDownScorers := make(map[string]search.Scorer)
	for dim, drillWeight := range w.drillDownWeights {
		drillScorer, err := drillWeight.Scorer(ctx)
		if err != nil {
			return nil, fmt.Errorf("creating drill-down scorer for %s: %w", dim, err)
		}
		drillDownScorers[dim] = drillScorer
	}

	return NewDrillSidewaysScorer(w, baseScorer, drillDownScorers, w.query.DrillDownDimensions), nil
}

// Explain explains the score for the given document.
func (w *DrillSidewaysWeight) Explain(ctx *index.LeafReaderContext, doc int) (search.Explanation, error) {
	if w.baseWeight != nil && ctx != nil {
		return w.baseWeight.Explain(ctx, doc)
	}
	return nil, nil
}

// ScorerSupplier creates a ScorerSupplier for this weight.
func (w *DrillSidewaysWeight) ScorerSupplier(ctx *index.LeafReaderContext) (search.ScorerSupplier, error) {
	return nil, nil
}

// BulkScorer creates a bulk scorer for efficient bulk scoring.
func (w *DrillSidewaysWeight) BulkScorer(ctx *index.LeafReaderContext) (search.BulkScorer, error) {
	scorer, err := w.Scorer(ctx)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return search.NewDefaultBulkScorer(scorer), nil
}

// Count returns the count of matching documents.
func (w *DrillSidewaysWeight) Count(ctx *index.LeafReaderContext) (int, error) {
	return -1, nil
}

// Matches returns the matches for a specific document.
func (w *DrillSidewaysWeight) Matches(ctx *index.LeafReaderContext, doc int) (search.Matches, error) {
	return nil, nil
}

// DrillSidewaysScorer is the Scorer implementation for DrillSidewaysQuery.
// It combines the base query scorer with drill-down scorers.
type DrillSidewaysScorer struct {
	*search.BaseScorer

	// weight is the parent DrillSidewaysWeight
	weight *DrillSidewaysWeight

	// baseScorer is the scorer for the base query
	baseScorer search.Scorer

	// drillDownScorers are the scorers for each drill-down dimension
	drillDownScorers map[string]search.Scorer

	// drillDownDimensions maintains the order of dimensions
	drillDownDimensions []string

	// currentDoc is the current document ID
	currentDoc int
}

// NewDrillSidewaysScorer creates a new DrillSidewaysScorer.
func NewDrillSidewaysScorer(weight *DrillSidewaysWeight, baseScorer search.Scorer, drillDownScorers map[string]search.Scorer, drillDownDimensions []string) *DrillSidewaysScorer {
	return &DrillSidewaysScorer{
		BaseScorer:          search.NewBaseScorer(weight),
		weight:              weight,
		baseScorer:          baseScorer,
		drillDownScorers:    drillDownScorers,
		drillDownDimensions: drillDownDimensions,
		currentDoc:          -1,
	}
}

// NextDoc advances to the next matching document.
func (s *DrillSidewaysScorer) NextDoc() (int, error) {
	if s.baseScorer != nil {
		doc, err := s.baseScorer.NextDoc()
		if err != nil {
			return 0, err
		}
		s.currentDoc = doc
		return doc, nil
	}
	s.currentDoc = search.NO_MORE_DOCS
	return search.NO_MORE_DOCS, nil
}

// DocID returns the current document ID.
func (s *DrillSidewaysScorer) DocID() int {
	return s.currentDoc
}

// Score returns the score for the current document.
func (s *DrillSidewaysScorer) Score() float32 {
	if s.baseScorer != nil {
		return s.baseScorer.Score()
	}
	return 1.0
}

// Advance advances to the target document.
func (s *DrillSidewaysScorer) Advance(target int) (int, error) {
	if s.baseScorer != nil {
		doc, err := s.baseScorer.Advance(target)
		if err != nil {
			return 0, err
		}
		s.currentDoc = doc
		return doc, nil
	}
	s.currentDoc = search.NO_MORE_DOCS
	return search.NO_MORE_DOCS, nil
}

// Cost returns the estimated cost of this scorer.
func (s *DrillSidewaysScorer) Cost() int64 {
	if s.baseScorer != nil {
		return s.baseScorer.Cost()
	}
	return 0
}

// GetMaxScore returns the maximum score for documents up to the given doc.
func (s *DrillSidewaysScorer) GetMaxScore(upTo int) float32 {
	if s.baseScorer != nil {
		return s.baseScorer.GetMaxScore(upTo)
	}
	return 1.0
}

// DocIDRunEnd returns the end of the current run of consecutive doc IDs.
func (s *DrillSidewaysScorer) DocIDRunEnd() int {
	if s.baseScorer != nil {
		// Check if baseScorer has DocIDRunEnd method
		if dsi, ok := s.baseScorer.(interface{ DocIDRunEnd() int }); ok {
			return dsi.DocIDRunEnd()
		}
	}
	return s.currentDoc + 1
}

// Matches returns true if the current document matches all drill-down queries.
func (s *DrillSidewaysScorer) Matches() bool {
	if s.currentDoc == search.NO_MORE_DOCS {
		return false
	}

	// Check if document matches all drill-down dimensions
	for _, dim := range s.drillDownDimensions {
		if scorer, ok := s.drillDownScorers[dim]; ok {
			// Advance drill-down scorer to current doc
			doc, err := scorer.Advance(s.currentDoc)
			if err != nil {
				return false
			}
			if doc != s.currentDoc {
				return false
			}
		}
	}

	return true
}

// GetDrillDownScorer returns the scorer for the given drill-down dimension.
func (s *DrillSidewaysScorer) GetDrillDownScorer(dim string) search.Scorer {
	return s.drillDownScorers[dim]
}

// GetBaseScorer returns the base query scorer.
func (s *DrillSidewaysScorer) GetBaseScorer() search.Scorer {
	return s.baseScorer
}
