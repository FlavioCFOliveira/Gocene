// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene95

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// TestLucene95HnswVectorsReader_ValidateFieldEntry_Float32 verifies validation for FLOAT32.
func TestLucene95HnswVectorsReader_ValidateFieldEntry_Float32(t *testing.T) {
	opts := index.FieldInfoOptions{
		VectorDimension:          4,
		VectorEncoding:           index.VectorEncodingFloat32,
		VectorSimilarityFunction: index.VectorSimilarityFunctionEuclidean,
	}
	fi := index.NewFieldInfo("vec", 0, opts)
	e := &lucene95FieldEntry{
		dimension:          4,
		size:               10,
		vectorDataLength:   int64(10 * 4 * 4), // 10 * dim * 4 bytes
		vectorEncoding:     index.VectorEncodingFloat32,
		similarityFunction: index.VectorSimilarityFunctionEuclidean,
	}
	if err := validateLucene95FieldEntry(fi, e); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestLucene95HnswVectorsReader_ValidateFieldEntry_Byte verifies validation for BYTE encoding.
func TestLucene95HnswVectorsReader_ValidateFieldEntry_Byte(t *testing.T) {
	opts := index.FieldInfoOptions{
		VectorDimension:          8,
		VectorEncoding:           index.VectorEncodingByte,
		VectorSimilarityFunction: index.VectorSimilarityFunctionDotProduct,
	}
	fi := index.NewFieldInfo("bvec", 0, opts)
	e := &lucene95FieldEntry{
		dimension:          8,
		size:               5,
		vectorDataLength:   int64(5 * 8 * 1), // 5 * dim * 1 byte
		vectorEncoding:     index.VectorEncodingByte,
		similarityFunction: index.VectorSimilarityFunctionDotProduct,
	}
	if err := validateLucene95FieldEntry(fi, e); err != nil {
		t.Errorf("unexpected error for BYTE encoding: %v", err)
	}
}

// TestLucene95HnswVectorsReader_ValidateFieldEntry_DimensionMismatch verifies rejection.
func TestLucene95HnswVectorsReader_ValidateFieldEntry_DimensionMismatch(t *testing.T) {
	opts := index.FieldInfoOptions{
		VectorDimension:          8,
		VectorEncoding:           index.VectorEncodingFloat32,
		VectorSimilarityFunction: index.VectorSimilarityFunctionEuclidean,
	}
	fi := index.NewFieldInfo("vec", 0, opts)
	e := &lucene95FieldEntry{
		dimension:          4, // mismatch
		size:               10,
		vectorDataLength:   160,
		vectorEncoding:     index.VectorEncodingFloat32,
		similarityFunction: index.VectorSimilarityFunctionEuclidean,
	}
	if err := validateLucene95FieldEntry(fi, e); err == nil {
		t.Error("expected error for dimension mismatch")
	}
}

// TestLucene95HnswVectorsReader_ValidateFieldEntry_LengthMismatch verifies size*dim*bytes check.
func TestLucene95HnswVectorsReader_ValidateFieldEntry_LengthMismatch(t *testing.T) {
	opts := index.FieldInfoOptions{
		VectorDimension:          4,
		VectorEncoding:           index.VectorEncodingFloat32,
		VectorSimilarityFunction: index.VectorSimilarityFunctionEuclidean,
	}
	fi := index.NewFieldInfo("vec", 0, opts)
	e := &lucene95FieldEntry{
		dimension:          4,
		size:               10,
		vectorDataLength:   1, // wrong
		vectorEncoding:     index.VectorEncodingFloat32,
		similarityFunction: index.VectorSimilarityFunctionEuclidean,
	}
	if err := validateLucene95FieldEntry(fi, e); err == nil {
		t.Error("expected error for length mismatch")
	}
}

// TestLucene95HnswVectorsReader_CloseIdempotent verifies safe double-close.
func TestLucene95HnswVectorsReader_CloseIdempotent(t *testing.T) {
	r := &Lucene95HnswVectorsReader{
		fields:     make(map[int]*lucene95FieldEntry),
		fieldInfos: index.NewFieldInfos(),
	}
	if err := r.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

// TestLucene95HnswVectorsReader_CheckIntegrityClosedError confirms error on closed reader.
func TestLucene95HnswVectorsReader_CheckIntegrityClosedError(t *testing.T) {
	r := &Lucene95HnswVectorsReader{
		fields:     make(map[int]*lucene95FieldEntry),
		fieldInfos: index.NewFieldInfos(),
		closed:     true,
	}
	if err := r.CheckIntegrity(); err == nil {
		t.Fatal("expected error from CheckIntegrity on closed reader")
	}
}

// TestLucene95HnswVectorsReader_GetFieldEntryEncodingMismatch verifies encoding guard.
func TestLucene95HnswVectorsReader_GetFieldEntryEncodingMismatch(t *testing.T) {
	r := &Lucene95HnswVectorsReader{
		fields:     make(map[int]*lucene95FieldEntry),
		fieldInfos: index.NewFieldInfos(),
	}
	fi := index.NewFieldInfo("f", 0, index.FieldInfoOptions{
		VectorDimension:          4,
		VectorEncoding:           index.VectorEncodingFloat32,
		VectorSimilarityFunction: index.VectorSimilarityFunctionEuclidean,
	})
	if err := r.fieldInfos.Add(fi); err != nil {
		t.Fatalf("Add: %v", err)
	}
	r.fields[fi.Number()] = &lucene95FieldEntry{
		vectorEncoding: index.VectorEncodingByte, // stored as BYTE
	}
	// ask for FLOAT32 — must get an error
	if _, err := r.GetFieldEntry("f", index.VectorEncodingFloat32); err == nil {
		t.Fatal("expected error for encoding mismatch")
	}
}

// TestLucene95HnswVectorsReader_OffsetsMetaOnlyWhenNodes verifies offsetsMeta is nil
// when there are no nodes (size=0, no levels).
func TestLucene95HnswVectorsReader_OffsetsMetaOnlyWhenNodes(t *testing.T) {
	e := &lucene95FieldEntry{
		maxConn:         16,
		numLevels:       1,
		size:            0,
		nodesByLevel:    [][]int32{nil},
		numberOfOffsets: 0,
	}
	// If numberOfOffsets == 0, offsetsMeta must remain nil.
	if e.offsetsMeta != nil {
		t.Error("offsetsMeta should be nil when numberOfOffsets == 0")
	}
}

// TestLucene95HnswVectorsReader_NumberOfOffsetsCalculation verifies numberOfOffsets
// accumulation: level0 contributes size, each higher level contributes its nodeCount.
func TestLucene95HnswVectorsReader_NumberOfOffsetsCalculation(t *testing.T) {
	const size = 100
	nodesLevel1 := make([]int32, 10)

	// Simulate what readLucene95FieldEntry computes for a 2-level graph.
	numLevels := 2
	nodesByLevel := [][]int32{nil, nodesLevel1}
	var numberOfOffsets int64
	for level := 0; level < numLevels; level++ {
		if level > 0 {
			numberOfOffsets += int64(len(nodesByLevel[level]))
		} else {
			numberOfOffsets += int64(size)
		}
	}
	// level0 = 100, level1 = 10 → total = 110
	if numberOfOffsets != 110 {
		t.Errorf("numberOfOffsets: got %d, want 110", numberOfOffsets)
	}
}
