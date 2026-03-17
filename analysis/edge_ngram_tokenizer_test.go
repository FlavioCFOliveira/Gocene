// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"strings"
	"testing"
)

func TestEdgeNGramTokenizer_Basic(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		minGram  int
		maxGram  int
		expected []string
	}{
		{
			name:     "simple word",
			input:    "hello",
			minGram:  2,
			maxGram:  4,
			expected: []string{"he", "hel", "hell"},
		},
		{
			name:     "exact minGram equals input length",
			input:    "hi",
			minGram:  2,
			maxGram:  4,
			expected: []string{"hi"},
		},
		{
			name:     "input shorter than minGram",
			input:    "a",
			minGram:  2,
			maxGram:  4,
			expected: nil,
		},
		{
			name:     "minGram equals maxGram",
			input:    "hello",
			minGram:  3,
			maxGram:  3,
			expected: []string{"hel"},
		},
		{
			name:     "minGram 1",
			input:    "abc",
			minGram:  1,
			maxGram:  3,
			expected: []string{"a", "ab", "abc"},
		},
		{
			name:     "large maxGram",
			input:    "test",
			minGram:  2,
			maxGram:  100,
			expected: []string{"te", "tes", "test"},
		},
		{
			name:     "empty input",
			input:    "",
			minGram:  2,
			maxGram:  4,
			expected: nil,
		},
		{
			name:     "single char with minGram 1",
			input:    "x",
			minGram:  1,
			maxGram:  3,
			expected: []string{"x"},
		},
		{
			name:     "with spaces",
			input:    "hello world",
			minGram:  3,
			maxGram:  6,
			expected: []string{"hel", "hell", "hello", "hello "},
		},
		{
			name:     "with punctuation",
			input:    "test@example",
			minGram:  4,
			maxGram:  8,
			expected: []string{"test", "test@", "test@e", "test@ex", "test@exa"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer, err := NewEdgeNGramTokenizer(tt.minGram, tt.maxGram)
			if err != nil {
				t.Fatalf("NewEdgeNGramTokenizer failed: %v", err)
			}

			err = tokenizer.SetReader(strings.NewReader(tt.input))
			if err != nil {
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
				if termAttr != nil {
					if cta, ok := termAttr.(CharTermAttribute); ok {
						tokens = append(tokens, cta.String())
					}
				}
			}

			if len(tokens) != len(tt.expected) {
				t.Errorf("Expected %d tokens, got %d: %v", len(tt.expected), len(tokens), tokens)
				return
			}

			for i := range tokens {
				if tokens[i] != tt.expected[i] {
					t.Errorf("Token %d: expected %q, got %q", i, tt.expected[i], tokens[i])
				}
			}
		})
	}
}

func TestEdgeNGramTokenizer_Unicode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		minGram  int
		maxGram  int
		expected []string
	}{
		{
			name:     "Unicode characters",
			input:    "café",
			minGram:  2,
			maxGram:  4,
			expected: []string{"ca", "caf", "café"},
		},
		{
			name:     "CJK characters",
			input:    "日本語",
			minGram:  1,
			maxGram:  3,
			expected: []string{"日", "日本", "日本語"},
		},
		{
			name:     "Russian characters",
			input:    "Привет",
			minGram:  2,
			maxGram:  4,
			expected: []string{"Пр", "При", "Прив"},
		},
		{
			name:     "Emoji",
			input:    "🎉🚀💻",
			minGram:  1,
			maxGram:  2,
			expected: []string{"🎉", "🎉🚀"},
		},
		{
			name:     "Mixed Unicode",
			input:    "Hello世界",
			minGram:  3,
			maxGram:  6,
			expected: []string{"Hel", "Hell", "Hello", "Hello世"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer, err := NewEdgeNGramTokenizer(tt.minGram, tt.maxGram)
			if err != nil {
				t.Fatalf("NewEdgeNGramTokenizer failed: %v", err)
			}

			err = tokenizer.SetReader(strings.NewReader(tt.input))
			if err != nil {
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
				if termAttr != nil {
					if cta, ok := termAttr.(CharTermAttribute); ok {
						tokens = append(tokens, cta.String())
					}
				}
			}

			if len(tokens) != len(tt.expected) {
				t.Errorf("Expected %d tokens, got %d: %v", len(tt.expected), len(tokens), tokens)
				return
			}

			for i := range tokens {
				if tokens[i] != tt.expected[i] {
					t.Errorf("Token %d: expected %q, got %q", i, tt.expected[i], tokens[i])
				}
			}
		})
	}
}

