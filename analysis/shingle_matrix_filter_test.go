// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"strings"
	"testing"
)

// TestShingleMatrixFilter_Basic tests basic shingle generation with default settings.
// Source: TestShingleMatrixFilter.testShingleGeneration()
// Purpose: Tests that shingles are correctly generated from tokens in matrix pattern.
// Matrix pattern: row by row (all unigrams, then all bigrams)
func TestShingleMatrixFilter_Basic(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    []string
		expectedPos []int
	}{
		{
			name:        "Simple sentence",
			input:       "please divide this sentence",
			// Matrix pattern: unigrams first, then bigrams
			expected:    []string{"please", "divide", "this", "sentence", "please divide", "divide this", "this sentence"},
			expectedPos: []int{1, 0, 0, 0, 0, 0, 0},
		},
		{
			name:        "Two words",
			input:       "hello world",
			expected:    []string{"hello", "world", "hello world"},
			expectedPos: []int{1, 0, 0},
		},
		{
			name:        "Single word",
			input:       "hello",
			expected:    []string{"hello"},
			expectedPos: []int{1},
		},
		{
			name:        "Three words",
			input:       "a b c",
			expected:    []string{"a", "b", "c", "a b", "b c"},
			expectedPos: []int{1, 0, 0, 0, 0},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			filter := NewShingleMatrixFilter(tokenizer)
			defer filter.Close()

			var tokens []string
			var positions []int

			for {
				hasToken, err := filter.IncrementToken()
				if err != nil {
					t.Fatalf("Error incrementing token: %v", err)
				}
				if !hasToken {
					break
				}

				if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
					if termAttr, ok := attr.(CharTermAttribute); ok {
						tokens = append(tokens, termAttr.String())
					}
				}

				if attr := filter.GetAttributeSource().GetAttribute("PositionIncrementAttribute"); attr != nil {
					if posAttr, ok := attr.(PositionIncrementAttribute); ok {
						positions = append(positions, posAttr.GetPositionIncrement())
					}
				}
			}

			if !reflect.DeepEqual(tokens, tc.expected) {
				t.Errorf("Expected tokens %v, got %v", tc.expected, tokens)
			}

			if !reflect.DeepEqual(positions, tc.expectedPos) {
				t.Errorf("Expected positions %v, got %v", tc.expectedPos, positions)
			}
		})
	}
}

