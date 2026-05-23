// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package phonetic

import (
	"testing"
)

// TestSoundex_Algorithms validates Soundex against the expected outputs
// from TestPhoneticFilter.java in Lucene 10.4.0.
// Source: analysis/phonetic/src/test/.../TestPhoneticFilter.java
func TestSoundex_Algorithms(t *testing.T) {
	enc := NewSoundex()
	tests := []struct {
		input string
		want  string
	}{
		{"aaa", "A000"},
		{"bbb", "B000"},
		{"ccc", "C000"},
		{"easgasg", "E220"},
	}
	for _, tt := range tests {
		got := enc.Encode(tt.input)
		if got != tt.want {
			t.Errorf("Soundex(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// TestRefinedSoundex_Algorithms validates RefinedSoundex against the expected
// outputs from TestPhoneticFilter.java in Lucene 10.4.0.
// Source: analysis/phonetic/src/test/.../TestPhoneticFilter.java
func TestRefinedSoundex_Algorithms(t *testing.T) {
	enc := NewRefinedSoundex()
	tests := []struct {
		input string
		want  string
	}{
		{"aaa", "A0"},
		{"bbb", "B1"},
		{"ccc", "C3"},
		{"easgasg", "E034034"},
	}
	for _, tt := range tests {
		got := enc.Encode(tt.input)
		if got != tt.want {
			t.Errorf("RefinedSoundex(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// TestMetaphone_Algorithms validates Metaphone against the expected outputs
// from TestPhoneticFilter.java in Lucene 10.4.0.
// Source: analysis/phonetic/src/test/.../TestPhoneticFilter.java
func TestMetaphone_Algorithms(t *testing.T) {
	enc := NewMetaphone()
	tests := []struct {
		input string
		want  string
	}{
		{"aaa", "A"},
		{"bbb", "B"},
		{"ccc", "KKK"},
		{"easgasg", "ESKS"},
	}
	for _, tt := range tests {
		enc.MaxCodeLen = 4 // default
		got := enc.Encode(tt.input)
		if got != tt.want {
			t.Errorf("Metaphone(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
