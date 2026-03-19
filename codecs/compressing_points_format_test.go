// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestCompressingPointsFormat_Basic tests basic format creation
func TestCompressingPointsFormat_Basic(t *testing.T) {
	// Test default format
	format := DefaultCompressingPointsFormat()
	if format == nil {
		t.Fatal("DefaultCompressingPointsFormat returned nil")
	}

	if format.Name() != "CompressingPointsFormat" {
		t.Errorf("expected name 'CompressingPointsFormat', got '%s'", format.Name())
	}

	if format.CompressionMode() != CompressionModeLZ4Fast {
		t.Errorf("expected LZ4Fast mode, got %v", format.CompressionMode())
	}

	if format.ChunkSize() != 16*1024 {
		t.Errorf("expected chunk size 16KB, got %d", format.ChunkSize())
	}
}

// TestCompressingPointsFormat_CustomOptions tests format with custom options
func TestCompressingPointsFormat_CustomOptions(t *testing.T) {
	format := NewCompressingPointsFormat(CompressionModeDeflate, 8192)

	if format.CompressionMode() != CompressionModeDeflate {
		t.Errorf("expected DEFLATE mode, got %v", format.CompressionMode())
	}

	if format.ChunkSize() != 8192 {
		t.Errorf("expected chunk size 8192, got %d", format.ChunkSize())
	}
}

// TestCompressingPointsFormat_MinimumChunkSize tests minimum chunk size enforcement
func TestCompressingPointsFormat_MinimumChunkSize(t *testing.T) {
	// Pass chunk size below minimum
	format := NewCompressingPointsFormat(CompressionModeLZ4Fast, 512)

	// Should be clamped to 1024
	if format.ChunkSize() != 1024 {
		t.Errorf("expected chunk size clamped to 1024, got %d", format.ChunkSize())
	}
}

// TestCompressingPointsFormat_AllCompressionModes tests all compression modes
func TestCompressingPointsFormat_AllCompressionModes(t *testing.T) {
	modes := []CompressionMode{
		CompressionModeLZ4Fast,
		CompressionModeLZ4High,
		CompressionModeDeflate,
	}

	for _, mode := range modes {
		t.Run(mode.String(), func(t *testing.T) {
			format := NewCompressingPointsFormat(mode, 4096)

			if format.CompressionMode() != mode {
				t.Errorf("expected %v mode, got %v", mode, format.CompressionMode())
			}
		})
	}
}

// TestCompressingPointsWriter_Basic tests basic writer creation
func TestCompressingPointsWriter_Basic(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	segInfo := index.NewSegmentInfo("test_segment", 100, dir)

	state := &SegmentWriteState{
		SegmentInfo: segInfo,
		FieldInfos:  &index.FieldInfos{},
		Directory:   dir,
	}

	writer, err := NewCompressingPointsWriter(state, CompressionModeLZ4Fast, 16*1024)
	if err != nil {
		t.Fatalf("NewCompressingPointsWriter failed: %v", err)
	}
	if writer == nil {
		t.Fatal("NewCompressingPointsWriter returned nil")
	}

	// Close should succeed
	err = writer.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Closing again should be safe
	err = writer.Close()
	if err != nil {
		t.Errorf("Second Close failed: %v", err)
	}
}

// TestCompressingPointsReader_Basic tests basic reader creation
func TestCompressingPointsReader_Basic(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	segInfo := index.NewSegmentInfo("test_segment", 100, dir)

	state := &SegmentReadState{
		SegmentInfo: segInfo,
		FieldInfos:  &index.FieldInfos{},
		Directory:   dir,
	}

	reader, err := NewCompressingPointsReader(state, CompressionModeLZ4Fast, 16*1024)
	if err != nil {
		t.Fatalf("NewCompressingPointsReader failed: %v", err)
	}
	if reader == nil {
		t.Fatal("NewCompressingPointsReader returned nil")
	}

	// Close should succeed
	err = reader.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Closing again should be safe
	err = reader.Close()
	if err != nil {
		t.Errorf("Second Close failed: %v", err)
	}
}

// TestCompressingPointsReader_CheckIntegrity tests integrity checking
func TestCompressingPointsReader_CheckIntegrity(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	segInfo := index.NewSegmentInfo("test_segment", 100, dir)

	state := &SegmentReadState{
		SegmentInfo: segInfo,
		FieldInfos:  &index.FieldInfos{},
		Directory:   dir,
	}

	reader, err := NewCompressingPointsReader(state, CompressionModeLZ4Fast, 16*1024)
	if err != nil {
		t.Fatalf("NewCompressingPointsReader failed: %v", err)
	}
	defer reader.Close()

	// CheckIntegrity should succeed
	err = reader.CheckIntegrity()
	if err != nil {
		t.Errorf("CheckIntegrity failed: %v", err)
	}
}

