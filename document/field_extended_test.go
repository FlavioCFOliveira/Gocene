// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"bytes"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// TestField_Extended tests extended field functionality
// Ported from Lucene's TestField.java

func TestField_StringField(t *testing.T) {
	tests := []struct {
		name    string
		stored  bool
		wantErr bool
	}{
		{"stored", true, false},
		{"not_stored", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, err := NewStringField("foo", "bar", tt.stored)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewStringField() error = %v, wantErr %v", err, tt.wantErr)
			}
			if field == nil {
				t.Fatal("Expected non-nil field")
			}

			// Verify field properties
			if field.Name() != "foo" {
				t.Errorf("Expected name 'foo', got '%s'", field.Name())
			}
			if field.StringValue() != "bar" {
				t.Errorf("Expected value 'bar', got '%s'", field.StringValue())
			}

			// Verify FieldType properties
			ft := field.FieldType()
			if !ft.Indexed {
				t.Error("Expected field to be indexed")
			}
			if ft.Tokenized {
				t.Error("Expected field to not be tokenized")
			}
			if ft.OmitNorms != true {
				t.Error("Expected field to omit norms")
			}
			if ft.Stored != tt.stored {
				t.Errorf("Expected stored=%v, got %v", tt.stored, ft.Stored)
			}

			// Test that StringField cannot be modified with wrong value types
			// In Go, we test that the field value accessors work correctly
			// Note: StringField stores string value which provides a Reader via strings.NewReader
			if field.ReaderValue() == nil {
				t.Error("Expected ReaderValue to be available for StringField (via string conversion)")
			}
			if field.NumericValue() != nil {
				t.Error("Expected NumericValue to be nil for StringField")
			}
		})
	}
}

func TestField_StringFieldFromBytes(t *testing.T) {
	tests := []struct {
		name    string
		value   []byte
		stored  bool
		wantErr bool
	}{
		{"stored", []byte("bar"), true, false},
		{"not_stored", []byte("bar"), false, false},
		{"empty", []byte{}, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, err := NewStringFieldFromBytes("foo", tt.value, tt.stored)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewStringFieldFromBytes() error = %v, wantErr %v", err, tt.wantErr)
			}
			if field == nil {
				t.Fatal("Expected non-nil field")
			}

			// Verify binary value
			if !bytes.Equal(field.BinaryValue(), tt.value) {
				t.Errorf("Expected binary value %v, got %v", tt.value, field.BinaryValue())
			}

			// Verify string value
			if field.StringValue() != string(tt.value) {
				t.Errorf("Expected string value '%s', got '%s'", string(tt.value), field.StringValue())
			}
		})
	}
}

func TestField_TextField(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		stored  bool
		wantErr bool
	}{
		{"stored", "Hello World", true, false},
		{"not_stored", "Hello World", false, false},
		{"empty", "", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, err := NewTextField("content", tt.value, tt.stored)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewTextField() error = %v, wantErr %v", err, tt.wantErr)
			}
			if field == nil {
				t.Fatal("Expected non-nil field")
			}

			// Verify field properties
			if field.Name() != "content" {
				t.Errorf("Expected name 'content', got '%s'", field.Name())
			}
			if field.StringValue() != tt.value {
				t.Errorf("Expected value '%s', got '%s'", tt.value, field.StringValue())
			}

			// Verify FieldType properties
			ft := field.FieldType()
			if !ft.Indexed {
				t.Error("Expected field to be indexed")
			}
			if !ft.Tokenized {
				t.Error("Expected field to be tokenized")
			}
			if ft.Stored != tt.stored {
				t.Errorf("Expected stored=%v, got %v", tt.stored, ft.Stored)
			}
		})
	}
}

