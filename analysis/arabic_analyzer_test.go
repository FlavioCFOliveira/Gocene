// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"testing"
)

// TestArabicAnalyzer tests the ArabicAnalyzer
func TestArabicAnalyzer(t *testing.T) {
	analyzer := NewArabicAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input         string
		minTokenCount int
	}{
		{
			input:         "الكتاب الجميل",
			minTokenCount: 1,
		},
		{
			input:         "السلام عليكم ورحمة الله",
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

func TestArabicNormalizer(t *testing.T) {
	normalizer := NewArabicNormalizer()

	tests := []struct {
		input    string
		expected string
	}{
		// Test Kashida removal
		{input: "كـتـاب", expected: "كتاب"},
		// Test Alef normalization
		{input: "آدم", expected: "ادم"},
		{input: "أحمد", expected: "احمد"},
		{input: "إبراهيم", expected: "ابراهيم"},
		// Test Alef Maksura normalization (على -> علي)
		{input: "على", expected: "علي"},
		// Test no change
		{input: "كتاب", expected: "كتاب"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := normalizer.Normalize(tc.input)
			if result != tc.expected {
				t.Errorf("Normalize(%q) = %q, expected %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestArabicStemmer(t *testing.T) {
	stemmer := NewArabicStemmer()

	tests := []struct {
		input    string
		expected string
	}{
		// Test definite article removal
		{input: "الكتاب", expected: "كتاب"},
		// Test prefix removal with definite article
		{input: "بالمدرسة", expected: "مدرس"}, // Light stemming removes ta marbuta
		// Test possessive suffix removal
		{input: "كتابه", expected: "تاب"}, // Prefix removal happens after suffix
		// Test possessive suffix removal
		{input: "كتابها", expected: "تاب"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := stemmer.Stem(tc.input)
			if result != tc.expected {
				t.Errorf("Stem(%q) = %q, expected %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestArabicStopWords(t *testing.T) {
	// Test that common stop words are in the list
	stopWords := []string{"ال", "و", "في", "من", "هو", "هي"}
	stopSet := GetWordSetFromStrings(ArabicStopWords, true)

	for _, word := range stopWords {
		if !stopSet.ContainsString(word) {
			t.Errorf("Expected stop word %q to be in ArabicStopWords", word)
		}
	}
}

func TestIsArabicLetter(t *testing.T) {
	tests := []struct {
		r        rune
		expected bool
	}{
		{r: 'ك', expected: true},    // Arabic letter
		{r: 'a', expected: false},   // Latin letter
		{r: '1', expected: false},   // Digit
		{r: 'é', expected: false},   // Accented Latin
		{r: 0x060C, expected: true}, // Arabic comma
		{r: 0x061F, expected: true}, // Arabic question mark
	}

	for _, tc := range tests {
		t.Run(string(tc.r), func(t *testing.T) {
			result := IsArabicLetter(tc.r)
			if result != tc.expected {
				t.Errorf("IsArabicLetter(%q) = %v, expected %v", tc.r, result, tc.expected)
			}
		})
	}
}

func TestHasArabicText(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{input: "Hello كتاب", expected: true},
		{input: "Hello World", expected: false},
		{input: "السلام", expected: true},
		{input: "", expected: false},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := HasArabicText(tc.input)
			if result != tc.expected {
				t.Errorf("HasArabicText(%q) = %v, expected %v", tc.input, result, tc.expected)
			}
		})
	}
}
