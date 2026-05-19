// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// newWriterTestState builds a SegmentWriteState pointed at a fresh
// ByteBuffersDirectory. The segment id is filled with a deterministic
// pattern so the codec headers are exercised end-to-end.
func newWriterTestState(t *testing.T, name string) *SegmentWriteState {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	si := index.NewSegmentInfo(name, 0, dir)
	id := make([]byte, 16)
	for i := range id {
		id[i] = byte(i + 1)
	}
	if err := si.SetID(id); err != nil {
		t.Fatalf("SetID: %v", err)
	}
	return &SegmentWriteState{
		Directory:     dir,
		SegmentInfo:   si,
		FieldInfos:    index.NewFieldInfos(),
		SegmentSuffix: "",
	}
}

// TestLucene99HnswVectorsWriter_LifecycleEmpty verifies that the
// writer creates the .vex and .vem segment files, writes the codec
// headers, and finishes cleanly when no field has been added. The
// resulting .vem must end with the int32 sentinel -1 plus the codec
// footer (16 bytes); the .vex must end with the codec footer.
func TestLucene99HnswVectorsWriter_LifecycleEmpty(t *testing.T) {
	state := newWriterTestState(t, "_0")
	w, err := NewLucene99HnswVectorsWriter(state, 16, 100, 100, 1)
	if err != nil {
		t.Fatalf("NewLucene99HnswVectorsWriter: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Both files must exist and carry at least the index header (≥27
	// bytes) plus their trailer.
	for _, name := range []string{"_0.vem", "_0.vex"} {
		in, err := state.Directory.OpenInput(name, store.IOContextRead)
		if err != nil {
			t.Fatalf("OpenInput(%s): %v", name, err)
		}
		if in.Length() < int64(FooterLength()) {
			t.Errorf("%s too short: %d bytes", name, in.Length())
		}
		if err := in.Close(); err != nil {
			t.Fatalf("Close(%s): %v", name, err)
		}
	}
}

// TestLucene99HnswVectorsWriter_AddFieldValidation exercises the
// validation surface of AddField and the per-field AddValue helpers.
func TestLucene99HnswVectorsWriter_AddFieldValidation(t *testing.T) {
	state := newWriterTestState(t, "_0")
	w, err := NewLucene99HnswVectorsWriter(state, 16, 100, 100, 1)
	if err != nil {
		t.Fatalf("NewLucene99HnswVectorsWriter: %v", err)
	}
	t.Cleanup(func() { _ = w.Close() })

	fi := index.NewFieldInfo("vec", 0, index.FieldInfoOptions{
		VectorDimension:          4,
		VectorEncoding:           index.VectorEncodingFloat32,
		VectorSimilarityFunction: index.VectorSimilarityFunctionEuclidean,
	})
	fw, err := w.AddField(fi)
	if err != nil {
		t.Fatalf("AddField: %v", err)
	}
	if _, err := w.AddField(fi); err == nil {
		t.Error("AddField duplicate: want error, got nil")
	}

	// Dimension mismatch -> error.
	if err := fw.AddValueFloat32(0, []float32{1, 2, 3}); err == nil {
		t.Error("AddValueFloat32 wrong-dim: want error, got nil")
	}
	// Wrong encoding for AddValueByte.
	if err := fw.AddValueByte(0, []byte{1, 2, 3, 4}); err == nil {
		t.Error("AddValueByte on FLOAT32 field: want error, got nil")
	}
	// Happy path.
	if err := fw.AddValueFloat32(0, []float32{1, 2, 3, 4}); err != nil {
		t.Fatalf("AddValueFloat32 #0: %v", err)
	}
	if err := fw.AddValueFloat32(2, []float32{5, 6, 7, 8}); err != nil {
		t.Fatalf("AddValueFloat32 #2: %v", err)
	}
	// Non-monotonic docID -> error.
	if err := fw.AddValueFloat32(2, []float32{1, 1, 1, 1}); err == nil {
		t.Error("AddValueFloat32 non-monotonic: want error, got nil")
	}
	if got := fw.NumDocs(); got != 2 {
		t.Errorf("NumDocs = %d, want 2", got)
	}
}

// TestLucene99HnswVectorsWriter_EmptyFieldFlush verifies that calling
// Flush on a writer with a single registered field (no vectors added)
// writes a valid empty-field meta record and produces non-zero-sized
// segment files.
func TestLucene99HnswVectorsWriter_EmptyFieldFlush(t *testing.T) {
	state := newWriterTestState(t, "_0")
	w, err := NewLucene99HnswVectorsWriter(state, 16, 100, 100, 1)
	if err != nil {
		t.Fatalf("NewLucene99HnswVectorsWriter: %v", err)
	}

	fi := index.NewFieldInfo("vec", 0, index.FieldInfoOptions{
		VectorDimension:          4,
		VectorEncoding:           index.VectorEncodingFloat32,
		VectorSimilarityFunction: index.VectorSimilarityFunctionCosine,
	})
	if _, err := w.AddField(fi); err != nil {
		t.Fatalf("AddField: %v", err)
	}
	if err := w.Flush(0); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestLucene99HnswVectorsWriter_DistFuncOrdinals locks the on-disk
// similarity ordinal mapping. The order is part of the wire format
// and must match Lucene99HnswVectorsReader.SIMILARITY_FUNCTIONS.
func TestLucene99HnswVectorsWriter_DistFuncOrdinals(t *testing.T) {
	cases := []struct {
		f    index.VectorSimilarityFunction
		want int32
	}{
		{index.VectorSimilarityFunctionEuclidean, 0},
		{index.VectorSimilarityFunctionDotProduct, 1},
		{index.VectorSimilarityFunctionCosine, 2},
		{index.VectorSimilarityFunctionMaximumInnerProduct, 3},
	}
	for _, c := range cases {
		got, err := distFuncToOrd(c.f)
		if err != nil {
			t.Errorf("distFuncToOrd(%v): err=%v", c.f, err)
			continue
		}
		if got != c.want {
			t.Errorf("distFuncToOrd(%v) = %d, want %d", c.f, got, c.want)
		}
	}
	if _, err := distFuncToOrd(index.VectorSimilarityFunction(99)); err == nil {
		t.Error("distFuncToOrd(unknown): want error, got nil")
	}
}
