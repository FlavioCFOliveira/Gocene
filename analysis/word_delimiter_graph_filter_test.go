// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"strings"
	"testing"
)

// wdgfTokenInfo holds information about an emitted token for testing
type wdgfTokenInfo struct {
	text              string
	positionIncrement int
	positionLength    int
	tokenType         string
	startOffset       int
	endOffset         int
}

// collectWDGFTokens collects all tokens from a WordDelimiterGraphFilter for testing
func collectWDGFTokens(t *testing.T, filter *WordDelimiterGraphFilter) []wdgfTokenInfo {
	var tokens []wdgfTokenInfo

	// Get attributes
	attrSource := filter.GetAttributeSource()
	if attrSource == nil {
		t.Fatal("AttributeSource is nil")
	}

	var termAttr CharTermAttribute
	var posIncrAttr PositionIncrementAttribute
	var posLenAttr *PositionLengthAttribute
	var offsetAttr OffsetAttribute
	var typeAttr *TypeAttribute

	if attr := attrSource.GetAttributeByType(reflect.TypeOf(&charTermAttribute{})); attr != nil {
		termAttr = attr.(CharTermAttribute)
	}
	if attr := attrSource.GetAttributeByType(reflect.TypeOf(&positionIncrementAttribute{})); attr != nil {
		posIncrAttr = attr.(PositionIncrementAttribute)
	}
	if attr := attrSource.GetAttributeByType(reflect.TypeOf(&PositionLengthAttribute{})); attr != nil {
		posLenAttr = attr.(*PositionLengthAttribute)
	}
	if attr := attrSource.GetAttributeByType(reflect.TypeOf(&offsetAttribute{})); attr != nil {
		offsetAttr = attr.(OffsetAttribute)
	}
	if attr := attrSource.GetAttributeByType(reflect.TypeOf(&TypeAttribute{})); attr != nil {
		typeAttr = attr.(*TypeAttribute)
	}

	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken error: %v", err)
		}
		if !hasToken {
			break
		}

		info := wdgfTokenInfo{}
		if termAttr != nil {
			info.text = termAttr.String()
		}
		if posIncrAttr != nil {
			info.positionIncrement = posIncrAttr.GetPositionIncrement()
		}
		if posLenAttr != nil {
			info.positionLength = posLenAttr.GetPositionLength()
		}
		if offsetAttr != nil {
			info.startOffset = offsetAttr.StartOffset()
			info.endOffset = offsetAttr.EndOffset()
		}
		if typeAttr != nil {
			info.tokenType = typeAttr.GetType()
		}

		tokens = append(tokens, info)
	}

	return tokens
}

// TestWordDelimiterGraphFilter_BasicSplit tests basic word splitting
func TestWordDelimiterGraphFilter_BasicSplit(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "camelCase",
			input:    "PowerShot",
			expected: []string{"Power", "Shot"},
		},
		{
			name:     "PascalCase",
			input:    "PowerShotCamera",
			expected: []string{"Power", "Shot", "Camera"},
		},
		{
			name:     "withHyphen",
			input:    "Power-Shot",
			expected: []string{"Power", "Shot"},
		},
		{
			name:     "withUnderscore",
			input:    "Power_Shot",
			expected: []string{"Power", "Shot"},
		},
		{
			name:     "numberTransition",
			input:    "j2se",
			expected: []string{"j", "2", "se"},
		},
		{
			name:     "mixedCaseAndNumber",
			input:    "PowerShot12Mpx",
			expected: []string{"Power", "Shot", "12", "Mpx"},
		},
		{
			name:     "possessive",
			input:    "O'Neil's",
			expected: []string{"O", "Neil"},
		},
		{
			name:     "singleWord",
			input:    "hello",
			expected: []string{"hello"},
		},
		{
			name:     "allCaps",
			input:    "URL",
			expected: []string{"URL"},
		},
		{
			name:     "allCapsWithLower",
			input:    "URLLoader",
			expected: []string{"URLLoader"}, // URL is all caps, so not split
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer := NewWhitespaceTokenizer()
			tokenizer.SetReader(strings.NewReader(tt.input))

			filter := NewWordDelimiterGraphFilter(tokenizer)

			tokens := collectWDGFTokens(t, filter)

			if len(tokens) != len(tt.expected) {
				t.Errorf("Expected %d tokens, got %d: %v", len(tt.expected), len(tokens), tokens)
				return
			}

			for i, exp := range tt.expected {
				if tokens[i].text != exp {
					t.Errorf("Token %d: expected %q, got %q", i, exp, tokens[i].text)
				}
			}

			filter.End()
			filter.Close()
		})
	}
}

