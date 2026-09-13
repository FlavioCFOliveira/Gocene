// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// DoubleValues provides double values for use in queries and sorting.
type DoubleValues interface {
	// DoubleValue returns the double value for the current document.
	DoubleValue() (float64, error)
	// AdvanceExact positions the reader on doc and reports whether the document is present.
	AdvanceExact(doc int) (bool, error)
}

// DoubleValuesSource provides double values for use in queries and sorting.
// Mirrors org.apache.lucene.search.DoubleValuesSource.
type DoubleValuesSource interface {
	// GetValues returns the double values from the given context.
	GetValues(ctx *index.LeafReaderContext, scores DoubleValues) (DoubleValues, error)
	// Field returns the underlying field name.
	Field() string
	// NeedsScores reports whether the source consumes the underlying query's scores.
	NeedsScores() bool
	// IsCacheable reports whether results are safe to cache for this leaf.
	IsCacheable(ctx *index.LeafReaderContext) bool
	// Rewrite returns the rewritten source.
	Rewrite(searcher *IndexSearcher) DoubleValuesSource
}

// basicDoubleValuesSource is the default implementation of DoubleValuesSource.
type basicDoubleValuesSource struct {
	field string
}

// NewDoubleValuesSource creates a new DoubleValuesSource.
func NewDoubleValuesSource(field string) DoubleValuesSource {
	return &basicDoubleValuesSource{field: field}
}

func (s *basicDoubleValuesSource) Field() string { return s.field }

func (s *basicDoubleValuesSource) GetValues(ctx *index.LeafReaderContext, scores DoubleValues) (DoubleValues, error) {
	_ = scores
	if ctx == nil {
		return nil, fmt.Errorf("leaf reader context must not be nil")
	}
	reader := ctx.LeafReader()
	if reader == nil {
		return nil, fmt.Errorf("leaf reader must not be nil")
	}

	dv, err := numericProviderFromContext(ctx, s.field)
	if err != nil {
		return nil, err
	}
	if dv == nil {
		return nil, nil
	}

	return &basicDoubleValues{dv: dv}, nil
}

func (s *basicDoubleValuesSource) NeedsScores() bool { return false }

func (s *basicDoubleValuesSource) IsCacheable(ctx *index.LeafReaderContext) bool { return true }

func (s *basicDoubleValuesSource) Rewrite(searcher *IndexSearcher) DoubleValuesSource {
	return s
}

type basicDoubleValues struct {
	dv index.NumericDocValues
}

func (v *basicDoubleValues) DoubleValue() (float64, error) {
	val, err := v.dv.LongValue()
	if err != nil {
		return 0, err
	}
	return math.Float64frombits(uint64(val)), nil
}

func (v *basicDoubleValues) AdvanceExact(doc int) (bool, error) {
	currentDoc := v.dv.DocID()
	if doc < currentDoc {
		return false, nil
	}
	if currentDoc == doc {
		return true, nil
	}
	next, err := v.dv.Advance(doc)
	if err != nil {
		return false, err
	}
	return next == doc, nil
}
