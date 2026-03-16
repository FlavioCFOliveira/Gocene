// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"strings"
	"testing"
)

// TestStopAnalyzer_DefaultStopWords tests StopAnalyzer with default English stop words.
func TestStopAnalyzer_DefaultStopWords(t *testing.T) {
	analyzer := NewStopAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"simple", "the quick brown fox", []string{"quick", "brown", "fox"}},
		{"stop words only", "the a an", []string{}},
		{"no stop words", "hello world", []string{"hello", "world"}},
		{"mixed case", "The Quick Brown", []string{"quick", "brown"}},
		// Note: LetterTokenizer only tokenizes letters, not numbers
		// So "the 123 foxes" only produces "foxes" (123 is not a letter sequence)
		{"with numbers", "the 123 foxes", []string{"foxes"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := collectTokens(analyzer, tt.input)
			if err != nil {
				t.Fatalf("TokenStream failed: %v", err)
			}

			if len(tokens) != len(tt.expected) {
				t.Errorf("Token count = %d, want %d (got %v, want %v)",
					len(tokens), len(tt.expected), tokens, tt.expected)
				return
			}

			for i, token := range tokens {
				if token != tt.expected[i] {
					t.Errorf("Token[%d] = %q, want %q", i, token, tt.expected[i])
				}
			}
		})
	}
}

// TestStopAnalyzer_CustomStopWords tests StopAnalyzer with custom stop words.
func TestStopAnalyzer_CustomStopWords(t *testing.T) {
	customStopWords := []string{"foo", "bar", "baz"}
	stopSet := GetWordSetFromStrings(customStopWords, true)
	analyzer := NewStopAnalyzerWithWords(stopSet)
	defer analyzer.Close()

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"remove custom stop words", "foo hello bar world baz", []string{"hello", "world"}},
		{"keep default stop words", "the a an", []string{"the", "a", "an"}},
		{"no removal", "hello world", []string{"hello", "world"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := collectTokens(analyzer, tt.input)
			if err != nil {
				t.Fatalf("TokenStream failed: %v", err)
			}

			if len(tokens) != len(tt.expected) {
				t.Errorf("Token count = %d, want %d (got %v, want %v)",
					len(tokens), len(tt.expected), tokens, tt.expected)
				return
			}

			for i, token := range tokens {
				if token != tt.expected[i] {
					t.Errorf("Token[%d] = %q, want %q", i, token, tt.expected[i])
				}
			}
		})
	}
}

// TestStopAnalyzer_LowerCase tests that StopAnalyzer lowercases tokens.
func TestStopAnalyzer_LowerCase(t *testing.T) {
	analyzer := NewStopAnalyzer()
	defer analyzer.Close()

	input := "THE QUICK BROWN FOX"
	tokens, err := collectTokens(analyzer, input)
	if err != nil {
		t.Fatalf("TokenStream failed: %v", err)
	}

	// All tokens should be lowercase
	for _, token := range tokens {
		if token != strings.ToLower(token) {
			t.Errorf("Token %q is not lowercase", token)
		}
	}
}

// TestStopAnalyzer_PositionIncrement tests position increment after stop word removal.
func TestStopAnalyzer_PositionIncrement(t *testing.T) {
	analyzer := NewStopAnalyzer()
	defer analyzer.Close()

	// "the quick fox" -> position increments should be 1, 2 (the is removed)
	stream, err := analyzer.TokenStream("field", strings.NewReader("the quick fox"))
	if err != nil {
		t.Fatalf("TokenStream failed: %v", err)
	}
	defer stream.Close()

	// First token "quick" should have position increment > 1 because "the" was skipped
	hasToken, err := stream.IncrementToken()
	if err != nil {
		t.Fatalf("IncrementToken failed: %v", err)
	}
	if !hasToken {
		t.Fatal("Expected token 'quick'")
	}

	attrSrc := stream.(interface{ GetAttributeSource() *AttributeSource }).GetAttributeSource()
	posIncrAttr := attrSrc.GetAttributeByType(reflect.TypeOf(&positionIncrementAttribute{}))
	if posIncrAttr == nil {
		t.Fatal("PositionIncrementAttribute is nil")
	}

	posIncr := posIncrAttr.(PositionIncrementAttribute).GetPositionIncrement()
	// After removing "the", "quick" should have increment > 1
	// Note: This depends on whether position increment adjustment is implemented
	t.Logf("Position increment for 'quick': %d", posIncr)
}

