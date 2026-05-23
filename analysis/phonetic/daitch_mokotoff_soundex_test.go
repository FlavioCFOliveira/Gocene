// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package phonetic

import (
	"sort"
	"strings"
	"testing"
)

// sortedCodes returns the codes from a pipe-separated string in sorted order.
func sortedCodes(s string) []string {
	parts := strings.Split(s, "|")
	sort.Strings(parts)
	return parts
}

func codeSet(s string) map[string]bool {
	m := make(map[string]bool)
	for _, p := range strings.Split(s, "|") {
		m[p] = true
	}
	return m
}

// TestDMSoundex_Algorithms validates the DM Soundex encoder against the expected
// outputs from TestDaitchMokotoffSoundexFilter.testAlgorithms() in Lucene 10.4.0.
// Source: analysis/phonetic/src/test/.../TestDaitchMokotoffSoundexFilter.java
func TestDMSoundex_Algorithms(t *testing.T) {
	enc := &DaitchMokotoffSoundex{}

	// inject=false input: "aaa bbb ccc easgasg"
	// expected tokens (codes only, inject=false):
	// "000000","700000","400000","450000","454000","540000","545000","500000","045450"
	tests := []struct {
		input   string
		wantAll []string // all of these must be present in the output
	}{
		{"aaa", []string{"000000"}},
		{"bbb", []string{"700000"}},
		{"ccc", []string{"400000", "450000", "454000", "540000", "545000", "500000"}},
		{"easgasg", []string{"045450"}},
	}

	for _, tt := range tests {
		got := enc.Soundex(tt.input)
		parts := codeSet(got)
		for _, want := range tt.wantAll {
			if !parts[want] {
				t.Errorf("Soundex(%q) = %q, want it to contain %q", tt.input, got, want)
			}
		}
	}
}

// TestDMSoundex_International validates "international" which is used in
// TestDaitchMokotoffSoundexFilterFactory.testDefaults().
// Source: analysis/phonetic/src/test/.../TestDaitchMokotoffSoundexFilterFactory.java
func TestDMSoundex_International(t *testing.T) {
	enc := &DaitchMokotoffSoundex{}
	got := enc.Soundex("international")
	parts := codeSet(got)
	want := "063963"
	if !parts[want] {
		t.Errorf("Soundex(%q) = %q, want it to contain %q", "international", got, want)
	}
}
