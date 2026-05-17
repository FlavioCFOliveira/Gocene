// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// newFloatFieldInfo builds a minimal FieldInfo with a FLOAT32 vector field
// configured for the requested dimension.
func newFloatFieldInfo(name string, dim int) *index.FieldInfo {
	return index.NewFieldInfoBuilder(name, 0).
		SetVectorAttributes(dim, index.VectorEncodingFloat32, index.VectorSimilarityFunctionDotProduct).
		Build()
}

// newByteFieldInfo builds a minimal FieldInfo with a BYTE vector field
// configured for the requested dimension.
func newByteFieldInfo(name string, dim int) *index.FieldInfo {
	return index.NewFieldInfoBuilder(name, 0).
		SetVectorAttributes(dim, index.VectorEncodingByte, index.VectorSimilarityFunctionDotProduct).
		Build()
}

func TestBufferingKnnVectorsWriter_AddFloatFieldRoundTrip(t *testing.T) {
	t.Parallel()
	var seen []string
	hook := BufferingKnnVectorsHook{
		WriteFloatField: func(fi *index.FieldInfo, field *BufferedFloatVectorField) error {
			seen = append(seen, fi.Name())
			if got, want := len(field.DocIDs), 3; got != want {
				t.Errorf("field %q: docIDs=%d, want %d", fi.Name(), got, want)
			}
			return nil
		},
	}
	w := NewBufferingKnnVectorsWriter(nil, hook)
	fw, err := w.AddFloatField(newFloatFieldInfo("vec", 4))
	if err != nil {
		t.Fatalf("AddFloatField: %v", err)
	}
	for docID := 0; docID < 3; docID++ {
		if err := fw.AddValue(docID, []float32{1, 2, 3, 4}); err != nil {
			t.Fatalf("AddValue(%d): %v", docID, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if len(seen) != 1 || seen[0] != "vec" {
		t.Errorf("dispatched fields = %v, want [vec]", seen)
	}
}

func TestBufferingKnnVectorsWriter_OutOfOrderRejected(t *testing.T) {
	t.Parallel()
	hook := BufferingKnnVectorsHook{WriteFloatField: func(*index.FieldInfo, *BufferedFloatVectorField) error { return nil }}
	w := NewBufferingKnnVectorsWriter(nil, hook)
	fw, err := w.AddFloatField(newFloatFieldInfo("vec", 2))
	if err != nil {
		t.Fatalf("AddFloatField: %v", err)
	}
	if err := fw.AddValue(5, []float32{1, 2}); err != nil {
		t.Fatalf("AddValue(5): %v", err)
	}
	if err := fw.AddValue(4, []float32{3, 4}); err == nil {
		t.Errorf("expected out-of-order error, got nil")
	}
}

func TestBufferingKnnVectorsWriter_DimensionMismatchRejected(t *testing.T) {
	t.Parallel()
	hook := BufferingKnnVectorsHook{WriteFloatField: func(*index.FieldInfo, *BufferedFloatVectorField) error { return nil }}
	w := NewBufferingKnnVectorsWriter(nil, hook)
	fw, err := w.AddFloatField(newFloatFieldInfo("vec", 4))
	if err != nil {
		t.Fatalf("AddFloatField: %v", err)
	}
	if err := fw.AddValue(0, []float32{1, 2, 3}); err == nil {
		t.Errorf("expected dim mismatch error, got nil")
	}
}

func TestBufferingKnnVectorsWriter_AddByteField(t *testing.T) {
	t.Parallel()
	var seen []string
	hook := BufferingKnnVectorsHook{
		WriteByteField: func(fi *index.FieldInfo, field *BufferedByteVectorField) error {
			seen = append(seen, fi.Name())
			return nil
		},
	}
	w := NewBufferingKnnVectorsWriter(nil, hook)
	fw, err := w.AddByteField(newByteFieldInfo("bvec", 3))
	if err != nil {
		t.Fatalf("AddByteField: %v", err)
	}
	if err := fw.AddValue(0, []byte{1, 2, 3}); err != nil {
		t.Fatalf("AddValue: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if len(seen) != 1 || seen[0] != "bvec" {
		t.Errorf("dispatched fields = %v, want [bvec]", seen)
	}
}

func TestBufferingKnnVectorsWriter_DuplicateFieldRejected(t *testing.T) {
	t.Parallel()
	hook := BufferingKnnVectorsHook{WriteFloatField: func(*index.FieldInfo, *BufferedFloatVectorField) error { return nil }}
	w := NewBufferingKnnVectorsWriter(nil, hook)
	if _, err := w.AddFloatField(newFloatFieldInfo("dup", 2)); err != nil {
		t.Fatalf("first AddFloatField: %v", err)
	}
	if _, err := w.AddFloatField(newFloatFieldInfo("dup", 2)); err == nil {
		t.Errorf("expected duplicate-field error, got nil")
	}
}

func TestBufferingKnnVectorsWriter_OnFinishCalled(t *testing.T) {
	t.Parallel()
	called := false
	hook := BufferingKnnVectorsHook{
		WriteFloatField: func(*index.FieldInfo, *BufferedFloatVectorField) error { return nil },
		OnFinish:        func() error { called = true; return nil },
	}
	w := NewBufferingKnnVectorsWriter(nil, hook)
	if _, err := w.AddFloatField(newFloatFieldInfo("vec", 2)); err != nil {
		t.Fatalf("AddFloatField: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !called {
		t.Errorf("OnFinish was not invoked on Close")
	}
}

func TestBufferingKnnVectorsWriter_FieldOrderPreserved(t *testing.T) {
	t.Parallel()
	var seen []string
	hook := BufferingKnnVectorsHook{
		WriteFloatField: func(fi *index.FieldInfo, _ *BufferedFloatVectorField) error {
			seen = append(seen, fi.Name())
			return nil
		},
		WriteByteField: func(fi *index.FieldInfo, _ *BufferedByteVectorField) error {
			seen = append(seen, fi.Name())
			return nil
		},
	}
	w := NewBufferingKnnVectorsWriter(nil, hook)
	if _, err := w.AddFloatField(newFloatFieldInfo("a", 2)); err != nil {
		t.Fatalf("AddFloatField(a): %v", err)
	}
	if _, err := w.AddByteField(newByteFieldInfo("b", 3)); err != nil {
		t.Fatalf("AddByteField(b): %v", err)
	}
	if _, err := w.AddFloatField(newFloatFieldInfo("c", 4)); err != nil {
		t.Fatalf("AddFloatField(c): %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if got, want := seen, []string{"a", "b", "c"}; !stringSlicesEqual(got, want) {
		t.Errorf("dispatch order = %v, want %v", got, want)
	}
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
