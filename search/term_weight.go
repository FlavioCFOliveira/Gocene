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
func (w *TermWeight) Scorer(reader index.IndexReaderInterface) (Scorer, error) {
	// Get the leaf contexts
	leaves, err := reader.Leaves()
	if err != nil {
		return nil, err
	}

	// Try to get terms from the first leaf reader
	for _, leaf := range leaves {
		leafReader := leaf.LeafReader()
		if leafReader == nil {
			continue
		}

		// Get the terms for the field
		terms, err := leafReader.Terms(w.term.Field)
		if err != nil {
			return nil, err
		}
		if terms == nil {
			continue
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

	return nil, nil
}

// GetValueForNormalization returns the value for normalization.
func (w *TermWeight) GetValueForNormalization() float32 {
	return 1.0
}

// Normalize normalizes this weight.
func (w *TermWeight) Normalize(norm float32) {}

// Ensure TermWeight implements Weight
var _ Weight = (*TermWeight)(nil)
