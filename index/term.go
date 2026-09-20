// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// This file is part of the SPI unification work (rmp #4669 / phase 1.3,
// T4699). Term is the canonical declaration site in the leaf schema/
// package; index/ re-exports it via a Go type alias so historical callers
// that reach for index.Term continue to compile unchanged.
//
// Type aliases (type X = spi.X) make index.Term and spi.Term the
// same type at the type-system level, which means methods declared on
// *spi.Term, helper functions returning *spi.Term, and interface
// satisfaction all flow through without conversion.

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Term is an alias of spi.Term.
type Term = spi.Term

// NewTerm creates a new Term with the given field and text.
func NewTerm(field, text string) *Term {
	return spi.NewTerm(field, text)
}

// NewTermFromBytes creates a new Term with the given field and bytes.
func NewTermFromBytes(field string, bytes []byte) *Term {
	return spi.NewTermFromBytes(field, bytes)
}

// NewTermFromBytesRef creates a new Term with the given field and BytesRef.
func NewTermFromBytesRef(field string, bytesRef *util.BytesRef) *Term {
	return spi.NewTermFromBytesRef(field, bytesRef)
}

// TermCompare compares two terms.
func TermCompare(a, b *Term) int {
	return spi.TermCompare(a, b)
}

// TermEquals returns true if two terms are equal.
func TermEquals(a, b *Term) bool {
	return spi.TermEquals(a, b)
}

// TermBytesEquals returns true if two terms have equal bytes (ignoring field).
func TermBytesEquals(a, b *Term) bool {
	return spi.TermBytesEquals(a, b)
}
