// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// KnnByteVectorField is a dense int8 (byte) KNN vector field for
// similarity search via HNSW.
//
// Go port of Lucene 10.4.0's
// org.apache.lucene.document.KnnByteVectorField.
//
// Static query factories deferred — backlog #2698.
type KnnByteVectorField struct {
	*Field
	vector []byte
}

// NewKnnByteVectorField creates a new KnnByteVectorField with the given
// vector using the supplied similarity function.
func NewKnnByteVectorField(name string, vector []byte, similarity index.VectorSimilarityFunction) (*KnnByteVectorField, error) {
	if len(vector) == 0 {
		return nil, fmt.Errorf("vector cannot be empty")
	}
	ft := KnnByteVectorFieldType(len(vector), similarity)
	return newKnnByteVectorFieldFromType(name, vector, ft)
}

// NewKnnByteVectorFieldEuclidean creates a KnnByteVectorField with the
// default EUCLIDEAN similarity function.
func NewKnnByteVectorFieldEuclidean(name string, vector []byte) (*KnnByteVectorField, error) {
	return NewKnnByteVectorField(name, vector, index.VectorSimilarityFunctionEuclidean)
}

// NewKnnByteVectorFieldWithType creates a KnnByteVectorField from a
// pre-configured FieldType. The FieldType must declare VectorEncoding
// BYTE and a matching VectorDimension.
func NewKnnByteVectorFieldWithType(name string, vector []byte, ft *FieldType) (*KnnByteVectorField, error) {
	if ft == nil {
		return nil, fmt.Errorf("FieldType cannot be nil")
	}
	if ft.GetVectorEncoding() != index.VectorEncodingByte {
		return nil, fmt.Errorf("FieldType encoding %v != BYTE", ft.GetVectorEncoding())
	}
	if ft.GetVectorDimension() != len(vector) {
		return nil, fmt.Errorf("vector length %d != FieldType dimension %d", len(vector), ft.GetVectorDimension())
	}
	return newKnnByteVectorFieldFromType(name, vector, ft)
}

func newKnnByteVectorFieldFromType(name string, vector []byte, ft *FieldType) (*KnnByteVectorField, error) {
	if len(vector) == 0 {
		return nil, fmt.Errorf("vector cannot be empty")
	}
	dup := make([]byte, len(vector))
	copy(dup, vector)
	field, err := NewField(name, dup, ft)
	if err != nil {
		return nil, err
	}
	return &KnnByteVectorField{Field: field, vector: dup}, nil
}

// VectorValue returns the byte vector stored by this field (defensive copy).
func (f *KnnByteVectorField) VectorValue() []byte {
	out := make([]byte, len(f.vector))
	copy(out, f.vector)
	return out
}

// SetVectorValue replaces the field's vector. Panics if the new vector's
// dimensionality differs from the configured FieldType dimension.
func (f *KnnByteVectorField) SetVectorValue(value []byte) {
	if len(value) != f.FieldType().GetVectorDimension() {
		panic(fmt.Sprintf("vector length %d != FieldType dimension %d", len(value), f.FieldType().GetVectorDimension()))
	}
	dup := make([]byte, len(value))
	copy(dup, value)
	f.Field.SetBytesValue(dup)
	f.vector = dup
}

// KnnByteVectorFieldType creates the canonical FieldType for a
// KnnByteVectorField of the given dimensionality and similarity.
func KnnByteVectorFieldType(dimension int, similarity index.VectorSimilarityFunction) *FieldType {
	ft := NewFieldType()
	ft.SetVectorAttributes(dimension, index.VectorEncodingByte, similarity)
	ft.Freeze()
	return ft
}
