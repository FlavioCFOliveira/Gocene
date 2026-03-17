// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"strings"
	"testing"
)

// TestShingleFilter_Basic tests basic shingle generation with default settings.
// Source: TestShingleFilter.testShingleGeneration()
// Purpose: Tests that shingles are correctly generated from tokens.
func TestShingleFilter_Basic(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    []string
		expectedPos []int
	}{
		{
			name:        "Simple sentence",
			input:       "please divide this sentence",
			expected:    []string{"please", "please divide", "divide", "divide this", "this", "this sentence", "sentence"},
			expectedPos: []int{1, 0, 0, 0, 0, 0, 0},
		},
		{
			name:        "Two words",
			input:       "hello world",
			expected:    []string{"hello", "hello world", "world"},
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
			expected:    []string{"a", "a b", "b", "b c", "c"},
			expectedPos: []int{1, 0, 0, 0, 0},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			filter := NewShingleFilter(tokenizer)
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

// TestShingleFilter_NoUnigrams tests shingle generation without unigrams.
// Source: TestShingleFilter.testShinglesOnly()
// Purpose: Tests that only shingles are output when outputUnigrams=false.
func TestShingleFilter_NoUnigrams(t *testing.T) {
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

			filter := NewShingleFilter(tokenizer)
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

// TestShingleFilter_LargerSize tests shingle generation with larger max size.
// Source: TestShingleFilter.testMaxShingleSize()
// Purpose: Tests shingles with maxShingleSize > 2.
func TestShingleFilter_LargerSize(t *testing.T) {
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
			expected:    []string{"a", "a b", "a b c", "b", "b c", "b c d", "c", "c d", "d"},
			expectedPos: []int{1, 0, 0, 0, 0, 0, 0, 0, 0},
		},
		{
			name:        "Size 4",
			input:       "a b c d e",
			maxSize:     4,
			expected:    []string{"a", "a b", "a b c", "a b c d", "b", "b c", "b c d", "b c d e", "c", "c d", "c d e", "d", "d e", "e"},
			expectedPos: []int{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			filter := NewShingleFilter(tokenizer)
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

// TestShingleFilter_MinMaxSize tests shingle generation with custom min/max sizes.
// Source: TestShingleFilter.testMinMaxShingleSize()
// Purpose: Tests shingles with minShingleSize > 1.
func TestShingleFilter_MinMaxSize(t *testing.T) {
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
			expected: []string{"a b", "a b c", "b c", "b c d", "c d"},
		},
		{
			name:     "Min 3 Max 3",
			input:    "a b c d e",
			minSize:  3,
			maxSize:  3,
			expected: []string{"a b c", "b c d", "c d e"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			filter := NewShingleFilterWithSizes(tokenizer, tc.minSize, tc.maxSize)
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

// TestShingleFilter_TokenSeparator tests custom token separators.
// Source: TestShingleFilter.testTokenSeparator()
// Purpose: Tests that custom separators are correctly inserted.
func TestShingleFilter_TokenSeparator(t *testing.T) {
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
			expected:  []string{"hello", "hello_world", "world", "world_test", "test"},
		},
		{
			name:      "Empty separator",
			input:     "hello world test",
			separator: "",
			expected:  []string{"hello", "helloworld", "world", "worldtest", "test"},
		},
		{
			name:      "Hyphen separator",
			input:     "hello world test",
			separator: "-",
			expected:  []string{"hello", "hello-world", "world", "world-test", "test"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			filter := NewShingleFilter(tokenizer)
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

// TestShingleFilter_Offsets tests that offsets are correctly preserved.
// Source: TestShingleFilter.testOffsets()
// Purpose: Tests that start/end offsets are correct for shingles.
func TestShingleFilter_Offsets(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("hello world"))

	filter := NewShingleFilter(tokenizer)
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

	// "hello" [0, 5]
	if tokens[0].text != "hello" || tokens[0].startOffset != 0 || tokens[0].endOffset != 5 {
		t.Errorf("First token: expected hello [0,5], got %s [%d,%d]",
			tokens[0].text, tokens[0].startOffset, tokens[0].endOffset)
	}

	// "hello world" [0, 11]
	if tokens[1].text != "hello world" || tokens[1].startOffset != 0 || tokens[1].endOffset != 11 {
		t.Errorf("Second token: expected 'hello world' [0,11], got %s [%d,%d]",
			tokens[1].text, tokens[1].startOffset, tokens[1].endOffset)
	}

	// "world" [6, 11]
	if tokens[2].text != "world" || tokens[2].startOffset != 6 || tokens[2].endOffset != 11 {
		t.Errorf("Third token: expected world [6,11], got %s [%d,%d]",
			tokens[2].text, tokens[2].startOffset, tokens[2].endOffset)
	}
}

// TestShingleFilter_EmptyInput tests empty input handling.
// Source: TestShingleFilter.testEmpty()
// Purpose: Tests that empty input produces no tokens.
func TestShingleFilter_EmptyInput(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader(""))

	filter := NewShingleFilter(tokenizer)
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

// TestShingleFilter_SingleToken tests input with a single token.
// Source: TestShingleFilter.testSingleToken()
// Purpose: Tests that single token input works correctly.
func TestShingleFilter_SingleToken(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("hello"))

	filter := NewShingleFilter(tokenizer)
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

// TestShingleFilter_Chaining tests chaining with other filters.
// Source: TestShingleFilter.testChaining()
// Purpose: Tests that ShingleFilter works properly in filter chains.
func TestShingleFilter_Chaining(t *testing.T) {
	input := "HELLO world Test"

	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader(input))

	lowerFilter := NewLowerCaseFilter(tokenizer)
	shingleFilter := NewShingleFilter(lowerFilter)
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

	expected := []string{"hello", "hello world", "world", "world test", "test"}
	if !reflect.DeepEqual(tokens, expected) {
		t.Errorf("Expected %v, got %v", expected, tokens)
	}
}

// TestShingleFilter_EndMethod tests the End() method.
// Source: TestShingleFilter.testEnd()
// Purpose: Tests that End() is properly propagated.
func TestShingleFilter_EndMethod(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("hello world"))

	filter := NewShingleFilter(tokenizer)
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

// TestShingleFilter_Reset tests the Reset() method.
// Source: TestShingleFilter.testReset()
// Purpose: Tests that Reset() properly clears state.
func TestShingleFilter_Reset(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()

	// First run
	tokenizer.SetReader(strings.NewReader("hello world"))
	filter := NewShingleFilter(tokenizer)

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

	expected1 := []string{"hello", "hello world", "world"}
	expected2 := []string{"a", "a b", "b"}

	if !reflect.DeepEqual(tokens1, expected1) {
		t.Errorf("First run: expected %v, got %v", expected1, tokens1)
	}

	if !reflect.DeepEqual(tokens2, expected2) {
		t.Errorf("Second run: expected %v, got %v", expected2, tokens2)
	}

	filter.Close()
}

// TestShingleFilter_Factory tests the ShingleFilterFactory.
// Source: TestShingleFilter.testFactory()
// Purpose: Tests that the factory creates properly configured filters.
func TestShingleFilter_Factory(t *testing.T) {
	factory := NewShingleFilterFactoryWithSizes(2, 3)
	factory.SetTokenSeparator("_")
	factory.SetOutputUnigrams(false)

	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("a b c d"))

	filter := factory.Create(tokenizer).(*ShingleFilter)
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

	expected := []string{"a_b", "a_b_c", "b_c", "b_c_d", "c_d"}
	if !reflect.DeepEqual(tokens, expected) {
		t.Errorf("Expected %v, got %v", expected, tokens)
	}
}

// TestShingleFilter_GettersSetters tests getter and setter methods.
// Source: TestShingleFilter.testGettersSetters()
// Purpose: Tests that configuration methods work correctly.
func TestShingleFilter_GettersSetters(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	filter := NewShingleFilter(tokenizer)
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

// TestShingleFilter_BoundaryConditions tests boundary conditions.
// Source: TestShingleFilter.testBoundaryConditions()
// Purpose: Tests edge cases and boundary conditions.
func TestShingleFilter_BoundaryConditions(t *testing.T) {
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
			expected: []string{"a", "a b", "b"},
		},
		{
			name:     "Max size exceeds token count",
			input:    "a b",
			minSize:  2,
			maxSize:  5,
			expected: []string{"a", "a b", "b"},
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

			filter := NewShingleFilterWithSizes(tokenizer, tc.minSize, tc.maxSize)
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

// TestShingleFilter_LongTokens tests handling of long tokens.
// Source: TestShingleFilter.testLongTokens()
// Purpose: Tests that long tokens are handled correctly.
func TestShingleFilter_LongTokens(t *testing.T) {
	longToken1 := strings.Repeat("a", 100)
	longToken2 := strings.Repeat("b", 100)
	input := longToken1 + " " + longToken2

	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader(input))

	filter := NewShingleFilter(tokenizer)
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

	expectedShingle := longToken1 + " " + longToken2
	if tokens[1] != expectedShingle {
		t.Errorf("Shingle token mismatch: expected length %d, got length %d", len(expectedShingle), len(tokens[1]))
	}

	if tokens[2] != longToken2 {
		t.Errorf("Third token mismatch")
	}
}

// TestShingleFilter_SpecialCharacters tests handling of special characters.
// Source: TestShingleFilter.testSpecialCharacters()
// Purpose: Tests that special characters in tokens are preserved.
func TestShingleFilter_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Tokens with punctuation",
			input:    "hello, world! test.",
			expected: []string{"hello,", "hello, world!", "world!", "world! test.", "test."},
		},
		{
			name:     "Tokens with numbers",
			input:    "test123 abc456",
			expected: []string{"test123", "test123 abc456", "abc456"},
		},
		{
			name:     "Tokens with unicode",
			input:    "café résumé",
			expected: []string{"café", "café résumé", "résumé"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			filter := NewShingleFilter(tokenizer)
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

// TestShingleFilter_PositionIncrements tests position increment handling.
// Source: TestShingleFilter.testPositionIncrements()
// Purpose: Tests that position increments are correctly set.
func TestShingleFilter_PositionIncrements(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("a b c"))

	filter := NewShingleFilter(tokenizer)
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
	expected := []int{1, 0, 0, 0, 0}
	if !reflect.DeepEqual(positions, expected) {
		t.Errorf("Expected positions %v, got %v", expected, positions)
	}
}

// TestShingleFilter_NoUnigramsPositionIncrements tests position increments without unigrams.
// Source: TestShingleFilter.testNoUnigramsPositionIncrements()
// Purpose: Tests position increments when only outputting shingles.
func TestShingleFilter_NoUnigramsPositionIncrements(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("a b c"))

	filter := NewShingleFilter(tokenizer)
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
