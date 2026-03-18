// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"strings"
	"testing"
)

// TestNGramFilter_Basic tests basic n-gram generation.
// Source: TestNGramTokenFilter.testBasic()
// Purpose: Tests that n-grams are generated correctly.
func TestNGramFilter_Basic(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		minGram  int
		maxGram  int
		expected []string
	}{
		{
			name:     "Single character token with min=1 max=1",
			input:    "a",
			minGram:  1,
			maxGram:  1,
			expected: []string{"a"},
		},
		{
			name:     "Two character token with min=1 max=2",
			input:    "ab",
			minGram:  1,
			maxGram:  2,
			expected: []string{"a", "ab", "b"},
		},
		{
			name:     "Three character token with min=2 max=3",
			input:    "abc",
			minGram:  2,
			maxGram:  3,
			expected: []string{"ab", "abc", "bc"},
		},
		{
			name:     "Hello with min=2 max=3",
			input:    "hello",
			minGram:  2,
			maxGram:  3,
			expected: []string{"he", "hel", "el", "ell", "ll", "llo", "lo"},
		},
		{
			name:     "Hello with min=1 max=2",
			input:    "hello",
			minGram:  1,
			maxGram:  2,
			expected: []string{"h", "he", "e", "el", "l", "ll", "l", "lo", "o"},
		},
		{
			name:     "Multiple tokens",
			input:    "hi there",
			minGram:  2,
			maxGram:  3,
			expected: []string{"hi", "th", "the", "he", "her", "er", "ere", "re"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			ngramFilter := NewNGramFilter(tokenizer, tc.minGram, tc.maxGram)
			defer ngramFilter.Close()

			var tokens []string
			for {
				hasToken, err := ngramFilter.IncrementToken()
				if err != nil {
					t.Fatalf("Error incrementing token: %v", err)
				}
				if !hasToken {
					break
				}
				if attr := ngramFilter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
					if termAttr, ok := attr.(CharTermAttribute); ok {
						tokens = append(tokens, termAttr.String())
					}
				}
			}

			if !reflect.DeepEqual(tokens, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, tokens)
			}
		})
	}
}

// TestNGramFilter_PositionIncrement tests position increment handling.
// Source: TestNGramTokenFilter.testPositionIncrement()
// Purpose: Tests that position increments are handled correctly.
func TestNGramFilter_PositionIncrement(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		minGram  int
		maxGram  int
		expected []int
	}{
		{
			name:     "Single token with min=2 max=3",
			input:    "hello",
			minGram:  2,
			maxGram:  3,
			expected: []int{1, 0, 0, 0, 0, 0, 0},
		},
		{
			name:     "Two tokens with min=2 max=2",
			input:    "hi there",
			minGram:  2,
			maxGram:  2,
			expected: []int{1, 1, 0, 0, 0},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			ngramFilter := NewNGramFilter(tokenizer, tc.minGram, tc.maxGram)
			defer ngramFilter.Close()

			var positions []int
			for {
				hasToken, err := ngramFilter.IncrementToken()
				if err != nil {
					t.Fatalf("Error incrementing token: %v", err)
				}
				if !hasToken {
					break
				}
				if attr := ngramFilter.GetAttributeSource().GetAttribute("PositionIncrementAttribute"); attr != nil {
					if posAttr, ok := attr.(PositionIncrementAttribute); ok {
						positions = append(positions, posAttr.GetPositionIncrement())
					}
				}
			}

			if !reflect.DeepEqual(positions, tc.expected) {
				t.Errorf("Expected positions %v, got %v", tc.expected, positions)
			}
		})
	}
}

