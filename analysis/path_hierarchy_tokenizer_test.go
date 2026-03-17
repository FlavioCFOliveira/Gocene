// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"strings"
	"testing"
)

// TestPathHierarchyTokenizer_ForwardBasic tests basic forward tokenization.
func TestPathHierarchyTokenizer_ForwardBasic(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "simple path",
			input:    "/a/b/c",
			expected: []string{"/a", "/a/b", "/a/b/c"},
		},
		{
			name:     "path without leading delimiter",
			input:    "a/b/c",
			expected: []string{"a", "a/b", "a/b/c"},
		},
		{
			name:     "single component",
			input:    "/a",
			expected: []string{"/a"},
		},
		{
			name:     "single component no delimiter",
			input:    "a",
			expected: []string{"a"},
		},
		{
			name:     "empty path",
			input:    "",
			expected: []string{},
		},
		{
			name:     "just delimiter",
			input:    "/",
			expected: []string{"/"},
		},
		{
			name:     "multiple delimiters",
			input:    "/a//b",
			expected: []string{"/a", "/a/", "/a//b"},
		},
		{
			name:     "trailing delimiter",
			input:    "/a/b/",
			expected: []string{"/a", "/a/b", "/a/b/"},
		},
		{
			name:     "deep path",
			input:    "/usr/local/bin/go",
			expected: []string{"/usr", "/usr/local", "/usr/local/bin", "/usr/local/bin/go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer := NewPathHierarchyTokenizer()
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
				t.Errorf("Token count = %d, want %d. Got: %v", len(tokens), len(tt.expected), tokens)
				return
			}

			for i, expected := range tt.expected {
				if tokens[i] != expected {
					t.Errorf("Token[%d] = %q, want %q", i, tokens[i], expected)
				}
			}
		})
	}
}

// TestPathHierarchyTokenizer_ReverseBasic tests basic reverse tokenization.
func TestPathHierarchyTokenizer_ReverseBasic(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "simple path reverse",
			input:    "/a/b/c",
			expected: []string{"/a/b/c", "/b/c", "/c"},
		},
		{
			name:     "path without leading delimiter reverse",
			input:    "a/b/c",
			expected: []string{"a/b/c", "/b/c", "/c"},
		},
		{
			name:     "single component reverse",
			input:    "/a",
			expected: []string{"/a"},
		},
		{
			name:     "single component no delimiter reverse",
			input:    "a",
			expected: []string{"a"},
		},
		{
			name:     "empty path reverse",
			input:    "",
			expected: []string{},
		},
		{
			name:     "deep path reverse",
			input:    "/usr/local/bin/go",
			expected: []string{"/usr/local/bin/go", "/local/bin/go", "/bin/go", "/go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer := NewPathHierarchyTokenizer(WithReverse(true))
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
				t.Errorf("Token count = %d, want %d. Got: %v", len(tokens), len(tt.expected), tokens)
				return
			}

			for i, expected := range tt.expected {
				if tokens[i] != expected {
					t.Errorf("Token[%d] = %q, want %q", i, tokens[i], expected)
				}
			}
		})
	}
}

// TestPathHierarchyTokenizer_CustomDelimiter tests custom delimiter.
func TestPathHierarchyTokenizer_CustomDelimiter(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		delimiter byte
		expected  []string
	}{
		{
			name:      "dot delimiter",
			input:     "com.example.app",
			delimiter: '.',
			expected:  []string{"com", "com.example", "com.example.app"},
		},
		{
			name:      "backslash delimiter",
			input:     "\\Windows\\System32",
			delimiter: '\\',
			expected:  []string{"\\Windows", "\\Windows\\System32"},
		},
		{
			name:      "colon delimiter",
			input:     "category:subcategory:item",
			delimiter: ':',
			expected:  []string{"category", "category:subcategory", "category:subcategory:item"},
		},
		{
			name:      "greater than delimiter",
			input:     "Electronics>Computers>Laptops",
			delimiter: '>',
			expected:  []string{"Electronics", "Electronics>Computers", "Electronics>Computers>Laptops"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer := NewPathHierarchyTokenizer(WithDelimiter(tt.delimiter))
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
				t.Errorf("Token count = %d, want %d. Got: %v", len(tokens), len(tt.expected), tokens)
				return
			}

			for i, expected := range tt.expected {
				if tokens[i] != expected {
					t.Errorf("Token[%d] = %q, want %q", i, tokens[i], expected)
				}
			}
		})
	}
}

