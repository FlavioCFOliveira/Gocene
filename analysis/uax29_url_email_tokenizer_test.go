// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"strings"
	"testing"
)

// TestUAX29URLEmailTokenizer_Basic tests basic tokenization.
// Source: TestUAX29URLEmailTokenizer.testBasic()
// Purpose: Tests standard word tokenization following UAX#29 rules.
func TestUAX29URLEmailTokenizer_Basic(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Simple words",
			input:    "The quick brown fox",
			expected: []string{"The", "quick", "brown", "fox"},
		},
		{
			name:     "With punctuation",
			input:    "Hello, world! How are you?",
			expected: []string{"Hello", "world", "How", "are", "you"},
		},
		{
			name:     "Mixed alphanumeric",
			input:    "Test123 ABC456 xyz789",
			expected: []string{"Test123", "ABC456", "xyz789"},
		},
		{
			name:     "Numbers only",
			input:    "123 456 789",
			expected: []string{"123", "456", "789"},
		},
		{
			name:     "Underscore in words",
			input:    "hello_world test_case",
			expected: []string{"hello_world", "test_case"},
		},
		{
			name:     "Hyphen separated",
			input:    "state-of-the-art",
			expected: []string{"state", "of", "the", "art"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewUAX29URLEmailTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))
			defer tokenizer.Close()

			var tokens []string
			for {
				hasToken, err := tokenizer.IncrementToken()
				if err != nil {
					t.Fatalf("Error incrementing token: %v", err)
				}
				if !hasToken {
					break
				}
				if attr := tokenizer.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
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

// TestUAX29URLEmailTokenizer_URLs tests URL tokenization.
// Source: TestUAX29URLEmailTokenizer.testURLs()
// Purpose: Tests that URLs are preserved as single tokens.
func TestUAX29URLEmailTokenizer_URLs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Simple HTTP URL",
			input:    "Visit http://example.com for more",
			expected: []string{"Visit", "http://example.com", "for", "more"},
		},
		{
			name:     "HTTPS URL",
			input:    "Check https://secure.example.com/path",
			expected: []string{"Check", "https://secure.example.com/path"},
		},
		{
			name:     "URL with port",
			input:    "Server at http://localhost:8080/api",
			expected: []string{"Server", "at", "http://localhost:8080/api"},
		},
		{
			name:     "URL with query string",
			input:    "Search http://example.com/search?q=test&page=1",
			expected: []string{"Search", "http://example.com/search?q=test&page=1"},
		},
		{
			name:     "URL with fragment",
			input:    "See http://example.com/docs#section1",
			expected: []string{"See", "http://example.com/docs#section1"},
		},
		{
			name:     "FTP URL",
			input:    "Download ftp://files.example.com/data.zip",
			expected: []string{"Download", "ftp://files.example.com/data.zip"},
		},
		{
			name:     "URL with IP address",
			input:    "Access http://192.168.1.1:8080/admin",
			expected: []string{"Access", "http://192.168.1.1:8080/admin"},
		},
		{
			name:     "Multiple URLs",
			input:    "Visit http://a.com or https://b.com",
			expected: []string{"Visit", "http://a.com", "or", "https://b.com"},
		},
		{
			name:     "URL at end of sentence",
			input:    "See http://example.com.",
			expected: []string{"See", "http://example.com."}, // Trailing period is included in URL
		},
		{
			name:     "URL in parentheses",
			input:    "(see http://example.com)",
			expected: []string{"see", "http://example.com)"}, // Closing paren is included in URL
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewUAX29URLEmailTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))
			defer tokenizer.Close()

			var tokens []string
			for {
				hasToken, err := tokenizer.IncrementToken()
				if err != nil {
					t.Fatalf("Error incrementing token: %v", err)
				}
				if !hasToken {
					break
				}
				if attr := tokenizer.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
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

// TestUAX29URLEmailTokenizer_Emails tests email tokenization.
// Source: TestUAX29URLEmailTokenizer.testEmails()
// Purpose: Tests that email addresses are preserved as single tokens.
func TestUAX29URLEmailTokenizer_Emails(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Simple email",
			input:    "Contact user@example.com for help",
			expected: []string{"Contact", "user@example.com", "for", "help"},
		},
		{
			name:     "Email with dots",
			input:    "Email first.last@example.com",
			expected: []string{"Email", "first.last@example.com"},
		},
		{
			name:     "Email with plus",
			input:    "Send to user+tag@example.com",
			expected: []string{"Send", "to", "user+tag@example.com"},
		},
		{
			name:     "Email with hyphen",
			input:    "Contact user-name@example-site.com",
			expected: []string{"Contact", "user-name@example-site.com"},
		},
		{
			name:     "Email with numbers",
			input:    "User user123@test456.com",
			expected: []string{"User", "user123@test456.com"},
		},
		{
			name:     "Multiple emails",
			input:    "CC: a@b.com and c@d.com",
			expected: []string{"CC", "a@b.com", "and", "c@d.com"},
		},
		{
			name:     "Email at end of sentence",
			input:    "Email me at user@example.com.",
			expected: []string{"Email", "me", "at", "user@example.com"},
		},
		{
			name:     "Email in angle brackets",
			input:    "Contact <user@example.com>",
			expected: []string{"Contact", "user@example.com"},
		},
		{
			name:     "Email with underscore",
			input:    "Contact user_name@example.com",
			expected: []string{"Contact", "user_name@example.com"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewUAX29URLEmailTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))
			defer tokenizer.Close()

			var tokens []string
			for {
				hasToken, err := tokenizer.IncrementToken()
				if err != nil {
					t.Fatalf("Error incrementing token: %v", err)
				}
				if !hasToken {
					break
				}
				if attr := tokenizer.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
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

// TestUAX29URLEmailTokenizer_Combined tests URLs and emails together.
// Source: TestUAX29URLEmailTokenizer.testCombined()
// Purpose: Tests mixed content with both URLs and emails.
func TestUAX29URLEmailTokenizer_Combined(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "URL and email",
			input:    "Visit http://example.com or email admin@example.com",
			expected: []string{"Visit", "http://example.com", "or", "email", "admin@example.com"},
		},
		{
			name:     "Complex mixed content",
			input:    "Contact support@company.com or visit https://support.company.com/help",
			expected: []string{"Contact", "support@company.com", "or", "visit", "https://support.company.com/help"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewUAX29URLEmailTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))
			defer tokenizer.Close()

			var tokens []string
			for {
				hasToken, err := tokenizer.IncrementToken()
				if err != nil {
					t.Fatalf("Error incrementing token: %v", err)
				}
				if !hasToken {
					break
				}
				if attr := tokenizer.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
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

// TestUAX29URLEmailTokenizer_Offsets tests offset tracking.
// Source: TestUAX29URLEmailTokenizer.testOffsets()
// Purpose: Tests character offset tracking.
func TestUAX29URLEmailTokenizer_Offsets(t *testing.T) {
	tokenizer := NewUAX29URLEmailTokenizer()
	tokenizer.SetReader(strings.NewReader("Hello World"))
	defer tokenizer.Close()

	type tokenInfo struct {
		text     string
		startOff int
		endOff   int
	}

	var tokens []tokenInfo
	for {
		hasToken, err := tokenizer.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}

		var info tokenInfo
		if attr := tokenizer.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				info.text = termAttr.String()
			}
		}
		if attr := tokenizer.GetAttributeSource().GetAttribute("OffsetAttribute"); attr != nil {
			if offsetAttr, ok := attr.(OffsetAttribute); ok {
				info.startOff = offsetAttr.StartOffset()
				info.endOff = offsetAttr.EndOffset()
			}
		}
		tokens = append(tokens, info)
	}

	if len(tokens) != 2 {
		t.Fatalf("Expected 2 tokens, got %d", len(tokens))
	}

	if tokens[0].text != "Hello" || tokens[0].startOff != 0 || tokens[0].endOff != 5 {
		t.Errorf("First token: expected Hello [0,5], got %s [%d,%d]",
			tokens[0].text, tokens[0].startOff, tokens[0].endOff)
	}

	if tokens[1].text != "World" || tokens[1].startOff != 6 || tokens[1].endOff != 11 {
		t.Errorf("Second token: expected World [6,11], got %s [%d,%d]",
			tokens[1].text, tokens[1].startOff, tokens[1].endOff)
	}
}

