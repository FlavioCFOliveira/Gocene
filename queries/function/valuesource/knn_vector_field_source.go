// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// ByteKnnVectorFieldSource is an implementation for retrieving FunctionValues
// instances for byte knn vectors fields.
type ByteKnnVectorFieldSource struct {
	function.BaseValueSource
	fieldName string
}

// NewByteKnnVectorFieldSource creates a new ByteKnnVectorFieldSource.
func NewByteKnnVectorFieldSource(fieldName string) *ByteKnnVectorFieldSource {
	return &ByteKnnVectorFieldSource{
		fieldName: fieldName,
	}
}

// GetValues returns the FunctionValues for the given context and reader.
func (v *ByteKnnVectorFieldSource) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	reader := readerContext.LeafReader()
	vectorValues := reader.GetByteVectorValues(v.fieldName)

	if vectorValues == nil {
		if err := CheckField(reader, v.fieldName, index.VectorEncodingByte); err != nil {
			return nil, err
		}

		return &byteKnnVectorFieldFunctionEmpty{
			VectorFieldFunction: *NewVectorFieldFunction(v),
		}, nil
	}

	return &byteKnnVectorFieldFunction{
		VectorFieldFunction: *NewVectorFieldFunction(v),
		vectorValues:        vectorValues,
		iterator:            vectorValues.Iterator(),
	}, nil
}

// Equals reports value-equality.
func (v *ByteKnnVectorFieldSource) Equals(other function.ValueSource) bool {
	that, ok := other.(*ByteKnnVectorFieldSource)
	if !ok {
		return false
	}
	return v.fieldName == that.fieldName
}

// HashCode returns a stable hash.
func (v *ByteKnnVectorFieldSource) HashCode() int32 {
	return int32(len(v.fieldName)) // Simplified hash
}

// Description renders a human-readable representation.
func (v *ByteKnnVectorFieldSource) Description() string {
	return fmt.Sprintf("ByteKnnVectorFieldSource(%s)", v.fieldName)
}

type byteKnnVectorFieldFunction struct {
	*VectorFieldFunction
	vectorValues index.ByteVectorValues
	iterator     util.DocIdSetIterator
}

func (v *byteKnnVectorFieldFunction) ByteVectorVal(doc int) ([]byte, error) {
	exists, err := v.Exists(doc)
	if err != nil {
		return nil, err
	}
	if exists {
		return v.vectorValues.VectorValue(v.iterator.Index()), nil
	}
	return nil, nil
}

func (v *byteKnnVectorFieldFunction) getVectorIterator() util.DocIdSetIterator {
	return v.iterator
}

type byteKnnVectorFieldFunctionEmpty struct {
	*VectorFieldFunction
}

func (v *byteKnnVectorFieldFunctionEmpty) ByteVectorVal(_ int) ([]byte, error) {
	return nil, nil
}

func (v *byteKnnVectorFieldFunctionEmpty) getVectorIterator() util.DocIdSetIterator {
	return search.NewEmptyDocIdSetIterator()
}

// FloatKnnVectorFieldSource is an implementation for retrieving FunctionValues
// instances for float knn vectors fields.
type FloatKnnVectorFieldSource struct {
	function.BaseValueSource
	fieldName string
}

// NewFloatKnnVectorFieldSource creates a new FloatKnnVectorFieldSource.
func NewFloatKnnVectorFieldSource(fieldName string) *FloatKnnVectorFieldSource {
	return &FloatKnnVectorFieldSource{
		fieldName: fieldName,
	}
}

// GetValues returns the FunctionValues for the given context and reader.
func (v *FloatKnnVectorFieldSource) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	reader := readerContext.LeafReader()
	vectorValues := reader.GetFloatVectorValues(v.fieldName)

	if vectorValues == nil {
		if err := CheckField(reader, v.fieldName, index.VectorEncodingFloat32); err != nil {
			return nil, err
		}

		return &floatKnnVectorFieldFunctionEmpty{
			VectorFieldFunction: *NewVectorFieldFunction(v),
		}, nil
	}

	return &floatKnnVectorFieldFunction{
		VectorFieldFunction: *NewVectorFieldFunction(v),
		vectorValues:        vectorValues,
		iterator:            vectorValues.Iterator(),
	}, nil
}

// Equals reports value-equality.
func (v *FloatKnnVectorFieldSource) Equals(other function.ValueSource) bool {
	that, ok := other.(*FloatKnnVectorFieldSource)
	if !ok {
		return false
	}
	return v.fieldName == that.fieldName
}

// HashCode returns a stable hash.
func (v *FloatKnnVectorFieldSource) HashCode() int32 {
	return int32(len(v.fieldName)) // Simplified hash
}

// Description renders a human-readable representation.
func (v *FloatKnnVectorFieldSource) Description() string {
	return fmt.Sprintf("FloatKnnVectorFieldSource(%s)", v.fieldName)
}

type floatKnnVectorFieldFunction struct {
	*VectorFieldFunction
	vectorValues index.FloatVectorValues
	iterator     util.DocIdSetIterator
}

func (v *floatKnnVectorFieldFunction) FloatVectorVal(doc int) ([]float32, error) {
	exists, err := v.Exists(doc)
	if err != nil {
		return nil, err
	}
	if exists {
		return v.vectorValues.VectorValue(v.iterator.Index()), nil
	}
	return nil, nil
}

func (v *floatKnnVectorFieldFunction) getVectorIterator() util.DocIdSetIterator {
	return v.iterator
}

type floatKnnVectorFieldFunctionEmpty struct {
	*VectorFieldFunction
}

func (v *floatKnnVectorFieldFunctionEmpty) FloatVectorVal(_ int) ([]float32, error) {
	return nil, nil
}

func (v *floatKnnVectorFieldFunctionEmpty) getVectorIterator() util.DocIdSetIterator {
	return search.NewEmptyDocIdSetIterator()
}
