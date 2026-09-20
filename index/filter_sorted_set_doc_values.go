// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// FilterSortedSetDocValues delegates all methods to a wrapped SortedSetDocValues.
// Mirrors org.apache.lucene.index.FilterSortedSetDocValues from Apache Lucene 10.5.0.
//
// Divergence: spi.SortedSetDocValues does not declare docValueCount(),
// lookupTerm(), termsEnum() or intersect(). DocValueCount is therefore
// recovered from the delegate through the optional sortedSetDocValueCounter
// surface (every Lucene SortedSetDocValues implements docValueCount()), while
// LookupTerm/TermsEnum/Intersect run the concrete implementations Lucene
// declares on the SortedSetDocValues base class (LookupSetTerm,
// OpenSetTermsEnum, IntersectSet) against the wrapped values.
type FilterSortedSetDocValues struct {
	in SortedSetDocValues
}

// sortedSetDocValueCounter is the docValueCount() half of
// org.apache.lucene.index.SortedSetDocValues that spi.SortedSetDocValues does
// not declare. Lucene makes it abstract, so every concrete implementation
// provides it.
type sortedSetDocValueCounter interface {
	DocValueCount() int
}

// NewFilterSortedSetDocValues constructs a FilterSortedSetDocValues.
func NewFilterSortedSetDocValues(in SortedSetDocValues) *FilterSortedSetDocValues {
	if in == nil {
		panic("incoming SortedSetDocValues must not be null")
	}
	return &FilterSortedSetDocValues{in: in}
}

// AdvanceExact delegates to the wrapped values.
func (f *FilterSortedSetDocValues) AdvanceExact(target int) (bool, error) {
	return f.in.AdvanceExact(target)
}

// NextOrd delegates to the wrapped values.
func (f *FilterSortedSetDocValues) NextOrd() (int, error) { return f.in.NextOrd() }

// DocValueCount delegates to the wrapped values. It panics when the delegate
// does not expose docValueCount(), mirroring the UnsupportedOperationException
// a partial SortedSetDocValues would raise in Lucene.
func (f *FilterSortedSetDocValues) DocValueCount() int {
	counter, ok := f.in.(sortedSetDocValueCounter)
	if !ok {
		panic("FilterSortedSetDocValues: delegate does not implement docValueCount()")
	}
	return counter.DocValueCount()
}

// LookupOrd delegates to the wrapped values.
func (f *FilterSortedSetDocValues) LookupOrd(ord int) ([]byte, error) { return f.in.LookupOrd(ord) }

// GetValueCount delegates to the wrapped values.
func (f *FilterSortedSetDocValues) GetValueCount() int { return f.in.GetValueCount() }

// LookupTerm runs SortedSetDocValues.lookupTerm over the wrapped values.
func (f *FilterSortedSetDocValues) LookupTerm(key *util.BytesRef) (int, error) {
	return LookupSetTerm(f.in, key)
}

// TermsEnum runs SortedSetDocValues.termsEnum over the wrapped values.
func (f *FilterSortedSetDocValues) TermsEnum(field string) (TermsEnum, error) {
	return OpenSetTermsEnum(field, f.in)
}

// Intersect runs SortedSetDocValues.intersect over the wrapped values.
func (f *FilterSortedSetDocValues) Intersect(field string, compiled *automaton.CompiledAutomaton) (TermsEnum, error) {
	return IntersectSet(field, f.in, compiled)
}

// DocID delegates to the wrapped values.
func (f *FilterSortedSetDocValues) DocID() int { return f.in.DocID() }

// NextDoc delegates to the wrapped values.
func (f *FilterSortedSetDocValues) NextDoc() (int, error) { return f.in.NextDoc() }

// Advance delegates to the wrapped values.
func (f *FilterSortedSetDocValues) Advance(target int) (int, error) { return f.in.Advance(target) }

// Cost delegates to the wrapped values.
func (f *FilterSortedSetDocValues) Cost() int64 { return f.in.Cost() }

// GetDelegate returns the wrapped values.
func (f *FilterSortedSetDocValues) GetDelegate() SortedSetDocValues { return f.in }
