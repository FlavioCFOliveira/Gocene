// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

// Ported from Apache Lucene 10.5.0:
//   lucene/queries/src/java/org/apache/lucene/queries/function/valuesource/SortedSetFieldSource.java

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
	"github.com/FlavioCFOliveira/Gocene/queries/function/docvalues"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// SortedSetFieldSource retrieves [function.FunctionValues] instances for
// multi-valued string based fields.
//
// Port of org.apache.lucene.queries.function.valuesource.SortedSetFieldSource.
type SortedSetFieldSource struct {
	FieldCacheSource
	Selector search.SortedSetSelectorType
}

// NewSortedSetFieldSource creates a SortedSetFieldSource selecting the minimum
// value, mirroring SortedSetFieldSource(String).
func NewSortedSetFieldSource(field string) *SortedSetFieldSource {
	return NewSortedSetFieldSourceWithSelector(field, search.SortedSetSelectorMin)
}

// NewSortedSetFieldSourceWithSelector mirrors
// SortedSetFieldSource(String, SortedSetSelector.Type).
func NewSortedSetFieldSourceWithSelector(field string, selector search.SortedSetSelectorType) *SortedSetFieldSource {
	return &SortedSetFieldSource{
		FieldCacheSource: FieldCacheSource{Field: field},
		Selector:         selector,
	}
}

// Description returns "sortedset(<field>,selector=<selector>)".
//
// Mirrors SortedSetFieldSource.description().
func (f *SortedSetFieldSource) Description() string {
	return fmt.Sprintf("sortedset(%s,selector=%s)", f.Field, f.Selector)
}

// GetValues reads the field's SortedSetDocValues and reduces them to a single
// value per document with SortedSetSelector.wrap.
//
// Mirrors SortedSetFieldSource.getValues(Map, LeafReaderContext).
func (f *SortedSetFieldSource) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	sortedSet, err := index.GetSortedSet(readerContext.LeafReader(), f.Field)
	if err != nil {
		return nil, err
	}
	view := search.WrapSortedSet(sortedSet, f.Selector)

	dv := docvalues.NewDocTermsIndexDocValuesFromDV(f, view)

	return &sortedSetFallback{
		DocTermsIndexDocValues: dv,
	}, nil
}

// Equals reports value equality.
//
// Mirrors SortedSetFieldSource.equals(Object).
func (f *SortedSetFieldSource) Equals(other function.ValueSource) bool {
	o, ok := other.(*SortedSetFieldSource)
	if !ok || o == nil {
		return false
	}
	if f.Selector != o.Selector {
		return false
	}
	return f.Field == o.Field
}

// HashCode returns a stable hash.
//
// Mirrors SortedSetFieldSource.hashCode(): 31 * super.hashCode() plus the
// selector.
func (f *SortedSetFieldSource) HashCode() int32 {
	return 31*f.FieldCacheSource.HashCode() + int32(f.Selector)
}

// sortedSetFallback renders the anonymous DocTermsIndexDocValues subclass that
// getValues returns, overriding objectVal to delegate to strVal.
type sortedSetFallback struct {
	*docvalues.DocTermsIndexDocValues
}

// ObjectVal mirrors the anonymous class's objectVal(int) override.
func (f *sortedSetFallback) ObjectVal(doc int) (any, error) {
	return f.StrVal(doc)
}

var _ function.ValueSource = (*SortedSetFieldSource)(nil)
