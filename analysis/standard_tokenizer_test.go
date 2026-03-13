// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"strings"
	"testing"
)

// TestStandardTokenizer_Basic tests basic tokenization.
// Source: TestStandardTokenizer.testBasic()
// Purpose: Tests standard word tokenization.
func TestStandardTokenizer_Basic(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Simple words",
			input:    "The quick brown fox",
			expected: []string{"The", "quick", "brown", "fox"},
		},
		{
			name:     "With punctuation",
			input:    "Hello, world! How are you?",
			expected: []string{"Hello", "world", "How", "are", "you"},
		},
		{
			name:     "Mixed alphanumeric",
			input:    "Test123 ABC456 xyz789",
			expected: []string{"Test123", "ABC456", "xyz789"},
		},
		{
			name:     "Numbers only",
			input:    "123 456 789",
			expected: []string{"123", "456", "789"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewStandardTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))
			defer tokenizer.Close()

			var tokens []string
			for {
				hasToken, err := tokenizer.IncrementToken()
				if err != nil {
					t.Fatalf("Error incrementing token: %v", err)
				}
				if !hasToken {
					break
				}
				if attr := tokenizer.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
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

// TestStandardTokenizer_Offsets tests offset tracking.
// Source: TestStandardTokenizer.testOffsets()
// Purpose: Tests character offset tracking.
func TestStandardTokenizer_Offsets(t *testing.T) {
	tokenizer := NewStandardTokenizer()
	tokenizer.SetReader(strings.NewReader("Hello World"))
	defer tokenizer.Close()

	type tokenInfo struct {
		text     string
		startOff int
		endOff   int
	}

	var tokens []tokenInfo
	for {
		hasToken, err := tokenizer.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}

		var info tokenInfo
		if attr := tokenizer.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				info.text = termAttr.String()
			}
		}
		if attr := tokenizer.GetAttributeSource().GetAttribute("OffsetAttribute"); attr != nil {
			if offsetAttr, ok := attr.(OffsetAttribute); ok {
				info.startOff = offsetAttr.StartOffset()
				info.endOff = offsetAttr.EndOffset()
			}
		}
		tokens = append(tokens, info)
	}

	if len(tokens) != 2 {
		t.Fatalf("Expected 2 tokens, got %d", len(tokens))
	}

	if tokens[0].text != "Hello" || tokens[0].startOff != 0 || tokens[0].endOff != 5 {
		t.Errorf("First token: expected Hello [0,5], got %s [%d,%d]",
			tokens[0].text, tokens[0].startOff, tokens[0].endOff)
	}

	if tokens[1].text != "World" || tokens[1].startOff != 6 || tokens[1].endOff != 11 {
		t.Errorf("Second token: expected World [6,11], got %s [%d,%d]",
			tokens[1].text, tokens[1].startOff, tokens[1].endOff)
	}
}

// TestStandardTokenizer_PositionIncrement tests position increments.
// Source: TestStandardTokenizer.testPositionIncrement()
// Purpose: Tests position increment attribute.
func TestStandardTokenizer_PositionIncrement(t *testing.T) {
	tokenizer := NewStandardTokenizer()
	tokenizer.SetReader(strings.NewReader("one two three"))
	defer tokenizer.Close()

	positions := []int{}
	for {
		hasToken, err := tokenizer.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := tokenizer.GetAttributeSource().GetAttribute("PositionIncrementAttribute"); attr != nil {
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

// TestStandardTokenizer_EmptyInput tests empty input.
// Source: TestStandardTokenizer.testEmptyInput()
// Purpose: Tests empty input handling.
func TestStandardTokenizer_EmptyInput(t *testing.T) {
	tokenizer := NewStandardTokenizer()
	tokenizer.SetReader(strings.NewReader(""))
	defer tokenizer.Close()

	tokenCount := 0
	for {
		hasToken, err := tokenizer.IncrementToken()
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

// TestStandardTokenizer_Unicode tests Unicode handling.
// Source: TestStandardTokenizer.testUnicode()
// Purpose: Tests Unicode text segmentation.
func TestStandardTokenizer_Unicode(t *testing.T) {
	tests := []struct {
		name  string
		input string
		count int
	}{
		{
			name:  "Accented characters",
			input: "café résumé",
			count: 2,
		},
		{
			name:  "CJK characters",
			input: "日本語テスト",
			count: 1,
		},
		{
			name:  "Emoji",
			input: "test 😀 emoji",
			count: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewStandardTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))
			defer tokenizer.Close()

			tokenCount := 0
			for {
				hasToken, err := tokenizer.IncrementToken()
				if err != nil {
					t.Fatalf("Error incrementing token: %v", err)
				}
				if !hasToken {
					break
				}
				tokenCount++
			}

			if tokenCount != tc.count {
				t.Errorf("Expected %d tokens, got %d", tc.count, tokenCount)
			}
		})
	}
}

// TestStandardTokenizer_MaxTokenLength tests max token length.
// Source: TestStandardTokenizer.testMaxTokenLength()
// Purpose: Tests handling of very long tokens.
func TestStandardTokenizer_MaxTokenLength(t *testing.T) {
	longWord := strings.Repeat("a", 1000)

	tokenizer := NewStandardTokenizer()
	tokenizer.SetReader(strings.NewReader(longWord))
	defer tokenizer.Close()

	hasToken, err := tokenizer.IncrementToken()
	if err != nil {
		t.Fatalf("Error incrementing token: %v", err)
	}
	if !hasToken {
		t.Error("Expected to get the long token")
	}

	if attr := tokenizer.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
		if termAttr, ok := attr.(CharTermAttribute); ok {
			if len(termAttr.String()) != 1000 {
				t.Errorf("Expected token length 1000, got %d", len(termAttr.String()))
			}
		}
	}
}

// TestStandardTokenizer_Reuse tests tokenizer reuse.
// Source: TestStandardTokenizer.testReuse()
// Purpose: Tests reusing tokenizer with new input.
func TestStandardTokenizer_Reuse(t *testing.T) {
	tokenizer := NewStandardTokenizer()

	tokenizer.SetReader(strings.NewReader("first test"))

	var tokens1 []string
	for {
		hasToken, err := tokenizer.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := tokenizer.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				tokens1 = append(tokens1, termAttr.String())
			}
		}
	}

	tokenizer.Reset()
	tokenizer.SetReader(strings.NewReader("second run"))

	var tokens2 []string
	for {
		hasToken, err := tokenizer.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := tokenizer.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				tokens2 = append(tokens2, termAttr.String())
			}
		}
	}

	tokenizer.Close()

	if !reflect.DeepEqual(tokens1, []string{"first", "test"}) {
		t.Errorf("First run: expected [first test], got %v", tokens1)
	}
	if !reflect.DeepEqual(tokens2, []string{"second", "run"}) {
		t.Errorf("Second run: expected [second run], got %v", tokens2)
	}
}

