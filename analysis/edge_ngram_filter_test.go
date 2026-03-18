// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"strings"
	"testing"
)

// TestEdgeNGramFilter_Basic tests basic edge n-gram generation.
// Purpose: Tests that edge n-grams are correctly generated from tokens.
func TestEdgeNGramFilter_Basic(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		minGram  int
		maxGram  int
		expected []string
	}{
		{
			name:     "Simple word min2 max4",
			input:    "hello",
			minGram:  2,
			maxGram:  4,
			expected: []string{"he", "hel", "hell"},
		},
		{
			name:     "Simple word min1 max3",
			input:    "hello",
			minGram:  1,
			maxGram:  3,
			expected: []string{"h", "he", "hel"},
		},
		{
			name:     "Short word min2 max4",
			input:    "hi",
			minGram:  2,
			maxGram:  4,
			expected: []string{"hi"},
		},
		{
			name:     "Word shorter than minGram",
			input:    "a",
			minGram:  2,
			maxGram:  4,
			expected: nil,
		},
		{
			name:     "Multiple words",
			input:    "hello world",
			minGram:  2,
			maxGram:  3,
			expected: []string{"he", "hel", "wo", "wor"},
		},
		{
			name:     "minGram equals maxGram",
			input:    "hello",
			minGram:  3,
			maxGram:  3,
			expected: []string{"hel"},
		},
		{
			name:     "maxGram larger than word",
			input:    "hi",
			minGram:  1,
			maxGram:  10,
			expected: []string{"h", "hi"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			filter := NewEdgeNGramFilter(tokenizer, tc.minGram, tc.maxGram)
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

			if len(tokens) != len(tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, tokens)
			} else if len(tokens) > 0 && !reflect.DeepEqual(tokens, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, tokens)
			}
		})
	}
}

// TestEdgeNGramFilter_Unicode tests Unicode handling.
// Purpose: Tests that Unicode characters are properly handled.
func TestEdgeNGramFilter_Unicode(t *testing.T) {
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
			maxGram:  4,
			expected: []string{"ca", "caf", "café"},
		},
		{
			name:     "Chinese characters",
			input:    "中文测试",
			minGram:  1,
			maxGram:  3,
			expected: []string{"中", "中文", "中文测"},
		},
		{
			name:     "Mixed ASCII and Unicode",
			input:    "hello世界",
			minGram:  2,
			maxGram:  5,
			expected: []string{"he", "hel", "hell", "hello"},
		},
		{
			name:     "Emoji characters",
			input:    "👍👎👌",
			minGram:  1,
			maxGram:  2,
			expected: []string{"👍", "👍👎"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			filter := NewEdgeNGramFilter(tokenizer, tc.minGram, tc.maxGram)
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

// TestEdgeNGramFilter_PositionIncrement tests position increment handling.
// Purpose: Tests that position increments are correctly set.
func TestEdgeNGramFilter_PositionIncrement(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("hello world"))

	filter := NewEdgeNGramFilter(tokenizer, 2, 3)
	defer filter.Close()

	type tokenInfo struct {
		text         string
		posIncrement int
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
		if attr := filter.GetAttributeSource().GetAttribute("PositionIncrementAttribute"); attr != nil {
			if posAttr, ok := attr.(PositionIncrementAttribute); ok {
				info.posIncrement = posAttr.GetPositionIncrement()
			}
		}
		tokens = append(tokens, info)
	}

	// Expected: "he" (pos 1), "hel" (pos 0), "wo" (pos 1), "wor" (pos 0)
	expected := []tokenInfo{
		{"he", 1},
		{"hel", 0},
		{"wo", 1},
		{"wor", 0},
	}

	if len(tokens) != len(expected) {
		t.Fatalf("Expected %d tokens, got %d", len(expected), len(tokens))
	}

	for i, exp := range expected {
		if tokens[i].text != exp.text || tokens[i].posIncrement != exp.posIncrement {
			t.Errorf("Token[%d]: expected (%s, pos=%d), got (%s, pos=%d)",
				i, exp.text, exp.posIncrement, tokens[i].text, tokens[i].posIncrement)
		}
	}
}

// TestEdgeNGramFilter_Offsets tests offset handling.
// Purpose: Tests that character offsets are correctly adjusted for n-grams.
func TestEdgeNGramFilter_Offsets(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("hello"))

	filter := NewEdgeNGramFilter(tokenizer, 2, 4)
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

	// Expected offsets for "hello" with min=2, max=4:
	// "he": [0, 2], "hel": [0, 3], "hell": [0, 4]
	expected := []tokenInfo{
		{"he", 0, 2},
		{"hel", 0, 3},
		{"hell", 0, 4},
	}

	if len(tokens) != len(expected) {
		t.Fatalf("Expected %d tokens, got %d", len(expected), len(tokens))
	}

	for i, exp := range expected {
		if tokens[i].text != exp.text ||
			tokens[i].startOffset != exp.startOffset ||
			tokens[i].endOffset != exp.endOffset {
			t.Errorf("Token[%d]: expected (%s [%d,%d]), got (%s [%d,%d])",
				i, exp.text, exp.startOffset, exp.endOffset,
				tokens[i].text, tokens[i].startOffset, tokens[i].endOffset)
		}
	}
}

