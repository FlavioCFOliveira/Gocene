// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// TermWeight is the Weight implementation for TermQuery.
//
// This is the Go port of Lucene's org.apache.lucene.search.TermWeight.
type TermWeight struct {
	*BaseWeight
	term       *index.Term
	simScorer  SimScorer
	similarity Similarity
}

// NewTermWeight creates a new TermWeight.
func NewTermWeight(query Query, term *index.Term, searcher *IndexSearcher, needsScores bool) *TermWeight {
	w := &TermWeight{
		BaseWeight: NewBaseWeight(query),
		term:       term,
	}

	if needsScores {
		// Get collection statistics
		collectionStats := w.getCollectionStats(searcher)
		// Get term statistics
		termStats := w.getTermStats(searcher)
		// Create the similarity scorer
		w.similarity = NewClassicSimilarity()
		w.simScorer = w.similarity.Scorer(collectionStats, termStats)
	}

	return w
}

// getCollectionStats returns collection statistics for the term's field.
func (w *TermWeight) getCollectionStats(searcher *IndexSearcher) *CollectionStatistics {
	reader := searcher.GetIndexReader()
	return NewCollectionStatistics(w.term.Field, reader.MaxDoc(), reader.NumDocs(), -1, -1)
}

// getTermStats returns term statistics.
func (w *TermWeight) getTermStats(searcher *IndexSearcher) *TermStatistics {
	// Get doc freq for the term
	docFreq := 0
	if reader, ok := searcher.GetIndexReader().(index.IndexReaderInterface); ok {
		if leafReader, ok := reader.(*index.LeafReader); ok {
			terms, err := leafReader.Terms(w.term.Field)
			if err == nil && terms != nil {
				docFreq, _ = terms.GetDocCount()
			}
		}
	}
	return NewTermStatistics(w.term, docFreq, -1)
}

// Scorer creates a scorer for this weight.
func (w *TermWeight) Scorer(context *index.LeafReaderContext) (Scorer, error) {
	leafReader := context.LeafReader()
	if leafReader == nil {
		return nil, nil
	}

	// Get the terms for the field
	terms, err := leafReader.Terms(w.term.Field)
	if err != nil {
		return nil, err
	}
	if terms == nil {
		return nil, nil
	}

	// Get the terms enum iterator
	termsEnum, err := terms.GetIterator()
	if err != nil {
		return nil, err
	}

	// Seek to the term
	found, err := termsEnum.SeekExact(w.term)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}

	// Get the postings enum for the term
	postingsEnum, err := termsEnum.Postings(0)
	if err != nil {
		return nil, err
	}
	if postingsEnum == nil {
		return nil, nil
	}

	// Create and return the scorer
	return NewTermScorer(w, postingsEnum, w.simScorer), nil
}

// ScorerSupplier creates a scorer supplier for this weight.
func (w *TermWeight) ScorerSupplier(context *index.LeafReaderContext) (ScorerSupplier, error) {
	scorer, err := w.Scorer(context)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return NewScorerSupplierAdapter(scorer), nil
}

// Explain returns an explanation of the score for the given document.
func (w *TermWeight) Explain(context *index.LeafReaderContext, doc int) (Explanation, error) {
	return NewExplanation(false, 0, "TermWeight explanation not implemented"), nil
}

// BulkScorer creates a bulk scorer for efficient bulk scoring.
func (w *TermWeight) BulkScorer(context *index.LeafReaderContext) (BulkScorer, error) {
	scorer, err := w.Scorer(context)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return NewDefaultBulkScorer(scorer), nil
}

// IsCacheable returns true if this weight can be cached for the given leaf.
func (w *TermWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return true
}

// Count returns the count of matching documents in sub-linear time.
func (w *TermWeight) Count(context *index.LeafReaderContext) (int, error) {
	return -1, nil
}

// Matches returns the matches for a specific document.
func (w *TermWeight) Matches(context *index.LeafReaderContext, doc int) (Matches, error) {
	return nil, nil
}

// Ensure TermWeight implements Weight
var _ Weight = (*TermWeight)(nil)

// ScorerSupplierAdapter adapts a Scorer to a ScorerSupplier.
type ScorerSupplierAdapter struct {
	scorer Scorer
}

// NewScorerSupplierAdapter creates a new ScorerSupplierAdapter.
func NewScorerSupplierAdapter(scorer Scorer) *ScorerSupplierAdapter {
	return &ScorerSupplierAdapter{scorer: scorer}
}

// Get returns the scorer.
func (s *ScorerSupplierAdapter) Get(leadCost int64) (Scorer, error) {
	return s.scorer, nil
}

// Cost returns the estimated cost.
func (s *ScorerSupplierAdapter) Cost() int64 {
	if s.scorer == nil {
		return 0
	}
	return s.scorer.Cost()
}

// SetTopLevelScoringClause is a no-op.
func (s *ScorerSupplierAdapter) SetTopLevelScoringClause() {}

// Ensure ScorerSupplierAdapter implements ScorerSupplier
var _ ScorerSupplier = (*ScorerSupplierAdapter)(nil)
