// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Port of org.apache.lucene.util.mutable.MutableValueBool from Apache
// Lucene 10.4.0 (Apache License 2.0).

package mutable

import "strconv"

// MutableValueBool is the [MutableValue] specialisation for booleans.
//
// Per the Java contract, callers that flip Exists to false must also
// reset Value to false; the Equals/Compare/Hash helpers assume that
// invariant (mirroring Lucene's `assert exists || (false == value)`).
type MutableValueBool struct {
	BaseMutableValue

	// Value carries the boolean payload. Defaults to false.
	Value bool
}

// NewMutableValueBool returns a MutableValueBool with Exists set to
// true (mirroring Java's `public boolean exists = true;` initialiser).
func NewMutableValueBool() *MutableValueBool {
	v := &MutableValueBool{}
	v.SetExists(true)
	return v
}

// Copy implements MutableValue.Copy. Panics if source is not a
// *MutableValueBool, matching Java's ClassCastException semantics.
func (m *MutableValueBool) Copy(source MutableValue) {
	s := source.(*MutableValueBool)
	m.Value = s.Value
	m.SetExists(s.Exists())
}

// Duplicate implements MutableValue.Duplicate.
func (m *MutableValueBool) Duplicate() MutableValue {
	d := &MutableValueBool{Value: m.Value}
	d.SetExists(m.Exists())
	return d
}

// EqualsSameType implements MutableValue.EqualsSameType. Panics if
// other is not a *MutableValueBool.
func (m *MutableValueBool) EqualsSameType(other MutableValue) bool {
	b := other.(*MutableValueBool)
	return m.Value == b.Value && m.Exists() == b.Exists()
}

// CompareSameType implements MutableValue.CompareSameType. Panics if
// other is not a *MutableValueBool. Mirrors the Java decision tree:
// false < true on the Value axis, then absent < present.
func (m *MutableValueBool) CompareSameType(other MutableValue) int {
	b := other.(*MutableValueBool)
	if m.Value != b.Value {
		if m.Value {
			return 1
		}
		return -1
	}
	if m.Exists() == b.Exists() {
		return 0
	}
	if m.Exists() {
		return 1
	}
	return -1
}

// ToObject implements MutableValue.ToObject. Returns the boolean Value
// when Exists, otherwise nil.
func (m *MutableValueBool) ToObject() any {
	if m.Exists() {
		return m.Value
	}
	return nil
}

// HashCode implements MutableValue.HashCode. Mirrors Java's branch
// (`value ? 2 : (exists ? 1 : 0)`).
func (m *MutableValueBool) HashCode() int {
	switch {
	case m.Value:
		return 2
	case m.Exists():
		return 1
	default:
		return 0
	}
}

// String implements MutableValue.String. Returns Java's
// `Boolean.toString(value)` when present, "(null)" otherwise.
func (m *MutableValueBool) String() string {
	if !m.Exists() {
		return "(null)"
	}
	return strconv.FormatBool(m.Value)
}
