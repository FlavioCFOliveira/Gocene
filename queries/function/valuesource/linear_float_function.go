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

// LinearFloatFunction implements f(x) = slope * source + intercept.
//
// Go port of org.apache.lucene.queries.function.valuesource.LinearFloatFunction.
type LinearFloatFunction struct {
	function.BaseValueSource
	Source     function.ValueSource
	Slope      float32
	Intercept  float32
}

// NewLinearFloatFunction creates a LinearFloatFunction.
func NewLinearFloatFunction(source function.ValueSource, slope, intercept float32) *LinearFloatFunction {
	return &LinearFloatFunction{Source: source, Slope: slope, Intercept: intercept}
}

// Description returns "slope*float(source)+intercept".
func (s *LinearFloatFunction) Description() string {
	return fmt.Sprintf("%v*float(%s)+%v", s.Slope, s.Source.Description(), s.Intercept)
}

// CreateWeight delegates to the source.
func (s *LinearFloatFunction) CreateWeight(ctx function.Context, searcher any) error {
	return s.Source.CreateWeight(ctx, searcher)
}

// GetValues returns FunctionValues that compute slope * source + intercept.
func (s *LinearFloatFunction) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	vals, err := s.Source.GetValues(ctx, readerContext)
	if err != nil {
		return nil, err
	}
	v := &linearFloatValues{
		FloatDocValues: *docvalues.NewFloatDocValues(s, func(doc int) (float32, error) {
			fv, err := vals.FloatVal(doc)
			if err != nil {
				return 0, err
			}
			return fv*s.Slope + s.Intercept, nil
		}),
		vals: vals,
		vs:   s,
	}
	v.SetSelf(v)
	return v, nil
}

// Equals reports value equality.
func (s *LinearFloatFunction) Equals(other function.ValueSource) bool {
	o, ok := other.(*LinearFloatFunction)
	if !ok || o == nil {
		return false
	}
	return s.Slope == o.Slope && s.Intercept == o.Intercept && s.Source.Equals(o.Source)
}

// HashCode returns a stable hash.
func (s *LinearFloatFunction) HashCode() int32 {
	h := hashFloat32(s.Slope)
	h = (h >> 2) | (h << 30)
	h += hashFloat32(s.Intercept)
	h ^= (h << 14) | (h >> 19)
	return h + s.Source.HashCode()
}

type linearFloatValues struct {
	docvalues.FloatDocValues
	vals function.FunctionValues
	vs   *LinearFloatFunction
}

func (v *linearFloatValues) ToString(doc int) (string, error) {
	vs, err := v.vals.ToString(doc)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%v*float(%s)+%v", v.vs.Slope, vs, v.vs.Intercept), nil
}

var _ function.ValueSource = (*LinearFloatFunction)(nil)