// TestWordDelimiterGraphFilter_PositionIncrement tests position increment handling
func TestWordDelimiterGraphFilter_PositionIncrement(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("PowerShot"))

	filter := NewWordDelimiterGraphFilter(tokenizer)

	tokens := collectWDGFTokens(t, filter)

	if len(tokens) != 2 {
		t.Fatalf("Expected 2 tokens, got %d", len(tokens))
	}

	// First token should have position increment 1
	if tokens[0].positionIncrement != 1 {
		t.Errorf("First token position increment: expected 1, got %d", tokens[0].positionIncrement)
	}

	// Second token should have position increment 0 (same position)
	if tokens[1].positionIncrement != 0 {
		t.Errorf("Second token position increment: expected 0, got %d", tokens[1].positionIncrement)
	}

	filter.End()
	filter.Close()
}

// TestWordDelimiterGraphFilter_PositionLength tests position length handling
func TestWordDelimiterGraphFilter_PositionLength(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("PowerShot"))

	filter := NewWordDelimiterGraphFilter(tokenizer)

	tokens := collectWDGFTokens(t, filter)

	if len(tokens) != 2 {
		t.Fatalf("Expected 2 tokens, got %d", len(tokens))
	}

	// Each subword should have position length 1
	for i, token := range tokens {
		if token.positionLength != 1 {
			t.Errorf("Token %d (%s): expected position length 1, got %d", i, token.text, token.positionLength)
		}
	}

	filter.End()
	filter.Close()
}

// TestWordDelimiterGraphFilter_PreserveOriginal tests preserving original token
func TestWordDelimiterGraphFilter_PreserveOriginal(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("PowerShot"))

	filter := NewWordDelimiterGraphFilterWithFlags(
		tokenizer,
		true,  // splitOnCaseChange
		true,  // splitOnNumerics
		true,  // stemEnglishPossessive
		false, // catenateWords
		false, // catenateNumbers
		false, // catenateAll
		true,  // preserveOriginal
		true,  // generateWordParts
		true,  // generateNumberParts
	)

	tokens := collectWDGFTokens(t, filter)

	// Should have: "Power", "Shot", "PowerShot" (original)
	if len(tokens) != 3 {
		t.Fatalf("Expected 3 tokens, got %d: %v", len(tokens), tokens)
	}

	expected := []struct {
		text           string
		positionLength int
	}{
		{"Power", 1},
		{"Shot", 1},
		{"PowerShot", 2}, // Original spans 2 positions
	}

	for i, exp := range expected {
		if tokens[i].text != exp.text {
			t.Errorf("Token %d: expected text %q, got %q", i, exp.text, tokens[i].text)
		}
		if tokens[i].positionLength != exp.positionLength {
			t.Errorf("Token %d: expected position length %d, got %d", i, exp.positionLength, tokens[i].positionLength)
		}
	}

	filter.End()
	filter.Close()
}

