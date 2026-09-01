// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package function

import (
	"fmt"
	"github.com/FlavioCFOliveira/Gocene/index"
)

// ConstValueSource returns a constant for all documents.
// Mirrors org.apache.lucene.queries.function.valuesource.ConstValueSource from Apache Lucene 10.5.0.
type ConstValueSource struct {
	BaseValueSource
	constant float32
	dv       float64
}

// NewConstValueSource creates a new ConstValueSource.
func NewConstValueSource(constant float32) *ConstValueSource {
	return &ConstValueSource{
		constant: constant,
		dv:       float64(constant),
	}
}

func (c *ConstValueSource) Description() string {
	return fmt.Sprintf("const(%f)", c.constant)
}

func (c *ConstValueSource) GetValues(ctx Context, readerContext *index.LeafReaderContext) (FunctionValues, error) {
	fv := &constValueSourceValues{
		BaseFunctionValues: BaseFunctionValues{},
		vs:                 c,
	}
	fv.SetSelf(fv)
	return fv, nil
}

func (c *ConstValueSource) GetInt() int32 {
	return int32(c.constant)
}

func (c *ConstValueSource) GetLong() int64 {
	return int64(c.constant)
}

func (c *ConstValueSource) GetFloat() float32 {
	return c.constant
}

func (c *ConstValueSource) GetDouble() float64 {
	return c.dv
}

func (c *ConstValueSource) GetNumber() any {
	return c.constant
}

func (c *ConstValueSource) GetBool() bool {
	return c.constant != 0.0
}

func (c *ConstValueSource) Equals(other ValueSource) bool {
	if other == nil {
		return false
	}
	if o, ok := other.(*ConstValueSource); ok {
		return c.constant == o.constant
	}
	return false
}

func (c *ConstValueSource) HashCode() int32 {
	// Simplified mirror of Float.floatToIntBits(constant) * 31
	return int32(c.constant) * 31
}

type constValueSourceValues struct {
	BaseFunctionValues
	vs *ConstValueSource
}

func (v *constValueSourceValues) ByteVal(doc int) (int8, error) {
	return int8(v.vs.constant), nil
}

func (v *constValueSourceValues) ShortVal(doc int) (int16, error) {
	return int16(v.vs.constant), nil
}

func (v *constValueSourceValues) FloatVal(doc int) (float32, error) {
	return v.vs.constant, nil
}

func (v *constValueSourceValues) IntVal(doc int) (int32, error) {
	return int32(v.vs.constant), nil
}

func (v *constValueSourceValues) LongVal(doc int) (int64, error) {
	return int64(v.vs.constant), nil
}

func (v *constValueSourceValues) DoubleVal(doc int) (float64, error) {
	return v.vs.dv, nil
}

func (v *constValueSourceValues) StrVal(doc int) (string, error) {
	return fmt.Sprintf("%f", v.vs.constant), nil
}

func (v *constValueSourceValues) BoolVal(doc int) (bool, error) {
	return v.vs.constant != 0.0, nil
}

func (v *constValueSourceValues) ObjectVal(doc int) (any, error) {
	return v.vs.constant, nil
}

func (v *constValueSourceValues) Exists(doc int) (bool, error) {
	return true, nil
}

func (v *constValueSourceValues) ToString(doc int) (string, error) {
	return fmt.Sprintf("%s=%f", v.vs.Description(), v.vs.constant), nil
}
