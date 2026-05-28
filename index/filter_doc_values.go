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
//
// rmp #4710 dropped the legacy random-access Get(docID) / GetOrd(docID)
// delegates from these wrappers as part of collapsing the index-side
// value-type interfaces onto the spi/ iterator surface; callers that
// previously reached for Get/GetOrd now drive iteration through
// AdvanceExact + LongValue/BinaryValue/OrdValue/NextOrd.

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

// Advance moves to target docID.
func (f *FilterBinaryDocValues) Advance(target int) (int, error) { return f.In.Advance(target) }

// AdvanceExact delegates to the wrapped iterator.
func (f *FilterBinaryDocValues) AdvanceExact(target int) (bool, error) {
	return f.In.AdvanceExact(target)
}

// BinaryValue delegates to the wrapped iterator.
func (f *FilterBinaryDocValues) BinaryValue() ([]byte, error) { return f.In.BinaryValue() }

// NextDoc advances to the next doc that has a value.
func (f *FilterBinaryDocValues) NextDoc() (int, error) { return f.In.NextDoc() }

// DocID returns the current doc ID.
func (f *FilterBinaryDocValues) DocID() int { return f.In.DocID() }

// Cost delegates to the wrapped iterator.
func (f *FilterBinaryDocValues) Cost() int64 { return f.In.Cost() }

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

// Advance moves to target docID.
func (f *FilterNumericDocValues) Advance(target int) (int, error) { return f.In.Advance(target) }

// AdvanceExact delegates to the wrapped iterator.
func (f *FilterNumericDocValues) AdvanceExact(target int) (bool, error) {
	return f.In.AdvanceExact(target)
}

// LongValue delegates to the wrapped iterator.
func (f *FilterNumericDocValues) LongValue() (int64, error) { return f.In.LongValue() }

// NextDoc advances to the next doc that has a value.
func (f *FilterNumericDocValues) NextDoc() (int, error) { return f.In.NextDoc() }

// DocID returns the current doc ID.
func (f *FilterNumericDocValues) DocID() int { return f.In.DocID() }

// Cost delegates to the wrapped iterator.
func (f *FilterNumericDocValues) Cost() int64 { return f.In.Cost() }

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

// Advance moves to target docID.
func (f *FilterSortedDocValues) Advance(target int) (int, error) { return f.In.Advance(target) }

// AdvanceExact delegates to the wrapped iterator.
func (f *FilterSortedDocValues) AdvanceExact(target int) (bool, error) {
	return f.In.AdvanceExact(target)
}

// OrdValue delegates to the wrapped iterator.
func (f *FilterSortedDocValues) OrdValue() (int, error) { return f.In.OrdValue() }

// LongValue delegates to the wrapped iterator (inherited NumericDocValues
// surface; for sorted values the long is the ord cast to int64).
func (f *FilterSortedDocValues) LongValue() (int64, error) { return f.In.LongValue() }

// NextDoc advances to the next doc that has a value.
func (f *FilterSortedDocValues) NextDoc() (int, error) { return f.In.NextDoc() }

// DocID returns the current doc ID.
func (f *FilterSortedDocValues) DocID() int { return f.In.DocID() }

// LookupOrd returns the value for the given ordinal.
func (f *FilterSortedDocValues) LookupOrd(ord int) ([]byte, error) { return f.In.LookupOrd(ord) }

// GetValueCount returns the number of unique values.
func (f *FilterSortedDocValues) GetValueCount() int { return f.In.GetValueCount() }

// Cost delegates to the wrapped iterator.
func (f *FilterSortedDocValues) Cost() int64 { return f.In.Cost() }

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

// Advance moves to target docID.
func (f *FilterSortedNumericDocValues) Advance(target int) (int, error) {
	return f.In.Advance(target)
}

// AdvanceExact delegates to the wrapped iterator.
func (f *FilterSortedNumericDocValues) AdvanceExact(target int) (bool, error) {
	return f.In.AdvanceExact(target)
}

// NextValue delegates to the wrapped iterator.
func (f *FilterSortedNumericDocValues) NextValue() (int64, error) { return f.In.NextValue() }

// DocValueCount delegates to the wrapped iterator.
func (f *FilterSortedNumericDocValues) DocValueCount() (int, error) {
	return f.In.DocValueCount()
}

// LongValue delegates to the wrapped iterator (inherited NumericDocValues
// surface; returns the first value of the current document).
func (f *FilterSortedNumericDocValues) LongValue() (int64, error) { return f.In.LongValue() }

// NextDoc advances to the next doc that has values.
func (f *FilterSortedNumericDocValues) NextDoc() (int, error) { return f.In.NextDoc() }

// DocID returns the current doc ID.
func (f *FilterSortedNumericDocValues) DocID() int { return f.In.DocID() }

// Cost delegates to the wrapped iterator.
func (f *FilterSortedNumericDocValues) Cost() int64 { return f.In.Cost() }

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

// Advance moves to target docID.
func (f *FilterSortedSetDocValues) Advance(target int) (int, error) { return f.In.Advance(target) }

// AdvanceExact delegates to the wrapped iterator.
func (f *FilterSortedSetDocValues) AdvanceExact(target int) (bool, error) {
	return f.In.AdvanceExact(target)
}

// NextOrd delegates to the wrapped iterator.
func (f *FilterSortedSetDocValues) NextOrd() (int, error) { return f.In.NextOrd() }

// NextDoc advances to the next doc that has values.
func (f *FilterSortedSetDocValues) NextDoc() (int, error) { return f.In.NextDoc() }

// DocID returns the current doc ID.
func (f *FilterSortedSetDocValues) DocID() int { return f.In.DocID() }

// LookupOrd returns the value for the given ordinal.
func (f *FilterSortedSetDocValues) LookupOrd(ord int) ([]byte, error) { return f.In.LookupOrd(ord) }

// GetValueCount returns the number of unique values.
func (f *FilterSortedSetDocValues) GetValueCount() int { return f.In.GetValueCount() }

// Cost delegates to the wrapped iterator.
func (f *FilterSortedSetDocValues) Cost() int64 { return f.In.Cost() }
