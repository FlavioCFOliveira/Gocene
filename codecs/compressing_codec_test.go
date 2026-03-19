// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"testing"
)

// TestCompressingCodec_Basic tests basic codec creation
func TestCompressingCodec_Basic(t *testing.T) {
	// Test default codec
	codec := DefaultCompressingCodec()
	if codec == nil {
		t.Fatal("DefaultCompressingCodec returned nil")
	}

	if codec.Name() != "CompressingCodec" {
		t.Errorf("expected name 'CompressingCodec', got '%s'", codec.Name())
	}

	if codec.CompressionMode() != CompressionModeLZ4Fast {
		t.Errorf("expected LZ4Fast mode, got %v", codec.CompressionMode())
	}

	if codec.ChunkSize() != 16*1024 {
		t.Errorf("expected chunk size 16KB, got %d", codec.ChunkSize())
	}

	if codec.MaxDocsPerChunk() != 128 {
		t.Errorf("expected max docs per chunk 128, got %d", codec.MaxDocsPerChunk())
	}
}

// TestCompressingCodec_CustomOptions tests codec with custom options
func TestCompressingCodec_CustomOptions(t *testing.T) {
	codec := NewCompressingCodec(CompressionModeDeflate, 8192, 64)

	if codec.CompressionMode() != CompressionModeDeflate {
		t.Errorf("expected DEFLATE mode, got %v", codec.CompressionMode())
	}

	if codec.ChunkSize() != 8192 {
		t.Errorf("expected chunk size 8192, got %d", codec.ChunkSize())
	}

	if codec.MaxDocsPerChunk() != 64 {
		t.Errorf("expected max docs per chunk 64, got %d", codec.MaxDocsPerChunk())
	}
}

// TestCompressingCodec_FastCodec tests fast compression codec
func TestCompressingCodec_FastCodec(t *testing.T) {
	codec := FastCompressingCodec()

	if codec == nil {
		t.Fatal("FastCompressingCodec returned nil")
	}

	if codec.CompressionMode() != CompressionModeLZ4Fast {
		t.Errorf("expected LZ4Fast mode, got %v", codec.CompressionMode())
	}

	if codec.ChunkSize() != 8*1024 {
		t.Errorf("expected chunk size 8KB, got %d", codec.ChunkSize())
	}

	if codec.MaxDocsPerChunk() != 64 {
		t.Errorf("expected max docs per chunk 64, got %d", codec.MaxDocsPerChunk())
	}
}

// TestCompressingCodec_HighCompressionCodec tests high compression codec
func TestCompressingCodec_HighCompressionCodec(t *testing.T) {
	codec := HighCompressionCompressingCodec()

	if codec == nil {
		t.Fatal("HighCompressionCompressingCodec returned nil")
	}

	if codec.CompressionMode() != CompressionModeDeflate {
		t.Errorf("expected DEFLATE mode, got %v", codec.CompressionMode())
	}

	if codec.ChunkSize() != 64*1024 {
		t.Errorf("expected chunk size 64KB, got %d", codec.ChunkSize())
	}

	if codec.MaxDocsPerChunk() != 256 {
		t.Errorf("expected max docs per chunk 256, got %d", codec.MaxDocsPerChunk())
	}
}

// TestCompressingCodec_StoredFieldsFormat tests stored fields format
func TestCompressingCodec_StoredFieldsFormat(t *testing.T) {
	codec := DefaultCompressingCodec()

	format := codec.StoredFieldsFormat()
	if format == nil {
		t.Fatal("StoredFieldsFormat returned nil")
	}

	// Check that it's a CompressingStoredFieldsFormat
	if _, ok := format.(*CompressingStoredFieldsFormat); !ok {
		t.Errorf("expected *CompressingStoredFieldsFormat, got %T", format)
	}
}

// TestCompressingCodec_TermVectorsFormat tests term vectors format
func TestCompressingCodec_TermVectorsFormat(t *testing.T) {
	codec := DefaultCompressingCodec()

	format := codec.TermVectorsFormat()
	if format == nil {
		t.Fatal("TermVectorsFormat returned nil")
	}

	// Check that it's a CompressingTermVectorsFormat
	if _, ok := format.(*CompressingTermVectorsFormat); !ok {
		t.Errorf("expected *CompressingTermVectorsFormat, got %T", format)
	}
}

// TestCompressingCodec_FieldInfosFormat tests field infos format
func TestCompressingCodec_FieldInfosFormat(t *testing.T) {
	codec := DefaultCompressingCodec()

	format := codec.FieldInfosFormat()
	if format == nil {
		t.Fatal("FieldInfosFormat returned nil")
	}
}

// TestCompressingCodec_SegmentInfosFormat tests segment infos format
func TestCompressingCodec_SegmentInfosFormat(t *testing.T) {
	codec := DefaultCompressingCodec()

	format := codec.SegmentInfosFormat()
	if format == nil {
		t.Fatal("SegmentInfosFormat returned nil")
	}
}

