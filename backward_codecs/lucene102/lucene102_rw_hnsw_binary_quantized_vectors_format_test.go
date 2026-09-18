// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene102

import (
	"bytes"
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestLucene102RWHnswBinaryQuantizedVectorsFormat_Constructor verifies the
// HNSW binary quantized vectors format name, toString and parameter
// validation.
func TestLucene102RWHnswBinaryQuantizedVectorsFormat_Constructor(t *testing.T) {
	h := NewLucene102HnswBinaryQuantizedVectorsFormat()
	if got, want := h.Name(), "Lucene102HnswBinaryQuantizedVectorsFormat"; got != want {
		t.Errorf("Name = %q, want %q", got, want)
	}
	want := "Lucene102HnswBinaryQuantizedVectorsFormat(name=Lucene102HnswBinaryQuantizedVectorsFormat, maxConn=16, beamWidth=100, flatVectorFormat=" +
		lucene102HnswFlatVectorsFormat.String() + ")"
	if got := h.String(); got != want {
		t.Errorf("String = %q, want %q", got, want)
	}
	if _, err := NewLucene102HnswBinaryQuantizedVectorsFormatWithParams(0, 100, 1, nil); err == nil {
		t.Error("maxConn=0: want error, got nil")
	}
	if _, err := NewLucene102HnswBinaryQuantizedVectorsFormatWithParams(16, 0, 1, nil); err == nil {
		t.Error("beamWidth=0: want error, got nil")
	}
	if _, err := NewLucene102HnswBinaryQuantizedVectorsFormatWithParams(16, 100, 1, func(task func()) { task() }); err == nil {
		t.Error("numMergeWorkers=1 with an executor: want error, got nil")
	}
}

// TestLucene102RWHnswBinaryQuantizedVectorsFormat_OffHeapValues verifies that
// dense OffHeapBinarizedVectorValues read a binary code, its corrective terms
// and its unsigned quantized component sum as the format stores them.
func TestLucene102RWHnswBinaryQuantizedVectorsFormat_OffHeapValues(t *testing.T) {
	const dimension = 64
	dir := store.NewByteBuffersDirectory()
	conf := readOrdToDocConfiguration(t, dir, "dense", 1, 1)

	code := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	lower, upper, additional := float32(-0.5), float32(0.75), float32(1.25)
	out, err := dir.CreateOutput("veb", store.IOContextWrite)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	if err := out.WriteBytes(code, 0, len(code)); err != nil {
		t.Fatalf("WriteBytes: %v", err)
	}
	for _, f := range []float32{lower, upper, additional} {
		if err := out.WriteInt(int32(math.Float32bits(f))); err != nil {
			t.Fatalf("WriteInt: %v", err)
		}
	}
	if err := out.WriteShort(int16(-2)); err != nil { // 65534 unsigned
		t.Fatalf("WriteShort: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	in, err := dir.OpenInput("veb", store.IOContextRead)
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	defer func() {
		if err := in.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	}()

	values, err := loadOffHeapBinarizedVectorValues(
		conf, dimension, 1, nil, index.VectorSimilarityFunctionEuclidean,
		lucene102BinaryFlatVectorsScorer, make([]float32, dimension), 0, 0, in.Length(), in)
	if err != nil {
		t.Fatalf("loadOffHeapBinarizedVectorValues: %v", err)
	}
	if _, ok := values.(*DenseOffHeapVectorValues); !ok {
		t.Fatalf("load returned %T, want *DenseOffHeapVectorValues", values)
	}
	got, err := values.VectorValue(0)
	if err != nil {
		t.Fatalf("VectorValue: %v", err)
	}
	if !bytes.Equal(got, code) {
		t.Errorf("VectorValue = %v, want %v", got, code)
	}
	terms, err := values.GetCorrectiveTerms(0)
	if err != nil {
		t.Fatalf("GetCorrectiveTerms: %v", err)
	}
	if terms.LowerInterval != lower || terms.UpperInterval != upper ||
		terms.AdditionalCorrection != additional || terms.QuantizedComponentSum != 65534 {
		t.Errorf("GetCorrectiveTerms = %+v, want {%g %g %g 65534}", terms, lower, upper, additional)
	}
	if got := values.DiscretizedDimensions(); got != dimension {
		t.Errorf("DiscretizedDimensions = %d, want %d", got, dimension)
	}
}

// TestLucene102RWHnswBinaryQuantizedVectorsFormat_BinaryFormat verifies that
// the backward-compatibility formats refuse to write.
func TestLucene102RWHnswBinaryQuantizedVectorsFormat_BinaryFormat(t *testing.T) {
	if _, err := NewLucene102BinaryQuantizedVectorsFormat().FlatFieldsWriter(nil); err == nil {
		t.Error("Lucene102BinaryQuantizedVectorsFormat.FlatFieldsWriter: want error, got nil")
	}
	if _, err := NewLucene102HnswBinaryQuantizedVectorsFormat().FieldsWriter(nil); err == nil {
		t.Error("Lucene102HnswBinaryQuantizedVectorsFormat.FieldsWriter: want error, got nil")
	}
}
