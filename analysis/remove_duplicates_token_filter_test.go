// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"strings"
	"testing"
)

// TestRemoveDuplicatesTokenFilter_Basic tests basic duplicate removal.
// Source: TestRemoveDuplicatesTokenFilter.testBasic()
// Purpose: Tests that duplicate tokens at the same position are removed.
func TestRemoveDuplicatesTokenFilter_Basic(t *testing.T) {
	tests := []struct {
		name             string
		input            []tokenInfo
		expectedTokens   []string
		expectedPosIncr  []int
	}{
		{
			name: "No duplicates - different positions",
			input: []tokenInfo{
				{text: "hello", posIncr: 1},
				{text: "world", posIncr: 1},
			},
			expectedTokens:  []string{"hello", "world"},
			expectedPosIncr: []int{1, 1},
		},
		{
			name: "Duplicates at same position",
			input: []tokenInfo{
				{text: "test", posIncr: 1},
				{text: "test", posIncr: 0},
			},
			expectedTokens:  []string{"test"},
			expectedPosIncr: []int{1},
		},
		{
			name: "Multiple duplicates at same position",
			input: []tokenInfo{
				{text: "a", posIncr: 1},
				{text: "a", posIncr: 0},
				{text: "a", posIncr: 0},
			},
			expectedTokens:  []string{"a"},
			expectedPosIncr: []int{1},
		},
		{
			name: "Duplicates mixed with non-duplicates",
			input: []tokenInfo{
				{text: "hello", posIncr: 1},
				{text: "hello", posIncr: 0},
				{text: "hi", posIncr: 0},
				{text: "world", posIncr: 1},
			},
			expectedTokens:  []string{"hello", "hi", "world"},
			expectedPosIncr: []int{1, 0, 1},
		},
		{
			name: "Same text different positions - not duplicates",
			input: []tokenInfo{
				{text: "repeat", posIncr: 1},
				{text: "repeat", posIncr: 1},
			},
			expectedTokens:  []string{"repeat", "repeat"},
			expectedPosIncr: []int{1, 1},
		},
		{
			name: "Complex case with multiple positions",
			input: []tokenInfo{
				{text: "a", posIncr: 1},
				{text: "a", posIncr: 0},
				{text: "b", posIncr: 1},
				{text: "b", posIncr: 0},
				{text: "c", posIncr: 1},
			},
			expectedTokens:  []string{"a", "b", "c"},
			expectedPosIncr: []int{1, 1, 1},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create a mock token stream with the input tokens
			mockStream := newMockTokenStream(tc.input)

			filter := NewRemoveDuplicatesTokenFilter(mockStream)
			defer filter.Close()

			var tokens []string
			var posIncrs []int

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
						posIncrs = append(posIncrs, posAttr.GetPositionIncrement())
					}
				}
			}

			if !reflect.DeepEqual(tokens, tc.expectedTokens) {
				t.Errorf("Expected tokens %v, got %v", tc.expectedTokens, tokens)
			}

			if !reflect.DeepEqual(posIncrs, tc.expectedPosIncr) {
				t.Errorf("Expected position increments %v, got %v", tc.expectedPosIncr, posIncrs)
			}
		})
	}
}

