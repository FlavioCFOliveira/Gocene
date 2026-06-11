// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"math"
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// Unit-test replacements for the Lucene-origin TestManyKnnDocs monster suite.
//
// The upstream tests (testSameVectorIndexedMultipleTimes, testLargeSegment)
// require a full IndexWriter + HNSW codec + FSDirectory to exercise the
// HNSW indexing pipeline at scale. Those integration tests are deferred until
// the IndexWriter and HNSW writer wiring are complete.
//
// These replacements exercise the KnnFloatVectorField construction, Document
// integration, vector encoding round-trip, and dimension validation logic
// that IS available today — without any IndexWriter dependency.

// TestManyKnnDocs_SameVectorIndexedMultipleTimes verifies that:
//   - A KnnFloatVectorField can be constructed with a 16-dimensional vector
//   - Multiple KnnFloatVectorField instances can be added to a single Document
//   - The same vector can be indexed multiple times in different fields with
//     different names
//   - Vector values are correctly retrievable from each field
//   - Binary encoding produces the expected byte length (4 bytes per float32)
func TestManyKnnDocs_SameVectorIndexedMultipleTimes(t *testing.T) {
	vec := []float32{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8,
		0.9, 1.0, 1.1, 1.2, 1.3, 1.4, 1.5, 1.6}

	t.Run("construct_field", func(t *testing.T) {
		f, err := NewKnnFloatVectorField("vec_a", vec, index.VectorSimilarityFunctionDotProduct)
		if err != nil {
			t.Fatal(err)
		}
		if f.FieldType().GetVectorDimension() != 16 {
			t.Fatalf("VectorDimension = %d, want 16", f.FieldType().GetVectorDimension())
		}
		if f.FieldType().GetVectorEncoding() != index.VectorEncodingFloat32 {
			t.Fatalf("VectorEncoding = %v, want FLOAT32", f.FieldType().GetVectorEncoding())
		}
		if f.FieldType().GetVectorSimilarityFunction() != index.VectorSimilarityFunctionDotProduct {
			t.Fatalf("similarity = %v, want DOT_PRODUCT", f.FieldType().GetVectorSimilarityFunction())
		}
		if got := f.VectorValue(); len(got) != 16 {
			t.Fatalf("VectorValue returned %d elements, want 16", len(got))
		}
	})

	t.Run("multiple_fields_same_vector", func(t *testing.T) {
		doc := NewDocument()

		names := []string{"field_a", "field_b", "field_c"}
		for _, name := range names {
			f, err := NewKnnFloatVectorField(name, vec, index.VectorSimilarityFunctionDotProduct)
			if err != nil {
				t.Fatalf("NewKnnFloatVectorField(%q): %v", name, err)
			}
			doc.Add(f)
		}

		if doc.Size() != 3 {
			t.Fatalf("document has %d fields, want 3", doc.Size())
		}

		// All three field names present in GetFieldNames
		nameSet := make(map[string]bool)
		for _, n := range doc.GetFieldNames() {
			nameSet[n] = true
		}
		for _, want := range names {
			if !nameSet[want] {
				t.Fatalf("GetFieldNames missing %q", want)
			}
		}
	})

	t.Run("vector_values_retrievable", func(t *testing.T) {
		doc := NewDocument()
		for _, name := range []string{"v1", "v2", "v3"} {
			f, err := NewKnnFloatVectorField(name, vec, index.VectorSimilarityFunctionDotProduct)
			if err != nil {
				t.Fatal(err)
			}
			doc.Add(f)
		}

		for _, name := range []string{"v1", "v2", "v3"} {
			raw := doc.Get(name)
			if raw == nil {
				t.Fatalf("Get(%q) returned nil", name)
			}
			kf, ok := raw.(*KnnFloatVectorField)
			if !ok {
				t.Fatalf("Get(%q) returned type %T, want *KnnFloatVectorField", name, raw)
			}
			got := kf.VectorValue()
			if len(got) != len(vec) {
				t.Fatalf("field %q: vector length = %d, want %d", name, len(got), len(vec))
			}
			for i := range vec {
				if got[i] != vec[i] {
					t.Fatalf("field %q: vector[%d] = %f, want %f", name, i, got[i], vec[i])
				}
			}
		}
	})

	t.Run("binary_encoding_length", func(t *testing.T) {
		f, err := NewKnnFloatVectorField("enc", vec, index.VectorSimilarityFunctionDotProduct)
		if err != nil {
			t.Fatal(err)
		}
		if got := len(f.BinaryValue()); got != 4*16 {
			t.Fatalf("BinaryValue length = %d, want %d (4 bytes per float32 * 16 dims)", got, 4*16)
		}
	})
}

