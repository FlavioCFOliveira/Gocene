// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"strings"
	"testing"
)

// getTokenAttribute helper to extract CharTermAttribute from a TokenStream
func getTokenAttribute(stream TokenStream) (CharTermAttribute, bool) {
	// TokenStream is actually a TokenFilter wrapping a Tokenizer
	// The attributes are stored in the Tokenizer's AttributeSource

	// Try to unwrap through TokenFilter chain
	current := stream
	for {
		// Check if current has GetInput method (TokenFilter)
		if tf, ok := current.(interface{ GetInput() TokenStream }); ok {
			input := tf.GetInput()
			// Check if input has GetAttributeSource (Tokenizer or BaseTokenStream)
			if hasAttrSrc, ok := input.(interface{ GetAttributeSource() *AttributeSource }); ok {
				attrSrc := hasAttrSrc.GetAttributeSource()
				attr := attrSrc.GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
				if cta, ok := attr.(CharTermAttribute); ok {
					return cta, true
				}
				return nil, false
			}
			// Continue unwrapping
			current = input
		} else {
			// No more wrapping, check if current itself has attributes
			if hasAttrSrc, ok := current.(interface{ GetAttributeSource() *AttributeSource }); ok {
				attrSrc := hasAttrSrc.GetAttributeSource()
				attr := attrSrc.GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
				if cta, ok := attr.(CharTermAttribute); ok {
					return cta, true
				}
			}
			break
		}
	}
	return nil, false
}

func TestSimpleAnalyzer_Basic(t *testing.T) {
	analyzer := NewSimpleAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input    string
		expected []string
	}{
		{
			input:    "Hello World",
			expected: []string{"hello", "world"},
		},
		{
			input:    "HELLO WORLD",
			expected: []string{"hello", "world"},
		},
		{
			input:    "Hello123 World456",
			expected: []string{"hello", "world"},
		},
		{
			input:    "Hello, World! How are you?",
			expected: []string{"hello", "world", "how", "are", "you"},
		},
		{
			input:    "Mixed-Case_Example",
			expected: []string{"mixed", "case", "example"},
		},
		{
			input:    "the quick brown fox",
			expected: []string{"the", "quick", "brown", "fox"}, // No stop word filtering
		},
		{
			input:    "C++ Java# Python@",
			expected: []string{"c", "java", "python"},
		},
		{
			input:    "",
			expected: nil,
		},
		{
			input:    "123 456",
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

				if cta, ok := getTokenAttribute(stream); ok {
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

func TestSimpleAnalyzer_Lowercasing(t *testing.T) {
	analyzer := NewSimpleAnalyzer()
	defer analyzer.Close()

	// Test that tokens are lowercased
	stream, _ := analyzer.TokenStream("field", strings.NewReader("UPPER lower Mixed"))
	defer stream.Close()

	expected := []string{"upper", "lower", "mixed"}
	var tokens []string

	for {
		hasToken, _ := stream.IncrementToken()
		if !hasToken {
			break
		}
		if cta, ok := getTokenAttribute(stream); ok {
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

func TestSimpleAnalyzer_Unicode(t *testing.T) {
	analyzer := NewSimpleAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input    string
		expected []string
	}{
		{
			input:    "CAFÉ Résumé",
			expected: []string{"café", "résumé"},
		},
		{
			input:    "Héllo Wörld",
			expected: []string{"héllo", "wörld"},
		},
		{
			input:    "Привет мир",
			expected: []string{"привет", "мир"}, // Russian letters are lowercased
		},
		{
			input:    "日本語 テスト",
			expected: []string{"日本語", "テスト"}, // Japanese has no case
		},
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
				if cta, ok := getTokenAttribute(stream); ok {
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

func TestSimpleAnalyzer_Reuse(t *testing.T) {
	analyzer := NewSimpleAnalyzer()
	defer analyzer.Close()

	// First text
	stream1, _ := analyzer.TokenStream("field", strings.NewReader("First"))
	tokens1 := collectSimpleTokens(stream1)
	if len(tokens1) != 1 || tokens1[0] != "first" {
		t.Errorf("First stream failed: %v", tokens1)
	}
	stream1.Close()

	// Second text - reuse
	stream2, _ := analyzer.TokenStream("field", strings.NewReader("Second"))
	tokens2 := collectSimpleTokens(stream2)
	if len(tokens2) != 1 || tokens2[0] != "second" {
		t.Errorf("Second stream failed: %v", tokens2)
	}
	stream2.Close()
}

func collectSimpleTokens(stream TokenStream) []string {
	var tokens []string
	for {
		hasToken, _ := stream.IncrementToken()
		if !hasToken {
			break
		}
		if cta, ok := getTokenAttribute(stream); ok {
			tokens = append(tokens, cta.String())
		}
	}
	return tokens
}

func TestSimpleAnalyzer_NoStopWords(t *testing.T) {
	analyzer := NewSimpleAnalyzer()
	defer analyzer.Close()

	// SimpleAnalyzer does NOT remove stop words
	stream, _ := analyzer.TokenStream("field", strings.NewReader("the and or is"))
	defer stream.Close()

	// All tokens should be present (lowercased)
	expected := []string{"the", "and", "or", "is"}
	tokens := collectSimpleTokens(stream)

	if len(tokens) != len(expected) {
		t.Fatalf("Expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
	}

	for i := range expected {
		if tokens[i] != expected[i] {
			t.Errorf("Token %d: expected %q, got %q", i, expected[i], tokens[i])
		}
	}
}
