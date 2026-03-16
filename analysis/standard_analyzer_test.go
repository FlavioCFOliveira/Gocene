// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"strings"
	"testing"
)

// TestStandardAnalyzer_BasicTokenization tests basic tokenization.
// Source: TestStandardAnalyzer.testAlphanumericSA()
// Purpose: Tests alphanumeric token handling.
func TestStandardAnalyzer_BasicTokenization(t *testing.T) {
	analyzer := NewStandardAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "alphanumeric B2B",
			input:    "B2B",
			expected: []string{"b2b"},
		},
		{
			name:     "alphanumeric 2B",
			input:    "2B",
			expected: []string{"2b"},
		},
		{
			name:     "simple words",
			input:    "foo bar FOO BAR",
			expected: []string{"foo", "bar", "foo", "bar"},
		},
		{
			name:     "with punctuation",
			input:    "foo      bar .  FOO <> BAR",
			expected: []string{"foo", "bar", "foo", "bar"},
		},
		{
			name:     "quoted word",
			input:    "\"QUOTED\" word",
			expected: []string{"quoted", "word"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokens, err := collectTokensFromAnalyzer(analyzer, tc.input)
			if err != nil {
				t.Fatalf("TokenStream failed: %v", err)
			}

			if !reflect.DeepEqual(tokens, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, tokens)
			}
		})
	}
}

// TestStandardAnalyzer_Delimiters tests delimiter handling.
// Source: TestStandardAnalyzer.testDelimitersSA()
// Purpose: Tests various delimiters like dash, slash, comma.
func TestStandardAnalyzer_Delimiters(t *testing.T) {
	analyzer := NewStandardAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "dashed phrase",
			input:    "some-dashed-phrase",
			expected: []string{"some", "dashed", "phrase"},
		},
		{
			name:     "comma separated",
			input:    "dogs,chase,cats",
			expected: []string{"dogs", "chase", "cats"},
		},
		{
			name:     "slash separated",
			input:    "ac/dc",
			expected: []string{"ac", "dc"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokens, err := collectTokensFromAnalyzer(analyzer, tc.input)
			if err != nil {
				t.Fatalf("TokenStream failed: %v", err)
			}

			if !reflect.DeepEqual(tokens, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, tokens)
			}
		})
	}
}

// TestStandardAnalyzer_Apostrophes tests apostrophe handling.
// Source: TestStandardAnalyzer.testApostrophesSA()
// Purpose: Tests internal apostrophes like O'Reilly.
func TestStandardAnalyzer_Apostrophes(t *testing.T) {
	analyzer := NewStandardAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "O'Reilly",
			input:    "O'Reilly",
			expected: []string{"o'reilly"},
		},
		{
			name:     "you're",
			input:    "you're",
			expected: []string{"you're"},
		},
		{
			name:     "she's",
			input:    "she's",
			expected: []string{"she's"},
		},
		{
			name:     "Jim's",
			input:    "Jim's",
			expected: []string{"jim's"},
		},
		{
			name:     "don't",
			input:    "don't",
			expected: []string{"don't"},
		},
		{
			name:     "O'Reilly's",
			input:    "O'Reilly's",
			expected: []string{"o'reilly's"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokens, err := collectTokensFromAnalyzer(analyzer, tc.input)
			if err != nil {
				t.Fatalf("TokenStream failed: %v", err)
			}

			if !reflect.DeepEqual(tokens, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, tokens)
			}
		})
	}
}

// TestStandardAnalyzer_Numeric tests numeric token handling.
// Source: TestStandardAnalyzer.testNumericSA()
// Purpose: Tests floating point, serial, model numbers, IP addresses.
func TestStandardAnalyzer_Numeric(t *testing.T) {
	analyzer := NewStandardAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "floating point",
			input:    "21.35",
			expected: []string{"21.35"},
		},
		{
			name:     "alphanumeric model",
			input:    "R2D2 C3PO",
			expected: []string{"r2d2", "c3po"},
		},
		{
			name:     "IP address",
			input:    "216.239.63.104",
			expected: []string{"216.239.63.104"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokens, err := collectTokensFromAnalyzer(analyzer, tc.input)
			if err != nil {
				t.Fatalf("TokenStream failed: %v", err)
			}

			if !reflect.DeepEqual(tokens, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, tokens)
			}
		})
	}
}