// TestShingleMatrixFilter_NoUnigrams tests shingle generation without unigrams.
// Source: TestShingleMatrixFilter.testShinglesOnly()
// Purpose: Tests that only shingles are output when outputUnigrams=false.
func TestShingleMatrixFilter_NoUnigrams(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Simple sentence",
			input:    "please divide this sentence",
			expected: []string{"please divide", "divide this", "this sentence"},
		},
		{
			name:     "Two words",
			input:    "hello world",
			expected: []string{"hello world"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			filter := NewShingleMatrixFilter(tokenizer)
			filter.SetOutputUnigrams(false)
			defer filter.Close()

			var tokens []string
			for {
				hasToken, err := filter.IncrementToken()
				if err != nil {
					t.Fatalf("Error incrementing token: %v", err)
				}
				if !hasToken {
					break
				}

				if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
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

// TestShingleMatrixFilter_LargerSize tests shingle generation with larger max size.
// Source: TestShingleMatrixFilter.testMaxShingleSize()
// Purpose: Tests shingles with maxShingleSize > 2 in matrix pattern.
func TestShingleMatrixFilter_LargerSize(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		maxSize     int
		expected    []string
		expectedPos []int
	}{
		{
			name:        "Size 3",
			input:       "a b c d",
			maxSize:     3,
			// Matrix: unigrams, bigrams, trigrams
			expected:    []string{"a", "b", "c", "d", "a b", "b c", "c d", "a b c", "b c d"},
			expectedPos: []int{1, 0, 0, 0, 0, 0, 0, 0, 0},
		},
		{
			name:        "Size 4",
			input:       "a b c d e",
			maxSize:     4,
			// Matrix: unigrams, bigrams, trigrams, 4-grams
			expected:    []string{"a", "b", "c", "d", "e", "a b", "b c", "c d", "d e", "a b c", "b c d", "c d e", "a b c d", "b c d e"},
			expectedPos: []int{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			filter := NewShingleMatrixFilter(tokenizer)
			filter.SetMaxShingleSize(tc.maxSize)
			defer filter.Close()

			var tokens []string
			var positions []int

			for {
				hasToken, err := filter.IncrementToken()
				if err != nil {
					t.Fatalf("Error incrementing token: %v", err)
				}
				if !hasToken {
					break
				}

				if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
					if termAttr, ok := attr.(CharTermAttribute); ok {
						tokens = append(tokens, termAttr.String())
					}
				}

				if attr := filter.GetAttributeSource().GetAttribute("PositionIncrementAttribute"); attr != nil {
					if posAttr, ok := attr.(PositionIncrementAttribute); ok {
						positions = append(positions, posAttr.GetPositionIncrement())
					}
				}
			}

			if !reflect.DeepEqual(tokens, tc.expected) {
				t.Errorf("Expected tokens %v, got %v", tc.expected, tokens)
			}

			if !reflect.DeepEqual(positions, tc.expectedPos) {
				t.Errorf("Expected positions %v, got %v", tc.expectedPos, positions)
			}
		})
	}
}

// TestShingleMatrixFilter_MinMaxSize tests shingle generation with custom min/max sizes.
// Source: TestShingleMatrixFilter.testMinMaxShingleSize()
// Purpose: Tests shingles with minShingleSize > 1.
func TestShingleMatrixFilter_MinMaxSize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		minSize  int
		maxSize  int
		expected []string
	}{
		{
			name:     "Min 2 Max 3",
			input:    "a b c d",
			minSize:  2,
			maxSize:  3,
			// Matrix: bigrams, trigrams (no unigrams)
			expected: []string{"a b", "b c", "c d", "a b c", "b c d"},
		},
		{
			name:     "Min 3 Max 3",
			input:    "a b c d e",
			minSize:  3,
			maxSize:  3,
			// Matrix: trigrams only
			expected: []string{"a b c", "b c d", "c d e"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			filter := NewShingleMatrixFilterWithSizes(tokenizer, tc.minSize, tc.maxSize)
			filter.SetOutputUnigrams(false)
			defer filter.Close()

			var tokens []string
			for {
				hasToken, err := filter.IncrementToken()
				if err != nil {
					t.Fatalf("Error incrementing token: %v", err)
				}
				if !hasToken {
					break
				}

				if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
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

// TestShingleMatrixFilter_TokenSeparator tests custom token separators.
// Source: TestShingleMatrixFilter.testTokenSeparator()
// Purpose: Tests that custom separators are correctly inserted.
func TestShingleMatrixFilter_TokenSeparator(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		separator string
		expected  []string
	}{
		{
			name:      "Underscore separator",
			input:     "hello world test",
			separator: "_",
			// Matrix: unigrams, then bigrams
			expected:  []string{"hello", "world", "test", "hello_world", "world_test"},
		},
		{
			name:      "Empty separator",
			input:     "hello world test",
			separator: "",
			expected:  []string{"hello", "world", "test", "helloworld", "worldtest"},
		},
		{
			name:      "Hyphen separator",
			input:     "hello world test",
			separator: "-",
			expected:  []string{"hello", "world", "test", "hello-world", "world-test"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			filter := NewShingleMatrixFilter(tokenizer)
			filter.SetTokenSeparator(tc.separator)
			defer filter.Close()

			var tokens []string
			for {
				hasToken, err := filter.IncrementToken()
				if err != nil {
					t.Fatalf("Error incrementing token: %v", err)
				}
				if !hasToken {
					break
				}

				if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
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

// TestShingleMatrixFilter_Offsets tests that offsets are correctly preserved.
// Source: TestShingleMatrixFilter.testOffsets()
// Purpose: Tests that start/end offsets are correct for shingles.
func TestShingleMatrixFilter_Offsets(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("hello world"))

	filter := NewShingleMatrixFilter(tokenizer)
	defer filter.Close()

	type tokenInfo struct {
		text        string
		startOffset int
		endOffset   int
	}

	var tokens []tokenInfo
	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}

		var info tokenInfo
		if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				info.text = termAttr.String()
			}
		}
		if attr := filter.GetAttributeSource().GetAttribute("OffsetAttribute"); attr != nil {
			if offsetAttr, ok := attr.(OffsetAttribute); ok {
				info.startOffset = offsetAttr.StartOffset()
				info.endOffset = offsetAttr.EndOffset()
			}
		}
		tokens = append(tokens, info)
	}

	if len(tokens) != 3 {
		t.Fatalf("Expected 3 tokens, got %d", len(tokens))
	}

	// Matrix order: "hello" [0, 5], "world" [6, 11], "hello world" [0, 11]
	if tokens[0].text != "hello" || tokens[0].startOffset != 0 || tokens[0].endOffset != 5 {
		t.Errorf("First token: expected hello [0,5], got %s [%d,%d]",
			tokens[0].text, tokens[0].startOffset, tokens[0].endOffset)
	}

	if tokens[1].text != "world" || tokens[1].startOffset != 6 || tokens[1].endOffset != 11 {
		t.Errorf("Second token: expected world [6,11], got %s [%d,%d]",
			tokens[1].text, tokens[1].startOffset, tokens[1].endOffset)
	}

	if tokens[2].text != "hello world" || tokens[2].startOffset != 0 || tokens[2].endOffset != 11 {
		t.Errorf("Third token: expected 'hello world' [0,11], got %s [%d,%d]",
			tokens[2].text, tokens[2].startOffset, tokens[2].endOffset)
	}
}

// TestShingleMatrixFilter_EmptyInput tests empty input handling.
// Source: TestShingleMatrixFilter.testEmpty()
// Purpose: Tests that empty input produces no tokens.
func TestShingleMatrixFilter_EmptyInput(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader(""))

	filter := NewShingleMatrixFilter(tokenizer)
	defer filter.Close()

	tokenCount := 0
	for {
		hasToken, err := filter.IncrementToken()
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

// TestShingleMatrixFilter_SingleToken tests input with a single token.
// Source: TestShingleMatrixFilter.testSingleToken()
// Purpose: Tests that single token input works correctly.
func TestShingleMatrixFilter_SingleToken(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("hello"))

	filter := NewShingleMatrixFilter(tokenizer)
	defer filter.Close()

	var tokens []string
	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				tokens = append(tokens, termAttr.String())
			}
		}
	}

	expected := []string{"hello"}
	if !reflect.DeepEqual(tokens, expected) {
		t.Errorf("Expected %v, got %v", expected, tokens)
	}
}