func TestField_TextFieldFromReader(t *testing.T) {
	reader := strings.NewReader("Hello from reader")
	field, err := NewTextFieldFromReader("content", reader)
	if err != nil {
		t.Fatalf("NewTextFieldFromReader() error = %v", err)
	}
	if field == nil {
		t.Fatal("Expected non-nil field")
	}

	// Verify field properties
	if field.Name() != "content" {
		t.Errorf("Expected name 'content', got '%s'", field.Name())
	}

	// Reader value should be set
	if field.ReaderValue() == nil {
		t.Error("Expected ReaderValue to be set")
	}

	// Verify FieldType - should not be stored
	ft := field.FieldType()
	if ft.Stored {
		t.Error("Expected field from reader to not be stored")
	}
	if !ft.Tokenized {
		t.Error("Expected field to be tokenized")
	}
}

func TestField_StoredField(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
	}{
		{"string", "stored value"},
		{"bytes", []byte("stored bytes")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var field *StoredField
			var err error

			switch v := tt.value.(type) {
			case string:
				field, err = NewStoredField("stored", v)
			case []byte:
				field, err = NewStoredFieldFromBytes("stored", v)
			}

			if err != nil {
				t.Fatalf("Failed to create StoredField: %v", err)
			}
			if field == nil {
				t.Fatal("Expected non-nil field")
			}

			// Verify FieldType properties
			ft := field.FieldType()
			if ft.Indexed {
				t.Error("Expected stored field to not be indexed")
			}
			if !ft.Stored {
				t.Error("Expected field to be stored")
			}
			if ft.Tokenized {
				t.Error("Expected stored field to not be tokenized")
			}
		})
	}
}

func TestField_StoredFieldNumeric(t *testing.T) {
	tests := []struct {
		name     string
		create   func() (*StoredField, error)
		expected interface{}
	}{
		{
			name:     "int",
			create:   func() (*StoredField, error) { return NewStoredFieldFromInt("count", 42) },
			expected: int64(42),
		},
		{
			name:     "int64",
			create:   func() (*StoredField, error) { return NewStoredFieldFromInt64("timestamp", 1234567890) },
			expected: int64(1234567890),
		},
		{
			name:     "float64",
			create:   func() (*StoredField, error) { return NewStoredFieldFromFloat64("score", 3.14) },
			expected: 3.14,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, err := tt.create()
			if err != nil {
				t.Fatalf("Failed to create StoredField: %v", err)
			}
			if field == nil {
				t.Fatal("Expected non-nil field")
			}

			// Verify numeric value
			if field.NumericValue() != tt.expected {
				t.Errorf("Expected numeric value %v, got %v", tt.expected, field.NumericValue())
			}

			// Verify FieldType
			ft := field.FieldType()
			if ft.Indexed {
				t.Error("Expected stored field to not be indexed")
			}
			if !ft.Stored {
				t.Error("Expected field to be stored")
			}
		})
	}
}

func TestField_IntField(t *testing.T) {
	tests := []struct {
		name   string
		value  int
		stored bool
	}{
		{"stored", 42, true},
		{"not_stored", 42, false},
		{"zero", 0, true},
		{"negative", -100, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, err := NewIntField("count", tt.value, tt.stored)
			if err != nil {
				t.Fatalf("NewIntField() error = %v", err)
			}
			if field == nil {
				t.Fatal("Expected non-nil field")
			}

			// Verify field properties
			if field.Name() != "count" {
				t.Errorf("Expected name 'count', got '%s'", field.Name())
			}

			// Verify FieldType
			ft := field.FieldType()
			if !ft.Indexed {
				t.Error("Expected field to be indexed")
			}
			if ft.Stored != tt.stored {
				t.Errorf("Expected stored=%v, got %v", tt.stored, ft.Stored)
			}
		})
	}
}

func TestField_LongField(t *testing.T) {
	tests := []struct {
		name   string
		value  int64
		stored bool
	}{
		{"stored", int64(9223372036854775807), true},
		{"not_stored", int64(1234567890), false},
		{"zero", int64(0), true},
		{"negative", int64(-9999999999), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, err := NewLongField("timestamp", tt.value, tt.stored)
			if err != nil {
				t.Fatalf("NewLongField() error = %v", err)
			}
			if field == nil {
				t.Fatal("Expected non-nil field")
			}

			// Verify field properties
			if field.Name() != "timestamp" {
				t.Errorf("Expected name 'timestamp', got '%s'", field.Name())
			}

			// Verify FieldType
			ft := field.FieldType()
			if !ft.Indexed {
				t.Error("Expected field to be indexed")
			}
			if ft.Stored != tt.stored {
				t.Errorf("Expected stored=%v, got %v", tt.stored, ft.Stored)
			}
		})
	}
}

