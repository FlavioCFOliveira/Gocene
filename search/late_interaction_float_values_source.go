// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "errors"

// LateInteractionFloatValuesSource scores documents by comparing a multi-
// vector query against indexed multi-vectors stored in a BinaryDocValues
// field. It produces a DoubleValuesSource so the score can flow through the
// DoubleValuesSource composition pipeline.
//
// Mirrors org.apache.lucene.search.LateInteractionFloatValuesSource.
type LateInteractionFloatValuesSource struct {
	fieldName   string
	queryVector [][]float32
	similarity  VectorSimilarityFunction
	scoring     MultiVectorSimilarity
}

// NewLateInteractionFloatValuesSource validates queryVector and builds a
// source. similarity defaults to nil (caller must supply); scoring defaults
// to SumMaxSimilarity when nil.
func NewLateInteractionFloatValuesSource(fieldName string, queryVector [][]float32, similarity VectorSimilarityFunction, scoring MultiVectorSimilarity) (*LateInteractionFloatValuesSource, error) {
	if err := validateMultiVector(queryVector); err != nil {
		return nil, err
	}
	if scoring == nil {
		scoring = SumMaxSimilarity{}
	}
	return &LateInteractionFloatValuesSource{
		fieldName:   fieldName,
		queryVector: queryVector,
		similarity:  similarity,
		scoring:     scoring,
	}, nil
}

// FieldName returns the multi-vector field name.
func (s *LateInteractionFloatValuesSource) FieldName() string { return s.fieldName }

// QueryVector returns the query multi-vector (do not mutate).
func (s *LateInteractionFloatValuesSource) QueryVector() [][]float32 { return s.queryVector }

// Score compares the query multi-vector against doc using the configured
// similarity function and scoring strategy.
func (s *LateInteractionFloatValuesSource) Score(doc [][]float32) float32 {
	if s.similarity == nil || s.scoring == nil {
		return 0
	}
	return s.scoring.Compare(s.queryVector, doc, s.similarity)
}

// NeedsScores returns false — late-interaction sources contribute their own
// score and do not require the underlying query's score.
func (s *LateInteractionFloatValuesSource) NeedsScores() bool { return false }

// IsCacheable returns true; the source has no per-leaf state.
func (s *LateInteractionFloatValuesSource) IsCacheable() bool { return true }

// validateMultiVector ensures the query has at least one token vector and
// that all token vectors share the same dimension.
func validateMultiVector(qv [][]float32) error {
	if len(qv) == 0 {
		return errors.New("LateInteractionFloatValuesSource: query vector must not be empty")
	}
	dim := -1
	for i, v := range qv {
		if len(v) == 0 {
			return errors.New("LateInteractionFloatValuesSource: token vector must not be empty")
		}
		if dim < 0 {
			dim = len(v)
			continue
		}
		if len(v) != dim {
			return errors.New("LateInteractionFloatValuesSource: token vectors must share the same dimension")
		}
		_ = i
	}
	return nil
}
