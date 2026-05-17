// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import "math"

// FloatDocValuesField is syntactic sugar for encoding floats as numeric
// doc-values via Float32bits / Float32frombits. Mirrors Lucene 10.4.0's
// org.apache.lucene.document.FloatDocValuesField.
//
// Note: storing every float per document still costs 8 bytes (the
// underlying NumericDocValuesField is int64-backed). Custom encoding may
// be more efficient when reduced precision is acceptable.
type FloatDocValuesField struct {
	*NumericDocValuesField
}

// NewFloatDocValuesField creates a new FloatDocValuesField with the given
// name and float32 value. The float is stored via Float32bits then
// promoted to int64.
func NewFloatDocValuesField(name string, value float32) (*FloatDocValuesField, error) {
	n, err := NewNumericDocValuesField(name, int64(math.Float32bits(value)))
	if err != nil {
		return nil, err
	}
	return &FloatDocValuesField{NumericDocValuesField: n}, nil
}

// SetFloatValue updates the field's value (re-encoded via Float32bits).
func (f *FloatDocValuesField) SetFloatValue(value float32) {
	f.NumericDocValuesField.SetLongValue(int64(math.Float32bits(value)))
}

// GetFloatValue returns the field's value decoded from its int32 storage.
func (f *FloatDocValuesField) GetFloatValue() float32 {
	return math.Float32frombits(uint32(f.NumericDocValuesField.GetValue()))
}
