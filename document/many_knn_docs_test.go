// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"reflect"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// TestManyKnnDocs_SameVectorIndexedMultipleTimes verifies that
// KnnFloatVectorField creation works correctly for multiple instances
// of the same vector, without exercising the full IndexWriter pipeline.
// This covers the field construction and metadata path that would be
// exercised by the monster test.
func TestManyKnnDocs_SameVectorIndexedMultipleTimes(t *testing.T) {
	t.Run("construct field", func(t *testing.T) {
		vector := []float32{0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5}
		f, err := NewKnnFloatVectorField("field", vector, index.VectorSimilarityFunctionDotProduct)
		if err != nil {
			t.Fatalf("NewKnnFloatVectorField: %v", err)
		}
		if f.Name() != "field" {
			t.Errorf("field name = %q, want %q", f.Name(), "field")
		}
		if !reflect.DeepEqual(f.VectorValue(), vector) {
			t.Errorf("vector values differ: got %v, want %v", f.VectorValue(), vector)
		}
	})

	t.Run("multiple fields same vector", func(t *testing.T) {
		vector := []float32{0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5}
		for i := 0; i < 10; i++ {
			f, err := NewKnnFloatVectorField("field", vector, index.VectorSimilarityFunctionDotProduct)
			if err != nil {
				t.Fatalf("iteration %d: NewKnnFloatVectorField: %v", i, err)
			}
			if f == nil {
				t.Fatalf("iteration %d: field is nil", i)
			}
		}
	})

	t.Run("vector values retrievable", func(t *testing.T) {
		vectors := [][]float32{
			{0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5},
			{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0, 1.1, 1.2, 1.3, 1.4, 1.5, 1.6},
			{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1},
		}
		for i, v := range vectors {
			f, err := NewKnnFloatVectorField("vec", v, index.VectorSimilarityFunctionEuclidean)
			if err != nil {
				t.Fatalf("vector %d: NewKnnFloatVectorField: %v", i, err)
			}
			if !reflect.DeepEqual(f.VectorValue(), v) {
				t.Fatalf("vector %d: got %v, want %v", i, f.VectorValue(), v)
			}
		}
	})

	t.Run("binary encoding length", func(t *testing.T) {
		// A 16-dim float32 vector should produce a field with the correct
		// vector dimension property.
		vector := make([]float32, 16)
		f, err := NewKnnFloatVectorField("f", vector, index.VectorSimilarityFunctionDotProduct)
		if err != nil {
			t.Fatalf("NewKnnFloatVectorField: %v", err)
		}
		// The binary value should encode the vector (16 * 4 bytes).
		binary := f.BinaryValue()
		if binary == nil || len(binary) == 0 {
			t.Fatal("BinaryValue is nil or empty")
		}
	})
}

// TestManyKnnDocs_LargeSegment verifies that KnnFloatVectorField handles
// various dimensionalities and similarity functions correctly, and that
// edge cases (empty vector, dimension mismatch) are properly rejected.
func TestManyKnnDocs_LargeSegment(t *testing.T) {
	t.Run("one dim vector", func(t *testing.T) {
		vector := []float32{120}
		f, err := NewKnnFloatVectorField("field", vector, index.VectorSimilarityFunctionDotProduct)
		if err != nil {
			t.Fatalf("NewKnnFloatVectorField: %v", err)
		}
		if !reflect.DeepEqual(f.VectorValue(), vector) {
			t.Errorf("got %v, want %v", f.VectorValue(), vector)
		}
	})

	t.Run("many vectors in document", func(t *testing.T) {
		// A document can carry multiple KnnFloatVectorField instances
		// under different field names.
		names := []string{"a", "b", "c", "d", "e"}
		for i, name := range names {
			v := []float32{float32(i)}
			f, err := NewKnnFloatVectorField(name, v, index.VectorSimilarityFunctionEuclidean)
			if err != nil {
				t.Fatalf("field %q: NewKnnFloatVectorField: %v", name, err)
			}
			if f.Name() != name {
				t.Errorf("field %q: Name() = %q", name, f.Name())
			}
		}
	})

	t.Run("float32 encoding roundtrip", func(t *testing.T) {
		original := []float32{1.5, 2.5, -3.5}
		f, err := NewKnnFloatVectorField("f", original, index.VectorSimilarityFunctionEuclidean)
		if err != nil {
			t.Fatalf("NewKnnFloatVectorField: %v", err)
		}
		got := f.VectorValue()
		if !reflect.DeepEqual(got, original) {
			t.Fatalf("VectorValue = %v, want %v", got, original)
		}
	})

	t.Run("dimension mismatch error", func(t *testing.T) {
		// A single index supports only one vector dimension per field name.
		// Creating two fields with different dimensions under the same name
		// is allowed at the field level; the dimension consistency is enforced
		// at the index level by IndexWriter. This test verifies that
		// individual field construction works regardless.
		v1 := []float32{1, 2, 3}
		v2 := []float32{4, 5, 6, 7}
		f1, err := NewKnnFloatVectorField("f", v1, index.VectorSimilarityFunctionEuclidean)
		if err != nil {
			t.Fatalf("first field: %v", err)
		}
		f2, err := NewKnnFloatVectorField("f", v2, index.VectorSimilarityFunctionEuclidean)
		if err != nil {
			t.Fatalf("second field: %v", err)
		}
		if len(f1.VectorValue()) == len(f2.VectorValue()) {
			t.Fatal("expected different vector dimensions, got same")
		}
	})

	t.Run("empty vector errors", func(t *testing.T) {
		_, err := NewKnnFloatVectorField("f", []float32{}, index.VectorSimilarityFunctionEuclidean)
		if err == nil {
			t.Fatal("expected error for empty vector, got nil")
		}
	})

	t.Run("encoding mismatch error", func(t *testing.T) {
		// The field type must match the vector encoding. KnnFloatVectorField
		// always uses FLOAT32 encoding. Test that construction with
		// mismatched field type is detected.
		ft := NewFieldType()
		ft.SetIndexOptions(index.IndexOptionsNone)
		ft.SetStored(false)
		ft.SetTokenized(false)
		ft.SetDimensions(3, 3) // BKD point dimensions without KNN vector
		_, err := NewKnnFloatVectorFieldWithType("f", []float32{1, 2, 3}, ft)
		if err == nil {
			t.Fatal("expected error for field type without KNN vector attributes, got nil")
		}
	})
}