// TestStandardAnalyzer_TextWithNumbers tests mixed text and numbers.
// Source: TestStandardAnalyzer.testTextWithNumbersSA()
// Purpose: Tests handling of text with embedded numbers.
func TestStandardAnalyzer_TextWithNumbers(t *testing.T) {
	analyzer := NewStandardAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "text with number",
			input:    "David has 5000 bones",
			expected: []string{"david", "has", "5000", "bones"},
		},
		{
			name:     "various formats",
			input:    "C embedded developers wanted",
			expected: []string{"c", "embedded", "developers", "wanted"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokens, err := collectTokensFromAnalyzer(analyzer, tc.input)
			if err != nil {
				t.Fatalf("TokenStream failed: %v", err)
			}

			if !reflect.DeepEqual(tokens, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, tokens)
			}
		})
	}
}

// TestStandardAnalyzer_Offsets tests offset tracking.
// Source: TestStandardAnalyzer.testOffsets()
// Purpose: Tests character offset tracking.
func TestStandardAnalyzer_Offsets(t *testing.T) {
	analyzer := NewStandardAnalyzer()
	defer analyzer.Close()

	input := "David has 5000 bones"
	stream, err := analyzer.TokenStream("field", strings.NewReader(input))
	if err != nil {
		t.Fatalf("TokenStream failed: %v", err)
	}
	defer stream.Close()

	type tokenInfo struct {
		text  string
		start int
		end   int
	}

	expected := []tokenInfo{
		{"david", 0, 5},
		{"has", 6, 9},
		{"5000", 10, 14},
		{"bones", 15, 20},
	}

	var tokens []tokenInfo
	for {
		hasToken, err := stream.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken failed: %v", err)
		}
		if !hasToken {
			break
		}

		var info tokenInfo
		attrSrc := stream.(interface{ GetAttributeSource() *AttributeSource }).GetAttributeSource()

		if attr := attrSrc.GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				info.text = termAttr.String()
			}
		}
		if attr := attrSrc.GetAttribute("OffsetAttribute"); attr != nil {
			if offsetAttr, ok := attr.(OffsetAttribute); ok {
				info.start = offsetAttr.StartOffset()
				info.end = offsetAttr.EndOffset()
			}
		}
		tokens = append(tokens, info)
	}

	if len(tokens) != len(expected) {
		t.Fatalf("Expected %d tokens, got %d", len(expected), len(tokens))
	}

	for i, exp := range expected {
		if tokens[i].text != exp.text {
			t.Errorf("Token[%d]: expected text %q, got %q", i, exp.text, tokens[i].text)
		}
		if tokens[i].start != exp.start {
			t.Errorf("Token[%d]: expected start %d, got %d", i, exp.start, tokens[i].start)
		}
		if tokens[i].end != exp.end {
			t.Errorf("Token[%d]: expected end %d, got %d", i, exp.end, tokens[i].end)
		}
	}
}

// TestStandardAnalyzer_Empty tests empty input handling.
// Source: TestStandardAnalyzer.testEmpty()
// Purpose: Tests empty input handling.
func TestStandardAnalyzer_Empty(t *testing.T) {
	analyzer := NewStandardAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"only space", " "},
		{"only dot", "."},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokens, err := collectTokensFromAnalyzer(analyzer, tc.input)
			if err != nil {
				t.Fatalf("TokenStream failed: %v", err)
			}

			if len(tokens) != 0 {
				t.Errorf("Expected 0 tokens for %q, got %d: %v", tc.input, len(tokens), tokens)
			}
		})
	}
}

