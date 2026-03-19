// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"testing"
)

// TestCompressingDocValuesFormat_Basic tests basic format creation
func TestCompressingDocValuesFormat_Basic(t *testing.T) {
	// Test default format
	format := DefaultCompressingDocValuesFormat()
	if format == nil {
		t.Fatal("DefaultCompressingDocValuesFormat returned nil")
	}

	if format.Name() != "CompressingDocValuesFormat" {
		t.Errorf("expected name 'CompressingDocValuesFormat', got '%s'", format.Name())
	}

	if format.CompressionMode() != CompressionModeLZ4Fast {
		t.Errorf("expected LZ4Fast mode, got %v", format.CompressionMode())
	}

	if format.ChunkSize() != 16*1024 {
		t.Errorf("expected chunk size 16KB, got %d", format.ChunkSize())
	}
}

// TestCompressingDocValuesFormat_CustomOptions tests format with custom options
func TestCompressingDocValuesFormat_CustomOptions(t *testing.T) {
	format := NewCompressingDocValuesFormat(CompressionModeDeflate, 8192)

	if format.CompressionMode() != CompressionModeDeflate {
		t.Errorf("expected DEFLATE mode, got %v", format.CompressionMode())
	}

	if format.ChunkSize() != 8192 {
		t.Errorf("expected chunk size 8192, got %d", format.ChunkSize())
	}
}

// TestCompressingDocValuesFormat_MinimumChunkSize tests minimum chunk size enforcement
func TestCompressingDocValuesFormat_MinimumChunkSize(t *testing.T) {
	// Pass chunk size below minimum
	format := NewCompressingDocValuesFormat(CompressionModeLZ4Fast, 512)

	// Should be clamped to 1024
	if format.ChunkSize() != 1024 {
		t.Errorf("expected chunk size clamped to 1024, got %d", format.ChunkSize())
	}
}

// TestCompressingDocValuesFormat_AllCompressionModes tests all compression modes
func TestCompressingDocValuesFormat_AllCompressionModes(t *testing.T) {
	modes := []CompressionMode{
		CompressionModeLZ4Fast,
		CompressionModeLZ4High,
		CompressionModeDeflate,
	}

	for _, mode := range modes {
		t.Run(mode.String(), func(t *testing.T) {
			format := NewCompressingDocValuesFormat(mode, 4096)

			if format.CompressionMode() != mode {
				t.Errorf("expected %v mode, got %v", mode, format.CompressionMode())
			}
		})
	}
}
