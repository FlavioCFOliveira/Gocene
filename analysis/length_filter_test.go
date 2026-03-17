// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"strings"
	"testing"
)

func TestNewLengthFilter(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	filter := NewLengthFilter(tokenizer, 3, 10)

	if filter.GetMinLength() != 3 {
		t.Errorf("expected minLength=3, got %d", filter.GetMinLength())
	}

	if filter.GetMaxLength() != 10 {
		t.Errorf("expected maxLength=10, got %d", filter.GetMaxLength())
	}
}

func TestLengthFilter_IncrementToken(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		minLength int
		maxLength int
		expected  []string
	}{
		{
			name:      "filter_short_and_long_tokens",
			input:     "a ab abc abcd abcdefghijklmnopqrstuvwxyz",
			minLength: 3,
			maxLength: 5,
			expected:  []string{"abc", "abcd"},
		},
		{
			name:      "keep_exact_length_tokens",
			input:     "abc abcd",
			minLength: 3,
			maxLength: 4,
			expected:  []string{"abc", "abcd"},
		},
		{
			name:      "filter_all_short",
			input:     "a ab",
			minLength: 3,
			maxLength: 10,
			expected:  []string{},
		},
		{
			name:      "filter_all_long",
			input:     "verylongtoken",
			minLength: 3,
			maxLength: 5,
			expected:  []string{},
		},
		{
			name:      "keep_all_tokens",
			input:     "ab abc abcd",
			minLength: 1,
			maxLength: 10,
			expected:  []string{"ab", "abc", "abcd"},
		},
		{
			name:      "exact_length_match",
			input:     "abc abcd abcde abcdef",
			minLength: 4,
			maxLength: 4,
			expected:  []string{"abcd"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tt.input))
			filter := NewLengthFilter(tokenizer, tt.minLength, tt.maxLength)

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

func TestLengthFilter_EmptyInput(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader(""))
	filter := NewLengthFilter(tokenizer, 2, 10)

	hasToken, err := filter.IncrementToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasToken {
		t.Error("expected no tokens for empty input")
	}
}

func TestLengthFilterFactory(t *testing.T) {
	factory := NewLengthFilterFactory(3, 10)
	tokenizer := NewWhitespaceTokenizer()
	filter := factory.Create(tokenizer)

	lf, ok := filter.(*LengthFilter)
	if !ok {
		t.Fatal("expected LengthFilter from factory")
	}

	if lf.GetMinLength() != 3 {
		t.Errorf("expected minLength=3 from factory, got %d", lf.GetMinLength())
	}

	if lf.GetMaxLength() != 10 {
		t.Errorf("expected maxLength=10 from factory, got %d", lf.GetMaxLength())
	}
}
