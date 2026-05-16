// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Behavioural coverage for MutableValueDate.
//
// Lucene 10.4.0 ships no direct TestMutableValueDate.java; this file
// exercises the contract declared in the abstract base
// (org.apache.lucene.util.mutable.MutableValue) plus the Date-specific
// overrides (ToObject returns time.Time, Duplicate yields a
// *MutableValueDate so reflective type checks treat dates as a distinct
// class from longs).

package mutable

import (
	"testing"
	"time"
)

func dateWith(value int64, exists bool) *MutableValueDate {
	v := &MutableValueDate{}
	v.Value = value
	v.SetExists(exists)
	return v
}

func TestMutableValueDate_DefaultsAndExists(t *testing.T) {
	v := NewMutableValueDate()
	if !v.Exists() {
		t.Errorf("new value Exists() = false, want true")
	}
	if v.Value != 0 {
		t.Errorf("new value.Value = %d, want 0", v.Value)
	}
	got := v.ToObject()
	want := time.UnixMilli(0).UTC()
	if g, ok := got.(time.Time); !ok || !g.Equal(want) {
		t.Errorf("ToObject(): got %v want %v", got, want)
	}
	v.SetExists(false)
	if got := v.ToObject(); got != nil {
		t.Errorf("ToObject() absent: got %v want nil", got)
	}
}

func TestMutableValueDate_Copy(t *testing.T) {
	src := dateWith(1_600_000_000_000, true)
	dst := NewMutableValueDate()
	dst.Copy(src)
	if dst.Value != 1_600_000_000_000 || !dst.Exists() {
		t.Errorf("after Copy: Value=%d Exists=%v", dst.Value, dst.Exists())
	}
}

func TestMutableValueDate_CopyRejectsForeignType(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Copy from MutableValueLong should panic")
		}
	}()
	src := longWith(1_600_000_000_000, true)
	dst := NewMutableValueDate()
	dst.Copy(src)
}

func TestMutableValueDate_Duplicate(t *testing.T) {
	src := dateWith(42, true)
	d := src.Duplicate()
	if _, ok := d.(*MutableValueDate); !ok {
		t.Fatalf("Duplicate did not return *MutableValueDate (got %T)", d)
	}
	dd := d.(*MutableValueDate)
	if dd == src {
		t.Errorf("Duplicate returned same pointer")
	}
	if dd.Value != 42 || !dd.Exists() {
		t.Errorf("Duplicate state mismatch: Value=%d Exists=%v", dd.Value, dd.Exists())
	}
}

func TestMutableValueDate_EqualsSameType(t *testing.T) {
	cases := []struct {
		name string
		a, b *MutableValueDate
		want bool
	}{
		{"equal", dateWith(123, true), dateWith(123, true), true},
		{"value diff", dateWith(1, true), dateWith(2, true), false},
		{"exists diff", dateWith(0, true), dateWith(0, false), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.a.EqualsSameType(tc.b); got != tc.want {
				t.Errorf("EqualsSameType: got %v want %v", got, tc.want)
			}
		})
	}
}

func TestMutableValueDate_CompareSameType(t *testing.T) {
	cases := []struct {
		name   string
		a, b   *MutableValueDate
		expect func(int) bool
	}{
		{"less", dateWith(1, true), dateWith(2, true), func(c int) bool { return c < 0 }},
		{"greater", dateWith(2, true), dateWith(1, true), func(c int) bool { return c > 0 }},
		{"equal", dateWith(7, true), dateWith(7, true), func(c int) bool { return c == 0 }},
		{"present>absent", dateWith(0, true), dateWith(0, false), func(c int) bool { return c > 0 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.a.CompareSameType(tc.b); !tc.expect(got) {
				t.Errorf("CompareSameType: got %d", got)
			}
		})
	}
}

func TestMutableValueDate_InheritedHashCodeAndString(t *testing.T) {
	// Java MutableValueDate does not override hashCode/toString; these
	// values must match what MutableValueLong produces for the same
	// payload.
	d := dateWith(0x00000001_00000001, true)
	l := longWith(0x00000001_00000001, true)
	if d.HashCode() != l.HashCode() {
		t.Errorf("HashCode parity: date=%d long=%d", d.HashCode(), l.HashCode())
	}
	if d.String() != l.String() {
		t.Errorf("String parity: date=%q long=%q", d.String(), l.String())
	}
	dAbsent := dateWith(0, false)
	if got := dAbsent.String(); got != "(null)" {
		t.Errorf("String absent: got %q want (null)", got)
	}
}

func TestMutableValueDate_DistinctFromLong(t *testing.T) {
	// MutableValueDate and MutableValueLong sharing the same payload
	// must never be reported equal by the polymorphic helper; this is
	// the Java getClass() == other.getClass() rule.
	d := dateWith(1, true)
	l := longWith(1, true)
	if Equals(d, l) {
		t.Errorf("Equals(date,long): got true want false")
	}
	if CompareTo(d, l) == 0 {
		t.Errorf("CompareTo(date,long): got 0 want non-zero")
	}
}

func TestMutableValueDate_ToObjectIsUTC(t *testing.T) {
	d := dateWith(1_700_000_000_123, true)
	got := d.ToObject()
	tm, ok := got.(time.Time)
	if !ok {
		t.Fatalf("ToObject not time.Time: %T", got)
	}
	if tm.UnixMilli() != 1_700_000_000_123 {
		t.Errorf("UnixMilli: got %d want 1700000000123", tm.UnixMilli())
	}
	if loc := tm.Location().String(); loc != "UTC" {
		t.Errorf("Location: got %q want UTC", loc)
	}
}
