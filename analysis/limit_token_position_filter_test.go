// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"strings"
	"testing"
)

func TestNewLimitTokenPositionFilter(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	filter := NewLimitTokenPositionFilter(tokenizer, 5)

	if filter.GetMaxTokenPosition() != 5 {
		t.Errorf("expected maxTokenPosition=5, got %d", filter.GetMaxTokenPosition())
	}
}

func TestLimitTokenPositionFilter_IncrementToken(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		maxTokenPosition int
		expected         []string
	}{
		{
			name:             "limit_by_position",
			input:            "one two three four five six",
			maxTokenPosition: 3,
			expected:         []string{"one", "two", "three"},
		},
		{
			name:             "zero_position",
			input:            "one two",
			maxTokenPosition: 0,
			expected:         []string{},
		},
		{
			name:             "large_position",
			input:            "one two three",
			maxTokenPosition: 100,
			expected:         []string{"one", "two", "three"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tt.input))
			filter := NewLimitTokenPositionFilter(tokenizer, tt.maxTokenPosition)

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

func TestLimitTokenPositionFilterFactory(t *testing.T) {
	factory := NewLimitTokenPositionFilterFactory(10)
	tokenizer := NewWhitespaceTokenizer()
	filter := factory.Create(tokenizer)

	ltpf, ok := filter.(*LimitTokenPositionFilter)
	if !ok {
		t.Fatal("expected LimitTokenPositionFilter from factory")
	}

	if ltpf.GetMaxTokenPosition() != 10 {
		t.Errorf("expected maxTokenPosition=10 from factory, got %d", ltpf.GetMaxTokenPosition())
	}
}
