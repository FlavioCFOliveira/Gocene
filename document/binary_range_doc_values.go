// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

// BinaryRangeDocValues wraps a binary doc-values payload and exposes the
// decoded N-dimensional range as a packed byte slice.
//
// Go port of Lucene 10.4.0's
// org.apache.lucene.document.BinaryRangeDocValues. The Java original
// extends BinaryDocValues to plug into the search/range-query pipeline.
// Gocene mirrors the type as a thin struct over an in-memory packed value;
// integration with a future Gocene BinaryDocValues iterator is deferred
// pending the Sprint 22 search wiring.
type BinaryRangeDocValues struct {
	packedValue          []byte
	numDims              int
	numBytesPerDimension int
}

// NewBinaryRangeDocValues constructs a BinaryRangeDocValues holding the
// given packed range payload. numBytesPerDimension reflects the size of a
// single value (not the [min, max] pair).
func NewBinaryRangeDocValues(packedValue []byte, numDims, numBytesPerDimension int) *BinaryRangeDocValues {
	dup := make([]byte, len(packedValue))
	copy(dup, packedValue)
	return &BinaryRangeDocValues{
		packedValue:          dup,
		numDims:              numDims,
		numBytesPerDimension: numBytesPerDimension,
	}
}

// PackedValue returns the underlying packed byte payload.
func (b *BinaryRangeDocValues) PackedValue() []byte { return b.packedValue }

// NumDims returns the number of dimensions per range.
func (b *BinaryRangeDocValues) NumDims() int { return b.numDims }

// NumBytesPerDimension returns the size of a single dimension value.
func (b *BinaryRangeDocValues) NumBytesPerDimension() int { return b.numBytesPerDimension }