// TestCompressingCodec_PostingsFormat tests postings format
func TestCompressingCodec_PostingsFormat(t *testing.T) {
	codec := DefaultCompressingCodec()

	format := codec.PostingsFormat()
	if format == nil {
		t.Fatal("PostingsFormat returned nil")
	}
}

// TestCompressingCodec_DocValuesFormat tests doc values format
func TestCompressingCodec_DocValuesFormat(t *testing.T) {
	codec := DefaultCompressingCodec()

	format := codec.DocValuesFormat()
	if format == nil {
		t.Fatal("DocValuesFormat returned nil")
	}
}

// TestCompressingCodec_NormsFormat tests norms format
func TestCompressingCodec_NormsFormat(t *testing.T) {
	codec := DefaultCompressingCodec()

	format := codec.NormsFormat()
	if format == nil {
		t.Fatal("NormsFormat returned nil")
	}
}

// TestCompressingCodec_LiveDocsFormat tests live docs format
func TestCompressingCodec_LiveDocsFormat(t *testing.T) {
	codec := DefaultCompressingCodec()

	format := codec.LiveDocsFormat()
	if format == nil {
		t.Fatal("LiveDocsFormat returned nil")
	}
}

// TestCompressingCodec_PointsFormat tests points format
func TestCompressingCodec_PointsFormat(t *testing.T) {
	codec := DefaultCompressingCodec()

	format := codec.PointsFormat()
	if format == nil {
		t.Fatal("PointsFormat returned nil")
	}
}

// TestCompressingCodecFactory tests the codec factory
func TestCompressingCodecFactory(t *testing.T) {
	factory := NewCompressingCodecFactory()

	// Create a new codec
	codec := factory.GetOrCreate("test", CompressionModeLZ4Fast, 4096, 32)
	if codec == nil {
		t.Fatal("GetOrCreate returned nil")
	}

	// Get the same codec again
	codec2, err := factory.Get("test")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if codec2 != codec {
		t.Error("Get returned different codec instance")
	}

	// Check available codecs
	codecs := factory.AvailableCodecs()
	found := false
	for _, name := range codecs {
		if name == "test" {
			found = true
			break
		}
	}
	if !found {
		t.Error("'test' not found in available codecs")
	}
}

// TestCompressingCodecFactory_NotFound tests getting a non-existent codec
func TestCompressingCodecFactory_NotFound(t *testing.T) {
	factory := NewCompressingCodecFactory()

	_, err := factory.Get("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent codec")
	}
}

// TestGlobalCompressingCodecRegistry tests the global registry
func TestGlobalCompressingCodecRegistry(t *testing.T) {
	// Get the default codecs
	codec, err := GetCompressingCodec("Compressing")
	if err != nil {
		t.Fatalf("GetCompressingCodec failed: %v", err)
	}
	if codec == nil {
		t.Fatal("GetCompressingCodec returned nil")
	}

	// Check fast codec
	fastCodec, err := GetCompressingCodec("CompressingFast")
	if err != nil {
		t.Fatalf("GetCompressingCodec(CompressingFast) failed: %v", err)
	}
	if fastCodec == nil {
		t.Fatal("GetCompressingCodec(CompressingFast) returned nil")
	}

	// Check high compression codec
	highCodec, err := GetCompressingCodec("CompressingHighCompression")
	if err != nil {
		t.Fatalf("GetCompressingCodec(CompressingHighCompression) failed: %v", err)
	}
	if highCodec == nil {
		t.Fatal("GetCompressingCodec(CompressingHighCompression) returned nil")
	}
}

// TestCompressingCodec_AllCompressionModes tests all compression modes
func TestCompressingCodec_AllCompressionModes(t *testing.T) {
	modes := []CompressionMode{
		CompressionModeLZ4Fast,
		CompressionModeLZ4High,
		CompressionModeDeflate,
	}

	for _, mode := range modes {
		t.Run(mode.String(), func(t *testing.T) {
			codec := NewCompressingCodec(mode, 4096, 16)

			if codec.CompressionMode() != mode {
				t.Errorf("expected %v mode, got %v", mode, codec.CompressionMode())
			}

			// Verify all formats are available
			if codec.StoredFieldsFormat() == nil {
				t.Error("StoredFieldsFormat is nil")
			}
			if codec.TermVectorsFormat() == nil {
				t.Error("TermVectorsFormat is nil")
			}
			if codec.FieldInfosFormat() == nil {
				t.Error("FieldInfosFormat is nil")
			}
			if codec.SegmentInfosFormat() == nil {
				t.Error("SegmentInfosFormat is nil")
			}
			if codec.PostingsFormat() == nil {
				t.Error("PostingsFormat is nil")
			}
		})
	}
}