// TestEdgeNGramFilter_PreserveOriginal tests preserving original tokens.
// Purpose: Tests that original tokens can be preserved alongside n-grams.
func TestEdgeNGramFilter_PreserveOriginal(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		minGram  int
		maxGram  int
		expected []string
	}{
		{
			name:     "Preserve original with min2 max3",
			input:    "hello",
			minGram:  2,
			maxGram:  3,
			expected: []string{"he", "hel", "hello"},
		},
		{
			name:     "Preserve original with short word",
			input:    "hi",
			minGram:  2,
			maxGram:  4,
			expected: []string{"hi", "hi"}, // n-gram + original
		},
		{
			name:     "Preserve original with word shorter than minGram",
			input:    "a",
			minGram:  2,
			maxGram:  4,
			expected: []string{"a"}, // only original since shorter than minGram
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			filter := NewEdgeNGramFilterWithOptions(tokenizer, tc.minGram, tc.maxGram, true)
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

// TestEdgeNGramFilter_PreserveOriginalPositionIncrement tests position increments with preserved original.
// Purpose: Tests that position increments are correct when preserving original tokens.
func TestEdgeNGramFilter_PreserveOriginalPositionIncrement(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("hello"))

	filter := NewEdgeNGramFilterWithOptions(tokenizer, 2, 3, true)
	defer filter.Close()

	type tokenInfo struct {
		text         string
		posIncrement int
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
		if attr := filter.GetAttributeSource().GetAttribute("PositionIncrementAttribute"); attr != nil {
			if posAttr, ok := attr.(PositionIncrementAttribute); ok {
				info.posIncrement = posAttr.GetPositionIncrement()
			}
		}
		tokens = append(tokens, info)
	}

	// Expected: "he" (pos 1), "hel" (pos 0), "hello" (pos 0 - same position as last n-gram)
	expected := []tokenInfo{
		{"he", 1},
		{"hel", 0},
		{"hello", 0},
	}

	if len(tokens) != len(expected) {
		t.Fatalf("Expected %d tokens, got %d", len(expected), len(tokens))
	}

	for i, exp := range expected {
		if tokens[i].text != exp.text || tokens[i].posIncrement != exp.posIncrement {
			t.Errorf("Token[%d]: expected (%s, pos=%d), got (%s, pos=%d)",
				i, exp.text, exp.posIncrement, tokens[i].text, tokens[i].posIncrement)
		}
	}
}

// TestEdgeNGramFilter_EmptyInput tests empty input handling.
// Purpose: Tests that empty input is handled correctly.
func TestEdgeNGramFilter_EmptyInput(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader(""))

	filter := NewEdgeNGramFilter(tokenizer, 2, 4)
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

// TestEdgeNGramFilter_EndMethod tests the End() method.
// Purpose: Tests that End() is properly propagated.
func TestEdgeNGramFilter_EndMethod(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("test"))

	filter := NewEdgeNGramFilter(tokenizer, 2, 3)
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

// TestEdgeNGramFilter_Chaining tests chaining with other filters.
// Purpose: Tests that EdgeNGramFilter works properly in filter chains.
func TestEdgeNGramFilter_Chaining(t *testing.T) {
	input := "HELLO world"

	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader(input))

	lowerFilter := NewLowerCaseFilter(tokenizer)
	ngramFilter := NewEdgeNGramFilter(lowerFilter, 2, 3)
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

	// Expected: "he", "hel" from "hello", "wo", "wor" from "world"
	expected := []string{"he", "hel", "wo", "wor"}
	if !reflect.DeepEqual(tokens, expected) {
		t.Errorf("Expected %v, got %v", expected, tokens)
	}
}

