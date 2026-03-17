// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// TestSimplePatternTokenizer_Basic tests basic pattern tokenization.
func TestSimplePatternTokenizer_Basic(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		input    string
		expected []string
	}{
		{
			name:     "word pattern",
			pattern:  `\w+`,
			input:    "Hello, World! 123",
			expected: []string{"Hello", "World", "123"},
		},
		{
			name:     "letter only pattern",
			pattern:  `[a-zA-Z]+`,
			input:    "Hello123 World456",
			expected: []string{"Hello", "World"},
		},
		{
			name:     "digit pattern",
			pattern:  `\d+`,
			input:    "abc 123 def 456",
			expected: []string{"123", "456"},
		},
		{
			name:     "email-like pattern",
			pattern:  `[\w@.]+`,
			input:    "Contact: user@example.com for info",
			expected: []string{"Contact", "user@example.com", "for", "info"},
		},
		{
			name:     "single char tokens",
			pattern:  `.`,
			input:    "abc",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "empty input",
			pattern:  `\w+`,
			input:    "",
			expected: []string{},
		},
		{
			name:     "no matches",
			pattern:  `\d+`,
			input:    "abc xyz",
			expected: []string{},
		},
		{
			name:     "whitespace separated words",
			pattern:  `[^\s]+`,
			input:    "one two three",
			expected: []string{"one", "two", "three"},
		},
		{
			name:     "pattern with special chars",
			pattern:  `[#@]\w+`,
			input:    "Hello #world @user test",
			expected: []string{"#world", "@user"},
		},
		{
			name:     "consecutive matches",
			pattern:  `a+`,
			input:    "aaabaaa",
			expected: []string{"aaa", "aaa"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer, err := NewSimplePatternTokenizer(tt.pattern)
			if err != nil {
				t.Fatalf("Failed to create tokenizer: %v", err)
			}

			if err := tokenizer.SetReader(strings.NewReader(tt.input)); err != nil {
				t.Fatalf("SetReader failed: %v", err)
			}

			var tokens []string
			for {
				hasToken, err := tokenizer.IncrementToken()
				if err != nil {
					t.Fatalf("IncrementToken failed: %v", err)
				}
				if !hasToken {
					break
				}

				termAttr := tokenizer.GetAttributeSource().GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
				if termAttr == nil {
					t.Fatal("CharTermAttribute is nil")
				}

				term := termAttr.(CharTermAttribute).String()
				tokens = append(tokens, term)
			}

			if len(tokens) != len(tt.expected) {
				t.Errorf("Got %d tokens, want %d: got %v, want %v", len(tokens), len(tt.expected), tokens, tt.expected)
				return
			}

			for i, expected := range tt.expected {
				if tokens[i] != expected {
					t.Errorf("Token %d = %q, want %q", i, tokens[i], expected)
				}
			}
		})
	}
}

// TestSimplePatternTokenizer_Offsets tests offset attribute handling.
func TestSimplePatternTokenizer_Offsets(t *testing.T) {
	input := "Hello, World!"
	tokenizer, err := NewSimplePatternTokenizer(`\w+`)
	if err != nil {
		t.Fatalf("Failed to create tokenizer: %v", err)
	}

	if err := tokenizer.SetReader(strings.NewReader(input)); err != nil {
		t.Fatalf("SetReader failed: %v", err)
	}

	expectedOffsets := []struct {
		start int
		end   int
		text  string
	}{
		{0, 5, "Hello"},
		{7, 12, "World"},
	}

	for i, expected := range expectedOffsets {
		hasToken, err := tokenizer.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken failed: %v", err)
		}
		if !hasToken {
			t.Fatalf("Expected token %d", i)
		}

		termAttr := tokenizer.GetAttributeSource().GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
		offsetAttr := tokenizer.GetAttributeSource().GetAttributeByType(reflect.TypeOf(&offsetAttribute{}))

		if termAttr == nil {
			t.Fatal("CharTermAttribute is nil")
		}
		if offsetAttr == nil {
			t.Fatal("OffsetAttribute is nil")
		}

		term := termAttr.(CharTermAttribute).String()
		offset := offsetAttr.(OffsetAttribute)

		if term != expected.text {
			t.Errorf("Token %d text = %q, want %q", i, term, expected.text)
		}
		if offset.StartOffset() != expected.start {
			t.Errorf("Token %d start offset = %d, want %d", i, offset.StartOffset(), expected.start)
		}
		if offset.EndOffset() != expected.end {
			t.Errorf("Token %d end offset = %d, want %d", i, offset.EndOffset(), expected.end)
		}
	}
}

