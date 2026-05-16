// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Port of org.apache.lucene.util.mutable.MutableValueDate from Apache
// Lucene 10.4.0 (Apache License 2.0).

package mutable

import "time"

// MutableValueDate is the [MutableValue] specialisation that stores a
// timestamp expressed as milliseconds since the Unix epoch, mirroring
// Lucene's `MutableValueDate extends MutableValueLong`.
//
// The underlying payload is the same int64 carried by
// [MutableValueLong]; the only behavioural differences from the Long
// variant are ToObject (which boxes the value as a [time.Time] in UTC)
// and Duplicate (which produces a *MutableValueDate so reflective
// type-equality checks distinguish dates from raw longs). The
// concrete-type assertions in Copy/EqualsSameType/CompareSameType
// therefore target *MutableValueDate instead of *MutableValueLong:
// mixing a Date with a Long is a programming error and panics, just as
// it would throw ClassCastException in the Java reference.
//
// HashCode and String are inherited unchanged from MutableValueLong via
// struct embedding; this matches Java where MutableValueDate does not
// override Object#hashCode or Object#toString.
type MutableValueDate struct {
	MutableValueLong
}

// NewMutableValueDate returns a MutableValueDate with Exists set to
// true, matching Java's `public boolean exists = true;` initialiser.
func NewMutableValueDate() *MutableValueDate {
	d := &MutableValueDate{}
	d.SetExists(true)
	return d
}

// Copy implements MutableValue.Copy. Panics if source is not a
// *MutableValueDate, mirroring Java's ClassCastException semantics
// (Java's getClass()-strict equals propagates the same intent: a Long
// and a Date never compare equal even when they share the underlying
// payload).
func (m *MutableValueDate) Copy(source MutableValue) {
	s := source.(*MutableValueDate)
	m.Value = s.Value
	m.SetExists(s.Exists())
}

// Duplicate implements MutableValue.Duplicate. Returns a fresh
// *MutableValueDate so reflect.TypeOf checks treat the copy as a Date.
func (m *MutableValueDate) Duplicate() MutableValue {
	d := &MutableValueDate{}
	d.Value = m.Value
	d.SetExists(m.Exists())
	return d
}

// EqualsSameType implements MutableValue.EqualsSameType. Panics if
// other is not a *MutableValueDate.
func (m *MutableValueDate) EqualsSameType(other MutableValue) bool {
	b := other.(*MutableValueDate)
	return m.Value == b.Value && m.Exists() == b.Exists()
}

// CompareSameType implements MutableValue.CompareSameType. Panics if
// other is not a *MutableValueDate. Algorithm matches MutableValueLong
// (signed compare on Value, ties broken by Exists with absent < present).
func (m *MutableValueDate) CompareSameType(other MutableValue) int {
	b := other.(*MutableValueDate)
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

// ToObject implements MutableValue.ToObject. Returns the payload as a
// [time.Time] in UTC with millisecond precision when Exists, otherwise
// nil. Mirrors Lucene's `return exists ? new Date(value) : null;` where
// java.util.Date is a millisecond-precision UTC instant wrapper.
func (m *MutableValueDate) ToObject() any {
	if m.Exists() {
		return time.UnixMilli(m.Value).UTC()
	}
	return nil
}
