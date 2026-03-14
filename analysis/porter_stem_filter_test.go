// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"strings"
	"testing"
)

// TestPorterStemFilter_Basic tests basic stemming functionality.
// Source: TestPorterStemFilter.java
// Purpose: Tests that words are properly stemmed.
func TestPorterStemFilter_Basic(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"running", "run"},
		{"flies", "fli"},
		{"dies", "die"},
		{"mules", "mule"},
		{"denied", "deni"},
		{"died", "die"},
		{"agreed", "agre"},
		{"owned", "own"},
		{"humbled", "humbl"},
		{"sized", "size"},
		{"meeting", "meet"},
		{"stating", "state"},
		{"sensational", "sensat"},
		{"traditional", "tradit"},
		{"reference", "refer"},
		{"plotted", "plot"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			stemFilter := NewPorterStemFilter(tokenizer)
			defer stemFilter.Close()

			var tokens []string
			for {
				hasToken, err := stemFilter.IncrementToken()
				if err != nil {
					t.Fatalf("Error: %v", err)
				}
				if !hasToken {
					break
				}
				if attr := stemFilter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
					if termAttr, ok := attr.(CharTermAttribute); ok {
						tokens = append(tokens, termAttr.String())
					}
				}
			}

			if len(tokens) != 1 {
				t.Fatalf("Expected 1 token, got %d", len(tokens))
			}

			if tokens[0] != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, tokens[0])
			}
		})
	}
}

// TestPorterStemFilter_MultipleWords tests stemming multiple words.
// Source: TestPorterStemFilter.java
// Purpose: Tests that multiple words are properly stemmed.
func TestPorterStemFilter_MultipleWords(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("running jumping swimming"))

	stemFilter := NewPorterStemFilter(tokenizer)
	defer stemFilter.Close()

	var tokens []string
	for {
		hasToken, err := stemFilter.IncrementToken()
		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := stemFilter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				tokens = append(tokens, termAttr.String())
			}
		}
	}

	expected := []string{"run", "jump", "swim"}
	if len(tokens) != len(expected) {
		t.Errorf("Expected %v, got %v", expected, tokens)
	}

	for i, token := range tokens {
		if token != expected[i] {
			t.Errorf("Token %d: expected %q, got %q", i, expected[i], token)
		}
	}
}

// TestPorterStemFilter_EmptyTokens tests handling of empty tokens.
// Source: TestPorterStemFilter.java
// Purpose: Tests that empty tokens are handled correctly.
func TestPorterStemFilter_EmptyTokens(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("a an"))

	stemFilter := NewPorterStemFilter(tokenizer)
	defer stemFilter.Close()

	var tokens []string
	for {
		hasToken, err := stemFilter.IncrementToken()
		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := stemFilter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				tokens = append(tokens, termAttr.String())
			}
		}
	}

	// Short words should remain unchanged
	if len(tokens) != 2 {
		t.Errorf("Expected 2 tokens, got %d", len(tokens))
	}
}

// TestPorterStemmer_Direct tests the PorterStemmer directly.
func TestPorterStemmer_Direct(t *testing.T) {
	stemmer := NewPorterStemmer()

	tests := []struct {
		input    string
		expected string
	}{
		{"cats", "cat"},
		{"running", "run"},
		{"national", "nation"},
		{"rationality", "ration"},
		{"rational", "ration"},
		{"rationalization", "ration"},
		{"rationalizations", "ration"},
		{"rationalizing", "ration"},
		{"rationalized", "ration"},
		{"rationalizes", "ration"},
		{"rationalizer", "ration"},
		{"rationalizers", "ration"},
		{"rationalizing", "ration"},
		{"rationalized", "ration"},
		{"rationalizes", "ration"},
		{"rationalizer", "ration"},
		{"rationalizers", "ration"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := stemmer.Stem(tc.input)
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}

// TestPorterStemmer_ShortWords tests that short words are not modified.
func TestPorterStemmer_ShortWords(t *testing.T) {
	stemmer := NewPorterStemmer()

	shortWords := []string{"a", "ab", "abc"}
	for _, word := range shortWords {
		result := stemmer.Stem(word)
		if result != word {
			t.Errorf("Short word %q should not be modified, got %q", word, result)
		}
	}
}