// TestShingleMatrixFilter_Chaining tests chaining with other filters.
// Source: TestShingleMatrixFilter.testChaining()
// Purpose: Tests that ShingleMatrixFilter works properly in filter chains.
func TestShingleMatrixFilter_Chaining(t *testing.T) {
	input := "HELLO world Test"

	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader(input))

	lowerFilter := NewLowerCaseFilter(tokenizer)
	shingleFilter := NewShingleMatrixFilter(lowerFilter)
	defer shingleFilter.Close()

	var tokens []string
	for {
		hasToken, err := shingleFilter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := shingleFilter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				tokens = append(tokens, termAttr.String())
			}
		}
	}

	// Matrix: unigrams, then bigrams
	expected := []string{"hello", "world", "test", "hello world", "world test"}
	if !reflect.DeepEqual(tokens, expected) {
		t.Errorf("Expected %v, got %v", expected, tokens)
	}
}

// TestShingleMatrixFilter_EndMethod tests the End() method.
// Source: TestShingleMatrixFilter.testEnd()
// Purpose: Tests that End() is properly propagated.
func TestShingleMatrixFilter_EndMethod(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("hello world"))

	filter := NewShingleMatrixFilter(tokenizer)
	defer filter.Close()

	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
	}

	err := filter.End()
	if err != nil {
		t.Errorf("End() returned error: %v", err)
	}
}

