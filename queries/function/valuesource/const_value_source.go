// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
	"github.com/FlavioCFOliveira/Gocene/queries/function/docvalues"
)

// ConstValueSource returns a constant float32 for every document.
// Go port of org.apache.lucene.queries.function.valuesource.ConstValueSource.
type ConstValueSource struct {
	function.BaseValueSource
	constant float32
	dv       float64
}

// NewConstValueSource builds a ConstValueSource with the supplied value.
func NewConstValueSource(constant float32) *ConstValueSource {
	return &ConstValueSource{constant: constant, dv: float64(constant)}
}

// Description renders "const(<value>)" identically to Lucene.
func (c *ConstValueSource) Description() string { return fmt.Sprintf("const(%v)", c.constant) }

// Equals reports value equality.
func (c *ConstValueSource) Equals(other function.ValueSource) bool {
	o, ok := other.(*ConstValueSource)
	return ok && o.constant == c.constant
}

// HashCode mirrors Float.floatToIntBits(constant) * 31.
func (c *ConstValueSource) HashCode() int32 { return hashFloat32(c.constant) * 31 }

// GetValues returns a FloatDocValues view of the constant, with overrides
// for DoubleVal (uses the precomputed dv) and ToString (returns the source
// description) — mirroring the anonymous FloatDocValues subclass Lucene
// constructs inside ConstValueSource.getValues.
func (c *ConstValueSource) GetValues(_ function.Context, _ *index.LeafReaderContext) (function.FunctionValues, error) {
	v := &constFloatValues{
		FloatDocValues: *docvalues.NewFloatDocValues(c, func(_ int) (float32, error) { return c.constant, nil }),
		dv:             c.dv,
		desc:           c.Description(),
	}
	v.SetSelf(v)
	return v, nil
}

// constFloatValues overrides DoubleVal/ToString on the embedded FloatDocValues.
type constFloatValues struct {
	docvalues.FloatDocValues
	dv   float64
	desc string
}

func (v *constFloatValues) DoubleVal(_ int) (float64, error) { return v.dv, nil }
func (v *constFloatValues) ToString(_ int) (string, error)   { return v.desc, nil }

// ToString is for fmt.Stringer.
func (c *ConstValueSource) String() string { return c.Description() }

// GetInt returns the constant as int32.
func (c *ConstValueSource) GetInt() int32 { return int32(c.constant) }

// GetLong returns the constant as int64.
func (c *ConstValueSource) GetLong() int64 { return int64(c.constant) }

// GetFloat returns the constant.
func (c *ConstValueSource) GetFloat() float32 { return c.constant }

// GetDouble returns the precomputed float64 widening.
func (c *ConstValueSource) GetDouble() float64 { return c.dv }

// GetNumber returns the constant as any.
func (c *ConstValueSource) GetNumber() any { return c.constant }

// GetBool reports constant != 0.
func (c *ConstValueSource) GetBool() bool { return c.constant != 0 }

var _ ConstNumberSource = (*ConstValueSource)(nil)
