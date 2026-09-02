// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"
	"io"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// fieldValue is an interface for the different types of values a Field can hold.
type fieldValue interface {
	isFieldValue()
}

// stringValue wraps a string value.
type stringValue string

func (stringValue) isFieldValue() {}

// readerValue wraps a Reader value.
type readerValue struct {
	r io.Reader
}

func (readerValue) isFieldValue() {}

// binaryValue wraps a []byte value.
type binaryValue []byte

func (binaryValue) isFieldValue() {}

// numericValue wraps a numeric value (int, int64, float32, float64, byte, int16, int32).
type numericValue struct {
	n interface{}
}

func (numericValue) isFieldValue() {}

// Field is a section of a Document.
// Each field has three parts: name, type, and value.
// Values may be text (string, Reader), binary ([]byte), or numeric (a Number).
//
// This is the Go port of Lucene's org.apache.lucene.document.Field.
type Field struct {
	name  string
	ft    *FieldType
	value fieldValue
}

// NewField creates a field with the given name, value, and type.
// The value can be a string, io.Reader, []byte, int, int64, float32, or float64.
func NewField(name string, value interface{}, ft *FieldType) (*Field, error) {
	if name == "" {
		return nil, fmt.Errorf("field name must not be empty")
	}
	if ft == nil {
		return nil, fmt.Errorf("field type must not be nil")
	}

	f := &Field{
		name: name,
		ft:   ft,
	}

	switch v := value.(type) {
	case string:
		f.value = stringValue(v)
	case io.Reader:
		if ft.Stored {
			return nil, fmt.Errorf("fields with a Reader value cannot be stored")
		}
		if ft.IsIndexed() && !ft.Tokenized {
			return nil, fmt.Errorf("non-tokenized fields must use String values")
		}
		f.value = readerValue{r: v}
	case []byte:
		f.value = binaryValue(v)
	case int:
		f.value = numericValue{n: int32(v)}
	case int32:
		f.value = numericValue{n: v}
	case int64:
		f.value = numericValue{n: v}
	case float32:
		f.value = numericValue{n: v}
	case float64:
		f.value = numericValue{n: v}
	case byte:
		f.value = numericValue{n: v}
	case int16:
		f.value = numericValue{n: v}
	case nil:
		// Allow nil value for now
		f.value = nil
	default:
		return nil, fmt.Errorf("unsupported value type: %T", value)
	}

	return f, nil
}


// Name returns the field name.
func (f *Field) Name() string {
	return f.name
}

// FieldType returns the field type.
func (f *Field) FieldType() *FieldType {
	return f.ft
}

// StringValue returns the string value of the field.
// Returns empty string if the field has no string value.
func (f *Field) StringValue() string {
	if f.value == nil {
		return ""
	}
	switch v := f.value.(type) {
	case stringValue:
		return string(v)
	case binaryValue:
		return string(v)
	default:
		return ""
	}
}

// ReaderValue returns a reader for the field value.
// Returns nil if the field has no reader value.
func (f *Field) ReaderValue() io.Reader {
	if f.value == nil {
		return nil
	}
	switch v := f.value.(type) {
	case readerValue:
		return v.r
	default:
		return nil
	}
}

// BinaryValue returns the binary value of the field.
// Returns nil if the field has no binary value.
func (f *Field) BinaryValue() []byte {
	if f.value == nil {
		return nil
	}
	switch v := f.value.(type) {
	case binaryValue:
		return []byte(v)
	case stringValue:
		return []byte(v)
	default:
		return nil
	}
}

// NumericValue returns the numeric value of the field.
// The interface{} can be byte, int16, int32, int64, float32, or float64.
// Returns nil if the field has no numeric value.
func (f *Field) NumericValue() interface{} {
	if f.value == nil {
		return nil
	}
	switch v := f.value.(type) {
	case numericValue:
		return v.n
	default:
		return nil
	}
}

