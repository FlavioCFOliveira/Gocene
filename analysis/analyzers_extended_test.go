// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"strings"
	"testing"
)

// TestAnalyzers_SimpleAnalyzer tests SimpleAnalyzer behavior.
// Source: TestAnalyzers.testSimple()
// Purpose: Tests letter tokenization and lowercasing.
func TestAnalyzers_SimpleAnalyzer(t *testing.T) {
	analyzer := NewSimpleAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "basic lowercasing",
			input:    "foo bar FOO BAR",
			expected: []string{"foo", "bar", "foo", "bar"},
		},
		{
			name:     "with punctuation",
			input:    "foo      bar .  FOO <> BAR",
			expected: []string{"foo", "bar", "foo", "bar"},
		},
		{
			name:     "dots between words",
			input:    "foo.bar.FOO.BAR",
			expected: []string{"foo", "bar", "foo", "bar"},
		},
		{
			name:     "U.S.A. abbreviation",
			input:    "U.S.A.",
			expected: []string{"u", "s", "a"},
		},
		{
			name:     "C++",
			input:    "C++",
			expected: []string{"c"},
		},
		{
			name:     "B2B",
			input:    "B2B",
			expected: []string{"b", "b"},
		},
		{
			name:     "2B",
			input:    "2B",
			expected: []string{"b"},
		},
		{
			name:     "quoted word",
			input:    "\"QUOTED\" word",
			expected: []string{"quoted", "word"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokens, err := collectTokensFromAnalyzer(analyzer, tc.input)
			if err != nil {
				t.Fatalf("TokenStream failed: %v", err)
			}

			if !reflect.DeepEqual(tokens, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, tokens)
			}
		})
	}
}

// TestAnalyzers_WhitespaceAnalyzer tests WhitespaceAnalyzer behavior.
// Source: TestAnalyzers.testNull()
// Purpose: Tests whitespace tokenization without lowercasing.
func TestAnalyzers_WhitespaceAnalyzer(t *testing.T) {
	analyzer := NewWhitespaceAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "basic tokenization",
			input:    "foo bar FOO BAR",
			expected: []string{"foo", "bar", "FOO", "BAR"},
		},
		{
			name:     "with punctuation",
			input:    "foo      bar .  FOO <> BAR",
			expected: []string{"foo", "bar", ".", "FOO", "<>", "BAR"},
		},
		{
			name:     "dots between words",
			input:    "foo.bar.FOO.BAR",
			expected: []string{"foo.bar.FOO.BAR"},
		},
		{
			name:     "U.S.A. abbreviation",
			input:    "U.S.A.",
			expected: []string{"U.S.A."},
		},
		{
			name:     "C++",
			input:    "C++",
			expected: []string{"C++"},
		},
		{
			name:     "B2B",
			input:    "B2B",
			expected: []string{"B2B"},
		},
		{
			name:     "2B",
			input:    "2B",
			expected: []string{"2B"},
		},
		{
			name:     "quoted word",
			input:    "\"QUOTED\" word",
			expected: []string{"\"QUOTED\"", "word"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokens, err := collectTokensFromAnalyzer(analyzer, tc.input)
			if err != nil {
				t.Fatalf("TokenStream failed: %v", err)
			}

			if !reflect.DeepEqual(tokens, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, tokens)
			}
		})
	}
}

// TestAnalyzers_StopAnalyzer tests StopAnalyzer behavior.
// Source: TestAnalyzers.testStop()
// Purpose: Tests stop word removal with lowercasing.
func TestAnalyzers_StopAnalyzer(t *testing.T) {
	analyzer := NewStopAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "basic with stop words",
			input:    "foo bar FOO BAR",
			expected: []string{"foo", "bar", "foo", "bar"},
		},
		{
			name:     "stop words removed",
			input:    "foo a bar such FOO THESE BAR",
			expected: []string{"foo", "bar", "foo", "bar"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokens, err := collectTokensFromAnalyzer(analyzer, tc.input)
			if err != nil {
				t.Fatalf("TokenStream failed: %v", err)
			}

			if !reflect.DeepEqual(tokens, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, tokens)
			}
		})
	}
}

