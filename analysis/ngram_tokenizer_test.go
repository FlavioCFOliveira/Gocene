// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"strings"
	"testing"
)

func TestNGramTokenizer_Basic(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		minGram  int
		maxGram  int
		expected []string
	}{
		{
			name:     "Simple 2-grams",
			input:    "abc",
			minGram:  2,
			maxGram:  2,
			expected: []string{"ab", "bc"},
		},
		{
			name:     "Simple 3-grams",
			input:    "abcd",
			minGram:  3,
			maxGram:  3,
			expected: []string{"abc", "bcd"},
		},
		{
			name:     "Mixed 2-3 grams",
			input:    "abc",
			minGram:  2,
			maxGram:  3,
			expected: []string{"ab", "abc", "bc"},
		},
		{
			name:     "Single character input with minGram=1",
			input:    "a",
			minGram:  1,
			maxGram:  2,
			expected: []string{"a"},
		},
		{
			name:     "Two character input with minGram=1",
			input:    "ab",
			minGram:  1,
			maxGram:  2,
			expected: []string{"a", "ab", "b"},
		},
		{
			name:     "Longer text with 2-grams",
			input:    "hello",
			minGram:  2,
			maxGram:  2,
			expected: []string{"he", "el", "ll", "lo"},
		},
		{
			name:     "Longer text with 2-4 grams",
			input:    "hello",
			minGram:  2,
			maxGram:  4,
			expected: []string{"he", "hel", "hell", "el", "ell", "ello", "ll", "llo", "lo"},
		},
		{
			name:     "Empty input",
			input:    "",
			minGram:  2,
			maxGram:  3,
			expected: nil,
		},
		{
			name:     "Input shorter than minGram",
			input:    "ab",
			minGram:  3,
			maxGram:  5,
			expected: nil,
		},
		{
			name:     "Unicode characters",
			input:    "日本語",
			minGram:  2,
			maxGram:  2,
			expected: []string{"日本", "本語"},
		},
		{
			name:     "Unicode with mixed sizes",
			input:    "日本語",
			minGram:  1,
			maxGram:  2,
			expected: []string{"日", "日本", "本", "本語", "語"},
		},
		{
			name:     "Mixed ASCII and Unicode",
			input:    "a日本b",
			minGram:  2,
			maxGram:  2,
			expected: []string{"a日", "日本", "本b"},
		},
		{
			name:     "Whitespace included",
			input:    "a b",
			minGram:  2,
			maxGram:  2,
			expected: []string{"a ", " b"},
		},
		{
			name:     "Numbers and symbols",
			input:    "a1b2c",
			minGram:  2,
			maxGram:  2,
			expected: []string{"a1", "1b", "b2", "2c"},
		},
		{
			name:     "minGram=1 maxGram=1",
			input:    "abc",
			minGram:  1,
			maxGram:  1,
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "Large maxGram",
			input:    "short",
			minGram:  10,
			maxGram:  20,
			expected: nil,
		},
		{
			name:     "Exact length match",
			input:    "hello",
			minGram:  5,
			maxGram:  5,
			expected: []string{"hello"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer := NewNGramTokenizer(tt.minGram, tt.maxGram)
			if tokenizer == nil {
				t.Fatalf("Failed to create NGramTokenizer with minGram=%d, maxGram=%d", tt.minGram, tt.maxGram)
			}

			err := tokenizer.SetReader(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("SetReader failed: %v", err)
			}

			var tokens []string
			for {
				hasToken, err := tokenizer.IncrementToken()
				if err != nil {
					t.Fatalf("IncrementToken failed: %v", err)
				}
				if !hasToken {
					break
				}

				termAttr := tokenizer.GetAttributeSource().GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
				if termAttr != nil {
					if cta, ok := termAttr.(CharTermAttribute); ok {
						tokens = append(tokens, cta.String())
					}
				}
			}

			if len(tokens) != len(tt.expected) {
				t.Errorf("Expected %d tokens, got %d: %v", len(tt.expected), len(tokens), tokens)
				return
			}

			for i := range tokens {
				if tokens[i] != tt.expected[i] {
					t.Errorf("Token %d: expected %q, got %q", i, tt.expected[i], tokens[i])
				}
			}
		})
	}
}

