// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// FilterBinaryDocValues delegates all methods to a wrapped BinaryDocValues.
// Mirrors org.apache.lucene.index.FilterBinaryDocValues from Apache Lucene 10.5.0.
type FilterBinaryDocValues struct {
	in BinaryDocValues
}

// NewFilterBinaryDocValues constructs a FilterBinaryDocValues.
func NewFilterBinaryDocValues(in BinaryDocValues) *FilterBinaryDocValues {
	if in == nil {
		panic("incoming BinaryDocValues must not be null")
	}
	return &FilterBinaryDocValues{in: in}
}

func (f *FilterBinaryDocValues) DocID() int                                { return f.in.DocID() }
func (f *FilterBinaryDocValues) NextDoc() (int, error)                      { return f.in.NextDoc() }
func (f *FilterBinaryDocValues) Advance(target int) (int, error)            { return f.in.Advance(target) }
func (f *FilterBinaryDocValues) AdvanceExact(target int) (bool, error)      { return f.in.AdvanceExact(target) }
func (f *FilterBinaryDocValues) Cost() int64                                { return f.in.Cost() }
func (f *FilterBinaryDocValues) BinaryValue() ([]byte, error)               { return f.in.BinaryValue() }
func (f *FilterBinaryDocValues) GetDelegate() BinaryDocValues               { return f.in }
