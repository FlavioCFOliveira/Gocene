// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"strings"
	"testing"
)

// getWhitespaceTokenAttribute helper to extract CharTermAttribute from WhitespaceTokenizer
func getWhitespaceTokenAttribute(stream TokenStream) (CharTermAttribute, bool) {
	// WhitespaceAnalyzer returns WhitespaceTokenizer directly
	if wt, ok := stream.(*WhitespaceTokenizer); ok {
		attr := wt.GetAttributeSource().GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
		if cta, ok := attr.(CharTermAttribute); ok {
			return cta, true
		}
	}
	return nil, false
}

func TestWhitespaceAnalyzer_Basic(t *testing.T) {
	analyzer := NewWhitespaceAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input    string
		expected []string
	}{
		{
			input:    "Hello World",
			expected: []string{"Hello", "World"},
		},
		{
			input:    "Hello   World",
			expected: []string{"Hello", "World"},
		},
		{
			input:    "CASE preserved Tokens",
			expected: []string{"CASE", "preserved", "Tokens"},
		},
		{
			input:    "mixed123 Numbers456",
			expected: []string{"mixed123", "Numbers456"},
		},
		{
			input:    "test@test.com http://example.com",
			expected: []string{"test@test.com", "http://example.com"},
		},
		{
			input:    "",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			stream, err := analyzer.TokenStream("field", strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("TokenStream failed: %v", err)
			}
			defer stream.Close()

			var tokens []string
			for {
				hasToken, err := stream.IncrementToken()
				if err != nil {
					t.Fatalf("IncrementToken failed: %v", err)
				}
				if !hasToken {
					break
				}

				if cta, ok := getWhitespaceTokenAttribute(stream); ok {
					tokens = append(tokens, cta.String())
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

func TestWhitespaceAnalyzer_CasePreservation(t *testing.T) {
	analyzer := NewWhitespaceAnalyzer()
	defer analyzer.Close()

	// Test that case is preserved
	stream, _ := analyzer.TokenStream("field", strings.NewReader("UPPER lower Mixed"))
	defer stream.Close()

	expected := []string{"UPPER", "lower", "Mixed"}
	var tokens []string

	for {
		hasToken, _ := stream.IncrementToken()
		if !hasToken {
			break
		}
		if cta, ok := getWhitespaceTokenAttribute(stream); ok {
			tokens = append(tokens, cta.String())
		}
	}

	if len(tokens) != len(expected) {
		t.Fatalf("Expected %d tokens, got %d", len(expected), len(tokens))
	}

	for i := range expected {
		if tokens[i] != expected[i] {
			t.Errorf("Token %d: expected %q, got %q", i, expected[i], tokens[i])
		}
	}
}

func TestWhitespaceAnalyzer_UnicodeWhitespace(t *testing.T) {
	analyzer := NewWhitespaceAnalyzer()
	defer analyzer.Close()

	// Test various Unicode whitespace characters
	tests := []struct {
		input    string
		expected []string
	}{
		{"Hello\tWorld", []string{"Hello", "World"}},     // Tab
		{"Hello\nWorld", []string{"Hello", "World"}},     // Newline
		{"Hello\r\nWorld", []string{"Hello", "World"}},   // Carriage return + newline
		{"Hello\u00A0World", []string{"Hello", "World"}}, // Non-breaking space
	}

	for _, tt := range tests {
		t.Run("unicode", func(t *testing.T) {
			stream, _ := analyzer.TokenStream("field", strings.NewReader(tt.input))
			defer stream.Close()

			var tokens []string
			for {
				hasToken, _ := stream.IncrementToken()
				if !hasToken {
					break
				}
				if cta, ok := getWhitespaceTokenAttribute(stream); ok {
					tokens = append(tokens, cta.String())
				}
			}

			if len(tokens) != len(tt.expected) {
				t.Errorf("Expected %d tokens, got %d: %v", len(tt.expected), len(tokens), tokens)
				return
			}

			for i := range tt.expected {
				if tokens[i] != tt.expected[i] {
					t.Errorf("Token %d: expected %q, got %q", i, tt.expected[i], tokens[i])
				}
			}
		})
	}
}

func TestWhitespaceAnalyzer_Reuse(t *testing.T) {
	analyzer := NewWhitespaceAnalyzer()
	defer analyzer.Close()

	// First text
	stream1, _ := analyzer.TokenStream("field", strings.NewReader("First"))
	tokens1 := collectWhitespaceTokens(stream1)
	if len(tokens1) != 1 || tokens1[0] != "First" {
		t.Errorf("First stream failed: %v", tokens1)
	}
	stream1.Close()

	// Second text - reuse
	stream2, _ := analyzer.TokenStream("field", strings.NewReader("Second"))
	tokens2 := collectWhitespaceTokens(stream2)
	if len(tokens2) != 1 || tokens2[0] != "Second" {
		t.Errorf("Second stream failed: %v", tokens2)
	}
	stream2.Close()
}

func collectWhitespaceTokens(stream TokenStream) []string {
	var tokens []string
	for {
		hasToken, _ := stream.IncrementToken()
		if !hasToken {
			break
		}
		if cta, ok := getWhitespaceTokenAttribute(stream); ok {
			tokens = append(tokens, cta.String())
		}
	}
	return tokens
}