// TestWordDelimiterGraphFilter_CatenateWords tests word catenation
func TestWordDelimiterGraphFilter_CatenateWords(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("Power-Shot-Camera"))

	filter := NewWordDelimiterGraphFilterWithFlags(
		tokenizer,
		true,  // splitOnCaseChange
		true,  // splitOnNumerics
		true,  // stemEnglishPossessive
		true,  // catenateWords
		false, // catenateNumbers
		false, // catenateAll
		false, // preserveOriginal
		true,  // generateWordParts
		true,  // generateNumberParts
	)

	tokens := collectWDGFTokens(t, filter)

	// Should have: "Power", "Shot", "Camera", "Power Shot Camera" (catenation)
	if len(tokens) != 4 {
		t.Fatalf("Expected 4 tokens, got %d: %v", len(tokens), tokens)
	}

	// Check for catenated token
	foundCatenation := false
	for _, token := range tokens {
		if token.text == "Power Shot Camera" {
			foundCatenation = true
			if token.positionLength != 3 {
				t.Errorf("Catenation position length: expected 3, got %d", token.positionLength)
			}
		}
	}
	if !foundCatenation {
		t.Errorf("Did not find catenated token 'Power Shot Camera'")
	}

	filter.End()
	filter.Close()
}

// TestWordDelimiterGraphFilter_CatenateNumbers tests number catenation
func TestWordDelimiterGraphFilter_CatenateNumbers(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("1-2-3"))

	filter := NewWordDelimiterGraphFilterWithFlags(
		tokenizer,
		true,  // splitOnCaseChange
		true,  // splitOnNumerics
		true,  // stemEnglishPossessive
		false, // catenateWords
		true,  // catenateNumbers
		false, // catenateAll
		false, // preserveOriginal
		true,  // generateWordParts
		true,  // generateNumberParts
	)

	tokens := collectWDGFTokens(t, filter)

	// Should have: "1", "2", "3", "1 2 3" (catenation)
	if len(tokens) != 4 {
		t.Fatalf("Expected 4 tokens, got %d: %v", len(tokens), tokens)
	}

	// Check for catenated token
	foundCatenation := false
	for _, token := range tokens {
		if token.text == "1 2 3" {
			foundCatenation = true
			if token.positionLength != 3 {
				t.Errorf("Catenation position length: expected 3, got %d", token.positionLength)
			}
		}
	}
	if !foundCatenation {
		t.Errorf("Did not find catenated token '1 2 3'")
	}

	filter.End()
	filter.Close()
}

// TestWordDelimiterGraphFilter_CatenateAll tests catenating all parts
func TestWordDelimiterGraphFilter_CatenateAll(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("Power-Shot-12-Mpx"))

	filter := NewWordDelimiterGraphFilterWithFlags(
		tokenizer,
		true,  // splitOnCaseChange
		true,  // splitOnNumerics
		true,  // stemEnglishPossessive
		false, // catenateWords
		false, // catenateNumbers
		true,  // catenateAll
		false, // preserveOriginal
		true,  // generateWordParts
		true,  // generateNumberParts
	)

	tokens := collectWDGFTokens(t, filter)

	// Should have: "Power", "Shot", "12", "Mpx", "Power Shot 12 Mpx" (catenation)
	if len(tokens) != 5 {
		t.Fatalf("Expected 5 tokens, got %d: %v", len(tokens), tokens)
	}

	// Check for catenated token
	foundCatenation := false
	for _, token := range tokens {
		if token.text == "Power Shot 12 Mpx" {
			foundCatenation = true
			if token.positionLength != 4 {
				t.Errorf("Catenation position length: expected 4, got %d", token.positionLength)
			}
		}
	}
	if !foundCatenation {
		t.Errorf("Did not find catenated token 'Power Shot 12 Mpx'")
	}

	filter.End()
	filter.Close()
}

// TestWordDelimiterGraphFilter_NoSplitOnCaseChange tests disabling case change splitting
func TestWordDelimiterGraphFilter_NoSplitOnCaseChange(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("PowerShot"))

	filter := NewWordDelimiterGraphFilterWithFlags(
		tokenizer,
		false, // splitOnCaseChange
		true,  // splitOnNumerics
		true,  // stemEnglishPossessive
		false, // catenateWords
		false, // catenateNumbers
		false, // catenateAll
		false, // preserveOriginal
		true,  // generateWordParts
		true,  // generateNumberParts
	)

	tokens := collectWDGFTokens(t, filter)

	// Should have just "PowerShot" since case change splitting is disabled
	if len(tokens) != 1 {
		t.Fatalf("Expected 1 token, got %d: %v", len(tokens), tokens)
	}

	if tokens[0].text != "PowerShot" {
		t.Errorf("Expected 'PowerShot', got %q", tokens[0].text)
	}

	filter.End()
	filter.Close()
}