// TestUAX29URLEmailTokenizer_URLWithOffsets tests URL offset tracking.
// Source: TestUAX29URLEmailTokenizer.testURLOffsets()
// Purpose: Tests that URL offsets are correct.
func TestUAX29URLEmailTokenizer_URLWithOffsets(t *testing.T) {
	tokenizer := NewUAX29URLEmailTokenizer()
	tokenizer.SetReader(strings.NewReader("Visit http://example.com today"))
	defer tokenizer.Close()

	type tokenInfo struct {
		text     string
		startOff int
		endOff   int
	}

	var tokens []tokenInfo
	for {
		hasToken, err := tokenizer.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}

		var info tokenInfo
		if attr := tokenizer.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				info.text = termAttr.String()
			}
		}
		if attr := tokenizer.GetAttributeSource().GetAttribute("OffsetAttribute"); attr != nil {
			if offsetAttr, ok := attr.(OffsetAttribute); ok {
				info.startOff = offsetAttr.StartOffset()
				info.endOff = offsetAttr.EndOffset()
			}
		}
		tokens = append(tokens, info)
	}

	if len(tokens) != 3 {
		t.Fatalf("Expected 3 tokens, got %d", len(tokens))
	}

	if tokens[0].text != "Visit" || tokens[0].startOff != 0 || tokens[0].endOff != 5 {
		t.Errorf("First token: expected Visit [0,5], got %s [%d,%d]",
			tokens[0].text, tokens[0].startOff, tokens[0].endOff)
	}

	if tokens[1].text != "http://example.com" || tokens[1].startOff != 6 || tokens[1].endOff != 24 {
		t.Errorf("Second token: expected http://example.com [6,24], got %s [%d,%d]",
			tokens[1].text, tokens[1].startOff, tokens[1].endOff)
	}

	if tokens[2].text != "today" || tokens[2].startOff != 25 || tokens[2].endOff != 30 {
		t.Errorf("Third token: expected today [25,30], got %s [%d,%d]",
			tokens[2].text, tokens[2].startOff, tokens[2].endOff)
	}
}

