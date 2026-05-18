// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import "github.com/FlavioCFOliveira/Gocene/index"

// MultiLongValuesSource is a per-segment factory of MultiLongValues
// iterators. Mirrors org.apache.lucene.facet.MultiLongValuesSource.
type MultiLongValuesSource interface {
	GetValues(ctx index.LeafReaderContext) (MultiLongValues, error)
	NeedsScores() bool
	IsCacheable(ctx index.LeafReaderContext) bool
}

// ConstantMultiLongValuesSource wraps a fixed slice of values.
type ConstantMultiLongValuesSource struct {
	values []int64
}

// NewConstantMultiLongValuesSource builds the constant source.
func NewConstantMultiLongValuesSource(values ...int64) *ConstantMultiLongValuesSource {
	out := make([]int64, len(values))
	copy(out, values)
	return &ConstantMultiLongValuesSource{values: out}
}

// GetValues returns an iterator backed by the constant slice.
func (s *ConstantMultiLongValuesSource) GetValues(_ index.LeafReaderContext) (MultiLongValues, error) {
	return &sliceLongValues{values: s.values}, nil
}

// NeedsScores always returns false.
func (s *ConstantMultiLongValuesSource) NeedsScores() bool { return false }

// IsCacheable always returns true.
func (s *ConstantMultiLongValuesSource) IsCacheable(_ index.LeafReaderContext) bool { return true }

type sliceLongValues struct {
	values []int64
	pos    int
}

func (s *sliceLongValues) AdvanceExact(_ int) (bool, error) {
	s.pos = 0
	return len(s.values) > 0, nil
}

func (s *sliceLongValues) DocValueCount() int { return len(s.values) }

func (s *sliceLongValues) NextValue() (int64, error) {
	if s.pos >= len(s.values) {
		return 0, nil
	}
	v := s.values[s.pos]
	s.pos++
	return v, nil
}
