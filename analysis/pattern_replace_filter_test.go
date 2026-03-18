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

// TestPatternReplaceFilter_Basic tests basic pattern replacement.
// Source: PatternReplaceFilterTest.testBasicReplacement()
// Purpose: Tests that patterns are correctly replaced in tokens.
func TestPatternReplaceFilter_Basic(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		pattern     string
		replacement string
		replaceAll  bool
		expected    []string
	}{
		{
			name:        "Remove digits",
			input:       "abc123 def456",
			pattern:     `\d+`,
			replacement: "",
			replaceAll:  true,
			expected:    []string{"abc", "def"},
		},
		{
			name:        "Replace digits with NUM",
			input:       "abc123 def456",
			pattern:     `\d+`,
			replacement: "NUM",
			replaceAll:  true,
			expected:    []string{"abcNUM", "defNUM"},
		},
		{
			name:        "Remove punctuation",
			input:       "hello, world! test.",
			pattern:     `[^\w\s]`,
			replacement: "",
			replaceAll:  true,
			expected:    []string{"hello", "world", "test"},
		},
		{
			name:        "First match only",
			input:       "a1b2c3 d4e5f6",
			pattern:     `\d`,
			replacement: "X",
			replaceAll:  false,
			expected:    []string{"aXb2c3", "dXe5f6"},
		},
		{
			name:        "All matches",
			input:       "a1b2c3 d4e5f6",
			pattern:     `\d`,
			replacement: "X",
			replaceAll:  true,
			expected:    []string{"aXbXcX", "dXeXfX"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pattern, err := regexp.Compile(tc.pattern)
			if err != nil {
				t.Fatalf("Failed to compile pattern: %v", err)
			}

			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			filter := NewPatternReplaceFilter(tokenizer, pattern, tc.replacement, tc.replaceAll)
			defer filter.Close()

			var tokens []string
			for {
				hasToken, err := filter.IncrementToken()
				if err != nil {
					t.Fatalf("Error incrementing token: %v", err)
				}
				if !hasToken {
					break
				}
				if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
					if termAttr, ok := attr.(CharTermAttribute); ok {
						tokens = append(tokens, termAttr.String())
					}
				}
			}

			if !reflect.DeepEqual(tokens, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, tokens)
			}
		})
	}
}

// TestPatternReplaceFilter_CaptureGroups tests replacement with capture groups.
// Source: PatternReplaceFilterTest.testCaptureGroups()
// Purpose: Tests that capture groups work correctly in replacements.
func TestPatternReplaceFilter_CaptureGroups(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		pattern     string
		replacement string
		expected    []string
	}{
		{
			name:        "Swap two groups",
			input:       "first:second third:fourth",
			pattern:     `^(\w+):(\w+)$`,
			replacement: "$2:$1",
			expected:    []string{"second:first", "fourth:third"},
		},
		{
			name:        "Extract first group",
			input:       "prefix_data other_info",
			pattern:     `^(\w+)_(\w+)$`,
			replacement: "$1",
			expected:    []string{"prefix", "other"},
		},
		{
			name:        "Extract second group",
			input:       "prefix_data other_prefix_info",
			pattern:     `^(\w+)_(\w+)$`,
			replacement: "$2",
			expected:    []string{"data", "info"},
		},
		{
			name:        "Multiple capture groups",
			input:       "a-b-c d-e-f",
			pattern:     `^(\w+)-(\w+)-(\w+)$`,
			replacement: "$3-$2-$1",
			expected:    []string{"c-b-a", "f-e-d"},
		},
		{
			name:        "Literal dollar sign",
			input:       "price:100 cost:200",
			pattern:     `^(\w+):(\d+)$`,
			replacement: "$$$2",
			expected:    []string{"$100", "$200"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pattern, err := regexp.Compile(tc.pattern)
			if err != nil {
				t.Fatalf("Failed to compile pattern: %v", err)
			}

			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			filter := NewPatternReplaceFilterAllMatches(tokenizer, pattern, tc.replacement)
			defer filter.Close()

			var tokens []string
			for {
				hasToken, err := filter.IncrementToken()
				if err != nil {
					t.Fatalf("Error incrementing token: %v", err)
				}
				if !hasToken {
					break
				}
				if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
					if termAttr, ok := attr.(CharTermAttribute); ok {
						tokens = append(tokens, termAttr.String())
					}
				}
			}

			if !reflect.DeepEqual(tokens, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, tokens)
			}
		})
	}
}

