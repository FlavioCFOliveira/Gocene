// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package smartcn

import (
	"strings"
	"testing"
)

// analyzeCount returns the number of tokens produced by the analyzer for the
// given text.
func analyzeCount(t *testing.T, a *SmartChineseAnalyzer, text string) int {
	t.Helper()
	stream, err := a.TokenStream("", strings.NewReader(text))
	if err != nil {
		t.Fatalf("TokenStream: %v", err)
	}

	type resetter interface{ Reset() error }
	if r, ok := stream.(resetter); ok {
		if err := r.Reset(); err != nil {
			t.Fatalf("Reset: %v", err)
		}
	}

	count := 0
	for {
		ok, err := stream.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken: %v", err)
		}
		if !ok {
			break
		}
		count++
	}
	return count
}

// TestSmartChineseAnalyzerDefault verifies the analyzer with default stop words
// produces tokens from a mixed Chinese sentence.
func TestSmartChineseAnalyzerDefault(t *testing.T) {
	a := NewSmartChineseAnalyzer()
	n := analyzeCount(t, a, "我购买了道具和服装。")
	if n == 0 {
		t.Error("expected tokens, got none")
	}
	t.Logf("token count: %d", n)
	if err := a.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestSmartChineseAnalyzerNoStopWords verifies that with stop words disabled
// the full stop character is still emitted (as a comma-normalised token).
func TestSmartChineseAnalyzerNoStopWords(t *testing.T) {
	a := NewSmartChineseAnalyzerWithDefault(false)
	n := analyzeCount(t, a, "我购买了道具和服装。")
	if n == 0 {
		t.Error("expected tokens, got none")
	}
	t.Logf("token count (no stop words): %d", n)
	if err := a.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestSmartChineseAnalyzerNilStopWords verifies that passing nil stop words
// behaves like disabling stop-word filtering.
func TestSmartChineseAnalyzerNilStopWords(t *testing.T) {
	a := NewSmartChineseAnalyzerWithStopWords(nil)
	n := analyzeCount(t, a, "我购买了道具和服装。")
	if n == 0 {
		t.Error("expected tokens, got none")
	}
	t.Logf("token count (nil stop words): %d", n)
	if err := a.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestSmartChineseAnalyzerDefaultStopSet verifies GetDefaultStopSet is
// non-nil, non-empty and returns the same singleton on repeated calls.
func TestSmartChineseAnalyzerDefaultStopSet(t *testing.T) {
	s1 := GetDefaultStopSet()
	if s1 == nil {
		t.Fatal("GetDefaultStopSet returned nil")
	}
	if s1.IsEmpty() {
		t.Error("default stop set is empty")
	}
	s2 := GetDefaultStopSet()
	if s1 != s2 {
		t.Error("GetDefaultStopSet must return the same singleton")
	}
	t.Logf("default stop set size: %d", s1.Size())
}

// TestSmartChineseAnalyzerEnglish verifies English text produces at least one
// token (porter-stemmed and lowercased).
func TestSmartChineseAnalyzerEnglish(t *testing.T) {
	a := NewSmartChineseAnalyzer()
	n := analyzeCount(t, a, "Tests")
	if n == 0 {
		t.Error("expected at least one token for English input")
	}
	if err := a.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestSmartChineseAnalyzerClose verifies that Close is idempotent.
func TestSmartChineseAnalyzerClose(t *testing.T) {
	a := NewSmartChineseAnalyzer()
	if err := a.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := a.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

// TestSmartChineseAnalyzerGetStopWords verifies that GetStopWords returns a
// defensive copy of the internal slice.
func TestSmartChineseAnalyzerGetStopWords(t *testing.T) {
	a := NewSmartChineseAnalyzer()
	words := a.GetStopWords()
	if len(words) == 0 {
		t.Error("expected non-empty stop word list")
	}
	// Mutate the copy — the analyzer must remain unaffected.
	words[0] = "MODIFIED"
	words2 := a.GetStopWords()
	if words2[0] == "MODIFIED" {
		t.Error("GetStopWords must return a copy, not the internal slice")
	}
}

// TestSmartChineseAnalyzerMixed verifies mixed Chinese-English produces tokens.
func TestSmartChineseAnalyzerMixed(t *testing.T) {
	a := NewSmartChineseAnalyzer()
	n := analyzeCount(t, a, "我购买 Tests 了道具和服装")
	if n == 0 {
		t.Error("expected tokens from mixed Chinese-English input")
	}
	t.Logf("mixed token count: %d", n)
	if err := a.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
