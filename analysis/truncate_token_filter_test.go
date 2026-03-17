// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"strings"
	"testing"
)

func TestNewTruncateTokenFilter(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	filter := NewTruncateTokenFilter(tokenizer, 5)

	if filter.GetMaxLength() != 5 {
		t.Errorf("expected maxLength=5, got %d", filter.GetMaxLength())
	}
}

func TestTruncateTokenFilter_IncrementToken(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		maxLength int
		expected  []string
	}{
		{
			name:      "truncate_long_tokens",
			input:     "hello world",
			maxLength: 3,
			expected:  []string{"hel", "wor"},
		},
		{
			name:      "exact_length_preserved",
			input:     "hello",
			maxLength: 5,
			expected:  []string{"hello"},
		},
		{
			name:      "short_tokens_preserved",
			input:     "hi go",
			maxLength: 5,
			expected:  []string{"hi", "go"},
		},
		{
			name:      "truncate_to_one_char",
			input:     "hello world",
			maxLength: 1,
			expected:  []string{"h", "w"},
		},
		{
			name:      "large_max_length",
			input:     "short",
			maxLength: 100,
			expected:  []string{"short"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tt.input))
			filter := NewTruncateTokenFilter(tokenizer, tt.maxLength)

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

func TestTruncateTokenFilterFactory(t *testing.T) {
	factory := NewTruncateTokenFilterFactory(10)
	tokenizer := NewWhitespaceTokenizer()
	filter := factory.Create(tokenizer)

	ttf, ok := filter.(*TruncateTokenFilter)
	if !ok {
		t.Fatal("expected TruncateTokenFilter from factory")
	}

	if ttf.GetMaxLength() != 10 {
		t.Errorf("expected maxLength=10 from factory, got %d", ttf.GetMaxLength())
	}
}