func TestField_FloatField(t *testing.T) {
	tests := []struct {
		name   string
		value  float32
		stored bool
	}{
		{"stored", float32(3.14159), true},
		{"not_stored", float32(2.71828), false},
		{"zero", float32(0.0), true},
		{"negative", float32(-99.99), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, err := NewFloatField("rating", tt.value, tt.stored)
			if err != nil {
				t.Fatalf("NewFloatField() error = %v", err)
			}
			if field == nil {
				t.Fatal("Expected non-nil field")
			}

			// Verify field properties
			if field.Name() != "rating" {
				t.Errorf("Expected name 'rating', got '%s'", field.Name())
			}

			// Verify FieldType
			ft := field.FieldType()
			if !ft.Indexed {
				t.Error("Expected field to be indexed")
			}
			if ft.Stored != tt.stored {
				t.Errorf("Expected stored=%v, got %v", tt.stored, ft.Stored)
			}
		})
	}
}

func TestField_DoubleField(t *testing.T) {
	tests := []struct {
		name   string
		value  float64
		stored bool
	}{
		{"stored", 3.14159265359, true},
		{"not_stored", 2.71828182846, false},
		{"zero", 0.0, true},
		{"negative", -999999.999999, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, err := NewDoubleField("precision", tt.value, tt.stored)
			if err != nil {
				t.Fatalf("NewDoubleField() error = %v", err)
			}
			if field == nil {
				t.Fatal("Expected non-nil field")
			}

			// Verify field properties
			if field.Name() != "precision" {
				t.Errorf("Expected name 'precision', got '%s'", field.Name())
			}

			// Verify FieldType
			ft := field.FieldType()
			if !ft.Indexed {
				t.Error("Expected field to be indexed")
			}
			if ft.Stored != tt.stored {
				t.Errorf("Expected stored=%v, got %v", tt.stored, ft.Stored)
			}
		})
	}
}

func TestField_PointFields(t *testing.T) {
	tests := []struct {
		name       string
		createFunc func() (IndexableField, error)
		wantValue  interface{}
	}{
		{
			name:       "IntPoint",
			createFunc: func() (IndexableField, error) { f, err := NewIntPoint("ip", 42); return f, err },
			wantValue:  42,
		},
		{
			name:       "LongPoint",
			createFunc: func() (IndexableField, error) { f, err := NewLongPoint("lp", int64(123)); return f, err },
			wantValue:  int64(123),
		},
		{
			name:       "FloatPoint",
			createFunc: func() (IndexableField, error) { f, err := NewFloatPoint("fp", float32(3.14)); return f, err },
			wantValue:  float32(3.14),
		},
		{
			name:       "DoublePoint",
			createFunc: func() (IndexableField, error) { f, err := NewDoublePoint("dp", 3.14159); return f, err },
			wantValue:  3.14159,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, err := tt.createFunc()
			if err != nil {
				t.Fatalf("Failed to create point field: %v", err)
			}
			if field == nil {
				t.Fatal("Expected non-nil field")
			}

			// Verify FieldType
			ft := field.FieldType()
			if !ft.Indexed {
				t.Error("Expected point field to be indexed")
			}
			if ft.Stored {
				t.Error("Expected point field to not be stored")
			}
		})
	}
}

