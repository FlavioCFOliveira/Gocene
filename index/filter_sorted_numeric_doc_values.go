// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// FilterSortedNumericDocValues delegates all methods to a wrapped SortedNumericDocValues.
// Mirrors org.apache.lucene.index.FilterSortedNumericDocValues from Apache Lucene 10.5.0.
type FilterSortedNumericDocValues struct {
	in SortedNumericDocValues
}

// NewFilterSortedNumericDocValues constructs a FilterSortedNumericDocValues.
func NewFilterSortedNumericDocValues(in SortedNumericDocValues) *FilterSortedNumericDocValues {
	if in == nil {
		panic("incoming SortedNumericDocValues must not be null")
	}
	return &FilterSortedNumericDocValues{in: in}
}

func (f *FilterSortedNumericDocValues) AdvanceExact(target int) (bool, error) {
	return f.in.AdvanceExact(target)
}
func (f *FilterSortedNumericDocValues) NextValue() (int64, error)           { return f.in.NextValue() }
func (f *FilterSortedNumericDocValues) DocValueCount() (int, error)         { return f.in.DocValueCount() }
func (f *FilterSortedNumericDocValues) DocID() int                          { return f.in.DocID() }
func (f *FilterSortedNumericDocValues) NextDoc() (int, error)               { return f.in.NextDoc() }
func (f *FilterSortedNumericDocValues) Advance(target int) (int, error)     { return f.in.Advance(target) }
func (f *FilterSortedNumericDocValues) Cost() int64                         { return f.in.Cost() }
func (f *FilterSortedNumericDocValues) GetDelegate() SortedNumericDocValues { return f.in }
