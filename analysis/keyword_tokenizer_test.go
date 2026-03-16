// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"strings"
	"testing"
)

// TestKeywordTokenizer_Simple tests basic keyword tokenization.
func TestKeywordTokenizer_Simple(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple word", "hello", "hello"},
		{"multiple words", "hello world", "hello world"},
		{"with punctuation", "Hello, World!", "Hello, World!"},
		{"with numbers", "test123", "test123"},
		{"special chars", "a@b#c$d%e", "a@b#c$d%e"},
		{"whitespace", "  spaces  ", "  spaces  "},
		{"tabs", "\ttabs\t", "\ttabs\t"},
		{"newlines", "line1\nline2", "line1\nline2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer := NewKeywordTokenizer()
			if err := tokenizer.SetReader(strings.NewReader(tt.input)); err != nil {
				t.Fatalf("SetReader failed: %v", err)
			}

			hasToken, err := tokenizer.IncrementToken()
			if err != nil {
				t.Fatalf("IncrementToken failed: %v", err)
			}

			if !hasToken {
				t.Fatalf("IncrementToken returned false, expected true")
			}

			termAttr := tokenizer.GetAttributeSource().GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
			if termAttr == nil {
				t.Fatal("CharTermAttribute is nil")
			}

			term := termAttr.(CharTermAttribute).String()
			if term != tt.expected {
				t.Errorf("Token = %q, want %q", term, tt.expected)
			}

			// Should be only one token
			hasMore, err := tokenizer.IncrementToken()
			if err != nil {
				t.Fatalf("Second IncrementToken failed: %v", err)
			}
			if hasMore {
				t.Error("Expected only one token, got more")
			}
		})
	}
}

// TestKeywordTokenizer_Factory tests the factory creation.
func TestKeywordTokenizer_Factory(t *testing.T) {
	factory := NewKeywordTokenizerFactory()
	tokenizer := factory.Create()

	if tokenizer == nil {
		t.Fatal("Factory.Create returned nil")
	}

	// Verify it's a KeywordTokenizer
	_, ok := tokenizer.(*KeywordTokenizer)
	if !ok {
		t.Error("Factory did not create KeywordTokenizer")
	}
}

// TestKeywordTokenizer_Empty tests empty input handling.
func TestKeywordTokenizer_Empty(t *testing.T) {
	tokenizer := NewKeywordTokenizer()
	if err := tokenizer.SetReader(strings.NewReader("")); err != nil {
		t.Fatalf("SetReader failed: %v", err)
	}

	hasToken, err := tokenizer.IncrementToken()
	if err != nil {
		t.Fatalf("IncrementToken failed: %v", err)
	}

	if !hasToken {
		t.Error("Expected one empty token, got none")
	}

	termAttr := tokenizer.GetAttributeSource().GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
	if termAttr == nil {
		t.Fatal("CharTermAttribute is nil")
	}

	term := termAttr.(CharTermAttribute).String()
	if term != "" {
		t.Errorf("Empty input should produce empty token, got %q", term)
	}
}

// TestKeywordTokenizer_OffsetAttribute tests offset attribute handling.
func TestKeywordTokenizer_OffsetAttribute(t *testing.T) {
	input := "Hello World"
	tokenizer := NewKeywordTokenizer()
	if err := tokenizer.SetReader(strings.NewReader(input)); err != nil {
		t.Fatalf("SetReader failed: %v", err)
	}

	hasToken, err := tokenizer.IncrementToken()
	if err != nil {
		t.Fatalf("IncrementToken failed: %v", err)
	}
	if !hasToken {
		t.Fatal("Expected token")
	}

	offsetAttr := tokenizer.GetAttributeSource().GetAttributeByType(reflect.TypeOf(&offsetAttribute{}))
	if offsetAttr == nil {
		t.Fatal("OffsetAttribute is nil")
	}

	offset := offsetAttr.(OffsetAttribute)
	start := offset.StartOffset()
	end := offset.EndOffset()

	if start != 0 {
		t.Errorf("Start offset = %d, want 0", start)
	}
	if end != len(input) {
		t.Errorf("End offset = %d, want %d", end, len(input))
	}
}

