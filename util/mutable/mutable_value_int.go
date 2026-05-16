// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Port of org.apache.lucene.util.mutable.MutableValueInt from Apache
// Lucene 10.4.0 (Apache License 2.0).

package mutable

import "strconv"

// MutableValueInt is the [MutableValue] specialisation for the
// 32-bit signed integer payload Lucene maps to Java's primitive
// `int`. The Go field uses int32 to preserve byte-level parity with
// the Java reference; callers that treat the field as a host-size int
// must cast explicitly.
//
// Per the Java contract, callers that flip Exists to false must also
// reset Value to 0 (Lucene encodes this with
// `assert exists || 0 == value`).
type MutableValueInt struct {
	BaseMutableValue

	// Value is the 32-bit signed integer payload.
	Value int32
}

// NewMutableValueInt returns a MutableValueInt with Exists set to
// true, matching Java's `public boolean exists = true;` initialiser.
func NewMutableValueInt() *MutableValueInt {
	v := &MutableValueInt{}
	v.SetExists(true)
	return v
}

// Copy implements MutableValue.Copy. Panics if source is not a
// *MutableValueInt.
func (m *MutableValueInt) Copy(source MutableValue) {
	s := source.(*MutableValueInt)
	m.Value = s.Value
	m.SetExists(s.Exists())
}

// Duplicate implements MutableValue.Duplicate.
func (m *MutableValueInt) Duplicate() MutableValue {
	d := &MutableValueInt{Value: m.Value}
	d.SetExists(m.Exists())
	return d
}

// EqualsSameType implements MutableValue.EqualsSameType. Panics if
// other is not a *MutableValueInt.
func (m *MutableValueInt) EqualsSameType(other MutableValue) bool {
	b := other.(*MutableValueInt)
	return m.Value == b.Value && m.Exists() == b.Exists()
}

// CompareSameType implements MutableValue.CompareSameType. Panics if
// other is not a *MutableValueInt. Mirrors the Java algorithm: order
// by Value first (signed), break ties by Exists (absent < present).
func (m *MutableValueInt) CompareSameType(other MutableValue) int {
	b := other.(*MutableValueInt)
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

// ToObject implements MutableValue.ToObject. Returns the int32 Value
// when Exists, otherwise nil.
func (m *MutableValueInt) ToObject() any {
	if m.Exists() {
		return m.Value
	}
	return nil
}

// HashCode implements MutableValue.HashCode. Mirrors Java exactly:
// `(value >> 8) + (value >> 16)` using a signed 32-bit arithmetic
// shift, which is `int32 >> n` in Go.
func (m *MutableValueInt) HashCode() int {
	return int((m.Value >> 8) + (m.Value >> 16))
}

// String implements MutableValue.String. Returns Java's
// `Integer.toString(value)` when present, "(null)" otherwise.
func (m *MutableValueInt) String() string {
	if !m.Exists() {
		return "(null)"
	}
	return strconv.FormatInt(int64(m.Value), 10)
}