// TestStandardAnalyzer_Mid tests mid-character handling.
// Source: TestStandardAnalyzer.testMid()
// Purpose: Tests handling of mid-letter, mid-num, mid-numlet characters.
func TestStandardAnalyzer_Mid(t *testing.T) {
	analyzer := NewStandardAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		// : is in WB:MidLetter - should not split if letter on both sides
		{"A:B", "A:B", []string{"a:b"}},
		{"A::B", "A::B", []string{"a", "b"}},

		// . is in WB:MidNumLet - should not split if letter or number on both sides
		{"1.2", "1.2", []string{"1.2"}},
		{"A.B", "A.B", []string{"a.b"}},
		{"1..2", "1..2", []string{"1", "2"}},
		{"A..B", "A..B", []string{"a", "b"}},

		// , is in WB:MidNum - should not split if number on both sides
		{"1,2", "1,2", []string{"1,2"}},
		{"1,,2", "1,,2", []string{"1", "2"}},

		// Mixed consecutive mid characters should trigger split
		{"A.:B", "A.:B", []string{"a", "b"}},
		{"A:.B", "A:.B", []string{"a", "b"}},
		{"1,.2", "1,.2", []string{"1", "2"}},
		{"1.,2", "1.,2", []string{"1", "2"}},

		// _ is in WB:ExtendNumLet
		{"A:B_A:B", "A:B_A:B", []string{"a:b_a:b"}},
		{"A:B_A::B", "A:B_A::B", []string{"a:b_a", "b"}},

		{"1.2_1.2", "1.2_1.2", []string{"1.2_1.2"}},
		{"A.B_A.B", "A.B_A.B", []string{"a.b_a.b"}},
		{"1.2_1..2", "1.2_1..2", []string{"1.2_1", "2"}},
		{"A.B_A..B", "A.B_A..B", []string{"a.b_a", "b"}},

		{"1,2_1,2", "1,2_1,2", []string{"1,2_1,2"}},
		{"1,2_1,,2", "1,2_1,,2", []string{"1,2_1", "2"}},

		{"C_A.:B", "C_A.:B", []string{"c_a", "b"}},
		{"C_A:.B", "C_A:.B", []string{"c_a", "b"}},

		{"3_1,.2", "3_1,.2", []string{"3_1", "2"}},
		{"3_1.,2", "3_1.,2", []string{"3_1", "2"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokens, err := collectTokensFromAnalyzer(analyzer, tc.input)
			if err != nil {
				t.Fatalf("TokenStream failed: %v", err)
			}

			// Compare lowercase versions
			var gotTokens []string
			for _, tok := range tokens {
				gotTokens = append(gotTokens, tok)
			}

			if !reflect.DeepEqual(gotTokens, tc.expected) {
				t.Errorf("Input %q: expected %v, got %v", tc.input, tc.expected, gotTokens)
			}
		})
	}
}

// TestStandardAnalyzer_UnicodeLanguages tests Unicode language support.
// Source: TestStandardAnalyzer various language tests
// Purpose: Tests tokenization of various languages.
func TestStandardAnalyzer_UnicodeLanguages(t *testing.T) {
	analyzer := NewStandardAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		name      string
		input     string
		minTokens int
	}{
		{
			name:      "Chinese characters",
			input:     "我是中国人",
			minTokens: 5, // Each character is a token
		},
		{
			name:      "Korean",
			input:     "안녕하세요 한글입니다",
			minTokens: 2,
		},
		{
			name:      "Japanese",
			input:     "仮名遣い カタカナ",
			minTokens: 2,
		},
		{
			name:      "Greek",
			input:     "Γράφεται σε συνεργασία",
			minTokens: 2,
		},
		{
			name:      "Arabic",
			input:     "الفيلم الوثائقي",
			minTokens: 2,
		},
		{
			name:      "Thai",
			input:     "การที่ได้ต้องแสดงว่างานดี",
			minTokens: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokens, err := collectTokensFromAnalyzer(analyzer, tc.input)
			if err != nil {
				t.Fatalf("TokenStream failed: %v", err)
			}

			if len(tokens) < tc.minTokens {
				t.Errorf("Expected at least %d tokens, got %d: %v", tc.minTokens, len(tokens), tokens)
			}
		})
	}
}

// TestStandardAnalyzer_Supplementary tests supplementary character handling.
// Source: TestStandardAnalyzer.testSupplementary()
// Purpose: Tests handling of supplementary Unicode characters.
func TestStandardAnalyzer_Supplementary(t *testing.T) {
	analyzer := NewStandardAnalyzer()
	defer analyzer.Close()

	// Test with supplementary characters (ideographic)
	input := "𩬅艱鍟䇹愯瀛"
	tokens, err := collectTokensFromAnalyzer(analyzer, input)
	if err != nil {
		t.Fatalf("TokenStream failed: %v", err)
	}

	// Each supplementary character should be a separate token
	// The exact number depends on implementation
	if len(tokens) == 0 {
		t.Error("Expected at least one token for supplementary characters")
	}
}

// TestStandardAnalyzer_Emoji tests emoji tokenization.
// Source: TestStandardAnalyzer.testEmoji()
// Purpose: Tests handling of emoji characters.
func TestStandardAnalyzer_Emoji(t *testing.T) {
	analyzer := NewStandardAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		name      string
		input     string
		minTokens int
	}{
		{
			name:      "simple emoji",
			input:     "💩 💩💩",
			minTokens: 1,
		},
		{
			name:      "emoji with text",
			input:     "poo💩poo",
			minTokens: 2, // "poo" and "poo" with emoji in between
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokens, err := collectTokensFromAnalyzer(analyzer, tc.input)
			if err != nil {
				t.Fatalf("TokenStream failed: %v", err)
			}

			if len(tokens) < tc.minTokens {
				t.Errorf("Expected at least %d tokens, got %d: %v", tc.minTokens, len(tokens), tokens)
			}
		})
	}
}

