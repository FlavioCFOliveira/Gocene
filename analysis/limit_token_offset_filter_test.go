// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"strings"
	"testing"
)

func TestNewLimitTokenOffsetFilter(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	filter := NewLimitTokenOffsetFilter(tokenizer, 100)

	if filter.GetMaxStartOffset() != 100 {
		t.Errorf("expected maxStartOffset=100, got %d", filter.GetMaxStartOffset())
	}
}

func TestLimitTokenOffsetFilter_IncrementToken(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		maxStartOffset int
		expected       []string
	}{
		{
			name:           "filter_by_offset",
			input:          "one two three four five",
			maxStartOffset: 8, // "one two" ends at offset 7
			expected:       []string{"one", "two"},
		},
		{
			name:           "zero_offset",
			input:          "one two",
			maxStartOffset: 0,
			expected:       []string{},
		},
		{
			name:           "large_offset",
			input:          "one two three",
			maxStartOffset: 1000,
			expected:       []string{"one", "two", "three"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tt.input))
			filter := NewLimitTokenOffsetFilter(tokenizer, tt.maxStartOffset)

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

func TestLimitTokenOffsetFilterFactory(t *testing.T) {
	factory := NewLimitTokenOffsetFilterFactory(50)
	tokenizer := NewWhitespaceTokenizer()
	filter := factory.Create(tokenizer)

	ltof, ok := filter.(*LimitTokenOffsetFilter)
	if !ok {
		t.Fatal("expected LimitTokenOffsetFilter from factory")
	}

	if ltof.GetMaxStartOffset() != 50 {
		t.Errorf("expected maxStartOffset=50 from factory, got %d", ltof.GetMaxStartOffset())
	}
}