// TestField_Combinations tests various field option combinations
func TestField_Combinations(t *testing.T) {
	tests := []struct {
		name      string
		indexed   bool
		stored    bool
		tokenized bool
		wantErr   bool
	}{
		{"indexed_only", true, false, false, false},
		{"stored_only", false, true, false, false},
		{"indexed_and_stored", true, true, false, false},
		{"tokenized_and_indexed", true, false, true, false},
		{"all_options", true, true, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ft := NewFieldType()
			ft.SetIndexed(tt.indexed).
				SetStored(tt.stored).
				SetTokenized(tt.tokenized)

			// If indexed, must set IndexOptions
			if tt.indexed {
				ft.SetIndexOptions(index.IndexOptionsDocs)
			}

			field, err := NewField("test", "value", ft)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewField() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}

			// Verify properties
			if field.IsIndexed() != tt.indexed {
				t.Errorf("Expected indexed=%v, got %v", tt.indexed, field.IsIndexed())
			}
			if field.IsStored() != tt.stored {
				t.Errorf("Expected stored=%v, got %v", tt.stored, field.IsStored())
			}
			if field.IsTokenized() != tt.tokenized {
				t.Errorf("Expected tokenized=%v, got %v", tt.tokenized, field.IsTokenized())
			}
		})
	}
}

// TestField_ValueTypes tests field creation with different value types
func TestField_ValueTypes(t *testing.T) {
	ft := NewFieldType()
	ft.SetStored(true)

	tests := []struct {
		name     string
		value    interface{}
		expected string
	}{
		{"string", "test value", "test value"},
		{"bytes", []byte("byte value"), "byte value"},
		{"int", 42, "42"},
		{"int64", int64(9223372036854775807), "9223372036854775807"},
		{"float32", float32(3.14), "3.14"},
		{"float64", 3.14159, "3.14159"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, err := NewField("test", tt.value, ft)
			if err != nil {
				t.Fatalf("NewField() error = %v", err)
			}

			if field.StringValue() != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, field.StringValue())
			}
		})
	}
}

// TestField_ValueAccessors tests field value accessor methods
func TestField_ValueAccessors(t *testing.T) {
	t.Run("string_value", func(t *testing.T) {
		ft := NewFieldType()
		ft.SetStored(true)
		field, _ := NewField("test", "string value", ft)

		if field.StringValue() != "string value" {
			t.Errorf("Expected 'string value', got '%s'", field.StringValue())
		}
		if field.BinaryValue() == nil {
			t.Error("Expected BinaryValue to be available for string")
		}
		if field.ReaderValue() != nil {
			t.Error("Expected ReaderValue to be nil for string field")
		}
	})

	t.Run("binary_value", func(t *testing.T) {
		ft := NewFieldType()
		ft.SetStored(true)
		field, _ := NewField("test", []byte("binary value"), ft)

		if field.StringValue() != "binary value" {
			t.Errorf("Expected 'binary value', got '%s'", field.StringValue())
		}
		if !bytes.Equal(field.BinaryValue(), []byte("binary value")) {
			t.Error("Expected BinaryValue to match")
		}
	})

	t.Run("reader_value", func(t *testing.T) {
		ft := NewFieldType()
		ft.SetStored(true)
		reader := strings.NewReader("reader value")
		field, _ := NewField("test", reader, ft)

		if field.ReaderValue() == nil {
			t.Error("Expected ReaderValue to be set")
		}
		// Reader value String() returns empty
		if field.StringValue() != "" {
			t.Errorf("Expected empty string for reader value, got '%s'", field.StringValue())
		}
	})

	t.Run("numeric_value", func(t *testing.T) {
		ft := NewFieldType()
		ft.SetStored(true)
		field, _ := NewField("test", 42, ft)

		if field.NumericValue() == nil {
			t.Fatal("Expected NumericValue to be set")
		}
		if field.NumericValue() != int64(42) {
			t.Errorf("Expected 42, got %v", field.NumericValue())
		}
	})
}

