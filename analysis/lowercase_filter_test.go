// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"strings"
	"testing"
)

// TestLowerCaseFilter_Basic tests basic lowercasing.
// Source: TestLowerCaseFilter.testLowerCasing()
// Purpose: Tests that tokens are converted to lowercase.
func TestLowerCaseFilter_Basic(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Mixed case",
			input:    "Hello World",
			expected: []string{"hello", "world"},
		},
		{
			name:     "All uppercase",
			input:    "HELLO WORLD",
			expected: []string{"hello", "world"},
		},
		{
			name:     "All lowercase",
			input:    "hello world",
			expected: []string{"hello", "world"},
		},
		{
			name:     "Mixed alphanumeric",
			input:    "Test123 ABC456",
			expected: []string{"test123", "abc456"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			lowerFilter := NewLowerCaseFilter(tokenizer)
			defer lowerFilter.Close()

			var tokens []string
			for {
				hasToken, err := lowerFilter.IncrementToken()
				if err != nil {
					t.Fatalf("Error incrementing token: %v", err)
				}
				if !hasToken {
					break
				}
				if attr := lowerFilter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
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

// TestLowerCaseFilter_Unicode tests Unicode lowercasing.
// Source: TestLowerCaseFilter.testUnicode()
// Purpose: Tests proper handling of Unicode characters.
func TestLowerCaseFilter_Unicode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Accented characters",
			input:    "CAFÉ RÉSUMÉ",
			expected: []string{"café", "résumé"},
		},
		{
			name:     "German eszett",
			input:    "STRASSE",
			expected: []string{"strasse"},
		},
		{
			name:     "Greek letters",
			input:    "ΑΒΓ ΔΕΖ",
			expected: []string{"αβγ", "δεζ"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			lowerFilter := NewLowerCaseFilter(tokenizer)
			defer lowerFilter.Close()

			var tokens []string
			for {
				hasToken, err := lowerFilter.IncrementToken()
				if err != nil {
					t.Fatalf("Error incrementing token: %v", err)
				}
				if !hasToken {
					break
				}
				if attr := lowerFilter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
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

// TestLowerCaseFilter_EmptyInput tests empty input handling.
// Source: TestLowerCaseFilter.testEmpty()
// Purpose: Tests that empty input is handled correctly.
func TestLowerCaseFilter_EmptyInput(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader(""))

	lowerFilter := NewLowerCaseFilter(tokenizer)
	defer lowerFilter.Close()

	tokenCount := 0
	for {
		hasToken, err := lowerFilter.IncrementToken()
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

// TestLowerCaseFilter_PositionIncrement tests position increment preservation.
// Source: TestLowerCaseFilter.testPositionIncrement()
// Purpose: Tests that position increments are preserved.
func TestLowerCaseFilter_PositionIncrement(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("A B C"))

	lowerFilter := NewLowerCaseFilter(tokenizer)
	defer lowerFilter.Close()

	positions := []int{}
	for {
		hasToken, err := lowerFilter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := lowerFilter.GetAttributeSource().GetAttribute("PositionIncrementAttribute"); attr != nil {
			if posAttr, ok := attr.(PositionIncrementAttribute); ok {
				positions = append(positions, posAttr.GetPositionIncrement())
			}
		}
	}

	expected := []int{1, 1, 1}
	if !reflect.DeepEqual(positions, expected) {
		t.Errorf("Expected positions %v, got %v", expected, positions)
	}
}

// TestLowerCaseFilter_Offset tests offset preservation.
// Source: TestLowerCaseFilter.testOffset()
// Purpose: Tests that character offsets are preserved.
func TestLowerCaseFilter_Offset(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("HELLO world"))

	lowerFilter := NewLowerCaseFilter(tokenizer)
	defer lowerFilter.Close()

	type tokenInfo struct {
		text        string
		startOffset int
		endOffset   int
	}

	var tokens []tokenInfo
	for {
		hasToken, err := lowerFilter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}

		var info tokenInfo
		if attr := lowerFilter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				info.text = termAttr.String()
			}
		}
		if attr := lowerFilter.GetAttributeSource().GetAttribute("OffsetAttribute"); attr != nil {
			if offsetAttr, ok := attr.(OffsetAttribute); ok {
				info.startOffset = offsetAttr.StartOffset()
				info.endOffset = offsetAttr.EndOffset()
			}
		}
		tokens = append(tokens, info)
	}

	if len(tokens) != 2 {
		t.Fatalf("Expected 2 tokens, got %d", len(tokens))
	}

	if tokens[0].text != "hello" || tokens[0].startOffset != 0 || tokens[0].endOffset != 5 {
		t.Errorf("First token: expected hello [0,5], got %s [%d,%d]",
			tokens[0].text, tokens[0].startOffset, tokens[0].endOffset)
	}

	if tokens[1].text != "world" || tokens[1].startOffset != 6 || tokens[1].endOffset != 11 {
		t.Errorf("Second token: expected world [6,11], got %s [%d,%d]",
			tokens[1].text, tokens[1].startOffset, tokens[1].endOffset)
	}
}

// TestLowerCaseFilter_EndMethod tests the End() method.
// Source: TestLowerCaseFilter.testEnd()
// Purpose: Tests that End() is properly propagated.
func TestLowerCaseFilter_EndMethod(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("test"))

	lowerFilter := NewLowerCaseFilter(tokenizer)
	defer lowerFilter.Close()

	for {
		hasToken, err := lowerFilter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
	}

	err := lowerFilter.End()
	if err != nil {
		t.Errorf("End() returned error: %v", err)
	}
}

// TestLowerCaseFilter_Chaining tests chaining with other filters.
// Source: TestLowerCaseFilter.testChaining()
// Purpose: Tests that LowerCaseFilter works properly in filter chains.
func TestLowerCaseFilter_Chaining(t *testing.T) {
	input := "HELLO WORLD"

	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader(input))

	lowerFilter := NewLowerCaseFilter(tokenizer)
	defer lowerFilter.Close()

	var tokens []string
	for {
		hasToken, err := lowerFilter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := lowerFilter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				tokens = append(tokens, termAttr.String())
			}
		}
	}

	expected := []string{"hello", "world"}
	if !reflect.DeepEqual(tokens, expected) {
		t.Errorf("Expected %v, got %v", expected, tokens)
	}
}

// TestLowerCaseFilter_SpecialCharacters tests special characters.
// Source: TestLowerCaseFilter.testSpecialCharacters()
// Purpose: Tests handling of special characters.
func TestLowerCaseFilter_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Mixed alphanumeric",
			input:    "Test123 ABC456 xyz789",
			expected: []string{"test123", "abc456", "xyz789"},
		},
		{
			name:     "With underscores",
			input:    "TEST_VAR another_VAR",
			expected: []string{"test_var", "another_var"},
		},
		{
			name:     "With hyphens",
			input:    "UPPER-lower MiXeD-Case",
			expected: []string{"upper-lower", "mixed-case"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			lowerFilter := NewLowerCaseFilter(tokenizer)
			defer lowerFilter.Close()

			var tokens []string
			for {
				hasToken, err := lowerFilter.IncrementToken()
				if err != nil {
					t.Fatalf("Error incrementing token: %v", err)
				}
				if !hasToken {
					break
				}
				if attr := lowerFilter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
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