// TestSimplePatternTokenizer_PositionIncrement tests position increment attribute.
func TestSimplePatternTokenizer_PositionIncrement(t *testing.T) {
	tokenizer, err := NewSimplePatternTokenizer(`\w+`)
	if err != nil {
		t.Fatalf("Failed to create tokenizer: %v", err)
	}

	if err := tokenizer.SetReader(strings.NewReader("one two three")); err != nil {
		t.Fatalf("SetReader failed: %v", err)
	}

	for i := 0; i < 3; i++ {
		hasToken, err := tokenizer.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken failed: %v", err)
		}
		if !hasToken {
			t.Fatalf("Expected token %d", i)
		}

		posIncrAttr := tokenizer.GetAttributeSource().GetAttributeByType(reflect.TypeOf(&positionIncrementAttribute{}))
		if posIncrAttr == nil {
			t.Fatal("PositionIncrementAttribute is nil")
		}

		posIncr := posIncrAttr.(PositionIncrementAttribute).GetPositionIncrement()
		if posIncr != 1 {
			t.Errorf("Token %d position increment = %d, want 1", i, posIncr)
		}
	}
}

// TestSimplePatternTokenizer_Reset tests reset functionality.
func TestSimplePatternTokenizer_Reset(t *testing.T) {
	tokenizer, err := NewSimplePatternTokenizer(`\w+`)
	if err != nil {
		t.Fatalf("Failed to create tokenizer: %v", err)
	}

	// First input
	if err := tokenizer.SetReader(strings.NewReader("first second")); err != nil {
		t.Fatalf("SetReader failed: %v", err)
	}

	// Consume first token
	hasToken, _ := tokenizer.IncrementToken()
	if !hasToken {
		t.Fatal("Expected token from first input")
	}

	termAttr := tokenizer.GetAttributeSource().GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
	term := termAttr.(CharTermAttribute).String()
	if term != "first" {
		t.Errorf("First token = %q, want 'first'", term)
	}

	// Reset and use new input
	if err := tokenizer.Reset(); err != nil {
		t.Fatalf("Reset failed: %v", err)
	}
	if err := tokenizer.SetReader(strings.NewReader("third fourth")); err != nil {
		t.Fatalf("SetReader failed: %v", err)
	}

	// Consume first token from new input
	hasToken, err = tokenizer.IncrementToken()
	if err != nil {
		t.Fatalf("IncrementToken failed: %v", err)
	}
	if !hasToken {
		t.Fatal("Expected token from second input")
	}

	termAttr = tokenizer.GetAttributeSource().GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
	term = termAttr.(CharTermAttribute).String()
	if term != "third" {
		t.Errorf("Token after reset = %q, want 'third'", term)
	}
}

// TestSimplePatternTokenizer_End tests end-of-stream operations.
func TestSimplePatternTokenizer_End(t *testing.T) {
	input := "Hello World"
	tokenizer, err := NewSimplePatternTokenizer(`\w+`)
	if err != nil {
		t.Fatalf("Failed to create tokenizer: %v", err)
	}

	if err := tokenizer.SetReader(strings.NewReader(input)); err != nil {
		t.Fatalf("SetReader failed: %v", err)
	}

	// Consume all tokens
	for {
		hasToken, err := tokenizer.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken failed: %v", err)
		}
		if !hasToken {
			break
		}
	}

	// Call End
	if err := tokenizer.End(); err != nil {
		t.Fatalf("End failed: %v", err)
	}

	offsetAttr := tokenizer.GetAttributeSource().GetAttributeByType(reflect.TypeOf(&offsetAttribute{}))
	if offsetAttr == nil {
		t.Fatal("OffsetAttribute is nil")
	}

	offset := offsetAttr.(OffsetAttribute)
	if offset.EndOffset() != len(input) {
		t.Errorf("End offset = %d, want %d", offset.EndOffset(), len(input))
	}
}

