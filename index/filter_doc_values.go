// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// This file ports Lucene 10.4.0's Filter*DocValues wrappers as a family of
// struct base types that callers can embed to inherit pass-through delegation
// and override only the methods they care about.
//
// Lucene maps:
//   FilterBinaryDocValues        -> *FilterBinaryDocValues
//   FilterNumericDocValues       -> *FilterNumericDocValues
//   FilterSortedDocValues        -> *FilterSortedDocValues
//   FilterSortedNumericDocValues -> *FilterSortedNumericDocValues
//   FilterSortedSetDocValues     -> *FilterSortedSetDocValues
//
// Because Gocene DocValues are interfaces (not abstract classes), the Java
// "extends X" mechanic is reproduced by embedding the wrapper struct in user
// code and supplying any overridden methods next to the embedded field.

// --- BinaryDocValues ---------------------------------------------------------

// FilterBinaryDocValues delegates every BinaryDocValues call to In.
type FilterBinaryDocValues struct {
	In BinaryDocValues
}

// NewFilterBinaryDocValues panics if in is nil; matches Java's
// Objects.requireNonNull behaviour.
func NewFilterBinaryDocValues(in BinaryDocValues) *FilterBinaryDocValues {
	if in == nil {
		panic("FilterBinaryDocValues: in must not be nil")
	}
	return &FilterBinaryDocValues{In: in}
}

// Get returns the binary value for docID.
func (f *FilterBinaryDocValues) Get(docID int) ([]byte, error) { return f.In.Get(docID) }

// Advance moves to target docID.
func (f *FilterBinaryDocValues) Advance(target int) (int, error) { return f.In.Advance(target) }

// NextDoc advances to the next doc that has a value.
func (f *FilterBinaryDocValues) NextDoc() (int, error) { return f.In.NextDoc() }

// DocID returns the current doc ID.
func (f *FilterBinaryDocValues) DocID() int { return f.In.DocID() }

// --- NumericDocValues --------------------------------------------------------

// FilterNumericDocValues delegates every NumericDocValues call to In.
type FilterNumericDocValues struct {
	In NumericDocValues
}

// NewFilterNumericDocValues panics if in is nil.
func NewFilterNumericDocValues(in NumericDocValues) *FilterNumericDocValues {
	if in == nil {
		panic("FilterNumericDocValues: in must not be nil")
	}
	return &FilterNumericDocValues{In: in}
}

// Get returns the numeric value for docID.
func (f *FilterNumericDocValues) Get(docID int) (int64, error) { return f.In.Get(docID) }

// Advance moves to target docID.
func (f *FilterNumericDocValues) Advance(target int) (int, error) { return f.In.Advance(target) }

// NextDoc advances to the next doc that has a value.
func (f *FilterNumericDocValues) NextDoc() (int, error) { return f.In.NextDoc() }

// DocID returns the current doc ID.
func (f *FilterNumericDocValues) DocID() int { return f.In.DocID() }

// --- SortedDocValues ---------------------------------------------------------

// FilterSortedDocValues delegates every SortedDocValues call to In.
type FilterSortedDocValues struct {
	In SortedDocValues
}

// NewFilterSortedDocValues panics if in is nil.
func NewFilterSortedDocValues(in SortedDocValues) *FilterSortedDocValues {
	if in == nil {
		panic("FilterSortedDocValues: in must not be nil")
	}
	return &FilterSortedDocValues{In: in}
}

// Get returns the binary value for docID.
func (f *FilterSortedDocValues) Get(docID int) ([]byte, error) { return f.In.Get(docID) }

// Advance moves to target docID.
func (f *FilterSortedDocValues) Advance(target int) (int, error) { return f.In.Advance(target) }

// NextDoc advances to the next doc that has a value.
func (f *FilterSortedDocValues) NextDoc() (int, error) { return f.In.NextDoc() }

// DocID returns the current doc ID.
func (f *FilterSortedDocValues) DocID() int { return f.In.DocID() }

// GetOrd returns the ordinal of the current doc.
func (f *FilterSortedDocValues) GetOrd(docID int) (int, error) { return f.In.GetOrd(docID) }

// LookupOrd returns the value for the given ordinal.
func (f *FilterSortedDocValues) LookupOrd(ord int) ([]byte, error) { return f.In.LookupOrd(ord) }

// GetValueCount returns the number of unique values.
func (f *FilterSortedDocValues) GetValueCount() int { return f.In.GetValueCount() }

// --- SortedNumericDocValues --------------------------------------------------

// FilterSortedNumericDocValues delegates every SortedNumericDocValues call to In.
type FilterSortedNumericDocValues struct {
	In SortedNumericDocValues
}

// NewFilterSortedNumericDocValues panics if in is nil.
func NewFilterSortedNumericDocValues(in SortedNumericDocValues) *FilterSortedNumericDocValues {
	if in == nil {
		panic("FilterSortedNumericDocValues: in must not be nil")
	}
	return &FilterSortedNumericDocValues{In: in}
}

// Get returns the numeric values for docID.
func (f *FilterSortedNumericDocValues) Get(docID int) ([]int64, error) { return f.In.Get(docID) }

// Advance moves to target docID.
func (f *FilterSortedNumericDocValues) Advance(target int) (int, error) {
	return f.In.Advance(target)
}

// NextDoc advances to the next doc that has values.
func (f *FilterSortedNumericDocValues) NextDoc() (int, error) { return f.In.NextDoc() }

// DocID returns the current doc ID.
func (f *FilterSortedNumericDocValues) DocID() int { return f.In.DocID() }

// --- SortedSetDocValues ------------------------------------------------------

// FilterSortedSetDocValues delegates every SortedSetDocValues call to In.
type FilterSortedSetDocValues struct {
	In SortedSetDocValues
}

// NewFilterSortedSetDocValues panics if in is nil.
func NewFilterSortedSetDocValues(in SortedSetDocValues) *FilterSortedSetDocValues {
	if in == nil {
		panic("FilterSortedSetDocValues: in must not be nil")
	}
	return &FilterSortedSetDocValues{In: in}
}

// Get returns the ordinals for docID.
func (f *FilterSortedSetDocValues) Get(docID int) ([]int, error) { return f.In.Get(docID) }

// Advance moves to target docID.
func (f *FilterSortedSetDocValues) Advance(target int) (int, error) { return f.In.Advance(target) }

// NextDoc advances to the next doc that has values.
func (f *FilterSortedSetDocValues) NextDoc() (int, error) { return f.In.NextDoc() }

// DocID returns the current doc ID.
func (f *FilterSortedSetDocValues) DocID() int { return f.In.DocID() }

// LookupOrd returns the value for the given ordinal.
func (f *FilterSortedSetDocValues) LookupOrd(ord int) ([]byte, error) { return f.In.LookupOrd(ord) }

// GetValueCount returns the number of unique values.
func (f *FilterSortedSetDocValues) GetValueCount() int { return f.In.GetValueCount() }
