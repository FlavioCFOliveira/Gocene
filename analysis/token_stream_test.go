// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"strings"
	"testing"
)

// TestTokenStream_BasicIncrement tests basic token incrementation.
// Source: TestTokenStream.testIncrementToken()
// Purpose: Tests basic token stream iteration.
func TestTokenStream_BasicIncrement(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("one two three"))

	var tokens []string
	for {
		hasToken, err := tokenizer.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
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
	tokenizer.Close()

	expected := []string{"one", "two", "three"}
	if !reflect.DeepEqual(tokens, expected) {
		t.Errorf("Expected %v, got %v", expected, tokens)
	}
}

// TestTokenStream_EndMethod tests the End() method.
// Source: TestTokenStream.testEnd()
// Purpose: Tests end-of-stream operations.
func TestTokenStream_EndMethod(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("test"))

	for {
		hasToken, err := tokenizer.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
	}

	err := tokenizer.End()
	if err != nil {
		t.Errorf("End() returned error: %v", err)
	}

	tokenizer.Close()
}

// TestTokenStream_CloseMethod tests the Close() method.
// Source: TestTokenStream.testClose()
// Purpose: Tests resource cleanup.
func TestTokenStream_CloseMethod(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("test"))

	err := tokenizer.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}

// TestTokenStream_ResetMethod tests the Reset() method.
// Source: TestTokenStream.testReset()
// Purpose: Tests token stream reset functionality.
func TestTokenStream_ResetMethod(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("first pass"))

	var tokens1 []string
	for {
		hasToken, err := tokenizer.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := tokenizer.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				tokens1 = append(tokens1, termAttr.String())
			}
		}
	}

	tokenizer.Reset()
	tokenizer.SetReader(strings.NewReader("second run"))

	var tokens2 []string
	for {
		hasToken, err := tokenizer.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := tokenizer.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				tokens2 = append(tokens2, termAttr.String())
			}
		}
	}

	tokenizer.Close()

	if !reflect.DeepEqual(tokens1, []string{"first", "pass"}) {
		t.Errorf("First pass: expected [first pass], got %v", tokens1)
	}
	if !reflect.DeepEqual(tokens2, []string{"second", "run"}) {
		t.Errorf("Second pass: expected [second run], got %v", tokens2)
	}
}

// TestTokenStream_EmptyInput tests empty input handling.
// Source: TestTokenStream.testEmptyInput()
// Purpose: Tests that empty input returns no tokens.
func TestTokenStream_EmptyInput(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader(""))

	tokenCount := 0
	for {
		hasToken, err := tokenizer.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		tokenCount++
	}
	tokenizer.Close()

	if tokenCount != 0 {
		t.Errorf("Expected 0 tokens for empty input, got %d", tokenCount)
	}
}

// TestTokenStream_ClearAttributes tests attribute clearing.
// Source: TestTokenStream.testClearAttributes()
// Purpose: Tests that attributes are properly cleared between tokens.
func TestTokenStream_ClearAttributes(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("a"))

	customAttr := NewCharTermAttribute()
	customAttr.SetValue("initial")
	tokenizer.GetAttributeSource().AddAttribute(customAttr)

	tokenizer.ClearAttributes()

	if customAttr.String() != "" {
		t.Errorf("Expected cleared attribute, got %s", customAttr.String())
	}

	tokenizer.Close()
}

// TestTokenStream_AddAttribute tests adding attributes.
// Source: TestTokenStream.testAddAttribute()
// Purpose: Tests attribute addition to token stream.
func TestTokenStream_AddAttribute(t *testing.T) {
	ts := NewBaseTokenStream()

	termAttr := NewCharTermAttribute()
	ts.AddAttribute(termAttr)

	retrieved := ts.GetAttributeSource().GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
	if retrieved == nil {
		t.Error("Expected to retrieve added attribute")
	}
}

