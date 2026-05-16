// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Port of org.apache.lucene.util.mutable.MutableValueLong from Apache
// Lucene 10.4.0 (Apache License 2.0).

package mutable

import "strconv"

// MutableValueLong is the [MutableValue] specialisation for the
// 64-bit signed integer payload Lucene maps to Java's primitive
// `long`. The Go field uses int64 to preserve byte-level parity with
// the Java reference.
//
// Per the Java contract, callers that flip Exists to false must also
// reset Value to 0 (Lucene encodes this with
// `assert exists || 0L == value`).
//
// MutableValueDate embeds this type and overrides Duplicate/ToObject;
// EqualsSameType and CompareSameType therefore remain type-strict
// (a Long and a Date never compare equal even when they share the
// same payload, mirroring Java's getClass() === check).
type MutableValueLong struct {
	BaseMutableValue

	// Value is the 64-bit signed integer payload.
	Value int64
}

// NewMutableValueLong returns a MutableValueLong with Exists set to
// true, matching Java's `public boolean exists = true;` initialiser.
func NewMutableValueLong() *MutableValueLong {
	v := &MutableValueLong{}
	v.SetExists(true)
	return v
}

// Copy implements MutableValue.Copy. Panics if source is not a
// *MutableValueLong.
func (m *MutableValueLong) Copy(source MutableValue) {
	s := source.(*MutableValueLong)
	m.SetExists(s.Exists())
	m.Value = s.Value
}

// Duplicate implements MutableValue.Duplicate.
func (m *MutableValueLong) Duplicate() MutableValue {
	d := &MutableValueLong{Value: m.Value}
	d.SetExists(m.Exists())
	return d
}

// EqualsSameType implements MutableValue.EqualsSameType. Panics if
// other is not a *MutableValueLong.
func (m *MutableValueLong) EqualsSameType(other MutableValue) bool {
	b := other.(*MutableValueLong)
	return m.Value == b.Value && m.Exists() == b.Exists()
}

// CompareSameType implements MutableValue.CompareSameType. Panics if
// other is not a *MutableValueLong.
func (m *MutableValueLong) CompareSameType(other MutableValue) int {
	b := other.(*MutableValueLong)
	if m.Value < b.Value {
		return -1
	}
	if m.Value > b.Value {
		return 1
	}
	if m.Exists() == b.Exists() {
		return 0
	}
	if m.Exists() {
		return 1
	}
	return -1
}

// ToObject implements MutableValue.ToObject. Returns the int64 Value
// when Exists, otherwise nil.
func (m *MutableValueLong) ToObject() any {
	if m.Exists() {
		return m.Value
	}
	return nil
}

// HashCode implements MutableValue.HashCode. Mirrors Java exactly:
// `(int) value + (int) (value >> 32)`. The shift is arithmetic
// (signed) in Java; using int64 in Go preserves sign extension. The
// (int) casts in Java truncate to 32 bits, which here is a uint32
// modular reduction folded back to int.
func (m *MutableValueLong) HashCode() int {
	lo := int32(m.Value)
	hi := int32(m.Value >> 32)
	return int(lo + hi)
}

// String implements MutableValue.String. Returns Java's
// `Long.toString(value)` when present, "(null)" otherwise.
func (m *MutableValueLong) String() string {
	if !m.Exists() {
		return "(null)"
	}
	return strconv.FormatInt(m.Value, 10)
}
