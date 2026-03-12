// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"strings"
	"testing"
)

func TestLetterTokenizer_Basic(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{
			input:    "Hello World",
			expected: []string{"Hello", "World"},
		},
		{
			input:    "Hello123 World456",
			expected: []string{"Hello", "World"},
		},
		{
			input:    "test@test.com",
			expected: []string{"test", "test", "com"}, // @ and . are separators
		},
		{
			input:    "Hello, World! How are you?",
			expected: []string{"Hello", "World", "How", "are", "you"},
		},
		{
			input:    "  leading   spaces  ",
			expected: []string{"leading", "spaces"},
		},
		{
			input:    "mixed-Case_Example",
			expected: []string{"mixed", "Case", "Example"},
		},
		{
			input:    "C++ Java# Python@",
			expected: []string{"C", "Java", "Python"},
		},
		{
			input:    "",
			expected: nil,
		},
		{
			input:    "123 456",
			expected: nil,
		},
		{
			input:    "!@#$%",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokenizer := NewLetterTokenizer()
			err := tokenizer.SetReader(strings.NewReader(tt.input))
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

func TestLetterTokenizer_Unicode(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{
			input:    "café résumé",
			expected: []string{"café", "résumé"},
		},
		{
			input:    "日本語 テスト",
			expected: []string{"日本語", "テスト"},
		},
		{
			input:    "Привет мир",
			expected: []string{"Привет", "мир"},
		},
		{
			input:    "Héllo Wörld",
			expected: []string{"Héllo", "Wörld"},
		},
	}

	for _, tt := range tests {
		t.Run("unicode", func(t *testing.T) {
			tokenizer := NewLetterTokenizer()
			err := tokenizer.SetReader(strings.NewReader(tt.input))
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

func TestLetterTokenizer_Offsets(t *testing.T) {
	tokenizer := NewLetterTokenizer()
	input := "Hello, World!"
	err := tokenizer.SetReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("SetReader failed: %v", err)
	}

	expected := []struct {
		term  string
		start int
		end   int
	}{
		{"Hello", 0, 5},
		{"World", 7, 12},
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

		if i >= len(expected) {
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

		if cta.String() != expected[i].term {
			t.Errorf("Token %d: expected term %q, got %q", i, expected[i].term, cta.String())
		}
		if oa.StartOffset() != expected[i].start {
			t.Errorf("Token %d: expected start offset %d, got %d", i, expected[i].start, oa.StartOffset())
		}
		if oa.EndOffset() != expected[i].end {
			t.Errorf("Token %d: expected end offset %d, got %d", i, expected[i].end, oa.EndOffset())
		}

		i++
	}
}

func TestLetterTokenizer_Reset(t *testing.T) {
	tokenizer := NewLetterTokenizer()

	// First run
	err := tokenizer.SetReader(strings.NewReader("Hello"))
	if err != nil {
		t.Fatalf("SetReader failed: %v", err)
	}

	hasToken, _ := tokenizer.IncrementToken()
	if !hasToken {
		t.Error("Expected token on first run")
	}

	// Reset and second run
	err = tokenizer.Reset()
	if err != nil {
		t.Fatalf("Reset failed: %v", err)
	}

	err = tokenizer.SetReader(strings.NewReader("World"))
	if err != nil {
		t.Fatalf("SetReader failed: %v", err)
	}

	hasToken, _ = tokenizer.IncrementToken()
	if !hasToken {
		t.Error("Expected token on second run")
	}

	termAttr := tokenizer.GetAttributeSource().GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
	cta, ok := termAttr.(CharTermAttribute)
	if !ok {
		t.Fatalf("Failed to cast attribute")
	}
	if cta.String() != "World" {
		t.Errorf("Expected token 'World', got %q", cta.String())
	}
}