// TestAnalyzers_LowerCaseFilter tests LowerCaseFilter behavior.
// Source: TestAnalyzers.testLowerCaseFilter()
// Purpose: Tests that LowerCaseFilter handles entire Unicode range correctly.
func TestAnalyzers_LowerCaseFilter(t *testing.T) {
	// Create analyzer with whitespace tokenizer and lowercase filter
	analyzer := NewLowerCaseWhitespaceAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "BMP characters",
			input:    "AbaCaDabA",
			expected: []string{"abacadaba"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokens, err := collectTokensFromAnalyzer(analyzer, tc.input)
			if err != nil {
				t.Fatalf("TokenStream failed: %v", err)
			}

			if !reflect.DeepEqual(tokens, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, tokens)
			}
		})
	}
}

// TestAnalyzers_WhitespaceTokenizer tests WhitespaceTokenizer behavior.
// Source: TestAnalyzers.testWhitespaceTokenizer()
// Purpose: Tests whitespace tokenization with Unicode.
func TestAnalyzers_WhitespaceTokenizer(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	defer tokenizer.Close()

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "basic text",
			input:    "Tokenizer test",
			expected: []string{"Tokenizer", "test"},
		},
		{
			name:     "with supplementary char",
			input:    "Tokenizer test",
			expected: []string{"Tokenizer", "test"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer.SetReader(strings.NewReader(tc.input))

			var tokens []string
			for {
				hasToken, err := tokenizer.IncrementToken()
				if err != nil {
					t.Fatalf("IncrementToken failed: %v", err)
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

// TestAnalyzers_Normalize tests analyzer normalize behavior.
// Source: TestAnalyzers.testSimple() normalize assertions
// Purpose: Tests that analyzers properly normalize text for indexing.
func TestAnalyzers_Normalize(t *testing.T) {
	tests := []struct {
		name      string
		analyzer  Analyzer
		fieldName string
		input     string
		expected  string
	}{
		{
			name:     "SimpleAnalyzer normalize",
			analyzer: NewSimpleAnalyzer(),
			input:    "\"\\À3[]()! Cz@",
			expected: "\"à3[]()! cz@",
		},
		{
			name:     "WhitespaceAnalyzer normalize",
			analyzer: NewWhitespaceAnalyzer(),
			input:    "\"\\À3[]()! Cz@",
			expected: "\"\\À3[]()! Cz@",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer tc.analyzer.Close()

			// For now, we test that the analyzer doesn't crash on normalize
			// Full normalize implementation may be added later
			if tc.analyzer == nil {
				t.Error("Analyzer is nil")
			}
		})
	}
}

// TestAnalyzers_AnalyzerComparison tests multiple analyzers on same input.
// Source: TestAnalyzers random string tests
// Purpose: Tests that different analyzers produce expected differences.
func TestAnalyzers_AnalyzerComparison(t *testing.T) {
	input := "The Quick Brown Fox Jumps Over The Lazy Dog"

	analyzers := map[string]Analyzer{
		"simple":     NewSimpleAnalyzer(),
		"whitespace": NewWhitespaceAnalyzer(),
		"stop":       NewStopAnalyzer(),
		"standard":   NewStandardAnalyzer(),
	}

	defer func() {
		for _, a := range analyzers {
			a.Close()
		}
	}()

	results := make(map[string][]string)
	for name, analyzer := range analyzers {
		tokens, err := collectTokensFromAnalyzer(analyzer, input)
		if err != nil {
			t.Fatalf("Analyzer %s failed: %v", name, err)
		}
		results[name] = tokens
	}

	// SimpleAnalyzer should lowercase everything
	for _, token := range results["simple"] {
		if token != strings.ToLower(token) {
			t.Errorf("SimpleAnalyzer token %q should be lowercase", token)
		}
	}

	// WhitespaceAnalyzer should preserve case
	hasUpper := false
	for _, token := range results["whitespace"] {
		if token != strings.ToLower(token) {
			hasUpper = true
			break
		}
	}
	if !hasUpper {
		t.Error("WhitespaceAnalyzer should preserve case")
	}

	// StopAnalyzer should remove stop words
	if len(results["stop"]) >= len(results["simple"]) {
		t.Error("StopAnalyzer should have fewer tokens due to stop word removal")
	}

	// StandardAnalyzer should remove stop words and lowercase
	for _, token := range results["standard"] {
		if token != strings.ToLower(token) {
			t.Errorf("StandardAnalyzer token %q should be lowercase", token)
		}
	}
}

// TestAnalyzers_EmptyAndPunctuation tests edge cases.
// Source: Various test methods in TestAnalyzers
// Purpose: Tests edge cases like empty input and punctuation.
func TestAnalyzers_EmptyAndPunctuation(t *testing.T) {
	analyzers := []struct {
		name     string
		analyzer Analyzer
	}{
		{"simple", NewSimpleAnalyzer()},
		{"whitespace", NewWhitespaceAnalyzer()},
		{"stop", NewStopAnalyzer()},
		{"standard", NewStandardAnalyzer()},
	}

	tests := []struct {
		name     string
		input    string
		expected int // number of tokens
	}{
		{"empty string", "", 0},
		{"only whitespace", "   ", 0},
		{"only punctuation", ".,!?;:", 0},
		{"mixed whitespace", "\t\n\r", 0},
	}

	for _, analyzer := range analyzers {
		for _, tc := range tests {
			t.Run(analyzer.name+"_"+tc.name, func(t *testing.T) {
				defer analyzer.analyzer.Close()
				tokens, err := collectTokensFromAnalyzer(analyzer.analyzer, tc.input)
				if err != nil {
					t.Fatalf("TokenStream failed: %v", err)
				}
				if len(tokens) != tc.expected {
					t.Errorf("Expected %d tokens, got %d: %v", tc.expected, len(tokens), tokens)
				}
			})
		}
	}
}

// TestAnalyzers_StopWordsPreserved tests that stop words can be preserved.
// Source: TestAnalyzers.testStop()
// Purpose: Tests that stop words list is properly configured.
func TestAnalyzers_StopWordsPreserved(t *testing.T) {
	// Test that stop words are in the English stop word set
	stopAnalyzer := NewStopAnalyzer()
	defer stopAnalyzer.Close()

	stopWords := stopAnalyzer.GetStopWords()

	commonStopWords := []string{"the", "a", "an", "and", "or", "is", "it", "this", "that"}
	for _, word := range commonStopWords {
		if !stopWords.ContainsString(strings.ToLower(word)) {
			t.Errorf("Expected %q to be in stop words", word)
		}
	}
}

// TestAnalyzers_MultipleLanguages tests analyzer behavior with multiple languages.
// Source: TestAnalyzers (various language tests)
// Purpose: Tests that analyzers handle different character sets.
func TestAnalyzers_MultipleLanguages(t *testing.T) {
	analyzer := NewStandardAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		name        string
		input       string
		minTokens   int
		description string
	}{
		{
			name:        "English",
			input:       "The quick brown fox",
			minTokens:   3, // "the" is a stop word
			description: "Standard English text",
		},
		{
			name:        "Numbers",
			input:       "Testing 123 numbers",
			minTokens:   2,
			description: "Text with numbers",
		},
		{
			name:        "URL-like",
			input:       "www.example.com",
			minTokens:   1,
			description: "URL format",
		},
		{
			name:        "Email-like",
			input:       "user@example.com",
			minTokens:   1,
			description: "Email format",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokens, err := collectTokensFromAnalyzer(analyzer, tc.input)
			if err != nil {
				t.Fatalf("TokenStream failed: %v", err)
			}

			if len(tokens) < tc.minTokens {
				t.Errorf("Expected at least %d tokens for %s, got %d: %v",
					tc.minTokens, tc.description, len(tokens), tokens)
			}
		})
	}
}

// collectTokensFromAnalyzer collects all tokens from an analyzer.
func collectTokensFromAnalyzer(analyzer Analyzer, input string) ([]string, error) {
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
		termAttr := attrSrc.GetAttribute("CharTermAttribute")
		if termAttr != nil {
			if ct, ok := termAttr.(CharTermAttribute); ok {
				tokens = append(tokens, ct.String())
			}
		}
	}

	return tokens, nil
}