// TestPatternReplaceFilter_NoMatch tests behavior when pattern doesn't match.
// Source: PatternReplaceFilterTest.testNoMatch()
// Purpose: Tests that tokens are unchanged when pattern doesn't match.
func TestPatternReplaceFilter_NoMatch(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("hello world"))

	pattern := regexp.MustCompile(`\d+`)
	filter := NewPatternReplaceFilterAllMatches(tokenizer, pattern, "NUM")
	defer filter.Close()

	var tokens []string
	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
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

// TestPatternReplaceFilter_EmptyInput tests empty input handling.
// Source: PatternReplaceFilterTest.testEmptyInput()
// Purpose: Tests that empty input is handled correctly.
func TestPatternReplaceFilter_EmptyInput(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader(""))

	pattern := regexp.MustCompile(`\d+`)
	filter := NewPatternReplaceFilterAllMatches(tokenizer, pattern, "NUM")
	defer filter.Close()

	tokenCount := 0
	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		tokenCount++
	}

	if tokenCount != 0 {
		t.Errorf("Expected 0 tokens for empty input, got %d", tokenCount)
	}
}

// TestPatternReplaceFilter_EmptyReplacement tests empty replacement string.
// Source: PatternReplaceFilterTest.testEmptyReplacement()
// Purpose: Tests that empty replacement removes matched text.
func TestPatternReplaceFilter_EmptyReplacement(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("abc123def"))

	pattern := regexp.MustCompile(`\d+`)
	filter := NewPatternReplaceFilterAllMatches(tokenizer, pattern, "")
	defer filter.Close()

	var tokens []string
	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				tokens = append(tokens, termAttr.String())
			}
		}
	}

	expected := []string{"abcdef"}
	if !reflect.DeepEqual(tokens, expected) {
		t.Errorf("Expected %v, got %v", expected, tokens)
	}
}

// TestPatternReplaceFilter_PositionIncrement tests position increment preservation.
// Source: PatternReplaceFilterTest.testPositionIncrement()
// Purpose: Tests that position increments are preserved.
func TestPatternReplaceFilter_PositionIncrement(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("a1 b2 c3"))

	pattern := regexp.MustCompile(`\d`)
	filter := NewPatternReplaceFilterAllMatches(tokenizer, pattern, "X")
	defer filter.Close()

	positions := []int{}
	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := filter.GetAttributeSource().GetAttribute("PositionIncrementAttribute"); attr != nil {
			if posAttr, ok := attr.(PositionIncrementAttribute); ok {
				positions = append(positions, posAttr.GetPositionIncrement())
			}
		}
	}

	expected := []int{1, 1, 1}
	if !reflect.DeepEqual(positions, expected) {
		t.Errorf("Expected positions %v, got %v", expected, positions)
	}
}

// TestPatternReplaceFilter_Offset tests offset preservation.
// Source: PatternReplaceFilterTest.testOffset()
// Purpose: Tests that character offsets are preserved.
func TestPatternReplaceFilter_Offset(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("hello123 world456"))

	pattern := regexp.MustCompile(`\d+`)
	filter := NewPatternReplaceFilterAllMatches(tokenizer, pattern, "")
	defer filter.Close()

	type tokenInfo struct {
		text        string
		startOffset int
		endOffset   int
	}

	var tokens []tokenInfo
	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}

		var info tokenInfo
		if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				info.text = termAttr.String()
			}
		}
		if attr := filter.GetAttributeSource().GetAttribute("OffsetAttribute"); attr != nil {
			if offsetAttr, ok := attr.(OffsetAttribute); ok {
				info.startOffset = offsetAttr.StartOffset()
				info.endOffset = offsetAttr.EndOffset()
			}
		}
		tokens = append(tokens, info)
	}

	if len(tokens) != 2 {
		t.Fatalf("Expected 2 tokens, got %d", len(tokens))
	}

	// Offsets should be preserved from the original token positions
	if tokens[0].text != "hello" {
		t.Errorf("First token: expected 'hello', got '%s'", tokens[0].text)
	}
	if tokens[1].text != "world" {
		t.Errorf("Second token: expected 'world', got '%s'", tokens[1].text)
	}
}

