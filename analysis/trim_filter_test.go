// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"strings"
	"testing"
)

func TestNewTrimFilter(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	filter := NewTrimFilter(tokenizer)

	if filter == nil {
		t.Error("expected filter to be created")
	}
}

func TestTrimFilter_IncrementToken(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "no_whitespace",
			input:    "hello world",
			expected: []string{"hello", "world"},
		},
		{
			name:     "leading_whitespace",
			input:    "  hello",
			expected: []string{"hello"},
		},
		{
			name:     "trailing_whitespace",
			input:    "hello  ",
			expected: []string{"hello"},
		},
		{
			name:     "both_sides_whitespace",
			input:    "  hello  ",
			expected: []string{"hello"},
		},
		{
			name:     "internal_whitespace_preserved",
			input:    "hello   world",
			expected: []string{"hello", "world"},
		},
		{
			name:     "tab_and_newline",
			input:    "\thello\n",
			expected: []string{"hello"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tt.input))
			filter := NewTrimFilter(tokenizer)

			var tokens []string
			for {
				hasToken, err := filter.IncrementToken()
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if !hasToken {
					break
				}
				if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
					tokens = append(tokens, attr.(CharTermAttribute).String())
				}
			}

			if len(tokens) != len(tt.expected) {
				t.Errorf("expected %d tokens, got %d: %v", len(tt.expected), len(tokens), tokens)
				return
			}

			for i, exp := range tt.expected {
				if tokens[i] != exp {
					t.Errorf("expected token[%d]=%q, got %q", i, exp, tokens[i])
				}
			}
		})
	}
}

func TestTrimFilter_EmptyInput(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader(""))
	filter := NewTrimFilter(tokenizer)

	hasToken, err := filter.IncrementToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasToken {
		t.Error("expected no tokens for empty input")
	}
}

func TestTrimFilterFactory(t *testing.T) {
	factory := NewTrimFilterFactory()
	tokenizer := NewWhitespaceTokenizer()
	filter := factory.Create(tokenizer)

	_, ok := filter.(*TrimFilter)
	if !ok {
		t.Fatal("expected TrimFilter from factory")
	}
}
