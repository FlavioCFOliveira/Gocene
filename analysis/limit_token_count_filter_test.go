// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"strings"
	"testing"
)

func TestNewLimitTokenCountFilter(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	filter := NewLimitTokenCountFilter(tokenizer, 5)

	if filter.GetMaxTokenCount() != 5 {
		t.Errorf("expected maxTokenCount=5, got %d", filter.GetMaxTokenCount())
	}
}

func TestLimitTokenCountFilter_IncrementToken(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		maxTokenCount int
		expected      []string
	}{
		{
			name:          "limit_to_first_tokens",
			input:         "one two three four five six seven",
			maxTokenCount: 3,
			expected:      []string{"one", "two", "three"},
		},
		{
			name:          "limit_zero_tokens",
			input:         "one two three",
			maxTokenCount: 0,
			expected:      []string{},
		},
		{
			name:          "limit_more_than_available",
			input:         "one two",
			maxTokenCount: 10,
			expected:      []string{"one", "two"},
		},
		{
			name:          "limit_exact_count",
			input:         "one two three",
			maxTokenCount: 3,
			expected:      []string{"one", "two", "three"},
		},
		{
			name:          "limit_one_token",
			input:         "first second third",
			maxTokenCount: 1,
			expected:      []string{"first"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tt.input))
			filter := NewLimitTokenCountFilter(tokenizer, tt.maxTokenCount)

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

func TestLimitTokenCountFilter_EmptyInput(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader(""))
	filter := NewLimitTokenCountFilter(tokenizer, 5)

	hasToken, err := filter.IncrementToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasToken {
		t.Error("expected no tokens for empty input")
	}
}

func TestLimitTokenCountFilterFactory(t *testing.T) {
	factory := NewLimitTokenCountFilterFactory(3)
	tokenizer := NewWhitespaceTokenizer()
	filter := factory.Create(tokenizer)

	ltcf, ok := filter.(*LimitTokenCountFilter)
	if !ok {
		t.Fatal("expected LimitTokenCountFilter from factory")
	}

	if ltcf.GetMaxTokenCount() != 3 {
		t.Errorf("expected maxTokenCount=3 from factory, got %d", ltcf.GetMaxTokenCount())
	}
}

func TestLimitTokenCountFilter_ConsumedState(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("one two three four five"))
	filter := NewLimitTokenCountFilter(tokenizer, 2)

	// Consume first token
	hasToken, _ := filter.IncrementToken()
	if !hasToken {
		t.Error("expected first token to be available")
	}

	// Consume second token
	hasToken, _ = filter.IncrementToken()
	if !hasToken {
		t.Error("expected second token to be available")
	}

	// After reaching limit, should return false
	hasToken, _ = filter.IncrementToken()
	if hasToken {
		t.Error("expected no more tokens after reaching limit")
	}

	// Subsequent calls should also return false
	hasToken, _ = filter.IncrementToken()
	if hasToken {
		t.Error("expected no tokens on subsequent calls")
	}
}
