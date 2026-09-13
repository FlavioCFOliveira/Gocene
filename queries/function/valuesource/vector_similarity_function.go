// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import (
	"fmt"
	"strconv"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
)

// VectorSimilarityFunction returns a similarity function between two knn vectors.
type VectorSimilarityFunction struct {
	function.BaseValueSource
	similarityFunction index.VectorSimilarityFunction
	vector1            function.ValueSource
	vector2            function.ValueSource
}

// NewVectorSimilarityFunction creates a new VectorSimilarityFunction.
func NewVectorSimilarityFunction(sim index.VectorSimilarityFunction, v1, v2 function.ValueSource) *VectorSimilarityFunction {
	return &VectorSimilarityFunction{
		similarityFunction: sim,
		vector1:            v1,
		vector2:            v2,
	}
}

// GetValues returns the similarity values for the given context and reader.
func (v *VectorSimilarityFunction) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	v1Vals, err := v.vector1.GetValues(ctx, readerContext)
	if err != nil {
		return nil, err
	}
	v2Vals, err := v.vector2.GetValues(ctx, readerContext)
	if err != nil {
		return nil, err
	}

	return &vectorSimilarityFunctionValues{
		parent: v,
		v1:     v1Vals,
		v2:     v2Vals,
	}, nil
}

// Equals reports value-equality.
func (v *VectorSimilarityFunction) Equals(other function.ValueSource) bool {
	that, ok := other.(*VectorSimilarityFunction)
	if !ok {
		return false
	}
	return v.vector1.Equals(that.vector1) && v.vector2.Equals(that.vector2)
}

// HashCode returns a stable hash.
func (v *VectorSimilarityFunction) HashCode() int32 {
	var h int32 = 17
	h = 31*h + v.vector1.HashCode()
	h = 31*h + v.vector2.HashCode()
	return h
}

// Description renders a human-readable representation.
func (v *VectorSimilarityFunction) Description() string {
	return fmt.Sprintf("%s(%s, %s)", v.similarityFunction.ID().String(), v.vector1.Description(), v.vector2.Description())
}

type vectorSimilarityFunctionValues struct {
	function.BaseFunctionValues
	parent *VectorSimilarityFunction
	v1, v2 function.FunctionValues
}

func (v *vectorSimilarityFunctionValues) FloatVal(doc int) (float32, error) {
	return v.parent.funcImpl(doc, v.v1, v.v2)
}

func (v *vectorSimilarityFunctionValues) DoubleVal(doc int) (float64, error) {
	fv, err := v.FloatVal(doc)
	if err != nil {
		return 0, err
	}
	return float64(fv), nil
}

func (v *vectorSimilarityFunctionValues) StrVal(doc int) (string, error) {
	fv, err := v.FloatVal(doc)
	if err != nil {
		return "", err
	}
	return strconv.FormatFloat(float64(fv), 'f', -1, 32), nil
}

func (v *vectorSimilarityFunctionValues) Exists(doc int) (bool, error) {
	e1, err := v.v1.Exists(doc)
	if err != nil {
		return false, err
	}
	e2, err := v.v2.Exists(doc)
	if err != nil {
		return false, err
	}
	return e1 && e2, nil
}

func (v *vectorSimilarityFunctionValues) ToString(doc int) (string, error) {
	val, err := v.StrVal(doc)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s = %s", v.parent.Description(), val), nil
}

// funcImpl is implemented by concrete VectorSimilarityFunction subtypes.
func (v *VectorSimilarityFunction) funcImpl(doc int, f1, f2 function.FunctionValues) (float32, error) {
	panic("VectorSimilarityFunction.funcImpl not implemented")
}

// ByteVectorSimilarityFunction returns a similarity function between two knn vectors with byte elements.
type ByteVectorSimilarityFunction struct {
	VectorSimilarityFunction
}

func NewByteVectorSimilarityFunction(sim index.VectorSimilarityFunction, v1, v2 function.ValueSource) *ByteVectorSimilarityFunction {
	return &ByteVectorSimilarityFunction{
		VectorSimilarityFunction: *NewVectorSimilarityFunction(sim, v1, v2),
	}
}

func (v *ByteVectorSimilarityFunction) funcImpl(doc int, f1, f2 function.FunctionValues) (float32, error) {
	vec1, err := f1.ByteVectorVal(doc)
	if err != nil {
		return 0, err
	}
	vec2, err := f2.ByteVectorVal(doc)
	if err != nil {
		return 0, err
	}

	if vec1 == nil || vec2 == nil {
		return 0, nil
	}

	if len(vec1) != len(vec2) {
		return 0, fmt.Errorf("vectors must have the same length: %d != %d", len(vec1), len(vec2))
	}

	return v.similarityFunction.CompareBytes(vec1, vec2), nil
}

// FloatVectorSimilarityFunction returns a similarity function between two knn vectors with float elements.
type FloatVectorSimilarityFunction struct {
	VectorSimilarityFunction
}

func NewFloatVectorSimilarityFunction(sim index.VectorSimilarityFunction, v1, v2 function.ValueSource) *FloatVectorSimilarityFunction {
	return &FloatVectorSimilarityFunction{
		VectorSimilarityFunction: *NewVectorSimilarityFunction(sim, v1, v2),
	}
}

func (v *FloatVectorSimilarityFunction) funcImpl(doc int, f1, f2 function.FunctionValues) (float32, error) {
	vec1, err := f1.FloatVectorVal(doc)
	if err != nil {
		return 0, err
	}
	vec2, err := f2.FloatVectorVal(doc)
	if err != nil {
		return 0, err
	}

	if vec1 == nil || vec2 == nil {
		return 0, nil
	}

	if len(vec1) != len(vec2) {
		return 0, fmt.Errorf("vectors must have the same length: %d != %d", len(vec1), len(vec2))
	}

	return v.similarityFunction.CompareFloat(vec1, vec2), nil
}