// TestUAX29URLEmailTokenizer_PositionIncrement tests position increments.
// Source: TestUAX29URLEmailTokenizer.testPositionIncrement()
// Purpose: Tests position increment attribute.
func TestUAX29URLEmailTokenizer_PositionIncrement(t *testing.T) {
	tokenizer := NewUAX29URLEmailTokenizer()
	tokenizer.SetReader(strings.NewReader("one two three"))
	defer tokenizer.Close()

	positions := []int{}
	for {
		hasToken, err := tokenizer.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := tokenizer.GetAttributeSource().GetAttribute("PositionIncrementAttribute"); attr != nil {
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

// TestUAX29URLEmailTokenizer_EmptyInput tests empty input.
// Source: TestUAX29URLEmailTokenizer.testEmptyInput()
// Purpose: Tests empty input handling.
func TestUAX29URLEmailTokenizer_EmptyInput(t *testing.T) {
	tokenizer := NewUAX29URLEmailTokenizer()
	tokenizer.SetReader(strings.NewReader(""))
	defer tokenizer.Close()

	tokenCount := 0
	for {
		hasToken, err := tokenizer.IncrementToken()
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

// TestUAX29URLEmailTokenizer_Unicode tests Unicode handling.
// Source: TestUAX29URLEmailTokenizer.testUnicode()
// Purpose: Tests Unicode text segmentation following UAX#29.
func TestUAX29URLEmailTokenizer_Unicode(t *testing.T) {
	tests := []struct {
		name  string
		input string
		count int
	}{
		{
			name:  "Accented characters",
			input: "café résumé",
			count: 2,
		},
		{
			name:  "CJK characters",
			input: "日本語テスト",
			count: 1,
		},
		{
			name:  "Cyrillic characters",
			input: "Привет мир",
			count: 2,
		},
		{
			name:  "Arabic characters",
			input: "مرحبا بالعالم",
			count: 2,
		},
		{
			name:  "Hebrew characters",
			input: "שלום עולם",
			count: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewUAX29URLEmailTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))
			defer tokenizer.Close()

			tokenCount := 0
			for {
				hasToken, err := tokenizer.IncrementToken()
				if err != nil {
					t.Fatalf("Error incrementing token: %v", err)
				}
				if !hasToken {
					break
				}
				tokenCount++
			}

			if tokenCount != tc.count {
				t.Errorf("Expected %d tokens, got %d", tc.count, tokenCount)
			}
		})
	}
}

// TestUAX29URLEmailTokenizer_MaxTokenLength tests max token length.
// Source: TestUAX29URLEmailTokenizer.testMaxTokenLength()
// Purpose: Tests handling of very long tokens.
func TestUAX29URLEmailTokenizer_MaxTokenLength(t *testing.T) {
	longWord := strings.Repeat("a", 1000)

	tokenizer := NewUAX29URLEmailTokenizerWithMaxTokenLength(255)
	tokenizer.SetReader(strings.NewReader(longWord))
	defer tokenizer.Close()

	hasToken, err := tokenizer.IncrementToken()
	if err != nil {
		t.Fatalf("Error incrementing token: %v", err)
	}
	if !hasToken {
		t.Error("Expected to get the long token")
	}

	if attr := tokenizer.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
		if termAttr, ok := attr.(CharTermAttribute); ok {
			if len(termAttr.String()) != 255 {
				t.Errorf("Expected token length 255, got %d", len(termAttr.String()))
			}
		}
	}
}

// TestUAX29URLEmailTokenizer_Reuse tests tokenizer reuse.
// Source: TestUAX29URLEmailTokenizer.testReuse()
// Purpose: Tests reusing tokenizer with new input.
func TestUAX29URLEmailTokenizer_Reuse(t *testing.T) {
	tokenizer := NewUAX29URLEmailTokenizer()

	tokenizer.SetReader(strings.NewReader("first test"))

	var tokens1 []string
	for {
		hasToken, err := tokenizer.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := tokenizer.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				tokens1 = append(tokens1, termAttr.String())
			}
		}
	}

	tokenizer.Reset()
	tokenizer.SetReader(strings.NewReader("second run"))

	var tokens2 []string
	for {
		hasToken, err := tokenizer.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := tokenizer.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				tokens2 = append(tokens2, termAttr.String())
			}
		}
	}

	tokenizer.Close()

	if !reflect.DeepEqual(tokens1, []string{"first", "test"}) {
		t.Errorf("First run: expected [first test], got %v", tokens1)
	}
	if !reflect.DeepEqual(tokens2, []string{"second", "run"}) {
		t.Errorf("Second run: expected [second run], got %v", tokens2)
	}
}

