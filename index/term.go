// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"bytes"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// Term represents the smallest unit of indexing and searching.
// It consists of a field name and the text value of that field.
//
// This is the Go port of Lucene's org.apache.lucene.index.Term.
type Term struct {
	// Field is the name of the field this term belongs to.
	Field string

	// Bytes is the text value of the term.
	// This is a reference to the actual bytes, which may be shared.
	Bytes *util.BytesRef
}

// NewTerm creates a new Term with the given field and text.
// The text is converted to bytes using UTF-8 encoding.
func NewTerm(field, text string) *Term {
	return &Term{
		Field: field,
		Bytes: util.NewBytesRef([]byte(text)),
	}
}

// NewTermFromBytes creates a new Term with the given field and bytes.
// The bytes are copied to ensure the Term owns its own copy.
func NewTermFromBytes(field string, bytes []byte) *Term {
	return &Term{
		Field: field,
		Bytes: util.NewBytesRef(bytes),
	}
}

// NewTermFromBytesRef creates a new Term with the given field and BytesRef.
// The BytesRef is cloned to ensure the Term owns its own copy.
func NewTermFromBytesRef(field string, bytesRef *util.BytesRef) *Term {
	return &Term{
		Field: field,
		Bytes: bytesRef.Clone(),
	}
}

// Text returns the term text as a string.
func (t *Term) Text() string {
	if t.Bytes == nil {
		return ""
	}
	return t.Bytes.String()
}

// BytesValue returns the term bytes.
func (t *Term) BytesValue() *util.BytesRef {
	return t.Bytes
}

// Clone creates a copy of this Term with its own copy of the bytes.
func (t *Term) Clone() *Term {
	if t == nil {
		return nil
	}
	return &Term{
		Field: t.Field,
		Bytes: t.Bytes.Clone(),
	}
}

// Equals returns true if this term equals another term.
// Two terms are equal if they have the same field name and the same bytes.
func (t *Term) Equals(other *Term) bool {
	if t == other {
		return true
	}
	if t == nil || other == nil {
		return false
	}
	if t.Field != other.Field {
		return false
	}
	return util.BytesRefEquals(t.Bytes, other.Bytes)
}

// CompareTo compares this term with another term.
// First compares by field name, then by bytes.
// Returns:
//
//	-1 if this term is less than other
//	 0 if this term equals other
//	 1 if this term is greater than other
func (t *Term) CompareTo(other *Term) int {
	if t == other {
		return 0
	}
	if t == nil {
		return -1
	}
	if other == nil {
		return 1
	}

	// First compare by field
	if t.Field < other.Field {
		return -1
	}
	if t.Field > other.Field {
		return 1
	}

	// Fields are equal, compare by bytes
	return util.BytesRefCompare(t.Bytes, other.Bytes)
}

// HashCode returns a hash code for this term.
// Combines the hash of the field name and the bytes.
func (t *Term) HashCode() int {
	if t == nil {
		return 0
	}
	h := 0
	// Hash the field name
	for i := 0; i < len(t.Field); i++ {
		h = 31*h + int(t.Field[i])
	}
	// Combine with bytes hash
	if t.Bytes != nil {
		h = 31*h + t.Bytes.HashCode()
	}
	return h
}

// String returns a string representation of this term.
func (t *Term) String() string {
	if t == nil {
		return "nil"
	}
	return fmt.Sprintf("%s:%s", t.Field, t.Text())
}

// SetBytesRef sets the bytes of this term.
// This creates a copy of the provided bytes.
func (t *Term) SetBytesRef(bytes *util.BytesRef) {
	if bytes == nil {
		t.Bytes = nil
		return
	}
	t.Bytes = bytes.Clone()
}

// SetBytes sets the bytes of this term.
// This creates a copy of the provided bytes.
// If bytes is nil, the Bytes field will be set to nil.
func (t *Term) SetBytes(bytes []byte) {
	if bytes == nil {
		t.Bytes = nil
		return
	}
	t.Bytes = util.NewBytesRef(bytes)
}

// TermCompare compares two terms.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func TermCompare(a, b *Term) int {
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return -1
	}
	return a.CompareTo(b)
}

// TermEquals returns true if two terms are equal.
func TermEquals(a, b *Term) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Equals(b)
}

// TermBytesEquals returns true if two terms have equal bytes (ignoring field).
// This is useful for comparing term values within the same field.
func TermBytesEquals(a, b *Term) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return util.BytesRefEquals(a.Bytes, b.Bytes)
}

// GetBytesRef returns a BytesRef representation of the term's text.
// If the term's Bytes is nil, returns an empty BytesRef.
func (t *Term) GetBytesRef() *util.BytesRef {
	if t.Bytes == nil {
		return util.NewBytesRefEmpty()
	}
	return t.Bytes
}

// StartsWith returns true if this term's bytes start with the given prefix.
func (t *Term) StartsWith(prefix []byte) bool {
	if t.Bytes == nil || t.Bytes.Length < len(prefix) {
		return false
	}
	return bytes.Equal(t.Bytes.ValidBytes()[:len(prefix)], prefix)
}

// StartsWithTerm returns true if this term's bytes start with another term's bytes.
func (t *Term) StartsWithTerm(other *Term) bool {
	if other == nil || other.Bytes == nil {
		return true
	}
	if t.Bytes == nil {
		return other.Bytes.Length == 0
	}
	return t.StartsWith(other.Bytes.ValidBytes())
}
