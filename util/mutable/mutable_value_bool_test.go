// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Behavioural coverage for MutableValueBool.
//
// Lucene 10.4.0 ships no direct TestMutableValueBool.java; this file
// exercises the contract declared in the abstract base
// (org.apache.lucene.util.mutable.MutableValue) as observed against
// the Java source of MutableValueBool: exists semantics, copy,
// duplicate, equals/compare/hash invariants, toObject, and
// toString.

package mutable

import "testing"

func TestMutableValueBool_DefaultsAndExists(t *testing.T) {
	v := NewMutableValueBool()
	if !v.Exists() {
		t.Errorf("new value Exists() = false, want true")
	}
	if v.Value != false {
		t.Errorf("new value.Value = %v, want false", v.Value)
	}
	if got := v.ToObject(); got != false {
		t.Errorf("ToObject() with Exists=true Value=false: got %v want false", got)
	}
	v.SetExists(false)
	if got := v.ToObject(); got != nil {
		t.Errorf("ToObject() with Exists=false: got %v want nil", got)
	}
}

func TestMutableValueBool_Copy(t *testing.T) {
	src := NewMutableValueBool()
	src.Value = true
	src.SetExists(true)
	dst := NewMutableValueBool()
	dst.Copy(src)
	if dst.Value != true || !dst.Exists() {
		t.Errorf("after Copy: Value=%v Exists=%v; want true,true", dst.Value, dst.Exists())
	}
	// Mutating src must not change dst (no shared state).
	src.Value = false
	if dst.Value != true {
		t.Errorf("Copy not isolated: src mutation affected dst")
	}
}

func TestMutableValueBool_Duplicate(t *testing.T) {
	src := NewMutableValueBool()
	src.Value = true
	d := src.Duplicate().(*MutableValueBool)
	if d == src {
		t.Errorf("Duplicate returned same pointer; want fresh instance")
	}
	if d.Value != true || !d.Exists() {
		t.Errorf("Duplicate state: Value=%v Exists=%v; want true,true", d.Value, d.Exists())
	}
}

func TestMutableValueBool_EqualsSameType(t *testing.T) {
	cases := []struct {
		name    string
		a, b    *MutableValueBool
		want    bool
	}{
		{"equal true",
			boolWith(true, true), boolWith(true, true), true},
		{"equal false",
			boolWith(false, true), boolWith(false, true), true},
		{"value diff",
			boolWith(true, true), boolWith(false, true), false},
		{"exists diff",
			boolWith(false, true), boolWith(false, false), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.a.EqualsSameType(tc.b); got != tc.want {
				t.Errorf("EqualsSameType: got %v want %v", got, tc.want)
			}
		})
	}
}

func TestMutableValueBool_CompareSameType(t *testing.T) {
	cases := []struct {
		name   string
		a, b   *MutableValueBool
		expect func(int) bool
	}{
		{"true > false",
			boolWith(true, true), boolWith(false, true),
			func(c int) bool { return c > 0 }},
		{"false < true",
			boolWith(false, true), boolWith(true, true),
			func(c int) bool { return c < 0 }},
		{"equal",
			boolWith(true, true), boolWith(true, true),
			func(c int) bool { return c == 0 }},
		{"same value, present > absent",
			boolWith(false, true), boolWith(false, false),
			func(c int) bool { return c > 0 }},
		{"same value, absent < present",
			boolWith(false, false), boolWith(false, true),
			func(c int) bool { return c < 0 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.a.CompareSameType(tc.b); !tc.expect(got) {
				t.Errorf("CompareSameType: got %d, predicate failed", got)
			}
		})
	}
}

func TestMutableValueBool_HashCode(t *testing.T) {
	// Mirror Java: value ? 2 : (exists ? 1 : 0)
	cases := []struct {
		name string
		v    *MutableValueBool
		want int
	}{
		{"true,exists=true", boolWith(true, true), 2},
		{"false,exists=true", boolWith(false, true), 1},
		{"false,exists=false", boolWith(false, false), 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.v.HashCode(); got != tc.want {
				t.Errorf("HashCode: got %d want %d", got, tc.want)
			}
		})
	}
}

func TestMutableValueBool_String(t *testing.T) {
	cases := []struct {
		name string
		v    *MutableValueBool
		want string
	}{
		{"true", boolWith(true, true), "true"},
		{"false", boolWith(false, true), "false"},
		{"absent", boolWith(false, false), "(null)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.v.String(); got != tc.want {
				t.Errorf("String: got %q want %q", got, tc.want)
			}
		})
	}
}

func TestMutableValueBool_PolymorphicEquals(t *testing.T) {
	a := boolWith(true, true)
	b := boolWith(true, true)
	if !Equals(a, b) {
		t.Errorf("Equals polymorphic: got false want true")
	}
}

func TestMutableValueBool_PolymorphicCompare(t *testing.T) {
	a := boolWith(false, true)
	b := boolWith(true, true)
	if got := CompareTo(a, b); got >= 0 {
		t.Errorf("CompareTo(false,true): got %d want <0", got)
	}
}

// boolWith builds a MutableValueBool with the given (value, exists).
func boolWith(value, exists bool) *MutableValueBool {
	v := &MutableValueBool{Value: value}
	v.SetExists(exists)
	return v
}
