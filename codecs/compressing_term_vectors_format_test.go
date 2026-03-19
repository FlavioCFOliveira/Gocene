// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestCompressingTermVectorsFormat_Basic tests basic format creation
func TestCompressingTermVectorsFormat_Basic(t *testing.T) {
	// Test default format
	format := DefaultCompressingTermVectorsFormat()
	if format == nil {
		t.Fatal("DefaultCompressingTermVectorsFormat returned nil")
	}

	if format.Name() != "CompressingTermVectorsFormat" {
		t.Errorf("expected name 'CompressingTermVectorsFormat', got '%s'", format.Name())
	}

	if format.CompressionMode() != CompressionModeLZ4Fast {
		t.Errorf("expected LZ4Fast mode, got %v", format.CompressionMode())
	}

	if format.ChunkSize() != 16*1024 {
		t.Errorf("expected chunk size 16KB, got %d", format.ChunkSize())
	}

	if format.MaxDocsPerChunk() != 128 {
		t.Errorf("expected max docs per chunk 128, got %d", format.MaxDocsPerChunk())
	}
}

// TestCompressingTermVectorsFormat_CustomOptions tests format with custom options
func TestCompressingTermVectorsFormat_CustomOptions(t *testing.T) {
	format := NewCompressingTermVectorsFormat(CompressionModeDeflate, 8192, 64)

	if format.CompressionMode() != CompressionModeDeflate {
		t.Errorf("expected DEFLATE mode, got %v", format.CompressionMode())
	}

	if format.ChunkSize() != 8192 {
		t.Errorf("expected chunk size 8192, got %d", format.ChunkSize())
	}

	if format.MaxDocsPerChunk() != 64 {
		t.Errorf("expected max docs per chunk 64, got %d", format.MaxDocsPerChunk())
	}
}

// TestCompressingTermVectorsFormat_MinimumChunkSize tests minimum chunk size enforcement
func TestCompressingTermVectorsFormat_MinimumChunkSize(t *testing.T) {
	// Pass chunk size below minimum
	format := NewCompressingTermVectorsFormat(CompressionModeLZ4Fast, 512, 10)

	// Should be clamped to 1024
	if format.ChunkSize() != 1024 {
		t.Errorf("expected chunk size clamped to 1024, got %d", format.ChunkSize())
	}
}

// TestCompressingTermVectorsFormat_MinimumDocsPerChunk tests minimum docs per chunk enforcement
func TestCompressingTermVectorsFormat_MinimumDocsPerChunk(t *testing.T) {
	// Pass docs per chunk below minimum
	format := NewCompressingTermVectorsFormat(CompressionModeLZ4Fast, 4096, 0)

	// Should be clamped to 1
	if format.MaxDocsPerChunk() != 1 {
		t.Errorf("expected max docs per chunk clamped to 1, got %d", format.MaxDocsPerChunk())
	}
}

// TestCompressingTermVectorsFormat_ReaderWriter tests reader/writer creation
func TestCompressingTermVectorsFormat_ReaderWriter(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	segmentInfo := index.NewSegmentInfo("_0", 1, dir)
	fieldInfos := index.NewFieldInfos()

	state := &SegmentWriteState{
		Directory:   dir,
		SegmentInfo: segmentInfo,
		FieldInfos:  fieldInfos,
	}

	format := DefaultCompressingTermVectorsFormat()

	// Test VectorsWriter creation
	writer, err := format.VectorsWriter(state)
	if err != nil {
		t.Fatalf("VectorsWriter failed: %v", err)
	}
	if writer == nil {
		t.Fatal("VectorsWriter returned nil")
	}
	writer.Close()

	// Test VectorsReader creation
	reader, err := format.VectorsReader(dir, segmentInfo, fieldInfos, store.IOContext{Context: store.ContextRead})
	if err != nil {
		t.Fatalf("VectorsReader failed: %v", err)
	}
	if reader == nil {
		t.Fatal("VectorsReader returned nil")
	}
	reader.Close()
}