// TestCompressingPointsWriter_WriteField tests writing a field
func TestCompressingPointsWriter_WriteField(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	segInfo := index.NewSegmentInfo("test_segment", 100, dir)

	state := &SegmentWriteState{
		SegmentInfo: segInfo,
		FieldInfos:  &index.FieldInfos{},
		Directory:   dir,
	}

	writer, err := NewCompressingPointsWriter(state, CompressionModeLZ4Fast, 16*1024)
	if err != nil {
		t.Fatalf("NewCompressingPointsWriter failed: %v", err)
	}
	defer writer.Close()

	fieldInfo := index.NewFieldInfo("test_field", 1, index.FieldInfoOptions{
		IndexOptions:  index.IndexOptionsDocs,
		DocValuesType: index.DocValuesTypeNumeric,
		Stored:        true,
	})

	// Create a placeholder reader
	reader := &testPointsReader{}

	err = writer.WriteField(fieldInfo, reader)
	if err != nil {
		t.Errorf("WriteField failed: %v", err)
	}
}

// TestCompressingPointsWriter_WriteFieldAfterClose tests writing after close
func TestCompressingPointsWriter_WriteFieldAfterClose(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	segInfo := index.NewSegmentInfo("test_segment", 100, dir)

	state := &SegmentWriteState{
		SegmentInfo: segInfo,
		FieldInfos:  &index.FieldInfos{},
		Directory:   dir,
	}

	writer, err := NewCompressingPointsWriter(state, CompressionModeLZ4Fast, 16*1024)
	if err != nil {
		t.Fatalf("NewCompressingPointsWriter failed: %v", err)
	}

	// Close the writer
	err = writer.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	fieldInfo := index.NewFieldInfo("test_field", 1, index.FieldInfoOptions{
		IndexOptions:  index.IndexOptionsDocs,
		DocValuesType: index.DocValuesTypeNumeric,
		Stored:        true,
	})

	reader := &testPointsReader{}

	// Writing after close should fail
	err = writer.WriteField(fieldInfo, reader)
	if err == nil {
		t.Error("expected WriteField to fail after Close, but it succeeded")
	}
}

// TestCompressingPointsWriter_Finish tests the Finish method
func TestCompressingPointsWriter_Finish(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	segInfo := index.NewSegmentInfo("test_segment", 100, dir)

	state := &SegmentWriteState{
		SegmentInfo: segInfo,
		FieldInfos:  &index.FieldInfos{},
		Directory:   dir,
	}

	writer, err := NewCompressingPointsWriter(state, CompressionModeLZ4Fast, 16*1024)
	if err != nil {
		t.Fatalf("NewCompressingPointsWriter failed: %v", err)
	}
	defer writer.Close()

	err = writer.Finish()
	if err != nil {
		t.Errorf("Finish failed: %v", err)
	}

	// Finish again should be safe
	err = writer.Finish()
	if err != nil {
		t.Errorf("Second Finish failed: %v", err)
	}
}

// TestEmptyPointValues tests the empty point values implementation
func TestEmptyPointValues(t *testing.T) {
	empty := &emptyPointValues{}

	err := empty.Intersect(nil)
	if err != nil {
		t.Errorf("Intersect returned error: %v", err)
	}

	count := empty.EstimatePointCount(nil)
	if count != 0 {
		t.Errorf("expected EstimatePointCount to return 0, got %d", count)
	}

	if empty.GetMinPackedValue() != nil {
		t.Error("expected GetMinPackedValue to return nil")
	}

	if empty.GetMaxPackedValue() != nil {
		t.Error("expected GetMaxPackedValue to return nil")
	}

	if empty.GetNumDimensions() != 0 {
		t.Errorf("expected GetNumDimensions to return 0, got %d", empty.GetNumDimensions())
	}

	if empty.GetBytesPerDimension() != 0 {
		t.Errorf("expected GetBytesPerDimension to return 0, got %d", empty.GetBytesPerDimension())
	}

	if empty.GetDocCount() != 0 {
		t.Errorf("expected GetDocCount to return 0, got %d", empty.GetDocCount())
	}
}

// testPointsReader is a test implementation of PointsReader
type testPointsReader struct{}

func (t *testPointsReader) CheckIntegrity() error { return nil }
func (t *testPointsReader) Close() error          { return nil }
