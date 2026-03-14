// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"strings"
	"testing"
)

// TestASCIIFoldingFilter_Basic tests basic ASCII folding.
// Source: TestASCIIFoldingFilter.java
// Purpose: Tests that non-ASCII characters are folded to ASCII.
func TestASCIIFoldingFilter_Basic(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"café", "cafe"},
		{"naïve", "naive"},
		{"résumé", "resume"},
		{"über", "uber"},
		{"Æther", "Aether"},
		{"naïveté", "naivete"},
		{"déjà vu", "deja vu"},
		{"Français", "Francais"},
		{"Größe", "Grosse"},
		{"Straße", "Strasse"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			foldFilter := NewASCIIFoldingFilter(tokenizer)
			defer foldFilter.Close()

			var tokens []string
			for {
				hasToken, err := foldFilter.IncrementToken()
				if err != nil {
					t.Fatalf("Error: %v", err)
				}
				if !hasToken {
					break
				}
				if attr := foldFilter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
					if termAttr, ok := attr.(CharTermAttribute); ok {
						tokens = append(tokens, termAttr.String())
					}
				}
			}

			result := strings.Join(tokens, " ")
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}

// TestASCIIFoldingFilter_ASCIIOnly tests that ASCII characters are unchanged.
// Source: TestASCIIFoldingFilter.java
// Purpose: Tests that ASCII text is not modified.
func TestASCIIFoldingFilter_ASCIIOnly(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("hello world"))

	foldFilter := NewASCIIFoldingFilter(tokenizer)
	defer foldFilter.Close()

	var tokens []string
	for {
		hasToken, err := foldFilter.IncrementToken()
		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := foldFilter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				tokens = append(tokens, termAttr.String())
			}
		}
	}

	expected := []string{"hello", "world"}
	if len(tokens) != len(expected) {
		t.Errorf("Expected %v, got %v", expected, tokens)
	}

	for i, token := range tokens {
		if token != expected[i] {
			t.Errorf("Token %d: expected %q, got %q", i, expected[i], token)
		}
	}
}

// TestASCIIFoldingFilter_PreserveOriginal tests preserving original tokens.
// Source: TestASCIIFoldingFilter.java
// Purpose: Tests that original tokens can be preserved.
func TestASCIIFoldingFilter_PreserveOriginal(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("café"))

	foldFilter := NewASCIIFoldingFilterWithOptions(tokenizer, true)
	defer foldFilter.Close()

	var tokens []string
	for {
		hasToken, err := foldFilter.IncrementToken()
		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := foldFilter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				tokens = append(tokens, termAttr.String())
			}
		}
	}

	// Should have both original and folded versions
	if len(tokens) != 2 {
		t.Errorf("Expected 2 tokens (original + folded), got %d: %v", len(tokens), tokens)
	}

	if len(tokens) >= 2 {
		if tokens[0] != "café" {
			t.Errorf("First token should be original 'café', got %q", tokens[0])
		}
		if tokens[1] != "cafe" {
			t.Errorf("Second token should be folded 'cafe', got %q", tokens[1])
		}
	}
}

// TestASCIIFoldingFilter_MultipleAccents tests multiple accented characters.
// Source: TestASCIIFoldingFilter.java
// Purpose: Tests handling of multiple accents in one token.
func TestASCIIFoldingFilter_MultipleAccents(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("áéíóú"))

	foldFilter := NewASCIIFoldingFilter(tokenizer)
	defer foldFilter.Close()

	var tokens []string
	for {
		hasToken, err := foldFilter.IncrementToken()
		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := foldFilter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				tokens = append(tokens, termAttr.String())
			}
		}
	}

	if len(tokens) != 1 {
		t.Fatalf("Expected 1 token, got %d", len(tokens))
	}

	if tokens[0] != "aeiou" {
		t.Errorf("Expected 'aeiou', got %q", tokens[0])
	}
}

// TestASCIIFoldingFilter_SpecialCharacters tests special character folding.
// Source: TestASCIIFoldingFilter.java
// Purpose: Tests folding of special characters like ß, æ, œ.
func TestASCIIFoldingFilter_SpecialCharacters(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"ß", "ss"},
		{"æ", "ae"},
		{"œ", "oe"},
		{"Æ", "AE"},
		{"Œ", "OE"},
		{"þ", "th"},
		{"Þ", "TH"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			foldFilter := NewASCIIFoldingFilter(tokenizer)
			defer foldFilter.Close()

			var tokens []string
			for {
				hasToken, err := foldFilter.IncrementToken()
				if err != nil {
					t.Fatalf("Error: %v", err)
				}
				if !hasToken {
					break
				}
				if attr := foldFilter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
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

// TestASCIIFoldingFilter_Direct tests the folding function directly.
func TestASCIIFoldingFilter_Direct(t *testing.T) {
	filter := NewASCIIFoldingFilter(nil)

	tests := []struct {
		input    string
		expected string
	}{
		{"àáâãäå", "aaaaaa"},
		{"èéêë", "eeee"},
		{"ìíîï", "iiii"},
		{"òóôõöø", "oooooo"},
		{"ùúûü", "uuuu"},
		{"ñ", "n"},
		{"ç", "c"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := filter.foldToASCII(tc.input)
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}
