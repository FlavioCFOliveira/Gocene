// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestCompressingStoredFieldsFormat_Basic tests basic format creation
func TestCompressingStoredFieldsFormat_Basic(t *testing.T) {
	// Test default format
	format := DefaultCompressingStoredFieldsFormat()
	if format == nil {
		t.Fatal("DefaultCompressingStoredFieldsFormat returned nil")
	}

	if format.Name() != "CompressingStoredFieldsFormat" {
		t.Errorf("expected name 'CompressingStoredFieldsFormat', got '%s'", format.Name())
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

// TestCompressingStoredFieldsFormat_CustomOptions tests format with custom options
func TestCompressingStoredFieldsFormat_CustomOptions(t *testing.T) {
	format := NewCompressingStoredFieldsFormat(CompressionModeDeflate, 8192, 64)

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

// TestCompressingStoredFieldsFormat_MinimumChunkSize tests minimum chunk size enforcement
func TestCompressingStoredFieldsFormat_MinimumChunkSize(t *testing.T) {
	// Pass chunk size below minimum
	format := NewCompressingStoredFieldsFormat(CompressionModeLZ4Fast, 512, 10)

	// Should be clamped to 1024
	if format.ChunkSize() != 1024 {
		t.Errorf("expected chunk size clamped to 1024, got %d", format.ChunkSize())
	}
}

// TestCompressingStoredFieldsFormat_MinimumDocsPerChunk tests minimum docs per chunk enforcement
func TestCompressingStoredFieldsFormat_MinimumDocsPerChunk(t *testing.T) {
	// Pass docs per chunk below minimum
	format := NewCompressingStoredFieldsFormat(CompressionModeLZ4Fast, 4096, 0)

	// Should be clamped to 1
	if format.MaxDocsPerChunk() != 1 {
		t.Errorf("expected max docs per chunk clamped to 1, got %d", format.MaxDocsPerChunk())
	}
}

// TestCompressingStoredFieldsFormat_FieldsReaderWriter tests reader/writer creation
func TestCompressingStoredFieldsFormat_FieldsReaderWriter(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	segmentInfo := index.NewSegmentInfo("_0", 1, dir)
	fieldInfos := index.NewFieldInfos()

	format := DefaultCompressingStoredFieldsFormat()

	// Test FieldsWriter creation
	writer, err := format.FieldsWriter(dir, segmentInfo, store.IOContext{Context: store.ContextWrite})
	if err != nil {
		t.Fatalf("FieldsWriter failed: %v", err)
	}
	if writer == nil {
		t.Fatal("FieldsWriter returned nil")
	}
	writer.Close()

	// Test FieldsReader creation
	reader, err := format.FieldsReader(dir, segmentInfo, fieldInfos, store.IOContext{Context: store.ContextRead})
	if err != nil {
		t.Fatalf("FieldsReader failed: %v", err)
	}
	if reader == nil {
		t.Fatal("FieldsReader returned nil")
	}
	reader.Close()
}

// TestCompressingStoredFieldsWriter_WriteDocument tests writing a document
func TestCompressingStoredFieldsWriter_WriteDocument(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	segmentInfo := index.NewSegmentInfo("_0", 1, dir)

	format := DefaultCompressingStoredFieldsFormat()
	writer, err := format.FieldsWriter(dir, segmentInfo, store.IOContext{Context: store.ContextWrite})
	if err != nil {
		t.Fatalf("FieldsWriter failed: %v", err)
	}
	defer writer.Close()

	// Write a simple document
	if err := writer.StartDocument(); err != nil {
		t.Fatalf("StartDocument failed: %v", err)
	}

	field, err := document.NewStringField("test_field", "test_value", true)
	if err != nil {
		t.Fatalf("NewStringField failed: %v", err)
	}
	if err := writer.WriteField(field); err != nil {
		t.Fatalf("WriteField failed: %v", err)
	}

	if err := writer.FinishDocument(); err != nil {
		t.Fatalf("FinishDocument failed: %v", err)
	}
}

// TestCompressingStoredFieldsWriter_WriteMultipleDocuments tests writing multiple documents
func TestCompressingStoredFieldsWriter_WriteMultipleDocuments(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	segmentInfo := index.NewSegmentInfo("_0", 5, dir)

	format := DefaultCompressingStoredFieldsFormat()
	writer, err := format.FieldsWriter(dir, segmentInfo, store.IOContext{Context: store.ContextWrite})
	if err != nil {
		t.Fatalf("FieldsWriter failed: %v", err)
	}
	defer writer.Close()

	// Write multiple documents
	for i := 0; i < 5; i++ {
		if err := writer.StartDocument(); err != nil {
			t.Fatalf("StartDocument failed: %v", err)
		}

		field, err := document.NewStringField("id", string(rune('0'+i)), true)
		if err != nil {
			t.Fatalf("NewStringField failed: %v", err)
		}
		if err := writer.WriteField(field); err != nil {
			t.Fatalf("WriteField failed: %v", err)
		}

		if err := writer.FinishDocument(); err != nil {
			t.Fatalf("FinishDocument failed: %v", err)
		}
	}
}

// TestCompressionMode_String tests string representation of compression modes
func TestCompressionMode_String(t *testing.T) {
	tests := []struct {
		mode     CompressionMode
		expected string
	}{
		{CompressionModeLZ4Fast, "LZ4_FAST"},
		{CompressionModeLZ4High, "LZ4_HIGH"},
		{CompressionModeDeflate, "DEFLATE"},
		{CompressionMode(999), "UNKNOWN"}, // Invalid mode
	}

	for _, test := range tests {
		result := test.mode.String()
		if result != test.expected {
			t.Errorf("CompressionMode(%d).String() = %s, expected %s", test.mode, result, test.expected)
		}
	}
}

// MockStoredFieldVisitor is a mock visitor for testing
type MockStoredFieldVisitor struct {
	fields map[string]interface{}
}

func NewMockStoredFieldVisitor() *MockStoredFieldVisitor {
	return &MockStoredFieldVisitor{
		fields: make(map[string]interface{}),
	}
}

func (v *MockStoredFieldVisitor) StringField(field string, value string) {
	v.fields[field] = value
}

func (v *MockStoredFieldVisitor) BinaryField(field string, value []byte) {
	v.fields[field] = value
}

func (v *MockStoredFieldVisitor) IntField(field string, value int) {
	v.fields[field] = value
}

func (v *MockStoredFieldVisitor) LongField(field string, value int64) {
	v.fields[field] = value
}

func (v *MockStoredFieldVisitor) FloatField(field string, value float32) {
	v.fields[field] = value
}

func (v *MockStoredFieldVisitor) DoubleField(field string, value float64) {
	v.fields[field] = value
}

// TestCompressingStoredFieldsReader_VisitDocument tests reading documents
func TestCompressingStoredFieldsReader_VisitDocument(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	segmentInfo := index.NewSegmentInfo("_0", 1, dir)
	fieldInfos := index.NewFieldInfos()

	format := DefaultCompressingStoredFieldsFormat()
	writer, err := format.FieldsWriter(dir, segmentInfo, store.IOContext{Context: store.ContextWrite})
	if err != nil {
		t.Fatalf("FieldsWriter failed: %v", err)
	}

	// Write a document
	if err := writer.StartDocument(); err != nil {
		t.Fatalf("StartDocument failed: %v", err)
	}

	field, err := document.NewStringField("test_field", "test_value", true)
	if err != nil {
		t.Fatalf("NewStringField failed: %v", err)
	}
	if err := writer.WriteField(field); err != nil {
		t.Fatalf("WriteField failed: %v", err)
	}

	if err := writer.FinishDocument(); err != nil {
		t.Fatalf("FinishDocument failed: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Writer Close failed: %v", err)
	}

	// Read the document
	reader, err := format.FieldsReader(dir, segmentInfo, fieldInfos, store.IOContext{Context: store.ContextRead})
	if err != nil {
		t.Fatalf("FieldsReader failed: %v", err)
	}
	defer reader.Close()

	visitor := NewMockStoredFieldVisitor()
	if err := reader.VisitDocument(0, visitor); err != nil {
		t.Fatalf("VisitDocument failed: %v", err)
	}

	if value, ok := visitor.fields["test_field"]; !ok {
		t.Error("test_field not found in visitor")
	} else if str, ok := value.(string); !ok || str != "test_value" {
		t.Errorf("expected test_value, got %v", value)
	}
}

// TestCompressingStoredFieldsReader_OutOfRange tests out of range document access
func TestCompressingStoredFieldsReader_OutOfRange(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	segmentInfo := index.NewSegmentInfo("_0", 0, dir)
	fieldInfos := index.NewFieldInfos()

	format := DefaultCompressingStoredFieldsFormat()
	reader, err := format.FieldsReader(dir, segmentInfo, fieldInfos, store.IOContext{Context: store.ContextRead})
	if err != nil {
		t.Fatalf("FieldsReader failed: %v", err)
	}
	defer reader.Close()

	visitor := NewMockStoredFieldVisitor()
	err = reader.VisitDocument(0, visitor)
	if err == nil {
		t.Error("expected error for out of range document, got nil")
	}
}

// TestCompressingStoredFieldsFormat_AllCompressionModes tests all compression modes
func TestCompressingStoredFieldsFormat_AllCompressionModes(t *testing.T) {
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

			format := NewCompressingStoredFieldsFormat(mode, 4096, 16)

			// Write
			writer, err := format.FieldsWriter(dir, segmentInfo, store.IOContext{Context: store.ContextWrite})
			if err != nil {
				t.Fatalf("FieldsWriter failed: %v", err)
			}

			if err := writer.StartDocument(); err != nil {
				t.Fatalf("StartDocument failed: %v", err)
			}

			field, err := document.NewStringField("field", "value", true)
			if err != nil {
				t.Fatalf("NewStringField failed: %v", err)
			}
			if err := writer.WriteField(field); err != nil {
				t.Fatalf("WriteField failed: %v", err)
			}

			if err := writer.FinishDocument(); err != nil {
				t.Fatalf("FinishDocument failed: %v", err)
			}

			if err := writer.Close(); err != nil {
				t.Fatalf("Writer Close failed: %v", err)
			}

			// Read
			reader, err := format.FieldsReader(dir, segmentInfo, fieldInfos, store.IOContext{Context: store.ContextRead})
			if err != nil {
				t.Fatalf("FieldsReader failed: %v", err)
			}
			defer reader.Close()

			visitor := NewMockStoredFieldVisitor()
			if err := reader.VisitDocument(0, visitor); err != nil {
				t.Fatalf("VisitDocument failed: %v", err)
			}

			if value, ok := visitor.fields["field"]; !ok {
				t.Error("field not found")
			} else if str, ok := value.(string); !ok || str != "value" {
				t.Errorf("expected 'value', got %v", value)
			}
		})
	}
}
