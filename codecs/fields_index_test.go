// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"testing"
)

// TestFieldsIndexImpl_Basic tests basic FieldsIndexImpl creation
func TestFieldsIndexImpl_Basic(t *testing.T) {
	chunks := []ChunkInfo{
		{StartDocID: 0, NumDocs: 10, StartPointer: 0, CompressedLength: 100, UncompressedLength: 200},
		{StartDocID: 10, NumDocs: 10, StartPointer: 100, CompressedLength: 100, UncompressedLength: 200},
	}

	index, err := NewFieldsIndexImpl(chunks)
	if err != nil {
		t.Fatalf("NewFieldsIndexImpl failed: %v", err)
	}

	if index.GetNumChunks() != 2 {
		t.Errorf("expected 2 chunks, got %d", index.GetNumChunks())
	}

	if index.GetDocsPerChunk() != 10 {
		t.Errorf("expected 10 docs per chunk, got %d", index.GetDocsPerChunk())
	}
}

// TestFieldsIndexImpl_Empty tests empty FieldsIndexImpl
func TestFieldsIndexImpl_Empty(t *testing.T) {
	index, err := NewFieldsIndexImpl(nil)
	if err != nil {
		t.Fatalf("NewFieldsIndexImpl failed: %v", err)
	}

	if index.GetNumChunks() != 0 {
		t.Errorf("expected 0 chunks, got %d", index.GetNumChunks())
	}

	if index.GetDocsPerChunk() != 0 {
		t.Errorf("expected 0 docs per chunk, got %d", index.GetDocsPerChunk())
	}
}

// TestFieldsIndexImpl_GetStartPointer tests GetStartPointer
func TestFieldsIndexImpl_GetStartPointer(t *testing.T) {
	chunks := []ChunkInfo{
		{StartDocID: 0, NumDocs: 10, StartPointer: 0, CompressedLength: 100, UncompressedLength: 200},
		{StartDocID: 10, NumDocs: 10, StartPointer: 100, CompressedLength: 100, UncompressedLength: 200},
	}

	index, err := NewFieldsIndexImpl(chunks)
	if err != nil {
		t.Fatalf("NewFieldsIndexImpl failed: %v", err)
	}

	// Test first chunk
	ptr, err := index.GetStartPointer(0)
	if err != nil {
		t.Errorf("GetStartPointer(0) failed: %v", err)
	}
	if ptr != 0 {
		t.Errorf("expected pointer 0, got %d", ptr)
	}

	// Test second chunk
	ptr, err = index.GetStartPointer(10)
	if err != nil {
		t.Errorf("GetStartPointer(10) failed: %v", err)
	}
	if ptr != 100 {
		t.Errorf("expected pointer 100, got %d", ptr)
	}

	// Test middle of first chunk
	ptr, err = index.GetStartPointer(5)
	if err != nil {
		t.Errorf("GetStartPointer(5) failed: %v", err)
	}
	if ptr != 0 {
		t.Errorf("expected pointer 0, got %d", ptr)
	}
}

// TestFieldsIndexImpl_GetStartPointer_OutOfRange tests out of range access
func TestFieldsIndexImpl_GetStartPointer_OutOfRange(t *testing.T) {
	chunks := []ChunkInfo{
		{StartDocID: 0, NumDocs: 10, StartPointer: 0, CompressedLength: 100, UncompressedLength: 200},
	}

	index, err := NewFieldsIndexImpl(chunks)
	if err != nil {
		t.Fatalf("NewFieldsIndexImpl failed: %v", err)
	}

	// Test negative docID
	_, err = index.GetStartPointer(-1)
	if err == nil {
		t.Error("expected error for negative docID")
	}

	// Test docID beyond range
	_, err = index.GetStartPointer(20)
	if err == nil {
		t.Error("expected error for out of range docID")
	}
}