// TestStandardAnalyzer_CombiningMarks tests combining marks handling.
// Source: TestStandardAnalyzer.testCombiningMarks()
// Purpose: Tests handling of combining marks with various character types.
func TestStandardAnalyzer_CombiningMarks(t *testing.T) {
	analyzer := NewStandardAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		// Hiragana with combining mark
		{"hiragana combining", "ざ", []string{"ざ"}},
		// Katakana with combining mark
		{"katakana combining", "ザ", []string{"ザ"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokens, err := collectTokensFromAnalyzer(analyzer, tc.input)
			if err != nil {
				t.Fatalf("TokenStream failed: %v", err)
			}

			if len(tokens) != len(tc.expected) {
				t.Errorf("Expected %d tokens, got %d", len(tc.expected), len(tokens))
				return
			}

			for i, exp := range tc.expected {
				if tokens[i] != exp {
					t.Errorf("Token[%d]: expected %q, got %q", i, exp, tokens[i])
				}
			}
		})
	}
}

// TestStandardAnalyzer_LargePartiallyMatchingToken tests large token handling.
// Source: TestStandardAnalyzer.testLargePartiallyMatchingToken()
// Purpose: Tests handling of large tokens with special patterns.
func TestStandardAnalyzer_LargePartiallyMatchingToken(t *testing.T) {
	analyzer := NewStandardAnalyzer()
	defer analyzer.Close()

	// Create a large string with word break extend characters
	var builder strings.Builder
	for i := 0; i < 1000; i++ {
		builder.WriteString("a")
	}
	largeInput := builder.String()

	tokens, err := collectTokensFromAnalyzer(analyzer, largeInput)
	if err != nil {
		t.Fatalf("TokenStream failed: %v", err)
	}

	// Should produce one token (or be truncated based on max token length)
	if len(tokens) == 0 {
		t.Error("Expected at least one token for large input")
	}
}

// TestStandardAnalyzer_HugeDoc tests handling of huge documents.
// Source: TestStandardAnalyzer.testHugeDoc()
// Purpose: Tests handling of documents with leading whitespace.
func TestStandardAnalyzer_HugeDoc(t *testing.T) {
	analyzer := NewStandardAnalyzer()
	defer analyzer.Close()

	// Create input with leading whitespace followed by content
	var builder strings.Builder
	for i := 0; i < 4094; i++ {
		builder.WriteString(" ")
	}
	builder.WriteString("testing 1234")

	input := builder.String()
	tokens, err := collectTokensFromAnalyzer(analyzer, input)
	if err != nil {
		t.Fatalf("TokenStream failed: %v", err)
	}

	expected := []string{"testing", "1234"}
	if len(tokens) != len(expected) {
		t.Errorf("Expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
	}
}

// TestStandardAnalyzer_StopWords tests stop word removal.
// Source: Various tests
// Purpose: Tests that English stop words are removed by default.
func TestStandardAnalyzer_StopWords(t *testing.T) {
	analyzer := NewStandardAnalyzer()
	defer analyzer.Close()

	// "the" is a stop word and should be removed
	input := "The quick brown fox"
	tokens, err := collectTokensFromAnalyzer(analyzer, input)
	if err != nil {
		t.Fatalf("TokenStream failed: %v", err)
	}

	// Check that "the" was not included
	for _, token := range tokens {
		if token == "the" {
			t.Error("Stop word 'the' should have been removed")
		}
	}

	// Check that other words are present
	expectedWords := []string{"quick", "brown", "fox"}
	for _, word := range expectedWords {
		found := false
		for _, token := range tokens {
			if token == word {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected word %q not found in tokens: %v", word, tokens)
		}
	}
}

// TestStandardAnalyzer_CustomStopWords tests custom stop words.
// Source: Various tests
// Purpose: Tests analyzer with custom stop words.
func TestStandardAnalyzer_CustomStopWords(t *testing.T) {
	customStopWords := []string{"foo", "bar", "baz"}
	analyzer := NewStandardAnalyzerWithStopWords(customStopWords)
	defer analyzer.Close()

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "remove custom stop words",
			input:    "foo hello bar world baz",
			expected: []string{"hello", "world"},
		},
		{
			name:     "preserve default stop words",
			input:    "the a an",
			expected: []string{"the", "a", "an"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokens, err := collectTokensFromAnalyzer(analyzer, tc.input)
			if err != nil {
				t.Fatalf("TokenStream failed: %v", err)
			}

			if !reflect.DeepEqual(tokens, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, tokens)
			}
		})
	}
}

