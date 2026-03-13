// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"strings"
	"testing"
)

// TestStopFilter_Basic tests basic stop word filtering.
// Source: TestStopFilter.testStopFilter()
// Purpose: Tests that stop words are removed from token stream.
func TestStopFilter_Basic(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		stopWords []string
		expected  []string
	}{
		{
			name:      "English stop words",
			input:     "The quick brown fox jumps over the lazy dog",
			stopWords: EnglishStopWords,
			expected:  []string{"quick", "brown", "fox", "jumps", "over", "lazy", "dog"},
		},
		{
			name:      "Custom stop words",
			input:     "foo bar baz qux",
			stopWords: []string{"bar", "qux"},
			expected:  []string{"foo", "baz"},
		},
		{
			name:      "All stop words",
			input:     "the a an",
			stopWords: []string{"the", "a", "an"},
			expected:  nil,
		},
		{
			name:      "No stop words",
			input:     "quick brown fox",
			stopWords: []string{"the", "a", "an"},
			expected:  []string{"quick", "brown", "fox"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			stopFilter := NewStopFilter(tokenizer, tc.stopWords)
			defer stopFilter.Close()

			var tokens []string
			for {
				hasToken, err := stopFilter.IncrementToken()
				if err != nil {
					t.Fatalf("Error incrementing token: %v", err)
				}
				if !hasToken {
					break
				}
				if attr := stopFilter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
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

// TestStopFilter_PositionIncrement tests position increments with stop words.
// Source: TestStopFilter.testPositionIncrement()
// Purpose: Tests that position increments are adjusted when stop words are removed.
func TestStopFilter_PositionIncrement(t *testing.T) {
	input := "the a quick"
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader(input))

	stopFilter := NewStopFilter(tokenizer, []string{"the", "a"})
	defer stopFilter.Close()

	type tokenInfo struct {
		text     string
		position int
	}

	var tokens []tokenInfo
	for {
		hasToken, err := stopFilter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}

		var info tokenInfo
		if attr := stopFilter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				info.text = termAttr.String()
			}
		}
		if attr := stopFilter.GetAttributeSource().GetAttribute("PositionIncrementAttribute"); attr != nil {
			if posAttr, ok := attr.(PositionIncrementAttribute); ok {
				info.position = posAttr.GetPositionIncrement()
			}
		}
		tokens = append(tokens, info)
	}

	if len(tokens) != 1 {
		t.Fatalf("Expected 1 token, got %d", len(tokens))
	}

	if tokens[0].text != "quick" || tokens[0].position != 3 {
		t.Errorf("Expected 'quick' with position 3, got '%s' with position %d",
			tokens[0].text, tokens[0].position)
	}
}

// TestStopFilter_CaseSensitivity tests case sensitivity of stop words.
// Source: TestStopFilter.testCaseSensitivity()
// Purpose: Tests that stop word matching respects case.
func TestStopFilter_CaseSensitivity(t *testing.T) {
	input := "The THE the"
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader(input))

	stopFilter := NewStopFilter(tokenizer, []string{"the"})
	defer stopFilter.Close()

	var tokens []string
	for {
		hasToken, err := stopFilter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := stopFilter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				tokens = append(tokens, termAttr.String())
			}
		}
	}

	expected := []string{"The", "THE"}
	if !reflect.DeepEqual(tokens, expected) {
		t.Errorf("Expected %v, got %v", expected, tokens)
	}
}

// TestStopFilter_AddRemove tests adding and removing stop words.
// Source: TestStopFilter.testAddRemove()
// Purpose: Tests dynamic modification of stop word set.
func TestStopFilter_AddRemove(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("foo bar baz"))

	stopFilter := NewStopFilter(tokenizer, []string{"bar"})
	defer stopFilter.Close()

	stopFilter.AddStopWord("baz")

	var tokens []string
	for {
		hasToken, err := stopFilter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := stopFilter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				tokens = append(tokens, termAttr.String())
			}
		}
	}

	expected := []string{"foo"}
	if !reflect.DeepEqual(tokens, expected) {
		t.Errorf("Expected %v, got %v", expected, tokens)
	}

	if !stopFilter.IsStopWord("bar") {
		t.Error("Expected 'bar' to be a stop word")
	}
	if stopFilter.IsStopWord("foo") {
		t.Error("Expected 'foo' not to be a stop word")
	}
}

// TestStopFilter_EmptyInput tests stop filter with empty input.
// Source: TestStopFilter.testEmpty()
// Purpose: Tests that empty input is handled correctly.
func TestStopFilter_EmptyInput(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader(""))

	stopFilter := NewStopFilter(tokenizer, EnglishStopWords)
	defer stopFilter.Close()

	tokenCount := 0
	for {
		hasToken, err := stopFilter.IncrementToken()
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

// TestStopFilter_EmptyStopWords tests stop filter with empty stop word list.
// Source: TestStopFilter.testEmptyStopWords()
// Purpose: Tests that empty stop word list passes all tokens through.
func TestStopFilter_EmptyStopWords(t *testing.T) {
	input := "the quick brown fox"
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader(input))

	stopFilter := NewStopFilter(tokenizer, []string{})
	defer stopFilter.Close()

	var tokens []string
	for {
		hasToken, err := stopFilter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := stopFilter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				tokens = append(tokens, termAttr.String())
			}
		}
	}

	expected := []string{"the", "quick", "brown", "fox"}
	if !reflect.DeepEqual(tokens, expected) {
		t.Errorf("Expected %v, got %v", expected, tokens)
	}
}

// TestStopFilter_WithLowerCase tests stop filter combined with lowercasing.
// Source: TestStopFilter.testWithLowerCase()
// Purpose: Tests interaction with LowerCaseFilter.
func TestStopFilter_WithLowerCase(t *testing.T) {
	input := "The Quick Brown Fox"

	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader(input))

	lowerFilter := NewLowerCaseFilter(tokenizer)
	stopFilter := NewStopFilter(lowerFilter, []string{"the"})
	defer stopFilter.Close()

	var tokens []string
	for {
		hasToken, err := stopFilter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := stopFilter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				tokens = append(tokens, termAttr.String())
			}
		}
	}

	expected := []string{"quick", "brown", "fox"}
	if !reflect.DeepEqual(tokens, expected) {
		t.Errorf("Expected %v, got %v", expected, tokens)
	}
}

// TestStopFilter_EndMethod tests the End() method.
// Source: TestStopFilter.testEnd()
// Purpose: Tests that End() is properly propagated.
func TestStopFilter_End(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("test"))

	stopFilter := NewStopFilter(tokenizer, []string{})
	defer stopFilter.Close()

	for {
		hasToken, err := stopFilter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
	}

	err := stopFilter.End()
	if err != nil {
		t.Errorf("End() returned error: %v", err)
	}
}

// TestStopFilter_EnglishStopWords tests the default English stop words.
// Source: TestStopFilter.testEnglishStopWords()
// Purpose: Tests the built-in English stop word list.
func TestStopFilter_EnglishStopWords(t *testing.T) {
	if len(EnglishStopWords) == 0 {
		t.Error("EnglishStopWords should not be empty")
	}

	commonStops := []string{"the", "a", "an", "and", "or", "in", "is", "it", "to"}
	stopSet := make(map[string]bool)
	for _, word := range EnglishStopWords {
		stopSet[word] = true
	}

	for _, word := range commonStops {
		if !stopSet[word] {
			t.Errorf("Expected '%s' to be in EnglishStopWords", word)
		}
	}
}
