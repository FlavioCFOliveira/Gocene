// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene94

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// TestLucene94HnswVectorsReader_GraphOffsets verifies graphOffsetsByLevel derivation.
func TestLucene94HnswVectorsReader_GraphOffsets(t *testing.T) {
	const m = 16
	const size = 100
	nodesLevel1 := make([]int32, 10)

	e := &lucene94FieldEntry{
		maxConn:      m,
		numLevels:    2,
		size:         size,
		nodesByLevel: [][]int32{nil, nodesLevel1},
	}

	e.graphOffsetsByLevel = make([]int64, e.numLevels)
	bytesLevel0 := int64(1+2*e.maxConn) * 4
	bytesOther := int64(1+e.maxConn) * 4
	for level := 0; level < e.numLevels; level++ {
		switch level {
		case 0:
			e.graphOffsetsByLevel[0] = 0
		case 1:
			e.graphOffsetsByLevel[1] = bytesLevel0 * int64(e.size)
		default:
			prevCount := int64(len(e.nodesByLevel[level-1]))
			e.graphOffsetsByLevel[level] = e.graphOffsetsByLevel[level-1] + bytesOther*prevCount
		}
	}

	if e.graphOffsetsByLevel[0] != 0 {
		t.Errorf("level 0: got %d, want 0", e.graphOffsetsByLevel[0])
	}
	// (1 + 2*16) * 4 * 100 = 33 * 4 * 100 = 13200
	wantL1 := int64(13200)
	if e.graphOffsetsByLevel[1] != wantL1 {
		t.Errorf("level 1: got %d, want %d", e.graphOffsetsByLevel[1], wantL1)
	}
}

// TestLucene94HnswVectorsReader_ValidateFieldEntry_Float32 verifies validation for FLOAT32.
func TestLucene94HnswVectorsReader_ValidateFieldEntry_Float32(t *testing.T) {
	opts := index.FieldInfoOptions{
		VectorDimension:          4,
		VectorEncoding:           index.VectorEncodingFloat32,
		VectorSimilarityFunction: index.VectorSimilarityFunctionEuclidean,
	}
	fi := index.NewFieldInfo("vec", 0, opts)
	e := &lucene94FieldEntry{
		dimension:          4,
		size:               10,
		vectorDataLength:   int64(10 * 4 * 4), // 10 * dim * 4 bytes
		vectorEncoding:     index.VectorEncodingFloat32,
		similarityFunction: index.VectorSimilarityFunctionEuclidean,
	}
	if err := validateLucene94FieldEntry(fi, e); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestLucene94HnswVectorsReader_ValidateFieldEntry_Byte verifies validation for BYTE encoding.
func TestLucene94HnswVectorsReader_ValidateFieldEntry_Byte(t *testing.T) {
	opts := index.FieldInfoOptions{
		VectorDimension:          8,
		VectorEncoding:           index.VectorEncodingByte,
		VectorSimilarityFunction: index.VectorSimilarityFunctionDotProduct,
	}
	fi := index.NewFieldInfo("bvec", 0, opts)
	e := &lucene94FieldEntry{
		dimension:          8,
		size:               5,
		vectorDataLength:   int64(5 * 8 * 1), // 5 * dim * 1 byte
		vectorEncoding:     index.VectorEncodingByte,
		similarityFunction: index.VectorSimilarityFunctionDotProduct,
	}
	if err := validateLucene94FieldEntry(fi, e); err != nil {
		t.Errorf("unexpected error for BYTE encoding: %v", err)
	}
}

// TestLucene94HnswVectorsReader_ValidateFieldEntry_DimensionMismatch verifies rejection.
func TestLucene94HnswVectorsReader_ValidateFieldEntry_DimensionMismatch(t *testing.T) {
	opts := index.FieldInfoOptions{
		VectorDimension:          8,
		VectorEncoding:           index.VectorEncodingFloat32,
		VectorSimilarityFunction: index.VectorSimilarityFunctionEuclidean,
	}
	fi := index.NewFieldInfo("vec", 0, opts)
	e := &lucene94FieldEntry{
		dimension:          4, // mismatch
		size:               10,
		vectorDataLength:   160,
		vectorEncoding:     index.VectorEncodingFloat32,
		similarityFunction: index.VectorSimilarityFunctionEuclidean,
	}
	if err := validateLucene94FieldEntry(fi, e); err == nil {
		t.Error("expected error for dimension mismatch")
	}
}

// TestLucene94HnswVectorsReader_CloseIdempotent verifies safe double-close.
func TestLucene94HnswVectorsReader_CloseIdempotent(t *testing.T) {
	r := &Lucene94HnswVectorsReader{
		fields:     make(map[int]*lucene94FieldEntry),
		fieldInfos: index.NewFieldInfos(),
	}
	if err := r.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

// TestLucene94HnswVectorsReader_CheckIntegrityClosedError confirms error on closed reader.
func TestLucene94HnswVectorsReader_CheckIntegrityClosedError(t *testing.T) {
	r := &Lucene94HnswVectorsReader{
		fields:     make(map[int]*lucene94FieldEntry),
		fieldInfos: index.NewFieldInfos(),
		closed:     true,
	}
	if err := r.CheckIntegrity(); err == nil {
		t.Fatal("expected error from CheckIntegrity on closed reader")
	}
}

// TestLucene94HnswVectorsReader_GetFieldEntryEncodingMismatch verifies encoding guard.
func TestLucene94HnswVectorsReader_GetFieldEntryEncodingMismatch(t *testing.T) {
	r := &Lucene94HnswVectorsReader{
		fields:     make(map[int]*lucene94FieldEntry),
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
	r.fields[fi.Number()] = &lucene94FieldEntry{
		vectorEncoding: index.VectorEncodingByte, // stored as BYTE
	}
	// ask for FLOAT32 — must get an error
	if _, err := r.GetFieldEntry("f", index.VectorEncodingFloat32); err == nil {
		t.Fatal("expected error for encoding mismatch")
	}
}