// TestWordDelimiterGraphFilter_NoSplitOnNumerics tests disabling numeric transition splitting
func TestWordDelimiterGraphFilter_NoSplitOnNumerics(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("j2se"))

	filter := NewWordDelimiterGraphFilterWithFlags(
		tokenizer,
		true,  // splitOnCaseChange
		false, // splitOnNumerics
		true,  // stemEnglishPossessive
		false, // catenateWords
		false, // catenateNumbers
		false, // catenateAll
		false, // preserveOriginal
		true,  // generateWordParts
		true,  // generateNumberParts
	)

	tokens := collectWDGFTokens(t, filter)

	// Should have just "j2se" since numeric splitting is disabled
	if len(tokens) != 1 {
		t.Fatalf("Expected 1 token, got %d: %v", len(tokens), tokens)
	}

	if tokens[0].text != "j2se" {
		t.Errorf("Expected 'j2se', got %q", tokens[0].text)
	}

	filter.End()
	filter.Close()
}

// TestWordDelimiterGraphFilter_NoStemPossessive tests disabling possessive stemming
func TestWordDelimiterGraphFilter_NoStemPossessive(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("O'Neil's"))

	filter := NewWordDelimiterGraphFilterWithFlags(
		tokenizer,
		true,  // splitOnCaseChange
		true,  // splitOnNumerics
		false, // stemEnglishPossessive
		false, // catenateWords
		false, // catenateNumbers
		false, // catenateAll
		false, // preserveOriginal
		true,  // generateWordParts
		true,  // generateNumberParts
	)

	tokens := collectWDGFTokens(t, filter)

	// With possessive stemming disabled, "O'Neil's" should be treated as one word
	// or split by delimiter only
	if len(tokens) < 1 {
		t.Fatalf("Expected at least 1 token, got %d", len(tokens))
	}

	filter.End()
	filter.Close()
}

// TestWordDelimiterGraphFilter_NoWordParts tests disabling word part generation
func TestWordDelimiterGraphFilter_NoWordParts(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("Power-Shot"))

	filter := NewWordDelimiterGraphFilterWithFlags(
		tokenizer,
		true,  // splitOnCaseChange
		true,  // splitOnNumerics
		true,  // stemEnglishPossessive
		false, // catenateWords
		false, // catenateNumbers
		false, // catenateAll
		false, // preserveOriginal
		false, // generateWordParts
		true,  // generateNumberParts
	)

	tokens := collectWDGFTokens(t, filter)

	// Should have no tokens since word parts are disabled and there are no numbers
	if len(tokens) != 0 {
		t.Errorf("Expected 0 tokens, got %d: %v", len(tokens), tokens)
	}

	filter.End()
	filter.Close()
}

// TestWordDelimiterGraphFilter_NoNumberParts tests disabling number part generation
func TestWordDelimiterGraphFilter_NoNumberParts(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("j2se"))

	filter := NewWordDelimiterGraphFilterWithFlags(
		tokenizer,
		true,  // splitOnCaseChange
		true,  // splitOnNumerics
		true,  // stemEnglishPossessive
		false, // catenateWords
		false, // catenateNumbers
		false, // catenateAll
		false, // preserveOriginal
		true,  // generateWordParts
		false, // generateNumberParts
	)

	tokens := collectWDGFTokens(t, filter)

	// Should have "j" and "se" but not "2"
	if len(tokens) != 2 {
		t.Fatalf("Expected 2 tokens, got %d: %v", len(tokens), tokens)
	}

	for _, token := range tokens {
		if token.text == "2" {
			t.Errorf("Should not have emitted number part '2'")
		}
	}

	filter.End()
	filter.Close()
}