// TestField_IndexOptions tests field with different index options
func TestField_IndexOptions(t *testing.T) {
	tests := []struct {
		name        string
		options     index.IndexOptions
		shouldWork  bool
	}{
		{"docs_only", index.IndexOptionsDocs, true},
		{"docs_and_freqs", index.IndexOptionsDocsAndFreqs, true},
		{"docs_freqs_positions", index.IndexOptionsDocsAndFreqsAndPositions, true},
		{"docs_freqs_positions_offsets", index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ft := NewFieldType()
			ft.SetIndexed(true).
				SetIndexOptions(tt.options)

			field, err := NewField("test", "value", ft)
			if tt.shouldWork && err != nil {
				t.Fatalf("Expected field creation to succeed: %v", err)
			}
			if !tt.shouldWork && err == nil {
				t.Fatal("Expected field creation to fail")
			}

			if field != nil && field.IndexOptions() != tt.options {
				t.Errorf("Expected IndexOptions %v, got %v", tt.options, field.IndexOptions())
			}
		})
	}
}

// TestField_Validation tests FieldType validation
func TestField_Validation(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*FieldType)
		wantErr bool
	}{
		{
			name: "valid_indexed",
			setup: func(ft *FieldType) {
				ft.SetIndexed(true).SetIndexOptions(index.IndexOptionsDocs)
			},
			wantErr: false,
		},
		{
			name: "invalid_indexed_no_options",
			setup: func(ft *FieldType) {
				ft.SetIndexed(true)
				// IndexOptions not set, should fail
			},
			wantErr: true,
		},
		{
			name: "invalid_tokenized_not_indexed",
			setup: func(ft *FieldType) {
				ft.SetTokenized(true)
				// Indexed not set, should fail
			},
			wantErr: true,
		},
		{
			name: "valid_stored_only",
			setup: func(ft *FieldType) {
				ft.SetStored(true)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ft := NewFieldType()
			tt.setup(ft)

			_, err := NewField("test", "value", ft)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewField() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestField_NilHandling tests handling of nil values
func TestField_NilHandling(t *testing.T) {
	t.Run("nil_field_type", func(t *testing.T) {
		_, err := NewField("test", "value", nil)
		if err == nil {
			t.Error("Expected error for nil FieldType")
		}
	})

	t.Run("empty_name", func(t *testing.T) {
		ft := NewFieldType()
		_, err := NewField("", "value", ft)
		if err == nil {
			t.Error("Expected error for empty field name")
		}
	})

	t.Run("nil_value_in_document", func(t *testing.T) {
		doc := NewDocument()
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic when adding nil field to document")
			}
		}()
		doc.Add(nil)
	})
}

// TestField_DocValuesType tests field with doc values
func TestField_DocValuesType(t *testing.T) {
	tests := []struct {
		name        string
		docValuesType index.DocValuesType
	}{
		{"numeric", index.DocValuesTypeNumeric},
		{"binary", index.DocValuesTypeBinary},
		{"sorted", index.DocValuesTypeSorted},
		{"sorted_set", index.DocValuesTypeSortedSet},
		{"sorted_numeric", index.DocValuesTypeSortedNumeric},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ft := NewFieldType()
			ft.SetStored(true).
				SetDocValuesType(tt.docValuesType)

			field, err := NewField("test", "value", ft)
			if err != nil {
				t.Fatalf("NewField() error = %v", err)
			}

			if field.FieldType().DocValuesType != tt.docValuesType {
				t.Errorf("Expected DocValuesType %v, got %v", tt.docValuesType, field.FieldType().DocValuesType)
			}
		})
	}
}