// TestEdgeNGramFilter_ParameterValidation tests parameter validation.
// Purpose: Tests that invalid parameters are handled correctly.
func TestEdgeNGramFilter_ParameterValidation(t *testing.T) {
	tests := []struct {
		name            string
		minGram         int
		maxGram         int
		expectedMinGram int
		expectedMaxGram int
	}{
		{
			name:            "minGram less than 1",
			minGram:         0,
			maxGram:         4,
			expectedMinGram: 1,
			expectedMaxGram: 4,
		},
		{
			name:            "maxGram less than minGram",
			minGram:         3,
			maxGram:         2,
			expectedMinGram: 3,
			expectedMaxGram: 3,
		},
		{
			name:            "negative minGram",
			minGram:         -5,
			maxGram:         4,
			expectedMinGram: 1,
			expectedMaxGram: 4,
		},
		{
			name:            "valid parameters",
			minGram:         2,
			maxGram:         5,
			expectedMinGram: 2,
			expectedMaxGram: 5,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader("test"))

			filter := NewEdgeNGramFilter(tokenizer, tc.minGram, tc.maxGram)

			if filter.GetMinGram() != tc.expectedMinGram {
				t.Errorf("Expected minGram %d, got %d", tc.expectedMinGram, filter.GetMinGram())
			}
			if filter.GetMaxGram() != tc.expectedMaxGram {
				t.Errorf("Expected maxGram %d, got %d", tc.expectedMaxGram, filter.GetMaxGram())
			}
		})
	}
}

// TestEdgeNGramFilter_Factory tests the factory.
// Purpose: Tests that the factory creates filters correctly.
func TestEdgeNGramFilter_Factory(t *testing.T) {
	factory := NewEdgeNGramFilterFactory(2, 4)

	if factory.GetMinGram() != 2 {
		t.Errorf("Expected minGram 2, got %d", factory.GetMinGram())
	}
	if factory.GetMaxGram() != 4 {
		t.Errorf("Expected maxGram 4, got %d", factory.GetMaxGram())
	}
	if factory.IsPreserveOriginal() {
		t.Error("Expected preserveOriginal to be false")
	}

	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("hello"))

	filter := factory.Create(tokenizer)
	defer filter.Close()

	edgeFilter, ok := filter.(*EdgeNGramFilter)
	if !ok {
		t.Fatal("Expected *EdgeNGramFilter from factory")
	}

	if edgeFilter.GetMinGram() != 2 {
		t.Errorf("Expected filter minGram 2, got %d", edgeFilter.GetMinGram())
	}
	if edgeFilter.GetMaxGram() != 4 {
		t.Errorf("Expected filter maxGram 4, got %d", edgeFilter.GetMaxGram())
	}
}

// TestEdgeNGramFilter_FactoryWithOptions tests the factory with options.
// Purpose: Tests that the factory with options creates filters correctly.
func TestEdgeNGramFilter_FactoryWithOptions(t *testing.T) {
	factory := NewEdgeNGramFilterFactoryWithOptions(2, 4, true)

	if factory.GetMinGram() != 2 {
		t.Errorf("Expected minGram 2, got %d", factory.GetMinGram())
	}
	if factory.GetMaxGram() != 4 {
		t.Errorf("Expected maxGram 4, got %d", factory.GetMaxGram())
	}
	if !factory.IsPreserveOriginal() {
		t.Error("Expected preserveOriginal to be true")
	}

	// Verify factory implements TokenFilterFactory
	var _ TokenFilterFactory = factory
}

