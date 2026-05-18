// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import "github.com/FlavioCFOliveira/Gocene/index"

// MultiDoubleValuesSource is a per-segment factory of MultiDoubleValues
// iterators. Mirrors org.apache.lucene.facet.MultiDoubleValuesSource.
type MultiDoubleValuesSource interface {
	// GetValues returns the iterator for the supplied leaf reader.
	GetValues(ctx index.LeafReaderContext) (MultiDoubleValues, error)

	// NeedsScores reports whether the source consumes per-document scores.
	NeedsScores() bool

	// IsCacheable reports whether the source's output is cacheable per leaf.
	IsCacheable(ctx index.LeafReaderContext) bool
}

// ConstantMultiDoubleValuesSource wraps a fixed slice of values that every
// document is reported to carry — useful for tests and synthetic sources.
type ConstantMultiDoubleValuesSource struct {
	values []float64
}

// NewConstantMultiDoubleValuesSource builds the constant source.
func NewConstantMultiDoubleValuesSource(values ...float64) *ConstantMultiDoubleValuesSource {
	out := make([]float64, len(values))
	copy(out, values)
	return &ConstantMultiDoubleValuesSource{values: out}
}

// GetValues returns an iterator backed by the constant slice.
func (s *ConstantMultiDoubleValuesSource) GetValues(_ index.LeafReaderContext) (MultiDoubleValues, error) {
	return &sliceDoubleValues{values: s.values}, nil
}

// NeedsScores always returns false.
func (s *ConstantMultiDoubleValuesSource) NeedsScores() bool { return false }

// IsCacheable always returns true.
func (s *ConstantMultiDoubleValuesSource) IsCacheable(_ index.LeafReaderContext) bool { return true }

type sliceDoubleValues struct {
	values []float64
	pos    int
}

func (s *sliceDoubleValues) AdvanceExact(_ int) (bool, error) {
	s.pos = 0
	return len(s.values) > 0, nil
}

func (s *sliceDoubleValues) DocValueCount() int { return len(s.values) }

func (s *sliceDoubleValues) NextValue() (float64, error) {
	if s.pos >= len(s.values) {
		return 0, nil
	}
	v := s.values[s.pos]
	s.pos++
	return v, nil
}
