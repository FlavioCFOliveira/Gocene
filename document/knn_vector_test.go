// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"bytes"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

func TestKnnFloatVectorField_Basic(t *testing.T) {
	v := []float32{1.0, 2.0, 3.0}
	f, err := NewKnnFloatVectorField("vec", v, index.VectorSimilarityFunctionCosine)
	if err != nil {
		t.Fatal(err)
	}
	got := f.VectorValue()
	if len(got) != 3 || got[0] != 1.0 || got[2] != 3.0 {
		t.Fatalf("VectorValue = %v", got)
	}
	if f.FieldType().GetVectorDimension() != 3 {
		t.Fatalf("dim = %d", f.FieldType().GetVectorDimension())
	}
	if f.FieldType().GetVectorEncoding() != index.VectorEncodingFloat32 {
		t.Fatalf("encoding = %v", f.FieldType().GetVectorEncoding())
	}
	// Encoded length = 12 bytes (3 * 4)
	if len(f.BinaryValue()) != 12 {
		t.Fatalf("encoded len = %d", len(f.BinaryValue()))
	}
}

func TestKnnFloatVectorField_DefaultEuclidean(t *testing.T) {
	f, err := NewKnnFloatVectorFieldEuclidean("vec", []float32{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	if f.FieldType().GetVectorSimilarityFunction() != index.VectorSimilarityFunctionEuclidean {
		t.Fatalf("similarity = %v", f.FieldType().GetVectorSimilarityFunction())
	}
}

func TestKnnFloatVectorField_DimMismatch(t *testing.T) {
	ft := KnnFloatVectorFieldType(3, index.VectorSimilarityFunctionEuclidean)
	if _, err := NewKnnFloatVectorFieldWithType("vec", []float32{1, 2}, ft); err == nil {
		t.Fatalf("expected error for vector length != FieldType dim")
	}
}

func TestKnnFloatVectorField_EmptyErrors(t *testing.T) {
	if _, err := NewKnnFloatVectorField("vec", nil, index.VectorSimilarityFunctionCosine); err == nil {
		t.Fatalf("expected error for empty vector")
	}
}

func TestKnnByteVectorField_Basic(t *testing.T) {
	v := []byte{1, 2, 3, 4}
	f, err := NewKnnByteVectorField("vec", v, index.VectorSimilarityFunctionDotProduct)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(f.VectorValue(), v) {
		t.Fatalf("VectorValue = %v", f.VectorValue())
	}
	if f.FieldType().GetVectorEncoding() != index.VectorEncodingByte {
		t.Fatalf("encoding = %v", f.FieldType().GetVectorEncoding())
	}
}

func TestKnnByteVectorField_DefaultEuclidean(t *testing.T) {
	f, err := NewKnnByteVectorFieldEuclidean("vec", []byte{0, 1, 2})
	if err != nil {
		t.Fatal(err)
	}
	if f.FieldType().GetVectorSimilarityFunction() != index.VectorSimilarityFunctionEuclidean {
		t.Fatalf("similarity wrong")
	}
}

func TestKnnByteVectorField_WrongEncodingErrors(t *testing.T) {
	ft := KnnFloatVectorFieldType(3, index.VectorSimilarityFunctionEuclidean)
	if _, err := NewKnnByteVectorFieldWithType("vec", []byte{1, 2, 3}, ft); err == nil {
		t.Fatalf("expected error for FLOAT32 FieldType passed to byte ctor")
	}
}