// TestField_TermVectorOptions tests field with term vector options
func TestField_TermVectorOptions(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*FieldType)
		wantErr  bool
	}{
		{
			name: "valid_term_vectors",
			setup: func(ft *FieldType) {
				ft.SetIndexed(true).
					SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions).
					SetStoreTermVectors(true)
			},
			wantErr: false,
		},
		{
			name: "term_vectors_without_indexed",
			setup: func(ft *FieldType) {
				ft.SetStoreTermVectors(true)
			},
			wantErr: false, // This is allowed (just stored)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ft := NewFieldType()
			tt.setup(ft)

			_, err := NewField("test", "value", ft)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewField() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestField_OmitNorms tests field with omit norms option
func TestField_OmitNorms(t *testing.T) {
	ft := NewFieldType()
	ft.SetIndexed(true).
		SetIndexOptions(index.IndexOptionsDocs).
		SetOmitNorms(true)

	field, err := NewField("test", "value", ft)
	if err != nil {
		t.Fatalf("NewField() error = %v", err)
	}

	if !field.FieldType().OmitNorms {
		t.Error("Expected OmitNorms to be true")
	}
}

// TestField_FloatValueAccessor tests FloatField value accessor
func TestField_FloatValueAccessor(t *testing.T) {
	field, err := NewFloatField("rating", float32(3.14), true)
	if err != nil {
		t.Fatalf("NewFloatField() error = %v", err)
	}

	// Test FloatValue method
	val := field.FloatValue()
	if val != float32(3.14) {
		t.Errorf("Expected 3.14, got %f", val)
	}
}

// TestField_DoubleValueAccessor tests DoubleField value accessor
func TestField_DoubleValueAccessor(t *testing.T) {
	field, err := NewDoubleField("precision", 3.14159265359, true)
	if err != nil {
		t.Fatalf("NewDoubleField() error = %v", err)
	}

	// Test DoubleValue method
	val := field.DoubleValue()
	if val != 3.14159265359 {
		t.Errorf("Expected 3.14159265359, got %f", val)
	}
}

// TestField_EncodingDecoding tests encoding/decoding of point values
func TestField_EncodingDecoding(t *testing.T) {
	t.Run("int32_encoding", func(t *testing.T) {
		tests := []int{0, 1, -1, 2147483647, -2147483648, 42, -100}
		for _, v := range tests {
			encoded := encodeInt32(v)
			decoded := decodeInt32(encoded)
			if decoded != v {
				t.Errorf("int32: expected %d, got %d", v, decoded)
			}
		}
	})

	t.Run("float32_encoding", func(t *testing.T) {
		tests := []float32{0.0, 1.0, -1.0, 3.14, -99.99, 1.5e10, -1.5e-10}
		for _, v := range tests {
			encoded := encodeFloat32(v)
			decoded := decodeFloat32(encoded)
			if decoded != v {
				t.Errorf("float32: expected %f, got %f", v, decoded)
			}
		}
	})

	t.Run("float64_encoding", func(t *testing.T) {
		tests := []float64{0.0, 1.0, -1.0, 3.14159265359, -999999.999999, 1.5e100, -1.5e-100}
		for _, v := range tests {
			encoded := encodeFloat64(v)
			decoded := decodeFloat64(encoded)
			if decoded != v {
				t.Errorf("float64: expected %f, got %f", v, decoded)
			}
		}
	})
}

// TestField_BinaryValueConsistency tests that binary values are consistent
func TestField_BinaryValueConsistency(t *testing.T) {
	tests := []struct {
		name  string
		value []byte
	}{
		{"ascii", []byte("hello")},
		{"utf8", []byte("Hello, 世界")},
		{"binary", []byte{0x00, 0x01, 0x02, 0xFF}},
		{"empty", []byte{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ft := NewFieldType()
			ft.SetStored(true)
			field, err := NewField("test", tt.value, ft)
			if err != nil {
				t.Fatalf("NewField() error = %v", err)
			}

			// Binary value should match input
			if !bytes.Equal(field.BinaryValue(), tt.value) {
				t.Errorf("BinaryValue mismatch: expected %v, got %v", tt.value, field.BinaryValue())
			}

			// String value should be UTF-8 decoded
			if field.StringValue() != string(tt.value) {
				t.Errorf("StringValue mismatch: expected '%s', got '%s'", string(tt.value), field.StringValue())
			}
		})
	}
}

// TestField_FieldTypeImmutability tests that frozen FieldTypes cannot be modified
func TestField_FieldTypeImmutability(t *testing.T) {
	ft := NewFieldType()
	ft.SetStored(true)
	ft.Freeze()

	// Should panic when trying to modify frozen FieldType
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when modifying frozen FieldType")
		}
	}()

	ft.SetIndexed(true)
}

