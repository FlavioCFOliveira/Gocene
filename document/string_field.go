// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// StringField is a field for non-tokenized, indexed string values.
// The string value is indexed as a single term (not tokenized),
// making it suitable for exact match searches, filtering, and sorting.
//
// This is the Go port of Lucene's org.apache.lucene.document.StringField.
type StringField struct {
	*Field
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
		SetIndexOptions(index.IndexOptionsDocs)
	StringFieldTypeStored.Freeze()

	StringFieldTypeNotStored = NewFieldType().
		SetIndexed(true).
		SetStored(false).
		SetTokenized(false).
		SetOmitNorms(true).
		SetIndexOptions(index.IndexOptionsDocs)
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

	return &StringField{Field: field}, nil
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

	return &StringField{Field: field}, nil
}