// TestNGramFilter_Offset tests offset handling.
// Source: TestNGramTokenFilter.testOffset()
// Purpose: Tests that offsets are preserved correctly.
func TestNGramFilter_Offset(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		minGram        int
		maxGram        int
		expectedTokens []string
		expectedStarts []int
		expectedEnds   []int
	}{
		{
			name:           "Hello with min=2 max=3",
			input:          "hello",
			minGram:        2,
			maxGram:        3,
			expectedTokens: []string{"he", "hel", "el", "ell", "ll", "llo", "lo"},
			expectedStarts: []int{0, 0, 1, 1, 2, 2, 3},
			expectedEnds:   []int{2, 3, 3, 4, 4, 5, 5},
		},
		{
			name:           "Two tokens with min=2 max=2",
			input:          "hi there",
			minGram:        2,
			maxGram:        2,
			expectedTokens: []string{"hi", "th", "he", "er", "re"},
			expectedStarts: []int{0, 3, 4, 5, 6},
			expectedEnds:   []int{2, 5, 6, 7, 8},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			ngramFilter := NewNGramFilter(tokenizer, tc.minGram, tc.maxGram)
			defer ngramFilter.Close()

			type tokenInfo struct {
				text        string
				startOffset int
				endOffset   int
			}

			var tokens []tokenInfo
			for {
				hasToken, err := ngramFilter.IncrementToken()
				if err != nil {
					t.Fatalf("Error incrementing token: %v", err)
				}
				if !hasToken {
					break
				}

				var info tokenInfo
				if attr := ngramFilter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
					if termAttr, ok := attr.(CharTermAttribute); ok {
						info.text = termAttr.String()
					}
				}
				if attr := ngramFilter.GetAttributeSource().GetAttribute("OffsetAttribute"); attr != nil {
					if offsetAttr, ok := attr.(OffsetAttribute); ok {
						info.startOffset = offsetAttr.StartOffset()
						info.endOffset = offsetAttr.EndOffset()
					}
				}
				tokens = append(tokens, info)
			}

			if len(tokens) != len(tc.expectedTokens) {
				t.Fatalf("Expected %d tokens, got %d", len(tc.expectedTokens), len(tokens))
			}

			for i, token := range tokens {
				if token.text != tc.expectedTokens[i] {
					t.Errorf("Token %d: expected text %s, got %s", i, tc.expectedTokens[i], token.text)
				}
				if token.startOffset != tc.expectedStarts[i] {
					t.Errorf("Token %d (%s): expected start offset %d, got %d", i, token.text, tc.expectedStarts[i], token.startOffset)
				}
				if token.endOffset != tc.expectedEnds[i] {
					t.Errorf("Token %d (%s): expected end offset %d, got %d", i, token.text, tc.expectedEnds[i], token.endOffset)
				}
			}
		})
	}
}

// TestNGramFilter_EmptyInput tests empty input handling.
// Source: TestNGramTokenFilter.testEmpty()
// Purpose: Tests that empty input is handled correctly.
func TestNGramFilter_EmptyInput(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader(""))

	ngramFilter := NewNGramFilter(tokenizer, 2, 3)
	defer ngramFilter.Close()

	tokenCount := 0
	for {
		hasToken, err := ngramFilter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		tokenCount++
	}

	if tokenCount != 0 {
		t.Errorf("Expected 0 tokens for empty input, got %d", tokenCount)
	}
}

// TestNGramFilter_ShortToken tests handling of tokens shorter than minGram.
// Source: TestNGramTokenFilter.testShortToken()
// Purpose: Tests that tokens shorter than minGram are filtered out.
func TestNGramFilter_ShortToken(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		minGram  int
		maxGram  int
		expected []string
	}{
		{
			name:     "Token shorter than minGram",
			input:    "a",
			minGram:  2,
			maxGram:  3,
			expected: []string{},
		},
		{
			name:     "Mixed length tokens",
			input:    "a bc def",
			minGram:  2,
			maxGram:  3,
			expected: []string{"bc", "de", "def", "ef"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			ngramFilter := NewNGramFilter(tokenizer, tc.minGram, tc.maxGram)
			defer ngramFilter.Close()

			tokens := []string{}
			for {
				hasToken, err := ngramFilter.IncrementToken()
				if err != nil {
					t.Fatalf("Error incrementing token: %v", err)
				}
				if !hasToken {
					break
				}
				if attr := ngramFilter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
					if termAttr, ok := attr.(CharTermAttribute); ok {
						tokens = append(tokens, termAttr.String())
					}
				}
			}

			if !reflect.DeepEqual(tokens, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, tokens)
			}
		})
	}
}

