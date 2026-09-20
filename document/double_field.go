// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
package document
import (
	"strconv"
	"github.com/FlavioCFOliveira/Gocene/spi"
)
// DoubleField is a field for indexing float64 values.
type DoubleField struct {
	*Field
}
// NewDoubleField creates a new DoubleField.
func NewDoubleField(name string, value float64, store bool) (*DoubleField, error) {
	ft := NewFieldType()
	ft.SetStored(store)
	ft.SetIndexed(true)
	ft.SetIndexOptions(spi.IndexOptionsDocs)
	ft.Freeze()
	field, err := NewField(name, strconv.FormatFloat(value, 'f', -1, 64), ft)
	if err != nil {
		return nil, err
	}
	return &DoubleField{Field: field}, nil
}
// DoubleValue returns the float64 value.
func (f *DoubleField) DoubleValue() float64 {
	val, _ := strconv.ParseFloat(f.StringValue(), 64)
	return val
}
// encodeFloat64Legacy encodes a float64 to a sortable byte representation.
func encodeFloat64Legacy(f float64) []byte {
	return PackDouble(f)
}
// decodeFloat64Legacy decodes a byte representation back to float64.
func decodeFloat64Legacy(buf []byte) float64 {
	return UnpackDouble(buf)
}
// DoublePoint is now defined in double_point.go
// NewDoublePoint is now defined in double_point.go
// NewDoublePoints is now defined in double_point.go
// DoubleValue is now defined in double_point.go
// DoubleValues is now defined in double_point.go