// TestField_StoredFieldFromReader tests creating stored field from reader
func TestField_StoredFieldFromReader(t *testing.T) {
	reader := strings.NewReader("reader content")
	field, err := NewStoredFieldFromReader("stored", reader)
	if err != nil {
		t.Fatalf("NewStoredFieldFromReader() error = %v", err)
	}

	if field.ReaderValue() == nil {
		t.Error("Expected ReaderValue to be set")
	}

	// Verify FieldType
	ft := field.FieldType()
	if ft.Indexed {
		t.Error("Expected stored field to not be indexed")
	}
	if !ft.Stored {
		t.Error("Expected field to be stored")
	}
}

// TestField_MultipleFieldsSameName tests document with multiple fields of same name
func TestField_MultipleFieldsSameName(t *testing.T) {
	doc := NewDocument()

	// Add multiple fields with same name
	for i := 0; i < 3; i++ {
		field, _ := NewStringField("tag", "value", true)
		doc.Add(field)
	}

	// Verify count
	if doc.GetFieldCount("tag") != 3 {
		t.Errorf("Expected 3 fields named 'tag', got %d", doc.GetFieldCount("tag"))
	}

	// Verify GetFieldsByName returns all
	fields := doc.GetFieldsByName("tag")
	if len(fields) != 3 {
		t.Errorf("Expected 3 fields, got %d", len(fields))
	}

	// Verify Get returns first
	first := doc.Get("tag")
	if first == nil {
		t.Error("Expected to get first field")
	}
}

// TestField_FieldTypeEquality tests FieldType comparison
func TestField_FieldTypeEquality(t *testing.T) {
	ft1 := NewFieldType()
	ft1.SetIndexed(true).SetStored(true).SetTokenized(true)
	ft1.SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions)

	ft2 := NewFieldType()
	ft2.SetIndexed(true).SetStored(true).SetTokenized(true)
	ft2.SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions)

	// Compare properties
	if ft1.Indexed != ft2.Indexed {
		t.Error("Indexed should match")
	}
	if ft1.Stored != ft2.Stored {
		t.Error("Stored should match")
	}
	if ft1.Tokenized != ft2.Tokenized {
		t.Error("Tokenized should match")
	}
	if ft1.IndexOptions != ft2.IndexOptions {
		t.Error("IndexOptions should match")
	}
}

// TestField_EdgeCases tests various edge cases
func TestField_EdgeCases(t *testing.T) {
	t.Run("very_long_name", func(t *testing.T) {
		ft := NewFieldType()
		ft.SetStored(true)
		longName := string(make([]byte, 1000))
		for i := range longName {
			longName = longName[:i] + "a" + longName[i+1:]
		}
		field, err := NewField(longName, "value", ft)
		if err != nil {
			t.Fatalf("Failed to create field with long name: %v", err)
		}
		if field.Name() != longName {
			t.Error("Field name mismatch")
		}
	})

	t.Run("unicode_name", func(t *testing.T) {
		ft := NewFieldType()
		ft.SetStored(true)
		unicodeName := "字段_日本語_한국어"
		field, err := NewField(unicodeName, "value", ft)
		if err != nil {
			t.Fatalf("Failed to create field with unicode name: %v", err)
		}
		if field.Name() != unicodeName {
			t.Error("Field name mismatch")
		}
	})

	t.Run("empty_value", func(t *testing.T) {
		ft := NewFieldType()
		ft.SetStored(true)
		field, err := NewField("test", "", ft)
		if err != nil {
			t.Fatalf("Failed to create field with empty value: %v", err)
		}
		if field.StringValue() != "" {
			t.Error("Expected empty string value")
		}
	})

	t.Run("whitespace_value", func(t *testing.T) {
		ft := NewFieldType()
		ft.SetStored(true)
		field, err := NewField("test", "   \t\n  ", ft)
		if err != nil {
			t.Fatalf("Failed to create field with whitespace value: %v", err)
		}
		if field.StringValue() != "   \t\n  " {
			t.Error("Expected whitespace string value")
		}
	})
}