// TestSimplePatternTokenizer_InvalidPattern tests invalid pattern handling.
func TestSimplePatternTokenizer_InvalidPattern(t *testing.T) {
	_, err := NewSimplePatternTokenizer(`[invalid`)
	if err == nil {
		t.Error("Expected error for invalid pattern, got nil")
	}
}

// TestSimplePatternTokenizer_WithPrecompiledRegexp tests creation with pre-compiled regexp.
func TestSimplePatternTokenizer_WithPrecompiledRegexp(t *testing.T) {
	re := regexp.MustCompile(`\w+`)
	tokenizer := NewSimplePatternTokenizerWithRegexp(re)

	if tokenizer == nil {
		t.Fatal("Tokenizer is nil")
	}

	if tokenizer.GetPattern() != re {
		t.Error("Pattern mismatch")
	}

	if err := tokenizer.SetReader(strings.NewReader("hello world")); err != nil {
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
	if term != "hello" {
		t.Errorf("Token = %q, want 'hello'", term)
	}
}

// TestSimplePatternTokenizer_Unicode tests Unicode handling.
func TestSimplePatternTokenizer_Unicode(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		input    string
		expected []string
	}{
		{
			name:     "Unicode letters",
			pattern:  `\p{L}+`,
			input:    "Hello 世界",
			expected: []string{"Hello", "世界"},
		},
		{
			name:     "Unicode words",
			pattern:  `\w+`,
			input:    "test_123 テスト",
			expected: []string{"test_123"},
		},
		{
			name:     "emoji pattern",
			pattern:  `[\x{1F600}-\x{1F64F}]`,
			input:    "Hello 😀 World 😁",
			expected: []string{"😀", "😁"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer, err := NewSimplePatternTokenizer(tt.pattern)
			if err != nil {
				t.Fatalf("Failed to create tokenizer: %v", err)
			}

			if err := tokenizer.SetReader(strings.NewReader(tt.input)); err != nil {
				t.Fatalf("SetReader failed: %v", err)
			}

			var tokens []string
			for {
				hasToken, err := tokenizer.IncrementToken()
				if err != nil {
					t.Fatalf("IncrementToken failed: %v", err)
				}
				if !hasToken {
					break
				}

				termAttr := tokenizer.GetAttributeSource().GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
				term := termAttr.(CharTermAttribute).String()
				tokens = append(tokens, term)
			}

			if len(tokens) != len(tt.expected) {
				t.Errorf("Got %d tokens, want %d: got %v, want %v", len(tokens), len(tt.expected), tokens, tt.expected)
				return
			}

			for i, expected := range tt.expected {
				if tokens[i] != expected {
					t.Errorf("Token %d = %q, want %q", i, tokens[i], expected)
				}
			}
		})
	}
}

// TestSimplePatternTokenizer_LargeInput tests handling large input.
func TestSimplePatternTokenizer_LargeInput(t *testing.T) {
	// Create a large input with repeated pattern
	large := strings.Repeat("word ", 1000)

	tokenizer, err := NewSimplePatternTokenizer(`\w+`)
	if err != nil {
		t.Fatalf("Failed to create tokenizer: %v", err)
	}

	if err := tokenizer.SetReader(strings.NewReader(large)); err != nil {
		t.Fatalf("SetReader failed: %v", err)
	}

	count := 0
	for {
		hasToken, err := tokenizer.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken failed: %v", err)
		}
		if !hasToken {
			break
		}
		count++
	}

	if count != 1000 {
		t.Errorf("Token count = %d, want 1000", count)
	}
}