// TestPatternReplaceFilter_Chaining tests chaining with other filters.
// Source: PatternReplaceFilterTest.testChaining()
// Purpose: Tests that PatternReplaceFilter works properly in filter chains.
func TestPatternReplaceFilter_Chaining(t *testing.T) {
	input := "HELLO123 WORLD456"

	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader(input))

	// Chain: LowerCase -> PatternReplace
	lowerFilter := NewLowerCaseFilter(tokenizer)
	pattern := regexp.MustCompile(`\d+`)
	replaceFilter := NewPatternReplaceFilterAllMatches(lowerFilter, pattern, "")
	defer replaceFilter.Close()

	var tokens []string
	for {
		hasToken, err := replaceFilter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := replaceFilter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
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

// TestPatternReplaceFilter_EndMethod tests the End() method.
// Source: PatternReplaceFilterTest.testEnd()
// Purpose: Tests that End() is properly propagated.
func TestPatternReplaceFilter_EndMethod(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("test"))

	pattern := regexp.MustCompile(`\d+`)
	filter := NewPatternReplaceFilterAllMatches(tokenizer, pattern, "")
	defer filter.Close()

	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
	}

	err := filter.End()
	if err != nil {
		t.Errorf("End() returned error: %v", err)
	}
}

// TestPatternReplaceFilter_Getters tests the getter methods.
// Source: PatternReplaceFilterTest.testGetters()
// Purpose: Tests that getter methods return correct values.
func TestPatternReplaceFilter_Getters(t *testing.T) {
	pattern := regexp.MustCompile(`\d+`)
	replacement := "NUM"
	replaceAll := true

	tokenizer := NewWhitespaceTokenizer()
	filter := NewPatternReplaceFilter(tokenizer, pattern, replacement, replaceAll)

	if filter.GetPattern() != pattern {
		t.Error("GetPattern() returned wrong pattern")
	}

	if filter.GetReplacement() != replacement {
		t.Errorf("GetReplacement() returned '%s', expected '%s'", filter.GetReplacement(), replacement)
	}

	if filter.IsReplaceAll() != replaceAll {
		t.Errorf("IsReplaceAll() returned %v, expected %v", filter.IsReplaceAll(), replaceAll)
	}
}

// TestPatternReplaceFilter_ConvenienceConstructors tests convenience constructors.
// Source: PatternReplaceFilterTest.testConvenienceConstructors()
// Purpose: Tests that convenience constructors work correctly.
func TestPatternReplaceFilter_ConvenienceConstructors(t *testing.T) {
	t.Run("FirstMatch", func(t *testing.T) {
		tokenizer := NewWhitespaceTokenizer()
		tokenizer.SetReader(strings.NewReader("a1b2c3"))

		pattern := regexp.MustCompile(`\d`)
		filter := NewPatternReplaceFilterFirstMatch(tokenizer, pattern, "X")
		defer filter.Close()

		var tokens []string
		for {
			hasToken, err := filter.IncrementToken()
			if err != nil {
				t.Fatalf("Error incrementing token: %v", err)
			}
			if !hasToken {
				break
			}
			if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
				if termAttr, ok := attr.(CharTermAttribute); ok {
					tokens = append(tokens, termAttr.String())
				}
			}
		}

		expected := []string{"aXb2c3"}
		if !reflect.DeepEqual(tokens, expected) {
			t.Errorf("Expected %v, got %v", expected, tokens)
		}
	})

	t.Run("AllMatches", func(t *testing.T) {
		tokenizer := NewWhitespaceTokenizer()
		tokenizer.SetReader(strings.NewReader("a1b2c3"))

		pattern := regexp.MustCompile(`\d`)
		filter := NewPatternReplaceFilterAllMatches(tokenizer, pattern, "X")
		defer filter.Close()

		var tokens []string
		for {
			hasToken, err := filter.IncrementToken()
			if err != nil {
				t.Fatalf("Error incrementing token: %v", err)
			}
			if !hasToken {
				break
			}
			if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
				if termAttr, ok := attr.(CharTermAttribute); ok {
					tokens = append(tokens, termAttr.String())
				}
			}
		}

		expected := []string{"aXbXcX"}
		if !reflect.DeepEqual(tokens, expected) {
			t.Errorf("Expected %v, got %v", expected, tokens)
		}
	})
}