// TokenStream returns a TokenStream for the field value.
// Currently returns nil (to be implemented with analysis integration).
func (f *Field) TokenStream() analysis.TokenStream {
	// TODO: Implement token stream handling when analysis.TokenStream is fully available
	return nil
}

// IsIndexed returns whether this field is indexed.
func (f *Field) IsIndexed() bool {
	if f.ft == nil {
		return false
	}
	return f.ft.Indexed
}

// IsStored returns whether this field is stored.
func (f *Field) IsStored() bool {
	if f.ft == nil {
		return false
	}
	return f.ft.Stored
}

// IsTokenized returns whether this field is tokenized.
func (f *Field) IsTokenized() bool {
	if f.ft == nil {
		return false
	}
	return f.ft.Tokenized
}

// IndexOptions returns the index options for this field.
func (f *Field) IndexOptions() interface{} {
	if f.ft == nil {
		return nil
	}
	return f.ft.IndexOptions
}

// FloatValue returns the float32 value of the field if it has one.
func (f *Field) FloatValue() interface{} {
	if f.value == nil {
		return nil
	}
	switch v := f.value.(type) {
	case numericValue:
		if fv, ok := v.n.(float32); ok {
			return fv
		}
	}
	return nil
}

// DoubleValue returns the float64 value of the field if it has one.
func (f *Field) DoubleValue() interface{} {
	if f.value == nil {
		return nil
	}
	switch v := f.value.(type) {
	case numericValue:
		if dv, ok := v.n.(float64); ok {
			return dv
		}
	}
	return nil
}

// NewIntField creates an indexed int32 field.
func NewIntField(name string, value int, stored bool) (*Field, error) {
	ft := NewFieldType()
	ft.SetStored(stored)
	ft.SetIndexed(true)
	ft.SetTokenized(false)
	ft.SetOmitNorms(true)
	return NewField(name, int32(value), ft)
}

// NewLongField creates an indexed int64 field.
func NewLongField(name string, value int64, stored bool) (*Field, error) {
	ft := NewFieldType()
	ft.SetStored(stored)
	ft.SetIndexed(true)
	ft.SetTokenized(false)
	ft.SetOmitNorms(true)
	return NewField(name, value, ft)
}

// NewFloatField creates an indexed float32 field.
func NewFloatField(name string, value float32, stored bool) (*Field, error) {
	ft := NewFieldType()
	ft.SetStored(stored)
	ft.SetIndexed(true)
	ft.SetTokenized(false)
	ft.SetOmitNorms(true)
	return NewField(name, value, ft)
}

// NewDoubleField creates an indexed float64 field.
func NewDoubleField(name string, value float64, stored bool) (*Field, error) {
	ft := NewFieldType()
	ft.SetStored(stored)
	ft.SetIndexed(true)
	ft.SetTokenized(false)
	ft.SetOmitNorms(true)
	return NewField(name, value, ft)
}

// NewIntPoint creates a point field for int32 values.
func NewIntPoint(name string, value int32) *Field {
	ft := NewFieldType()
	ft.SetIndexed(true)
	ft.SetStored(false)
	ft.SetTokenized(false)
	f, _ := NewField(name, value, ft)
	return f
}

// NewLongPoint creates a point field for int64 values.
func NewLongPoint(name string, value int64) *Field {
	ft := NewFieldType()
	ft.SetIndexed(true)
	ft.SetStored(false)
	ft.SetTokenized(false)
	f, _ := NewField(name, value, ft)
	return f
}

// NewFloatPoint creates a point field for float32 values.
func NewFloatPoint(name string, value float32) *Field {
	ft := NewFieldType()
	ft.SetIndexed(true)
	ft.SetStored(false)
	ft.SetTokenized(false)
	f, _ := NewField(name, value, ft)
	return f
}

// NewDoublePoint creates a point field for float64 values.
func NewDoublePoint(name string, value float64) *Field {
	ft := NewFieldType()
	ft.SetIndexed(true)
	ft.SetStored(false)
	ft.SetTokenized(false)
	f, _ := NewField(name, value, ft)
	return f
}
