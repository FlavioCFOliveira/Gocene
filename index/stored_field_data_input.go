// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "github.com/FlavioCFOliveira/Gocene/store"

// StoredFieldDataInput pairs a length with the DataInput from which the
// stored-field bytes can be read. Mirrors
// org.apache.lucene.index.StoredFieldDataInput from Apache Lucene 10.4.0
// (a record carrying a DataInput and an int length).
//
// Used by codecs that decode stored fields lazily — the visitor receives
// the (in, length) pair and may consume exactly length bytes itself.
//
// The fields are exported so that codec implementations in package codecs
// can build a StoredFieldDataInput without going through the constructor
// (which is the Java pattern); production code should otherwise prefer
// the NewStoredFieldDataInput* helpers below for the input validation
// they apply.
type StoredFieldDataInput struct {
	// Length is the number of bytes available for this stored field.
	Length int

	// In is the source DataInput positioned at the first byte of the field.
	In store.DataInput
}

// NewStoredFieldDataInput constructs a StoredFieldDataInput from an
// explicit DataInput and a byte length.
//
// Panics when length is negative; that mirrors the
// IllegalArgumentException users get from Java's record contract.
func NewStoredFieldDataInput(in store.DataInput, length int) *StoredFieldDataInput {
	if length < 0 {
		panic("StoredFieldDataInput: length must not be negative")
	}
	return &StoredFieldDataInput{Length: length, In: in}
}

// NewStoredFieldDataInputFromByteArray builds a StoredFieldDataInput
// whose byte budget is taken from a ByteArrayDataInput's own buffer
// length. Mirrors Lucene 10.4.0's
// StoredFieldDataInput(ByteArrayDataInput) convenience constructor.
//
// Panics when in is nil to match the Java NullPointerException.
func NewStoredFieldDataInputFromByteArray(in *store.ByteArrayDataInput) *StoredFieldDataInput {
	if in == nil {
		panic("StoredFieldDataInput: ByteArrayDataInput must not be nil")
	}
	return &StoredFieldDataInput{Length: in.Length(), In: in}
}

// DataInput returns the underlying DataInput. Provided for parity with
// Java's record accessor of the same name.
func (s *StoredFieldDataInput) DataInput() store.DataInput {
	return s.In
}

// GetLength returns the byte budget callers must respect when consuming
// the data input. Equivalent to reading the Length field directly; kept
// for API parity with the Java record's getLength().
func (s *StoredFieldDataInput) GetLength() int {
	return s.Length
}
