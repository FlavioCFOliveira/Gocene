// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import "github.com/FlavioCFOliveira/Gocene/queries/function"

// MinFloatFunction returns the min of its components.
//
// Go port of org.apache.lucene.queries.function.valuesource.MinFloatFunction.
type MinFloatFunction struct {
	*MultiFloatFunction
}

// NewMinFloatFunction creates a MinFloatFunction.
func NewMinFloatFunction(sources []function.ValueSource) *MinFloatFunction {
	return &MinFloatFunction{MultiFloatFunction: NewMultiFloatFunction(sources, "min")}
}

func (s *MinFloatFunction) Func(doc int, valsArr []function.FunctionValues) (float32, error) {
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
		if !found || fv < val {
			val = fv
			found = true
		}
	}
	if !found {
		return 0, nil
	}
	return val, nil
}

func (s *MinFloatFunction) ExistsFunc(doc int, valsArr []function.FunctionValues) (bool, error) {
	return AnyExists(doc, valsArr)
}

var _ function.ValueSource = (*MinFloatFunction)(nil)