// TestCompressingTermVectorsWriter_DocumentLifecycle tests the document writing lifecycle
func TestCompressingTermVectorsWriter_DocumentLifecycle(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	segmentInfo := index.NewSegmentInfo("_0", 1, dir)
	fieldInfos := index.NewFieldInfos()

	state := &SegmentWriteState{
		Directory:   dir,
		SegmentInfo: segmentInfo,
		FieldInfos:  fieldInfos,
	}

	format := DefaultCompressingTermVectorsFormat()
	writer, err := format.VectorsWriter(state)
	if err != nil {
		t.Fatalf("VectorsWriter failed: %v", err)
	}
	defer writer.Close()

	// Start a document
	if err := writer.StartDocument(1); err != nil {
		t.Fatalf("StartDocument failed: %v", err)
	}

	// Create a field info using NewFieldInfo with default options
	opts := index.FieldInfoOptions{
		IndexOptions:             index.IndexOptionsDocsAndFreqsAndPositions,
		DocValuesType:            index.DocValuesTypeNone,
		StoreTermVectors:         true,
		StoreTermVectorPositions: true,
	}
	fieldInfo := index.NewFieldInfo("test_field", 0, opts)

	// Start a field
	if err := writer.StartField(fieldInfo, 1, true, false, false); err != nil {
		t.Fatalf("StartField failed: %v", err)
	}

	// Start a term
	if err := writer.StartTerm([]byte("test")); err != nil {
		t.Fatalf("StartTerm failed: %v", err)
	}

	// Add a position
	if err := writer.AddPosition(0, -1, -1, nil); err != nil {
		t.Fatalf("AddPosition failed: %v", err)
	}

	// Finish the term
	if err := writer.FinishTerm(); err != nil {
		t.Fatalf("FinishTerm failed: %v", err)
	}

	// Finish the field
	if err := writer.FinishField(); err != nil {
		t.Fatalf("FinishField failed: %v", err)
	}

	// Finish the document
	if err := writer.FinishDocument(); err != nil {
		t.Fatalf("FinishDocument failed: %v", err)
	}
}

// TestCompressingTermVectorsWriter_MultipleDocuments tests writing multiple documents
func TestCompressingTermVectorsWriter_MultipleDocuments(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	segmentInfo := index.NewSegmentInfo("_0", 3, dir)
	fieldInfos := index.NewFieldInfos()

	state := &SegmentWriteState{
		Directory:   dir,
		SegmentInfo: segmentInfo,
		FieldInfos:  fieldInfos,
	}

	format := DefaultCompressingTermVectorsFormat()
	writer, err := format.VectorsWriter(state)
	if err != nil {
		t.Fatalf("VectorsWriter failed: %v", err)
	}
	defer writer.Close()

	// Write multiple documents
	for i := 0; i < 3; i++ {
		if err := writer.StartDocument(1); err != nil {
			t.Fatalf("StartDocument failed: %v", err)
		}

		opts := index.FieldInfoOptions{
			IndexOptions:             index.IndexOptionsDocsAndFreqsAndPositions,
			DocValuesType:            index.DocValuesTypeNone,
			StoreTermVectors:         true,
			StoreTermVectorPositions: true,
		}
		fieldInfo := index.NewFieldInfo("content", 0, opts)

		if err := writer.StartField(fieldInfo, 1, true, false, false); err != nil {
			t.Fatalf("StartField failed: %v", err)
		}

		if err := writer.StartTerm([]byte("term")); err != nil {
			t.Fatalf("StartTerm failed: %v", err)
		}

		if err := writer.AddPosition(i, -1, -1, nil); err != nil {
			t.Fatalf("AddPosition failed: %v", err)
		}

		if err := writer.FinishTerm(); err != nil {
			t.Fatalf("FinishTerm failed: %v", err)
		}

		if err := writer.FinishField(); err != nil {
			t.Fatalf("FinishField failed: %v", err)
		}

		if err := writer.FinishDocument(); err != nil {
			t.Fatalf("FinishDocument failed: %v", err)
		}
	}
}