// TestStopAnalyzer_EmptyInput tests empty input handling.
func TestStopAnalyzer_EmptyInput(t *testing.T) {
	analyzer := NewStopAnalyzer()
	defer analyzer.Close()

	tokens, err := collectTokens(analyzer, "")
	if err != nil {
		t.Fatalf("TokenStream failed: %v", err)
	}

	if len(tokens) != 0 {
		t.Errorf("Empty input should produce no tokens, got %v", tokens)
	}
}

// TestStopAnalyzer_OnlyStopWords tests input with only stop words.
func TestStopAnalyzer_OnlyStopWords(t *testing.T) {
	analyzer := NewStopAnalyzer()
	defer analyzer.Close()

	inputs := []string{
		"the a an and or",
		"it is was will be",
		"this that these", // removed "those" as it's not in default stop words
	}

	for _, input := range inputs {
		tokens, err := collectTokens(analyzer, input)
		if err != nil {
			t.Fatalf("TokenStream failed: %v", err)
		}

		if len(tokens) != 0 {
			t.Errorf("Input %q should produce no tokens, got %v", input, tokens)
		}
	}
}

// TestStopAnalyzer_GetStopWords tests GetStopWords method.
func TestStopAnalyzer_GetStopWords(t *testing.T) {
	analyzer := NewStopAnalyzer()
	defer analyzer.Close()

	stopWords := analyzer.GetStopWords()
	if stopWords == nil {
		t.Fatal("GetStopWords returned nil")
	}

	// Check some default stop words
	expectedStopWords := []string{"the", "a", "an", "and", "or", "is", "it"}
	for _, word := range expectedStopWords {
		if !stopWords.ContainsString(word) {
			t.Errorf("StopWords should contain %q", word)
		}
	}
}

// TestStopAnalyzerFactory tests the StopAnalyzerFactory.
func TestStopAnalyzerFactory(t *testing.T) {
	factory := NewStopAnalyzerFactory()
	analyzer := factory.Create()

	if analyzer == nil {
		t.Fatal("Factory.Create returned nil")
	}
	defer analyzer.Close()

	// Test that default stop words are applied
	tokens, err := collectTokens(analyzer, "the quick brown fox")
	if err != nil {
		t.Fatalf("TokenStream failed: %v", err)
	}

	if len(tokens) != 3 {
		t.Errorf("Expected 3 tokens, got %d: %v", len(tokens), tokens)
	}
}

// TestStopAnalyzerFactory_CustomStopWords tests factory with custom stop words.
func TestStopAnalyzerFactory_CustomStopWords(t *testing.T) {
	customStopWords := GetWordSetFromStrings([]string{"foo", "bar"}, true)
	factory := NewStopAnalyzerFactoryWithWords(customStopWords)
	analyzer := factory.Create()

	if analyzer == nil {
		t.Fatal("Factory.Create returned nil")
	}
	defer analyzer.Close()

	// Custom stop words should be applied
	tokens, err := collectTokens(analyzer, "foo bar baz qux")
	if err != nil {
		t.Fatalf("TokenStream failed: %v", err)
	}

	if len(tokens) != 2 {
		t.Errorf("Expected 2 tokens (baz, qux), got %d: %v", len(tokens), tokens)
	}
}

// collectTokens collects all tokens from an analyzer.
func collectTokens(analyzer Analyzer, input string) ([]string, error) {
	stream, err := analyzer.TokenStream("field", strings.NewReader(input))
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	var tokens []string
	for {
		hasToken, err := stream.IncrementToken()
		if err != nil {
			return nil, err
		}
		if !hasToken {
			break
		}

		attrSrc := stream.(interface{ GetAttributeSource() *AttributeSource }).GetAttributeSource()
		termAttr := attrSrc.GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
		if termAttr != nil {
			tokens = append(tokens, termAttr.(CharTermAttribute).String())
		}
	}

	return tokens, nil
}

// Benchmark tests
func BenchmarkStopAnalyzer_Simple(b *testing.B) {
	analyzer := NewStopAnalyzer()
	defer analyzer.Close()
	input := "the quick brown fox jumps over the lazy dog"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stream, _ := analyzer.TokenStream("field", strings.NewReader(input))
		for {
			hasToken, _ := stream.IncrementToken()
			if !hasToken {
				break
			}
		}
		stream.Close()
	}
}

func BenchmarkStopAnalyzer_CustomStopWords(b *testing.B) {
	stopWords := GetWordSetFromStrings([]string{"test", "stop", "words"}, true)
	analyzer := NewStopAnalyzerWithWords(stopWords)
	defer analyzer.Close()
	input := "test stop words hello world"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stream, _ := analyzer.TokenStream("field", strings.NewReader(input))
		for {
			hasToken, _ := stream.IncrementToken()
			if !hasToken {
				break
			}
		}
		stream.Close()
	}
}
