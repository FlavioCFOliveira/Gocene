// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"strings"
	"testing"
	"unicode"
)

func collectTokens(t *testing.T, ct *CharTokenizer, input string) []CharToken {
	t.Helper()
	ct.Reset(strings.NewReader(input))
	var tokens []CharToken
	for {
		tok, ok, err := ct.Next()
		if err != nil {
			t.Fatalf("Next(): %v", err)
		}
		if !ok {
			break
		}
		tokens = append(tokens, tok)
	}
	return tokens
}

// TestCharTokenizer_Letter mirrors LetterTokenizer behaviour.
func TestCharTokenizer_Letter(t *testing.T) {
	ct := FromTokenCharPredicate(unicode.IsLetter)
	tokens := collectTokens(t, ct, "Hello, World! 123 Test.")
	want := []string{"Hello", "World", "Test"}
	if len(tokens) != len(want) {
		t.Fatalf("got %d tokens, want %d: %v", len(tokens), len(want), tokens)
	}
	for i, w := range want {
		if tokens[i].Text != w {
			t.Errorf("token[%d].Text = %q, want %q", i, tokens[i].Text, w)
		}
	}
}

// TestCharTokenizer_Whitespace mirrors WhitespaceTokenizer behaviour.
func TestCharTokenizer_Whitespace(t *testing.T) {
	ct := FromSeparatorCharPredicate(unicode.IsSpace)
	tokens := collectTokens(t, ct, "one two  three")
	want := []string{"one", "two", "three"}
	if len(tokens) != len(want) {
		t.Fatalf("got %d tokens, want %d: %v", len(tokens), len(want), tokens)
	}
	for i, w := range want {
		if tokens[i].Text != w {
			t.Errorf("token[%d].Text = %q, want %q", i, tokens[i].Text, w)
		}
	}
}

// TestCharTokenizer_MaxWordLen verifies token splitting at maxTokenLen.
func TestCharTokenizer_MaxWordLen(t *testing.T) {
	const maxLen = 5
	ct := NewCharTokenizerWithMaxLen(maxLen)
	ct.IsTokenChar = unicode.IsLetter
	tokens := collectTokens(t, ct, "abcdefghij")
	if len(tokens) != 2 {
		t.Fatalf("got %d tokens, want 2: %v", len(tokens), tokens)
	}
	if tokens[0].Text != "abcde" {
		t.Errorf("token[0] = %q, want %q", tokens[0].Text, "abcde")
	}
	if tokens[1].Text != "fghij" {
		t.Errorf("token[1] = %q, want %q", tokens[1].Text, "fghij")
	}
}

// TestCharTokenizer_Offsets verifies StartOffset and EndOffset.
func TestCharTokenizer_Offsets(t *testing.T) {
	ct := FromTokenCharPredicate(unicode.IsLetter)
	tokens := collectTokens(t, ct, "abc xyz")
	if len(tokens) != 2 {
		t.Fatalf("got %d tokens, want 2", len(tokens))
	}
	if tokens[0].StartOffset != 0 || tokens[0].EndOffset != 3 {
		t.Errorf("token[0] offsets = [%d,%d], want [0,3]", tokens[0].StartOffset, tokens[0].EndOffset)
	}
	if tokens[1].StartOffset != 4 || tokens[1].EndOffset != 7 {
		t.Errorf("token[1] offsets = [%d,%d], want [4,7]", tokens[1].StartOffset, tokens[1].EndOffset)
	}
}

// TestCharTokenizer_Unicode verifies multi-byte runes are handled.
func TestCharTokenizer_Unicode(t *testing.T) {
	ct := FromTokenCharPredicate(unicode.IsLetter)
	tokens := collectTokens(t, ct, "héllo wörld")
	want := []string{"héllo", "wörld"}
	if len(tokens) != len(want) {
		t.Fatalf("got %d tokens, want %d: %v", len(tokens), len(want), tokens)
	}
	for i, w := range want {
		if tokens[i].Text != w {
			t.Errorf("token[%d] = %q, want %q", i, tokens[i].Text, w)
		}
	}
}

// TestCharTokenizer_Empty verifies empty input produces no tokens.
func TestCharTokenizer_Empty(t *testing.T) {
	ct := FromTokenCharPredicate(unicode.IsLetter)
	tokens := collectTokens(t, ct, "")
	if len(tokens) != 0 {
		t.Errorf("expected no tokens, got %v", tokens)
	}
}
