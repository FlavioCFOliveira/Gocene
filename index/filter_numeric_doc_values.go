// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// FilterNumericDocValues delegates all methods to a wrapped NumericDocValues.
// Mirrors org.apache.lucene.index.FilterNumericDocValues from Apache Lucene 10.5.0.
type FilterNumericDocValues struct {
	in NumericDocValues
}

// NewFilterNumericDocValues constructs a FilterNumericDocValues.
func NewFilterNumericDocValues(in NumericDocValues) *FilterNumericDocValues {
	if in == nil {
		panic("incoming NumericDocValues must not be null")
	}
	return &FilterNumericDocValues{in: in}
}

func (f *FilterNumericDocValues) DocID() int                                { return f.in.DocID() }
func (f *FilterNumericDocValues) NextDoc() (int, error)                      { return f.in.NextDoc() }
func (f *FilterNumericDocValues) Advance(target int) (int, error)            { return f.in.Advance(target) }
func (f *FilterNumericDocValues) AdvanceExact(target int) (bool, error)      { return f.in.AdvanceExact(target) }
func (f *FilterNumericDocValues) Cost() int64                                { return f.in.Cost() }
func (f *FilterNumericDocValues) LongValue() (int64, error)                  { return f.in.LongValue() }
func (f *FilterNumericDocValues) GetDelegate() NumericDocValues             { return f.in }
