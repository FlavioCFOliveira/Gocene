// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// KnnFloatVectorField is a dense float32 KNN vector field for similarity
// search via HNSW (Hierarchical Navigable Small World).
//
// Go port of Lucene 10.4.0's
// org.apache.lucene.document.KnnFloatVectorField.
//
// Static query factories (NewVectorQuery returning a KnnFloatVectorQuery)
// are deferred — backlog #2698.
type KnnFloatVectorField struct {
	*Field
	vector []float32
}

// NewKnnFloatVectorField creates a new KnnFloatVectorField with the given
// vector using the supplied similarity function.
func NewKnnFloatVectorField(name string, vector []float32, similarity index.VectorSimilarityFunction) (*KnnFloatVectorField, error) {
	if len(vector) == 0 {
		return nil, fmt.Errorf("vector cannot be empty")
	}
	ft := KnnFloatVectorFieldType(len(vector), similarity)
	return newKnnFloatVectorFieldFromType(name, vector, ft)
}

// NewKnnFloatVectorFieldEuclidean creates a KnnFloatVectorField using the
// default EUCLIDEAN similarity. Mirrors Lucene's two-arg ctor.
func NewKnnFloatVectorFieldEuclidean(name string, vector []float32) (*KnnFloatVectorField, error) {
	return NewKnnFloatVectorField(name, vector, index.VectorSimilarityFunctionEuclidean)
}

// NewKnnFloatVectorFieldWithType creates a KnnFloatVectorField from a
// pre-configured FieldType. The FieldType must declare VectorEncoding
// FLOAT32 and a non-zero VectorDimension matching len(vector).
func NewKnnFloatVectorFieldWithType(name string, vector []float32, ft *FieldType) (*KnnFloatVectorField, error) {
	if ft == nil {
		return nil, fmt.Errorf("FieldType cannot be nil")
	}
	if ft.GetVectorEncoding() != index.VectorEncodingFloat32 {
		return nil, fmt.Errorf("FieldType encoding %v != FLOAT32", ft.GetVectorEncoding())
	}
	if ft.GetVectorDimension() != len(vector) {
		return nil, fmt.Errorf("vector length %d != FieldType dimension %d", len(vector), ft.GetVectorDimension())
	}
	return newKnnFloatVectorFieldFromType(name, vector, ft)
}

func newKnnFloatVectorFieldFromType(name string, vector []float32, ft *FieldType) (*KnnFloatVectorField, error) {
	if len(vector) == 0 {
		return nil, fmt.Errorf("vector cannot be empty")
	}
	encoded := encodeFloat32Vector(vector)
	field, err := NewField(name, encoded, ft)
	if err != nil {
		return nil, err
	}
	dup := make([]float32, len(vector))
	copy(dup, vector)
	return &KnnFloatVectorField{Field: field, vector: dup}, nil
}

// VectorValue returns the float32 vector stored by this field.
func (f *KnnFloatVectorField) VectorValue() []float32 {
	out := make([]float32, len(f.vector))
	copy(out, f.vector)
	return out
}

// SetVectorValue replaces the field's vector. Panics if the new vector
// dimensionality differs from the configured FieldType dimension.
func (f *KnnFloatVectorField) SetVectorValue(value []float32) {
	if len(value) != f.FieldType().GetVectorDimension() {
		panic(fmt.Sprintf("vector length %d != FieldType dimension %d", len(value), f.FieldType().GetVectorDimension()))
	}
	encoded := encodeFloat32Vector(value)
	f.Field.SetBytesValue(encoded)
	dup := make([]float32, len(value))
	copy(dup, value)
	f.vector = dup
}

// KnnFloatVectorFieldType creates the canonical FieldType for a
// KnnFloatVectorField of the given dimensionality and similarity.
// Mirrors Lucene's createFieldType helper.
func KnnFloatVectorFieldType(dimension int, similarity index.VectorSimilarityFunction) *FieldType {
	ft := NewFieldType()
	ft.SetVectorAttributes(dimension, index.VectorEncodingFloat32, similarity)
	ft.Freeze()
	return ft
}

func encodeFloat32Vector(v []float32) []byte {
	out := make([]byte, 4*len(v))
	for i, f := range v {
		binary.LittleEndian.PutUint32(out[i*4:], math.Float32bits(f))
	}
	return out
}