func TestEdgeNGramTokenizer_Offsets(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		minGram       int
		maxGram       int
		expectedTerms []string
		expectedStart []int
		expectedEnd   []int
	}{
		{
			name:          "ASCII offsets",
			input:         "hello",
			minGram:       2,
			maxGram:       4,
			expectedTerms: []string{"he", "hel", "hell"},
			expectedStart: []int{0, 0, 0},
			expectedEnd:   []int{2, 3, 4},
		},
		{
			name:          "Unicode byte offsets",
			input:         "café", // é is 2 bytes in UTF-8
			minGram:       2,
			maxGram:       3,
			expectedTerms: []string{"ca", "caf"},
			expectedStart: []int{0, 0},
			expectedEnd:   []int{2, 3},
		},
		{
			name:          "CJK byte offsets",
			input:         "日本語", // each CJK char is 3 bytes in UTF-8
			minGram:       1,
			maxGram:       2,
			expectedTerms: []string{"日", "日本"},
			expectedStart: []int{0, 0},
			expectedEnd:   []int{3, 6}, // byte offsets
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer, err := NewEdgeNGramTokenizer(tt.minGram, tt.maxGram)
			if err != nil {
				t.Fatalf("NewEdgeNGramTokenizer failed: %v", err)
			}

			err = tokenizer.SetReader(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("SetReader failed: %v", err)
			}

			i := 0
			for {
				hasToken, err := tokenizer.IncrementToken()
				if err != nil {
					t.Fatalf("IncrementToken failed: %v", err)
				}
				if !hasToken {
					break
				}

				if i >= len(tt.expectedTerms) {
					t.Errorf("More tokens than expected")
					break
				}

				termAttr := tokenizer.GetAttributeSource().GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
				offsetAttr := tokenizer.GetAttributeSource().GetAttributeByType(reflect.TypeOf(&offsetAttribute{}))

				cta, ok1 := termAttr.(CharTermAttribute)
				oa, ok2 := offsetAttr.(OffsetAttribute)
				if !ok1 || !ok2 {
					t.Fatalf("Failed to cast attributes")
				}

				if cta.String() != tt.expectedTerms[i] {
					t.Errorf("Token %d: expected term %q, got %q", i, tt.expectedTerms[i], cta.String())
				}
				if oa.StartOffset() != tt.expectedStart[i] {
					t.Errorf("Token %d: expected start offset %d, got %d", i, tt.expectedStart[i], oa.StartOffset())
				}
				if oa.EndOffset() != tt.expectedEnd[i] {
					t.Errorf("Token %d: expected end offset %d, got %d", i, tt.expectedEnd[i], oa.EndOffset())
				}

				i++
			}

			if i != len(tt.expectedTerms) {
				t.Errorf("Expected %d tokens, got %d", len(tt.expectedTerms), i)
			}
		})
	}
}

