// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

// DoubleRangeDocValuesField is a DocValues field for DoubleRange.
// It is a single-valued field (one value per document) and stores
// an N-dimensional double range as binary doc-values.
//
// Go port of Lucene 10.4.0's org.apache.lucene.document.DoubleRangeDocValuesField.
type DoubleRangeDocValuesField struct {
	*BinaryRangeDocValuesField
	min []float64
	max []float64
}

// NewDoubleRangeDocValuesField creates a new DoubleRangeDocValuesField.
// min and max must have the same length (1..4) and min[i] <= max[i].
func NewDoubleRangeDocValuesField(name string, min, max []float64) (*DoubleRangeDocValuesField, error) {
	if err := checkRangeDocValuesArgs(len(min), len(max)); err != nil {
		return nil, err
	}

	encoded, err := EncodeDoubleRangeLucene(min, max)
	if err != nil {
		return nil, err
	}

	// Call NewBinaryRangeDocValuesField to mirror Java's super() call.
	// DoubleRangeBytes is the width of a single dimension component.
	b, err := NewBinaryRangeDocValuesField(name, encoded, len(min), DoubleRangeBytes)
	if err != nil {
		return nil, err
	}

	dupMin := make([]float64, len(min))
	dupMax := make([]float64, len(max))
	copy(dupMin, min)
	copy(dupMax, max)

	return &DoubleRangeDocValuesField{
		BinaryRangeDocValuesField: b,
		min:                       dupMin,
		max:                       dupMax,
	}, nil
}

// GetMin returns the minimum value for the given dimension.
func (f *DoubleRangeDocValuesField) GetMin(dim int) float64 {
	mustDim(dim, len(f.min))
	return f.min[dim]
}

// GetMax returns the maximum value for the given dimension.
func (f *DoubleRangeDocValuesField) GetMax(dim int) float64 {
	mustDim(dim, len(f.max))
	return f.max[dim]
}

// NOTE: NewSlowIntersectsQuery is deferred until search.RangeFieldQuery is implemented
// (see backlog #2695).
