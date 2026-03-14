// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"strings"
	"testing"
)

// TestAnalyzerUtils_Tokenize tests the Tokenize function.
// Source: TestAnalyzerUtil.java
// Purpose: Tests that text can be tokenized.
func TestAnalyzerUtils_Tokenize(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokens, err := Tokenize(tokenizer, "hello world test")
	if err != nil {
		t.Fatalf("Tokenize error: %v", err)
	}

	expected := []string{"hello", "world", "test"}
	if len(tokens) != len(expected) {
		t.Errorf("Expected %v, got %v", expected, tokens)
	}

	for i, token := range tokens {
		if token != expected[i] {
			t.Errorf("Token %d: expected %q, got %q", i, expected[i], token)
		}
	}
}

// TestAnalyzerUtils_TokenizeWithAnalyzer tests tokenizing with an Analyzer.
// Source: TestAnalyzerUtil.java
// Purpose: Tests that text can be tokenized using an Analyzer.
func TestAnalyzerUtils_TokenizeWithAnalyzer(t *testing.T) {
	analyzer := NewWhitespaceAnalyzer()
	tokens, err := TokenizeWithAnalyzer(analyzer, "field", "one two three")
	if err != nil {
		t.Fatalf("TokenizeWithAnalyzer error: %v", err)
	}

	expected := []string{"one", "two", "three"}
	if len(tokens) != len(expected) {
		t.Errorf("Expected %v, got %v", expected, tokens)
	}
}

// TestAnalyzerUtils_GetTokenPositions tests getting token positions.
// Source: TestAnalyzerUtil.java
// Purpose: Tests that token positions can be retrieved.
func TestAnalyzerUtils_GetTokenPositions(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("a b c"))

	positions, err := GetTokenPositions(tokenizer)
	if err != nil {
		t.Fatalf("GetTokenPositions error: %v", err)
	}

	// Default position increment is 1
	expected := []int{1, 2, 3}
	if len(positions) != len(expected) {
		t.Errorf("Expected positions %v, got %v", expected, positions)
	}
}

// TestAnalyzerUtils_GetTokenOffsets tests getting token offsets.
// Source: TestAnalyzerUtil.java
// Purpose: Tests that token offsets can be retrieved.
func TestAnalyzerUtils_GetTokenOffsets(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("hi there"))

	offsets, err := GetTokenOffsets(tokenizer)
	if err != nil {
		t.Fatalf("GetTokenOffsets error: %v", err)
	}

	if len(offsets) != 2 {
		t.Errorf("Expected 2 offset pairs, got %d", len(offsets))
	}

	// "hi" should be at positions 0-2
	if offsets[0][0] != 0 || offsets[0][1] != 2 {
		t.Errorf("Expected offsets [0, 2] for 'hi', got [%d, %d]", offsets[0][0], offsets[0][1])
	}
}

// TestAnalyzerUtils_CountTokens tests counting tokens.
// Source: TestAnalyzerUtil.java
// Purpose: Tests that tokens can be counted.
func TestAnalyzerUtils_CountTokens(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("one two three four"))

	count, err := CountTokens(tokenizer)
	if err != nil {
		t.Fatalf("CountTokens error: %v", err)
	}

	if count != 4 {
		t.Errorf("Expected 4 tokens, got %d", count)
	}
}

// TestAnalyzerUtils_IsEmpty tests checking if a stream is empty.
// Source: TestAnalyzerUtil.java
// Purpose: Tests that empty streams are detected.
func TestAnalyzerUtils_IsEmpty(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader(""))

	empty, err := IsEmpty(tokenizer)
	if err != nil {
		t.Fatalf("IsEmpty error: %v", err)
	}

	if !empty {
		t.Error("Expected empty stream for empty input")
	}
}

// TestAnalyzerUtils_IsEmpty_NotEmpty tests checking a non-empty stream.
func TestAnalyzerUtils_IsEmpty_NotEmpty(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("not empty"))

	empty, err := IsEmpty(tokenizer)
	if err != nil {
		t.Fatalf("IsEmpty error: %v", err)
	}

	// Note: IsEmpty consumes the first token, so it returns true if there's at least one token
	// This is actually checking if the stream has no tokens after trying to read one
	if empty {
		t.Error("Expected non-empty stream")
	}
}

// TestAnalyzerUtils_ClearAttributes tests clearing attributes.
// Source: TestAnalyzerUtil.java
// Purpose: Tests that attributes can be cleared.
func TestAnalyzerUtils_ClearAttributes(t *testing.T) {
	source := NewAttributeSource()
	source.AddAttribute(NewCharTermAttribute())

	// Get the attribute and set a value
	if attr := source.GetAttribute("CharTermAttribute"); attr != nil {
		if termAttr, ok := attr.(CharTermAttribute); ok {
			termAttr.AppendString("test")
		}
	}

	// Clear attributes
	ClearAttributes(source)

	// Verify cleared
	if attr := source.GetAttribute("CharTermAttribute"); attr != nil {
		if termAttr, ok := attr.(CharTermAttribute); ok {
			if termAttr.String() != "" {
				t.Error("Attributes should be cleared")
			}
		}
	}
}

// TestAnalyzerUtils_HasAttribute tests checking for attribute existence.
// Source: TestAnalyzerUtil.java
// Purpose: Tests that attribute existence can be checked.
func TestAnalyzerUtils_HasAttribute(t *testing.T) {
	source := NewAttributeSource()
	source.AddAttribute(NewCharTermAttribute())

	if !HasAttribute(source, "CharTermAttribute") {
		t.Error("Should have CharTermAttribute")
	}

	if HasAttribute(source, "NonExistentAttribute") {
		t.Error("Should not have NonExistentAttribute")
	}
}
