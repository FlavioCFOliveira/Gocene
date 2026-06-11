// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/queries/function"
)

// PowFloatFunction raises base "a" to exponent "b".
//
// Go port of org.apache.lucene.queries.function.valuesource.PowFloatFunction.
type PowFloatFunction struct {
	*DualFloatFunction
}

// NewPowFloatFunction creates a PowFloatFunction.
func NewPowFloatFunction(a, b function.ValueSource) *PowFloatFunction {
	return &PowFloatFunction{DualFloatFunction: NewDualFloatFunction(a, b, "pow")}
}

func (s *PowFloatFunction) Func(doc int, aVal, bVal float32) float32 {
	return float32(math.Pow(float64(aVal), float64(bVal)))
}

var _ function.ValueSource = (*PowFloatFunction)(nil)
