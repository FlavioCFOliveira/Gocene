// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
)

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

// LongToDoubleFunc decodes a raw long doc-value into a double. Mirrors Java's
// java.util.function.LongToDoubleFunction passed to
// MultiDoubleValuesSource.fromField.
type LongToDoubleFunc func(int64) float64

// NewMultiDoubleValuesSourceFromField returns a MultiDoubleValuesSource that
// reads the named field's SortedNumericDocValues per leaf and decodes each raw
// long into a double via decoder, preserving the stored order. Mirrors
// org.apache.lucene.facet.MultiDoubleValuesSource.fromField.
func NewMultiDoubleValuesSourceFromField(field string, decoder LongToDoubleFunc) MultiDoubleValuesSource {
	return &fieldMultiDoubleValuesSource{field: field, decoder: decoder}
}

// NewMultiDoubleValuesSourceFromLongField wraps a long-valued field, casting
// each stored long to a double. Mirrors
// MultiDoubleValuesSource.fromLongField (and fromIntField, which delegates).
func NewMultiDoubleValuesSourceFromLongField(field string) MultiDoubleValuesSource {
	return NewMultiDoubleValuesSourceFromField(field, func(v int64) float64 { return float64(v) })
}

// NewMultiDoubleValuesSourceFromDoubleField wraps a double-valued field whose
// values were stored via Double.doubleToRawLongBits. Mirrors
// MultiDoubleValuesSource.fromDoubleField (Double::longBitsToDouble).
func NewMultiDoubleValuesSourceFromDoubleField(field string) MultiDoubleValuesSource {
	return NewMultiDoubleValuesSourceFromField(field, func(v int64) float64 {
		return math.Float64frombits(uint64(v))
	})
}

// NewMultiDoubleValuesSourceFromFloatField wraps a float-valued field whose
// values were stored via Float.floatToRawIntBits. Mirrors
// MultiDoubleValuesSource.fromFloatField (Float.intBitsToFloat).
func NewMultiDoubleValuesSourceFromFloatField(field string) MultiDoubleValuesSource {
	return NewMultiDoubleValuesSourceFromField(field, func(v int64) float64 {
		return float64(math.Float32frombits(uint32(v)))
	})
}

// fieldMultiDoubleValuesSource is the field-backed MultiDoubleValuesSource.
// Mirrors the private FieldMultiValuedSource in Lucene's
// MultiDoubleValuesSource.
type fieldMultiDoubleValuesSource struct {
	field   string
	decoder LongToDoubleFunc
}

// GetValues obtains the field's SortedNumericDocValues for the leaf and wraps
// it in a decoding MultiDoubleValues iterator.
func (s *fieldMultiDoubleValuesSource) GetValues(ctx index.LeafReaderContext) (MultiDoubleValues, error) {
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
		return NewEmptyMultiDoubleValues(), nil
	}
	return &sortedNumericMultiDoubleValues{dv: dv, decoder: s.decoder}, nil
}

// NeedsScores always returns false; the field source never consumes scores.
func (s *fieldMultiDoubleValuesSource) NeedsScores() bool { return false }

// IsCacheable reports whether the source's output is cacheable per leaf. Field
// doc values are deterministic per segment, so this is always true.
func (s *fieldMultiDoubleValuesSource) IsCacheable(_ index.LeafReaderContext) bool { return true }

// sortedNumericMultiDoubleValues adapts a SortedNumericDocValues iterator to
// the MultiDoubleValues contract, decoding each raw long via decoder. Mirrors
// the anonymous MultiDoubleValues returned by FieldMultiValuedSource.getValues.
type sortedNumericMultiDoubleValues struct {
	dv      index.SortedNumericDocValues
	decoder LongToDoubleFunc
	count   int
}

// AdvanceExact positions the underlying doc-values iterator on docID and caches
// the value count for DocValueCount. Returns false when the document has no
// value.
func (m *sortedNumericMultiDoubleValues) AdvanceExact(docID int) (bool, error) {
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
func (m *sortedNumericMultiDoubleValues) DocValueCount() int { return m.count }

// NextValue returns the next stored value for the current document, decoded
// into a double via the source's decoder.
func (m *sortedNumericMultiDoubleValues) NextValue() (float64, error) {
	v, err := m.dv.NextValue()
	if err != nil {
		return 0, err
	}
	return m.decoder(v), nil
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
