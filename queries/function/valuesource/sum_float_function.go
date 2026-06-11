// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import "github.com/FlavioCFOliveira/Gocene/queries/function"

// SumFloatFunction returns the sum of its components.
//
// Go port of org.apache.lucene.queries.function.valuesource.SumFloatFunction.
type SumFloatFunction struct {
	*MultiFloatFunction
}

// NewSumFloatFunction creates a SumFloatFunction.
func NewSumFloatFunction(sources []function.ValueSource) *SumFloatFunction {
	return &SumFloatFunction{MultiFloatFunction: NewMultiFloatFunction(sources, "sum")}
}

func (s *SumFloatFunction) Func(doc int, valsArr []function.FunctionValues) (float32, error) {
	var val float32
	for _, vals := range valsArr {
		fv, err := vals.FloatVal(doc)
		if err != nil {
			return 0, err
		}
		val += fv
	}
	return val, nil
}

var _ function.ValueSource = (*SumFloatFunction)(nil)
