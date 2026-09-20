// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"errors"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
)

// LateInteractionFloatValuesSource scores documents by comparing a multi-
// vector query against indexed multi-vectors stored in a BinaryDocValues
// field. It produces a DoubleValuesSource so the score can flow through the
// DoubleValuesSource composition pipeline.
//
// Mirrors org.apache.lucene.search.LateInteractionFloatValuesSource.
type LateInteractionFloatValuesSource struct {
	fieldName   string
	queryVector [][]float32
	similarity  index.VectorSimilarityFunction
	scoring     MultiVectorSimilarity
}

// NewLateInteractionFloatValuesSource validates queryVector and builds a
// source. similarity defaults to nil (caller must supply); scoring defaults
// to SumMaxSimilarity when nil.
func NewLateInteractionFloatValuesSource(fieldName string, queryVector [][]float32, similarity index.VectorSimilarityFunction, scoring MultiVectorSimilarity) (*LateInteractionFloatValuesSource, error) {
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

// Field returns the multi-vector field name.
//
// Apache Lucene 10.5.0 declares no field() on DoubleValuesSource; the accessor
// exists because Gocene's search.DoubleValuesSource carries one, and it returns
// the fieldName the Java class holds privately.
func (s *LateInteractionFloatValuesSource) Field() string { return s.fieldName }

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

// GetValues mirrors LateInteractionFloatValuesSource.getValues: it opens the
// field's BinaryDocValues on the leaf and returns a DoubleValues that decodes
// the stored multi-vector and scores it against the query multi-vector. A leaf
// without the field yields DoubleValues.EMPTY, spelled here as a nil result
// (the convention already used by
// FullPrecisionFloatVectorSimilarityValuesSource.GetValues).
func (s *LateInteractionFloatValuesSource) GetValues(ctx *index.LeafReaderContext, _ DoubleValues) (DoubleValues, error) {
	if ctx == nil {
		return nil, errors.New("LateInteractionFloatValuesSource: leaf reader context must not be nil")
	}
	reader := ctx.LeafReader()
	if reader == nil {
		return nil, errors.New("LateInteractionFloatValuesSource: leaf reader must not be nil")
	}
	values, err := reader.GetBinaryDocValues(s.fieldName)
	if err != nil {
		return nil, err
	}
	if values == nil {
		return nil, nil
	}
	return &lateInteractionDoubleValues{src: s, values: values}, nil
}

// lateInteractionDoubleValues renders the anonymous DoubleValues returned by
// LateInteractionFloatValuesSource.getValues.
type lateInteractionDoubleValues struct {
	src    *LateInteractionFloatValuesSource
	values index.BinaryDocValues
}

// DoubleValue mirrors the anonymous doubleValue(): decode the stored
// multi-vector with LateInteractionField.decode and score it.
func (v *lateInteractionDoubleValues) DoubleValue() (float64, error) {
	payload, err := v.values.BinaryValue()
	if err != nil {
		return 0, err
	}
	docVector, err := document.DecodeLateInteraction(payload)
	if err != nil {
		return 0, err
	}
	return float64(v.src.Score(docVector)), nil
}

// AdvanceExact mirrors the anonymous advanceExact(int).
func (v *lateInteractionDoubleValues) AdvanceExact(doc int) (bool, error) {
	return v.values.AdvanceExact(doc)
}

// NeedsScores returns false — late-interaction sources contribute their own
// score and do not require the underlying query's score.
func (s *LateInteractionFloatValuesSource) NeedsScores() bool { return false }

// Rewrite mirrors LateInteractionFloatValuesSource.rewrite, whose body is
// `return this;`.
func (s *LateInteractionFloatValuesSource) Rewrite(_ *IndexSearcher) DoubleValuesSource {
	return s
}

// IsCacheable returns true; the source has no per-leaf state.
func (s *LateInteractionFloatValuesSource) IsCacheable(_ *index.LeafReaderContext) bool { return true }

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
