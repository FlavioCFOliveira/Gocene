// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestKeywordAnalyzer_Basic tests basic keyword analyzer functionality.
func TestKeywordAnalyzer_Basic(t *testing.T) {
	analyzer := NewKeywordAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"single word", "hello", "hello"},
		{"multiple words", "hello world", "hello world"},
		{"with punctuation", "Hello, World!", "Hello, World!"},
		{"numbers", "12345", "12345"},
		{"mixed", "test 123 !@#", "test 123 !@#"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream, err := analyzer.TokenStream("field", strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("TokenStream failed: %v", err)
			}
			defer stream.Close()

			hasToken, err := stream.IncrementToken()
			if err != nil {
				t.Fatalf("IncrementToken failed: %v", err)
			}

			if !hasToken {
				t.Fatalf("Expected token for input %q", tt.input)
			}

			// Get attribute source
			attrSrc := stream.(interface{ GetAttributeSource() *AttributeSource }).GetAttributeSource()
			termAttr := attrSrc.GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
			if termAttr == nil {
				t.Fatal("CharTermAttribute is nil")
			}

			term := termAttr.(CharTermAttribute).String()
			if term != tt.expected {
				t.Errorf("Token = %q, want %q", term, tt.expected)
			}

			// Should be only one token
			hasMore, err := stream.IncrementToken()
			if err != nil {
				t.Fatalf("Second IncrementToken failed: %v", err)
			}
			if hasMore {
				t.Error("KeywordAnalyzer should produce exactly one token")
			}
		})
	}
}

// TestKeywordAnalyzer_OffsetAttribute tests offset attribute handling.
func TestKeywordAnalyzer_OffsetAttribute(t *testing.T) {
	analyzer := NewKeywordAnalyzer()
	defer analyzer.Close()

	input := "Hello World"
	stream, err := analyzer.TokenStream("field", strings.NewReader(input))
	if err != nil {
		t.Fatalf("TokenStream failed: %v", err)
	}
	defer stream.Close()

	hasToken, err := stream.IncrementToken()
	if err != nil {
		t.Fatalf("IncrementToken failed: %v", err)
	}
	if !hasToken {
		t.Fatal("Expected token")
	}

	attrSrc := stream.(interface{ GetAttributeSource() *AttributeSource }).GetAttributeSource()
	offsetAttr := attrSrc.GetAttributeByType(reflect.TypeOf(&offsetAttribute{}))
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

// TestKeywordAnalyzer_Unicode tests Unicode handling.
func TestKeywordAnalyzer_Unicode(t *testing.T) {
	analyzer := NewKeywordAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		name  string
		input string
	}{
		{"Chinese", "你好世界"},
		{"Japanese", "こんにちは"},
		{"Arabic", "مرحبا"},
		{"Russian", "Привет"},
		{"Emoji", "🎉🚀💻"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream, err := analyzer.TokenStream("field", strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("TokenStream failed: %v", err)
			}
			defer stream.Close()

			hasToken, err := stream.IncrementToken()
			if err != nil {
				t.Fatalf("IncrementToken failed: %v", err)
			}
			if !hasToken {
				t.Fatal("Expected token")
			}

			attrSrc := stream.(interface{ GetAttributeSource() *AttributeSource }).GetAttributeSource()
			termAttr := attrSrc.GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
			term := termAttr.(CharTermAttribute).String()
			if term != tt.input {
				t.Errorf("Unicode token = %q, want %q", term, tt.input)
			}
		})
	}
}

// TestKeywordAnalyzer_PreserveInput tests that input is preserved exactly.
func TestKeywordAnalyzer_PreserveInput(t *testing.T) {
	analyzer := NewKeywordAnalyzer()
	defer analyzer.Close()

	// KeywordAnalyzer should preserve input exactly, including:
	// - Case (no lowercasing)
	// - Whitespace
	// - Punctuation
	// - Special characters
	inputs := []string{
		"Hello World",
		"  leading spaces",
		"trailing spaces  ",
		"  both  spaces  ",
		"UPPERCASE",
		"MiXeD CaSe",
		"test@email.com",
		"http://example.com/path?query=value",
		"a@b#c$d%e",
		"123 + 456 = 579",
	}

	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			stream, err := analyzer.TokenStream("field", strings.NewReader(input))
			if err != nil {
				t.Fatalf("TokenStream failed: %v", err)
			}
			defer stream.Close()

			stream.IncrementToken()
			attrSrc := stream.(interface{ GetAttributeSource() *AttributeSource }).GetAttributeSource()
			termAttr := attrSrc.GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
			term := termAttr.(CharTermAttribute).String()

			if term != input {
				t.Errorf("Input not preserved: got %q, want %q", term, input)
			}
		})
	}
}

