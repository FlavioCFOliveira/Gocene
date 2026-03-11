// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"io"
)

// StoredField is a field that is stored but not indexed.
// The field value can be retrieved in search results but is not searchable.
// This is useful for storing metadata or large text that doesn't need to be searched.
//
// This is the Go port of Lucene's org.apache.lucene.document.StoredField.
type StoredField struct {
	*Field
}

var (
	// StoredFieldType is the FieldType for a stored-only field.
	// The value is stored but not indexed.
	StoredFieldType *FieldType
)

func init() {
	// Initialize the FieldType
	StoredFieldType = NewFieldType().
		SetIndexed(false).
		SetStored(true).
		SetTokenized(false)
	StoredFieldType.Freeze()
}

// NewStoredField creates a new StoredField with the given name and string value.
func NewStoredField(name string, value string) (*StoredField, error) {
	field, err := NewField(name, value, StoredFieldType)
	if err != nil {
		return nil, err
	}

	return &StoredField{Field: field}, nil
}

// NewStoredFieldFromBytes creates a new StoredField from a byte slice.
func NewStoredFieldFromBytes(name string, value []byte) (*StoredField, error) {
	field, err := NewField(name, value, StoredFieldType)
	if err != nil {
		return nil, err
	}

	return &StoredField{Field: field}, nil
}

// NewStoredFieldFromReader creates a new StoredField from an io.Reader.
func NewStoredFieldFromReader(name string, reader io.Reader) (*StoredField, error) {
	field, err := NewField(name, reader, StoredFieldType)
	if err != nil {
		return nil, err
	}

	return &StoredField{Field: field}, nil
}

// NewStoredFieldFromInt creates a new StoredField from an int value.
func NewStoredFieldFromInt(name string, value int) (*StoredField, error) {
	field, err := NewField(name, value, StoredFieldType)
	if err != nil {
		return nil, err
	}

	return &StoredField{Field: field}, nil
}

// NewStoredFieldFromInt64 creates a new StoredField from an int64 value.
func NewStoredFieldFromInt64(name string, value int64) (*StoredField, error) {
	field, err := NewField(name, value, StoredFieldType)
	if err != nil {
		return nil, err
	}

	return &StoredField{Field: field}, nil
}

// NewStoredFieldFromFloat64 creates a new StoredField from a float64 value.
func NewStoredFieldFromFloat64(name string, value float64) (*StoredField, error) {
	field, err := NewField(name, value, StoredFieldType)
	if err != nil {
		return nil, err
	}

	return &StoredField{Field: field}, nil
}
