// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import "github.com/FlavioCFOliveira/Gocene/queries/function"

// ProductFloatFunction returns the product of its components.
//
// Go port of org.apache.lucene.queries.function.valuesource.ProductFloatFunction.
type ProductFloatFunction struct {
	*MultiFloatFunction
}

// NewProductFloatFunction creates a ProductFloatFunction.
func NewProductFloatFunction(sources []function.ValueSource) *ProductFloatFunction {
	return &ProductFloatFunction{MultiFloatFunction: NewMultiFloatFunction(sources, "product")}
}

func (s *ProductFloatFunction) Func(doc int, valsArr []function.FunctionValues) (float32, error) {
	val := float32(1.0)
	for _, vals := range valsArr {
		fv, err := vals.FloatVal(doc)
		if err != nil {
			return 0, err
		}
		val *= fv
	}
	return val, nil
}

var _ function.ValueSource = (*ProductFloatFunction)(nil)