func TestNGramTokenizer_Offsets(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		minGram        int
		maxGram        int
		expectedTokens []struct {
			term  string
			start int
			end   int
		}
	}{
		{
			name:    "2-gram offsets",
			input:   "abcd",
			minGram: 2,
			maxGram: 2,
			expectedTokens: []struct {
				term  string
				start int
				end   int
			}{
				{"ab", 0, 2},
				{"bc", 1, 3},
				{"cd", 2, 4},
			},
		},
		{
			name:    "Mixed 2-3 gram offsets",
			input:   "abc",
			minGram: 2,
			maxGram: 3,
			expectedTokens: []struct {
				term  string
				start int
				end   int
			}{
				{"ab", 0, 2},
				{"abc", 0, 3},
				{"bc", 1, 3},
			},
		},
		{
			name:    "Unicode offsets",
			input:   "日本語",
			minGram: 2,
			maxGram: 2,
			expectedTokens: []struct {
				term  string
				start int
				end   int
			}{
				{"日本", 0, 2},
				{"本語", 1, 3},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer := NewNGramTokenizer(tt.minGram, tt.maxGram)
			err := tokenizer.SetReader(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("SetReader failed: %v", err)
			}

			i := 0
			for {
				hasToken, err := tokenizer.IncrementToken()
				if err != nil {
					t.Fatalf("IncrementToken failed: %v", err)
				}
				if !hasToken {
					break
				}

				if i >= len(tt.expectedTokens) {
					t.Errorf("More tokens than expected")
					break
				}

				termAttr := tokenizer.GetAttributeSource().GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
				offsetAttr := tokenizer.GetAttributeSource().GetAttributeByType(reflect.TypeOf(&offsetAttribute{}))

				cta, ok1 := termAttr.(CharTermAttribute)
				oa, ok2 := offsetAttr.(OffsetAttribute)
				if !ok1 || !ok2 {
					t.Fatalf("Failed to cast attributes")
				}

				if cta.String() != tt.expectedTokens[i].term {
					t.Errorf("Token %d: expected term %q, got %q", i, tt.expectedTokens[i].term, cta.String())
				}
				if oa.StartOffset() != tt.expectedTokens[i].start {
					t.Errorf("Token %d: expected start offset %d, got %d", i, tt.expectedTokens[i].start, oa.StartOffset())
				}
				if oa.EndOffset() != tt.expectedTokens[i].end {
					t.Errorf("Token %d: expected end offset %d, got %d", i, tt.expectedTokens[i].end, oa.EndOffset())
				}

				i++
			}

			if i != len(tt.expectedTokens) {
				t.Errorf("Expected %d tokens, got %d", len(tt.expectedTokens), i)
			}
		})
	}
}

func TestNGramTokenizer_PositionIncrement(t *testing.T) {
	tokenizer := NewNGramTokenizer(2, 2)
	err := tokenizer.SetReader(strings.NewReader("abcd"))
	if err != nil {
		t.Fatalf("SetReader failed: %v", err)
	}

	// All tokens should have position increment of 1
	expectedIncrements := []int{1, 1, 1}

	i := 0
	for {
		hasToken, err := tokenizer.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken failed: %v", err)
		}
		if !hasToken {
			break
		}

		if i >= len(expectedIncrements) {
			t.Errorf("More tokens than expected")
			break
		}

		posIncrAttr := tokenizer.GetAttributeSource().GetAttributeByType(reflect.TypeOf(&positionIncrementAttribute{}))
		pia, ok := posIncrAttr.(PositionIncrementAttribute)
		if !ok {
			t.Fatalf("Failed to cast PositionIncrementAttribute")
		}

		if pia.GetPositionIncrement() != expectedIncrements[i] {
			t.Errorf("Token %d: expected position increment %d, got %d",
				i, expectedIncrements[i], pia.GetPositionIncrement())
		}

		i++
	}
}

func TestNGramTokenizer_Reset(t *testing.T) {
	tokenizer := NewNGramTokenizer(2, 2)

	// First run
	err := tokenizer.SetReader(strings.NewReader("abc"))
	if err != nil {
		t.Fatalf("SetReader failed: %v", err)
	}

	tokens1 := []string{}
	for {
		hasToken, _ := tokenizer.IncrementToken()
		if !hasToken {
			break
		}
		termAttr := tokenizer.GetAttributeSource().GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
		if cta, ok := termAttr.(CharTermAttribute); ok {
			tokens1 = append(tokens1, cta.String())
		}
	}

	// Reset and second run
	err = tokenizer.Reset()
	if err != nil {
		t.Fatalf("Reset failed: %v", err)
	}

	err = tokenizer.SetReader(strings.NewReader("xyz"))
	if err != nil {
		t.Fatalf("SetReader failed: %v", err)
	}

	tokens2 := []string{}
	for {
		hasToken, _ := tokenizer.IncrementToken()
		if !hasToken {
			break
		}
		termAttr := tokenizer.GetAttributeSource().GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
		if cta, ok := termAttr.(CharTermAttribute); ok {
			tokens2 = append(tokens2, cta.String())
		}
	}

	// Verify different outputs
	expected1 := []string{"ab", "bc"}
	expected2 := []string{"xy", "yz"}

	if len(tokens1) != len(expected1) {
		t.Errorf("First run: expected %v, got %v", expected1, tokens1)
	}
	if len(tokens2) != len(expected2) {
		t.Errorf("Second run: expected %v, got %v", expected2, tokens2)
	}

	for i := range tokens1 {
		if tokens1[i] != expected1[i] {
			t.Errorf("First run token %d: expected %q, got %q", i, expected1[i], tokens1[i])
		}
	}
	for i := range tokens2 {
		if tokens2[i] != expected2[i] {
			t.Errorf("Second run token %d: expected %q, got %q", i, expected2[i], tokens2[i])
		}
	}
}

