// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"strings"
	"testing"
)

// wdTokenInfo holds information about an emitted token for testing
type wdTokenInfo struct {
	text        string
	startOffset int
	endOffset   int
	posInc      int
}

// TestWordDelimiterFilter_Basic tests basic word delimiter filtering.
// Source: TestWordDelimiterFilter.testBasic()
// Purpose: Tests that tokens are split correctly at word boundaries.
func TestWordDelimiterFilter_Basic(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Simple word",
			input:    "hello",
			expected: []string{"hello"},
		},
		{
			name:     "CamelCase",
			input:    "PowerShot",
			expected: []string{"Power", "Shot"},
		},
		{
			name:     "With hyphen",
			input:    "Power-Shot",
			expected: []string{"Power", "Shot"},
		},
		{
			name:     "With underscore",
			input:    "Power_Shot",
			expected: []string{"Power", "Shot"},
		},
		{
			name:     "Numeric transition",
			input:    "j2se",
			expected: []string{"j", "2", "se"},
		},
		{
			name:     "Multiple words",
			input:    "hello world",
			expected: []string{"hello", "world"},
		},
		{
			name:     "Mixed delimiters",
			input:    "Power_Shot-Test",
			expected: []string{"Power", "Shot", "Test"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			filter := NewWordDelimiterFilter(tokenizer, true, true, true, false)
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

// TestWordDelimiterFilter_PreserveOriginal tests preserving the original token.
// Source: TestWordDelimiterFilter.testPreserveOriginal()
// Purpose: Tests that the original token is preserved when requested.
func TestWordDelimiterFilter_PreserveOriginal(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "CamelCase with original",
			input:    "PowerShot",
			expected: []string{"PowerShot", "Power", "Shot"},
		},
		{
			name:     "Simple word with original",
			input:    "hello",
			expected: []string{"hello"},
		},
		{
			name:     "With delimiter and original",
			input:    "Power-Shot",
			expected: []string{"Power-Shot", "Power", "Shot"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			filter := NewWordDelimiterFilter(tokenizer, true, true, true, true)
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

// TestWordDelimiterFilter_PositionIncrement tests position increment handling.
// Source: TestWordDelimiterFilter.testPositionIncrement()
// Purpose: Tests that position increments are set correctly for split tokens.
func TestWordDelimiterFilter_PositionIncrement(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		preserveOriginal bool
		expectedTokens  []string
		expectedPosIncs []int
	}{
		{
			name:            "Split without original",
			input:           "PowerShot",
			preserveOriginal: false,
			expectedTokens:  []string{"Power", "Shot"},
			expectedPosIncs: []int{1, 0},
		},
		{
			name:            "Split with original",
			input:           "PowerShot",
			preserveOriginal: true,
			expectedTokens:  []string{"PowerShot", "Power", "Shot"},
			expectedPosIncs: []int{1, 0, 0},
		},
		{
			name:            "Multiple words split",
			input:           "j2se test",
			preserveOriginal: false,
			expectedTokens:  []string{"j", "2", "se", "test"},
			expectedPosIncs: []int{1, 0, 0, 1},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			filter := NewWordDelimiterFilter(tokenizer, true, true, true, tc.preserveOriginal)
			defer filter.Close()

			var tokens []string
			var posIncs []int
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
				if attr := filter.GetAttributeSource().GetAttribute("PositionIncrementAttribute"); attr != nil {
					if posAttr, ok := attr.(PositionIncrementAttribute); ok {
						posIncs = append(posIncs, posAttr.GetPositionIncrement())
					}
				}
			}

			if !reflect.DeepEqual(tokens, tc.expectedTokens) {
				t.Errorf("Expected tokens %v, got %v", tc.expectedTokens, tokens)
			}
			if !reflect.DeepEqual(posIncs, tc.expectedPosIncs) {
				t.Errorf("Expected position increments %v, got %v", tc.expectedPosIncs, posIncs)
			}
		})
	}
}

// TestWordDelimiterFilter_Offsets tests offset handling.
// Source: TestWordDelimiterFilter.testOffsets()
// Purpose: Tests that character offsets are correctly calculated for split tokens.
func TestWordDelimiterFilter_Offsets(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedTokens []wdTokenInfo
	}{
		{
			name:  "CamelCase offsets",
			input: "PowerShot",
			expectedTokens: []wdTokenInfo{
				{text: "Power", startOffset: 0, endOffset: 5, posInc: 1},
				{text: "Shot", startOffset: 5, endOffset: 9, posInc: 0},
			},
		},
		{
			name:  "Hyphen offsets",
			input: "Power-Shot",
			expectedTokens: []wdTokenInfo{
				{text: "Power", startOffset: 0, endOffset: 5, posInc: 1},
				{text: "Shot", startOffset: 6, endOffset: 10, posInc: 0},
			},
		},
		{
			name:  "Numeric offsets",
			input: "j2se",
			expectedTokens: []wdTokenInfo{
				{text: "j", startOffset: 0, endOffset: 1, posInc: 1},
				{text: "2", startOffset: 1, endOffset: 2, posInc: 0},
				{text: "se", startOffset: 2, endOffset: 4, posInc: 0},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			filter := NewWordDelimiterFilter(tokenizer, true, true, true, false)
			defer filter.Close()

			var tokens []wdTokenInfo
			for {
				hasToken, err := filter.IncrementToken()
				if err != nil {
					t.Fatalf("Error incrementing token: %v", err)
				}
				if !hasToken {
					break
				}

				var info wdTokenInfo
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
				if attr := filter.GetAttributeSource().GetAttribute("PositionIncrementAttribute"); attr != nil {
					if posAttr, ok := attr.(PositionIncrementAttribute); ok {
						info.posInc = posAttr.GetPositionIncrement()
					}
				}
				tokens = append(tokens, info)
			}

			if len(tokens) != len(tc.expectedTokens) {
				t.Fatalf("Expected %d tokens, got %d", len(tc.expectedTokens), len(tokens))
			}

			for i, expected := range tc.expectedTokens {
				if tokens[i].text != expected.text {
					t.Errorf("Token %d: expected text %q, got %q", i, expected.text, tokens[i].text)
				}
				if tokens[i].startOffset != expected.startOffset {
					t.Errorf("Token %d (%s): expected startOffset %d, got %d", i, tokens[i].text, expected.startOffset, tokens[i].startOffset)
				}
				if tokens[i].endOffset != expected.endOffset {
					t.Errorf("Token %d (%s): expected endOffset %d, got %d", i, tokens[i].text, expected.endOffset, tokens[i].endOffset)
				}
				if tokens[i].posInc != expected.posInc {
					t.Errorf("Token %d (%s): expected posInc %d, got %d", i, tokens[i].text, expected.posInc, tokens[i].posInc)
				}
			}
		})
	}
}