// TestRemoveDuplicatesTokenFilter_EmptyInput tests empty input handling.
// Source: TestRemoveDuplicatesTokenFilter.testEmpty()
// Purpose: Tests that empty input is handled correctly.
func TestRemoveDuplicatesTokenFilter_EmptyInput(t *testing.T) {
	mockStream := newMockTokenStream([]tokenInfo{})

	filter := NewRemoveDuplicatesTokenFilter(mockStream)
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

// TestRemoveDuplicatesTokenFilter_SingleToken tests single token handling.
// Source: TestRemoveDuplicatesTokenFilter.testSingleToken()
// Purpose: Tests that a single token passes through unchanged.
func TestRemoveDuplicatesTokenFilter_SingleToken(t *testing.T) {
	mockStream := newMockTokenStream([]tokenInfo{
		{text: "hello", posIncr: 1},
	})

	filter := NewRemoveDuplicatesTokenFilter(mockStream)
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

// TestRemoveDuplicatesTokenFilter_WithRealTokenizer tests with a real tokenizer.
// Source: TestRemoveDuplicatesTokenFilter.testWithRealTokenizer()
// Purpose: Tests integration with actual tokenizers.
func TestRemoveDuplicatesTokenFilter_WithRealTokenizer(t *testing.T) {
	// Create a simple input that won't have duplicates naturally
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("hello world test"))

	filter := NewRemoveDuplicatesTokenFilter(tokenizer)
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

	expected := []string{"hello", "world", "test"}
	if !reflect.DeepEqual(tokens, expected) {
		t.Errorf("Expected %v, got %v", expected, tokens)
	}
}

// TestRemoveDuplicatesTokenFilter_PositionIncrementAccumulation tests position increment accumulation.
// Source: TestRemoveDuplicatesTokenFilter.testPositionIncrementAccumulation()
// Purpose: Tests that position increments are properly accumulated when duplicates are removed.
func TestRemoveDuplicatesTokenFilter_PositionIncrementAccumulation(t *testing.T) {
	// When duplicates are removed, the position increment of the next non-duplicate
	// token should include the increments of the removed duplicates
	mockStream := newMockTokenStream([]tokenInfo{
		{text: "a", posIncr: 1},
		{text: "a", posIncr: 0}, // duplicate, removed
		{text: "a", posIncr: 0}, // duplicate, removed
		{text: "b", posIncr: 1},
	})

	filter := NewRemoveDuplicatesTokenFilter(mockStream)
	defer filter.Close()

	var posIncrs []int
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
				posIncrs = append(posIncrs, posAttr.GetPositionIncrement())
			}
		}
	}

	// The position increment for "b" should be 1 (its original) + 0 + 0 (from removed duplicates)
	// But since the duplicates had posIncr=0, they don't add to the position
	expected := []int{1, 1}
	if !reflect.DeepEqual(posIncrs, expected) {
		t.Errorf("Expected position increments %v, got %v", expected, posIncrs)
	}
}

