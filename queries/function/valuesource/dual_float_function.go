// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
	"github.com/FlavioCFOliveira/Gocene/queries/function/docvalues"
)

// DualFloatFunction is a ValueSource that wraps two sources and applies
// a float function to their values.
//
// Go port of org.apache.lucene.queries.function.valuesource.DualFloatFunction.
type DualFloatFunction struct {
	function.BaseValueSource
	A    function.ValueSource
	B    function.ValueSource
	name string
}

// NewDualFloatFunction returns a DualFloatFunction wrapping a and b.
func NewDualFloatFunction(a, b function.ValueSource, name string) *DualFloatFunction {
	return &DualFloatFunction{A: a, B: b, name: name}
}

// Name returns the function name.
func (s *DualFloatFunction) Name() string { return s.name }

// Description returns "name(a,b)".
func (s *DualFloatFunction) Description() string {
	return s.name + "(" + s.A.Description() + "," + s.B.Description() + ")"
}

// CreateWeight delegates to both sources.
func (s *DualFloatFunction) CreateWeight(ctx function.Context, searcher any) error {
	if err := s.A.CreateWeight(ctx, searcher); err != nil {
		return err
	}
	return s.B.CreateWeight(ctx, searcher)
}

// GetValues returns FunctionValues that apply Func to the two source values.
func (s *DualFloatFunction) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	aVals, err := s.A.GetValues(ctx, readerContext)
	if err != nil {
		return nil, err
	}
	bVals, err := s.B.GetValues(ctx, readerContext)
	if err != nil {
		return nil, err
	}
	v := &dualFloatFunctionValues{
		FloatDocValues: *docvalues.NewFloatDocValues(s, func(doc int) (float32, error) {
			av, err := aVals.FloatVal(doc)
			if err != nil {
				return 0, err
			}
			bv, err := bVals.FloatVal(doc)
			if err != nil {
				return 0, err
			}
			return s.Func(doc, av, bv), nil
		}),
		aVals: aVals,
		bVals: bVals,
		name:  s.name,
	}
	v.SetSelf(v)
	return v, nil
}

// Func applies the float function. Concrete types implement this.
func (s *DualFloatFunction) Func(doc int, aVal, bVal float32) float32 { return 0 }

// Equals reports value equality.
func (s *DualFloatFunction) Equals(other function.ValueSource) bool {
	o, ok := other.(*DualFloatFunction)
	if !ok || o == nil {
		return false
	}
	return s.name == o.name && s.A.Equals(o.A) && s.B.Equals(o.B)
}

// HashCode returns a stable hash.
func (s *DualFloatFunction) HashCode() int32 {
	h := hashString(s.name)
	h ^= (h << 13) | (h >> 20)
	h += s.A.HashCode()
	h ^= (h << 23) | (h >> 10)
	h += s.B.HashCode()
	return h
}

type dualFloatFunctionValues struct {
	docvalues.FloatDocValues
	aVals, bVals function.FunctionValues
	name         string
}

func (v *dualFloatFunctionValues) ToString(doc int) (string, error) {
	as, err := v.aVals.ToString(doc)
	if err != nil {
		return "", err
	}
	bs, err := v.bVals.ToString(doc)
	if err != nil {
		return "", err
	}
	return v.name + "(" + as + "," + bs + ")", nil
}
