// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Behavioural coverage for MutableValueInt.
//
// Lucene 10.4.0 ships no direct TestMutableValueInt.java; this file
// exercises the contract declared in the abstract base
// (org.apache.lucene.util.mutable.MutableValue) as observed against
// the Java source of MutableValueInt: exists semantics, copy,
// duplicate, equals/compare/hash invariants, toObject, and
// toString.

package mutable

import "testing"

func intWith(value int32, exists bool) *MutableValueInt {
	v := &MutableValueInt{Value: value}
	v.SetExists(exists)
	return v
}

func TestMutableValueInt_DefaultsAndExists(t *testing.T) {
	v := NewMutableValueInt()
	if !v.Exists() {
		t.Errorf("new value Exists() = false, want true")
	}
	if v.Value != 0 {
		t.Errorf("new value.Value = %d, want 0", v.Value)
	}
	if got := v.ToObject(); got != int32(0) {
		t.Errorf("ToObject() with Exists=true Value=0: got %v want int32(0)", got)
	}
	v.SetExists(false)
	if got := v.ToObject(); got != nil {
		t.Errorf("ToObject() with Exists=false: got %v want nil", got)
	}
}

func TestMutableValueInt_Copy(t *testing.T) {
	src := intWith(42, true)
	dst := NewMutableValueInt()
	dst.Copy(src)
	if dst.Value != 42 || !dst.Exists() {
		t.Errorf("after Copy: Value=%d Exists=%v; want 42,true", dst.Value, dst.Exists())
	}
}

func TestMutableValueInt_Duplicate(t *testing.T) {
	src := intWith(-7, true)
	d := src.Duplicate().(*MutableValueInt)
	if d == src {
		t.Errorf("Duplicate returned same pointer")
	}
	if d.Value != -7 || !d.Exists() {
		t.Errorf("Duplicate state: Value=%d Exists=%v; want -7,true", d.Value, d.Exists())
	}
}

func TestMutableValueInt_EqualsSameType(t *testing.T) {
	cases := []struct {
		name string
		a, b *MutableValueInt
		want bool
	}{
		{"equal positive", intWith(10, true), intWith(10, true), true},
		{"equal negative", intWith(-3, true), intWith(-3, true), true},
		{"value diff", intWith(1, true), intWith(2, true), false},
		{"exists diff", intWith(0, true), intWith(0, false), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.a.EqualsSameType(tc.b); got != tc.want {
				t.Errorf("EqualsSameType: got %v want %v", got, tc.want)
			}
		})
	}
}

func TestMutableValueInt_CompareSameType(t *testing.T) {
	cases := []struct {
		name   string
		a, b   *MutableValueInt
		expect func(int) bool
	}{
		{"1 < 2", intWith(1, true), intWith(2, true), func(c int) bool { return c < 0 }},
		{"2 > 1", intWith(2, true), intWith(1, true), func(c int) bool { return c > 0 }},
		{"equal", intWith(7, true), intWith(7, true), func(c int) bool { return c == 0 }},
		{"same val, present > absent", intWith(0, true), intWith(0, false), func(c int) bool { return c > 0 }},
		{"same val, absent < present", intWith(0, false), intWith(0, true), func(c int) bool { return c < 0 }},
		{"negative < positive", intWith(-1, true), intWith(1, true), func(c int) bool { return c < 0 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.a.CompareSameType(tc.b); !tc.expect(got) {
				t.Errorf("CompareSameType: got %d", got)
			}
		})
	}
}

func TestMutableValueInt_HashCode(t *testing.T) {
	// hash = (value >> 8) + (value >> 16) using Java-style arithmetic
	// shift on a signed 32-bit integer.
	cases := []struct {
		v    int32
		want int
	}{
		{0, 0},
		{256, 1 + 0},
		{0x10000, 256 + 1},
		{-1, (-1 >> 8) + (-1 >> 16)}, // both shifts preserve sign in Go for int32
		{0x12345678, (0x12345678 >> 8) + (0x12345678 >> 16)},
	}
	for _, tc := range cases {
		v := intWith(tc.v, true)
		if got := v.HashCode(); got != tc.want {
			t.Errorf("HashCode(%d): got %d want %d", tc.v, got, tc.want)
		}
	}
}

func TestMutableValueInt_String(t *testing.T) {
	cases := []struct {
		v    *MutableValueInt
		want string
	}{
		{intWith(0, true), "0"},
		{intWith(123, true), "123"},
		{intWith(-456, true), "-456"},
		{intWith(0, false), "(null)"},
	}
	for _, tc := range cases {
		if got := tc.v.String(); got != tc.want {
			t.Errorf("String: got %q want %q", got, tc.want)
		}
	}
}

func TestMutableValueInt_PolymorphicHelpers(t *testing.T) {
	a := intWith(5, true)
	b := intWith(5, true)
	if !Equals(a, b) {
		t.Errorf("Equals: got false")
	}
	if c := CompareTo(a, b); c != 0 {
		t.Errorf("CompareTo: got %d want 0", c)
	}
	c := intWith(6, true)
	if CompareTo(a, c) >= 0 {
		t.Errorf("CompareTo(5,6): not < 0")
	}
}
