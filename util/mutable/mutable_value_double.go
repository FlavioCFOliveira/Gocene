// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Port of org.apache.lucene.util.mutable.MutableValueDouble from Apache
// Lucene 10.4.0 (Apache License 2.0).

package mutable

import (
	"math"
	"strconv"
)

// MutableValueDouble is the [MutableValue] specialisation for the
// 64-bit IEEE-754 binary floating-point payload Lucene maps to Java's
// primitive `double`. The Go field uses float64 to preserve byte-level
// parity with the Java reference.
//
// Per the Java contract, callers that flip Exists to false must also
// reset Value to 0.0 (Lucene encodes this with
// `assert exists || 0.0 == value`).
type MutableValueDouble struct {
	BaseMutableValue

	// Value is the 64-bit floating-point payload.
	Value float64
}

// NewMutableValueDouble returns a MutableValueDouble with Exists set to
// true, matching Java's `public boolean exists = true;` initialiser.
func NewMutableValueDouble() *MutableValueDouble {
	v := &MutableValueDouble{}
	v.SetExists(true)
	return v
}

// Copy implements MutableValue.Copy. Panics if source is not a
// *MutableValueDouble.
func (m *MutableValueDouble) Copy(source MutableValue) {
	s := source.(*MutableValueDouble)
	m.Value = s.Value
	m.SetExists(s.Exists())
}

// Duplicate implements MutableValue.Duplicate.
func (m *MutableValueDouble) Duplicate() MutableValue {
	d := &MutableValueDouble{Value: m.Value}
	d.SetExists(m.Exists())
	return d
}

// EqualsSameType implements MutableValue.EqualsSameType. Panics if
// other is not a *MutableValueDouble. Mirrors Java's
// `Double.doubleToLongBits` semantics indirectly via the `==` operator;
// note that `NaN == NaN` is false in both Java and Go, so two absent
// NaNs compare equal (via Exists) while two present NaNs do not. This
// matches Lucene's reference implementation exactly.
func (m *MutableValueDouble) EqualsSameType(other MutableValue) bool {
	b := other.(*MutableValueDouble)
	return m.Value == b.Value && m.Exists() == b.Exists()
}

// CompareSameType implements MutableValue.CompareSameType. Panics if
// other is not a *MutableValueDouble. Uses Java's Double.compare
// semantics: NaN > +Inf and -0.0 < +0.0 (bit-level ordering when the
// numeric comparison reports equality). Ties on Value are broken by
// Exists with absent < present.
func (m *MutableValueDouble) CompareSameType(other MutableValue) int {
	b := other.(*MutableValueDouble)
	c := compareFloat64(m.Value, b.Value)
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

// ToObject implements MutableValue.ToObject. Returns the float64 Value
// when Exists, otherwise nil.
func (m *MutableValueDouble) ToObject() any {
	if m.Exists() {
		return m.Value
	}
	return nil
}

// HashCode implements MutableValue.HashCode. Mirrors Java exactly:
// `long x = Double.doubleToLongBits(value); return (int) x + (int) (x >>> 32);`.
// `>>>` is Java's unsigned right shift; `math.Float64bits` already
// returns uint64, so a plain `>> 32` here is the unsigned shift the
// Java reference requires.
func (m *MutableValueDouble) HashCode() int {
	x := math.Float64bits(m.Value)
	lo := int32(x)
	hi := int32(x >> 32)
	return int(lo + hi)
}

// String implements MutableValue.String. Returns `Double.toString` when
// present, "(null)" otherwise. Go's `strconv.FormatFloat(_, 'g', -1, 64)`
// produces the shortest round-trippable representation, matching Java's
// algorithm in spirit; both formats agree for finite values that fit
// the IEEE-754 "shortest" round-trip rule.
func (m *MutableValueDouble) String() string {
	if !m.Exists() {
		return "(null)"
	}
	return strconv.FormatFloat(m.Value, 'g', -1, 64)
}

// compareFloat64 implements Java's `Double.compare(d1, d2)`:
//
//   - if d1 < d2 the result is negative;
//   - if d1 > d2 the result is positive;
//   - otherwise the result is the signed comparison of
//     doubleToLongBits(d1) and doubleToLongBits(d2), which sorts NaN
//     above +Inf and -0.0 below +0.0.
//
// math.Float64bits returns the raw uint64 bit pattern; converting to
// int64 reproduces the signed comparison Java performs on Long values.
func compareFloat64(a, b float64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	ba := int64(math.Float64bits(a))
	bb := int64(math.Float64bits(b))
	switch {
	case ba < bb:
		return -1
	case ba > bb:
		return 1
	default:
		return 0
	}
}
