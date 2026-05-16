// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Behavioural coverage for MutableValueStr.
//
// Lucene 10.4.0 ships no direct TestMutableValueStr.java; this file
// exercises the contract declared in the abstract base
// (org.apache.lucene.util.mutable.MutableValue) plus the Str-specific
// requirements: empty-string default, ASCII-fast and UTF-16-correct
// Java String.hashCode parity, lexicographic CompareSameType.

package mutable

import (
	"testing"
	"unicode/utf16"
)

func strWith(value string, exists bool) *MutableValueStr {
	v := &MutableValueStr{Value: value}
	v.SetExists(exists)
	return v
}

// javaStringHash recomputes Java's String.hashCode() directly from the
// UTF-16 expansion of s; used as the oracle for HashCode parity.
func javaStringHash(s string) int32 {
	var h int32
	for _, u := range utf16.Encode([]rune(s)) {
		h = 31*h + int32(u)
	}
	return h
}

func TestMutableValueStr_DefaultsAndExists(t *testing.T) {
	v := NewMutableValueStr()
	if !v.Exists() {
		t.Errorf("new value Exists() = false, want true")
	}
	if v.Value != "" {
		t.Errorf("new value.Value = %q, want \"\"", v.Value)
	}
	if got := v.ToObject(); got != "" {
		t.Errorf("ToObject(): got %v want \"\"", got)
	}
	v.SetExists(false)
	if got := v.ToObject(); got != nil {
		t.Errorf("ToObject() absent: got %v want nil", got)
	}
}

func TestMutableValueStr_Copy(t *testing.T) {
	src := strWith("hello", true)
	dst := NewMutableValueStr()
	dst.Copy(src)
	if dst.Value != "hello" || !dst.Exists() {
		t.Errorf("after Copy: Value=%q Exists=%v", dst.Value, dst.Exists())
	}
}

func TestMutableValueStr_Duplicate(t *testing.T) {
	src := strWith("dup", true)
	d := src.Duplicate().(*MutableValueStr)
	if d == src {
		t.Errorf("Duplicate returned same pointer")
	}
	if d.Value != "dup" || !d.Exists() {
		t.Errorf("Duplicate state mismatch: %+v", d)
	}
}

func TestMutableValueStr_EqualsSameType(t *testing.T) {
	cases := []struct {
		name string
		a, b *MutableValueStr
		want bool
	}{
		{"equal", strWith("abc", true), strWith("abc", true), true},
		{"value diff", strWith("abc", true), strWith("abd", true), false},
		{"exists diff", strWith("", true), strWith("", false), false},
		{"empty equal", strWith("", true), strWith("", true), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.a.EqualsSameType(tc.b); got != tc.want {
				t.Errorf("EqualsSameType: got %v want %v", got, tc.want)
			}
		})
	}
}

func TestMutableValueStr_CompareSameType(t *testing.T) {
	cases := []struct {
		name   string
		a, b   *MutableValueStr
		expect func(int) bool
	}{
		{"abc < abd", strWith("abc", true), strWith("abd", true), func(c int) bool { return c < 0 }},
		{"abd > abc", strWith("abd", true), strWith("abc", true), func(c int) bool { return c > 0 }},
		{"equal", strWith("xyz", true), strWith("xyz", true), func(c int) bool { return c == 0 }},
		{"prefix shorter", strWith("ab", true), strWith("abc", true), func(c int) bool { return c < 0 }},
		{"present>absent", strWith("x", true), strWith("x", false), func(c int) bool { return c > 0 }},
		{"absent<present", strWith("x", false), strWith("x", true), func(c int) bool { return c < 0 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.a.CompareSameType(tc.b); !tc.expect(got) {
				t.Errorf("CompareSameType: got %d", got)
			}
		})
	}
}

func TestMutableValueStr_HashCode(t *testing.T) {
	cases := []string{
		"",
		"a",
		"abc",
		"Hello, World!",
		"café",      // multi-byte UTF-8 inside the BMP
		"𝄞musique", // astral-plane character requires surrogate pair
	}
	for _, s := range cases {
		t.Run(s, func(t *testing.T) {
			want := int(javaStringHash(s))
			got := strWith(s, true).HashCode()
			if got != want {
				t.Errorf("HashCode(%q): got %d want %d", s, got, want)
			}
		})
	}
}

func TestMutableValueStr_HashCodeKnownConstants(t *testing.T) {
	// Cross-checked against Java's String.hashCode() values.
	cases := []struct {
		s    string
		want int
	}{
		{"", 0},
		// 'a' = 97; h = 31*0 + 97 = 97
		{"a", 97},
		// 'a','b','c' → 31*(31*97 + 98) + 99 = 96354
		{"abc", 96354},
		// "Hello, World!".hashCode() = 1498789909 in Java.
		{"Hello, World!", 1498789909},
	}
	for _, tc := range cases {
		got := strWith(tc.s, true).HashCode()
		if got != tc.want {
			t.Errorf("HashCode(%q): got %d want %d", tc.s, got, tc.want)
		}
	}
}

func TestMutableValueStr_String(t *testing.T) {
	cases := []struct {
		v    *MutableValueStr
		want string
	}{
		{strWith("", true), ""},
		{strWith("payload", true), "payload"},
		{strWith("x", false), "(null)"},
	}
	for _, tc := range cases {
		if got := tc.v.String(); got != tc.want {
			t.Errorf("String: got %q want %q", got, tc.want)
		}
	}
}

func TestMutableValueStr_PolymorphicHelpers(t *testing.T) {
	a := strWith("alpha", true)
	b := strWith("alpha", true)
	if !Equals(a, b) {
		t.Errorf("Equals: got false want true")
	}
	if c := CompareTo(a, b); c != 0 {
		t.Errorf("CompareTo: got %d want 0", c)
	}
}