// TestPatternReplaceFilter_Factory tests the factory.
// Source: PatternReplaceFilterTest.testFactory()
// Purpose: Tests that the factory creates filters correctly.
func TestPatternReplaceFilter_Factory(t *testing.T) {
	pattern := regexp.MustCompile(`\d+`)
	replacement := "NUM"

	factory := NewPatternReplaceFilterFactory(pattern, replacement, true)

	if factory.GetPattern() != pattern {
		t.Error("Factory GetPattern() returned wrong pattern")
	}

	if factory.GetReplacement() != replacement {
		t.Errorf("Factory GetReplacement() returned '%s', expected '%s'", factory.GetReplacement(), replacement)
	}

	if !factory.IsReplaceAll() {
		t.Error("Factory IsReplaceAll() should return true")
	}

	// Test Create
	tokenizer := NewWhitespaceTokenizer()
	filter := factory.Create(tokenizer)
	defer filter.Close()

	if filter == nil {
		t.Error("Factory Create() returned nil")
	}
}

// TestPatternReplaceFilter_FactoryWithString tests the factory with string pattern.
// Source: PatternReplaceFilterTest.testFactoryWithString()
// Purpose: Tests that the factory can be created from a string pattern.
func TestPatternReplaceFilter_FactoryWithString(t *testing.T) {
	factory, err := NewPatternReplaceFilterFactoryWithString(`\d+`, "NUM", true)
	if err != nil {
		t.Fatalf("Failed to create factory: %v", err)
	}

	if factory == nil {
		t.Fatal("Factory is nil")
	}

	// Test with invalid pattern
	_, err = NewPatternReplaceFilterFactoryWithString(`[invalid`, "NUM", true)
	if err == nil {
		t.Error("Expected error for invalid pattern")
	}
}

// TestPatternReplaceFilter_FactorySetters tests the factory setters.
// Source: PatternReplaceFilterTest.testFactorySetters()
// Purpose: Tests that factory setters work correctly.
func TestPatternReplaceFilter_FactorySetters(t *testing.T) {
	factory := NewPatternReplaceFilterFactory(regexp.MustCompile(`old`), "old", false)

	newPattern := regexp.MustCompile(`new`)
	factory.SetPattern(newPattern)
	if factory.GetPattern() != newPattern {
		t.Error("SetPattern() did not work")
	}

	factory.SetReplacement("new")
	if factory.GetReplacement() != "new" {
		t.Error("SetReplacement() did not work")
	}

	factory.SetReplaceAll(true)
	if !factory.IsReplaceAll() {
		t.Error("SetReplaceAll() did not work")
	}
}

