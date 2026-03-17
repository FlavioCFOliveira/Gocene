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

// TestPatternTokenizer_SplitMode tests the tokenizer in split mode (default).
func TestPatternTokenizer_SplitMode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		pattern  string
		expected []string
	}{
		{
			name:     "simple whitespace split",
			input:    "hello world test",
			pattern:  `\s+`,
			expected: []string{"hello", "world", "test"},
		},
		{
			name:     "comma delimiter",
			input:    "a,b,c,d",
			pattern:  `,`,
			expected: []string{"a", "b", "c", "d"},
		},
		{
			name:     "multiple spaces",
			input:    "hello    world",
			pattern:  `\s+`,
			expected: []string{"hello", "world"},
		},
		{
			name:     "leading delimiter",
			input:    "  hello world",
			pattern:  `\s+`,
			expected: []string{"hello", "world"},
		},
		{
			name:     "trailing delimiter",
			input:    "hello world  ",
			pattern:  `\s+`,
			expected: []string{"hello", "world"},
		},
		{
			name:     "empty input",
			input:    "",
			pattern:  `\s+`,
			expected: []string{},
		},
		{
			name:     "only delimiters",
			input:    "   ",
			pattern:  `\s+`,
			expected: []string{},
		},
		{
			name:     "no match",
			input:    "helloworld",
			pattern:  `\s+`,
			expected: []string{"helloworld"},
		},
		{
			name:     "complex pattern",
			input:    "foo123bar456baz",
			pattern:  `\d+`,
			expected: []string{"foo", "bar", "baz"},
		},
		{
			name:     "tab and newline",
			input:    "hello\tworld\ntest",
			pattern:  `\s+`,
			expected: []string{"hello", "world", "test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			re := regexp.MustCompile(tt.pattern)
			tokenizer := NewPatternTokenizer(re)

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
				tokens = append(tokens, termAttr.(CharTermAttribute).String())
			}

			if len(tokens) != len(tt.expected) {
				t.Errorf("Got %d tokens, expected %d: got %v, want %v", len(tokens), len(tt.expected), tokens, tt.expected)
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

// TestPatternTokenizer_MatchMode tests the tokenizer in match mode.
func TestPatternTokenizer_MatchMode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		pattern  string
		group    int
		expected []string
	}{
		{
			name:     "extract words",
			input:    "hello, world! test.",
			pattern:  `\b\w+\b`,
			group:    0,
			expected: []string{"hello", "world", "test"},
		},
		{
			name:     "extract numbers",
			input:    "foo123bar456baz789",
			pattern:  `\d+`,
			group:    0,
			expected: []string{"123", "456", "789"},
		},
		{
			name:     "extract email-like patterns",
			input:    "Contact us at test@example.com or support@company.org",
			pattern:  `\w+@\w+\.\w+`,
			group:    0,
			expected: []string{"test@example.com", "support@company.org"},
		},
		{
			name:     "capturing group - extract from tags",
			input:    "<tag1> <tag2> <tag3>",
			pattern:  `<(\w+)>`,
			group:    1,
			expected: []string{"tag1", "tag2", "tag3"},
		},
		{
			name:     "capturing group - extract domain from email",
			input:    "user1@gmail.com user2@yahoo.com",
			pattern:  `\w+@(\w+\.\w+)`,
			group:    1,
			expected: []string{"gmail.com", "yahoo.com"},
		},
		{
			name:     "no matches",
			input:    "hello world",
			pattern:  `\d+`,
			group:    0,
			expected: []string{},
		},
		{
			name:     "empty input",
			input:    "",
			pattern:  `\w+`,
			group:    0,
			expected: []string{},
		},
		{
			name:     "single match",
			input:    "hello",
			pattern:  `\w+`,
			group:    0,
			expected: []string{"hello"},
		},
		{
			name:     "multiple capturing groups - use first",
			input:    "a1b a2b a3b",
			pattern:  `a(\d)(b)`,
			group:    1,
			expected: []string{"1", "2", "3"},
		},
		{
			name:     "multiple capturing groups - use second",
			input:    "a1b a2b a3b",
			pattern:  `a(\d)(b)`,
			group:    2,
			expected: []string{"b", "b", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			re := regexp.MustCompile(tt.pattern)
			tokenizer := NewPatternTokenizerWithGroup(re, tt.group)

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
				tokens = append(tokens, termAttr.(CharTermAttribute).String())
			}

			if len(tokens) != len(tt.expected) {
				t.Errorf("Got %d tokens, expected %d: got %v, want %v", len(tokens), len(tt.expected), tokens, tt.expected)
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

// TestPatternTokenizer_Offsets tests that offsets are correctly set.
func TestPatternTokenizer_Offsets(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		pattern       string
		group         int
		expectedStart []int
		expectedEnd   []int
	}{
		{
			name:          "split mode offsets",
			input:         "hello world",
			pattern:       `\s+`,
			group:         -1,
			expectedStart: []int{0, 6},
			expectedEnd:   []int{5, 11},
		},
		{
			name:          "match mode offsets",
			input:         "hello, world!",
			pattern:       `\b\w+\b`,
			group:         0,
			expectedStart: []int{0, 7},
			expectedEnd:   []int{5, 12},
		},
		{
			name:          "capturing group offsets",
			input:         "<tag>content</tag>",
			pattern:       `<(/?\w+)>`,
			group:         1,
			expectedStart: []int{1, 13},
			expectedEnd:   []int{4, 17},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			re := regexp.MustCompile(tt.pattern)
			tokenizer := NewPatternTokenizerWithGroup(re, tt.group)

			if err := tokenizer.SetReader(strings.NewReader(tt.input)); err != nil {
				t.Fatalf("SetReader failed: %v", err)
			}

			var startOffsets []int
			var endOffsets []int

			for {
				hasToken, err := tokenizer.IncrementToken()
				if err != nil {
					t.Fatalf("IncrementToken failed: %v", err)
				}
				if !hasToken {
					break
				}

				offsetAttr := tokenizer.GetAttributeSource().GetAttributeByType(reflect.TypeOf(&offsetAttribute{}))
				if offsetAttr == nil {
					t.Fatal("OffsetAttribute is nil")
				}

				oa := offsetAttr.(OffsetAttribute)
				startOffsets = append(startOffsets, oa.StartOffset())
				endOffsets = append(endOffsets, oa.EndOffset())
			}

			if len(startOffsets) != len(tt.expectedStart) {
				t.Errorf("Got %d tokens, expected %d", len(startOffsets), len(tt.expectedStart))
				return
			}

			for i := range tt.expectedStart {
				if startOffsets[i] != tt.expectedStart[i] {
					t.Errorf("Token %d start offset = %d, want %d", i, startOffsets[i], tt.expectedStart[i])
				}
				if endOffsets[i] != tt.expectedEnd[i] {
					t.Errorf("Token %d end offset = %d, want %d", i, endOffsets[i], tt.expectedEnd[i])
				}
			}
		})
	}
}

// TestPatternTokenizer_TypeAttribute tests that type attribute is set.
func TestPatternTokenizer_TypeAttribute(t *testing.T) {
	input := "hello world"
	re := regexp.MustCompile(`\s+`)
	tokenizer := NewPatternTokenizer(re)

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

	typeAttr := tokenizer.GetAttributeSource().GetAttributeByType(reflect.TypeOf(&TypeAttribute{}))
	if typeAttr == nil {
		t.Fatal("TypeAttribute is nil")
	}

	ta := typeAttr.(*TypeAttribute)
	if ta.GetType() != "word" {
		t.Errorf("Type = %q, want 'word'", ta.GetType())
	}
}

// TestPatternTokenizer_PositionIncrement tests position increment attribute.
func TestPatternTokenizer_PositionIncrement(t *testing.T) {
	input := "hello world test"
	re := regexp.MustCompile(`\s+`)
	tokenizer := NewPatternTokenizer(re)

	if err := tokenizer.SetReader(strings.NewReader(input)); err != nil {
		t.Fatalf("SetReader failed: %v", err)
	}

	tokenCount := 0
	for {
		hasToken, err := tokenizer.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken failed: %v", err)
		}
		if !hasToken {
			break
		}

		posIncrAttr := tokenizer.GetAttributeSource().GetAttributeByType(reflect.TypeOf(&positionIncrementAttribute{}))
		if posIncrAttr == nil {
			t.Fatal("PositionIncrementAttribute is nil")
		}

		pi := posIncrAttr.(PositionIncrementAttribute)
		if pi.GetPositionIncrement() != 1 {
			t.Errorf("Token %d: position increment = %d, want 1", tokenCount, pi.GetPositionIncrement())
		}
		tokenCount++
	}
}

// TestPatternTokenizer_Reset tests reset functionality.
func TestPatternTokenizer_Reset(t *testing.T) {
	tokenizer := NewPatternTokenizer(regexp.MustCompile(`\s+`))

	// First input
	if err := tokenizer.SetReader(strings.NewReader("hello world")); err != nil {
		t.Fatalf("SetReader failed: %v", err)
	}

	count1 := 0
	for {
		hasToken, _ := tokenizer.IncrementToken()
		if !hasToken {
			break
		}
		count1++
	}

	if count1 != 2 {
		t.Errorf("First input: got %d tokens, expected 2", count1)
	}

	// Reset and use new input
	if err := tokenizer.Reset(); err != nil {
		t.Fatalf("Reset failed: %v", err)
	}

	if err := tokenizer.SetReader(strings.NewReader("a b c")); err != nil {
		t.Fatalf("SetReader failed: %v", err)
	}

	count2 := 0
	for {
		hasToken, err := tokenizer.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken failed: %v", err)
		}
		if !hasToken {
			break
		}
		count2++
	}

	if count2 != 3 {
		t.Errorf("Second input: got %d tokens, expected 3", count2)
	}
}

// TestPatternTokenizer_End tests end-of-stream operations.
func TestPatternTokenizer_End(t *testing.T) {
	input := "hello world"
	re := regexp.MustCompile(`\s+`)
	tokenizer := NewPatternTokenizer(re)

	if err := tokenizer.SetReader(strings.NewReader(input)); err != nil {
		t.Fatalf("SetReader failed: %v", err)
	}

	// Consume all tokens
	for {
		hasToken, _ := tokenizer.IncrementToken()
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

	oa := offsetAttr.(OffsetAttribute)
	if oa.EndOffset() != len(input) {
		t.Errorf("End offset = %d, want %d", oa.EndOffset(), len(input))
	}
}

// TestPatternTokenizer_Unicode tests Unicode handling.
func TestPatternTokenizer_Unicode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		pattern  string
		group    int
		expected []string
	}{
		{
			name:     "unicode words split",
			input:    "hello 世界 test",
			pattern:  `\s+`,
			group:    -1,
			expected: []string{"hello", "世界", "test"},
		},
		{
			name:     "unicode match",
			input:    "hello 世界 test 日本語",
			pattern:  `\p{Han}+`,
			group:    0,
			expected: []string{"世界", "日本語"},
		},
		{
			name:     "emoji handling",
			input:    "hello 🎉 world 🚀 test",
			pattern:  `\s+`,
			group:    -1,
			expected: []string{"hello", "🎉", "world", "🚀", "test"},
		},
		{
			name:     "mixed unicode",
			input:    "Привет мир hello world",
			pattern:  `\s+`,
			group:    -1,
			expected: []string{"Привет", "мир", "hello", "world"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			re := regexp.MustCompile(tt.pattern)
			tokenizer := NewPatternTokenizerWithGroup(re, tt.group)

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
				tokens = append(tokens, termAttr.(CharTermAttribute).String())
			}

			if len(tokens) != len(tt.expected) {
				t.Errorf("Got %d tokens, expected %d: got %v, want %v", len(tokens), len(tt.expected), tokens, tt.expected)
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

// TestPatternTokenizer_LargeInput tests handling of large input.
func TestPatternTokenizer_LargeInput(t *testing.T) {
	// Create a large input with repeated pattern
	large := strings.Repeat("word ", 1000)
	re := regexp.MustCompile(`\s+`)
	tokenizer := NewPatternTokenizer(re)

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
		t.Errorf("Got %d tokens, expected 1000", count)
	}
}

// TestPatternTokenizer_ConsecutiveDelimiters tests consecutive delimiters.
func TestPatternTokenizer_ConsecutiveDelimiters(t *testing.T) {
	input := "a,,b,,,c"
	re := regexp.MustCompile(`,+`)
	tokenizer := NewPatternTokenizer(re)

	if err := tokenizer.SetReader(strings.NewReader(input)); err != nil {
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
		tokens = append(tokens, termAttr.(CharTermAttribute).String())
	}

	expected := []string{"a", "b", "c"}
	if len(tokens) != len(expected) {
		t.Errorf("Got %d tokens, expected %d: got %v, want %v", len(tokens), len(expected), tokens, expected)
		return
	}

	for i, exp := range expected {
		if tokens[i] != exp {
			t.Errorf("Token %d = %q, want %q", i, tokens[i], exp)
		}
	}
}

// TestPatternTokenizer_Getters tests the getter methods.
func TestPatternTokenizer_Getters(t *testing.T) {
	re := regexp.MustCompile(`\s+`)

	// Split mode (group = -1)
	tokenizer1 := NewPatternTokenizer(re)
	if tokenizer1.GetPattern() != re {
		t.Error("GetPattern returned wrong pattern")
	}
	if tokenizer1.GetGroup() != -1 {
		t.Errorf("GetGroup = %d, want -1", tokenizer1.GetGroup())
	}
	if tokenizer1.IsMatchMode() {
		t.Error("IsMatchMode should be false for split mode")
	}

	// Match mode (group = 0)
	tokenizer2 := NewPatternTokenizerWithGroup(re, 0)
	if tokenizer2.GetGroup() != 0 {
		t.Errorf("GetGroup = %d, want 0", tokenizer2.GetGroup())
	}
	if !tokenizer2.IsMatchMode() {
		t.Error("IsMatchMode should be true for match mode")
	}

	// Match mode with group
	tokenizer3 := NewPatternTokenizerWithGroup(re, 2)
	if tokenizer3.GetGroup() != 2 {
		t.Errorf("GetGroup = %d, want 2", tokenizer3.GetGroup())
	}
}

// TestPatternTokenizerFactory tests the factory creation.
func TestPatternTokenizerFactory(t *testing.T) {
	// Valid pattern
	factory := NewPatternTokenizerFactory(`\s+`)
	if factory == nil {
		t.Fatal("NewPatternTokenizerFactory returned nil")
	}

	tokenizer := factory.Create()
	if tokenizer == nil {
		t.Fatal("Factory.Create returned nil")
	}

	// Verify it's a PatternTokenizer
	pt, ok := tokenizer.(*PatternTokenizer)
	if !ok {
		t.Error("Factory did not create PatternTokenizer")
		return
	}

	if pt.IsMatchMode() {
		t.Error("Factory with -1 group should create split mode tokenizer")
	}

	// Invalid pattern
	factoryInvalid := NewPatternTokenizerFactory(`[invalid`)
	if factoryInvalid != nil {
		t.Error("Expected nil for invalid pattern")
	}
}

// TestPatternTokenizerFactory_WithGroup tests factory with group parameter.
func TestPatternTokenizerFactory_WithGroup(t *testing.T) {
	factory := NewPatternTokenizerFactoryWithGroup(`<(\w+)>`, 1)
	if factory == nil {
		t.Fatal("NewPatternTokenizerFactoryWithGroup returned nil")
	}

	tokenizer := factory.Create()
	if tokenizer == nil {
		t.Fatal("Factory.Create returned nil")
	}

	pt := tokenizer.(*PatternTokenizer)
	if pt.GetGroup() != 1 {
		t.Errorf("GetGroup = %d, want 1", pt.GetGroup())
	}
	if !pt.IsMatchMode() {
		t.Error("Expected match mode")
	}
}

// TestPatternTokenizer_Close tests resource cleanup.
func TestPatternTokenizer_Close(t *testing.T) {
	re := regexp.MustCompile(`\s+`)
	tokenizer := NewPatternTokenizer(re)

	if err := tokenizer.SetReader(strings.NewReader("hello world")); err != nil {
		t.Fatalf("SetReader failed: %v", err)
	}

	// Consume one token
	tokenizer.IncrementToken()

	// Close
	if err := tokenizer.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// After close, should not have more tokens
	hasToken, _ := tokenizer.IncrementToken()
	if hasToken {
		t.Error("Expected no tokens after close")
	}
}

// BenchmarkPatternTokenizer_Split benchmarks split mode.
func BenchmarkPatternTokenizer_Split(b *testing.B) {
	input := "hello world test foo bar baz"
	re := regexp.MustCompile(`\s+`)
	reader := strings.NewReader(input)
	tokenizer := NewPatternTokenizer(re)

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
		tokenizer.Reset()
	}
}

// BenchmarkPatternTokenizer_Match benchmarks match mode.
func BenchmarkPatternTokenizer_Match(b *testing.B) {
	input := "hello, world! test. foo bar baz."
	re := regexp.MustCompile(`\b\w+\b`)
	reader := strings.NewReader(input)
	tokenizer := NewPatternTokenizerWithGroup(re, 0)

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
		tokenizer.Reset()
	}
}

// BenchmarkPatternTokenizer_Large benchmarks large input.
func BenchmarkPatternTokenizer_Large(b *testing.B) {
	input := strings.Repeat("word ", 1000)
	re := regexp.MustCompile(`\s+`)
	reader := strings.NewReader(input)
	tokenizer := NewPatternTokenizer(re)

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
		tokenizer.Reset()
	}
}
