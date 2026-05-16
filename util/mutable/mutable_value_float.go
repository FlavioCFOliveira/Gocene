// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Port of org.apache.lucene.util.mutable.MutableValueFloat from Apache
// Lucene 10.4.0 (Apache License 2.0).

package mutable

import (
	"math"
	"strconv"
)

// MutableValueFloat is the [MutableValue] specialisation for the
// 32-bit IEEE-754 binary floating-point payload Lucene maps to Java's
// primitive `float`. The Go field uses float32 to preserve byte-level
// parity with the Java reference.
//
// Per the Java contract, callers that flip Exists to false must also
// reset Value to 0.0 (Lucene encodes this with
// `assert exists || 0.0f == value`).
type MutableValueFloat struct {
	BaseMutableValue

	// Value is the 32-bit floating-point payload.
	Value float32
}

// NewMutableValueFloat returns a MutableValueFloat with Exists set to
// true, matching Java's `public boolean exists = true;` initialiser.
func NewMutableValueFloat() *MutableValueFloat {
	v := &MutableValueFloat{}
	v.SetExists(true)
	return v
}

// Copy implements MutableValue.Copy. Panics if source is not a
// *MutableValueFloat.
func (m *MutableValueFloat) Copy(source MutableValue) {
	s := source.(*MutableValueFloat)
	m.Value = s.Value
	m.SetExists(s.Exists())
}

// Duplicate implements MutableValue.Duplicate.
func (m *MutableValueFloat) Duplicate() MutableValue {
	d := &MutableValueFloat{Value: m.Value}
	d.SetExists(m.Exists())
	return d
}

// EqualsSameType implements MutableValue.EqualsSameType. Panics if
// other is not a *MutableValueFloat. NaN != NaN matches Java's `==`
// semantics used by Lucene's reference.
func (m *MutableValueFloat) EqualsSameType(other MutableValue) bool {
	b := other.(*MutableValueFloat)
	return m.Value == b.Value && m.Exists() == b.Exists()
}

// CompareSameType implements MutableValue.CompareSameType. Panics if
// other is not a *MutableValueFloat. Uses Java's Float.compare
// semantics: NaN > +Inf and -0.0 < +0.0 (bit-level ordering when the
// numeric comparison reports equality). Ties on Value are broken by
// Exists with absent < present.
func (m *MutableValueFloat) CompareSameType(other MutableValue) int {
	b := other.(*MutableValueFloat)
	c := compareFloat32(m.Value, b.Value)
	if c != 0 {
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

// ToObject implements MutableValue.ToObject. Returns the float32 Value
// when Exists, otherwise nil.
func (m *MutableValueFloat) ToObject() any {
	if m.Exists() {
		return m.Value
	}
	return nil
}

// HashCode implements MutableValue.HashCode. Mirrors Java exactly:
// `Float.floatToIntBits(value)`. math.Float32bits returns the raw
// uint32 bit pattern; the cast to int32 then int reproduces Java's
// signed `int` width.
func (m *MutableValueFloat) HashCode() int {
	return int(int32(math.Float32bits(m.Value)))
}

// String implements MutableValue.String. Returns `Float.toString` when
// present, "(null)" otherwise. Go's `strconv.FormatFloat(_, 'g', -1, 32)`
// produces the shortest round-trippable representation for a float32
// value, agreeing with Java's algorithm for the values where both
// platforms emit a minimal representation.
func (m *MutableValueFloat) String() string {
	if !m.Exists() {
		return "(null)"
	}
	return strconv.FormatFloat(float64(m.Value), 'g', -1, 32)
}

// compareFloat32 implements Java's `Float.compare(f1, f2)`:
//
//   - if f1 < f2 the result is negative;
//   - if f1 > f2 the result is positive;
//   - otherwise the result is the signed comparison of
//     floatToIntBits(f1) and floatToIntBits(f2), which sorts NaN above
//     +Inf and -0.0 below +0.0.
func compareFloat32(a, b float32) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	ba := int32(math.Float32bits(a))
	bb := int32(math.Float32bits(b))
	switch {
	case ba < bb:
		return -1
	case ba > bb:
		return 1
	default:
		return 0
	}
}
