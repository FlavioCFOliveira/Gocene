// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// This file is part of the SPI unification work (rmp #4669 / phase 1.3,
// T4699). Term is the canonical declaration site in the leaf schema/
// package; index/ re-exports it via a Go type alias so historical callers
// that reach for index.Term continue to compile unchanged.
//
// Type aliases (type X = schema.X) make index.Term and schema.Term the
// same type at the type-system level, which means methods declared on
// *schema.Term, helper functions returning *schema.Term, and interface
// satisfaction all flow through without conversion.

import (
	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Term is an alias of schema.Term.
type Term = schema.Term

// NewTerm creates a new Term with the given field and text.
func NewTerm(field, text string) *Term {
	return schema.NewTerm(field, text)
}

// NewTermFromBytes creates a new Term with the given field and bytes.
func NewTermFromBytes(field string, bytes []byte) *Term {
	return schema.NewTermFromBytes(field, bytes)
}

// NewTermFromBytesRef creates a new Term with the given field and BytesRef.
func NewTermFromBytesRef(field string, bytesRef *util.BytesRef) *Term {
	return schema.NewTermFromBytesRef(field, bytesRef)
}

// TermCompare compares two terms.
func TermCompare(a, b *Term) int {
	return schema.TermCompare(a, b)
}

// TermEquals returns true if two terms are equal.
func TermEquals(a, b *Term) bool {
	return schema.TermEquals(a, b)
}

// TermBytesEquals returns true if two terms have equal bytes (ignoring field).
func TermBytesEquals(a, b *Term) bool {
	return schema.TermBytesEquals(a, b)
}