// TestCompressingTermVectorsWriter_AllCompressionModes tests all compression modes
func TestCompressingTermVectorsWriter_AllCompressionModes(t *testing.T) {
	modes := []CompressionMode{
		CompressionModeLZ4Fast,
		CompressionModeLZ4High,
		CompressionModeDeflate,
	}

	for _, mode := range modes {
		t.Run(mode.String(), func(t *testing.T) {
			dir := store.NewByteBuffersDirectory()
			defer dir.Close()

			segmentInfo := index.NewSegmentInfo("_0", 1, dir)
			fieldInfos := index.NewFieldInfos()

			state := &SegmentWriteState{
				Directory:   dir,
				SegmentInfo: segmentInfo,
				FieldInfos:  fieldInfos,
			}

			format := NewCompressingTermVectorsFormat(mode, 4096, 16)

			writer, err := format.VectorsWriter(state)
			if err != nil {
				t.Fatalf("VectorsWriter failed: %v", err)
			}

			// Write a simple document
			if err := writer.StartDocument(1); err != nil {
				t.Fatalf("StartDocument failed: %v", err)
			}

			opts := index.FieldInfoOptions{
				IndexOptions:     index.IndexOptionsDocsAndFreqs,
				DocValuesType:    index.DocValuesTypeNone,
				StoreTermVectors: true,
			}
			fieldInfo := index.NewFieldInfo("field", 0, opts)

			if err := writer.StartField(fieldInfo, 1, false, false, false); err != nil {
				t.Fatalf("StartField failed: %v", err)
			}

			if err := writer.StartTerm([]byte("term")); err != nil {
				t.Fatalf("StartTerm failed: %v", err)
			}

			if err := writer.FinishTerm(); err != nil {
				t.Fatalf("FinishTerm failed: %v", err)
			}

			if err := writer.FinishField(); err != nil {
				t.Fatalf("FinishField failed: %v", err)
			}

			if err := writer.FinishDocument(); err != nil {
				t.Fatalf("FinishDocument failed: %v", err)
			}

			if err := writer.Close(); err != nil {
				t.Fatalf("Writer Close failed: %v", err)
			}
		})
	}
}

// TestCompressingTermVectorsReader_Get tests the Get method
func TestCompressingTermVectorsReader_Get(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	segmentInfo := index.NewSegmentInfo("_0", 1, dir)
	fieldInfos := index.NewFieldInfos()

	format := DefaultCompressingTermVectorsFormat()
	reader, err := format.VectorsReader(dir, segmentInfo, fieldInfos, store.IOContext{Context: store.ContextRead})
	if err != nil {
		t.Fatalf("VectorsReader failed: %v", err)
	}
	defer reader.Close()

	// Get term vectors for document 0
	fields, err := reader.Get(0)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if fields == nil {
		t.Error("Get returned nil fields")
	}
}

// TestCompressingTermVectorsReader_GetField tests the GetField method
func TestCompressingTermVectorsReader_GetField(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	segmentInfo := index.NewSegmentInfo("_0", 1, dir)
	fieldInfos := index.NewFieldInfos()

	format := DefaultCompressingTermVectorsFormat()

	// Write term vectors first
	state := &SegmentWriteState{
		Directory:   dir,
		SegmentInfo: segmentInfo,
		FieldInfos:  fieldInfos,
	}

	writer, err := format.VectorsWriter(state)
	if err != nil {
		t.Fatalf("VectorsWriter failed: %v", err)
	}

	// Write a document with term vectors
	if err := writer.StartDocument(1); err != nil {
		t.Fatalf("StartDocument failed: %v", err)
	}

	opts := index.FieldInfoOptions{
		IndexOptions:             index.IndexOptionsDocsAndFreqsAndPositions,
		DocValuesType:            index.DocValuesTypeNone,
		StoreTermVectors:         true,
		StoreTermVectorPositions: true,
	}
	fieldInfo := index.NewFieldInfo("test_field", 0, opts)

	if err := writer.StartField(fieldInfo, 1, true, false, false); err != nil {
		t.Fatalf("StartField failed: %v", err)
	}

	if err := writer.StartTerm([]byte("term1")); err != nil {
		t.Fatalf("StartTerm failed: %v", err)
	}

	if err := writer.AddPosition(0, -1, -1, nil); err != nil {
		t.Fatalf("AddPosition failed: %v", err)
	}

	if err := writer.FinishTerm(); err != nil {
		t.Fatalf("FinishTerm failed: %v", err)
	}

	if err := writer.FinishField(); err != nil {
		t.Fatalf("FinishField failed: %v", err)
	}

	if err := writer.FinishDocument(); err != nil {
		t.Fatalf("FinishDocument failed: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Now read the term vectors
	reader, err := format.VectorsReader(dir, segmentInfo, fieldInfos, store.IOContext{Context: store.ContextRead})
	if err != nil {
		t.Fatalf("VectorsReader failed: %v", err)
	}
	defer reader.Close()

	// Get term vectors for a specific field
	terms, err := reader.GetField(0, "test_field")
	if err != nil {
		t.Fatalf("GetField failed: %v", err)
	}
	if terms == nil {
		t.Error("GetField returned nil terms")
	}
}
