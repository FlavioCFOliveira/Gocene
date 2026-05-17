// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import "fmt"

// StoredValueType discriminates the variant carried by a StoredValue.
// Mirrors Lucene 10.4.0's StoredValue.Type enum.
type StoredValueType int

const (
	// StoredValueTypeInteger is a 32-bit signed integer.
	StoredValueTypeInteger StoredValueType = iota

	// StoredValueTypeLong is a 64-bit signed integer.
	StoredValueTypeLong

	// StoredValueTypeFloat is a 32-bit floating-point value.
	StoredValueTypeFloat

	// StoredValueTypeDouble is a 64-bit floating-point value.
	StoredValueTypeDouble

	// StoredValueTypeBinary is a raw byte sequence (BytesRef in Java).
	StoredValueTypeBinary

	// StoredValueTypeDataInput is a streamed value backed by a
	// StoredFieldDataInput in Lucene. Deferred in Gocene — backed by a
	// io.Reader in a later sprint when StoredFieldDataInput is ported.
	StoredValueTypeDataInput

	// StoredValueTypeString is a UTF-8 string.
	StoredValueTypeString
)

// String returns the canonical Lucene name.
func (t StoredValueType) String() string {
	switch t {
	case StoredValueTypeInteger:
		return "INTEGER"
	case StoredValueTypeLong:
		return "LONG"
	case StoredValueTypeFloat:
		return "FLOAT"
	case StoredValueTypeDouble:
		return "DOUBLE"
	case StoredValueTypeBinary:
		return "BINARY"
	case StoredValueTypeDataInput:
		return "DATA_INPUT"
	case StoredValueTypeString:
		return "STRING"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", int(t))
	}
}

// StoredValue holds the value of a stored field. This is the Go port of
// Lucene 10.4.0's org.apache.lucene.document.StoredValue (a tagged union
// holding one of: int32, int64, float32, float64, []byte, or string).
//
// Deviation: the DATA_INPUT variant is deferred until
// StoredFieldDataInput is ported; constructors and getters for that
// variant return errors when used.
type StoredValue struct {
	kind StoredValueType
	i32  int32
	i64  int64
	f32  float32
	f64  float64
	str  string
	bin  []byte
}

// NewStoredValueInt creates a StoredValue carrying an int32 value.
func NewStoredValueInt(v int32) *StoredValue {
	return &StoredValue{kind: StoredValueTypeInteger, i32: v}
}

// NewStoredValueLong creates a StoredValue carrying an int64 value.
func NewStoredValueLong(v int64) *StoredValue {
	return &StoredValue{kind: StoredValueTypeLong, i64: v}
}

// NewStoredValueFloat creates a StoredValue carrying a float32 value.
func NewStoredValueFloat(v float32) *StoredValue {
	return &StoredValue{kind: StoredValueTypeFloat, f32: v}
}

// NewStoredValueDouble creates a StoredValue carrying a float64 value.
func NewStoredValueDouble(v float64) *StoredValue {
	return &StoredValue{kind: StoredValueTypeDouble, f64: v}
}

// NewStoredValueBinary creates a StoredValue carrying a byte slice.
// The slice is borrowed, not copied — mirrors Lucene's BytesRef contract.
// Panics if v is nil to match Java's NullPointerException behaviour.
func NewStoredValueBinary(v []byte) *StoredValue {
	if v == nil {
		panic("binary value cannot be nil")
	}
	return &StoredValue{kind: StoredValueTypeBinary, bin: v}
}

// NewStoredValueString creates a StoredValue carrying a string.
// Panics if v is empty-init nil (Java NullPointerException) — Go strings
// cannot be nil, so this is a no-op guard preserved for parity.
func NewStoredValueString(v string) *StoredValue {
	return &StoredValue{kind: StoredValueTypeString, str: v}
}

// GetType returns the discriminator describing this StoredValue's payload.
func (s *StoredValue) GetType() StoredValueType { return s.kind }

// GetIntValue returns the int32 payload. Panics if the StoredValue does
// not hold an INTEGER. Mirrors Lucene's IllegalArgumentException.
func (s *StoredValue) GetIntValue() int32 {
	s.expect(StoredValueTypeInteger)
	return s.i32
}

// GetLongValue returns the int64 payload. Panics if the StoredValue does
// not hold a LONG.
func (s *StoredValue) GetLongValue() int64 {
	s.expect(StoredValueTypeLong)
	return s.i64
}

// GetFloatValue returns the float32 payload. Panics if the StoredValue does
// not hold a FLOAT.
func (s *StoredValue) GetFloatValue() float32 {
	s.expect(StoredValueTypeFloat)
	return s.f32
}

// GetDoubleValue returns the float64 payload. Panics if the StoredValue does
// not hold a DOUBLE.
func (s *StoredValue) GetDoubleValue() float64 {
	s.expect(StoredValueTypeDouble)
	return s.f64
}

// GetBinaryValue returns the binary payload. Panics if the StoredValue does
// not hold a BINARY.
func (s *StoredValue) GetBinaryValue() []byte {
	s.expect(StoredValueTypeBinary)
	return s.bin
}

// GetStringValue returns the string payload. Panics if the StoredValue does
// not hold a STRING.
func (s *StoredValue) GetStringValue() string {
	s.expect(StoredValueTypeString)
	return s.str
}

// SetIntValue replaces the int32 payload. Panics if the StoredValue is not
// of type INTEGER.
func (s *StoredValue) SetIntValue(v int32) {
	s.expect(StoredValueTypeInteger)
	s.i32 = v
}

// SetLongValue replaces the int64 payload. Panics if not of type LONG.
func (s *StoredValue) SetLongValue(v int64) {
	s.expect(StoredValueTypeLong)
	s.i64 = v
}

// SetFloatValue replaces the float32 payload. Panics if not of type FLOAT.
func (s *StoredValue) SetFloatValue(v float32) {
	s.expect(StoredValueTypeFloat)
	s.f32 = v
}

// SetDoubleValue replaces the float64 payload. Panics if not of type DOUBLE.
func (s *StoredValue) SetDoubleValue(v float64) {
	s.expect(StoredValueTypeDouble)
	s.f64 = v
}

// SetBinaryValue replaces the binary payload. Panics if not of type BINARY
// or if v is nil.
func (s *StoredValue) SetBinaryValue(v []byte) {
	s.expect(StoredValueTypeBinary)
	if v == nil {
		panic("binary value cannot be nil")
	}
	s.bin = v
}

// SetStringValue replaces the string payload. Panics if not of type STRING.
func (s *StoredValue) SetStringValue(v string) {
	s.expect(StoredValueTypeString)
	s.str = v
}

func (s *StoredValue) expect(want StoredValueType) {
	if s.kind != want {
		panic(fmt.Sprintf("StoredValue is of type %s, cannot be accessed as %s", s.kind, want))
	}
}
