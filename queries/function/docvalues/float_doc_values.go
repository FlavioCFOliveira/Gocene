// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package docvalues

import (
	"strconv"

	"github.com/FlavioCFOliveira/Gocene/queries/function"
)

// FloatDocValues is the Go port of
// org.apache.lucene.queries.function.docvalues.FloatDocValues.
type FloatDocValues struct {
	function.BaseFunctionValues
	VS        function.ValueSource
	FloatFunc func(doc int) (float32, error)
}

// NewFloatDocValues wires vs + floatFunc into a ready-to-use FloatDocValues.
func NewFloatDocValues(vs function.ValueSource, floatFunc func(doc int) (float32, error)) *FloatDocValues {
	v := &FloatDocValues{VS: vs, FloatFunc: floatFunc}
	v.SetSelf(v)
	return v
}

// FloatVal delegates to the embedded function.
func (f *FloatDocValues) FloatVal(doc int) (float32, error) { return f.FloatFunc(doc) }

// ByteVal returns the truncated int8 of FloatVal.
func (f *FloatDocValues) ByteVal(doc int) (int8, error) {
	v, err := f.FloatFunc(doc)
	return int8(v), err
}

// ShortVal returns the truncated int16 of FloatVal.
func (f *FloatDocValues) ShortVal(doc int) (int16, error) {
	v, err := f.FloatFunc(doc)
	return int16(v), err
}

// IntVal returns the truncated int32 of FloatVal.
func (f *FloatDocValues) IntVal(doc int) (int32, error) {
	v, err := f.FloatFunc(doc)
	return int32(v), err
}

// LongVal returns the truncated int64 of FloatVal.
func (f *FloatDocValues) LongVal(doc int) (int64, error) {
	v, err := f.FloatFunc(doc)
	return int64(v), err
}

// BoolVal reports FloatVal != 0.
func (f *FloatDocValues) BoolVal(doc int) (bool, error) {
	v, err := f.FloatFunc(doc)
	return v != 0, err
}

// DoubleVal widens FloatVal to float64.
func (f *FloatDocValues) DoubleVal(doc int) (float64, error) {
	v, err := f.FloatFunc(doc)
	return float64(v), err
}

// StrVal renders FloatVal as a base-10 string.
func (f *FloatDocValues) StrVal(doc int) (string, error) {
	v, err := f.FloatFunc(doc)
	if err != nil {
		return "", err
	}
	return strconv.FormatFloat(float64(v), 'g', -1, 32), nil
}

// ObjectVal returns the float32 when the doc has a value.
func (f *FloatDocValues) ObjectVal(doc int) (any, error) {
	ok, err := f.Exists(doc)
	if err != nil || !ok {
		return nil, err
	}
	return f.FloatFunc(doc)
}

// ToString renders "<vs.description>=<value>".
func (f *FloatDocValues) ToString(doc int) (string, error) {
	s, err := f.StrVal(doc)
	if err != nil {
		return "", err
	}
	return f.VS.Description() + "=" + s, nil
}
