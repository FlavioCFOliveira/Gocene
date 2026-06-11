// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
	"github.com/FlavioCFOliveira/Gocene/queries/function/docvalues"
)

// SimpleFloatFunction applies a float function to a single wrapped
// ValueSource. Concrete instances set the Func field.
//
// Go port of org.apache.lucene.queries.function.valuesource.SimpleFloatFunction.
type SimpleFloatFunction struct {
	*SingleFunction
	// Func applies the float transformation. Default returns val unchanged.
	Func func(doc int, val float32) float32
}

// NewSimpleFloatFunction returns a SimpleFloatFunction with the given source and name.
func NewSimpleFloatFunction(source function.ValueSource, name string) *SimpleFloatFunction {
	return &SimpleFloatFunction{
		SingleFunction: NewSingleFunction(source, name),
		Func:           func(_ int, val float32) float32 { return val },
	}
}

// GetValues returns FunctionValues that apply Func to the source value.
func (s *SimpleFloatFunction) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	vals, err := s.Source.GetValues(ctx, readerContext)
	if err != nil {
		return nil, err
	}
	v := &simpleFloatFunctionValues{
		FloatDocValues: *docvalues.NewFloatDocValues(s, func(doc int) (float32, error) {
			fv, err := vals.FloatVal(doc)
			if err != nil {
				return 0, err
			}
			return s.Func(doc, fv), nil
		}),
		vals: vals,
		name: s.Name(),
	}
	v.SetSelf(v)
	return v, nil
}

// Equals reports value equality.
func (s *SimpleFloatFunction) Equals(other function.ValueSource) bool {
	o, ok := other.(*SimpleFloatFunction)
	if !ok || o == nil {
		return false
	}
	return s.Source.Equals(o.Source) && s.Name() == o.Name()
}

// HashCode returns a stable hash.
func (s *SimpleFloatFunction) HashCode() int32 {
	return s.Source.HashCode() + hashString(s.Name())
}

type simpleFloatFunctionValues struct {
	docvalues.FloatDocValues
	vals function.FunctionValues
	name string
}

func (v *simpleFloatFunctionValues) ToString(doc int) (string, error) {
	vs, err := v.vals.ToString(doc)
	if err != nil {
		return "", err
	}
	return v.name + "(" + vs + ")", nil
}

var _ function.ValueSource = (*SimpleFloatFunction)(nil)
