// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
// analysis/common/src/test/org/apache/lucene/analysis/classic/TestClassicFactories.java

package classic

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// drainClassicTokenizer drains all CharTermAttribute strings from t.
func drainClassicTokenizer(tb *testing.T, tok *ClassicTokenizer) []string {
	tb.Helper()
	var tokens []string
	for {
		ok, err := tok.IncrementToken()
		if err != nil {
			tb.Fatalf("IncrementToken: %v", err)
		}
		if !ok {
			break
		}
		attr := tok.GetAttribute("CharTermAttribute")
		if attr == nil {
			break
		}
		tokens = append(tokens, attr.(analysis.CharTermAttribute).String())
	}
	return tokens
}

// drainClassicFilter drains all CharTermAttribute strings from f.
func drainClassicFilter(tb *testing.T, f *ClassicFilter) []string {
	tb.Helper()
	var tokens []string
	for {
		ok, err := f.IncrementToken()
		if err != nil {
			tb.Fatalf("IncrementToken: %v", err)
		}
		if !ok {
			break
		}
		attr := f.GetAttribute("CharTermAttribute")
		if attr == nil {
			break
		}
		tokens = append(tokens, attr.(analysis.CharTermAttribute).String())
	}
	return tokens
}

// TestClassicFactories_Tokenizer verifies that ClassicTokenizer splits
// "What's this thing do?" into ["What's", "this", "thing", "do"].
//
// Source: TestClassicFactories.testClassicTokenizer
func TestClassicFactories_Tokenizer(t *testing.T) {
	tok := NewClassicTokenizer()
	tok.SetReader(strings.NewReader("What's this thing do?"))
	_ = tok.Reset()

	got := drainClassicTokenizer(t, tok)
	want := []string{"What's", "this", "thing", "do"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("token[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

// TestClassicFactories_TokenizerMaxTokenLength verifies that a 700-character
// word is kept when maxTokenLength is set to 1000.
//
// Source: TestClassicFactories.testClassicTokenizerMaxTokenLength
func TestClassicFactories_TokenizerMaxTokenLength(t *testing.T) {
	// Build 700-char word.
	var sb strings.Builder
	for i := 0; i < 100; i++ {
		sb.WriteString("abcdefg")
	}
	longWord := sb.String()
	content := "one two three " + longWord + " four five six"

	tok := NewClassicTokenizer()
	tok.SetMaxTokenLength(1000)
	tok.SetReader(strings.NewReader(content))
	_ = tok.Reset()

	got := drainClassicTokenizer(t, tok)
	want := []string{"one", "two", "three", longWord, "four", "five", "six"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("token[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

// TestClassicFactories_Filter verifies that ClassicFilter removes the
// possessive "'s" suffix.
//
// Source: TestClassicFactories.testClassicFilter
func TestClassicFactories_Filter(t *testing.T) {
	tok := NewClassicTokenizer()
	tok.SetReader(strings.NewReader("What's this thing do?"))
	_ = tok.Reset()

	f := NewClassicFilter(tok)
	got := drainClassicFilter(t, f)
	want := []string{"What", "this", "thing", "do"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("token[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}
