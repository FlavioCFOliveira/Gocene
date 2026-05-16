// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Behavioural coverage for MutableValueFloat.
//
// Lucene 10.4.0 ships no direct TestMutableValueFloat.java; this file
// exercises the contract declared in the abstract base
// (org.apache.lucene.util.mutable.MutableValue) as observed against
// the Java source of MutableValueFloat, including the Java-specific
// Float.compare semantics (NaN > +Inf, -0.0 < +0.0) and
// floatToIntBits-based hash.

package mutable

import (
	"math"
	"testing"
)

func floatWith(value float32, exists bool) *MutableValueFloat {
	v := &MutableValueFloat{Value: value}
	v.SetExists(exists)
	return v
}

func TestMutableValueFloat_DefaultsAndExists(t *testing.T) {
	v := NewMutableValueFloat()
	if !v.Exists() {
		t.Errorf("new value Exists() = false, want true")
	}
	if v.Value != 0.0 {
		t.Errorf("new value.Value = %v, want 0", v.Value)
	}
	if got := v.ToObject(); got != float32(0.0) {
		t.Errorf("ToObject(): got %v want 0.0", got)
	}
	v.SetExists(false)
	if got := v.ToObject(); got != nil {
		t.Errorf("ToObject() absent: got %v want nil", got)
	}
}

func TestMutableValueFloat_Copy(t *testing.T) {
	src := floatWith(3.14, true)
	dst := NewMutableValueFloat()
	dst.Copy(src)
	if dst.Value != 3.14 || !dst.Exists() {
		t.Errorf("after Copy: Value=%v Exists=%v", dst.Value, dst.Exists())
	}
}

func TestMutableValueFloat_Duplicate(t *testing.T) {
	src := floatWith(-1.25, true)
	d := src.Duplicate().(*MutableValueFloat)
	if d == src {
		t.Errorf("Duplicate returned same pointer")
	}
	if d.Value != -1.25 || !d.Exists() {
		t.Errorf("Duplicate state mismatch")
	}
}

func TestMutableValueFloat_EqualsSameType(t *testing.T) {
	cases := []struct {
		name string
		a, b *MutableValueFloat
		want bool
	}{
		{"equal", floatWith(1.5, true), floatWith(1.5, true), true},
		{"value diff", floatWith(1.0, true), floatWith(2.0, true), false},
		{"exists diff", floatWith(0.0, true), floatWith(0.0, false), false},
		{"nan != nan", floatWith(float32(math.NaN()), true), floatWith(float32(math.NaN()), true), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.a.EqualsSameType(tc.b); got != tc.want {
				t.Errorf("EqualsSameType: got %v want %v", got, tc.want)
			}
		})
	}
}

func TestMutableValueFloat_CompareSameType(t *testing.T) {
	nan := float32(math.NaN())
	posInf := float32(math.Inf(1))
	negInf := float32(math.Inf(-1))
	cases := []struct {
		name   string
		a, b   *MutableValueFloat
		expect func(int) bool
	}{
		{"less", floatWith(1.0, true), floatWith(2.0, true), func(c int) bool { return c < 0 }},
		{"greater", floatWith(2.0, true), floatWith(1.0, true), func(c int) bool { return c > 0 }},
		{"equal", floatWith(7.0, true), floatWith(7.0, true), func(c int) bool { return c == 0 }},
		{"present>absent", floatWith(0, true), floatWith(0, false), func(c int) bool { return c > 0 }},
		{"-0 < +0", floatWith(float32(math.Copysign(0, -1)), true), floatWith(0.0, true), func(c int) bool { return c < 0 }},
		{"nan > +inf", floatWith(nan, true), floatWith(posInf, true), func(c int) bool { return c > 0 }},
		{"+inf > finite", floatWith(posInf, true), floatWith(1e38, true), func(c int) bool { return c > 0 }},
		{"-inf < finite", floatWith(negInf, true), floatWith(-1e38, true), func(c int) bool { return c < 0 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.a.CompareSameType(tc.b); !tc.expect(got) {
				t.Errorf("CompareSameType: got %d", got)
			}
		})
	}
}

func TestMutableValueFloat_HashCode(t *testing.T) {
	// hash = (int) Float.floatToIntBits(value)
	cases := []float32{0.0, 1.0, -1.0, 3.14, float32(math.NaN()), float32(math.Inf(1)), float32(math.Inf(-1))}
	for _, f := range cases {
		want := int(int32(math.Float32bits(f)))
		got := floatWith(f, true).HashCode()
		if got != want {
			t.Errorf("HashCode(%v): got %d want %d", f, got, want)
		}
	}
}

func TestMutableValueFloat_String(t *testing.T) {
	cases := []struct {
		v    *MutableValueFloat
		want string
	}{
		{floatWith(0.0, true), "0"},
		{floatWith(1.5, true), "1.5"},
		{floatWith(0, false), "(null)"},
	}
	for _, tc := range cases {
		if got := tc.v.String(); got != tc.want {
			t.Errorf("String: got %q want %q", got, tc.want)
		}
	}
}

func TestMutableValueFloat_PolymorphicHelpers(t *testing.T) {
	a := floatWith(5.5, true)
	b := floatWith(5.5, true)
	if !Equals(a, b) {
		t.Errorf("Equals: got false want true")
	}
	if c := CompareTo(a, b); c != 0 {
		t.Errorf("CompareTo: got %d want 0", c)
	}
}

func TestMutableValueFloat_DistinctFromDouble(t *testing.T) {
	// Float and Double sharing a numeric payload must not compare equal
	// under the polymorphic helpers; the reflective getClass() check
	// guarantees this independence.
	f := floatWith(1.0, true)
	d := doubleWith(1.0, true)
	if Equals(f, d) {
		t.Errorf("Equals(float,double): got true want false")
	}
}