// TestWordDelimiterFilter_NoCaseChangeSplit tests with splitOnCaseChange=false.
// Source: TestWordDelimiterFilter.testNoCaseChangeSplit()
// Purpose: Tests that case changes don't cause splits when disabled.
func TestWordDelimiterFilter_NoCaseChangeSplit(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "CamelCase without split",
			input:    "PowerShot",
			expected: []string{"PowerShot"},
		},
		{
			name:     "Delimiter still splits",
			input:    "Power-Shot",
			expected: []string{"Power", "Shot"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			filter := NewWordDelimiterFilter(tokenizer, false, true, true, false)
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

// TestWordDelimiterFilter_NoNumericSplit tests with splitOnNumerics=false.
// Source: TestWordDelimiterFilter.testNoNumericSplit()
// Purpose: Tests that numeric transitions don't cause splits when disabled.
func TestWordDelimiterFilter_NoNumericSplit(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "j2se without split",
			input:    "j2se",
			expected: []string{"j2se"},
		},
		{
			name:     "Delimiter still splits",
			input:    "abc-123",
			expected: []string{"abc", "123"},
		},
		{
			name:     "Case change still splits",
			input:    "PowerShot2Test",
			expected: []string{"Power", "Shot2Test"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			filter := NewWordDelimiterFilter(tokenizer, true, false, true, false)
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

// TestWordDelimiterFilter_NoPossessiveStemming tests with stemEnglishPossessive=false.
// Source: TestWordDelimiterFilter.testNoPossessiveStemming()
// Purpose: Tests that possessives are not removed when disabled.
func TestWordDelimiterFilter_NoPossessiveStemming(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Possessive kept",
			input:    "John's",
			expected: []string{"John", "s"},
		},
		{
			name:     "O'Neil's kept",
			input:    "O'Neil's",
			expected: []string{"O", "Neil", "s"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			filter := NewWordDelimiterFilter(tokenizer, true, true, false, false)
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

// TestWordDelimiterFilter_Possessive tests English possessive handling.
// Source: TestWordDelimiterFilter.testPossessive()
// Purpose: Tests that trailing "'s" is removed.
func TestWordDelimiterFilter_Possessive(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Simple possessive",
			input:    "John's",
			expected: []string{"John"},
		},
		{
			name:     "Possessive with delimiter",
			input:    "O'Neil's",
			expected: []string{"O", "Neil"},
		},
		{
			name:     "Uppercase possessive",
			input:    "JOHN'S",
			expected: []string{"JOHN"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			filter := NewWordDelimiterFilter(tokenizer, true, true, true, false)
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

// TestWordDelimiterFilter_EmptyInput tests empty input handling.
// Source: TestWordDelimiterFilter.testEmptyInput()
// Purpose: Tests that empty input is handled correctly.
func TestWordDelimiterFilter_EmptyInput(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader(""))

	filter := NewWordDelimiterFilter(tokenizer, true, true, true, false)
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

// TestWordDelimiterFilter_Unicode tests Unicode handling.
// Source: TestWordDelimiterFilter.testUnicode()
// Purpose: Tests proper handling of Unicode characters.
func TestWordDelimiterFilter_Unicode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Accented characters",
			input:    "caféRésultat",
			expected: []string{"café", "Résultat"},
		},
		{
			name:     "Greek letters",
			input:    "ΑλφαΒήτα",
			expected: []string{"Αλφα", "Βήτα"},
		},
		{
			name:     "Cyrillic",
			input:    "ПриветМир",
			expected: []string{"Привет", "Мир"},
		},
		{
			name:     "Mixed Unicode and ASCII",
			input:    "café-Test",
			expected: []string{"café", "Test"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			filter := NewWordDelimiterFilter(tokenizer, true, true, true, false)
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

// TestWordDelimiterFilter_Complex tests complex real-world examples.
// Source: TestWordDelimiterFilter.testComplex()
// Purpose: Tests complex real-world tokenization scenarios.
func TestWordDelimiterFilter_Complex(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Java class name",
			input:    "ArrayIndexOutOfBoundsException",
			expected: []string{"Array", "Index", "Out", "Of", "Bounds", "Exception"},
		},
		{
			name:     "HTTP header style",
			input:    "X-Custom-Header-Value",
			expected: []string{"X", "Custom", "Header", "Value"},
		},
		{
			name:     "Snake case",
			input:    "this_is_snake_case",
			expected: []string{"this", "is", "snake", "case"},
		},
		{
			name:     "Product code",
			input:    "ABC-1234-XYZ",
			expected: []string{"ABC", "1234", "XYZ"},
		},
		{
			name:     "Mixed case and delimiters",
			input:    "XML-Parser_v2_Test",
			expected: []string{"XML", "Parser", "v", "2", "Test"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			filter := NewWordDelimiterFilter(tokenizer, true, true, true, false)
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

// TestWordDelimiterFilter_Chaining tests chaining with other filters.
// Source: TestWordDelimiterFilter.testChaining()
// Purpose: Tests that WordDelimiterFilter works properly in filter chains.
func TestWordDelimiterFilter_Chaining(t *testing.T) {
	input := "PowerShot Test-Case"

	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader(input))

	// Chain: WhitespaceTokenizer -> WordDelimiterFilter -> LowerCaseFilter
	wdFilter := NewWordDelimiterFilter(tokenizer, true, true, true, false)
	defer wdFilter.Close()
	lowerFilter := NewLowerCaseFilter(wdFilter)
	defer lowerFilter.Close()

	var tokens []string
	for {
		hasToken, err := lowerFilter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := lowerFilter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				tokens = append(tokens, termAttr.String())
			}
		}
	}

	expected := []string{"power", "shot", "test", "case"}
	if !reflect.DeepEqual(tokens, expected) {
		t.Errorf("Expected %v, got %v", expected, tokens)
	}
}

// TestWordDelimiterFilter_EndMethod tests the End() method.
// Source: TestWordDelimiterFilter.testEnd()
// Purpose: Tests that End() is properly propagated.
func TestWordDelimiterFilter_EndMethod(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("test"))

	filter := NewWordDelimiterFilter(tokenizer, true, true, true, false)
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

// TestWordDelimiterFilter_CustomTable tests with a custom character type table.
// Source: TestWordDelimiterFilter.testCustomTable()
// Purpose: Tests that custom character type tables work correctly.
func TestWordDelimiterFilter_CustomTable(t *testing.T) {
	// Create a custom table where '.' is treated as a delimiter
	customTable := make([]byte, 256)
	copy(customTable, DEFAULT_WORD_DELIM_TABLE)
	customTable['.'] = SUBWORD_DELIM

	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("hello.world"))

	filter := NewWordDelimiterFilterWithTable(tokenizer, customTable, true, true, true, false)
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

// TestWordDelimiterFilter_Factory tests the factory.
// Source: TestWordDelimiterFilter.testFactory()
// Purpose: Tests that the factory creates filters correctly.
func TestWordDelimiterFilter_Factory(t *testing.T) {
	factory := NewWordDelimiterFilterFactory(true, true, true, false)

	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("PowerShot"))

	filter := factory.Create(tokenizer).(*WordDelimiterFilter)
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

	expected := []string{"Power", "Shot"}
	if !reflect.DeepEqual(tokens, expected) {
		t.Errorf("Expected %v, got %v", expected, tokens)
	}
}

// TestWordDelimiterFilter_MultipleTokens tests multiple input tokens.
// Source: TestWordDelimiterFilter.testMultipleTokens()
// Purpose: Tests handling of multiple input tokens.
func TestWordDelimiterFilter_MultipleTokens(t *testing.T) {
	input := "PowerShot j2se Test-Case"

	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader(input))

	filter := NewWordDelimiterFilter(tokenizer, true, true, true, false)
	defer filter.Close()

	var tokens []string
	var posIncs []int
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
		if attr := filter.GetAttributeSource().GetAttribute("PositionIncrementAttribute"); attr != nil {
			if posAttr, ok := attr.(PositionIncrementAttribute); ok {
				posIncs = append(posIncs, posAttr.GetPositionIncrement())
			}
		}
	}

	expectedTokens := []string{"Power", "Shot", "j", "2", "se", "Test", "Case"}
	expectedPosIncs := []int{1, 0, 1, 0, 0, 1, 0}

	if !reflect.DeepEqual(tokens, expectedTokens) {
		t.Errorf("Expected tokens %v, got %v", expectedTokens, tokens)
	}
	if !reflect.DeepEqual(posIncs, expectedPosIncs) {
		t.Errorf("Expected position increments %v, got %v", expectedPosIncs, posIncs)
	}
}

// TestWordDelimiterFilter_SingleWordNotSplit tests that single words are not split.
// Source: TestWordDelimiterFilter.testSingleWordNotSplit()
// Purpose: Tests that single words without delimiters are passed through.
func TestWordDelimiterFilter_SingleWordNotSplit(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "All lowercase",
			input:    "hello",
			expected: []string{"hello"},
		},
		{
			name:     "All uppercase",
			input:    "HELLO",
			expected: []string{"HELLO"},
		},
		{
			name:     "Single char",
			input:    "a",
			expected: []string{"a"},
		},
		{
			name:     "Digits only",
			input:    "123",
			expected: []string{"123"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			filter := NewWordDelimiterFilter(tokenizer, true, true, true, false)
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

// TestWordDelimiterFilter_LeadingTrailingDelimiters tests handling of leading/trailing delimiters.
// Source: TestWordDelimiterFilter.testLeadingTrailingDelimiters()
// Purpose: Tests that leading and trailing delimiters are handled correctly.
func TestWordDelimiterFilter_LeadingTrailingDelimiters(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Leading delimiter",
			input:    "-hello",
			expected: []string{"hello"},
		},
		{
			name:     "Trailing delimiter",
			input:    "hello-",
			expected: []string{"hello"},
		},
		{
			name:     "Both delimiters",
			input:    "-hello-",
			expected: []string{"hello"},
		},
		{
			name:     "Multiple delimiters",
			input:    "--hello__world",
			expected: []string{"hello", "world"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			filter := NewWordDelimiterFilter(tokenizer, true, true, true, false)
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

// TestWordDelimiterFilter_OnlyDelimiters tests input with only delimiters.
// Source: TestWordDelimiterFilter.testOnlyDelimiters()
// Purpose: Tests that input with only delimiters produces no tokens.
func TestWordDelimiterFilter_OnlyDelimiters(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "Single delimiter",
			input: "-",
		},
		{
			name:  "Multiple delimiters",
			input: "-_-",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))

			filter := NewWordDelimiterFilter(tokenizer, true, true, true, false)
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
				t.Errorf("Expected 0 tokens for input %q, got %d", tc.input, tokenCount)
			}
		})
	}
}