// TestShingleMatrixFilter_Reset tests the Reset() method.
// Source: TestShingleMatrixFilter.testReset()
// Purpose: Tests that Reset() properly clears state.
func TestShingleMatrixFilter_Reset(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()

	// First run
	tokenizer.SetReader(strings.NewReader("hello world"))
	filter := NewShingleMatrixFilter(tokenizer)

	var tokens1 []string
	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				tokens1 = append(tokens1, termAttr.String())
			}
		}
	}

	// Reset and run again with different input
	filter.Reset()
	tokenizer.SetReader(strings.NewReader("a b"))

	var tokens2 []string
	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				tokens2 = append(tokens2, termAttr.String())
			}
		}
	}

	// Matrix pattern
	expected1 := []string{"hello", "world", "hello world"}
	expected2 := []string{"a", "b", "a b"}

	if !reflect.DeepEqual(tokens1, expected1) {
		t.Errorf("First run: expected %v, got %v", expected1, tokens1)
	}

	if !reflect.DeepEqual(tokens2, expected2) {
		t.Errorf("Second run: expected %v, got %v", expected2, tokens2)
	}

	filter.Close()
}

// TestShingleMatrixFilter_Factory tests the ShingleMatrixFilterFactory.
// Source: TestShingleMatrixFilter.testFactory()
// Purpose: Tests that the factory creates properly configured filters.
func TestShingleMatrixFilter_Factory(t *testing.T) {
	factory := NewShingleMatrixFilterFactoryWithSizes(2, 3)
	factory.SetTokenSeparator("_")
	factory.SetOutputUnigrams(false)

	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("a b c d"))

	filter := factory.Create(tokenizer).(*ShingleMatrixFilter)
	defer filter.Close()

	var tokens []string
	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				tokens = append(tokens, termAttr.String())
			}
		}
	}

	// Matrix: bigrams, trigrams
	expected := []string{"a_b", "b_c", "c_d", "a_b_c", "b_c_d"}
	if !reflect.DeepEqual(tokens, expected) {
		t.Errorf("Expected %v, got %v", expected, tokens)
	}
}

// TestShingleMatrixFilter_GettersSetters tests getter and setter methods.
// Source: TestShingleMatrixFilter.testGettersSetters()
// Purpose: Tests that configuration methods work correctly.
func TestShingleMatrixFilter_GettersSetters(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	filter := NewShingleMatrixFilter(tokenizer)
	defer filter.Close()

	// Test defaults
	if filter.GetMaxShingleSize() != 2 {
		t.Errorf("Expected default maxShingleSize=2, got %d", filter.GetMaxShingleSize())
	}
	if filter.GetMinShingleSize() != 2 {
		t.Errorf("Expected default minShingleSize=2, got %d", filter.GetMinShingleSize())
	}
	if filter.GetTokenSeparator() != " " {
		t.Errorf("Expected default tokenSeparator=' ', got '%s'", filter.GetTokenSeparator())
	}
	if !filter.IsOutputUnigrams() {
		t.Errorf("Expected default outputUnigrams=true")
	}

	// Test setters
	filter.SetMaxShingleSize(4)
	if filter.GetMaxShingleSize() != 4 {
		t.Errorf("Expected maxShingleSize=4, got %d", filter.GetMaxShingleSize())
	}

	filter.SetMinShingleSize(3)
	if filter.GetMinShingleSize() != 3 {
		t.Errorf("Expected minShingleSize=3, got %d", filter.GetMinShingleSize())
	}

	filter.SetTokenSeparator("-")
	if filter.GetTokenSeparator() != "-" {
		t.Errorf("Expected tokenSeparator='-', got '%s'", filter.GetTokenSeparator())
	}

	filter.SetOutputUnigrams(false)
	if filter.IsOutputUnigrams() {
		t.Errorf("Expected outputUnigrams=false")
	}
}