// TestNGramFilter_MinMaxValidation tests min/max gram size validation.
// Source: TestNGramTokenFilter.testMinMaxValidation()
// Purpose: Tests that min and max gram sizes are validated.
func TestNGramFilter_MinMaxValidation(t *testing.T) {
	tests := []struct {
		name        string
		minGram     int
		maxGram     int
		expectedMin int
		expectedMax int
	}{
		{
			name:        "Valid min and max",
			minGram:     2,
			maxGram:     4,
			expectedMin: 2,
			expectedMax: 4,
		},
		{
			name:        "Min less than 1",
			minGram:     0,
			maxGram:     3,
			expectedMin: 1,
			expectedMax: 3,
		},
		{
			name:        "Max less than min",
			minGram:     3,
			maxGram:     2,
			expectedMin: 3,
			expectedMax: 3,
		},
		{
			name:        "Negative min",
			minGram:     -1,
			maxGram:     3,
			expectedMin: 1,
			expectedMax: 3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader("test"))

			ngramFilter := NewNGramFilter(tokenizer, tc.minGram, tc.maxGram)
			defer ngramFilter.Close()

			if ngramFilter.GetMinGram() != tc.expectedMin {
				t.Errorf("Expected minGram %d, got %d", tc.expectedMin, ngramFilter.GetMinGram())
			}
			if ngramFilter.GetMaxGram() != tc.expectedMax {
				t.Errorf("Expected maxGram %d, got %d", tc.expectedMax, ngramFilter.GetMaxGram())
			}
		})
	}
}

// TestNGramFilter_EndMethod tests the End() method.
// Source: TestNGramTokenFilter.testEnd()
// Purpose: Tests that End() is properly propagated.
func TestNGramFilter_EndMethod(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("test"))

	ngramFilter := NewNGramFilter(tokenizer, 2, 3)
	defer ngramFilter.Close()

	for {
		hasToken, err := ngramFilter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
	}

	err := ngramFilter.End()
	if err != nil {
		t.Errorf("End() returned error: %v", err)
	}
}

// TestNGramFilter_CloseMethod tests the Close() method.
// Source: TestNGramTokenFilter.testClose()
// Purpose: Tests that Close() is properly propagated.
func TestNGramFilter_CloseMethod(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("test"))

	ngramFilter := NewNGramFilter(tokenizer, 2, 3)

	err := ngramFilter.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}

// TestNGramFilter_Chaining tests chaining with other filters.
// Source: TestNGramTokenFilter.testChaining()
// Purpose: Tests that NGramFilter works properly in filter chains.
func TestNGramFilter_Chaining(t *testing.T) {
	input := "HELLO world"

	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader(input))

	lowerFilter := NewLowerCaseFilter(tokenizer)
	ngramFilter := NewNGramFilter(lowerFilter, 2, 3)
	defer ngramFilter.Close()

	var tokens []string
	for {
		hasToken, err := ngramFilter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := ngramFilter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				tokens = append(tokens, termAttr.String())
			}
		}
	}

	// "hello" -> "he", "hel", "el", "ell", "ll", "llo", "lo"
	// "world" -> "wo", "wor", "or", "orl", "rl", "rld", "ld"
	expected := []string{"he", "hel", "el", "ell", "ll", "llo", "lo", "wo", "wor", "or", "orl", "rl", "rld", "ld"}
	if !reflect.DeepEqual(tokens, expected) {
		t.Errorf("Expected %v, got %v", expected, tokens)
	}
}

