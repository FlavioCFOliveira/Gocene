// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// MultiLongValuesSource is a per-segment factory of MultiLongValues
// iterators. Mirrors org.apache.lucene.facet.MultiLongValuesSource.
type MultiLongValuesSource interface {
	GetValues(ctx index.LeafReaderContext) (MultiLongValues, error)
	NeedsScores() bool
	IsCacheable(ctx index.LeafReaderContext) bool
}

// sortedNumericProvider is the subset of the leaf-reader surface needed to
// obtain a field's SortedNumericDocValues. *index.SegmentReader and
// *index.LeafReader both satisfy it.
type sortedNumericProvider interface {
	GetSortedNumericDocValues(field string) (index.SortedNumericDocValues, error)
}

// NewMultiLongValuesSourceFromField returns a MultiLongValuesSource that reads
// the named field's SortedNumericDocValues per leaf, exposing the stored values
// in their on-disk order. Mirrors
// org.apache.lucene.facet.MultiLongValuesSource.fromField.
func NewMultiLongValuesSourceFromField(field string) MultiLongValuesSource {
	return &fieldMultiLongValuesSource{field: field}
}

// fieldMultiLongValuesSource is the field-backed MultiLongValuesSource.
// Mirrors the private FieldMultiValueSource in Lucene's MultiLongValuesSource.
type fieldMultiLongValuesSource struct {
	field string
}

// GetValues obtains the field's SortedNumericDocValues for the leaf and wraps
// it in a MultiLongValues iterator.
func (s *fieldMultiLongValuesSource) GetValues(ctx index.LeafReaderContext) (MultiLongValues, error) {
	reader := ctx.LeafReader()
	provider, ok := reader.(sortedNumericProvider)
	if !ok {
		return nil, fmt.Errorf("leaf reader %T does not expose SortedNumericDocValues", reader)
	}
	dv, err := provider.GetSortedNumericDocValues(s.field)
	if err != nil {
		return nil, fmt.Errorf("get sorted-numeric doc values for %q: %w", s.field, err)
	}
	if dv == nil {
		return NewEmptyMultiLongValues(), nil
	}
	return &sortedNumericMultiLongValues{dv: dv}, nil
}

// NeedsScores always returns false; the field source never consumes scores.
func (s *fieldMultiLongValuesSource) NeedsScores() bool { return false }

// IsCacheable reports whether the source's output is cacheable per leaf. Field
// doc values are deterministic per segment, so this is always true.
func (s *fieldMultiLongValuesSource) IsCacheable(_ index.LeafReaderContext) bool { return true }

// sortedNumericMultiLongValues adapts a SortedNumericDocValues iterator to the
// MultiLongValues contract. Mirrors the anonymous MultiLongValues returned by
// FieldMultiValueSource.getValues.
type sortedNumericMultiLongValues struct {
	dv    index.SortedNumericDocValues
	count int
}

// AdvanceExact positions the underlying doc-values iterator on docID and caches
// the value count for DocValueCount. Returns false when the document has no
// value.
func (m *sortedNumericMultiLongValues) AdvanceExact(docID int) (bool, error) {
	ok, err := m.dv.AdvanceExact(docID)
	if err != nil {
		return false, err
	}
	if !ok {
		m.count = 0
		return false, nil
	}
	cnt, err := m.dv.DocValueCount()
	if err != nil {
		return false, err
	}
	m.count = cnt
	return true, nil
}

// DocValueCount returns the number of values for the document positioned by the
// last successful AdvanceExact.
func (m *sortedNumericMultiLongValues) DocValueCount() int { return m.count }

// NextValue returns the next stored value for the current document, in on-disk
// order.
func (m *sortedNumericMultiLongValues) NextValue() (int64, error) { return m.dv.NextValue() }

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