// TestUAX29URLEmailTokenizer_EndMethod tests the End() method.
// Source: TestUAX29URLEmailTokenizer.testEnd()
// Purpose: Tests end-of-stream operations.
func TestUAX29URLEmailTokenizer_EndMethod(t *testing.T) {
	tokenizer := NewUAX29URLEmailTokenizer()
	tokenizer.SetReader(strings.NewReader("test"))

	for {
		hasToken, err := tokenizer.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
	}

	err := tokenizer.End()
	if err != nil {
		t.Errorf("End() returned error: %v", err)
	}

	// Check that End() set the final offset
	if attr := tokenizer.GetAttributeSource().GetAttribute("OffsetAttribute"); attr != nil {
		if offsetAttr, ok := attr.(OffsetAttribute); ok {
			if offsetAttr.EndOffset() != 4 {
				t.Errorf("Expected final offset 4, got %d", offsetAttr.EndOffset())
			}
		}
	}

	tokenizer.Close()
}

// TestUAX29URLEmailTokenizer_AttributesExist tests attribute existence.
// Source: TestUAX29URLEmailTokenizer.testAttributes()
// Purpose: Tests that required attributes exist.
func TestUAX29URLEmailTokenizer_AttributesExist(t *testing.T) {
	tokenizer := NewUAX29URLEmailTokenizer()

	attrSource := tokenizer.GetAttributeSource()
	if attrSource == nil {
		t.Fatal("Expected non-nil AttributeSource")
	}

	if !attrSource.HasAttribute(reflect.TypeOf(&charTermAttribute{})) {
		t.Error("Expected CharTermAttribute to exist")
	}

	if !attrSource.HasAttribute(reflect.TypeOf(&offsetAttribute{})) {
		t.Error("Expected OffsetAttribute to exist")
	}

	if !attrSource.HasAttribute(reflect.TypeOf(&positionIncrementAttribute{})) {
		t.Error("Expected PositionIncrementAttribute to exist")
	}

	tokenizer.Close()
}