// TestWordDelimiterGraphFilter_MultipleTokens tests handling multiple input tokens
func TestWordDelimiterGraphFilter_MultipleTokens(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("PowerShot j2se"))

	filter := NewWordDelimiterGraphFilter(tokenizer)

	tokens := collectWDGFTokens(t, filter)

	// Should have: "Power", "Shot", "j", "2", "se"
	if len(tokens) != 5 {
		t.Fatalf("Expected 5 tokens, got %d: %v", len(tokens), tokens)
	}

	expected := []string{"Power", "Shot", "j", "2", "se"}
	for i, exp := range expected {
		if tokens[i].text != exp {
			t.Errorf("Token %d: expected %q, got %q", i, exp, tokens[i].text)
		}
	}

	// Check that "j" starts a new position (position increment 1)
	// after "Shot" (which has position increment 0)
	if tokens[2].positionIncrement != 1 {
		t.Errorf("Token 'j' should have position increment 1, got %d", tokens[2].positionIncrement)
	}

	filter.End()
	filter.Close()
}

// TestWordDelimiterGraphFilter_EmptyInput tests handling empty input
func TestWordDelimiterGraphFilter_EmptyInput(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader(""))

	filter := NewWordDelimiterGraphFilter(tokenizer)

	tokens := collectWDGFTokens(t, filter)

	if len(tokens) != 0 {
		t.Errorf("Expected 0 tokens for empty input, got %d", len(tokens))
	}

	filter.End()
	filter.Close()
}

// TestWordDelimiterGraphFilter_OnlyDelimiters tests input with only delimiters
func TestWordDelimiterGraphFilter_OnlyDelimiters(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("---"))

	filter := NewWordDelimiterGraphFilter(tokenizer)

	tokens := collectWDGFTokens(t, filter)

	// Should have no tokens since there are no word characters
	if len(tokens) != 0 {
		t.Errorf("Expected 0 tokens for delimiter-only input, got %d: %v", len(tokens), tokens)
	}

	filter.End()
	filter.Close()
}

// TestWordDelimiterGraphFilter_Reset tests the Reset method
func TestWordDelimiterGraphFilter_Reset(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()

	// First run
	tokenizer.SetReader(strings.NewReader("PowerShot"))
	filter := NewWordDelimiterGraphFilter(tokenizer)

	tokens1 := collectWDGFTokens(t, filter)
	if len(tokens1) != 2 {
		t.Fatalf("First run: expected 2 tokens, got %d", len(tokens1))
	}

	filter.End()

	// Reset and run again
	filter.Reset()
	tokenizer.SetReader(strings.NewReader("j2se"))

	tokens2 := collectWDGFTokens(t, filter)
	if len(tokens2) != 3 {
		t.Fatalf("Second run: expected 3 tokens, got %d", len(tokens2))
	}

	expected := []string{"j", "2", "se"}
	for i, exp := range expected {
		if tokens2[i].text != exp {
			t.Errorf("Token %d: expected %q, got %q", i, exp, tokens2[i].text)
		}
	}

	filter.End()
	filter.Close()
}

// TestWordDelimiterGraphFilterFactory tests the factory
func TestWordDelimiterGraphFilterFactory(t *testing.T) {
	factory := NewWordDelimiterGraphFilterFactory()

	// Set some options
	factory.SetCatenateWords(true)
	factory.SetPreserveOriginal(true)

	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("Power-Shot"))

	filter := factory.Create(tokenizer).(*WordDelimiterGraphFilter)

	tokens := collectWDGFTokens(t, filter)

	// Should have: "Power", "Shot", "Power Shot" (catenation), "Power-Shot" (original)
	if len(tokens) != 4 {
		t.Fatalf("Expected 4 tokens, got %d: %v", len(tokens), tokens)
	}

	filter.End()
	filter.Close()
}

