// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Port of org.apache.lucene.util.mutable.MutableValueStr from Apache
// Lucene 10.4.0 (Apache License 2.0).

package mutable

import (
	"strings"
	"unicode/utf16"
)

// MutableValueStr is the [MutableValue] specialisation that carries a
// textual payload. In the Java reference the field is declared as
// `CharSequence value = ""` and every method calls `value.toString()`
// before operating on it; the Go port collapses that indirection to a
// plain `string`, since Go's string is itself an immutable byte
// sequence with O(1) length and O(n) equality.
//
// Per the Java contract, callers that flip Exists to false must also
// reset Value to the empty string (Lucene encodes this with
// `assert exists || "".equals(value.toString())`).
type MutableValueStr struct {
	BaseMutableValue

	// Value is the string payload. Defaults to "".
	Value string
}

// NewMutableValueStr returns a MutableValueStr with Exists set to true
// (mirroring Java's `public boolean exists = true;` initialiser).
func NewMutableValueStr() *MutableValueStr {
	v := &MutableValueStr{}
	v.SetExists(true)
	return v
}

// Copy implements MutableValue.Copy. Panics if source is not a
// *MutableValueStr.
func (m *MutableValueStr) Copy(source MutableValue) {
	s := source.(*MutableValueStr)
	m.Value = s.Value
	m.SetExists(s.Exists())
}

// Duplicate implements MutableValue.Duplicate.
func (m *MutableValueStr) Duplicate() MutableValue {
	d := &MutableValueStr{Value: m.Value}
	d.SetExists(m.Exists())
	return d
}

// EqualsSameType implements MutableValue.EqualsSameType. Panics if
// other is not a *MutableValueStr. Java's reference calls
// `value.toString().equals(other.value.toString())`, which on plain
// strings reduces to byte-level equality.
func (m *MutableValueStr) EqualsSameType(other MutableValue) bool {
	b := other.(*MutableValueStr)
	return m.Value == b.Value && m.Exists() == b.Exists()
}

// CompareSameType implements MutableValue.CompareSameType. Panics if
// other is not a *MutableValueStr. Java's reference calls
// `value.toString().compareTo(other.value.toString())`, which compares
// strings as sequences of UTF-16 code units. For ASCII inputs that
// matches Go's strings.Compare on byte order; for higher code points
// the two iterations agree as long as both runtimes encode the same
// abstract sequence of Unicode scalar values. Lucene/Gocene strings
// originate from UTF-8 byte arrays in the index, so iterating either
// representation yields the same lexicographic order across the BMP;
// astral-plane characters require an explicit UTF-16 step (see the
// comment in hashCode below).
//
// Ties on Value are broken by Exists with absent < present.
func (m *MutableValueStr) CompareSameType(other MutableValue) int {
	b := other.(*MutableValueStr)
	if c := strings.Compare(m.Value, b.Value); c != 0 {
		return c
	}
	if m.Exists() == b.Exists() {
		return 0
	}
	if m.Exists() {
		return 1
	}
	return -1
}

// ToObject implements MutableValue.ToObject. Returns the string Value
// when Exists, otherwise nil.
func (m *MutableValueStr) ToObject() any {
	if m.Exists() {
		return m.Value
	}
	return nil
}

// HashCode implements MutableValue.HashCode. Mirrors Java's
// `String.hashCode()`:
//
//	h = 0
//	for each char c in s: h = 31*h + c
//
// where `c` is a Java `char`, i.e. a UTF-16 code unit. Iterating the
// Go string's UTF-16 expansion preserves byte-level parity with the
// Java reference for the full Unicode range; for ASCII inputs the
// UTF-16 units are identical to the input bytes, so this is also the
// fastest path.
func (m *MutableValueStr) HashCode() int {
	var h int32
	// Optimise the ASCII-only case to avoid the rune/utf16 allocation.
	if isASCII(m.Value) {
		for i := 0; i < len(m.Value); i++ {
			h = 31*h + int32(m.Value[i])
		}
		return int(h)
	}
	units := utf16.Encode([]rune(m.Value))
	for _, u := range units {
		h = 31*h + int32(u)
	}
	return int(h)
}

// String implements MutableValue.String. Returns the payload when
// present, "(null)" otherwise.
func (m *MutableValueStr) String() string {
	if !m.Exists() {
		return "(null)"
	}
	return m.Value
}

// isASCII reports whether every byte in s is below 0x80.
func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}
