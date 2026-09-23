// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// StringField is a field for non-tokenized, indexed string values.
// The string value is indexed as a single term (not tokenized),
// making it suitable for exact match searches, filtering, and sorting.
//
// This is the Go port of Lucene's org.apache.lucene.document.StringField.
type StringField struct {
	*Field

	// binaryValue and storedValue mirror the private fields of Lucene's
	// StringField: the indexed term bytes and the stored value (nil when the
	// field is not stored).
	binaryValue []byte
	storedValue *StoredValue
}

var (
	// StringFieldTypeStored is the FieldType for a stored StringField.
	// The string is indexed as a single term and stored.
	StringFieldTypeStored *FieldType

	// StringFieldTypeNotStored is the FieldType for a non-stored StringField.
	// The string is indexed as a single term but not stored.
	StringFieldTypeNotStored *FieldType

	// StringFieldTYPESTORED is the Lucene-canonical alias for
	// StringFieldTypeStored (mirrors public static final FieldType TYPE_STORED).
	StringFieldTYPESTORED *FieldType

	// StringFieldTYPENOTSTORED is the Lucene-canonical alias for
	// StringFieldTypeNotStored (mirrors public static final FieldType TYPE_NOT_STORED).
	StringFieldTYPENOTSTORED *FieldType
)

func init() {
	// Initialize the FieldTypes
	StringFieldTypeStored = NewFieldType().
		SetIndexed(true).
		SetStored(true).
		SetTokenized(false).
		SetOmitNorms(true).
		SetIndexOptions(spi.IndexOptionsDocs)
	StringFieldTypeStored.Freeze()

	StringFieldTypeNotStored = NewFieldType().
		SetIndexed(true).
		SetStored(false).
		SetTokenized(false).
		SetOmitNorms(true).
		SetIndexOptions(spi.IndexOptionsDocs)
	StringFieldTypeNotStored.Freeze()

	StringFieldTYPESTORED = StringFieldTypeStored
	StringFieldTYPENOTSTORED = StringFieldTypeNotStored
}

// NewStringFieldFromBytesRef mirrors Lucene's
// StringField(String, BytesRef, Store) constructor. The Gocene version uses
// a []byte instead of a BytesRef.
func NewStringFieldFromBytesRef(name string, value []byte, stored bool) (*StringField, error) {
	return NewStringFieldFromBytes(name, value, stored)
}

// NewStringField creates a new StringField with the given name and value.
// If stored is true, the field value will be stored in the index.
func NewStringField(name string, value string, stored bool) (*StringField, error) {
	ft := StringFieldTypeNotStored
	if stored {
		ft = StringFieldTypeStored
	}

	field, err := NewField(name, value, ft)
	if err != nil {
		return nil, err
	}

	// Java: binaryValue = new BytesRef(value); storedValue = new StoredValue(value) when stored.
	f := &StringField{Field: field, binaryValue: []byte(value)}
	if stored {
		f.storedValue = NewStoredValueString(value)
	}
	return f, nil
}

// NewStringFieldFromBytes creates a new StringField from a byte slice.
// If stored is true, the field value will be stored in the index.
func NewStringFieldFromBytes(name string, value []byte, stored bool) (*StringField, error) {
	ft := StringFieldTypeNotStored
	if stored {
		ft = StringFieldTypeStored
	}

	field, err := NewField(name, value, ft)
	if err != nil {
		return nil, err
	}

	// Java: binaryValue = value; storedValue = new StoredValue(value) when stored.
	f := &StringField{Field: field, binaryValue: value}
	if stored {
		f.storedValue = NewStoredValueBinary(value)
	}
	return f, nil
}

// InvertableType mirrors StringField.invertableType(): the value is indexed
// as a single BINARY term.
func (f *StringField) InvertableType() InvertableType {
	return InvertableTypeBinary
}

// BinaryValue mirrors StringField.binaryValue(): the term bytes, for a
// String value as well as for a BytesRef value.
func (f *StringField) BinaryValue() []byte {
	return f.binaryValue
}

// SetStringValue mirrors StringField.setStringValue(String).
func (f *StringField) SetStringValue(value string) {
	f.Field.SetStringValue(value)
	f.binaryValue = []byte(value)
	if f.storedValue != nil {
		f.storedValue.SetStringValue(value)
	}
}

// SetBytesValue mirrors StringField.setBytesValue(BytesRef).
func (f *StringField) SetBytesValue(value []byte) {
	f.Field.SetBytesValue(value)
	f.binaryValue = value
	if f.storedValue != nil {
		f.storedValue.SetBinaryValue(value)
	}
}

// StoredValue mirrors StringField.storedValue().
func (f *StringField) StoredValue() *StoredValue {
	return f.storedValue
}