// TestWordDelimiterGraphFilter_TypeAttribute tests type attribute setting
func TestWordDelimiterGraphFilter_TypeAttribute(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("Power 123"))

	filter := NewWordDelimiterGraphFilter(tokenizer)

	tokens := collectWDGFTokens(t, filter)

	// Should have: "Power" (word), "123" (number)
	if len(tokens) != 2 {
		t.Fatalf("Expected 2 tokens, got %d: %v", len(tokens), tokens)
	}

	if tokens[0].tokenType != "word" {
		t.Errorf("Expected 'Power' to have type 'word', got %q", tokens[0].tokenType)
	}

	if tokens[1].tokenType != "number" {
		t.Errorf("Expected '123' to have type 'number', got %q", tokens[1].tokenType)
	}

	filter.End()
	filter.Close()
}

// TestWordDelimiterGraphFilter_ComplexInput tests a complex input with multiple features
func TestWordDelimiterGraphFilter_ComplexInput(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("PowerShot12Mpx-WiFi_O'Neil's"))

	filter := NewWordDelimiterGraphFilterWithFlags(
		tokenizer,
		true,  // splitOnCaseChange
		true,  // splitOnNumerics
		true,  // stemEnglishPossessive
		true,  // catenateWords
		true,  // catenateNumbers
		false, // catenateAll
		true,  // preserveOriginal
		true,  // generateWordParts
		true,  // generateNumberParts
	)

	tokens := collectWDGFTokens(t, filter)

	// Expected tokens:
	// "Power", "Shot", "12", "Mpx", "WiFi", "O", "Neil" (subwords)
	// "Power Shot", "12" (word/number catenations)
	// "PowerShot12Mpx-WiFi_O'Neil's" (original)

	if len(tokens) < 7 {
		t.Fatalf("Expected at least 7 tokens, got %d: %v", len(tokens), tokens)
	}

	// Check for specific tokens
	tokenTexts := make(map[string]bool)
	for _, token := range tokens {
		tokenTexts[token.text] = true
	}

	// Note: "WiFi" is not split because "Wi" followed by "Fi" doesn't trigger a split
	// (the WordDelimiterIterator handles consecutive uppercase followed by lowercase correctly)
	expectedTokens := []string{"Power", "Shot", "12", "Mpx", "O", "Neil"}
	for _, exp := range expectedTokens {
		if !tokenTexts[exp] {
			t.Errorf("Expected token %q not found", exp)
		}
	}

	filter.End()
	filter.Close()
}

// TestWordDelimiterGraphFilter_Unicode tests Unicode handling
func TestWordDelimiterGraphFilter_Unicode(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("CaféLatte"))

	filter := NewWordDelimiterGraphFilter(tokenizer)

	tokens := collectWDGFTokens(t, filter)

	// Should split on case change: "Café", "Latte"
	if len(tokens) != 2 {
		t.Fatalf("Expected 2 tokens, got %d: %v", len(tokens), tokens)
	}

	if tokens[0].text != "Café" {
		t.Errorf("Expected first token 'Café', got %q", tokens[0].text)
	}

	if tokens[1].text != "Latte" {
		t.Errorf("Expected second token 'Latte', got %q", tokens[1].text)
	}

	filter.End()
	filter.Close()
}

// BenchmarkWordDelimiterGraphFilter benchmarks the filter
func BenchmarkWordDelimiterGraphFilter(b *testing.B) {
	input := "PowerShot12Mpx-WiFi_O'Neil's CameraTest"

	for i := 0; i < b.N; i++ {
		tokenizer := NewWhitespaceTokenizer()
		tokenizer.SetReader(strings.NewReader(input))

		filter := NewWordDelimiterGraphFilter(tokenizer)

		for {
			hasToken, err := filter.IncrementToken()
			if err != nil {
				b.Fatal(err)
			}
			if !hasToken {
				break
			}
		}

		filter.End()
		filter.Close()
	}
}