// TestPathHierarchyTokenizer_Skip tests skip functionality.
func TestPathHierarchyTokenizer_Skip(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		skip     int
		reverse  bool
		expected []string
	}{
		{
			name:     "skip 1 forward",
			input:    "/a/b/c",
			skip:     1,
			reverse:  false,
			expected: []string{"/a/b", "/a/b/c"},
		},
		{
			name:     "skip 2 forward",
			input:    "/a/b/c",
			skip:     2,
			reverse:  false,
			expected: []string{"/a/b/c"},
		},
		{
			name:     "skip all forward",
			input:    "/a/b/c",
			skip:     3,
			reverse:  false,
			expected: []string{},
		},
		{
			name:     "skip more than components forward",
			input:    "/a/b/c",
			skip:     5,
			reverse:  false,
			expected: []string{},
		},
		{
			name:     "skip 1 reverse",
			input:    "/a/b/c",
			skip:     1,
			reverse:  true,
			expected: []string{"/b/c", "/c"},
		},
		{
			name:     "skip 2 reverse",
			input:    "/a/b/c",
			skip:     2,
			reverse:  true,
			expected: []string{"/c"},
		},
		{
			name:     "skip all reverse",
			input:    "/a/b/c",
			skip:     3,
			reverse:  true,
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer := NewPathHierarchyTokenizer(
				WithSkip(tt.skip),
				WithReverse(tt.reverse),
			)
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
				t.Errorf("Token count = %d, want %d. Got: %v", len(tokens), len(tt.expected), tokens)
				return
			}

			for i, expected := range tt.expected {
				if tokens[i] != expected {
					t.Errorf("Token[%d] = %q, want %q", i, tokens[i], expected)
				}
			}
		})
	}
}

// TestPathHierarchyTokenizer_Offsets tests offset attribute handling.
func TestPathHierarchyTokenizer_Offsets(t *testing.T) {
	input := "/a/b/c"
	tokenizer := NewPathHierarchyTokenizer()
	if err := tokenizer.SetReader(strings.NewReader(input)); err != nil {
		t.Fatalf("SetReader failed: %v", err)
	}

	expectedOffsets := []struct {
		start int
		end   int
	}{
		{0, 2},  // "/a"
		{0, 4},  // "/a/b"
		{0, 6},  // "/a/b/c"
	}

	tokenIndex := 0
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

		offset := offsetAttr.(OffsetAttribute)
		start := offset.StartOffset()
		end := offset.EndOffset()

		if tokenIndex >= len(expectedOffsets) {
			t.Fatalf("More tokens than expected")
		}

		if start != expectedOffsets[tokenIndex].start {
			t.Errorf("Token %d start offset = %d, want %d", tokenIndex, start, expectedOffsets[tokenIndex].start)
		}
		if end != expectedOffsets[tokenIndex].end {
			t.Errorf("Token %d end offset = %d, want %d", tokenIndex, end, expectedOffsets[tokenIndex].end)
		}

		tokenIndex++
	}

	if tokenIndex != len(expectedOffsets) {
		t.Errorf("Token count = %d, want %d", tokenIndex, len(expectedOffsets))
	}
}

