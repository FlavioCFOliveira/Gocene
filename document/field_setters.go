// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"
	"io"
)

// This file extends Field (defined in field.go) with the Lucene 10.4.0
// setter / accessor surface that was missing from the pre-existing Gocene
// implementation. The setters follow Lucene's type-checking contract: each
// setter asserts that the field's underlying value is of the expected kind
// and panics otherwise (matching the Java original's IllegalArgumentException).

// SetStringValue replaces the string content of the field.
// Panics if the field's current value is not a string.
func (f *Field) SetStringValue(value string) {
	if _, ok := f.value.(stringValue); !ok && f.value != nil {
		panic(fmt.Sprintf("cannot change value type from %T to string", f.value))
	}
	f.value = stringValue(value)
}

// SetReaderValue replaces the reader value of the field.
// Panics if the field's current value is not a Reader.
func (f *Field) SetReaderValue(value io.Reader) {
	if _, ok := f.value.(readerValue); !ok && f.value != nil {
		panic(fmt.Sprintf("cannot change value type from %T to io.Reader", f.value))
	}
	f.value = readerValue{r: value}
}

// SetBytesValue replaces the binary value of the field.
// Panics if the field's current value is not binary.
func (f *Field) SetBytesValue(value []byte) {
	if _, ok := f.value.(binaryValue); !ok && f.value != nil {
		panic(fmt.Sprintf("cannot change value type from %T to []byte", f.value))
	}
	if f.ft != nil && f.ft.IndexOptions != 0 {
		// Lucene rejects binary mutation on indexed fields.
		panic("cannot change binary value of an indexed field")
	}
	f.value = binaryValue(value)
}

// SetByteValue replaces the numeric byte value of the field.
// Panics if the field's current value is not a numeric byte.
func (f *Field) SetByteValue(value byte) {
	if !isNumeric(f.value) {
		panic(fmt.Sprintf("cannot change value type from %T to byte", f.value))
	}
	f.value = numericValue{n: value}
}

// SetShortValue replaces the numeric int16 value of the field.
// Panics if the field's current value is not numeric.
func (f *Field) SetShortValue(value int16) {
	if !isNumeric(f.value) {
		panic(fmt.Sprintf("cannot change value type from %T to int16", f.value))
	}
	f.value = numericValue{n: value}
}

// SetIntValue replaces the numeric int32 value of the field.
// Panics if the field's current value is not numeric.
func (f *Field) SetIntValue(value int32) {
	if !isNumeric(f.value) {
		panic(fmt.Sprintf("cannot change value type from %T to int32", f.value))
	}
	f.value = numericValue{n: value}
}

// SetLongValue replaces the numeric int64 value of the field.
// Panics if the field's current value is not numeric.
func (f *Field) SetLongValue(value int64) {
	if !isNumeric(f.value) {
		panic(fmt.Sprintf("cannot change value type from %T to int64", f.value))
	}
	f.value = numericValue{n: value}
}

// SetFloatValue replaces the numeric float32 value of the field.
// Panics if the field's current value is not numeric.
func (f *Field) SetFloatValue(value float32) {
	if !isNumeric(f.value) {
		panic(fmt.Sprintf("cannot change value type from %T to float32", f.value))
	}
	f.value = numericValue{n: value}
}

// SetDoubleValue replaces the numeric float64 value of the field.
// Panics if the field's current value is not numeric.
func (f *Field) SetDoubleValue(value float64) {
	if !isNumeric(f.value) {
		panic(fmt.Sprintf("cannot change value type from %T to float64", f.value))
	}
	f.value = numericValue{n: value}
}

// InvertableType returns the InvertableType describing how this field will
// be inverted during indexing. Mirrors Lucene's Field#invertableType()
// which always returns InvertableType.TOKEN_STREAM (subclasses may override).
//
// Mirrors Lucene 10.4.0 default behaviour.
func (f *Field) InvertableType() InvertableType {
	return InvertableTypeTokenStream
}

// GetCharSequenceValue returns the field's string value (CharSequence in
// Java); identical to StringValue() in the Gocene model — Go has no
// CharSequence equivalent so the two methods share an implementation.
func (f *Field) GetCharSequenceValue() string { return f.StringValue() }

// isNumeric reports whether v is one of the numeric wrappers used by Field.
func isNumeric(v fieldValue) bool {
	if v == nil {
		return true
	}
	_, ok := v.(numericValue)
	return ok
}
