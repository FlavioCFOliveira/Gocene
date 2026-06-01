// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// This file extends StoredField (defined in stored_field.go) with the
// Lucene 10.4.0 constructor overloads that were missing.

// StoredFieldTYPE is the Lucene-canonical alias for StoredFieldType.
// Populated lazily in init() to avoid Go's package-var init ordering trap
// (StoredFieldType itself is also set up in an init() in stored_field.go).
var StoredFieldTYPE *FieldType

func init() {
	StoredFieldTYPE = StoredFieldType
}

// NewStoredFieldFromBytesOffset creates a StoredField that references a
// window of the supplied byte slice [offset, offset+length).
// Mirrors Lucene's StoredField(String, byte[], int, int).
func NewStoredFieldFromBytesOffset(name string, value []byte, offset, length int) (*StoredField, error) {
	if offset < 0 || length < 0 || offset+length > len(value) {
		return nil, fmt.Errorf("invalid offset/length: offset=%d length=%d cap=%d", offset, length, len(value))
	}
	clone := make([]byte, length)
	copy(clone, value[offset:offset+length])
	return NewStoredFieldFromBytes(name, clone)
}

// NewStoredFieldFromFloat32 creates a StoredField from a float32 value.
// Mirrors Lucene's StoredField(String, float).
func NewStoredFieldFromFloat32(name string, value float32) (*StoredField, error) {
	field, err := NewField(name, value, StoredFieldType)
	if err != nil {
		return nil, err
	}
	return &StoredField{Field: field}, nil
}

// NewStoredFieldFromBytesWithType creates a StoredField from raw bytes with
// an explicit FieldType, mirroring Lucene's StoredField(String, BytesRef,
// FieldType) expert overload.
func NewStoredFieldFromBytesWithType(name string, value []byte, ft *FieldType) (*StoredField, error) {
	if ft == nil {
		return nil, fmt.Errorf("FieldType cannot be nil")
	}
	field, err := NewField(name, value, ft)
	if err != nil {
		return nil, err
	}
	return &StoredField{Field: field}, nil
}

// NewStoredFieldFromStringWithType creates a StoredField from a string with
// an explicit FieldType, mirroring Lucene's StoredField(String, String,
// FieldType) expert overload.
func NewStoredFieldFromStringWithType(name string, value string, ft *FieldType) (*StoredField, error) {
	if ft == nil {
		return nil, fmt.Errorf("FieldType cannot be nil")
	}
	field, err := NewField(name, value, ft)
	if err != nil {
		return nil, err
	}
	return &StoredField{Field: field}, nil
}

// StoredValue returns a StoredValue snapshot of this field's payload.
// Mirrors Lucene's Field#storedValue() narrowed for stored fields.
//
// The returned StoredValue's type reflects the underlying field's value:
//   - STRING for string payloads
//   - BINARY for byte payloads
//   - INTEGER/LONG/FLOAT/DOUBLE for numeric payloads (Go-typed)
//
// Returns nil if the field's value is of an unsupported variant.
func (s *StoredField) StoredValue() *StoredValue {
	switch v := s.Field.value.(type) {
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
			return NewStoredValueLong(int64(n))
		}
	}
	return nil
}

// NewStoredFieldFromDataInput creates a StoredField from a StoredFieldDataInput by
// materialising all bytes from the stream into a binary stored field.
// Mirrors Lucene's StoredField(String, StoredValue) with StoredValue(StoredFieldDataInput).
func NewStoredFieldFromDataInput(name string, dataInput *index.StoredFieldDataInput) (*StoredField, error) {
	if dataInput == nil {
		return nil, fmt.Errorf("StoredFieldDataInput cannot be nil")
	}
	if dataInput.In == nil {
		return nil, fmt.Errorf("StoredFieldDataInput.In cannot be nil")
	}
	buf := make([]byte, dataInput.Length)
	if err := dataInput.In.ReadBytes(buf); err != nil {
		return nil, fmt.Errorf("NewStoredFieldFromDataInput: read bytes: %w", err)
	}
	return NewStoredFieldFromBytes(name, buf)
}
