// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

// BinaryRangeDocValuesField is the shared base of the concrete N-dimensional
// range doc-values fields (IntRangeDocValuesField, LongRangeDocValuesField,
// FloatRangeDocValuesField, DoubleRangeDocValuesField, ...). It stores the
// packed [min, max] range payload as a binary doc-value plus the dimension
// metadata required to decode it.
//
// Go port of Lucene 10.4.0's
// org.apache.lucene.document.BinaryRangeDocValuesField. The Java original is
// package-private and abstract; in Gocene we expose the type so concrete
// range fields living in this same package can embed it via
// *BinaryRangeDocValuesField, mirroring the Java inheritance shape while
// staying idiomatic in Go.
type BinaryRangeDocValuesField struct {
	*BinaryDocValuesField
	field                string
	packedValue          []byte
	numDims              int
	numBytesPerDimension int
}

// NewBinaryRangeDocValuesField constructs a BinaryRangeDocValuesField for the
// given field name, packed range payload, dimension count and per-dimension
// byte width. The packed payload is defensively copied so external mutations
// after construction do not affect the indexed value.
func NewBinaryRangeDocValuesField(
	field string,
	packedValue []byte,
	numDims int,
	numBytesPerDimension int,
) (*BinaryRangeDocValuesField, error) {
	dup := make([]byte, len(packedValue))
	copy(dup, packedValue)
	b, err := NewBinaryDocValuesField(field, dup)
	if err != nil {
		return nil, err
	}
	return &BinaryRangeDocValuesField{
		BinaryDocValuesField: b,
		field:                field,
		packedValue:          dup,
		numDims:              numDims,
		numBytesPerDimension: numBytesPerDimension,
	}, nil
}

// FieldName returns the field name. Mirrors the Java `field` final field.
// We name the accessor FieldName to avoid colliding with the embedded
// *Field.Name() while still exposing the value the Java base stores in its
// `field` slot.
func (f *BinaryRangeDocValuesField) FieldName() string { return f.field }

// PackedValue returns the underlying packed [min, max] payload. The returned
// slice aliases the internal buffer and must not be mutated by callers.
func (f *BinaryRangeDocValuesField) PackedValue() []byte { return f.packedValue }

// NumDims returns the number of dimensions encoded in the packed value.
func (f *BinaryRangeDocValuesField) NumDims() int { return f.numDims }

// NumBytesPerDimension returns the size, in bytes, of a single dimension
// component (one half of a [min, max] pair).
func (f *BinaryRangeDocValuesField) NumBytesPerDimension() int { return f.numBytesPerDimension }
