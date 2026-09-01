// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// FilterSortedDocValues delegates all methods to a wrapped SortedDocValues.
// Mirrors org.apache.lucene.index.FilterSortedDocValues from Apache Lucene 10.5.0.
type FilterSortedDocValues struct {
	in SortedDocValues
}

// NewFilterSortedDocValues constructs a FilterSortedDocValues.
func NewFilterSortedDocValues(in SortedDocValues) *FilterSortedDocValues {
	if in == nil {
		panic("incoming SortedDocValues must not be null")
	}
	return &FilterSortedDocValues{in: in}
}

func (f *FilterSortedDocValues) AdvanceExact(target int) (bool, error) { return f.in.AdvanceExact(target) }
func (f *FilterSortedDocValues) OrdValue() (int, error)              { return f.in.OrdValue() }
func (f *FilterSortedDocValues) LookupOrd(ord int) ([]byte, error)    { return f.in.LookupOrd(ord) }
func (f *FilterSortedDocValues) GetValueCount() int                  { return f.in.GetValueCount() }
func (f *FilterSortedDocValues) LookupTerm(key []byte) (int, error)  { return f.in.LookupTerm(key) }
func (f *FilterSortedDocValues) TermsEnum() (TermsEnum, error)         { return f.in.TermsEnum() }
func (f *FilterSortedDocValues) Intersect(automaton util.CompiledAutomaton) (TermsEnum, error) {
	return f.in.Intersect(automaton)
}
func (f *FilterSortedDocValues) DocID() int                                { return f.in.DocID() }
func (f *FilterSortedDocValues) NextDoc() (int, error)                      { return f.in.NextDoc() }
func (f *FilterSortedDocValues) Advance(target int) (int, error)            { return f.in.Advance(target) }
func (f *FilterSortedDocValues) Cost() int64                                { return f.in.Cost() }
func (f *FilterSortedDocValues) GetDelegate() SortedDocValues               { return f.in }