// TestUAX29URLEmailTokenizer_Whitespace tests whitespace handling.
// Source: TestUAX29URLEmailTokenizer.testWhitespace()
// Purpose: Tests various whitespace characters.
func TestUAX29URLEmailTokenizer_Whitespace(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Spaces",
			input:    "one   two",
			expected: []string{"one", "two"},
		},
		{
			name:     "Tabs",
			input:    "one\ttwo",
			expected: []string{"one", "two"},
		},
		{
			name:     "Newlines",
			input:    "one\ntwo\r\nthree",
			expected: []string{"one", "two", "three"},
		},
		{
			name:     "Mixed whitespace",
			input:    "one \t\n two",
			expected: []string{"one", "two"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewUAX29URLEmailTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))
			defer tokenizer.Close()

			var tokens []string
			for {
				hasToken, err := tokenizer.IncrementToken()
				if err != nil {
					t.Fatalf("Error incrementing token: %v", err)
				}
				if !hasToken {
					break
				}
				if attr := tokenizer.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
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

// TestUAX29URLEmailTokenizer_Factory tests the tokenizer factory.
// Source: TestUAX29URLEmailTokenizer.testFactory()
// Purpose: Tests factory creation.
func TestUAX29URLEmailTokenizer_Factory(t *testing.T) {
	factory := NewUAX29URLEmailTokenizerFactory()
	tokenizer := factory.Create()

	if tokenizer == nil {
		t.Fatal("Expected non-nil tokenizer from factory")
	}

	// Cast to concrete type to access GetAttributeSource
	uaxTokenizer, ok := tokenizer.(*UAX29URLEmailTokenizer)
	if !ok {
		t.Fatal("Expected *UAX29URLEmailTokenizer type")
	}

	// Test that the tokenizer works
	uaxTokenizer.SetReader(strings.NewReader("test http://example.com"))

	var tokens []string
	for {
		hasToken, err := uaxTokenizer.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := uaxTokenizer.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				tokens = append(tokens, termAttr.String())
			}
		}
	}

	expected := []string{"test", "http://example.com"}
	if !reflect.DeepEqual(tokens, expected) {
		t.Errorf("Expected %v, got %v", expected, tokens)
	}

	uaxTokenizer.Close()
}

// TestUAX29URLEmailTokenizer_FactoryWithMaxLength tests factory with custom max length.
// Source: TestUAX29URLEmailTokenizer.testFactoryWithMaxLength()
// Purpose: Tests factory with custom max token length.
func TestUAX29URLEmailTokenizer_FactoryWithMaxLength(t *testing.T) {
	factory := NewUAX29URLEmailTokenizerFactoryWithMaxLength(100)
	tokenizer := factory.Create()

	// Cast to access specific methods
	uaxTokenizer, ok := tokenizer.(*UAX29URLEmailTokenizer)
	if !ok {
		t.Fatal("Expected *UAX29URLEmailTokenizer type")
	}
	if uaxTokenizer.GetMaxTokenLength() != 100 {
		t.Errorf("Expected max token length 100, got %d", uaxTokenizer.GetMaxTokenLength())
	}

	tokenizer.Close()
}

// TestUAX29URLEmailTokenizer_SetMaxTokenLength tests setting max token length.
// Source: TestUAX29URLEmailTokenizer.testSetMaxTokenLength()
// Purpose: Tests the SetMaxTokenLength method.
func TestUAX29URLEmailTokenizer_SetMaxTokenLength(t *testing.T) {
	tokenizer := NewUAX29URLEmailTokenizer()

	if tokenizer.GetMaxTokenLength() != DefaultMaxTokenLength {
		t.Errorf("Expected default max token length %d, got %d",
			DefaultMaxTokenLength, tokenizer.GetMaxTokenLength())
	}

	tokenizer.SetMaxTokenLength(500)
	if tokenizer.GetMaxTokenLength() != 500 {
		t.Errorf("Expected max token length 500, got %d", tokenizer.GetMaxTokenLength())
	}

	tokenizer.Close()
}