// TestManyKnnDocs_LargeSegment verifies that:
//   - KnnFloatVectorField validates vector dimensions correctly (1-dim accepted)
//   - A large number of KnnFloatVectorField instances can be created and added
//     to a single Document
//   - Float32 vector values round-trip correctly through the encoding layer
//   - Dimension mismatches produce errors at field-construction time
//   - Nil and empty vectors produce errors
func TestManyKnnDocs_LargeSegment(t *testing.T) {
	t.Run("one_dim_vector", func(t *testing.T) {
		f, err := NewKnnFloatVectorField("oned", []float32{3.14}, index.VectorSimilarityFunctionEuclidean)
		if err != nil {
			t.Fatal(err)
		}
		if f.FieldType().GetVectorDimension() != 1 {
			t.Fatalf("VectorDimension = %d, want 1", f.FieldType().GetVectorDimension())
		}
		got := f.VectorValue()
		if len(got) != 1 || got[0] != 3.14 {
			t.Fatalf("VectorValue = %v, want [3.14]", got)
		}
	})

	t.Run("many_vectors_in_document", func(t *testing.T) {
		doc := NewDocument()
		n := 100
		for i := 0; i < n; i++ {
			v := []float32{float32(i), float32(i * 2)}
			f, err := NewKnnFloatVectorField(
				"vec_"+strconv.Itoa(i),
				v,
				index.VectorSimilarityFunctionCosine,
			)
			if err != nil {
				t.Fatalf("creating field %d: %v", i, err)
			}
			doc.Add(f)
		}
		if doc.Size() != n {
			t.Fatalf("document has %d fields, want %d", doc.Size(), n)
		}
		names := doc.GetFieldNames()
		if len(names) != n {
			t.Fatalf("GetFieldNames returned %d names, want %d", len(names), n)
		}
	})

	t.Run("float32_encoding_roundtrip", func(t *testing.T) {
		original := []float32{1.5, -2.5, 3.0, -4.0, 0.0, 1e-10, 1e10, 123.456}

		// Use the package-level encode function (visible because test is
		// in package document, not document_test).
		encoded := encodeFloat32Vector(original)
		if len(encoded) != 4*len(original) {
			t.Fatalf("encoded length = %d, want %d", len(encoded), 4*len(original))
		}

		// Manually decode back to float32 via little-endian uint32.
		decoded := make([]float32, len(original))
		for i := range original {
			bits := uint32(encoded[i*4]) |
				uint32(encoded[i*4+1])<<8 |
				uint32(encoded[i*4+2])<<16 |
				uint32(encoded[i*4+3])<<24
			decoded[i] = math.Float32frombits(bits)
		}

		for i := range original {
			if decoded[i] != original[i] {
				t.Fatalf("round-trip: decoded[%d] = %f, want %f", i, decoded[i], original[i])
			}
		}
	})

	t.Run("dimension_mismatch_error", func(t *testing.T) {
		ft := KnnFloatVectorFieldType(5, index.VectorSimilarityFunctionCosine)
		_, err := NewKnnFloatVectorFieldWithType("bad", []float32{1, 2, 3}, ft)
		if err == nil {
			t.Fatal("expected error for 3-dim vector with 5-dim FieldType")
		}
	})

	t.Run("empty_vector_errors", func(t *testing.T) {
		_, err := NewKnnFloatVectorField("nil", nil, index.VectorSimilarityFunctionCosine)
		if err == nil {
			t.Fatal("expected error for nil vector")
		}
		_, err = NewKnnFloatVectorField("empty", []float32{}, index.VectorSimilarityFunctionCosine)
		if err == nil {
			t.Fatal("expected error for empty vector slice")
		}
	})

	t.Run("encoding_mismatch_error", func(t *testing.T) {
		// Float vector field with Byte FieldType should fail
		ft := KnnByteVectorFieldType(3, index.VectorSimilarityFunctionEuclidean)
		_, err := NewKnnFloatVectorFieldWithType("bad_enc", []float32{1, 2, 3}, ft)
		if err == nil {
			t.Fatal("expected error for FLOAT32 vector with BYTE FieldType")
		}
	})
}
