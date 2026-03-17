// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"testing"
)

// TestWordDelimiterIterator_Basic tests basic word delimiter iteration.
// Source: TestWordDelimiterIterator.testBasicIteration()
// Purpose: Tests that words are split correctly based on case changes and delimiters.
func TestWordDelimiterIterator_Basic(t *testing.T) {
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
			name:     "Mixed delimiters",
			input:    "Power_Shot-Test",
			expected: []string{"Power", "Shot", "Test"},
		},
		{
			name:     "Numeric transition",
			input:    "j2se",
			expected: []string{"j", "2", "se"},
		},
		{
			name:     "Multiple numbers",
			input:    "abc123def456",
			expected: []string{"abc", "123", "def", "456"},
		},
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
			name:     "Multiple delimiters",
			input:    "--hello__world",
			expected: []string{"hello", "world"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			iter := NewWordDelimiterIterator(true, true, true)
			runes := []rune(tc.input)
			iter.SetText(runes, len(runes))

			var result []string
			for iter.Next() != DONE {
				result = append(result, iter.GetCurrentSubword())
			}

			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

// TestWordDelimiterIterator_CaseChanges tests case change handling.
// Source: TestWordDelimiterIterator.testCaseChanges()
// Purpose: Tests that case changes are handled correctly.
func TestWordDelimiterIterator_CaseChanges(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Lower to upper (camelCase)",
			input:    "camelCase",
			expected: []string{"camel", "Case"},
		},
		{
			name:     "Upper to lower (PascalCase)",
			input:    "PascalCase",
			expected: []string{"Pascal", "Case"},
		},
		{
			name:     "All caps",
			input:    "HELLO",
			expected: []string{"HELLO"},
		},
		{
			name:     "All lower",
			input:    "hello",
			expected: []string{"hello"},
		},
		{
			name:     "Mixed case with transition",
			input:    "XMLParser",
			expected: []string{"XMLParser"}, // Consecutive UPPER followed by LOWER doesn't split
		},
		{
			name:     "Single upper in lower",
			input:    "testAValue",
			expected: []string{"test", "AValue"}, // LOWER to UPPER transition splits
		},
		{
			name:     "URL pattern",
			input:    "URLLoader",
			expected: []string{"URLLoader"}, // Consecutive UPPER followed by LOWER doesn't split
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			iter := NewWordDelimiterIterator(true, true, true)
			runes := []rune(tc.input)
			iter.SetText(runes, len(runes))

			var result []string
			for iter.Next() != DONE {
				result = append(result, iter.GetCurrentSubword())
			}

			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

// TestWordDelimiterIterator_Possessive tests English possessive handling.
// Source: TestWordDelimiterIterator.testPossessive()
// Purpose: Tests that trailing "'s" is removed.
func TestWordDelimiterIterator_Possessive(t *testing.T) {
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
		{
			name:     "Multiple words with possessive",
			input:    "John's car",
			expected: []string{"John", "car"},
		},
		{
			name:     "Apostrophe not possessive",
			input:    "don't",
			expected: []string{"don", "t"},
		},
		{
			name:     "O'Brien",
			input:    "O'Brien",
			expected: []string{"O", "Brien"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			iter := NewWordDelimiterIterator(true, true, true)
			runes := []rune(tc.input)
			iter.SetText(runes, len(runes))

			var result []string
			for iter.Next() != DONE {
				result = append(result, iter.GetCurrentSubword())
			}

			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

// TestWordDelimiterIterator_NoCaseChangeSplit tests with splitOnCaseChange=false.
// Source: TestWordDelimiterIterator.testNoCaseChangeSplit()
// Purpose: Tests that case changes don't cause splits when disabled.
func TestWordDelimiterIterator_NoCaseChangeSplit(t *testing.T) {
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
		{
			name:     "Mixed with delimiter",
			input:    "PowerShot-TestCase",
			expected: []string{"PowerShot", "TestCase"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			iter := NewWordDelimiterIterator(false, true, true)
			runes := []rune(tc.input)
			iter.SetText(runes, len(runes))

			var result []string
			for iter.Next() != DONE {
				result = append(result, iter.GetCurrentSubword())
			}

			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

// TestWordDelimiterIterator_NoNumericSplit tests with splitOnNumerics=false.
// Source: TestWordDelimiterIterator.testNoNumericSplit()
// Purpose: Tests that numeric transitions don't cause splits when disabled.
func TestWordDelimiterIterator_NoNumericSplit(t *testing.T) {
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
			name:     "abc123 without split",
			input:    "abc123def",
			expected: []string{"abc123def"},
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
			iter := NewWordDelimiterIterator(true, false, true)
			runes := []rune(tc.input)
			iter.SetText(runes, len(runes))

			var result []string
			for iter.Next() != DONE {
				result = append(result, iter.GetCurrentSubword())
			}

			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

// TestWordDelimiterIterator_NoPossessiveStemming tests with stemEnglishPossessive=false.
// Source: TestWordDelimiterIterator.testNoPossessiveStemming()
// Purpose: Tests that possessives are not removed when disabled.
func TestWordDelimiterIterator_NoPossessiveStemming(t *testing.T) {
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
			iter := NewWordDelimiterIterator(true, true, false)
			runes := []rune(tc.input)
			iter.SetText(runes, len(runes))

			var result []string
			for iter.Next() != DONE {
				result = append(result, iter.GetCurrentSubword())
			}

			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

// TestWordDelimiterIterator_IsSingleWord tests the IsSingleWord method.
// Source: TestWordDelimiterIterator.testIsSingleWord()
// Purpose: Tests detection of single-word tokens.
func TestWordDelimiterIterator_IsSingleWord(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedSingle bool
	}{
		{
			name:           "Single word",
			input:          "hello",
			expectedSingle: true,
		},
		{
			name:           "Single word with delimiters",
			input:          "-hello-",
			expectedSingle: true,
		},
		{
			name:           "Multiple words",
			input:          "hello-world",
			expectedSingle: false,
		},
		{
			name:           "CamelCase",
			input:          "PowerShot",
			expectedSingle: false,
		},
		{
			name:           "Possessive single word",
			input:          "John's",
			expectedSingle: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			iter := NewWordDelimiterIterator(true, true, true)
			runes := []rune(tc.input)
			iter.SetText(runes, len(runes))

			// Iterate to the first (and possibly only) subword
			iter.Next()

			if iter.IsSingleWord() != tc.expectedSingle {
				t.Errorf("Expected IsSingleWord()=%v for input %q", tc.expectedSingle, tc.input)
			}
		})
	}
}

// TestWordDelimiterIterator_Type tests the Type method.
// Source: TestWordDelimiterIterator.testType()
// Purpose: Tests that character types are correctly identified.
func TestWordDelimiterIterator_Type(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedType int
	}{
		{
			name:         "Lowercase word",
			input:        "hello",
			expectedType: ALPHA,
		},
		{
			name:         "Uppercase word",
			input:        "HELLO",
			expectedType: ALPHA,
		},
		{
			name:         "Digits",
			input:        "123",
			expectedType: DIGIT,
		},
		{
			name:         "Mixed alphanumeric",
			input:        "abc123",
			expectedType: ALPHA,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			iter := NewWordDelimiterIterator(true, true, true)
			runes := []rune(tc.input)
			iter.SetText(runes, len(runes))

			iter.Next()

			if iter.Type() != tc.expectedType {
				t.Errorf("Expected type %d, got %d", tc.expectedType, iter.Type())
			}
		})
	}
}

// TestWordDelimiterIterator_Unicode tests Unicode handling.
// Source: TestWordDelimiterIterator.testUnicode()
// Purpose: Tests proper handling of Unicode characters.
func TestWordDelimiterIterator_Unicode(t *testing.T) {
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
			name:     "Japanese (no case change)",
			input:    "テスト文字",
			expected: []string{"テスト文字"},
		},
		{
			name:     "Mixed Unicode and ASCII",
			input:    "café-Test",
			expected: []string{"café", "Test"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			iter := NewWordDelimiterIterator(true, true, true)
			runes := []rune(tc.input)
			iter.SetText(runes, len(runes))

			var result []string
			for iter.Next() != DONE {
				result = append(result, iter.GetCurrentSubword())
			}

			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

// TestWordDelimiterIterator_EmptyInput tests empty input handling.
// Source: TestWordDelimiterIterator.testEmptyInput()
// Purpose: Tests that empty input is handled correctly.
func TestWordDelimiterIterator_EmptyInput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "Only delimiters",
			input:    "-_-",
			expected: nil,
		},
		{
			name:     "Single delimiter",
			input:    "-",
			expected: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			iter := NewWordDelimiterIterator(true, true, true)
			runes := []rune(tc.input)
			iter.SetText(runes, len(runes))

			var result []string
			for iter.Next() != DONE {
				result = append(result, iter.GetCurrentSubword())
			}

			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

// TestWordDelimiterIterator_Reset tests iterator reset functionality.
// Source: TestWordDelimiterIterator.testReset()
// Purpose: Tests that the iterator can be reused with new text.
func TestWordDelimiterIterator_Reset(t *testing.T) {
	iter := NewWordDelimiterIterator(true, true, true)

	// First text
	runes1 := []rune("hello-world")
	iter.SetText(runes1, len(runes1))

	var result1 []string
	for iter.Next() != DONE {
		result1 = append(result1, iter.GetCurrentSubword())
	}

	expected1 := []string{"hello", "world"}
	if !reflect.DeepEqual(result1, expected1) {
		t.Errorf("First run: expected %v, got %v", expected1, result1)
	}

	// Second text (reuse iterator)
	runes2 := []rune("foo-bar-baz")
	iter.SetText(runes2, len(runes2))

	var result2 []string
	for iter.Next() != DONE {
		result2 = append(result2, iter.GetCurrentSubword())
	}

	expected2 := []string{"foo", "bar", "baz"}
	if !reflect.DeepEqual(result2, expected2) {
		t.Errorf("Second run: expected %v, got %v", expected2, result2)
	}
}

// TestWordDelimiterIterator_Bounds tests the bounds methods.
// Source: TestWordDelimiterIterator.testBounds()
// Purpose: Tests that bounds are correctly calculated.
func TestWordDelimiterIterator_Bounds(t *testing.T) {
	tests := []struct {
		name              string
		input             string
		expectedStart     int
		expectedEnd       int
		expectedSubwords  []string
	}{
		{
			name:             "No leading/trailing delimiters",
			input:            "hello-world",
			expectedStart:    0,
			expectedEnd:      11,
			expectedSubwords: []string{"hello", "world"},
		},
		{
			name:             "With leading delimiter",
			input:            "-hello-world",
			expectedStart:    1,
			expectedEnd:      12,
			expectedSubwords: []string{"hello", "world"},
		},
		{
			name:             "With trailing delimiter",
			input:            "hello-world-",
			expectedStart:    0,
			expectedEnd:      11,
			expectedSubwords: []string{"hello", "world"},
		},
		{
			name:             "With both delimiters",
			input:            "-hello-world-",
			expectedStart:    1,
			expectedEnd:      12,
			expectedSubwords: []string{"hello", "world"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			iter := NewWordDelimiterIterator(true, true, true)
			runes := []rune(tc.input)
			iter.SetText(runes, len(runes))

			if iter.GetStartBounds() != tc.expectedStart {
				t.Errorf("Expected startBounds %d, got %d", tc.expectedStart, iter.GetStartBounds())
			}

			if iter.GetEndBounds() != tc.expectedEnd {
				t.Errorf("Expected endBounds %d, got %d", tc.expectedEnd, iter.GetEndBounds())
			}

			var result []string
			for iter.Next() != DONE {
				result = append(result, iter.GetCurrentSubword())
			}

			if !reflect.DeepEqual(result, tc.expectedSubwords) {
				t.Errorf("Expected subwords %v, got %v", tc.expectedSubwords, result)
			}
		})
	}
}

// TestWordDelimiterIterator_CustomTable tests with a custom character type table.
// Source: TestWordDelimiterIterator.testCustomTable()
// Purpose: Tests that custom character type tables work correctly.
func TestWordDelimiterIterator_CustomTable(t *testing.T) {
	// Create a custom table where '.' is treated as a delimiter
	customTable := make([]byte, 256)
	copy(customTable, DEFAULT_WORD_DELIM_TABLE)
	customTable['.'] = SUBWORD_DELIM

	iter := NewWordDelimiterIteratorWithTable(customTable, true, true, true)
	runes := []rune("hello.world")
	iter.SetText(runes, len(runes))

	var result []string
	for iter.Next() != DONE {
		result = append(result, iter.GetCurrentSubword())
	}

	expected := []string{"hello", "world"}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

// TestWordDelimiterIterator_TypeChecks tests the type check functions.
// Source: TestWordDelimiterIterator.testTypeChecks()
// Purpose: Tests the IsAlpha, IsDigit, IsUpper, IsLower, IsSubwordDelim functions.
func TestWordDelimiterIterator_TypeChecks(t *testing.T) {
	tests := []struct {
		name     string
		type_    int
		isAlpha  bool
		isDigit  bool
		isUpper  bool
		isLower  bool
		isDelim  bool
	}{
		{
			name:    "LOWER only",
			type_:   LOWER,
			isAlpha: true,
			isDigit: false,
			isUpper: false,
			isLower: true,
			isDelim: false,
		},
		{
			name:    "UPPER only",
			type_:   UPPER,
			isAlpha: true,
			isDigit: false,
			isUpper: true,
			isLower: false,
			isDelim: false,
		},
		{
			name:    "DIGIT only",
			type_:   DIGIT,
			isAlpha: false,
			isDigit: true,
			isUpper: false,
			isLower: false,
			isDelim: false,
		},
		{
			name:    "SUBWORD_DELIM",
			type_:   SUBWORD_DELIM,
			isAlpha: false,
			isDigit: false,
			isUpper: false,
			isLower: false,
			isDelim: true,
		},
		{
			name:    "ALPHA (LOWER|UPPER)",
			type_:   ALPHA,
			isAlpha: true,
			isDigit: false,
			isUpper: true,  // ALPHA contains UPPER bit
			isLower: true,  // ALPHA contains LOWER bit
			isDelim: false,
		},
		{
			name:    "ALPHANUM",
			type_:   ALPHANUM,
			isAlpha: true,
			isDigit: true,
			isUpper: true,  // ALPHANUM contains UPPER bit
			isLower: true,  // ALPHANUM contains LOWER bit
			isDelim: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if IsAlpha(tc.type_) != tc.isAlpha {
				t.Errorf("IsAlpha(%d): expected %v, got %v", tc.type_, tc.isAlpha, IsAlpha(tc.type_))
			}
			if IsDigit(tc.type_) != tc.isDigit {
				t.Errorf("IsDigit(%d): expected %v, got %v", tc.type_, tc.isDigit, IsDigit(tc.type_))
			}
			if IsUpper(tc.type_) != tc.isUpper {
				t.Errorf("IsUpper(%d): expected %v, got %v", tc.type_, tc.isUpper, IsUpper(tc.type_))
			}
			if IsLower(tc.type_) != tc.isLower {
				t.Errorf("IsLower(%d): expected %v, got %v", tc.type_, tc.isLower, IsLower(tc.type_))
			}
			if IsSubwordDelim(tc.type_) != tc.isDelim {
				t.Errorf("IsSubwordDelim(%d): expected %v, got %v", tc.type_, tc.isDelim, IsSubwordDelim(tc.type_))
			}
		})
	}
}

// TestWordDelimiterIterator_GetWordDelimiterType tests the GetWordDelimiterType function.
// Source: TestWordDelimiterIterator.testGetWordDelimiterType()
// Purpose: Tests character type detection for various Unicode characters.
func TestWordDelimiterIterator_GetWordDelimiterType(t *testing.T) {
	tests := []struct {
		char rune
		want byte
	}{
		{'a', LOWER},
		{'A', UPPER},
		{'1', DIGIT},
		{'-', SUBWORD_DELIM},
		{'_', SUBWORD_DELIM},
		{' ', SUBWORD_DELIM},
		{'é', LOWER}, // Lowercase accented
		{'É', UPPER}, // Uppercase accented
		{'α', LOWER}, // Greek lowercase
		{'Α', UPPER}, // Greek uppercase
	}

	for _, tc := range tests {
		t.Run(string(tc.char), func(t *testing.T) {
			got := GetWordDelimiterType(tc.char)
			if got != tc.want {
				t.Errorf("GetWordDelimiterType(%q) = %d, want %d", tc.char, got, tc.want)
			}
		})
	}
}

// TestWordDelimiterIterator_Complex tests complex real-world examples.
// Source: TestWordDelimiterIterator.testComplex()
// Purpose: Tests complex real-world tokenization scenarios.
func TestWordDelimiterIterator_Complex(t *testing.T) {
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
			name:     "Mixed case and numbers",
			input:    "IPv6AddressParser",
			expected: []string{"IPv", "6", "Address", "Parser"}, // Lower-to-digit transition splits
		},
		{
			name:     "File name",
			input:    "my_file_v2_backup",
			expected: []string{"my", "file", "v", "2", "backup"}, // Lower-to-digit transition splits
		},
		{
			name:     "Product code",
			input:    "ABC-1234-XYZ",
			expected: []string{"ABC", "1234", "XYZ"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			iter := NewWordDelimiterIterator(true, true, true)
			runes := []rune(tc.input)
			iter.SetText(runes, len(runes))

			var result []string
			for iter.Next() != DONE {
				result = append(result, iter.GetCurrentSubword())
			}

			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

// TestWordDelimiterIterator_CurrentAndEnd tests the Current and End methods.
// Source: TestWordDelimiterIterator.testCurrentAndEnd()
// Purpose: Tests that Current and End return correct positions.
func TestWordDelimiterIterator_CurrentAndEnd(t *testing.T) {
	iter := NewWordDelimiterIterator(true, true, true)
	runes := []rune("hello-world-test")
	iter.SetText(runes, len(runes))

	// First subword: "hello" [0:5]
	iter.Next()
	if iter.Current() != 0 {
		t.Errorf("Expected Current()=0, got %d", iter.Current())
	}
	if iter.End() != 5 {
		t.Errorf("Expected End()=5, got %d", iter.End())
	}

	// Second subword: "world" [6:11]
	iter.Next()
	if iter.Current() != 6 {
		t.Errorf("Expected Current()=6, got %d", iter.Current())
	}
	if iter.End() != 11 {
		t.Errorf("Expected End()=11, got %d", iter.End())
	}

	// Third subword: "test" [12:16]
	iter.Next()
	if iter.Current() != 12 {
		t.Errorf("Expected Current()=12, got %d", iter.Current())
	}
	if iter.End() != 16 {
		t.Errorf("Expected End()=16, got %d", iter.End())
	}

	// No more subwords
	if iter.Next() != DONE {
		t.Error("Expected DONE after last subword")
	}
}

// TestWordDelimiterIterator_GetText tests the GetText method.
// Source: TestWordDelimiterIterator.testGetText()
// Purpose: Tests that GetText returns the original text.
func TestWordDelimiterIterator_GetText(t *testing.T) {
	input := "hello-world"
	iter := NewWordDelimiterIterator(true, true, true)
	runes := []rune(input)
	iter.SetText(runes, len(runes))

	got := iter.GetText()
	if !reflect.DeepEqual(got, runes) {
		t.Errorf("GetText() returned %v, expected %v", got, runes)
	}
}

// TestWordDelimiterIterator_DefaultTable tests the default word delimiter table.
// Source: TestWordDelimiterIterator.testDefaultTable()
// Purpose: Tests that the default table is correctly initialized.
func TestWordDelimiterIterator_DefaultTable(t *testing.T) {
	if len(DEFAULT_WORD_DELIM_TABLE) != 256 {
		t.Errorf("Expected DEFAULT_WORD_DELIM_TABLE length 256, got %d", len(DEFAULT_WORD_DELIM_TABLE))
	}

	// Test a few specific characters
	if DEFAULT_WORD_DELIM_TABLE['a'] != LOWER {
		t.Errorf("Expected 'a' to be LOWER, got %d", DEFAULT_WORD_DELIM_TABLE['a'])
	}
	if DEFAULT_WORD_DELIM_TABLE['A'] != UPPER {
		t.Errorf("Expected 'A' to be UPPER, got %d", DEFAULT_WORD_DELIM_TABLE['A'])
	}
	if DEFAULT_WORD_DELIM_TABLE['1'] != DIGIT {
		t.Errorf("Expected '1' to be DIGIT, got %d", DEFAULT_WORD_DELIM_TABLE['1'])
	}
	if DEFAULT_WORD_DELIM_TABLE['-'] != SUBWORD_DELIM {
		t.Errorf("Expected '-' to be SUBWORD_DELIM, got %d", DEFAULT_WORD_DELIM_TABLE['-'])
	}
}