// TestNGramFilter_Factory tests the NGramFilterFactory.
// Source: TestNGramTokenFilter.testFactory()
// Purpose: Tests that the factory creates filters correctly.
func TestNGramFilter_Factory(t *testing.T) {
	factory := NewNGramFilterFactory(2, 4)

	if factory.GetMinGram() != 2 {
		t.Errorf("Expected minGram 2, got %d", factory.GetMinGram())
	}
	if factory.GetMaxGram() != 4 {
		t.Errorf("Expected maxGram 4, got %d", factory.GetMaxGram())
	}

	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("test"))

	filter := factory.Create(tokenizer)
	defer filter.Close()

	ngramFilter, ok := filter.(*NGramFilter)
	if !ok {
		t.Fatalf("Factory did not create an NGramFilter")
	}

	if ngramFilter.GetMinGram() != 2 {
		t.Errorf("Expected filter minGram 2, got %d", ngramFilter.GetMinGram())
	}
	if ngramFilter.GetMaxGram() != 4 {
		t.Errorf("Expected filter maxGram 4, got %d", ngramFilter.GetMaxGram())
	}
}

// TestNGramFilter_LargeMaxGram tests handling when maxGram exceeds token length.
// Source: TestNGramTokenFilter.testLargeMaxGram()
// Purpose: Tests that maxGram larger than token length is handled correctly.
func TestNGramFilter_LargeMaxGram(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("hi"))

	ngramFilter := NewNGramFilter(tokenizer, 1, 10)
	defer ngramFilter.Close()

	var tokens []string
	for {
		hasToken, err := ngramFilter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := ngramFilter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				tokens = append(tokens, termAttr.String())
			}
		}
	}

	// "hi" with min=1 max=10 should generate: "h", "hi", "i"
	expected := []string{"h", "hi", "i"}
	if !reflect.DeepEqual(tokens, expected) {
		t.Errorf("Expected %v, got %v", expected, tokens)
	}
}

// TestNGramFilter_Unicode tests Unicode handling.
// Source: TestNGramTokenFilter.testUnicode()
// Purpose: Tests proper handling of Unicode characters.
func TestNGramFilter_Unicode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		minGram  int
		maxGram  int
		expected []string
	}{
		{
			name:     "Unicode characters",
			input:    "café",
			minGram:  2,
			maxGram:  3,
			expected: []string{"ca", "caf", "af", "afé", "fé"},
		},
		{
			name:     "Chinese characters",
			input:    "你好",
			minGram:  1,
			maxGram:  2,
			expected: []string{"你", "你好", "好"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			ngramFilter := NewNGramFilter(tokenizer, tc.minGram, tc.maxGram)
			defer ngramFilter.Close()

			var tokens []string
			for {
				hasToken, err := ngramFilter.IncrementToken()
				if err != nil {
					t.Fatalf("Error incrementing token: %v", err)
				}
				if !hasToken {
					break
				}
				if attr := ngramFilter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
					if termAttr, ok := attr.(CharTermAttribute); ok {
						tokens = append(tokens, termAttr.String())
					}
				}
			}

			if !reflect.DeepEqual(tokens, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, tokens)
			}
		})
	}
}

// TestNGramFilter_SingleGramSize tests when minGram equals maxGram.
// Source: TestNGramTokenFilter.testSingleGramSize()
// Purpose: Tests fixed-size n-gram generation.
func TestNGramFilter_SingleGramSize(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("hello"))

	ngramFilter := NewNGramFilter(tokenizer, 3, 3)
	defer ngramFilter.Close()

	var tokens []string
	for {
		hasToken, err := ngramFilter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := ngramFilter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				tokens = append(tokens, termAttr.String())
			}
		}
	}

	// "hello" with min=3 max=3 should generate: "hel", "ell", "llo"
	expected := []string{"hel", "ell", "llo"}
	if !reflect.DeepEqual(tokens, expected) {
		t.Errorf("Expected %v, got %v", expected, tokens)
	}
}

// TestNGramFilter_GetInput tests the GetInput method.
// Source: TestNGramTokenFilter.testGetInput()
// Purpose: Tests that GetInput returns the wrapped input.
func TestNGramFilter_GetInput(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("test"))

	ngramFilter := NewNGramFilter(tokenizer, 2, 3)
	defer ngramFilter.Close()

	input := ngramFilter.GetInput()
	if input == nil {
		t.Error("GetInput() returned nil")
	}
	if input != tokenizer {
		t.Error("GetInput() did not return the original tokenizer")
	}
}
