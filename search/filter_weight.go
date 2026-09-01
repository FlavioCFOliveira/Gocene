// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "github.com/FlavioCFOliveira/Gocene/index"

// FilterWeight contains another Weight and implements all methods by
// calling the contained weight's method.
// This is the Go port of Lucene's org.apache.lucene.search.FilterWeight.
type FilterWeight struct {
	*BaseWeight
	in Weight
}

// NewFilterWeight creates a new FilterWeight with the given weight.
func NewFilterWeight(in Weight) *FilterWeight {
	return NewFilterWeightWithQuery(in.GetQuery(), in)
}

// NewFilterWeightWithQuery creates a new FilterWeight with the given query and weight.
func NewFilterWeightWithQuery(query Query, in Weight) *FilterWeight {
	return &FilterWeight{
		BaseWeight: NewBaseWeight(query),
		in:         in,
	}
}

// Explain returns an explanation of the score for the given document by delegating to the wrapped weight.
func (fw *FilterWeight) Explain(context *index.LeafReaderContext, doc int) (Explanation, error) {
	return fw.in.Explain(context, doc)
}

// ScorerSupplier creates a ScorerSupplier for this weight by delegating to the wrapped weight.
func (fw *FilterWeight) ScorerSupplier(context *index.LeafReaderContext) (ScorerSupplier, error) {
	return fw.in.ScorerSupplier(context)
}

// IsCacheable returns true if this weight can be cached for the given leaf by delegating to the wrapped weight.
func (fw *FilterWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return fw.in.IsCacheable(ctx)
}

// Count returns the count of matching documents in sub-linear time by delegating to the wrapped weight.
func (fw *FilterWeight) Count(context *index.LeafReaderContext) (int, error) {
	return fw.in.Count(context)
}

// Matches returns the matches for a specific document by delegating to the wrapped weight.
func (fw *FilterWeight) Matches(context *index.LeafReaderContext, doc int) (Matches, error) {
	return fw.in.Matches(context, doc)
}
