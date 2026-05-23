// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package core_test

// TestDuelingAnalyzers ports org.apache.lucene.analysis.core.TestDuelingAnalyzers
// (Apache Lucene 10.4.0).
//
// The Java test builds a CharacterRunAutomaton from Character.isLetter and uses
// MockAnalyzer as a "reference" implementation to compare against LetterTokenizer.
// Both implementations should produce identical tokens for the same input.
//
// Deviation: Gocene has no MockAnalyzer or CharacterRunAutomaton test
// infrastructure.  The port uses a pure-Go reference tokenizer (referenceLetter)
// built from unicode.IsLetter to verify LetterTokenizer produces the same tokens
// on ASCII, HTML-ish, and Unicode inputs.

import (
	"strings"
	"testing"
	"unicode"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// referenceLetterTokenize splits input using Go's unicode.IsLetter,
// collecting maximal letter runs without any length cap — matching
// LetterTokenizer's unbounded default behaviour.
func referenceLetterTokenize(input string) []string {
	var tokens []string
	var cur []rune
	for _, r := range []rune(input) {
		if unicode.IsLetter(r) {
			cur = append(cur, r)
		} else {
			if len(cur) > 0 {
				tokens = append(tokens, string(cur))
				cur = cur[:0]
			}
		}
	}
	if len(cur) > 0 {
		tokens = append(tokens, string(cur))
	}
	return tokens
}

// drainLetterTokenizer drives a LetterTokenizer to exhaustion, collecting
// term text via the concrete type's GetAttributeSource.
func drainLetterTokenizer(t *testing.T, input string) []string {
	t.Helper()
	tok := analysis.NewLetterTokenizer()
	if err := tok.SetReader(strings.NewReader(input)); err != nil {
		t.Fatalf("SetReader: %v", err)
	}
	var tokens []string
	for {
		ok, err := tok.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken: %v", err)
		}
		if !ok {
			break
		}
		attr := tok.GetAttributeSource().GetAttribute(analysis.CharTermAttributeType)
		if attr != nil {
			if cta, ok := attr.(analysis.CharTermAttribute); ok {
				tokens = append(tokens, cta.String())
			}
		}
	}
	_ = tok.End()
	_ = tok.Close()
	return tokens
}

// assertEqual checks that left and right produce identical token sequences
// for the given input string.
func assertEqualTokens(t *testing.T, input string, left, right []string) {
	t.Helper()
	if len(left) != len(right) {
		t.Errorf("input=%q: left=%v (%d), right=%v (%d)",
			input, left, len(left), right, len(right))
		return
	}
	for i := range left {
		if left[i] != right[i] {
			t.Errorf("input=%q token[%d]: left=%q right=%q", input, i, left[i], right[i])
		}
	}
}

// TestDuelingAnalyzers_LetterAscii verifies LetterTokenizer agrees with
// the reference unicode.IsLetter tokenizer on ASCII strings.
// Mirrors TestDuelingAnalyzers.testLetterAscii.
func TestDuelingAnalyzers_LetterAscii(t *testing.T) {
	inputs := []string{
		"",
		"hello world",
		"foo bar FOO BAR",
		"foo.bar.FOO.BAR",
		"U.S.A.",
		"C++",
		"B2B",
		"2B",
		"\"QUOTED\" word",
		"test@test.com",
		"one,two,three",
	}
	for _, input := range inputs {
		t.Run(input[:min(len(input), 20)], func(t *testing.T) {
			want := referenceLetterTokenize(input)
			got := drainLetterTokenizer(t, input)
			assertEqualTokens(t, input, want, got)
		})
	}
}

// TestDuelingAnalyzers_LetterUnicode verifies LetterTokenizer agrees with
// the reference tokenizer on multi-script Unicode strings.
// Mirrors TestDuelingAnalyzers.testLetterUnicode.
func TestDuelingAnalyzers_LetterUnicode(t *testing.T) {
	inputs := []string{
		"café résumé",
		"über straße",
		"日本語テスト",
		"한국어",
		"مرحبا",
		"Привет",
		"abc123def",
		"αβγδε",
		"一二三四五",
		"AbaCaDabA",
	}
	for _, s := range inputs {
		t.Run(s, func(t *testing.T) {
			want := referenceLetterTokenize(s)
			got := drainLetterTokenizer(t, s)
			assertEqualTokens(t, s, want, got)
		})
	}
}

// TestDuelingAnalyzers_LetterHtmlish verifies LetterTokenizer on HTML-ish
// strings with punctuation, numbers, and entities.
// Mirrors TestDuelingAnalyzers.testLetterHtmlish.
func TestDuelingAnalyzers_LetterHtmlish(t *testing.T) {
	inputs := []string{
		"<p>Hello &amp; World</p>",
		"<a href=\"http://example.com\">Link</a>",
		"foo &lt; bar &gt; baz",
		"100% complete",
		"test<br/>case",
		"<div class=\"main\">content</div>",
	}
	for _, s := range inputs {
		t.Run(s[:min(len(s), 30)], func(t *testing.T) {
			want := referenceLetterTokenize(s)
			got := drainLetterTokenizer(t, s)
			assertEqualTokens(t, s, want, got)
		})
	}
}

// min returns the smaller of a and b.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
