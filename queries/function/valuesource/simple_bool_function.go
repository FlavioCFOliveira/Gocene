// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
	"github.com/FlavioCFOliveira/Gocene/queries/function/docvalues"
)

// SimpleBoolFunction is a BoolFunction that applies an extendible boolean
// function to the values of a single wrapped ValueSource.
//
// Go port of org.apache.lucene.queries.function.valuesource.SimpleBoolFunction.
type SimpleBoolFunction struct {
	*BoolFunction
	Source function.ValueSource
	name   string
}

// NewSimpleBoolFunction creates a SimpleBoolFunction.
func NewSimpleBoolFunction(source function.ValueSource, name string) *SimpleBoolFunction {
	return &SimpleBoolFunction{
		BoolFunction: &BoolFunction{},
		Source:       source,
		name:         name,
	}
}

// Name returns the function name.
func (s *SimpleBoolFunction) Name() string { return s.name }

// Description returns "name(source)".
func (s *SimpleBoolFunction) Description() string {
	return s.name + "(" + s.Source.Description() + ")"
}

// CreateWeight delegates to the source.
func (s *SimpleBoolFunction) CreateWeight(ctx function.Context, searcher any) error {
	return s.Source.CreateWeight(ctx, searcher)
}

// GetValues returns BoolDocValues applying Func.
func (s *SimpleBoolFunction) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	vals, err := s.Source.GetValues(ctx, readerContext)
	if err != nil {
		return nil, err
	}
	v := &simpleBoolFunctionValues{
		BoolDocValues: *docvalues.NewBoolDocValues(s, func(doc int) (bool, error) {
			return s.Func(doc, vals)
		}),
		vals: vals,
		name: s.name,
	}
	v.SetSelf(v)
	return v, nil
}

// Func applies the boolean function. Concrete types implement this.
func (s *SimpleBoolFunction) Func(doc int, vals function.FunctionValues) (bool, error) {
	return false, nil
}

// Equals reports value equality.
func (s *SimpleBoolFunction) Equals(other function.ValueSource) bool {
	o, ok := other.(*SimpleBoolFunction)
	if !ok || o == nil {
		return false
	}
	return s.name == o.name && s.Source.Equals(o.Source)
}

// HashCode returns a stable hash.
func (s *SimpleBoolFunction) HashCode() int32 {
	return s.Source.HashCode() + hashString(s.name)
}

type simpleBoolFunctionValues struct {
	docvalues.BoolDocValues
	vals function.FunctionValues
	name string
}

func (v *simpleBoolFunctionValues) ToString(doc int) (string, error) {
	vs, err := v.vals.ToString(doc)
	if err != nil {
		return "", err
	}
	return v.name + "(" + vs + ")", nil
}

var _ function.ValueSource = (*SimpleBoolFunction)(nil)
