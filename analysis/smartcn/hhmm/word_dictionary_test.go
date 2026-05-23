// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hhmm

import (
	"testing"
)

// TestWordDictionaryLoad verifies that the dictionary can be loaded.
func TestWordDictionaryLoad(t *testing.T) {
	wd, err := GetWordDictionary()
	if err != nil {
		t.Fatalf("GetWordDictionary: %v", err)
	}
	if wd == nil {
		t.Fatal("GetWordDictionary returned nil")
	}
}

// TestWordDictionaryFrequency verifies that known Chinese words have non-zero
// frequency and unknown sequences have zero frequency.
func TestWordDictionaryFrequency(t *testing.T) {
	wd, err := GetWordDictionary()
	if err != nil {
		t.Fatalf("GetWordDictionary: %v", err)
	}

	// "的" (de) is one of the most frequent Chinese characters; it should
	// appear in the dictionary.
	de := []rune("的")
	if freq := wd.GetFrequency(de); freq == 0 {
		t.Errorf("GetFrequency(%q): want > 0, got 0", string(de))
	}

	// An implausible sequence of characters should return 0.
	impossible := []rune("zzz")
	if freq := wd.GetFrequency(impossible); freq != 0 {
		t.Errorf("GetFrequency(%q): want 0, got %d", string(impossible), freq)
	}
}