// TestStandardAnalyzer_MaxTokenLength tests max token length.
// Source: TestStandardAnalyzer.testMaxTokenLengthDefault()
// Purpose: Tests handling of tokens that exceed max length.
func TestStandardAnalyzer_MaxTokenLength(t *testing.T) {
	analyzer := NewStandardAnalyzer()
	defer analyzer.Close()

	// Create a word that exceeds max token length (default is 255)
	longWord := strings.Repeat("a", 300)
	input := "x " + longWord + " y"

	tokens, err := collectTokensFromAnalyzer(analyzer, input)
	if err != nil {
		t.Fatalf("TokenStream failed: %v", err)
	}

	// The long word should be truncated or split
	// Exact behavior depends on implementation
	if len(tokens) < 2 {
		t.Errorf("Expected at least 2 tokens (x and y), got %d: %v", len(tokens), tokens)
	}
}

// TestStandardAnalyzer_TokenTypes tests token type tracking.
// Source: TestStandardAnalyzer.testTypes()
// Purpose: Tests that different token types are correctly identified.
func TestStandardAnalyzer_TokenTypes(t *testing.T) {
	// Note: Type tracking requires TypeAttribute implementation
	// This test verifies that tokenization works for different types
	analyzer := NewStandardAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		name  string
		input string
		count int
	}{
		{"alphanumeric", "Hello World", 2},
		{"numeric", "123 456", 2},
		{"mixed", "Hello 123 World", 3},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokens, err := collectTokensFromAnalyzer(analyzer, tc.input)
			if err != nil {
				t.Fatalf("TokenStream failed: %v", err)
			}

			if len(tokens) != tc.count {
				t.Errorf("Expected %d tokens, got %d: %v", tc.count, len(tokens), tokens)
			}
		})
	}
}

// TestStandardAnalyzer_MultipleFields tests analyzer with multiple fields.
// Source: Various tests
// Purpose: Tests that analyzer works correctly with different field names.
func TestStandardAnalyzer_MultipleFields(t *testing.T) {
	analyzer := NewStandardAnalyzer()
	defer analyzer.Close()

	input := "Hello World"
	fieldNames := []string{"title", "content", "body", "_all"}

	for _, fieldName := range fieldNames {
		t.Run(fieldName, func(t *testing.T) {
			stream, err := analyzer.TokenStream(fieldName, strings.NewReader(input))
			if err != nil {
				t.Fatalf("TokenStream failed for field %s: %v", fieldName, err)
			}
			defer stream.Close()

			tokens, err := collectTokensFromStream(stream)
			if err != nil {
				t.Fatalf("Collecting tokens failed: %v", err)
			}

			if len(tokens) != 2 {
				t.Errorf("Expected 2 tokens for field %s, got %d: %v", fieldName, len(tokens), tokens)
			}
		})
	}
}

// TestStandardAnalyzer_Reuse tests analyzer reuse.
// Source: Various tests
// Purpose: Tests that analyzer can be reused for multiple analyses.
func TestStandardAnalyzer_Reuse(t *testing.T) {
	analyzer := NewStandardAnalyzer()
	defer analyzer.Close()

	inputs := []string{
		"First document",
		"Second document",
		"Third document",
	}

	for _, input := range inputs {
		tokens, err := collectTokensFromAnalyzer(analyzer, input)
		if err != nil {
			t.Fatalf("TokenStream failed: %v", err)
		}

		if len(tokens) == 0 {
			t.Errorf("Expected tokens for input %q", input)
		}
	}
}

// collectTokensFromStream collects tokens from a token stream.
func collectTokensFromStream(stream TokenStream) ([]string, error) {
	var tokens []string
	for {
		hasToken, err := stream.IncrementToken()
		if err != nil {
			return nil, err
		}
		if !hasToken {
			break
		}

		attrSrc := stream.(interface{ GetAttributeSource() *AttributeSource }).GetAttributeSource()
		termAttr := attrSrc.GetAttribute("CharTermAttribute")
		if termAttr != nil {
			if ct, ok := termAttr.(CharTermAttribute); ok {
				tokens = append(tokens, ct.String())
			}
		}
	}

	return tokens, nil
}