// TestRemoveDuplicatesTokenFilter_EndMethod tests the End() method.
// Source: TestRemoveDuplicatesTokenFilter.testEnd()
// Purpose: Tests that End() is properly propagated and clears state.
func TestRemoveDuplicatesTokenFilter_EndMethod(t *testing.T) {
	mockStream := newMockTokenStream([]tokenInfo{
		{text: "test", posIncr: 1},
	})

	filter := NewRemoveDuplicatesTokenFilter(mockStream)
	defer filter.Close()

	// Process all tokens
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

// TestRemoveDuplicatesTokenFilter_Chaining tests chaining with other filters.
// Source: TestRemoveDuplicatesTokenFilter.testChaining()
// Purpose: Tests that RemoveDuplicatesTokenFilter works properly in filter chains.
func TestRemoveDuplicatesTokenFilter_Chaining(t *testing.T) {
	// Chain: mock stream with duplicates -> LowerCaseFilter -> RemoveDuplicatesTokenFilter
	// The mock stream produces tokens with position increment 0 for duplicates
	mockStream := newMockTokenStream([]tokenInfo{
		{text: "HELLO", posIncr: 1},
		{text: "hello", posIncr: 0}, // Same position as HELLO (simulating synonym expansion)
		{text: "WORLD", posIncr: 1},
	})

	lowerFilter := NewLowerCaseFilter(mockStream)
	filter := NewRemoveDuplicatesTokenFilter(lowerFilter)
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

	// After lowercasing, "HELLO" and "hello" both become "hello" at the same position
	// So the duplicate should be removed
	expected := []string{"hello", "world"}
	if !reflect.DeepEqual(tokens, expected) {
		t.Errorf("Expected %v, got %v", expected, tokens)
	}
}

// TestRemoveDuplicatesTokenFilter_Factory tests the factory.
// Source: TestRemoveDuplicatesTokenFilter.testFactory()
// Purpose: Tests that the factory creates correct filter instances.
func TestRemoveDuplicatesTokenFilter_Factory(t *testing.T) {
	factory := NewRemoveDuplicatesTokenFilterFactory()
	tokenizer := NewWhitespaceTokenizer()
	filter := factory.Create(tokenizer)

	_, ok := filter.(*RemoveDuplicatesTokenFilter)
	if !ok {
		t.Fatal("expected RemoveDuplicatesTokenFilter from factory")
	}
}

// TestRemoveDuplicatesTokenFilter_DifferentTextsSamePosition tests different texts at same position.
// Source: TestRemoveDuplicatesTokenFilter.testDifferentTextsSamePosition()
// Purpose: Tests that different texts at the same position are preserved.
func TestRemoveDuplicatesTokenFilter_DifferentTextsSamePosition(t *testing.T) {
	mockStream := newMockTokenStream([]tokenInfo{
		{text: "hello", posIncr: 1},
		{text: "hi", posIncr: 0},
		{text: "greetings", posIncr: 0},
	})

	filter := NewRemoveDuplicatesTokenFilter(mockStream)
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

	expected := []string{"hello", "hi", "greetings"}
	if !reflect.DeepEqual(tokens, expected) {
		t.Errorf("Expected %v, got %v", expected, tokens)
	}
}

// TestRemoveDuplicatesTokenFilter_MultiplePositionsWithDuplicates tests multiple positions with duplicates.
// Source: TestRemoveDuplicatesTokenFilter.testMultiplePositionsWithDuplicates()
// Purpose: Tests handling of duplicates across multiple positions.
func TestRemoveDuplicatesTokenFilter_MultiplePositionsWithDuplicates(t *testing.T) {
	mockStream := newMockTokenStream([]tokenInfo{
		{text: "a", posIncr: 1},
		{text: "a", posIncr: 0}, // duplicate at position 1, removed
		{text: "b", posIncr: 1},
		{text: "b", posIncr: 0}, // duplicate at position 2, removed
		{text: "c", posIncr: 1},
		{text: "a", posIncr: 1}, // "a" at position 3, NOT a duplicate (different position)
	})

	filter := NewRemoveDuplicatesTokenFilter(mockStream)
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

	// "a" appears twice but at different positions, so both should be kept
	expected := []string{"a", "b", "c", "a"}
	if !reflect.DeepEqual(tokens, expected) {
		t.Errorf("Expected %v, got %v", expected, tokens)
	}
}

// tokenInfo holds information about a token for testing
type tokenInfo struct {
	text    string
	posIncr int
}

// mockTokenStream is a mock TokenStream for testing
type mockTokenStream struct {
	*BaseTokenStream
	tokens      []tokenInfo
	currentIdx  int
	termAttr    CharTermAttribute
	posIncrAttr PositionIncrementAttribute
}

// newMockTokenStream creates a new mock token stream with the given tokens.
func newMockTokenStream(tokens []tokenInfo) *mockTokenStream {
	stream := &mockTokenStream{
		BaseTokenStream: NewBaseTokenStream(),
		tokens:          tokens,
		currentIdx:      0,
	}

	// Add attributes
	stream.termAttr = NewCharTermAttribute()
	stream.posIncrAttr = NewPositionIncrementAttribute()
	stream.AddAttribute(stream.termAttr)
	stream.AddAttribute(stream.posIncrAttr)

	return stream
}

// IncrementToken advances to the next token.
func (m *mockTokenStream) IncrementToken() (bool, error) {
	if m.currentIdx >= len(m.tokens) {
		return false, nil
	}

	token := m.tokens[m.currentIdx]
	m.currentIdx++

	// Set the attributes
	m.termAttr.SetValue(token.text)
	m.posIncrAttr.SetPositionIncrement(token.posIncr)

	return true, nil
}

// End performs end-of-stream operations.
func (m *mockTokenStream) End() error {
	return nil
}

// Close releases resources.
func (m *mockTokenStream) Close() error {
	return nil
}

// Ensure mockTokenStream implements TokenStream
var _ TokenStream = (*mockTokenStream)(nil)
