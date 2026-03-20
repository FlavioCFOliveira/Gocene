// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"testing"
)

// TestArmenianAnalyzer tests the ArmenianAnalyzer
func TestArmenianAnalyzer(t *testing.T) {
	analyzer := NewArmenianAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input         string
		minTokenCount int
	}{
		{
			input:         "Հայերեն լեզվի անալիզատոր",
			minTokenCount: 1,
		},
		{
			input:         "Ես սիրում եմ ծրագրավորում",
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

func TestArmenianStopWords(t *testing.T) {
	// Test that common stop words are in the list
	stopWords := []string{"և", "է", "որ", "կամ", "համար"}
	stopSet := GetWordSetFromStrings(ArmenianStopWords, true)

	for _, word := range stopWords {
		if !stopSet.ContainsString(word) {
			t.Errorf("Expected stop word %q to be in ArmenianStopWords", word)
		}
	}
}