func TestEdgeNGramTokenizer_PositionIncrement(t *testing.T) {
	tokenizer, err := NewEdgeNGramTokenizer(2, 4)
	if err != nil {
		t.Fatalf("NewEdgeNGramTokenizer failed: %v", err)
	}

	err = tokenizer.SetReader(strings.NewReader("hello"))
	if err != nil {
		t.Fatalf("SetReader failed: %v", err)
	}

	expectedIncrements := []int{1, 0, 0} // First token at position 1, others at same position
	i := 0

	for {
		hasToken, err := tokenizer.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken failed: %v", err)
		}
		if !hasToken {
			break
		}

		if i >= len(expectedIncrements) {
			t.Errorf("More tokens than expected")
			break
		}

		posIncrAttr := tokenizer.GetAttributeSource().GetAttributeByType(reflect.TypeOf(&positionIncrementAttribute{}))
		pia, ok := posIncrAttr.(PositionIncrementAttribute)
		if !ok {
			t.Fatalf("Failed to cast PositionIncrementAttribute")
		}

		if pia.GetPositionIncrement() != expectedIncrements[i] {
			t.Errorf("Token %d: expected position increment %d, got %d", i, expectedIncrements[i], pia.GetPositionIncrement())
		}

		i++
	}
}

func TestEdgeNGramTokenizer_Reset(t *testing.T) {
	tokenizer, err := NewEdgeNGramTokenizer(2, 4)
	if err != nil {
		t.Fatalf("NewEdgeNGramTokenizer failed: %v", err)
	}

	// First run
	err = tokenizer.SetReader(strings.NewReader("hello"))
	if err != nil {
		t.Fatalf("SetReader failed: %v", err)
	}

	tokens1 := []string{}
	for {
		hasToken, err := tokenizer.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken failed: %v", err)
		}
		if !hasToken {
			break
		}

		termAttr := tokenizer.GetAttributeSource().GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
		if cta, ok := termAttr.(CharTermAttribute); ok {
			tokens1 = append(tokens1, cta.String())
		}
	}

	// Reset and second run
	err = tokenizer.Reset()
	if err != nil {
		t.Fatalf("Reset failed: %v", err)
	}

	err = tokenizer.SetReader(strings.NewReader("world"))
	if err != nil {
		t.Fatalf("SetReader failed: %v", err)
	}

	tokens2 := []string{}
	for {
		hasToken, err := tokenizer.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken failed: %v", err)
		}
		if !hasToken {
			break
		}

		termAttr := tokenizer.GetAttributeSource().GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
		if cta, ok := termAttr.(CharTermAttribute); ok {
			tokens2 = append(tokens2, cta.String())
		}
	}

	// Verify second run produced correct tokens
	expected := []string{"wo", "wor", "worl"}
	if len(tokens2) != len(expected) {
		t.Errorf("Expected %d tokens after reset, got %d: %v", len(expected), len(tokens2), tokens2)
		return
	}

	for i := range tokens2 {
		if tokens2[i] != expected[i] {
			t.Errorf("Token %d after reset: expected %q, got %q", i, expected[i], tokens2[i])
		}
	}

	// Verify first and second runs produced different tokens
	if len(tokens1) == len(tokens2) {
		same := true
		for i := range tokens1 {
			if tokens1[i] != tokens2[i] {
				same = false
				break
			}
		}
		if same {
			t.Error("First and second runs produced identical tokens - reset may not be working")
		}
	}
}

