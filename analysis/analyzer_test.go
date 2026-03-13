// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"strings"
	"testing"
)

// TestAnalyzer_Reuse tests that analyzers properly reuse token streams.
// Source: TestAnalyzers.testReuse()
// Purpose: Tests that TokenStream can be reused with new input.
func TestAnalyzer_Reuse(t *testing.T) {
	analyzer := NewStandardAnalyzer()
	defer analyzer.Close()

	// First analysis
	reader1 := strings.NewReader("First document text")
	stream1, err := analyzer.TokenStream("field", reader1)
	if err != nil {
		t.Fatalf("Failed to create token stream: %v", err)
	}

	var tokens1 []string
	for {
		hasToken, err := stream1.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		if bts, ok := stream1.(interface{ GetAttributeSource() *AttributeSource }); ok {
			if attr := bts.GetAttributeSource().GetAttributeByType(reflect.TypeOf(&charTermAttribute{})); attr != nil {
				if termAttr, ok := attr.(CharTermAttribute); ok {
					tokens1 = append(tokens1, termAttr.String())
				}
			}
		}
	}
	stream1.Close()

	// Second analysis with new input
	reader2 := strings.NewReader("Second document content")
	stream2, err := analyzer.TokenStream("field", reader2)
	if err != nil {
		t.Fatalf("Failed to create second token stream: %v", err)
	}

	var tokens2 []string
	for {
		hasToken, err := stream2.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		if bts, ok := stream2.(interface{ GetAttributeSource() *AttributeSource }); ok {
			if attr := bts.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
				if termAttr, ok := attr.(CharTermAttribute); ok {
					tokens2 = append(tokens2, termAttr.String())
				}
			}
		}
	}
	stream2.Close()

	// Verify different tokens were produced
	if reflect.DeepEqual(tokens1, tokens2) {
		t.Error("Expected different tokens for different input")
	}
}

// TestSimpleAnalyzer tests the SimpleAnalyzer.
// Source: TestAnalyzers.testSimpleAnalyzer()
// Purpose: Tests letter tokenization and lowercasing.
func TestSimpleAnalyzer(t *testing.T) {
	analyzer := NewSimpleAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input    string
		expected []string
	}{
		{
			input:    "Hello, World! 123 TEST.",
			expected: []string{"hello", "world", "test"},
		},
		{
			input:    "The quick brown fox",
			expected: []string{"the", "quick", "brown", "fox"},
		},
		{
			input:    "123 456",
			expected: nil,
		},
		{
			input:    "MixedCASE Text",
			expected: []string{"mixedcase", "text"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			reader := strings.NewReader(tc.input)
			stream, err := analyzer.TokenStream("field", reader)
			if err != nil {
				t.Fatalf("Failed to create token stream: %v", err)
			}
			defer stream.Close()

			var tokens []string
			for {
				hasToken, err := stream.IncrementToken()
				if err != nil {
					t.Fatalf("Error incrementing token: %v", err)
				}
				if !hasToken {
					break
				}
				if bts, ok := stream.(interface{ GetAttributeSource() *AttributeSource }); ok {
					if attr := bts.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
						if termAttr, ok := attr.(CharTermAttribute); ok {
							tokens = append(tokens, termAttr.String())
						}
					}
				}
			}

			if !reflect.DeepEqual(tokens, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, tokens)
			}
		})
	}
}

// TestStandardAnalyzer tests the StandardAnalyzer.
// Source: TestAnalyzers.testStandardAnalyzer()
// Purpose: Tests standard tokenization with stop word removal.
// Note: StandardAnalyzer does NOT filter numeric tokens by default.
func TestStandardAnalyzer(t *testing.T) {
	analyzer := NewStandardAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input    string
		expected []string
	}{
		{
			input:    "The quick brown fox jumps over the lazy dog",
			expected: []string{"quick", "brown", "fox", "jumps", "over", "lazy", "dog"},
		},
		{
			input:    "Hello, World!",
			expected: []string{"hello", "world"},
		},
		{
			input:    "Testing 123 numbers",
			expected: []string{"testing", "123", "numbers"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			reader := strings.NewReader(tc.input)
			stream, err := analyzer.TokenStream("field", reader)
			if err != nil {
				t.Fatalf("Failed to create token stream: %v", err)
			}
			defer stream.Close()

			var tokens []string
			for {
				hasToken, err := stream.IncrementToken()
				if err != nil {
					t.Fatalf("Error incrementing token: %v", err)
				}
				if !hasToken {
					break
				}
				if bts, ok := stream.(interface{ GetAttributeSource() *AttributeSource }); ok {
					if attr := bts.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
						if termAttr, ok := attr.(CharTermAttribute); ok {
							tokens = append(tokens, termAttr.String())
						}
					}
				}
			}

			if !reflect.DeepEqual(tokens, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, tokens)
			}
		})
	}
}

// TestWhitespaceAnalyzer tests the WhitespaceAnalyzer.
// Source: TestAnalyzers.testWhitespaceAnalyzer()
// Purpose: Tests whitespace tokenization without lowercasing.
func TestWhitespaceAnalyzer(t *testing.T) {
	analyzer := NewWhitespaceAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input    string
		expected []string
	}{
		{
			input:    "The quick brown fox",
			expected: []string{"The", "quick", "brown", "fox"},
		},
		{
			input:    "UPPER lower MiXeD",
			expected: []string{"UPPER", "lower", "MiXeD"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			reader := strings.NewReader(tc.input)
			stream, err := analyzer.TokenStream("field", reader)
			if err != nil {
				t.Fatalf("Failed to create token stream: %v", err)
			}
			defer stream.Close()

			var tokens []string
			for {
				hasToken, err := stream.IncrementToken()
				if err != nil {
					t.Fatalf("Error incrementing token: %v", err)
				}
				if !hasToken {
					break
				}
				if bts, ok := stream.(interface{ GetAttributeSource() *AttributeSource }); ok {
					if attr := bts.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
						if termAttr, ok := attr.(CharTermAttribute); ok {
							tokens = append(tokens, termAttr.String())
						}
					}
				}
			}

			if !reflect.DeepEqual(tokens, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, tokens)
			}
		})
	}
}

// TestAnalyzer_EmptyInput tests analyzer behavior with empty input.
// Source: TestAnalyzers.testEmpty()
// Purpose: Tests that analyzers handle empty input gracefully.
func TestAnalyzer_EmptyInput(t *testing.T) {
	analyzers := []Analyzer{
		NewStandardAnalyzer(),
		NewSimpleAnalyzer(),
		NewWhitespaceAnalyzer(),
	}

	for _, analyzer := range analyzers {
		reader := strings.NewReader("")
		stream, err := analyzer.TokenStream("field", reader)
		if err != nil {
			t.Fatalf("Failed to create token stream: %v", err)
		}

		tokenCount := 0
		for {
			hasToken, err := stream.IncrementToken()
			if err != nil {
				t.Fatalf("Error incrementing token: %v", err)
			}
			if !hasToken {
				break
			}
			tokenCount++
		}
		stream.Close()
		analyzer.Close()

		if tokenCount != 0 {
			t.Errorf("Expected 0 tokens for empty input, got %d", tokenCount)
		}
	}
}