// TestKeywordTokenizer_Reset tests reset functionality.
func TestKeywordTokenizer_Reset(t *testing.T) {
	tokenizer := NewKeywordTokenizer()

	// First input
	if err := tokenizer.SetReader(strings.NewReader("first")); err != nil {
		t.Fatalf("SetReader failed: %v", err)
	}
	hasToken, _ := tokenizer.IncrementToken()
	if !hasToken {
		t.Fatal("Expected token from first input")
	}

	// Reset and use new input
	if err := tokenizer.Reset(); err != nil {
		t.Fatalf("Reset failed: %v", err)
	}
	if err := tokenizer.SetReader(strings.NewReader("second")); err != nil {
		t.Fatalf("SetReader failed: %v", err)
	}

	hasToken, err := tokenizer.IncrementToken()
	if err != nil {
		t.Fatalf("IncrementToken failed: %v", err)
	}
	if !hasToken {
		t.Fatal("Expected token from second input")
	}

	termAttr := tokenizer.GetAttributeSource().GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
	term := termAttr.(CharTermAttribute).String()
	if term != "second" {
		t.Errorf("Token after reset = %q, want 'second'", term)
	}
}

// TestKeywordTokenizer_LargeInput tests handling large input.
func TestKeywordTokenizer_LargeInput(t *testing.T) {
	// Create a large input
	large := strings.Repeat("a", 10000)

	tokenizer := NewKeywordTokenizer()
	if err := tokenizer.SetReader(strings.NewReader(large)); err != nil {
		t.Fatalf("SetReader failed: %v", err)
	}

	hasToken, err := tokenizer.IncrementToken()
	if err != nil {
		t.Fatalf("IncrementToken failed: %v", err)
	}
	if !hasToken {
		t.Fatal("Expected token")
	}

	termAttr := tokenizer.GetAttributeSource().GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
	term := termAttr.(CharTermAttribute).String()
	if term != large {
		t.Errorf("Large token length = %d, want %d", len(term), len(large))
	}
}

// TestKeywordTokenizer_Unicode tests Unicode handling.
func TestKeywordTokenizer_Unicode(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"Chinese", "你好世界"},
		{"Japanese", "こんにちは"},
		{"Arabic", "مرحبا"},
		{"Russian", "Привет"},
		{"Emoji", "🎉🚀💻"},
		{"Mixed", "Hello 世界 🌍"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer := NewKeywordTokenizer()
			if err := tokenizer.SetReader(strings.NewReader(tt.input)); err != nil {
				t.Fatalf("SetReader failed: %v", err)
			}

			hasToken, err := tokenizer.IncrementToken()
			if err != nil {
				t.Fatalf("IncrementToken failed: %v", err)
			}
			if !hasToken {
				t.Fatal("Expected token")
			}

			termAttr := tokenizer.GetAttributeSource().GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
			term := termAttr.(CharTermAttribute).String()
			if term != tt.input {
				t.Errorf("Unicode token = %q, want %q", term, tt.input)
			}
		})
	}
}

// Benchmark tests
func BenchmarkKeywordTokenizer_Short(b *testing.B) {
	input := "hello world"
	reader := strings.NewReader(input)
	tokenizer := NewKeywordTokenizer()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader.Reset(input)
		tokenizer.SetReader(reader)
		tokenizer.IncrementToken()
	}
}

func BenchmarkKeywordTokenizer_Large(b *testing.B) {
	input := strings.Repeat("a", 10000)
	reader := strings.NewReader(input)
	tokenizer := NewKeywordTokenizer()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader.Reset(input)
		tokenizer.SetReader(reader)
		tokenizer.IncrementToken()
	}
}
