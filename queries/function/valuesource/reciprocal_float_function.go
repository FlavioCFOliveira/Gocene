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

// ReciprocalFloatFunction implements f(x) = a/(m*x+b).
//
// Go port of org.apache.lucene.queries.function.valuesource.ReciprocalFloatFunction.
type ReciprocalFloatFunction struct {
	function.BaseValueSource
	Source function.ValueSource
	M, A, B float32
}

// NewReciprocalFloatFunction creates a ReciprocalFloatFunction.
func NewReciprocalFloatFunction(source function.ValueSource, m, a, b float32) *ReciprocalFloatFunction {
	return &ReciprocalFloatFunction{Source: source, M: m, A: a, B: b}
}

// Description returns "a/(m*float(source)+b)".
func (s *ReciprocalFloatFunction) Description() string {
	return fmt.Sprintf("%v/(%v*float(%s)+%v)", s.A, s.M, s.Source.Description(), s.B)
}

// CreateWeight delegates to the source.
func (s *ReciprocalFloatFunction) CreateWeight(ctx function.Context, searcher any) error {
	return s.Source.CreateWeight(ctx, searcher)
}

// GetValues returns FunctionValues that compute a/(m*x+b).
func (s *ReciprocalFloatFunction) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	vals, err := s.Source.GetValues(ctx, readerContext)
	if err != nil {
		return nil, err
	}
	v := &reciprocalFloatValues{
		FloatDocValues: *docvalues.NewFloatDocValues(s, func(doc int) (float32, error) {
			fv, err := vals.FloatVal(doc)
			if err != nil {
				return 0, err
			}
			return s.A / (s.M*fv + s.B), nil
		}),
		vals: vals,
		vs:   s,
	}
	v.SetSelf(v)
	return v, nil
}

// Equals reports value equality.
func (s *ReciprocalFloatFunction) Equals(other function.ValueSource) bool {
	o, ok := other.(*ReciprocalFloatFunction)
	if !ok || o == nil {
		return false
	}
	return s.M == o.M && s.A == o.A && s.B == o.B && s.Source.Equals(o.Source)
}

// HashCode returns a stable hash.
func (s *ReciprocalFloatFunction) HashCode() int32 {
	h := hashFloat32(s.A) + hashFloat32(s.M)
	h ^= (h << 13) | (h >> 20)
	h += hashFloat32(s.B) + s.Source.HashCode()
	return h
}

type reciprocalFloatValues struct {
	docvalues.FloatDocValues
	vals function.FunctionValues
	vs   *ReciprocalFloatFunction
}

func (v *reciprocalFloatValues) ToString(doc int) (string, error) {
	vs, err := v.vals.ToString(doc)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%v/(%v*float(%s)+%v)", v.vs.A, v.vs.M, vs, v.vs.B), nil
}

var _ function.ValueSource = (*ReciprocalFloatFunction)(nil)