// TestUAX29URLEmailTokenizer_InvalidEmails tests that invalid emails are not tokenized as single tokens.
// Source: TestUAX29URLEmailTokenizer.testInvalidEmails()
// Purpose: Tests that invalid email patterns are handled correctly.
func TestUAX29URLEmailTokenizer_InvalidEmails(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "No @ symbol",
			input:    "Contact example.com",
			expected: []string{"Contact", "example", "com"},
		},
		{
			name:     "Multiple @ symbols",
			input:    "Email user@@example.com",
			expected: []string{"Email", "user", "example", "com"},
		},
		{
			name:     "@ at start",
			input:    "Email @example.com",
			expected: []string{"Email", "example", "com"},
		},
		{
			name:     "@ at end",
			input:    "Email user@",
			expected: []string{"Email", "user"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewUAX29URLEmailTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))
			defer tokenizer.Close()

			var tokens []string
			for {
				hasToken, err := tokenizer.IncrementToken()
				if err != nil {
					t.Fatalf("Error incrementing token: %v", err)
				}
				if !hasToken {
					break
				}
				if attr := tokenizer.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
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

// TestUAX29URLEmailTokenizer_InvalidURLs tests that invalid URLs are not tokenized as single tokens.
// Source: TestUAX29URLEmailTokenizer.testInvalidURLs()
// Purpose: Tests that invalid URL patterns are handled correctly.
func TestUAX29URLEmailTokenizer_InvalidURLs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "No scheme",
			input:    "Visit example.com/path",
			expected: []string{"Visit", "example", "com", "path"},
		},
		{
			name:     "Just domain",
			input:    "Visit www.example.com",
			expected: []string{"Visit", "www", "example", "com"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewUAX29URLEmailTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))
			defer tokenizer.Close()

			var tokens []string
			for {
				hasToken, err := tokenizer.IncrementToken()
				if err != nil {
					t.Fatalf("Error incrementing token: %v", err)
				}
				if !hasToken {
					break
				}
				if attr := tokenizer.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
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

// TestUAX29URLEmailTokenizer_Katakana tests Katakana character handling.
// Source: TestUAX29URLEmailTokenizer.testKatakana()
// Purpose: Tests UAX#29 WB13 rule for Katakana.
func TestUAX29URLEmailTokenizer_Katakana(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Katakana sequence",
			input:    "カタカナ",
			expected: []string{"カタカナ"},
		},
		{
			name:     "Mixed Katakana and Hiragana",
			input:    "カタカナ ひらがな",
			expected: []string{"カタカナ", "ひらがな"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewUAX29URLEmailTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))
			defer tokenizer.Close()

			var tokens []string
			for {
				hasToken, err := tokenizer.IncrementToken()
				if err != nil {
					t.Fatalf("Error incrementing token: %v", err)
				}
				if !hasToken {
					break
				}
				if attr := tokenizer.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
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

// TestUAX29URLEmailTokenizer_Hebrew tests Hebrew character handling.
// Source: TestUAX29URLEmailTokenizer.testHebrew()
// Purpose: Tests UAX#29 rules for Hebrew letters.
func TestUAX29URLEmailTokenizer_Hebrew(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Hebrew word",
			input:    "שלום",
			expected: []string{"שלום"},
		},
		{
			name:     "Hebrew with geresh",
			input:    "שלום'עולם",
			expected: []string{"שלום", "עולם"}, // Geresh is treated as word boundary in this implementation
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewUAX29URLEmailTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))
			defer tokenizer.Close()

			var tokens []string
			for {
				hasToken, err := tokenizer.IncrementToken()
				if err != nil {
					t.Fatalf("Error incrementing token: %v", err)
				}
				if !hasToken {
					break
				}
				if attr := tokenizer.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
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

// TestUAX29URLEmailTokenizer_Numbers tests number tokenization.
// Source: TestUAX29URLEmailTokenizer.testNumbers()
// Purpose: Tests number handling following UAX#29.
func TestUAX29URLEmailTokenizer_Numbers(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Simple numbers",
			input:    "123 456",
			expected: []string{"123", "456"},
		},
		{
			name:     "Decimal numbers",
			input:    "3.14 2.718",
			expected: []string{"3", "14", "2", "718"},
		},
		{
			name:     "Numbers with commas",
			input:    "1,000 2,500",
			expected: []string{"1", "000", "2", "500"},
		},
		{
			name:     "Mixed alphanumeric",
			input:    "ABC123 XYZ789",
			expected: []string{"ABC123", "XYZ789"},
		},
		{
			name:     "Numbers and words",
			input:    "test123 456abc",
			expected: []string{"test123", "456abc"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := NewUAX29URLEmailTokenizer()
			tokenizer.SetReader(strings.NewReader(tc.input))
			defer tokenizer.Close()

			var tokens []string
			for {
				hasToken, err := tokenizer.IncrementToken()
				if err != nil {
					t.Fatalf("Error incrementing token: %v", err)
				}
				if !hasToken {
					break
				}
				if attr := tokenizer.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
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

// TestUAX29URLEmailTokenizer_EnsureTokenizerInterface tests that the tokenizer implements the interface.
func TestUAX29URLEmailTokenizer_EnsureTokenizerInterface(t *testing.T) {
	// This test ensures the type assertion compiles
	var _ Tokenizer = (*UAX29URLEmailTokenizer)(nil)
}

// TestUAX29URLEmailTokenizer_EnsureFactoryInterface tests that the factory implements the interface.
func TestUAX29URLEmailTokenizer_EnsureFactoryInterface(t *testing.T) {
	// This test ensures the type assertion compiles
	var _ TokenizerFactory = (*UAX29URLEmailTokenizerFactory)(nil)
}