// TestSimplePatternTokenizer_Factory tests the factory creation.
func TestSimplePatternTokenizer_Factory(t *testing.T) {
	factory, err := NewSimplePatternTokenizerFactory(`\w+`)
	if err != nil {
		t.Fatalf("Failed to create factory: %v", err)
	}

	if factory.GetPattern() != `\w+` {
		t.Errorf("Factory pattern = %q, want %q", factory.GetPattern(), `\w+`)
	}

	tokenizer := factory.Create()
	if tokenizer == nil {
		t.Fatal("Factory.Create returned nil")
	}

	// Verify it's a SimplePatternTokenizer
	concreteTokenizer, ok := tokenizer.(*SimplePatternTokenizer)
	if !ok {
		t.Fatal("Factory did not create SimplePatternTokenizer")
	}

	// Test that the tokenizer works
	if err := concreteTokenizer.SetReader(strings.NewReader("hello world")); err != nil {
		t.Fatalf("SetReader failed: %v", err)
	}

	hasToken, err := concreteTokenizer.IncrementToken()
	if err != nil {
		t.Fatalf("IncrementToken failed: %v", err)
	}
	if !hasToken {
		t.Fatal("Expected token")
	}

	termAttr := concreteTokenizer.GetAttributeSource().GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
	term := termAttr.(CharTermAttribute).String()
	if term != "hello" {
		t.Errorf("Token = %q, want 'hello'", term)
	}
}

// TestSimplePatternTokenizer_FactoryInvalidPattern tests factory with invalid pattern.
func TestSimplePatternTokenizer_FactoryInvalidPattern(t *testing.T) {
	_, err := NewSimplePatternTokenizerFactory(`[invalid`)
	if err == nil {
		t.Error("Expected error for invalid pattern, got nil")
	}
}

// TestSimplePatternTokenizer_OverlappingMatches tests overlapping pattern matches.
func TestSimplePatternTokenizer_OverlappingMatches(t *testing.T) {
	// Pattern that could match overlapping regions
	// Note: regexp.FindAllStringIndex doesn't return overlapping matches
	// This test verifies the standard behavior
	tokenizer, err := NewSimplePatternTokenizer(`a+`)
	if err != nil {
		t.Fatalf("Failed to create tokenizer: %v", err)
	}

	if err := tokenizer.SetReader(strings.NewReader("aaaa")); err != nil {
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
	if term != "aaaa" {
		t.Errorf("Token = %q, want 'aaaa'", term)
	}

	// Should be only one match for "aaaa"
	hasMore, _ := tokenizer.IncrementToken()
	if hasMore {
		t.Error("Expected only one token")
	}
}

// TestSimplePatternTokenizer_SingleToken tests input that produces single token.
func TestSimplePatternTokenizer_SingleToken(t *testing.T) {
	tokenizer, err := NewSimplePatternTokenizer(`\w+`)
	if err != nil {
		t.Fatalf("Failed to create tokenizer: %v", err)
	}

	if err := tokenizer.SetReader(strings.NewReader("single")); err != nil {
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
	if term != "single" {
		t.Errorf("Token = %q, want 'single'", term)
	}

	hasMore, _ := tokenizer.IncrementToken()
	if hasMore {
		t.Error("Expected only one token")
	}
}

// Benchmark tests
func BenchmarkSimplePatternTokenizer_Short(b *testing.B) {
	input := "Hello, World! How are you?"
	reader := strings.NewReader(input)
	tokenizer, _ := NewSimplePatternTokenizer(`\w+`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader.Reset(input)
		tokenizer.SetReader(reader)
		for {
			hasToken, _ := tokenizer.IncrementToken()
			if !hasToken {
				break
			}
		}
	}
}

func BenchmarkSimplePatternTokenizer_Large(b *testing.B) {
	input := strings.Repeat("word ", 1000)
	reader := strings.NewReader(input)
	tokenizer, _ := NewSimplePatternTokenizer(`\w+`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader.Reset(input)
		tokenizer.SetReader(reader)
		for {
			hasToken, _ := tokenizer.IncrementToken()
			if !hasToken {
				break
			}
		}
	}
}

func BenchmarkSimplePatternTokenizer_ComplexPattern(b *testing.B) {
	input := strings.Repeat("email@example.com ", 100)
	reader := strings.NewReader(input)
	// Complex pattern for email-like tokens
	tokenizer, _ := NewSimplePatternTokenizer(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader.Reset(input)
		tokenizer.SetReader(reader)
		for {
			hasToken, _ := tokenizer.IncrementToken()
			if !hasToken {
				break
			}
		}
	}
}