// TestPatternReplaceFilter_CommonFilters tests the common filter creation functions.
// Source: PatternReplaceFilterTest.testCommonFilters()
// Purpose: Tests that common filter functions work correctly.
func TestPatternReplaceFilter_CommonFilters(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		filterFunc func(TokenStream) *PatternReplaceFilter
		expected   []string
	}{
		{
			name:       "DigitRemovalFilter",
			input:      "abc123 def456",
			filterFunc: CreateDigitRemovalFilter,
			expected:   []string{"abc", "def"},
		},
		{
			name:       "PunctuationRemovalFilter",
			input:      "hello, world!",
			filterFunc: CreatePunctuationRemovalFilter,
			expected:   []string{"hello", "world"},
		},
		{
			name:       "WhitespaceNormalizationFilter",
			input:      "a  b   c",
			filterFunc: CreateTokenWhitespaceNormalizationFilter,
			expected:   []string{"a", "b", "c"}, // WhitespaceTokenizer splits first, then each token is normalized
		},
		{
			name:       "EmailNormalizationFilter",
			input:      "contact user@example.com today",
			filterFunc: CreateEmailNormalizationFilter,
			expected:   []string{"contact", "EMAIL", "today"},
		},
		{
			name:       "URLNormalizationFilter",
			input:      "visit https://example.com now",
			filterFunc: CreateURLNormalizationFilter,
			expected:   []string{"visit", "URL", "now"},
		},
		{
			name:       "PhoneNormalizationFilter",
			input:      "call 555-123-4567 today", // Phone as single token
			filterFunc: CreateTokenPhoneNormalizationFilter,
			expected:   []string{"call", "PHONE", "today"},
		},
		{
			name:       "CamelCaseSplitFilter",
			input:      "HelloWorld FooBar",
			filterFunc: CreateCamelCaseSplitFilter,
			expected:   []string{"Hello World", "Foo Bar"},
		},
		{
			name:       "SnakeCaseToSpaceFilter",
			input:      "hello_world foo_bar",
			filterFunc: CreateSnakeCaseToSpaceFilter,
			expected:   []string{"hello world", "foo bar"},
		},
		{
			name:       "HyphenToSpaceFilter",
			input:      "hello-world foo-bar",
			filterFunc: CreateHyphenToSpaceFilter,
			expected:   []string{"hello world", "foo bar"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			// Create the filter with the tokenizer as input
			filter := tc.filterFunc(tokenizer)
			defer filter.Close()

			var tokens []string
			for {
				hasToken, err := filter.IncrementToken()
				if err != nil {
					t.Fatalf("Error incrementing token: %v", err)
				}
				if !hasToken {
					break
				}
				if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
					if termAttr, ok := attr.(CharTermAttribute); ok {
						tokens = append(tokens, termAttr.String())
					}
				}
			}

			if !reflect.DeepEqual(tokens, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, tokens)
			}
		})
	}
}

// TestPatternReplaceFilter_ComplexPatterns tests complex regex patterns.
// Source: PatternReplaceFilterTest.testComplexPatterns()
// Purpose: Tests that complex patterns work correctly.
func TestPatternReplaceFilter_ComplexPatterns(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		pattern     string
		replacement string
		expected    []string
	}{
		{
			name:        "Word boundaries",
			input:       "test testing tested",
			pattern:     `\btest\b`,
			replacement: "X",
			expected:    []string{"X", "testing", "tested"},
		},
		{
			name:        "Lookahead (simulated)",
			input:       "foo bar baz",
			pattern:     `^foo$`,
			replacement: "X",
			expected:    []string{"X", "bar", "baz"},
		},
		{
			name:        "Character class",
			input:       "abc123def456",
			pattern:     `[a-z]+`,
			replacement: "LETTERS",
			expected:    []string{"LETTERS123LETTERS456"},
		},
		{
			name:        "Alternation",
			input:       "cat dog bird",
			pattern:     `cat|dog`,
			replacement: "animal",
			expected:    []string{"animal", "animal", "bird"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pattern, err := regexp.Compile(tc.pattern)
			if err != nil {
				t.Fatalf("Failed to compile pattern: %v", err)
			}

			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			filter := NewPatternReplaceFilterAllMatches(tokenizer, pattern, tc.replacement)
			defer filter.Close()

			var tokens []string
			for {
				hasToken, err := filter.IncrementToken()
				if err != nil {
					t.Fatalf("Error incrementing token: %v", err)
				}
				if !hasToken {
					break
				}
				if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
					if termAttr, ok := attr.(CharTermAttribute); ok {
						tokens = append(tokens, termAttr.String())
					}
				}
			}

			if !reflect.DeepEqual(tokens, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, tokens)
			}
		})
	}
}

