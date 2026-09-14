// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"
	"io"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
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

// tokenStreamValue wraps a pre-analyzed TokenStream value. Mirrors Lucene's
// Field(String, TokenStream, IndexableFieldType) constructor, which stores the
// TokenStream directly in fieldsData.
type tokenStreamValue struct {
	ts analysis.TokenStream
}

func (tokenStreamValue) isFieldValue() {}

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
	case analysis.TokenStream:
		f.value = tokenStreamValue{ts: v}
	case io.Reader:
		if ft.stored {
			return nil, fmt.Errorf("fields with a Reader value cannot be stored")
		}
		if ft.IsIndexed() && !ft.tokenized {
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

// FieldType returns the IndexableFieldType describing the properties of this
// field. Mirrors org.apache.lucene.document.Field#fieldType(), which returns
// the IndexableFieldType interface rather than the concrete FieldType.
func (f *Field) FieldType() spi.IndexableFieldType {
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

// SetBinaryValue sets the binary value of the field.
func (f *Field) SetBinaryValue(value []byte) {
	f.value = binaryValue(value)
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

// TokenStream creates the TokenStream used for indexing this field.
//
// Mirrors org.apache.lucene.document.Field#tokenStream(Analyzer, TokenStream)
// from Apache Lucene 10.5.0. The branches dispatch on the concrete value
// variant, which is exactly what Java's "fieldsData instanceof ..." chain does.
func (f *Field) TokenStream(analyzer analysis.Analyzer, reuse analysis.TokenStream) analysis.TokenStream {
	if f.ft.IndexOptions() == spi.IndexOptionsNone {
		// Not indexed
		return nil
	}

	if !f.ft.Tokenized() {
		switch v := f.value.(type) {
		case stringValue:
			sts, ok := reuse.(*stringTokenStream)
			if !ok {
				// lazy init the TokenStream as it is heavy to instantiate
				// (attributes,...) if not needed
				sts = newStringTokenStream()
			}
			sts.setValue(string(v))
			return sts
		case binaryValue:
			bts, ok := reuse.(*binaryTokenStream)
			if !ok {
				bts = newBinaryTokenStream()
			}
			bts.setValue([]byte(v))
			return bts
		default:
			panic("Non-Tokenized Fields must have a String value")
		}
	}

	switch v := f.value.(type) {
	case tokenStreamValue:
		return v.ts
	case readerValue:
		ts, err := analyzer.TokenStream(f.name, v.r)
		if err != nil {
			panic(err)
		}
		return ts
	case stringValue:
		ts, err := analyzer.TokenStream(f.name, strings.NewReader(string(v)))
		if err != nil {
			panic(err)
		}
		return ts
	}

	panic(fmt.Sprintf("Field must have either TokenStream, String, Reader or Number value; got %s", f.name))
}

// TokenStreamValue returns the pre-analyzed TokenStream this field was built
// with, or nil. Mirrors org.apache.lucene.document.Field#tokenStreamValue().
func (f *Field) TokenStreamValue() analysis.TokenStream {
	if v, ok := f.value.(tokenStreamValue); ok {
		return v.ts
	}
	return nil
}

// GetCharSequenceValue returns the field value as a character sequence.
//
// Mirrors org.apache.lucene.index.IndexableField#getCharSequenceValue(), which
// Field overrides as "fieldsData instanceof CharSequence ? (CharSequence)
// fieldsData : stringValue()".
func (f *Field) GetCharSequenceValue() string {
	if v, ok := f.value.(stringValue); ok {
		return string(v)
	}
	return f.StringValue()
}

// StoredValue returns the value to write into the stored-fields stream, or nil
// when the field is not stored.
//
// Mirrors org.apache.lucene.document.Field#storedValue() from Apache Lucene
// 10.5.0.
func (f *Field) StoredValue() *StoredValue {
	if !f.ft.Stored() {
		return nil
	}
	switch v := f.value.(type) {
	case stringValue:
		return NewStoredValueString(string(v))
	case binaryValue:
		return NewStoredValueBinary([]byte(v))
	case numericValue:
		switch n := v.n.(type) {
		case int32:
			return NewStoredValueInt(n)
		case int64:
			return NewStoredValueLong(n)
		case float32:
			return NewStoredValueFloat(n)
		case float64:
			return NewStoredValueDouble(n)
		case int:
			return NewStoredValueInt(int32(n))
		case int16:
			return NewStoredValueInt(int32(n))
		case byte:
			return NewStoredValueInt(int32(n))
		}
		panic(fmt.Sprintf("Cannot store value of type %T", v.n))
	case nil:
		panic("fieldsData is unset")
	}
	panic(fmt.Sprintf("Cannot store value of type %T", f.value))
}

// stringTokenStream returns a String as a single token.
//
// Mirrors the private nested class org.apache.lucene.document.Field.
// StringTokenStream. Lucene declares a second, independent StringTokenStream
// inside Analyzer; both are private and they are not the same class, so this
// port keeps them separate too.
type stringTokenStream struct {
	*analysis.BaseTokenStream
	termAttribute   analysis.CharTermAttribute
	offsetAttribute analysis.OffsetAttribute
	used            bool
	value           string
}

func newStringTokenStream() *stringTokenStream {
	ts := &stringTokenStream{BaseTokenStream: analysis.NewBaseTokenStream(), used: true}
	ts.termAttribute = ts.GetAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	ts.offsetAttribute = ts.GetAttribute(analysis.OffsetAttributeType).(analysis.OffsetAttribute)
	return ts
}

// setValue sets the string value.
func (ts *stringTokenStream) setValue(value string) { ts.value = value }

func (ts *stringTokenStream) IncrementToken() (bool, error) {
	if ts.used {
		return false, nil
	}
	ts.ClearAttributes()
	ts.termAttribute.AppendString(ts.value)
	ts.offsetAttribute.SetOffset(0, len(ts.value))
	ts.used = true
	return true, nil
}

func (ts *stringTokenStream) End() error {
	if err := ts.BaseTokenStream.End(); err != nil {
		return err
	}
	finalOffset := len(ts.value)
	ts.offsetAttribute.SetOffset(finalOffset, finalOffset)
	return nil
}

func (ts *stringTokenStream) Reset() error { ts.used = false; return nil }

func (ts *stringTokenStream) Close() error { ts.value = ""; return nil }

// binaryTokenStream returns a BytesRef as a single token.
//
// Mirrors the private nested class org.apache.lucene.document.Field.
// BinaryTokenStream.
type binaryTokenStream struct {
	*analysis.BaseTokenStream
	bytesAtt analysis.BytesTermAttribute
	used     bool
	value    []byte
}

func newBinaryTokenStream() *binaryTokenStream {
	ts := &binaryTokenStream{BaseTokenStream: analysis.NewBaseTokenStream(), used: true}
	ts.bytesAtt = ts.GetAttribute(analysis.BytesTermAttributeType).(analysis.BytesTermAttribute)
	return ts
}

// setValue sets the binary value.
func (ts *binaryTokenStream) setValue(value []byte) { ts.value = value }

func (ts *binaryTokenStream) IncrementToken() (bool, error) {
	if ts.used {
		return false, nil
	}
	ts.ClearAttributes()
	ts.bytesAtt.SetBytesRef(util.NewBytesRef(ts.value))
	ts.used = true
	return true, nil
}

func (ts *binaryTokenStream) Reset() error { ts.used = false; return nil }

func (ts *binaryTokenStream) Close() error { ts.value = nil; return nil }

// IsIndexed returns whether this field is indexed.
func (f *Field) IsIndexed() bool {
	if f.ft == nil {
		return false
	}
	return f.ft.indexed
}

// IsStored returns whether this field is stored.
func (f *Field) IsStored() bool {
	if f.ft == nil {
		return false
	}
	return f.ft.stored
}

// IsTokenized returns whether this field is tokenized.
func (f *Field) IsTokenized() bool {
	if f.ft == nil {
		return false
	}
	return f.ft.tokenized
}

// IndexOptions returns the index options for this field.
func (f *Field) IndexOptions() interface{} {
	if f.ft == nil {
		return nil
	}
	return f.ft.indexOptions
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
