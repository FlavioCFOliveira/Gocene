// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Behavioural coverage for MutableValueLong.
//
// Lucene 10.4.0 ships no direct TestMutableValueLong.java; this file
// exercises the contract declared in the abstract base
// (org.apache.lucene.util.mutable.MutableValue) as observed against
// the Java source of MutableValueLong.

package mutable

import "testing"

func longWith(value int64, exists bool) *MutableValueLong {
	v := &MutableValueLong{Value: value}
	v.SetExists(exists)
	return v
}

func TestMutableValueLong_DefaultsAndExists(t *testing.T) {
	v := NewMutableValueLong()
	if !v.Exists() {
		t.Errorf("new value Exists() = false, want true")
	}
	if v.Value != 0 {
		t.Errorf("new value.Value = %d, want 0", v.Value)
	}
	if got := v.ToObject(); got != int64(0) {
		t.Errorf("ToObject(): got %v want int64(0)", got)
	}
	v.SetExists(false)
	if got := v.ToObject(); got != nil {
		t.Errorf("ToObject() absent: got %v want nil", got)
	}
}

func TestMutableValueLong_Copy(t *testing.T) {
	src := longWith(1<<40, true)
	dst := NewMutableValueLong()
	dst.Copy(src)
	if dst.Value != 1<<40 || !dst.Exists() {
		t.Errorf("after Copy: Value=%d Exists=%v; want %d,true", dst.Value, dst.Exists(), int64(1)<<40)
	}
}

func TestMutableValueLong_Duplicate(t *testing.T) {
	src := longWith(-9999, true)
	d := src.Duplicate().(*MutableValueLong)
	if d == src {
		t.Errorf("Duplicate returned same pointer")
	}
	if d.Value != -9999 || !d.Exists() {
		t.Errorf("Duplicate state mismatch")
	}
}

func TestMutableValueLong_EqualsSameType(t *testing.T) {
	cases := []struct {
		name string
		a, b *MutableValueLong
		want bool
	}{
		{"equal", longWith(123456789, true), longWith(123456789, true), true},
		{"value diff", longWith(1, true), longWith(2, true), false},
		{"exists diff", longWith(0, true), longWith(0, false), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.a.EqualsSameType(tc.b); got != tc.want {
				t.Errorf("EqualsSameType: got %v want %v", got, tc.want)
			}
		})
	}
}

func TestMutableValueLong_CompareSameType(t *testing.T) {
	cases := []struct {
		name   string
		a, b   *MutableValueLong
		expect func(int) bool
	}{
		{"less", longWith(1, true), longWith(2, true), func(c int) bool { return c < 0 }},
		{"greater", longWith(2, true), longWith(1, true), func(c int) bool { return c > 0 }},
		{"equal", longWith(7, true), longWith(7, true), func(c int) bool { return c == 0 }},
		{"present>absent", longWith(0, true), longWith(0, false), func(c int) bool { return c > 0 }},
		{"absent<present", longWith(0, false), longWith(0, true), func(c int) bool { return c < 0 }},
		{"negative<positive", longWith(-1, true), longWith(1, true), func(c int) bool { return c < 0 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.a.CompareSameType(tc.b); !tc.expect(got) {
				t.Errorf("CompareSameType: got %d", got)
			}
		})
	}
}

func TestMutableValueLong_HashCode(t *testing.T) {
	// hash = (int)value + (int)(value >> 32) with arithmetic shift.
	cases := []struct {
		v    int64
		want int
	}{
		{0, 0},
		{int64(0x00000000_00000001), 1},
		{int64(0x00000001_00000000), 1},
		{int64(0x00000001_00000001), 2},
		{int64(-1), int(int32(-1)) + int(int32(-1))}, // sign extension keeps both halves at -1
	}
	for _, tc := range cases {
		v := longWith(tc.v, true)
		if got := v.HashCode(); got != tc.want {
			t.Errorf("HashCode(%d): got %d want %d", tc.v, got, tc.want)
		}
	}
}

func TestMutableValueLong_String(t *testing.T) {
	if got := longWith(0, true).String(); got != "0" {
		t.Errorf("String(0): got %q", got)
	}
	if got := longWith(12345, true).String(); got != "12345" {
		t.Errorf("String(12345): got %q", got)
	}
	if got := longWith(-67890, true).String(); got != "-67890" {
		t.Errorf("String(-67890): got %q", got)
	}
	if got := longWith(0, false).String(); got != "(null)" {
		t.Errorf("String absent: got %q want (null)", got)
	}
}

func TestMutableValueLong_PolymorphicHelpers(t *testing.T) {
	a := longWith(99, true)
	b := longWith(99, true)
	if !Equals(a, b) {
		t.Errorf("Equals: got false")
	}
	if c := CompareTo(a, b); c != 0 {
		t.Errorf("CompareTo: got %d want 0", c)
	}
}
