// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import "github.com/FlavioCFOliveira/Gocene/queries/function"

// MaxFloatFunction returns the max of its components.
//
// Go port of org.apache.lucene.queries.function.valuesource.MaxFloatFunction.
type MaxFloatFunction struct {
	*MultiFloatFunction
}

// NewMaxFloatFunction creates a MaxFloatFunction.
func NewMaxFloatFunction(sources []function.ValueSource) *MaxFloatFunction {
	return &MaxFloatFunction{MultiFloatFunction: NewMultiFloatFunction(sources, "max")}
}

func (s *MaxFloatFunction) Func(doc int, valsArr []function.FunctionValues) (float32, error) {
	found := false
	var val float32
	for _, vals := range valsArr {
		exists, err := vals.Exists(doc)
		if err != nil {
			return 0, err
		}
		if !exists {
			continue
		}
		fv, err := vals.FloatVal(doc)
		if err != nil {
			return 0, err
		}
		if !found || fv > val {
			val = fv
			found = true
		}
	}
	if !found {
		return 0, nil
	}
	return val, nil
}

func (s *MaxFloatFunction) ExistsFunc(doc int, valsArr []function.FunctionValues) (bool, error) {
	return AnyExists(doc, valsArr)
}

var _ function.ValueSource = (*MaxFloatFunction)(nil)