// TestShingleMatrixFilter_BoundaryConditions tests boundary conditions.
// Source: TestShingleMatrixFilter.testBoundaryConditions()
// Purpose: Tests edge cases and boundary conditions.
func TestShingleMatrixFilter_BoundaryConditions(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		minSize  int
		maxSize  int
		expected []string
	}{
		{
			name:     "Max size equals token count",
			input:    "a b",
			minSize:  2,
			maxSize:  2,
			expected: []string{"a", "b", "a b"},
		},
		{
			name:     "Max size exceeds token count",
			input:    "a b",
			minSize:  2,
			maxSize:  5,
			expected: []string{"a", "b", "a b"},
		},
		{
			name:     "Single token with min size 2",
			input:    "hello",
			minSize:  2,
			maxSize:  3,
			expected: []string{"hello"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			filter := NewShingleMatrixFilterWithSizes(tokenizer, tc.minSize, tc.maxSize)
			defer filter.Close()

			var tokens []string
			for {
				hasToken, err := filter.IncrementToken()
				if err != nil {
					t.Fatalf("Error incrementing token: %v", err)
				}
				if !hasToken {
					break
				}

				if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
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

// TestShingleMatrixFilter_LongTokens tests handling of long tokens.
// Source: TestShingleMatrixFilter.testLongTokens()
// Purpose: Tests that long tokens are handled correctly.
func TestShingleMatrixFilter_LongTokens(t *testing.T) {
	longToken1 := strings.Repeat("a", 100)
	longToken2 := strings.Repeat("b", 100)
	input := longToken1 + " " + longToken2

	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader(input))

	filter := NewShingleMatrixFilter(tokenizer)
	defer filter.Close()

	var tokens []string
	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				tokens = append(tokens, termAttr.String())
			}
		}
	}

	if len(tokens) != 3 {
		t.Fatalf("Expected 3 tokens, got %d", len(tokens))
	}

	if tokens[0] != longToken1 {
		t.Errorf("First token mismatch")
	}

	if tokens[1] != longToken2 {
		t.Errorf("Second token mismatch")
	}

	expectedShingle := longToken1 + " " + longToken2
	if tokens[2] != expectedShingle {
		t.Errorf("Shingle token mismatch: expected length %d, got length %d", len(expectedShingle), len(tokens[2]))
	}
}