func TestEdgeNGramTokenizer_End(t *testing.T) {
	tokenizer, err := NewEdgeNGramTokenizer(2, 4)
	if err != nil {
		t.Fatalf("NewEdgeNGramTokenizer failed: %v", err)
	}

	input := "hello"
	err = tokenizer.SetReader(strings.NewReader(input))
	if err != nil {
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
	err = tokenizer.End()
	if err != nil {
		t.Fatalf("End failed: %v", err)
	}

	// Check final offset
	offsetAttr := tokenizer.GetAttributeSource().GetAttributeByType(reflect.TypeOf(&offsetAttribute{}))
	oa, ok := offsetAttr.(OffsetAttribute)
	if !ok {
		t.Fatalf("Failed to cast OffsetAttribute")
	}

	if oa.EndOffset() != len(input) {
		t.Errorf("End offset = %d, want %d", oa.EndOffset(), len(input))
	}
}

func TestEdgeNGramTokenizer_ConstructorErrors(t *testing.T) {
	tests := []struct {
		name    string
		minGram int
		maxGram int
		wantErr bool
	}{
		{
			name:    "minGram less than 1",
			minGram: 0,
			maxGram: 4,
			wantErr: true,
		},
		{
			name:    "maxGram less than minGram",
			minGram: 4,
			maxGram: 2,
			wantErr: true,
		},
		{
			name:    "valid parameters",
			minGram: 1,
			maxGram: 10,
			wantErr: false,
		},
		{
			name:    "minGram equals maxGram",
			minGram: 3,
			maxGram: 3,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewEdgeNGramTokenizer(tt.minGram, tt.maxGram)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewEdgeNGramTokenizer() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEdgeNGramTokenizerFactory(t *testing.T) {
	factory, err := NewEdgeNGramTokenizerFactory(2, 4)
	if err != nil {
		t.Fatalf("NewEdgeNGramTokenizerFactory failed: %v", err)
	}

	tokenizer := factory.Create()
	if tokenizer == nil {
		t.Fatal("Factory.Create returned nil")
	}

	// Verify it's an EdgeNGramTokenizer
	edgeTokenizer, ok := tokenizer.(*EdgeNGramTokenizer)
	if !ok {
		t.Error("Factory did not create EdgeNGramTokenizer")
		return
	}

	if edgeTokenizer.GetMinGram() != 2 {
		t.Errorf("MinGram = %d, want 2", edgeTokenizer.GetMinGram())
	}

	if edgeTokenizer.GetMaxGram() != 4 {
		t.Errorf("MaxGram = %d, want 4", edgeTokenizer.GetMaxGram())
	}
}

func TestEdgeNGramTokenizerFactory_Error(t *testing.T) {
	_, err := NewEdgeNGramTokenizerFactory(0, 4)
	if err == nil {
		t.Error("Expected error for invalid minGram, got nil")
	}

	_, err = NewEdgeNGramTokenizerFactory(4, 2)
	if err == nil {
		t.Error("Expected error for maxGram < minGram, got nil")
	}
}

func TestEdgeNGramTokenizer_LargeInput(t *testing.T) {
	// Create a large input
	large := strings.Repeat("a", 1000)

	tokenizer, err := NewEdgeNGramTokenizer(2, 5)
	if err != nil {
		t.Fatalf("NewEdgeNGramTokenizer failed: %v", err)
	}

	err = tokenizer.SetReader(strings.NewReader(large))
	if err != nil {
		t.Fatalf("SetReader failed: %v", err)
	}

	// Should produce 4 tokens: "aa", "aaa", "aaaa", "aaaaa"
	expectedCount := 4
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

	if count != expectedCount {
		t.Errorf("Expected %d tokens, got %d", expectedCount, count)
	}
}

// Benchmark tests
func BenchmarkEdgeNGramTokenizer_Short(b *testing.B) {
	input := "hello"
	tokenizer, _ := NewEdgeNGramTokenizer(2, 4)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tokenizer.SetReader(strings.NewReader(input))
		for {
			hasToken, _ := tokenizer.IncrementToken()
			if !hasToken {
				break
			}
		}
		tokenizer.Reset()
	}
}

func BenchmarkEdgeNGramTokenizer_Medium(b *testing.B) {
	input := strings.Repeat("hello", 20) // 100 characters
	tokenizer, _ := NewEdgeNGramTokenizer(2, 10)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tokenizer.SetReader(strings.NewReader(input))
		for {
			hasToken, _ := tokenizer.IncrementToken()
			if !hasToken {
				break
			}
		}
		tokenizer.Reset()
	}
}

func BenchmarkEdgeNGramTokenizer_Unicode(b *testing.B) {
	input := "日本語テストデータ"
	tokenizer, _ := NewEdgeNGramTokenizer(2, 5)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tokenizer.SetReader(strings.NewReader(input))
		for {
			hasToken, _ := tokenizer.IncrementToken()
			if !hasToken {
				break
			}
		}
		tokenizer.Reset()
	}
}
