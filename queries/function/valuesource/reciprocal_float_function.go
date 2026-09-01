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

// ReciprocalFloatFunction implements a reciprocal function f(x) = a/(mx+b), based on
// the float value of a field or function.
//
// Go port of org.apache.lucene.queries.function.valuesource.ReciprocalFloatFunction.
type ReciprocalFloatFunction struct {
	function.BaseValueSource
	source function.ValueSource
	m        float32
	a        float32
	b        float32
}

// NewReciprocalFloatFunction creates a ReciprocalFloatFunction.
// Formula: f(source) = a/(m*float(source)+b)
func NewReciprocalFloatFunction(source function.ValueSource, m, a, b float32) *ReciprocalFloatFunction {
	return &ReciprocalFloatFunction{
		source: source,
		m:      m,
		a:      a,
		b:      b,
	}
}

func (r *ReciprocalFloatFunction) Description() string {
	return fmt.Sprintf("%f/(%f*float(%s)+%f)", r.a, r.m, r.source.Description(), r.b)
}

func (r *ReciprocalFloatFunction) CreateWeight(ctx function.Context, searcher any) error {
	return r.source.CreateWeight(ctx, searcher)
}

func (r *ReciprocalFloatFunction) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	vals, err := r.source.GetValues(ctx, readerContext)
	if err != nil {
		return nil, err
	}
	v := &reciprocalFloatFunctionValues{
		FloatDocValues: *docvalues.NewFloatDocValues(r, func(doc int) (float32, error) {
			fv, err := vals.FloatVal(doc)
			if err != nil {
				return 0, err
			}
			return r.a / (r.m*fv + r.b), nil
		}),
		vals: vals,
		self: r,
	}
	v.SetSelf(v)
	return v, nil
}

func (r *ReciprocalFloatFunction) Equals(other function.ValueSource) bool {
	o, ok := other.(*ReciprocalFloatFunction)
	if !ok || o == nil {
		return false
	}
	return r.m == o.m && r.a == o.a && r.b == o.b && r.source.Equals(o.source)
}

func (r *ReciprocalFloatFunction) HashCode() int32 {
	h := hashFloat32(r.a) + hashFloat32(r.m)
	h ^= (h << 13) | (h >> 20)
	return h + hashFloat32(r.b) + r.source.HashCode()
}

type reciprocalFloatFunctionValues struct {
	docvalues.FloatDocValues
	vals function.FunctionValues
	self *ReciprocalFloatFunction
}

func (v *reciprocalFloatFunctionValues) ToString(doc int) (string, error) {
	fv, err := v.vals.ToString(doc)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%f/(%f*%s+%f)", v.self.a, v.self.m, fv, v.self.b), nil
}

var _ function.ValueSource = (*ReciprocalFloatFunction)(nil)
