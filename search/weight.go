// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// Weight is the internal representation of a query.
type Weight interface {
	// GetQuery returns the parent query.
	GetQuery() Query

	// GetValueForNormalization returns the value for normalization.
	GetValueForNormalization() float32

	// Normalize normalizes this weight.
	Normalize(norm float32)

	// Scorer creates a scorer for this weight.
	Scorer(reader IndexReader) (Scorer, error)
}

// BaseWeight provides common functionality for weights.
type BaseWeight struct {
	query Query
}

// NewBaseWeight creates a new BaseWeight.
func NewBaseWeight(query Query) *BaseWeight {
	return &BaseWeight{query: query}
}

// GetQuery returns the parent query.
func (w *BaseWeight) GetQuery() Query {
	return w.query
}

// GetValueForNormalization returns the value for normalization.
func (w *BaseWeight) GetValueForNormalization() float32 {
	return 1.0
}

// Normalize normalizes this weight.
func (w *BaseWeight) Normalize(norm float32) {}
