// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"
	"io"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// Field is the base struct for all field types.
// It stores a field name and value along with indexing metadata.
//
// This is the Go port of Lucene's org.apache.lucene.document.Field.
type Field struct {
	name  string
	value fieldValue
	ft    *FieldType
}

// fieldValue is an interface for different field value types.
type fieldValue interface {
	String() string
	Binary() []byte
	Reader() io.Reader
	Numeric() interface{}
}

// stringValue wraps a string value.
type stringValue string

func (v stringValue) String() string       { return string(v) }
func (v stringValue) Binary() []byte       { return []byte(v) }
func (v stringValue) Reader() io.Reader    { return strings.NewReader(string(v)) }
func (v stringValue) Numeric() interface{} { return nil }

// binaryValue wraps a binary value.
type binaryValue []byte

func (v binaryValue) String() string       { return string(v) }
func (v binaryValue) Binary() []byte       { return v }
func (v binaryValue) Reader() io.Reader    { return nil }
func (v binaryValue) Numeric() interface{} { return nil }

// readerValue wraps an io.Reader.
type readerValue struct {
	r io.Reader
}

func (v readerValue) String() string       { return "" }
func (v readerValue) Binary() []byte       { return nil }
func (v readerValue) Reader() io.Reader    { return v.r }
func (v readerValue) Numeric() interface{} { return nil }

// numericValue wraps a numeric value.
type numericValue struct {
	n interface{}
}

func (v numericValue) String() string {
	return fmt.Sprintf("%v", v.n)
}
func (v numericValue) Binary() []byte       { return nil }
func (v numericValue) Reader() io.Reader    { return nil }
func (v numericValue) Numeric() interface{} { return v.n }

// NewField creates a new Field with the given name, value, and FieldType.
// The value can be a string, []byte, io.Reader, or numeric type (int, int64, float32, float64).
func NewField(name string, value interface{}, ft *FieldType) (*Field, error) {
	if name == "" {
		return nil, fmt.Errorf("field name cannot be empty")
	}
	if ft == nil {
		return nil, fmt.Errorf("field type cannot be nil")
	}
	if err := ft.Validate(); err != nil {
		return nil, err
	}

	f := &Field{
		name: name,
		ft:   ft,
	}

	switch v := value.(type) {
	case string:
		f.value = stringValue(v)
	case []byte:
		f.value = binaryValue(v)
	case io.Reader:
		f.value = readerValue{r: v}
	case int:
		f.value = numericValue{n: int64(v)}
	case int64:
		f.value = numericValue{n: v}
	case float32:
		f.value = numericValue{n: float64(v)}
	case float64:
		f.value = numericValue{n: v}
	default:
		return nil, fmt.Errorf("unsupported field value type: %T", value)
	}

	return f, nil
}

// Name returns the name of the field.
func (f *Field) Name() string {
	return f.name
}

// FieldType returns the FieldType for this field.
func (f *Field) FieldType() *FieldType {
	return f.ft
}

// StringValue returns the string value of the field.
func (f *Field) StringValue() string {
	if f.value == nil {
		return ""
	}
	return f.value.String()
}

// ReaderValue returns a reader for the field value.
func (f *Field) ReaderValue() io.Reader {
	if f.value == nil {
		return nil
	}
	return f.value.Reader()
}

// BinaryValue returns the binary value of the field.
func (f *Field) BinaryValue() []byte {
	if f.value == nil {
		return nil
	}
	return f.value.Binary()
}

// NumericValue returns the numeric value of the field.
func (f *Field) NumericValue() interface{} {
	if f.value == nil {
		return nil
	}
	return f.value.Numeric()
}

// IsStored returns true if the field value is stored.
func (f *Field) IsStored() bool {
	return f.ft.Stored
}

// IsIndexed returns true if the field is indexed.
func (f *Field) IsIndexed() bool {
	return f.ft.Indexed
}

// IsTokenized returns true if the field value is tokenized.
func (f *Field) IsTokenized() bool {
	return f.ft.Tokenized
}

// IndexOptions returns the index options for this field.
func (f *Field) IndexOptions() index.IndexOptions {
	return f.ft.IndexOptions
}

// Ensure Field implements IndexableField
var _ IndexableField = (*Field)(nil)
