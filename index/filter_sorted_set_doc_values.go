//go:build ignore

// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// FilterSortedSetDocValues delegates all methods to a wrapped SortedSetDocValues.
// Mirrors org.apache.lucene.index.FilterSortedSetDocValues from Apache Lucene 10.5.0.
type FilterSortedSetDocValues struct {
	in SortedSetDocValues
}

// NewFilterSortedSetDocValues constructs a FilterSortedSetDocValues.
func NewFilterSortedSetDocValues(in SortedSetDocValues) *FilterSortedSetDocValues {
	if in == nil {
		panic("incoming SortedSetDocValues must not be null")
	}
	return &FilterSortedSetDocValues{in: in}
}

func (f *FilterSortedSetDocValues) AdvanceExact(target int) (bool, error) { return f.in.AdvanceExact(target) }
func (f *FilterSortedSetDocValues) NextOrd() (int64, error)                { return f.in.NextOrd() }
func (f *FilterSortedSetDocValues) GetDocValueCount() int                  { return f.in.GetDocValueCount() }
func (f *FilterSortedSetDocValues) LookupOrd(ord int64) ([]byte, error)    { return f.in.LookupOrd(ord) }
func (f *FilterSortedSetDocValues) GetValueCount() int64                   { return f.in.GetValueCount() }
func (f *FilterSortedSetDocValues) LookupTerm(key []byte) (int64, error)    { return f.in.LookupTerm(key) }
func (f *FilterSortedSetDocValues) TermsEnum() (TermsEnum, error)           { return f.in.TermsEnum() }
func (f *FilterSortedSetDocValues) Intersect(automaton util.CompiledAutomaton) (TermsEnum, error) {
	return f.in.Intersect(automaton)
}
func (f *FilterSortedSetDocValues) DocID() int                                { return f.in.DocID() }
func (f *FilterSortedSetDocValues) NextDoc() (int, error)                      { return f.in.NextDoc() }
func (f *FilterSortedSetDocValues) Advance(target int) (int, error)            { return f.in.Advance(target) }
func (f *FilterSortedSetDocValues) Cost() int64                                { return f.in.Cost() }
func (f *FilterSortedSetDocValues) GetDelegate() SortedSetDocValues           { return f.in }