func TestNGramTokenizer_End(t *testing.T) {
	tokenizer := NewNGramTokenizer(2, 2)
	input := "abcd"
	err := tokenizer.SetReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("SetReader failed: %v", err)
	}

	// Consume all tokens
	for {
		hasToken, err := tokenizer.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken failed: %v", err)
		}
		if !hasToken {
			break
		}
	}

	// Call End
	err = tokenizer.End()
	if err != nil {
		t.Fatalf("End failed: %v", err)
	}

	// Check that End sets the final offset
	offsetAttr := tokenizer.GetAttributeSource().GetAttributeByType(reflect.TypeOf(&offsetAttribute{}))
	oa, ok := offsetAttr.(OffsetAttribute)
	if !ok {
		t.Fatalf("Failed to cast OffsetAttribute")
	}

	// End should set end offset to the length of the input in characters
	if oa.EndOffset() != len([]rune(input)) {
		t.Errorf("Expected end offset %d, got %d", len([]rune(input)), oa.EndOffset())
	}
}

func TestNGramTokenizer_InvalidParameters(t *testing.T) {
	tests := []struct {
		name    string
		minGram int
		maxGram int
	}{
		{
			name:    "minGram less than 1",
			minGram: 0,
			maxGram: 2,
		},
		{
			name:    "minGram negative",
			minGram: -1,
			maxGram: 2,
		},
		{
			name:    "maxGram less than minGram",
			minGram: 3,
			maxGram: 2,
		},
		{
			name:    "maxGram zero",
			minGram: 1,
			maxGram: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer := NewNGramTokenizer(tt.minGram, tt.maxGram)
			if tokenizer != nil {
				t.Errorf("Expected nil tokenizer for invalid parameters minGram=%d, maxGram=%d", tt.minGram, tt.maxGram)
			}
		})
	}
}

func TestNGramTokenizer_Getters(t *testing.T) {
	tokenizer := NewNGramTokenizer(2, 5)
	if tokenizer == nil {
		t.Fatal("Failed to create NGramTokenizer")
	}

	if tokenizer.GetMinGram() != 2 {
		t.Errorf("Expected GetMinGram() to return 2, got %d", tokenizer.GetMinGram())
	}

	if tokenizer.GetMaxGram() != 5 {
		t.Errorf("Expected GetMaxGram() to return 5, got %d", tokenizer.GetMaxGram())
	}
}

func TestNGramTokenizer_LargeInput(t *testing.T) {
	// Test with a larger input to ensure performance is reasonable
	input := "the quick brown fox jumps over the lazy dog"
	tokenizer := NewNGramTokenizer(3, 3)
	err := tokenizer.SetReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("SetReader failed: %v", err)
	}

	tokenCount := 0
	for {
		hasToken, err := tokenizer.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken failed: %v", err)
		}
		if !hasToken {
			break
		}
		tokenCount++
	}

	// For input of length N, we get N-2 trigrams
	runes := []rune(input)
	expectedCount := len(runes) - 2
	if tokenCount != expectedCount {
		t.Errorf("Expected %d tokens for input of length %d, got %d", expectedCount, len(runes), tokenCount)
	}
}

func TestNGramTokenizer_Newlines(t *testing.T) {
	// Test that newlines are treated as regular characters
	input := "a\nb"
	tokenizer := NewNGramTokenizer(2, 2)
	err := tokenizer.SetReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("SetReader failed: %v", err)
	}

	expected := []string{"a\n", "\nb"}

	var tokens []string
	for {
		hasToken, err := tokenizer.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken failed: %v", err)
		}
		if !hasToken {
			break
		}

		termAttr := tokenizer.GetAttributeSource().GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
		if cta, ok := termAttr.(CharTermAttribute); ok {
			tokens = append(tokens, cta.String())
		}
	}

	if len(tokens) != len(expected) {
		t.Errorf("Expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
		return
	}

	for i := range tokens {
		if tokens[i] != expected[i] {
			t.Errorf("Token %d: expected %q, got %q", i, expected[i], tokens[i])
		}
	}
}