// TestTokenStream_GetAttribute tests getting attributes.
// Source: TestTokenStream.testGetAttribute()
// Purpose: Tests attribute retrieval.
func TestTokenStream_GetAttribute(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("test"))

	attr := tokenizer.GetAttributeSource().GetAttribute("CharTermAttribute")
	if attr == nil {
		t.Error("Expected to retrieve CharTermAttribute")
	}

	nonExistent := tokenizer.GetAttributeSource().GetAttribute("NonExistent")
	if nonExistent != nil {
		t.Error("Expected nil for non-existent attribute")
	}

	tokenizer.Close()
}

// TestTokenStream_CaptureAndRestoreState tests state capture/restore.
// Source: TestTokenStream.testCaptureRestoreState()
// Purpose: Tests attribute state persistence.
func TestTokenStream_CaptureAndRestoreState(t *testing.T) {
	ts := NewBaseTokenStream()

	termAttr := NewCharTermAttribute()
	termAttr.SetValue("test")
	ts.AddAttribute(termAttr)

	state := ts.GetAttributeSource().CaptureState()

	termAttr.SetValue("modified")

	ts.GetAttributeSource().RestoreState(state)

	if termAttr.String() != "test" {
		t.Errorf("Expected 'test' after restore, got '%s'", termAttr.String())
	}
}

// TestTokenStream_Chaining tests token stream chaining.
// Source: TestTokenStream.testChaining()
// Purpose: Tests filter chain behavior.
func TestTokenStream_Chaining(t *testing.T) {
	input := "HELLO WORLD"

	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader(input))

	lowerFilter := NewLowerCaseFilter(tokenizer)
	defer lowerFilter.Close()

	var tokens []string
	for {
		hasToken, err := lowerFilter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := lowerFilter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				tokens = append(tokens, termAttr.String())
			}
		}
	}

	expected := []string{"hello", "world"}
	if !reflect.DeepEqual(tokens, expected) {
		t.Errorf("Expected %v, got %v", expected, tokens)
	}
}

// TestTokenStream_MultipleIncrementCalls tests multiple increment calls.
// Source: TestTokenStream.testMultipleIncrements()
// Purpose: Tests consistent behavior across multiple calls.
func TestTokenStream_MultipleIncrementCalls(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("one"))

	hasToken1, err := tokenizer.IncrementToken()
	if err != nil {
		t.Fatalf("Error on first increment: %v", err)
	}
	if !hasToken1 {
		t.Error("Expected true on first increment")
	}

	hasToken2, err := tokenizer.IncrementToken()
	if err != nil {
		t.Fatalf("Error on second increment: %v", err)
	}
	if hasToken2 {
		t.Error("Expected false on second increment (end of stream)")
	}

	hasToken3, err := tokenizer.IncrementToken()
	if err != nil {
		t.Fatalf("Error on third increment: %v", err)
	}
	if hasToken3 {
		t.Error("Expected false on third increment")
	}

	tokenizer.Close()
}

// TestTokenStream_Unicode tests Unicode token handling.
// Source: TestTokenStream.testUnicode()
// Purpose: Tests Unicode text processing.
func TestTokenStream_Unicode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "CJK characters",
			input:    "日本語 テスト",
			expected: []string{"日本語", "テスト"},
		},
		{
			name:     "Accented characters",
			input:    "café résumé",
			expected: []string{"café", "résumé"},
		},
		{
			name:     "Mixed scripts",
			input:    "Hello 世界 World",
			expected: []string{"Hello", "世界", "World"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			var tokens []string
			for {
				hasToken, err := tokenizer.IncrementToken()
				if err != nil {
					t.Fatalf("Error incrementing token: %v", err)
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
			tokenizer.Close()

			if !reflect.DeepEqual(tokens, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, tokens)
			}
		})
	}
}

// TestTokenStream_AttributeSource tests attribute source access.
// Source: TestTokenStream.testAttributeSource()
// Purpose: Tests attribute source functionality.
func TestTokenStream_AttributeSource(t *testing.T) {
	ts := NewBaseTokenStream()

	attrSource := ts.GetAttributeSource()
	if attrSource == nil {
		t.Fatal("Expected non-nil AttributeSource")
	}

	termAttr := NewCharTermAttribute()
	attrSource.AddAttribute(termAttr)

	if !attrSource.HasAttribute(reflect.TypeOf(&charTermAttribute{})) {
		t.Error("Expected attribute to exist in source")
	}
}
