// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import "github.com/FlavioCFOliveira/Gocene/queries/function"

// DivFloatFunction divides "a" by "b".
//
// Go port of org.apache.lucene.queries.function.valuesource.DivFloatFunction.
type DivFloatFunction struct {
	*DualFloatFunction
}

// NewDivFloatFunction creates a DivFloatFunction.
func NewDivFloatFunction(a, b function.ValueSource) *DivFloatFunction {
	return &DivFloatFunction{DualFloatFunction: NewDualFloatFunction(a, b, "div")}
}

func (s *DivFloatFunction) Func(doc int, aVal, bVal float32) float32 {
	return aVal / bVal
}

var _ function.ValueSource = (*DivFloatFunction)(nil)
