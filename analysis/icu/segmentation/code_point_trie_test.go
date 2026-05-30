// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package segmentation

import (
	"encoding/binary"
	"testing"
)

// trieFromBRK parses the named embedded .brk blob and returns its character
// category trie plus the forward state-table dict-start threshold.
func trieFromBRK(t *testing.T, name string) *codePointTrie {
	t.Helper()
	dict, err := LoadEmbeddedBRK(name)
	if err != nil {
		t.Fatalf("LoadEmbeddedBRK(%s): %v", name, err)
	}
	data, err := parseRBBIData(dict)
	if err != nil {
		t.Fatalf("parseRBBIData(%s): %v", name, err)
	}
	return data.trie
}

// TestCodePointTrie_DefaultCategories validates the Default.brk character
// category trie against ground-truth categories computed from the ICU4J
// CodePointTrie algorithm (see /tmp probe and ICU release-70-1 source).
func TestCodePointTrie_DefaultCategories(t *testing.T) {
	t.Parallel()
	trie := trieFromBRK(t, EmbeddedDefaultBRKName)

	cases := []struct {
		name string
		cp   int
		want int
	}{
		{"han 中", 0x4E2D, 27},
		{"thai ก", 0x0E01, 24},
		{"latin a", 'a', 14},
		{"digit 5", '5', 12},
		{"space", ' ', 7},
		{"emoji U+1F600", 0x1F600, 16},
	}
	for _, c := range cases {
		if got := trie.Get(c.cp); got != c.want {
			t.Errorf("Default trie Get(%#x %s) = %d, want %d", c.cp, c.name, got, c.want)
		}
	}
}

// TestCodePointTrie_MyanmarCategories validates the MyanmarSyllable.brk trie.
func TestCodePointTrie_MyanmarCategories(t *testing.T) {
	t.Parallel()
	trie := trieFromBRK(t, EmbeddedMyanmarSyllableBRKName)

	cases := []struct {
		name string
		cp   int
		want int
	}{
		{"consonant ka U+1000", 0x1000, 5},
		{"asat U+103A", 0x103A, 9},
		{"sign U+1036", 0x1036, 8},
		{"vowel U+102C", 0x102C, 8},
		{"medial U+103B", 0x103B, 8},
		{"independent vowel U+102A", 0x102A, 5},
		{"space", ' ', 3},
		{"latin A", 'A', 3},
	}
	for _, c := range cases {
		if got := trie.Get(c.cp); got != c.want {
			t.Errorf("Myanmar trie Get(%#x %s) = %d, want %d", c.cp, c.name, got, c.want)
		}
	}
}

// TestCodePointTrie_OutOfRange checks the error/high-value paths.
func TestCodePointTrie_OutOfRange(t *testing.T) {
	t.Parallel()
	trie := trieFromBRK(t, EmbeddedMyanmarSyllableBRKName)

	// Negative and above-Unicode code points return the error value without
	// panicking.
	if got := trie.Get(-1); got < 0 {
		t.Errorf("Get(-1) = %d, want a valid (non-negative) error value", got)
	}
	if got := trie.Get(0x110000); got < 0 {
		t.Errorf("Get(0x110000) = %d, want a valid error value", got)
	}
	// A code point well above highStart returns the high value.
	if got := trie.Get(0xF0000); got < 0 {
		t.Errorf("Get(0xF0000) = %d, want a valid high value", got)
	}
}

// TestCodePointTrie_SignatureGuard ensures a bad signature is rejected.
func TestCodePointTrie_SignatureGuard(t *testing.T) {
	t.Parallel()
	buf := make([]byte, trieHeaderSize)
	binary.BigEndian.PutUint32(buf, 0xdeadbeef)
	if _, _, err := parseCodePointTrie(buf, binary.BigEndian); err == nil {
		t.Fatal("parseCodePointTrie: expected error on bad signature")
	}
}
