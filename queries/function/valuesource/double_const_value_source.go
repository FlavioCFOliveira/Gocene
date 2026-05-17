// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import (
	"fmt"
	"strconv"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
	"github.com/FlavioCFOliveira/Gocene/queries/function/docvalues"
)

// DoubleConstValueSource returns a constant float64 for every document.
// Go port of org.apache.lucene.queries.function.valuesource.DoubleConstValueSource.
type DoubleConstValueSource struct {
	function.BaseValueSource
	constant float64
	fv       float32
	lv       int64
}

// NewDoubleConstValueSource builds a DoubleConstValueSource.
func NewDoubleConstValueSource(constant float64) *DoubleConstValueSource {
	return &DoubleConstValueSource{constant: constant, fv: float32(constant), lv: int64(constant)}
}

// Description renders "const(<value>)".
func (c *DoubleConstValueSource) Description() string { return fmt.Sprintf("const(%v)", c.constant) }

// Equals reports value equality.
func (c *DoubleConstValueSource) Equals(other function.ValueSource) bool {
	o, ok := other.(*DoubleConstValueSource)
	return ok && o.constant == c.constant
}

// HashCode mirrors Java's (int)(bits ^ (bits >>> 32)).
func (c *DoubleConstValueSource) HashCode() int32 { return hashFloat64(c.constant) }

// GetValues returns a DoubleDocValues view of the constant.
func (c *DoubleConstValueSource) GetValues(_ function.Context, _ *index.LeafReaderContext) (function.FunctionValues, error) {
	v := &doubleConstFunctionValues{
		DoubleDocValues: *docvalues.NewDoubleDocValues(c, func(_ int) (float64, error) { return c.constant, nil }),
		fv:              c.fv,
		lv:              c.lv,
		desc:            c.Description(),
		strRepr:         strconv.FormatFloat(c.constant, 'g', -1, 64),
		raw:             c.constant,
	}
	v.SetSelf(v)
	return v, nil
}

// String implements fmt.Stringer.
func (c *DoubleConstValueSource) String() string { return c.Description() }

// GetInt returns int32 narrowing of constant.
func (c *DoubleConstValueSource) GetInt() int32 { return int32(c.lv) }

// GetLong returns int64 narrowing of constant.
func (c *DoubleConstValueSource) GetLong() int64 { return c.lv }

// GetFloat returns float32 narrowing of constant.
func (c *DoubleConstValueSource) GetFloat() float32 { return c.fv }

// GetDouble returns the constant.
func (c *DoubleConstValueSource) GetDouble() float64 { return c.constant }

// GetNumber returns the constant as any.
func (c *DoubleConstValueSource) GetNumber() any { return c.constant }

// GetBool reports constant != 0.
func (c *DoubleConstValueSource) GetBool() bool { return c.constant != 0 }

type doubleConstFunctionValues struct {
	docvalues.DoubleDocValues
	fv      float32
	lv      int64
	desc    string
	strRepr string
	raw     float64
}

func (v *doubleConstFunctionValues) FloatVal(_ int) (float32, error) { return v.fv, nil }
func (v *doubleConstFunctionValues) IntVal(_ int) (int32, error)     { return int32(v.lv), nil }
func (v *doubleConstFunctionValues) LongVal(_ int) (int64, error)    { return v.lv, nil }
func (v *doubleConstFunctionValues) StrVal(_ int) (string, error)    { return v.strRepr, nil }
func (v *doubleConstFunctionValues) ObjectVal(_ int) (any, error)    { return v.raw, nil }
func (v *doubleConstFunctionValues) ToString(_ int) (string, error)  { return v.desc, nil }

var _ ConstNumberSource = (*DoubleConstValueSource)(nil)