// TestEdgeNGramFilter_SingleCharacterTokens tests single character tokens.
// Purpose: Tests that single character tokens are handled correctly.
func TestEdgeNGramFilter_SingleCharacterTokens(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("a b c"))

	filter := NewEdgeNGramFilter(tokenizer, 2, 3)
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

	// All tokens are single characters, shorter than minGram=2
	// So no tokens should be emitted
	if tokenCount != 0 {
		t.Errorf("Expected 0 tokens for single char input with minGram=2, got %d", tokenCount)
	}
}

// TestEdgeNGramFilter_MixedLengthTokens tests mixed length tokens.
// Purpose: Tests that tokens of varying lengths are handled correctly.
func TestEdgeNGramFilter_MixedLengthTokens(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("a hi hello"))

	filter := NewEdgeNGramFilter(tokenizer, 2, 4)
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

	// "a" is skipped (shorter than minGram=2)
	// "hi" -> "hi"
	// "hello" -> "he", "hel", "hell"
	expected := []string{"hi", "he", "hel", "hell"}
	if !reflect.DeepEqual(tokens, expected) {
		t.Errorf("Expected %v, got %v", expected, tokens)
	}
}

// TestEdgeNGramFilter_UnicodeOffsets tests Unicode character offsets.
// Purpose: Tests that offsets are correctly calculated for Unicode strings.
func TestEdgeNGramFilter_UnicodeOffsets(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("café"))

	filter := NewEdgeNGramFilter(tokenizer, 2, 3)
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

	// "café" has 4 characters but 5 bytes (é is 2 bytes)
	// "ca": [0, 2], "caf": [0, 3]
	expected := []tokenInfo{
		{"ca", 0, 2},
		{"caf", 0, 3},
	}

	if len(tokens) != len(expected) {
		t.Fatalf("Expected %d tokens, got %d", len(expected), len(tokens))
	}

	for i, exp := range expected {
		if tokens[i].text != exp.text ||
			tokens[i].startOffset != exp.startOffset ||
			tokens[i].endOffset != exp.endOffset {
			t.Errorf("Token[%d]: expected (%s [%d,%d]), got (%s [%d,%d])",
				i, exp.text, exp.startOffset, exp.endOffset,
				tokens[i].text, tokens[i].startOffset, tokens[i].endOffset)
		}
	}
}

// TestEdgeNGramFilter_LargeMaxGram tests with maxGram larger than token length.
// Purpose: Tests that maxGram larger than token length is handled correctly.
func TestEdgeNGramFilter_LargeMaxGram(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("hi"))

	filter := NewEdgeNGramFilter(tokenizer, 1, 100)
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

	// "hi" with min=1, max=100 should produce: "h", "hi"
	expected := []string{"h", "hi"}
	if !reflect.DeepEqual(tokens, expected) {
		t.Errorf("Expected %v, got %v", expected, tokens)
	}
}

// TestEdgeNGramFilter_ImplementsTokenFilter verifies the filter implements TokenFilter interface.
func TestEdgeNGramFilter_ImplementsTokenFilter(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	filter := NewEdgeNGramFilter(tokenizer, 2, 4)

	// Verify it implements TokenFilter
	var _ TokenFilter = filter

	// Verify we can get the input
	if filter.GetInput() != tokenizer {
		t.Error("GetInput() should return the input tokenizer")
	}
}

// TestEdgeNGramFilter_Getters tests the getter methods.
func TestEdgeNGramFilter_Getters(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()

	filter1 := NewEdgeNGramFilter(tokenizer, 2, 4)
	if filter1.GetMinGram() != 2 {
		t.Errorf("Expected minGram 2, got %d", filter1.GetMinGram())
	}
	if filter1.GetMaxGram() != 4 {
		t.Errorf("Expected maxGram 4, got %d", filter1.GetMaxGram())
	}
	if filter1.IsPreserveOriginal() {
		t.Error("Expected preserveOriginal to be false")
	}

	filter2 := NewEdgeNGramFilterWithOptions(tokenizer, 3, 5, true)
	if filter2.GetMinGram() != 3 {
		t.Errorf("Expected minGram 3, got %d", filter2.GetMinGram())
	}
	if filter2.GetMaxGram() != 5 {
		t.Errorf("Expected maxGram 5, got %d", filter2.GetMaxGram())
	}
	if !filter2.IsPreserveOriginal() {
		t.Error("Expected preserveOriginal to be true")
	}
}