// TestPathHierarchyTokenizer_PositionIncrement tests position increment.
func TestPathHierarchyTokenizer_PositionIncrement(t *testing.T) {
	input := "/a/b/c"
	tokenizer := NewPathHierarchyTokenizer()
	if err := tokenizer.SetReader(strings.NewReader(input)); err != nil {
		t.Fatalf("SetReader failed: %v", err)
	}

	for i := 0; i < 3; i++ {
		hasToken, err := tokenizer.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken failed: %v", err)
		}
		if !hasToken {
			t.Fatalf("Expected token at index %d", i)
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

// TestPathHierarchyTokenizer_Reset tests reset functionality.
func TestPathHierarchyTokenizer_Reset(t *testing.T) {
	tokenizer := NewPathHierarchyTokenizer()

	// First input
	if err := tokenizer.SetReader(strings.NewReader("/a/b")); err != nil {
		t.Fatalf("SetReader failed: %v", err)
	}

	// Consume first token
	hasToken, _ := tokenizer.IncrementToken()
	if !hasToken {
		t.Fatal("Expected token from first input")
	}

	termAttr := tokenizer.GetAttributeSource().GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
	term := termAttr.(CharTermAttribute).String()
	if term != "/a" {
		t.Errorf("First token = %q, want '/a'", term)
	}

	// Reset and use new input
	if err := tokenizer.Reset(); err != nil {
		t.Fatalf("Reset failed: %v", err)
	}
	if err := tokenizer.SetReader(strings.NewReader("/x/y/z")); err != nil {
		t.Fatalf("SetReader failed: %v", err)
	}

	// Consume tokens from new input
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

	expected := []string{"/x", "/x/y", "/x/y/z"}
	if len(tokens) != len(expected) {
		t.Errorf("Token count = %d, want %d", len(tokens), len(expected))
	}
	for i, exp := range expected {
		if tokens[i] != exp {
			t.Errorf("Token[%d] = %q, want %q", i, tokens[i], exp)
		}
	}
}

// TestPathHierarchyTokenizer_Factory tests the factory creation.
func TestPathHierarchyTokenizer_Factory(t *testing.T) {
	factory := NewPathHierarchyTokenizerFactory(
		WithFactoryDelimiter('.'),
		WithFactorySkip(1),
		WithFactoryReverse(true),
	)

	tokenizer := factory.Create()
	if tokenizer == nil {
		t.Fatal("Factory.Create returned nil")
	}

	// Verify it's a PathHierarchyTokenizer
	phTokenizer, ok := tokenizer.(*PathHierarchyTokenizer)
	if !ok {
		t.Fatal("Factory did not create PathHierarchyTokenizer")
	}

	// Test the created tokenizer
	if err := phTokenizer.SetReader(strings.NewReader("com.example.app")); err != nil {
		t.Fatalf("SetReader failed: %v", err)
	}

	var tokens []string
	for {
		hasToken, err := phTokenizer.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken failed: %v", err)
		}
		if !hasToken {
			break
		}

		termAttr := phTokenizer.GetAttributeSource().GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
		term := termAttr.(CharTermAttribute).String()
		tokens = append(tokens, term)
	}

	// With skip=1 and reverse=true: "com.example.app" -> skip "com" -> ".example.app", ".app"
	expected := []string{".example.app", ".app"}
	if len(tokens) != len(expected) {
		t.Errorf("Token count = %d, want %d. Got: %v", len(tokens), len(expected), tokens)
	}
	for i, exp := range expected {
		if tokens[i] != exp {
			t.Errorf("Token[%d] = %q, want %q", i, tokens[i], exp)
		}
	}
}

// TestPathHierarchyTokenizer_End tests End() functionality.
func TestPathHierarchyTokenizer_End(t *testing.T) {
	input := "/a/b/c"
	tokenizer := NewPathHierarchyTokenizer()
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

	// Check that end offset is set to input length
	offsetAttr := tokenizer.GetAttributeSource().GetAttributeByType(reflect.TypeOf(&offsetAttribute{}))
	if offsetAttr == nil {
		t.Fatal("OffsetAttribute is nil")
	}

	offset := offsetAttr.(OffsetAttribute)
	if offset.EndOffset() != len(input) {
		t.Errorf("End offset = %d, want %d", offset.EndOffset(), len(input))
	}
}

// TestPathHierarchyTokenizer_Unicode tests Unicode handling.
func TestPathHierarchyTokenizer_Unicode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Chinese path",
			input:    "/中文/文档/文件",
			expected: []string{"/中文", "/中文/文档", "/中文/文档/文件"},
		},
		{
			name:     "Mixed Unicode",
			input:    "/files/文档/file.txt",
			expected: []string{"/files", "/files/文档", "/files/文档/file.txt"},
		},
		{
			name:     "Emoji in path",
			input:    "/docs/📁/file",
			expected: []string{"/docs", "/docs/📁", "/docs/📁/file"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer := NewPathHierarchyTokenizer()
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
				t.Errorf("Token count = %d, want %d. Got: %v", len(tokens), len(tt.expected), tokens)
				return
			}

			for i, expected := range tt.expected {
				if tokens[i] != expected {
					t.Errorf("Token[%d] = %q, want %q", i, tokens[i], expected)
				}
			}
		})
	}
}

// TestPathHierarchyTokenizer_URLPaths tests URL path tokenization.
func TestPathHierarchyTokenizer_URLPaths(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "simple URL path",
			input:    "/products/electronics/laptops",
			expected: []string{"/products", "/products/electronics", "/products/electronics/laptops"},
		},
		{
			name:     "API path",
			input:    "/api/v1/users/profile",
			expected: []string{"/api", "/api/v1", "/api/v1/users", "/api/v1/users/profile"},
		},
		{
			name:     "deeply nested",
			input:    "/a/b/c/d/e/f",
			expected: []string{"/a", "/a/b", "/a/b/c", "/a/b/c/d", "/a/b/c/d/e", "/a/b/c/d/e/f"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer := NewPathHierarchyTokenizer()
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
				t.Errorf("Token count = %d, want %d. Got: %v", len(tokens), len(tt.expected), tokens)
				return
			}

			for i, expected := range tt.expected {
				if tokens[i] != expected {
					t.Errorf("Token[%d] = %q, want %q", i, tokens[i], expected)
				}
			}
		})
	}
}

// Benchmark tests
func BenchmarkPathHierarchyTokenizer_Short(b *testing.B) {
	input := "/a/b/c"
	reader := strings.NewReader(input)
	tokenizer := NewPathHierarchyTokenizer()

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

func BenchmarkPathHierarchyTokenizer_Long(b *testing.B) {
	input := "/usr/local/bin/go/projects/src/github.com/user/repo/package/subpackage"
	reader := strings.NewReader(input)
	tokenizer := NewPathHierarchyTokenizer()

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

func BenchmarkPathHierarchyTokenizer_Reverse(b *testing.B) {
	input := "/usr/local/bin/go/projects/src/github.com/user/repo/package/subpackage"
	reader := strings.NewReader(input)
	tokenizer := NewPathHierarchyTokenizer(WithReverse(true))

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
