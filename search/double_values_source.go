// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// DoubleValuesSource provides double values for use in queries and sorting.
// This is the Go port of Lucene's org.apache.lucene.search.DoubleValuesSource
// (Lucene 10.4.0).
//
// Values are decoded from NumericDocValues using the canonical IEEE-754
// long-bits representation written by document.PackDouble (low-level
// numeric doc-values are stored as int64s for both long and double).
type DoubleValuesSource struct {
	field string
}

// NewDoubleValuesSource creates a new DoubleValuesSource.
func NewDoubleValuesSource(field string) *DoubleValuesSource {
	return &DoubleValuesSource{field: field}
}

// Field returns the underlying field name.
func (s *DoubleValuesSource) Field() string { return s.field }

// GetValues returns the double values from the given context, materialised
// as a slice indexed by document id within the leaf.  See
// LongValuesSource.GetValues for the supported context shapes.
func (s *DoubleValuesSource) GetValues(context interface{}) ([]float64, error) {
	dv, err := numericProviderFromContext(context, s.field)
	if err != nil {
		return nil, err
	}
	if dv == nil {
		return nil, nil
	}
	var size int
	if lrc, ok := context.(*index.LeafReaderContext); ok && lrc != nil {
		if reader := lrc.LeafReader(); reader != nil {
			type maxDocer interface{ MaxDoc() int }
			if md, ok := interface{}(reader).(maxDocer); ok {
				size = md.MaxDoc()
			}
		}
	}
	var out []float64
	if size > 0 {
		out = make([]float64, size)
	}
	// Migrated to LongValue (rmp #4709). NextDoc positions the iterator,
	// LongValue reads the value at the current position. Monotonic.
	for {
		doc, err := dv.NextDoc()
		if err != nil {
			return nil, err
		}
		if doc == NO_MORE_DOCS {
			break
		}
		v, err := dv.LongValue()
		if err != nil {
			return nil, err
		}
		val := math.Float64frombits(uint64(v))
		if doc < len(out) {
			out[doc] = val
			continue
		}
		for len(out) <= doc {
			out = append(out, 0)
		}
		out[doc] = val
	}
	return out, nil
}

// GetSortField returns a SortField for sorting by these values.
func (s *DoubleValuesSource) GetSortField(reverse bool) *SortField {
	sf := NewSortField(s.field, SortFieldTypeDouble)
	sf.Reverse = reverse
	return sf
}

// GetRangeQuery returns a query that matches documents whose value for
// s.field falls within [lower, upper] inclusive.
//
// The bounds are encoded with math.Float64bits so the resulting long-bit
// range query selects documents whose underlying NumericDocValues entries
// decode to a double inside the requested range.  NaN bounds short-circuit
// to MatchNoDocs because Lucene treats NaN as a "no-match" sentinel.
func (s *DoubleValuesSource) GetRangeQuery(lower, upper float64) Query {
	if math.IsNaN(lower) || math.IsNaN(upper) {
		return NewMatchNoDocsQuery()
	}
	if lower > upper {
		return NewMatchNoDocsQuery()
	}
	if math.IsInf(lower, -1) && math.IsInf(upper, +1) {
		return NewFieldExistsQuery(s.field)
	}
	// Encode the bounds as sortable long bits so a NumericDocValuesRangeQuery
	// produces the right doc-id set.  The canonical IEEE-754 ordering is
	// preserved by mapping negative doubles through ^bits while leaving the
	// positives untouched, which is the same flip used by document.PackDouble.
	lo := encodeDoubleToLongBits(lower)
	hi := encodeDoubleToLongBits(upper)
	if lo > hi {
		lo, hi = hi, lo
	}
	return NewNumericDocValuesRangeQuery(s.field, lo, hi)
}

// encodeDoubleToLongBits converts a float64 to the int64 representation
// used by Lucene's PackDouble for sortable long-bit comparisons. The
// sign-flip on negatives makes lexicographic int64 ordering match
// numeric float64 ordering.
func encodeDoubleToLongBits(v float64) int64 {
	bits := int64(math.Float64bits(v))
	if bits < 0 {
		// Flip all bits for negatives.
		bits = ^bits
	} else {
		// Flip the sign bit for non-negatives (top bit of int64).
		const signBit int64 = -1 << 63
		bits ^= signBit
	}
	return bits
}
