// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// Fields, FieldIterator, and the canonical in-memory / single-field /
// multi-field implementations live in the leaf schema/ package as of
// rmp #4669 / phase 1.3 (T4699). This file aliases the historical
// index.* names so existing callers compile unchanged.

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// Fields is an alias of spi.Fields.
type Fields = spi.Fields

// FieldIterator is an alias of spi.FieldIterator.
type FieldIterator = spi.FieldIterator

// FieldsBase is an alias of spi.FieldsBase.
type FieldsBase = spi.FieldsBase

// EmptyFields is an alias of spi.EmptyFields.
type EmptyFields = spi.EmptyFields

// EmptyFieldIterator is an alias of spi.EmptyFieldIterator.
type EmptyFieldIterator = spi.EmptyFieldIterator

// MemoryFields is an alias of spi.MemoryFields.
type MemoryFields = spi.MemoryFields

// MemoryFieldIterator is an alias of spi.MemoryFieldIterator.
type MemoryFieldIterator = spi.MemoryFieldIterator

// SingleFieldFields is an alias of spi.SingleFieldFields.
type SingleFieldFields = spi.SingleFieldFields

// SingleFieldIterator is an alias of spi.SingleFieldIterator.
type SingleFieldIterator = spi.SingleFieldIterator


// FieldsStats is an alias of spi.FieldsStats.
type FieldsStats = spi.FieldsStats

// NewMemoryFields creates a new empty MemoryFields.
func NewMemoryFields() *MemoryFields {
	return spi.NewMemoryFields()
}

// NewMemoryFieldIterator builds a MemoryFieldIterator over names.
func NewMemoryFieldIterator(names []string) *MemoryFieldIterator {
	return spi.NewMemoryFieldIterator(names)
}

// NewSingleFieldFields creates a new SingleFieldFields.
func NewSingleFieldFields(field string, terms Terms) *SingleFieldFields {
	return spi.NewSingleFieldFields(field, terms)
}

