// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import "math"

// DoubleDocValuesField is syntactic sugar for encoding doubles as numeric
// doc-values via Float64bits / Float64frombits. Mirrors Lucene 10.4.0's
// org.apache.lucene.document.DoubleDocValuesField.
//
// Note: storing every double per document costs 8 bytes. Custom encoding
// (e.g. sortable bytes or scalar quantization) may be more efficient when
// reduced precision is acceptable.
type DoubleDocValuesField struct {
	*NumericDocValuesField
}

// NewDoubleDocValuesField creates a new DoubleDocValuesField with the given
// name and float64 value. The double is stored as int64 via Float64bits.
func NewDoubleDocValuesField(name string, value float64) (*DoubleDocValuesField, error) {
	n, err := NewNumericDocValuesField(name, int64(math.Float64bits(value)))
	if err != nil {
		return nil, err
	}
	return &DoubleDocValuesField{NumericDocValuesField: n}, nil
}

// SetDoubleValue updates the field's value (re-encoded via Float64bits).
func (f *DoubleDocValuesField) SetDoubleValue(value float64) {
	f.NumericDocValuesField.SetLongValue(int64(math.Float64bits(value)))
}

// GetDoubleValue returns the field's value decoded from its int64 storage.
func (f *DoubleDocValuesField) GetDoubleValue() float64 {
	return math.Float64frombits(uint64(f.NumericDocValuesField.GetValue()))
}
