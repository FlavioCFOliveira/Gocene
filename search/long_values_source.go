// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// LongValuesSource provides long values for use in queries and sorting.
// This is the Go port of Lucene's org.apache.lucene.search.LongValuesSource
// (Lucene 10.4.0).
//
// Values are read from NumericDocValues on the supplied leaf-reader-style
// context. Callers that have only the raw reader (without a wrapping
// LeafReaderContext) may pass the reader directly.
type LongValuesSource struct {
	field string
}

// NewLongValuesSource creates a new LongValuesSource.
func NewLongValuesSource(field string) *LongValuesSource {
	return &LongValuesSource{field: field}
}

// Field returns the underlying field name.
func (s *LongValuesSource) Field() string { return s.field }

// numericProvider is the narrow capability we require from any context
// argument: the ability to obtain a NumericDocValues iterator for the
// configured field.
type numericProvider interface {
	GetNumericDocValues(field string) (index.NumericDocValues, error)
}

// numericProviderFromContext extracts a NumericDocValues iterator from a
// context argument. Accepted shapes:
//
//   - *index.LeafReaderContext (unwraps via LeafReader())
//   - any type exposing GetNumericDocValues(field string)
//
// Returns nil iterator and no error when no provider can be located, which
// matches Lucene's "no docvalues for field" contract.
func numericProviderFromContext(ctx interface{}, field string) (index.NumericDocValues, error) {
	if ctx == nil {
		return nil, nil
	}
	switch v := ctx.(type) {
	case *index.LeafReaderContext:
		if v == nil {
			return nil, nil
		}
		reader := v.LeafReader()
		if reader == nil {
			return nil, nil
		}
		if np, ok := interface{}(reader).(numericProvider); ok {
			return np.GetNumericDocValues(field)
		}
		return nil, nil
	case numericProvider:
		return v.GetNumericDocValues(field)
	default:
		return nil, nil
	}
}

// GetValues returns the long values from the given context, materialised
// as a slice indexed by document id within the leaf.
//
// The leaf size is taken from the context when available; otherwise the
// iterator is exhausted into a dense slice sized to the highest doc id.
func (s *LongValuesSource) GetValues(context interface{}) ([]int64, error) {
	dv, err := numericProviderFromContext(context, s.field)
	if err != nil {
		return nil, err
	}
	if dv == nil {
		return nil, nil
	}
	// Determine the leaf size when possible so the returned slice is
	// dense.  We use the LeafReaderContext when present; otherwise we grow
	// the slice as we walk.
	var size int
	if lrc, ok := context.(*index.LeafReaderContext); ok && lrc != nil {
		if reader := lrc.LeafReader(); reader != nil {
			type maxDocer interface{ MaxDoc() int }
			if md, ok := interface{}(reader).(maxDocer); ok {
				size = md.MaxDoc()
			}
		}
	}
	var out []int64
	if size > 0 {
		out = make([]int64, size)
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
		if doc < len(out) {
			out[doc] = v
			continue
		}
		// Grow when size was unknown.
		for len(out) <= doc {
			out = append(out, 0)
		}
		out[doc] = v
	}
	return out, nil
}

// GetSortField returns a SortField for sorting by these values.
func (s *LongValuesSource) GetSortField(reverse bool) *SortField {
	sf := NewSortField(s.field, SortFieldTypeLong)
	sf.Reverse = reverse
	return sf
}

// GetRangeQuery returns a NumericDocValuesRangeQuery that matches
// documents whose value for s.field falls within [lower, upper] inclusive.
// Lower bound math.MinInt64 / upper bound math.MaxInt64 sentinels select
// open-ended ranges, matching Lucene's helper conventions.
func (s *LongValuesSource) GetRangeQuery(lower, upper int64) Query {
	if lower > upper {
		return NewMatchNoDocsQuery()
	}
	if lower == math.MinInt64 && upper == math.MaxInt64 {
		return NewFieldExistsQuery(s.field)
	}
	return NewNumericDocValuesRangeQuery(s.field, lower, upper)
}