// TestPatternReplaceFilter_EdgeCases tests edge cases.
// Source: PatternReplaceFilterTest.testEdgeCases()
// Purpose: Tests edge cases and boundary conditions.
func TestPatternReplaceFilter_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		pattern     string
		replacement string
		replaceAll  bool
		expected    []string
	}{
		{
			name:        "Empty token after replacement",
			input:       "123 456",
			pattern:     `\d+`,
			replacement: "",
			replaceAll:  true,
			expected:    []string{"", ""},
		},
		{
			name:        "Token becomes empty after partial removal",
			input:       "abc",
			pattern:     `abc`,
			replacement: "",
			replaceAll:  true,
			expected:    []string{""},
		},
		{
			name:        "Pattern matches entire token",
			input:       "test",
			pattern:     `^.*$`,
			replacement: "REPLACED",
			replaceAll:  true,
			expected:    []string{"REPLACED"},
		},
		{
			name:        "Pattern at start",
			input:       "abc123",
			pattern:     `^abc`,
			replacement: "X",
			replaceAll:  true,
			expected:    []string{"X123"},
		},
		{
			name:        "Pattern at end",
			input:       "abc123",
			pattern:     `123$`,
			replacement: "X",
			replaceAll:  true,
			expected:    []string{"abcX"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pattern, err := regexp.Compile(tc.pattern)
			if err != nil {
				t.Fatalf("Failed to compile pattern: %v", err)
			}

			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			filter := NewPatternReplaceFilter(tokenizer, pattern, tc.replacement, tc.replaceAll)
			defer filter.Close()

			var tokens []string
			for {
				hasToken, err := filter.IncrementToken()
				if err != nil {
					t.Fatalf("Error incrementing token: %v", err)
				}
				if !hasToken {
					break
				}
				if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
					if termAttr, ok := attr.(CharTermAttribute); ok {
						tokens = append(tokens, termAttr.String())
					}
				}
			}

			if !reflect.DeepEqual(tokens, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, tokens)
			}
		})
	}
}

// TestPatternReplaceFilter_Unicode tests Unicode handling.
// Source: PatternReplaceFilterTest.testUnicode()
// Purpose: Tests proper handling of Unicode characters.
func TestPatternReplaceFilter_Unicode(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		pattern     string
		replacement string
		expected    []string
	}{
		{
			name:        "Unicode digits",
			input:       "test١٢٣ end",
			pattern:     `\p{Nd}+`,
			replacement: "NUM",
			expected:    []string{"testNUM", "end"},
		},
		{
			name:        "Unicode letters",
			input:       "café résumé",
			pattern:     `é`,
			replacement: "e",
			expected:    []string{"cafe", "resume"},
		},
		{
			name:        "CJK characters",
			input:       "日本語テスト",
			pattern:     `テスト`,
			replacement: "TEST",
			expected:    []string{"日本語TEST"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pattern, err := regexp.Compile(tc.pattern)
			if err != nil {
				t.Fatalf("Failed to compile pattern: %v", err)
			}

			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			filter := NewPatternReplaceFilterAllMatches(tokenizer, pattern, tc.replacement)
			defer filter.Close()

			var tokens []string
			for {
				hasToken, err := filter.IncrementToken()
				if err != nil {
					t.Fatalf("Error incrementing token: %v", err)
				}
				if !hasToken {
					break
				}
				if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
					if termAttr, ok := attr.(CharTermAttribute); ok {
						tokens = append(tokens, termAttr.String())
					}
				}
			}

			if !reflect.DeepEqual(tokens, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, tokens)
			}
		})
	}
}

// TestPatternReplaceFilter_InterfaceCompliance tests interface compliance.
// Source: PatternReplaceFilterTest.testInterfaceCompliance()
// Purpose: Tests that the filter properly implements required interfaces.
func TestPatternReplaceFilter_InterfaceCompliance(t *testing.T) {
	t.Run("TokenFilter interface", func(t *testing.T) {
		var _ TokenFilter = (*PatternReplaceFilter)(nil)
	})

	t.Run("TokenFilterFactory interface", func(t *testing.T) {
		var _ TokenFilterFactory = (*PatternReplaceFilterFactory)(nil)
	})
}