// TestFieldsIndexImpl_GetChunkInfo tests GetChunkInfo
func TestFieldsIndexImpl_GetChunkInfo(t *testing.T) {
	chunks := []ChunkInfo{
		{StartDocID: 0, NumDocs: 10, StartPointer: 0, CompressedLength: 100, UncompressedLength: 200},
		{StartDocID: 10, NumDocs: 10, StartPointer: 100, CompressedLength: 150, UncompressedLength: 250},
	}

	index, err := NewFieldsIndexImpl(chunks)
	if err != nil {
		t.Fatalf("NewFieldsIndexImpl failed: %v", err)
	}

	// Get chunk info for first chunk
	info, err := index.GetChunkInfo(0)
	if err != nil {
		t.Errorf("GetChunkInfo(0) failed: %v", err)
	}
	if info.StartDocID != 0 {
		t.Errorf("expected StartDocID 0, got %d", info.StartDocID)
	}
	if info.NumDocs != 10 {
		t.Errorf("expected NumDocs 10, got %d", info.NumDocs)
	}

	// Get chunk info for second chunk
	info, err = index.GetChunkInfo(15)
	if err != nil {
		t.Errorf("GetChunkInfo(15) failed: %v", err)
	}
	if info.StartDocID != 10 {
		t.Errorf("expected StartDocID 10, got %d", info.StartDocID)
	}
	if info.CompressedLength != 150 {
		t.Errorf("expected CompressedLength 150, got %d", info.CompressedLength)
	}
}

// TestFieldsIndexImpl_GetTotalDocs tests GetTotalDocs
func TestFieldsIndexImpl_GetTotalDocs(t *testing.T) {
	chunks := []ChunkInfo{
		{StartDocID: 0, NumDocs: 10, StartPointer: 0, CompressedLength: 100, UncompressedLength: 200},
		{StartDocID: 10, NumDocs: 15, StartPointer: 100, CompressedLength: 100, UncompressedLength: 200},
	}

	index, err := NewFieldsIndexImpl(chunks)
	if err != nil {
		t.Fatalf("NewFieldsIndexImpl failed: %v", err)
	}

	totalDocs := index.GetTotalDocs()
	if totalDocs != 25 {
		t.Errorf("expected 25 total docs, got %d", totalDocs)
	}
}

// TestFieldsIndexImpl_Close tests Close
func TestFieldsIndexImpl_Close(t *testing.T) {
	chunks := []ChunkInfo{
		{StartDocID: 0, NumDocs: 10, StartPointer: 0, CompressedLength: 100, UncompressedLength: 200},
	}

	index, err := NewFieldsIndexImpl(chunks)
	if err != nil {
		t.Fatalf("NewFieldsIndexImpl failed: %v", err)
	}

	err = index.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// After close, operations should fail
	_, err = index.GetStartPointer(0)
	if err == nil {
		t.Error("expected error after close")
	}
}

// TestCompressingStoredFieldsIndex tests CompressingStoredFieldsIndex
func TestCompressingStoredFieldsIndex(t *testing.T) {
	chunks := []ChunkInfo{
		{StartDocID: 0, NumDocs: 10, StartPointer: 0, CompressedLength: 100, UncompressedLength: 200},
	}

	index, err := NewCompressingStoredFieldsIndex("test_segment", chunks)
	if err != nil {
		t.Fatalf("NewCompressingStoredFieldsIndex failed: %v", err)
	}

	if index.GetSegmentName() != "test_segment" {
		t.Errorf("expected segment name 'test_segment', got '%s'", index.GetSegmentName())
	}

	if index.GetIndexFileName() != "test_segment.fdx" {
		t.Errorf("expected index file name 'test_segment.fdx', got '%s'", index.GetIndexFileName())
	}

	if index.GetDataFileName() != "test_segment.fdt" {
		t.Errorf("expected data file name 'test_segment.fdt', got '%s'", index.GetDataFileName())
	}
}

// TestChunkInfo_Values tests ChunkInfo field values
func TestChunkInfo_Values(t *testing.T) {
	chunk := ChunkInfo{
		StartDocID:         100,
		NumDocs:            50,
		StartPointer:       1024,
		CompressedLength:   200,
		UncompressedLength: 400,
	}

	if chunk.StartDocID != 100 {
		t.Errorf("expected StartDocID 100, got %d", chunk.StartDocID)
	}
	if chunk.NumDocs != 50 {
		t.Errorf("expected NumDocs 50, got %d", chunk.NumDocs)
	}
	if chunk.StartPointer != 1024 {
		t.Errorf("expected StartPointer 1024, got %d", chunk.StartPointer)
	}
	if chunk.CompressedLength != 200 {
		t.Errorf("expected CompressedLength 200, got %d", chunk.CompressedLength)
	}
	if chunk.UncompressedLength != 400 {
		t.Errorf("expected UncompressedLength 400, got %d", chunk.UncompressedLength)
	}
}