// TestShingleMatrixFilter_SpecialCharacters tests handling of special characters.
// Source: TestShingleMatrixFilter.testSpecialCharacters()
// Purpose: Tests that special characters in tokens are preserved.
func TestShingleMatrixFilter_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Tokens with punctuation",
			input:    "hello, world! test.",
			// Matrix: unigrams, then bigrams
			expected: []string{"hello,", "world!", "test.", "hello, world!", "world! test."},
		},
		{
			name:     "Tokens with numbers",
			input:    "test123 abc456",
			expected: []string{"test123", "abc456", "test123 abc456"},
		},
		{
			name:     "Tokens with unicode",
			input:    "café résumé",
			expected: []string{"café", "résumé", "café résumé"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			filter := NewShingleMatrixFilter(tokenizer)
			defer filter.Close()

			var tokens []string
			for {
				hasToken, err := filter.IncrementToken()
				if err != nil {
					t.Fatalf("Error incrementing token: %v", err)
				}
				if !hasToken {
					break
				}

				if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
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

// TestShingleMatrixFilter_PositionIncrements tests position increment handling.
// Source: TestShingleMatrixFilter.testPositionIncrements()
// Purpose: Tests that position increments are correctly set.
func TestShingleMatrixFilter_PositionIncrements(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("a b c"))

	filter := NewShingleMatrixFilter(tokenizer)
	defer filter.Close()

	var positions []int
	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := filter.GetAttributeSource().GetAttribute("PositionIncrementAttribute"); attr != nil {
			if posAttr, ok := attr.(PositionIncrementAttribute); ok {
				positions = append(positions, posAttr.GetPositionIncrement())
			}
		}
	}

	// First token has position increment 1, rest have 0
	// Matrix: a, b, c, a b, b c (5 tokens)
	expected := []int{1, 0, 0, 0, 0}
	if !reflect.DeepEqual(positions, expected) {
		t.Errorf("Expected positions %v, got %v", expected, positions)
	}
}

// TestShingleMatrixFilter_NoUnigramsPositionIncrements tests position increments without unigrams.
// Source: TestShingleMatrixFilter.testNoUnigramsPositionIncrements()
// Purpose: Tests position increments when only outputting shingles.
func TestShingleMatrixFilter_NoUnigramsPositionIncrements(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("a b c"))

	filter := NewShingleMatrixFilter(tokenizer)
	filter.SetOutputUnigrams(false)
	defer filter.Close()

	var positions []int
	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := filter.GetAttributeSource().GetAttribute("PositionIncrementAttribute"); attr != nil {
			if posAttr, ok := attr.(PositionIncrementAttribute); ok {
				positions = append(positions, posAttr.GetPositionIncrement())
			}
		}
	}

	// With no unigrams, first shingle has position 1, rest have 0
	expected := []int{1, 0}
	if !reflect.DeepEqual(positions, expected) {
		t.Errorf("Expected positions %v, got %v", expected, positions)
	}
}

// TestShingleMatrixFilter_PositionLength tests position length attribute.
// Source: TestShingleMatrixFilter.testPositionLength()
// Purpose: Tests that position length is correctly set for shingles.
func TestShingleMatrixFilter_PositionLength(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("a b c"))

	filter := NewShingleMatrixFilter(tokenizer)
	filter.SetMaxShingleSize(3)
	defer filter.Close()

	type tokenInfo struct {
		text           string
		positionLength int
	}

	var tokens []tokenInfo
	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}

		var info tokenInfo
		if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				info.text = termAttr.String()
			}
		}
		if attr := filter.GetAttributeSource().GetAttribute("PositionLengthAttribute"); attr != nil {
			if posLenAttr, ok := attr.(*PositionLengthAttribute); ok {
				info.positionLength = posLenAttr.GetPositionLength()
			}
		}
		tokens = append(tokens, info)
	}

	// Matrix order: a (1), b (1), c (1), a b (2), b c (2), a b c (3)
	expected := []tokenInfo{
		{"a", 1},
		{"b", 1},
		{"c", 1},
		{"a b", 2},
		{"b c", 2},
		{"a b c", 3},
	}

	if len(tokens) != len(expected) {
		t.Fatalf("Expected %d tokens, got %d", len(expected), len(tokens))
	}

	for i, exp := range expected {
		if tokens[i].text != exp.text || tokens[i].positionLength != exp.positionLength {
			t.Errorf("Token %d: expected %v (len=%d), got %v (len=%d)",
				i, exp.text, exp.positionLength, tokens[i].text, tokens[i].positionLength)
		}
	}
}

// TestShingleMatrixFilter_MatrixPattern tests the matrix pattern generation.
// Source: TestShingleMatrixFilter.testMatrixPattern()
// Purpose: Tests that tokens are generated in proper matrix order.
func TestShingleMatrixFilter_MatrixPattern(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("a b c d"))

	filter := NewShingleMatrixFilter(tokenizer)
	filter.SetMaxShingleSize(3)
	defer filter.Close()

	var tokens []string
	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				tokens = append(tokens, termAttr.String())
			}
		}
	}

	// Matrix pattern: row by row (unigrams, then bigrams, then trigrams)
	expected := []string{
		// Row 0: unigrams
		"a", "b", "c", "d",
		// Row 1: bigrams
		"a b", "b c", "c d",
		// Row 2: trigrams
		"a b c", "b c d",
	}

	if !reflect.DeepEqual(tokens, expected) {
		t.Errorf("Expected matrix pattern %v, got %v", expected, tokens)
	}
}

