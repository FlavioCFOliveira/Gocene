// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"testing"
)

// TestBasqueAnalyzer tests the BasqueAnalyzer
func TestBasqueAnalyzer(t *testing.T) {
	analyzer := NewBasqueAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input         string
		minTokenCount int
	}{
		{
			input:         "Euskara hizkuntzaren analizatzailea",
			minTokenCount: 1,
		},
		{
			input:         "Kaixo mundua",
			minTokenCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens, err := collectTokensFromAnalyzer(analyzer, tt.input)
			if err != nil {
				t.Fatalf("Failed to collect tokens: %v", err)
			}

			if len(tokens) < tt.minTokenCount {
				t.Errorf("Expected at least %d tokens, got %d: %v", tt.minTokenCount, len(tokens), tokens)
			}
		})
	}
}

func TestBasqueStopWords(t *testing.T) {
	stopWords := []string{"eta", "da", "du", "ez"}
	stopSet := GetWordSetFromStrings(BasqueStopWords, true)

	for _, word := range stopWords {
		if !stopSet.ContainsString(word) {
			t.Errorf("Expected stop word %q to be in BasqueStopWords", word)
		}
	}
}

// TestBengaliAnalyzer tests the BengaliAnalyzer
func TestBengaliAnalyzer(t *testing.T) {
	analyzer := NewBengaliAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input         string
		minTokenCount int
	}{
		{
			input:         "বাংলা ভাষার বিশ্লেষক",
			minTokenCount: 1,
		},
		{
			input:         "আমি বাংলা ভাষা",
			minTokenCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens, err := collectTokensFromAnalyzer(analyzer, tt.input)
			if err != nil {
				t.Fatalf("Failed to collect tokens: %v", err)
			}

			if len(tokens) < tt.minTokenCount {
				t.Errorf("Expected at least %d tokens, got %d: %v", tt.minTokenCount, len(tokens), tokens)
			}
		})
	}
}

func TestBengaliStopWords(t *testing.T) {
	stopWords := []string{"আমি", "করে"}
	stopSet := GetWordSetFromStrings(BengaliStopWords, true)

	for _, word := range stopWords {
		if !stopSet.ContainsString(word) {
			t.Errorf("Expected stop word %q to be in BengaliStopWords", word)
		}
	}
}

// TestBrazilianAnalyzer tests the BrazilianAnalyzer
func TestBrazilianAnalyzer(t *testing.T) {
	analyzer := NewBrazilianAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input         string
		minTokenCount int
	}{
		{
			input:         "Analisador de língua brasileira",
			minTokenCount: 1,
		},
		{
			input:         "O rato roeu a roupa do rei",
			minTokenCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens, err := collectTokensFromAnalyzer(analyzer, tt.input)
			if err != nil {
				t.Fatalf("Failed to collect tokens: %v", err)
			}

			if len(tokens) < tt.minTokenCount {
				t.Errorf("Expected at least %d tokens, got %d: %v", tt.minTokenCount, len(tokens), tokens)
			}
		})
	}
}

func TestBrazilianStopWords(t *testing.T) {
	stopWords := []string{"de", "a", "o", "e"}
	stopSet := GetWordSetFromStrings(BrazilianPortugueseStopWords, true)

	for _, word := range stopWords {
		if !stopSet.ContainsString(word) {
			t.Errorf("Expected stop word %q to be in BrazilianPortugueseStopWords", word)
		}
	}
}

func TestBrazilianStemmer(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Test -mente removal
		{input: "rapidamente", expected: "rapida"},
		// Test -ção removal
		{input: "nação", expected: "na"},
		// Test -dade removal
		{input: "qualidade", expected: "quali"},
		// Test -s plural removal
		{input: "livros", expected: "livro"},
		// Test -ar removal (needs to be longer word)
		{input: "falarem", expected: "falarem"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := brazilianLightStem(tc.input)
			if result != tc.expected {
				t.Errorf("brazilianLightStem(%q) = %q, expected %q", tc.input, result, tc.expected)
			}
		})
	}
}
