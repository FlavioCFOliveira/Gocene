// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestFuzzyAutomatonBuilder.java
//   No Java test peer exists — synthetic Go tests covering the contract.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// TestFuzzyAutomatonBuilder_InvalidMaxEdits verifies error on invalid maxEdits.
func TestFuzzyAutomatonBuilder_InvalidMaxEdits(t *testing.T) {
	_, err := NewFuzzyAutomatonBuilder("hello", -1, 0, true)
	if err == nil {
		t.Fatal("expected error for maxEdits=-1")
	}
	_, err = NewFuzzyAutomatonBuilder("hello", automaton.MaximumSupportedLevenshteinDistance+1, 0, true)
	if err == nil {
		t.Fatalf("expected error for maxEdits=%d", automaton.MaximumSupportedLevenshteinDistance+1)
	}
}

// TestFuzzyAutomatonBuilder_NegativePrefixLength verifies error on negative prefix.
func TestFuzzyAutomatonBuilder_NegativePrefixLength(t *testing.T) {
	_, err := NewFuzzyAutomatonBuilder("hello", 1, -1, true)
	if err == nil {
		t.Fatal("expected error for prefixLength=-1")
	}
}

// TestFuzzyAutomatonBuilder_GetTermLength verifies Unicode codepoint count.
func TestFuzzyAutomatonBuilder_GetTermLength(t *testing.T) {
	b, err := NewFuzzyAutomatonBuilder("hello", 1, 0, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := b.GetTermLength(); got != 5 {
		t.Fatalf("expected termLength=5, got %d", got)
	}
}

// TestFuzzyAutomatonBuilder_GetTermLength_Multibyte verifies codepoint counting
// for a multi-byte Unicode string (2 codepoints, not 4 UTF-8 bytes).
func TestFuzzyAutomatonBuilder_GetTermLength_Multibyte(t *testing.T) {
	// "αβ" is 2 codepoints, each 2 bytes in UTF-8.
	b, err := NewFuzzyAutomatonBuilder("αβ", 1, 0, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := b.GetTermLength(); got != 2 {
		t.Fatalf("expected termLength=2 for 'αβ', got %d", got)
	}
}

// TestFuzzyAutomatonBuilder_BuildAutomatonSet_Length verifies slice length = maxEdits+1.
func TestFuzzyAutomatonBuilder_BuildAutomatonSet_Length(t *testing.T) {
	for _, maxEdits := range []int{0, 1, 2} {
		b, err := NewFuzzyAutomatonBuilder("test", maxEdits, 0, true)
		if err != nil {
			t.Fatalf("maxEdits=%d: unexpected error: %v", maxEdits, err)
		}
		set := b.BuildAutomatonSet()
		if len(set) != maxEdits+1 {
			t.Fatalf("maxEdits=%d: expected len=%d, got %d", maxEdits, maxEdits+1, len(set))
		}
	}
}

// TestFuzzyAutomatonBuilder_BuildAutomatonSet_NonNil verifies every element is non-nil.
func TestFuzzyAutomatonBuilder_BuildAutomatonSet_NonNil(t *testing.T) {
	b, err := NewFuzzyAutomatonBuilder("test", 2, 0, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i, ca := range b.BuildAutomatonSet() {
		if ca == nil {
			t.Fatalf("set[%d] is nil", i)
		}
	}
}

// TestFuzzyAutomatonBuilder_BuildMaxEditAutomaton_NonNil verifies the single compiled automaton.
func TestFuzzyAutomatonBuilder_BuildMaxEditAutomaton_NonNil(t *testing.T) {
	b, err := NewFuzzyAutomatonBuilder("test", 1, 0, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ca := b.BuildMaxEditAutomaton()
	if ca == nil {
		t.Fatal("BuildMaxEditAutomaton returned nil")
	}
}

// TestFuzzyAutomatonBuilder_PrefixClamping verifies that a prefix longer than
// the term is clamped to the term length without error.
func TestFuzzyAutomatonBuilder_PrefixClamping(t *testing.T) {
	// term "ab" has 2 codepoints; prefixLength=10 must be clamped to 2.
	b, err := NewFuzzyAutomatonBuilder("ab", 1, 10, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.GetTermLength() != 2 {
		t.Fatalf("expected termLength=2, got %d", b.GetTermLength())
	}
	// After full prefix, suffix is empty → automaton still compiles.
	ca := b.BuildMaxEditAutomaton()
	if ca == nil {
		t.Fatal("BuildMaxEditAutomaton returned nil for full-prefix term")
	}

// TestFuzzyAutomatonBuilder_StringToUTF32 verifies codepoint extraction.
func TestFuzzyAutomatonBuilder_StringToUTF32(t *testing.T) {
	cps := stringToUTF32("aβ")
	if len(cps) != 2 {
		t.Fatalf("expected 2 codepoints, got %d", len(cps))
	}
	if cps[0] != 'a' {
		t.Fatalf("cps[0]: expected %d ('a'), got %d", 'a', cps[0])
	}
	if cps[1] != 'β' {
		t.Fatalf("cps[1]: expected %d ('β'), got %d", 'β', cps[1])
	}
}