// TestShingleMatrixFilter_MatrixWithoutUnigrams tests matrix pattern without unigrams.
// Source: TestShingleMatrixFilter.testMatrixWithoutUnigrams()
// Purpose: Tests matrix generation when unigrams are disabled.
func TestShingleMatrixFilter_MatrixWithoutUnigrams(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("a b c d"))

	filter := NewShingleMatrixFilter(tokenizer)
	filter.SetMaxShingleSize(3)
	filter.SetOutputUnigrams(false)
	defer filter.Close()

	var tokens []string
	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				tokens = append(tokens, termAttr.String())
			}
		}
	}

	// Matrix pattern without unigrams: bigrams then trigrams
	expected := []string{
		// Row 1: bigrams
		"a b", "b c", "c d",
		// Row 2: trigrams
		"a b c", "b c d",
	}

	if !reflect.DeepEqual(tokens, expected) {
		t.Errorf("Expected matrix pattern %v, got %v", expected, tokens)
	}
}

// TestShingleMatrixFilter_ComplexMatrix tests a complex matrix with min/max sizes.
// Source: TestShingleMatrixFilter.testComplexMatrix()
// Purpose: Tests matrix generation with custom min/max sizes.
func TestShingleMatrixFilter_ComplexMatrix(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("a b c d e"))

	filter := NewShingleMatrixFilterWithSizes(tokenizer, 2, 4)
	filter.SetOutputUnigrams(false)
	defer filter.Close()

	var tokens []string
	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				tokens = append(tokens, termAttr.String())
			}
		}
	}

	// Matrix pattern: bigrams, trigrams, 4-grams
	expected := []string{
		// Row 1: bigrams
		"a b", "b c", "c d", "d e",
		// Row 2: trigrams
		"a b c", "b c d", "c d e",
		// Row 3: 4-grams
		"a b c d", "b c d e",
	}

	if !reflect.DeepEqual(tokens, expected) {
		t.Errorf("Expected matrix pattern %v, got %v", expected, tokens)
	}
}

// TestShingleMatrixFilter_MultipleResets tests multiple reset cycles.
// Source: TestShingleMatrixFilter.testMultipleResets()
// Purpose: Tests that multiple reset cycles work correctly.
func TestShingleMatrixFilter_MultipleResets(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()

	filter := NewShingleMatrixFilter(tokenizer)
	defer filter.Close()

	// First run
	tokenizer.SetReader(strings.NewReader("x y"))
	var tokens1 []string
	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				tokens1 = append(tokens1, termAttr.String())
			}
		}
	}

	// Second run
	filter.Reset()
	tokenizer.SetReader(strings.NewReader("1 2 3"))
	var tokens2 []string
	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				tokens2 = append(tokens2, termAttr.String())
			}
		}
	}

	// Third run
	filter.Reset()
	tokenizer.SetReader(strings.NewReader("alpha"))
	var tokens3 []string
	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				tokens3 = append(tokens3, termAttr.String())
			}
		}
	}

	// Matrix pattern
	expected1 := []string{"x", "y", "x y"}
	expected2 := []string{"1", "2", "3", "1 2", "2 3"}
	expected3 := []string{"alpha"}

	if !reflect.DeepEqual(tokens1, expected1) {
		t.Errorf("First run: expected %v, got %v", expected1, tokens1)
	}

	if !reflect.DeepEqual(tokens2, expected2) {
		t.Errorf("Second run: expected %v, got %v", expected2, tokens2)
	}

	if !reflect.DeepEqual(tokens3, expected3) {
		t.Errorf("Third run: expected %v, got %v", expected3, tokens3)
	}
}
