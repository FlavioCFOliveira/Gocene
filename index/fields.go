// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// Fields, FieldIterator, and the canonical in-memory / single-field /
// multi-field implementations live in the leaf schema/ package as of
// rmp #4669 / phase 1.3 (T4699). This file aliases the historical
// index.* names so existing callers compile unchanged.

import (
	"github.com/FlavioCFOliveira/Gocene/schema"
)

// Fields is an alias of schema.Fields.
type Fields = schema.Fields

// FieldIterator is an alias of schema.FieldIterator.
type FieldIterator = schema.FieldIterator

// FieldsBase is an alias of schema.FieldsBase.
type FieldsBase = schema.FieldsBase

// EmptyFields is an alias of schema.EmptyFields.
type EmptyFields = schema.EmptyFields

// EmptyFieldIterator is an alias of schema.EmptyFieldIterator.
type EmptyFieldIterator = schema.EmptyFieldIterator

// MemoryFields is an alias of schema.MemoryFields.
type MemoryFields = schema.MemoryFields

// MemoryFieldIterator is an alias of schema.MemoryFieldIterator.
type MemoryFieldIterator = schema.MemoryFieldIterator

// SingleFieldFields is an alias of schema.SingleFieldFields.
type SingleFieldFields = schema.SingleFieldFields

// SingleFieldIterator is an alias of schema.SingleFieldIterator.
type SingleFieldIterator = schema.SingleFieldIterator

// MultiFields is an alias of schema.MultiFields.
type MultiFields = schema.MultiFields

// FieldsStats is an alias of schema.FieldsStats.
type FieldsStats = schema.FieldsStats

// NewMemoryFields creates a new empty MemoryFields.
func NewMemoryFields() *MemoryFields {
	return schema.NewMemoryFields()
}

// NewMemoryFieldIterator builds a MemoryFieldIterator over names.
func NewMemoryFieldIterator(names []string) *MemoryFieldIterator {
	return schema.NewMemoryFieldIterator(names)
}

// NewSingleFieldFields creates a new SingleFieldFields.
func NewSingleFieldFields(field string, terms Terms) *SingleFieldFields {
	return schema.NewSingleFieldFields(field, terms)
}

// NewMultiFields creates a new MultiFields from a list of Fields.
func NewMultiFields(fields ...Fields) *MultiFields {
	return schema.NewMultiFields(fields...)
}