// TestStandardTokenizer_EndMethod tests the End() method.
// Source: TestStandardTokenizer.testEnd()
// Purpose: Tests end-of-stream operations.
func TestStandardTokenizer_EndMethod(t *testing.T) {
	tokenizer := NewStandardTokenizer()
	tokenizer.SetReader(strings.NewReader("test"))

	for {
		hasToken, err := tokenizer.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
	}

	err := tokenizer.End()
	if err != nil {
		t.Errorf("End() returned error: %v", err)
	}

	tokenizer.Close()
}

// TestStandardTokenizer_Numbers tests number tokenization.
// Source: TestStandardTokenizer.testNumbers()
// Purpose: Tests number handling.
func TestStandardTokenizer_Numbers(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Simple numbers",
			input:    "123 456",
			expected: []string{"123", "456"},
		},
		{
			name:     "Mixed alphanumeric",
			input:    "ABC123 XYZ789",
			expected: []string{"ABC123", "XYZ789"},
		},
		{
			name:     "Numbers and words",
			input:    "test123 456abc",
			expected: []string{"test123", "456abc"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewStandardTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))
			defer tokenizer.Close()

			var tokens []string
			for {
				hasToken, err := tokenizer.IncrementToken()
				if err != nil {
					t.Fatalf("Error incrementing token: %v", err)
				}
				if !hasToken {
					break
				}
				if attr := tokenizer.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
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

// TestStandardTokenizer_Whitespace tests whitespace handling.
// Source: TestStandardTokenizer.testWhitespace()
// Purpose: Tests various whitespace characters.
func TestStandardTokenizer_Whitespace(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Spaces",
			input:    "one   two",
			expected: []string{"one", "two"},
		},
		{
			name:     "Tabs",
			input:    "one\ttwo",
			expected: []string{"one", "two"},
		},
		{
			name:     "Newlines",
			input:    "one\ntwo\r\nthree",
			expected: []string{"one", "two", "three"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewStandardTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))
			defer tokenizer.Close()

			var tokens []string
			for {
				hasToken, err := tokenizer.IncrementToken()
				if err != nil {
					t.Fatalf("Error incrementing token: %v", err)
				}
				if !hasToken {
					break
				}
				if attr := tokenizer.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
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

// TestStandardTokenizer_AttributesExist tests attribute existence.
// Source: TestStandardTokenizer.testAttributes()
// Purpose: Tests that required attributes exist.
func TestStandardTokenizer_AttributesExist(t *testing.T) {
	tokenizer := NewStandardTokenizer()

	attrSource := tokenizer.GetAttributeSource()
	if attrSource == nil {
		t.Fatal("Expected non-nil AttributeSource")
	}

	if !attrSource.HasAttribute(reflect.TypeOf(&charTermAttribute{})) {
		t.Error("Expected CharTermAttribute to exist")
	}

	if !attrSource.HasAttribute(reflect.TypeOf(&offsetAttribute{})) {
		t.Error("Expected OffsetAttribute to exist")
	}

	if !attrSource.HasAttribute(reflect.TypeOf(&positionIncrementAttribute{})) {
		t.Error("Expected PositionIncrementAttribute to exist")
	}

	tokenizer.Close()
}
