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

func TestSimplePatternSplitTokenizer_BasicWhitespace(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		pattern  string
		expected []string
	}{
		{
			name:     "simple words",
			input:    "Hello World",
			pattern:  `\s+`,
			expected: []string{"Hello", "World"},
		},
		{
			name:     "multiple spaces",
			input:    "Hello   World",
			pattern:  `\s+`,
			expected: []string{"Hello", "World"},
		},
		{
			name:     "three words",
			input:    "one two three",
			pattern:  `\s+`,
			expected: []string{"one", "two", "three"},
		},
		{
			name:     "leading spaces",
			input:    "  leading spaces",
			pattern:  `\s+`,
			expected: []string{"leading", "spaces"},
		},
		{
			name:     "trailing spaces",
			input:    "trailing spaces  ",
			pattern:  `\s+`,
			expected: []string{"trailing", "spaces"},
		},
		{
			name:     "both leading and trailing spaces",
			input:    "  both  spaces  ",
			pattern:  `\s+`,
			expected: []string{"both", "spaces"},
		},
		{
			name:     "tabs and newlines",
			input:    "hello\tworld\ntest",
			pattern:  `\s+`,
			expected: []string{"hello", "world", "test"},
		},
		{
			name:     "mixed whitespace",
			input:    "a \t b \n c",
			pattern:  `\s+`,
			expected: []string{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer, err := NewSimplePatternSplitTokenizerWithString(tt.pattern)
			if err != nil {
				t.Fatalf("Failed to create tokenizer: %v", err)
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

func TestSimplePatternSplitTokenizer_Punctuation(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		pattern  string
		expected []string
	}{
		{
			name:     "comma separated",
			input:    "Hello,World,Test",
			pattern:  `,`,
			expected: []string{"Hello", "World", "Test"},
		},
		{
			name:     "period separated",
			input:    "Hello.World.Test",
			pattern:  `\.`,
			expected: []string{"Hello", "World", "Test"},
		},
		{
			name:     "comma or period",
			input:    "Hello,World.Test",
			pattern:  `[,.]`,
			expected: []string{"Hello", "World", "Test"},
		},
		{
			name:     "semicolon separated",
			input:    "a;b;c",
			pattern:  `;`,
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "pipe separated",
			input:    "a|b|c",
			pattern:  `\|`,
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "mixed punctuation",
			input:    "a,b.c;d|e",
			pattern:  `[,.;|]`,
			expected: []string{"a", "b", "c", "d", "e"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer, err := NewSimplePatternSplitTokenizerWithString(tt.pattern)
			if err != nil {
				t.Fatalf("Failed to create tokenizer: %v", err)
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

func TestSimplePatternSplitTokenizer_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		pattern  string
		expected []string
	}{
		{
			name:     "empty input",
			input:    "",
			pattern:  `\s+`,
			expected: nil,
		},
		{
			name:     "only delimiters",
			input:    "   ",
			pattern:  `\s+`,
			expected: nil,
		},
		{
			name:     "single token no delimiter",
			input:    "Hello",
			pattern:  `\s+`,
			expected: []string{"Hello"},
		},
		{
			name:     "consecutive delimiters",
			input:    "a,,b,,,c",
			pattern:  `,`,
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "delimiter at start",
			input:    ",Hello",
			pattern:  `,`,
			expected: []string{"Hello"},
		},
		{
			name:     "delimiter at end",
			input:    "Hello,",
			pattern:  `,`,
			expected: []string{"Hello"},
		},
		{
			name:     "delimiters at both ends",
			input:    ",Hello,",
			pattern:  `,`,
			expected: []string{"Hello"},
		},
		{
			name:     "unicode text",
			input:    "Hello 世界 Test",
			pattern:  `\s+`,
			expected: []string{"Hello", "世界", "Test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer, err := NewSimplePatternSplitTokenizerWithString(tt.pattern)
			if err != nil {
				t.Fatalf("Failed to create tokenizer: %v", err)
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

func TestSimplePatternSplitTokenizer_Offsets(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		pattern        string
		expectedTerms  []string
		expectedStarts []int
		expectedEnds   []int
	}{
		{
			name:           "basic offsets",
			input:          "Hello World",
			pattern:        `\s+`,
			expectedTerms:  []string{"Hello", "World"},
			expectedStarts: []int{0, 6},
			expectedEnds:   []int{5, 11},
		},
		{
			name:           "comma offsets",
			input:          "Hello,World,Test",
			pattern:        `,`,
			expectedTerms:  []string{"Hello", "World", "Test"},
			expectedStarts: []int{0, 6, 12},
			expectedEnds:   []int{5, 11, 16},
		},
		{
			name:           "offsets with leading delimiter",
			input:          ",Hello",
			pattern:        `,`,
			expectedTerms:  []string{"Hello"},
			expectedStarts: []int{1},
			expectedEnds:   []int{6},
		},
		{
			name:           "offsets with multi-char delimiter",
			input:          "Hello---World",
			pattern:        `-+`,
			expectedTerms:  []string{"Hello", "World"},
			expectedStarts: []int{0, 8},
			expectedEnds:   []int{5, 13},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer, err := NewSimplePatternSplitTokenizerWithString(tt.pattern)
			if err != nil {
				t.Fatalf("Failed to create tokenizer: %v", err)
			}

			err = tokenizer.SetReader(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("SetReader failed: %v", err)
			}

			var tokens []struct {
				term  string
				start int
				end   int
			}

			for {
				hasToken, err := tokenizer.IncrementToken()
				if err != nil {
					t.Fatalf("IncrementToken failed: %v", err)
				}
				if !hasToken {
					break
				}

				termAttr := tokenizer.GetAttributeSource().GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
				offsetAttr := tokenizer.GetAttributeSource().GetAttributeByType(reflect.TypeOf(&offsetAttribute{}))

				cta, ok1 := termAttr.(CharTermAttribute)
				oa, ok2 := offsetAttr.(OffsetAttribute)
				if !ok1 || !ok2 {
					t.Fatalf("Failed to cast attributes")
				}

				tokens = append(tokens, struct {
					term  string
					start int
					end   int
				}{
					term:  cta.String(),
					start: oa.StartOffset(),
					end:   oa.EndOffset(),
				})
			}

			if len(tokens) != len(tt.expectedTerms) {
				t.Errorf("Expected %d tokens, got %d", len(tt.expectedTerms), len(tokens))
				return
			}

			for i := range tokens {
				if tokens[i].term != tt.expectedTerms[i] {
					t.Errorf("Token %d: expected term %q, got %q", i, tt.expectedTerms[i], tokens[i].term)
				}
				if tokens[i].start != tt.expectedStarts[i] {
					t.Errorf("Token %d: expected start offset %d, got %d", i, tt.expectedStarts[i], tokens[i].start)
				}
				if tokens[i].end != tt.expectedEnds[i] {
					t.Errorf("Token %d: expected end offset %d, got %d", i, tt.expectedEnds[i], tokens[i].end)
				}
			}
		})
	}
}

func TestSimplePatternSplitTokenizer_PositionIncrement(t *testing.T) {
	tokenizer, err := NewSimplePatternSplitTokenizerWithString(`\s+`)
	if err != nil {
		t.Fatalf("Failed to create tokenizer: %v", err)
	}

	err = tokenizer.SetReader(strings.NewReader("Hello World Test"))
	if err != nil {
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
		pia, ok := posIncrAttr.(PositionIncrementAttribute)
		if !ok {
			t.Fatalf("Failed to cast PositionIncrementAttribute")
		}

		if pia.GetPositionIncrement() != 1 {
			t.Errorf("Token %d: expected position increment 1, got %d", i, pia.GetPositionIncrement())
		}
	}
}

func TestSimplePatternSplitTokenizer_Reset(t *testing.T) {
	tokenizer, err := NewSimplePatternSplitTokenizerWithString(`\s+`)
	if err != nil {
		t.Fatalf("Failed to create tokenizer: %v", err)
	}

	// First run
	err = tokenizer.SetReader(strings.NewReader("Hello World"))
	if err != nil {
		t.Fatalf("SetReader failed: %v", err)
	}

	var tokens1 []string
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

	if len(tokens1) != 2 || tokens1[0] != "Hello" || tokens1[1] != "World" {
		t.Errorf("First run: expected [Hello World], got %v", tokens1)
	}

	// Reset and second run
	err = tokenizer.Reset()
	if err != nil {
		t.Fatalf("Reset failed: %v", err)
	}

	err = tokenizer.SetReader(strings.NewReader("Foo Bar"))
	if err != nil {
		t.Fatalf("SetReader failed: %v", err)
	}

	var tokens2 []string
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

	if len(tokens2) != 2 || tokens2[0] != "Foo" || tokens2[1] != "Bar" {
		t.Errorf("Second run: expected [Foo Bar], got %v", tokens2)
	}
}

func TestSimplePatternSplitTokenizer_End(t *testing.T) {
	tokenizer, err := NewSimplePatternSplitTokenizerWithString(`\s+`)
	if err != nil {
		t.Fatalf("Failed to create tokenizer: %v", err)
	}

	input := "Hello World"
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

	offsetAttr := tokenizer.GetAttributeSource().GetAttributeByType(reflect.TypeOf(&offsetAttribute{}))
	oa, ok := offsetAttr.(OffsetAttribute)
	if !ok {
		t.Fatalf("Failed to cast OffsetAttribute")
	}

	if oa.EndOffset() != len(input) {
		t.Errorf("Expected end offset %d, got %d", len(input), oa.EndOffset())
	}
}

func TestSimplePatternSplitTokenizer_NilPattern(t *testing.T) {
	_, err := NewSimplePatternSplitTokenizer(nil)
	if err != ErrNilPattern {
		t.Errorf("Expected ErrNilPattern, got %v", err)
	}
}

func TestSimplePatternSplitTokenizer_InvalidPattern(t *testing.T) {
	_, err := NewSimplePatternSplitTokenizerWithString(`[invalid`)
	if err == nil {
		t.Error("Expected error for invalid pattern")
	}
}

func TestSimplePatternSplitTokenizer_GetPattern(t *testing.T) {
	pattern := regexp.MustCompile(`\s+`)
	tokenizer, err := NewSimplePatternSplitTokenizer(pattern)
	if err != nil {
		t.Fatalf("Failed to create tokenizer: %v", err)
	}

	if tokenizer.GetPattern() != pattern {
		t.Error("GetPattern returned different pattern")
	}
}

func TestSimplePatternSplitTokenizer_ComplexPatterns(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		pattern  string
		expected []string
	}{
		{
			name:     "split on digits",
			input:    "abc123def456ghi",
			pattern:  `\d+`,
			expected: []string{"abc", "def", "ghi"},
		},
		{
			name:     "split on non-word",
			input:    "hello@world#test",
			pattern:  `\W+`,
			expected: []string{"hello", "world", "test"},
		},
		{
			name:     "split on uppercase",
			input:    "HelloWorldTest",
			pattern:  `[A-Z]`,
			expected: []string{"ello", "orld", "est"},
		},
		{
			name:     "split on URL-like pattern",
			input:    "http://example.com/path",
			pattern:  `://`,
			expected: []string{"http", "example.com/path"},
		},
		{
			name:     "split on multiple newlines",
			input:    "para1\n\n\npara2",
			pattern:  `\n+`,
			expected: []string{"para1", "para2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer, err := NewSimplePatternSplitTokenizerWithString(tt.pattern)
			if err != nil {
				t.Fatalf("Failed to create tokenizer: %v", err)
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

func TestSimplePatternSplitTokenizer_Close(t *testing.T) {
	tokenizer, err := NewSimplePatternSplitTokenizerWithString(`\s+`)
	if err != nil {
		t.Fatalf("Failed to create tokenizer: %v", err)
	}

	err = tokenizer.SetReader(strings.NewReader("Hello World"))
	if err != nil {
		t.Fatalf("SetReader failed: %v", err)
	}

	// Consume a token
	hasToken, _ := tokenizer.IncrementToken()
	if !hasToken {
		t.Error("Expected token")
	}

	// Close the tokenizer
	err = tokenizer.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// After close, the tokenizer should be in a clean state
	// Reset should work to prepare for new input
	err = tokenizer.Reset()
	if err != nil {
		t.Fatalf("Reset after close failed: %v", err)
	}
}