// TestKeywordAnalyzer_MultipleDocuments tests that multiple documents can be processed.
// This is the Go port of Lucene's TestKeywordAnalyzer.testMutipleDocument().
func TestKeywordAnalyzer_MultipleDocuments(t *testing.T) {
	analyzer := NewKeywordAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		name  string
		input string
	}{
		{"first document", "Q36"},
		{"second document", "Q37"},
		{"third document", "Q38"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream, err := analyzer.TokenStream("partnum", strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("TokenStream failed: %v", err)
			}
			defer stream.Close()

			hasToken, err := stream.IncrementToken()
			if err != nil {
				t.Fatalf("IncrementToken failed: %v", err)
			}
			if !hasToken {
				t.Fatal("Expected token")
			}

			attrSrc := stream.(interface{ GetAttributeSource() *AttributeSource }).GetAttributeSource()
			termAttr := attrSrc.GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
			term := termAttr.(CharTermAttribute).String()

			if term != tt.input {
				t.Errorf("Token = %q, want %q", term, tt.input)
			}

			// Should be only one token per document
			hasMore, err := stream.IncrementToken()
			if err != nil {
				t.Fatalf("Second IncrementToken failed: %v", err)
			}
			if hasMore {
				t.Error("KeywordAnalyzer should produce exactly one token per document")
			}
		})
	}
}

// TestKeywordAnalyzer_RandomStrings tests the analyzer with random strings.
// This is the Go port of Lucene's TestKeywordAnalyzer.testRandomStrings().
func TestKeywordAnalyzer_RandomStrings(t *testing.T) {
	analyzer := NewKeywordAnalyzer()
	defer analyzer.Close()

	// Seed random number generator
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Test with multiple random strings
	iterations := 200
	for i := 0; i < iterations; i++ {
		// Generate random string of random length (up to 100 characters)
		length := rng.Intn(100) + 1
		input := generateRandomString(rng, length)

		stream, err := analyzer.TokenStream("field", strings.NewReader(input))
		if err != nil {
			t.Fatalf("TokenStream failed at iteration %d: %v", i, err)
		}

		hasToken, err := stream.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken failed at iteration %d: %v", i, err)
		}
		if !hasToken {
			t.Fatalf("Expected token at iteration %d", i)
		}

		attrSrc := stream.(interface{ GetAttributeSource() *AttributeSource }).GetAttributeSource()
		termAttr := attrSrc.GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
		term := termAttr.(CharTermAttribute).String()

		// KeywordAnalyzer should preserve the entire input as a single token
		if term != input {
			t.Errorf("Iteration %d: Token = %q, want %q", i, term, input)
		}

		// Should be only one token
		hasMore, err := stream.IncrementToken()
		if err != nil {
			t.Fatalf("Second IncrementToken failed at iteration %d: %v", i, err)
		}
		if hasMore {
			t.Errorf("Iteration %d: KeywordAnalyzer should produce exactly one token", i)
		}

		stream.Close()
	}
}

// generateRandomString generates a random string of the specified length.
func generateRandomString(rng *rand.Rand, length int) string {
	// Use a mix of ASCII and some Unicode characters
	var chars = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789" +
		"!@#$%^&*()_+-=[]{}|;':\",./<>?" +
		"你好世界こんにちはمرحباПривет")

	var result strings.Builder
	for i := 0; i < length; i++ {
		result.WriteRune(chars[rng.Intn(len(chars))])
	}
	return result.String()
}

// TestKeywordAnalyzer_Offsets tests offset attribute handling (LUCENE-1441).
// This is the Go port of Lucene's TestKeywordAnalyzer.testOffsets().
func TestKeywordAnalyzer_Offsets(t *testing.T) {
	analyzer := NewKeywordAnalyzer()
	defer analyzer.Close()

	input := "abcd"
	stream, err := analyzer.TokenStream("field", strings.NewReader(input))
	if err != nil {
		t.Fatalf("TokenStream failed: %v", err)
	}
	defer stream.Close()

	hasToken, err := stream.IncrementToken()
	if err != nil {
		t.Fatalf("IncrementToken failed: %v", err)
	}
	if !hasToken {
		t.Fatal("Expected token")
	}

	attrSrc := stream.(interface{ GetAttributeSource() *AttributeSource }).GetAttributeSource()
	offsetAttr := attrSrc.GetAttributeByType(reflect.TypeOf(&offsetAttribute{}))
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

	// Should be no more tokens
	hasMore, err := stream.IncrementToken()
	if err != nil {
		t.Fatalf("Second IncrementToken failed: %v", err)
	}
	if hasMore {
		t.Error("Expected no more tokens")
	}
}

// Benchmark tests
func BenchmarkKeywordAnalyzer_Short(b *testing.B) {
	analyzer := NewKeywordAnalyzer()
	defer analyzer.Close()
	input := "hello world"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stream, _ := analyzer.TokenStream("field", strings.NewReader(input))
		stream.IncrementToken()
		stream.Close()
	}
}

func BenchmarkKeywordAnalyzer_Large(b *testing.B) {
	analyzer := NewKeywordAnalyzer()
	defer analyzer.Close()
	input := strings.Repeat("a", 10000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stream, _ := analyzer.TokenStream("field", strings.NewReader(input))
		stream.IncrementToken()
		stream.Close()
	}
}