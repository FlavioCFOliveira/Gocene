// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// PerFieldSimilarityWrapper allows different Similarity implementations per field.
// This is the Go port of Lucene's org.apache.lucene.search.similarities.PerFieldSimilarityWrapper.
type PerFieldSimilarityWrapper struct {
	*BaseSimilarity
	defaultSimilarity Similarity
	fieldSimilarities map[string]Similarity
}

// NewPerFieldSimilarityWrapper creates a new PerFieldSimilarityWrapper.
func NewPerFieldSimilarityWrapper(defaultSimilarity Similarity) *PerFieldSimilarityWrapper {
	return &PerFieldSimilarityWrapper{
		BaseSimilarity:    NewBaseSimilarity(),
		defaultSimilarity: defaultSimilarity,
		fieldSimilarities: make(map[string]Similarity),
	}
}

// GetFieldSimilarity returns the Similarity for a field.
func (s *PerFieldSimilarityWrapper) GetFieldSimilarity(field string) Similarity {
	if sim, ok := s.fieldSimilarities[field]; ok {
		return sim
	}
	return s.defaultSimilarity
}

// SetFieldSimilarity sets the Similarity for a field.
func (s *PerFieldSimilarityWrapper) SetFieldSimilarity(field string, similarity Similarity) {
	s.fieldSimilarities[field] = similarity
}

// ComputeNorm computes the norm value for a field.
func (s *PerFieldSimilarityWrapper) ComputeNorm(field string, stats interface{}) float32 {
	return s.GetFieldSimilarity(field).ComputeNorm(field, stats)
}

// ComputeWeight computes the weight for a query.
func (s *PerFieldSimilarityWrapper) ComputeWeight(boost float32, collectionStats *CollectionStatistics, termStats *TermStatistics) SimWeight {
	return s.defaultSimilarity.ComputeWeight(boost, collectionStats, termStats)
}

// Scorer creates a SimScorer for scoring documents.
func (s *PerFieldSimilarityWrapper) Scorer(collectionStats *CollectionStatistics, termStats *TermStatistics) SimScorer {
	return s.defaultSimilarity.Scorer(collectionStats, termStats)
}

// Ensure PerFieldSimilarityWrapper implements Similarity
var _ Similarity = (*PerFieldSimilarityWrapper)(nil)
