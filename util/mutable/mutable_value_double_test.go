// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Behavioural coverage for MutableValueDouble.
//
// Lucene 10.4.0 ships no direct TestMutableValueDouble.java; this file
// exercises the contract declared in the abstract base
// (org.apache.lucene.util.mutable.MutableValue) as observed against
// the Java source of MutableValueDouble, including the Java-specific
// Double.compare semantics (NaN > +Inf, -0.0 < +0.0).

package mutable

import (
	"math"
	"testing"
)

func doubleWith(value float64, exists bool) *MutableValueDouble {
	v := &MutableValueDouble{Value: value}
	v.SetExists(exists)
	return v
}

func TestMutableValueDouble_DefaultsAndExists(t *testing.T) {
	v := NewMutableValueDouble()
	if !v.Exists() {
		t.Errorf("new value Exists() = false, want true")
	}
	if v.Value != 0.0 {
		t.Errorf("new value.Value = %v, want 0", v.Value)
	}
	if got := v.ToObject(); got != 0.0 {
		t.Errorf("ToObject(): got %v want 0.0", got)
	}
	v.SetExists(false)
	if got := v.ToObject(); got != nil {
		t.Errorf("ToObject() absent: got %v want nil", got)
	}
}

func TestMutableValueDouble_Copy(t *testing.T) {
	src := doubleWith(3.14159, true)
	dst := NewMutableValueDouble()
	dst.Copy(src)
	if dst.Value != 3.14159 || !dst.Exists() {
		t.Errorf("after Copy: Value=%v Exists=%v", dst.Value, dst.Exists())
	}
}

func TestMutableValueDouble_Duplicate(t *testing.T) {
	src := doubleWith(-2.5, true)
	d := src.Duplicate().(*MutableValueDouble)
	if d == src {
		t.Errorf("Duplicate returned same pointer")
	}
	if d.Value != -2.5 || !d.Exists() {
		t.Errorf("Duplicate state mismatch")
	}
}

func TestMutableValueDouble_EqualsSameType(t *testing.T) {
	cases := []struct {
		name string
		a, b *MutableValueDouble
		want bool
	}{
		{"equal", doubleWith(1.5, true), doubleWith(1.5, true), true},
		{"value diff", doubleWith(1.0, true), doubleWith(2.0, true), false},
		{"exists diff", doubleWith(0.0, true), doubleWith(0.0, false), false},
		// Two present NaNs do not compare equal (NaN != NaN), matching Java.
		{"nan != nan", doubleWith(math.NaN(), true), doubleWith(math.NaN(), true), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.a.EqualsSameType(tc.b); got != tc.want {
				t.Errorf("EqualsSameType: got %v want %v", got, tc.want)
			}
		})
	}
}

func TestMutableValueDouble_CompareSameType(t *testing.T) {
	nan := math.NaN()
	posInf := math.Inf(1)
	negInf := math.Inf(-1)
	cases := []struct {
		name   string
		a, b   *MutableValueDouble
		expect func(int) bool
	}{
		{"less", doubleWith(1.0, true), doubleWith(2.0, true), func(c int) bool { return c < 0 }},
		{"greater", doubleWith(2.0, true), doubleWith(1.0, true), func(c int) bool { return c > 0 }},
		{"equal", doubleWith(7.0, true), doubleWith(7.0, true), func(c int) bool { return c == 0 }},
		{"present>absent", doubleWith(0, true), doubleWith(0, false), func(c int) bool { return c > 0 }},
		// Java semantics: -0.0 < +0.0 (bit-level).
		{"-0 < +0", doubleWith(math.Copysign(0, -1), true), doubleWith(0.0, true), func(c int) bool { return c < 0 }},
		// Java semantics: NaN > +Inf > finite > -Inf.
		{"nan > +inf", doubleWith(nan, true), doubleWith(posInf, true), func(c int) bool { return c > 0 }},
		{"+inf > finite", doubleWith(posInf, true), doubleWith(1e308, true), func(c int) bool { return c > 0 }},
		{"-inf < finite", doubleWith(negInf, true), doubleWith(-1e308, true), func(c int) bool { return c < 0 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.a.CompareSameType(tc.b); !tc.expect(got) {
				t.Errorf("CompareSameType: got %d", got)
			}
		})
	}
}

func TestMutableValueDouble_HashCode(t *testing.T) {
	// hash = (int)bits + (int)(bits >>> 32) where bits = doubleToLongBits.
	cases := []float64{0.0, 1.0, -1.0, 3.14, math.Pi, math.NaN(), math.Inf(1), math.Inf(-1)}
	for _, f := range cases {
		bits := math.Float64bits(f)
		want := int(int32(bits) + int32(bits>>32))
		got := doubleWith(f, true).HashCode()
		if got != want {
			t.Errorf("HashCode(%v): got %d want %d", f, got, want)
		}
	}
}

func TestMutableValueDouble_String(t *testing.T) {
	cases := []struct {
		v    *MutableValueDouble
		want string
	}{
		{doubleWith(0.0, true), "0"},
		{doubleWith(1.5, true), "1.5"},
		{doubleWith(0, false), "(null)"},
	}
	for _, tc := range cases {
		if got := tc.v.String(); got != tc.want {
			t.Errorf("String: got %q want %q", got, tc.want)
		}
	}
}

func TestMutableValueDouble_PolymorphicHelpers(t *testing.T) {
	a := doubleWith(5.5, true)
	b := doubleWith(5.5, true)
	if !Equals(a, b) {
		t.Errorf("Equals: got false want true")
	}
	if c := CompareTo(a, b); c != 0 {
		t.Errorf("CompareTo: got %d want 0", c)
	}
}
