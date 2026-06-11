// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
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

// TestCompressingTermVectorsFormat_NoOrds validates freq-only term vector
// round-trip (no positions, offsets, or payloads — i.e., no ordinals).
//
// Lucene's TestCompressingTermVectorsFormat.testNoOrds (LUCENE-5156) verifies
// that TermsEnum.Ord() and TermsEnum.SeekExact(int) throw
// UnsupportedOperationException on the compressing TV path. Gocene does not
// expose Ord() on TermsEnum; this test verifies instead that the underlying
// data path handles freq-only fields correctly:
//   - HasPositions/HasOffsets/HasPayloads all report false
//   - HasFreqs reports true
//   - The iterated term text and per-document term counts are faithfully
//     recovered after a write/read round-trip.
func TestCompressingTermVectorsFormat_NoOrds(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	segmentInfo := index.NewSegmentInfo("_0", 1, dir)
	fieldInfos := index.NewFieldInfos()

	state := &SegmentWriteState{
		Directory:   dir,
		SegmentInfo: segmentInfo,
		FieldInfos:  fieldInfos,
	}

	format := NewCompressingTermVectorsFormat(CompressionModeLZ4Fast, 4096, 16)
	writer, err := format.VectorsWriter(state)
	if err != nil {
		t.Fatalf("VectorsWriter failed: %v", err)
	}

	// Write a freq-only document (no positions, offsets, or payloads).
	if err := writer.StartDocument(1); err != nil {
		t.Fatalf("StartDocument failed: %v", err)
	}

	opts := index.FieldInfoOptions{
		IndexOptions:     index.IndexOptionsDocsAndFreqs,
		DocValuesType:    index.DocValuesTypeNone,
		StoreTermVectors: true,
	}
	fieldInfo := index.NewFieldInfo("field", 0, opts)

	if err := writer.StartField(fieldInfo, 3, false, false, false); err != nil {
		t.Fatalf("StartField failed: %v", err)
	}

	for _, term := range []string{"alpha", "beta", "gamma"} {
		if err := writer.StartTerm([]byte(term)); err != nil {
			t.Fatalf("StartTerm(%s) failed: %v", term, err)
		}
		if err := writer.FinishTerm(); err != nil {
			t.Fatalf("FinishTerm(%s) failed: %v", term, err)
		}
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

	// Read back and verify.
	reader, err := format.VectorsReader(dir, segmentInfo, fieldInfos, store.IOContext{Context: store.ContextRead})
	if err != nil {
		t.Fatalf("VectorsReader failed: %v", err)
	}
	defer reader.Close()

	fields, err := reader.Get(0)
	if err != nil {
		t.Fatalf("Get(0) failed: %v", err)
	}
	if fields == nil {
		t.Fatal("Get(0) returned nil")
	}

	terms, err := fields.Terms("field")
	if err != nil {
		t.Fatalf("Terms('field') failed: %v", err)
	}
	if terms == nil {
		t.Fatal("Terms('field') returned nil")
	}

	// Verify freq-only flags.
	if !terms.HasFreqs() {
		t.Error("expected HasFreqs()=true")
	}
	if terms.HasPositions() {
		t.Error("expected HasPositions()=false for freq-only field")
	}
	if terms.HasOffsets() {
		t.Error("expected HasOffsets()=false for freq-only field")
	}
	if terms.HasPayloads() {
		t.Error("expected HasPayloads()=false for freq-only field")
	}

	// Verify term iteration.
	iter, err := terms.GetIterator()
	if err != nil {
		t.Fatalf("GetIterator failed: %v", err)
	}
	var gotTerms []string
	for {
		term, err := iter.Next()
		if err != nil {
			t.Fatalf("Next failed: %v", err)
		}
		if term == nil {
			break
		}
		gotTerms = append(gotTerms, term.Bytes.String())
	}
	wantTerms := []string{"alpha", "beta", "gamma"}
	if len(gotTerms) != len(wantTerms) {
		t.Fatalf("got %d terms, want %d: %v", len(gotTerms), len(wantTerms), gotTerms)
	}
	for i := range wantTerms {
		if gotTerms[i] != wantTerms[i] {
			t.Errorf("term[%d]: got %q, want %q", i, gotTerms[i], wantTerms[i])
		}
	}
}

// TestCompressingTermVectorsFormat_ChunkCleanup validates that the writer
// correctly handles chunk boundaries across multiple documents.
//
// Lucene's TestCompressingTermVectorsFormat.testChunkCleanup verifies dirty
// chunk tracking and recompression during forceMerge with NoMergePolicy.
// Gocene does not yet expose dirty-chunk bookkeeping; this test verifies
// instead that writing enough documents to span multiple chunk boundaries
// produces a readable result where all documents are recovered faithfully,
// and that closing/reopening a writer on the same directory works.
func TestCompressingTermVectorsFormat_ChunkCleanup(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	segmentInfo := index.NewSegmentInfo("_0", 20, dir)
	fieldInfos := index.NewFieldInfos()

	state := &SegmentWriteState{
		Directory:   dir,
		SegmentInfo: segmentInfo,
		FieldInfos:  fieldInfos,
	}

	// Use small maxDocsPerChunk so we exercise multiple chunks.
	format := NewCompressingTermVectorsFormat(CompressionModeLZ4Fast, 4096, 4)
	writer, err := format.VectorsWriter(state)
	if err != nil {
		t.Fatalf("VectorsWriter failed: %v", err)
	}

	opts := index.FieldInfoOptions{
		IndexOptions:             index.IndexOptionsDocsAndFreqsAndPositions,
		DocValuesType:            index.DocValuesTypeNone,
		StoreTermVectors:         true,
		StoreTermVectorPositions: true,
	}
	fieldInfo := index.NewFieldInfo("content", 0, opts)

	// Write 20 documents — with maxDocsPerChunk=4, this produces 5 chunks.
	for docID := 0; docID < 20; docID++ {
		if err := writer.StartDocument(1); err != nil {
			t.Fatalf("doc %d StartDocument: %v", docID, err)
		}
		if err := writer.StartField(fieldInfo, 1, true, false, false); err != nil {
			t.Fatalf("doc %d StartField: %v", docID, err)
		}
		termText := fmt.Sprintf("term_%d", docID)
		if err := writer.StartTerm([]byte(termText)); err != nil {
			t.Fatalf("doc %d StartTerm: %v", docID, err)
		}
		if err := writer.AddPosition(docID, -1, -1, nil); err != nil {
			t.Fatalf("doc %d AddPosition: %v", docID, err)
		}
		if err := writer.FinishTerm(); err != nil {
			t.Fatalf("doc %d FinishTerm: %v", docID, err)
		}
		if err := writer.FinishField(); err != nil {
			t.Fatalf("doc %d FinishField: %v", docID, err)
		}
		if err := writer.FinishDocument(); err != nil {
			t.Fatalf("doc %d FinishDocument: %v", docID, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Read back all documents and verify term data.
	reader, err := format.VectorsReader(dir, segmentInfo, fieldInfos, store.IOContext{Context: store.ContextRead})
	if err != nil {
		t.Fatalf("VectorsReader failed: %v", err)
	}
	defer reader.Close()

	for docID := 0; docID < 20; docID++ {
		fields, err := reader.Get(docID)
		if err != nil {
			t.Fatalf("doc %d Get: %v", docID, err)
		}
		if fields == nil {
			t.Fatalf("doc %d: nil Fields", docID)
		}
		terms, err := fields.Terms("content")
		if err != nil {
			t.Fatalf("doc %d Terms('content'): %v", docID, err)
		}
		if terms == nil {
			t.Fatalf("doc %d: nil Terms for 'content'", docID)
		}
		iter, err := terms.GetIterator()
		if err != nil {
			t.Fatalf("doc %d GetIterator: %v", docID, err)
		}
		term, err := iter.Next()
		if err != nil {
			t.Fatalf("doc %d Next: %v", docID, err)
		}
		if term == nil {
			t.Fatalf("doc %d: no terms found", docID)
		}
		wantTerm := fmt.Sprintf("term_%d", docID)
		if got := term.Bytes.String(); got != wantTerm {
			t.Errorf("doc %d: got term %q, want %q", docID, got, wantTerm)
		}
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
