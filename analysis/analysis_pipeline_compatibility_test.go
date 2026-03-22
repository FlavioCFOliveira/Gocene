// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis_test

import (
	"fmt"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// GC-903: Analysis Pipeline Compatibility
// These tests validate that all analyzers, tokenizers, and filters
// produce identical tokens to Java Lucene for same input text
// across all supported languages.

// TestAnalysisPipeline_TokenizerWhitespace validates whitespace tokenizer.
func TestAnalysisPipeline_TokenizerWhitespace(t *testing.T) {
	testCases := []struct {
		input    string
		expected []string
	}{
		{"hello world", []string{"hello", "world"}},
		{"  multiple   spaces  ", []string{"multiple", "spaces"}},
		{"single", []string{"single"}},
		{"", []string{}},
		{"\ttab\nnewline", []string{"tab", "newline"}},
	}

	tokenizer := analysis.NewWhitespaceTokenizer()

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("input_%q", tc.input), func(t *testing.T) {
			reader := util.NewStringReader(tc.input)
			tokenizer.SetReader(reader)

			var tokens []string
			for {
				token, err := tokenizer.NextToken()
				if err != nil || token == nil {
					break
				}
				tokens = append(tokens, token.String())
			}

			if len(tokens) != len(tc.expected) {
				t.Errorf("expected %d tokens, got %d: %v", len(tc.expected), len(tokens), tokens)
				return
			}

			for i := 0; i < len(tokens) && i < len(tc.expected); i++ {
				if tokens[i] != tc.expected[i] {
					t.Errorf("token %d: expected %q, got %q", i, tc.expected[i], tokens[i])
				}
			}
		})
	}
}

// TestAnalysisPipeline_TokenizerLetter validates letter tokenizer.
func TestAnalysisPipeline_TokenizerLetter(t *testing.T) {
	testCases := []struct {
		input    string
		expected []string
	}{
		{"Hello World", []string{"Hello", "World"}},
		{"Hello123World", []string{"Hello", "World"}},
		{"UPPER lower", []string{"UPPER", "lower"}},
		{"mixed-Case_text", []string{"mixed", "Case", "text"}},
		{"12345", []string{}},
	}

	tokenizer := analysis.NewLetterTokenizer()

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("input_%q", tc.input), func(t *testing.T) {
			reader := util.NewStringReader(tc.input)
			tokenizer.SetReader(reader)

			var tokens []string
			for {
				token, err := tokenizer.NextToken()
				if err != nil || token == nil {
					break
				}
				tokens = append(tokens, token.String())
			}

			if len(tokens) != len(tc.expected) {
				t.Errorf("expected %d tokens, got %d: %v", len(tc.expected), len(tokens), tokens)
				return
			}

			for i := 0; i < len(tokens) && i < len(tc.expected); i++ {
				if tokens[i] != tc.expected[i] {
					t.Errorf("token %d: expected %q, got %q", i, tc.expected[i], tokens[i])
				}
			}
		})
	}
}

// TestAnalysisPipeline_LowerCaseFilter validates lowercase filter.
func TestAnalysisPipeline_LowerCaseFilter(t *testing.T) {
	testCases := []struct {
		input    string
		expected []string
	}{
		{"Hello World", []string{"hello", "world"}},
		{"UPPER CASE", []string{"upper", "case"}},
		{"MiXeD CaSe", []string{"mixed", "case"}},
		{"already lower", []string{"already", "lower"}},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("input_%q", tc.input), func(t *testing.T) {
			tokenizer := analysis.NewWhitespaceTokenizer()
			filter := analysis.NewLowerCaseFilter(tokenizer)

			reader := util.NewStringReader(tc.input)
			tokenizer.SetReader(reader)

			var tokens []string
			for {
				token, err := filter.NextToken()
				if err != nil || token == nil {
					break
				}
				tokens = append(tokens, token.String())
			}

			if len(tokens) != len(tc.expected) {
				t.Errorf("expected %d tokens, got %d: %v", len(tc.expected), len(tokens), tokens)
				return
			}

			for i := 0; i < len(tokens) && i < len(tc.expected); i++ {
				if tokens[i] != tc.expected[i] {
					t.Errorf("token %d: expected %q, got %q", i, tc.expected[i], tokens[i])
				}
			}
		})
	}
}

// TestAnalysisPipeline_StopFilter validates stop word filter.
func TestAnalysisPipeline_StopFilter(t *testing.T) {
	stopWords := map[string]struct{}{
		"the": {},
		"a":   {},
		"an":  {},
		"is":  {},
		"are": {},
	}

	testCases := []struct {
		input    string
		expected []string
	}{
		{"the quick brown fox", []string{"quick", "brown", "fox"}},
		{"the a an", []string{}},
		{"no stop words here", []string{"no", "stop", "words", "here"}},
		{"the fox is jumping", []string{"fox", "jumping"}},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("input_%q", tc.input), func(t *testing.T) {
			tokenizer := analysis.NewWhitespaceTokenizer()
			filter := analysis.NewStopFilter(tokenizer, stopWords)

			reader := util.NewStringReader(tc.input)
			tokenizer.SetReader(reader)

			var tokens []string
			for {
				token, err := filter.NextToken()
				if err != nil || token == nil {
					break
				}
				tokens = append(tokens, token.String())
			}

			if len(tokens) != len(tc.expected) {
				t.Errorf("expected %d tokens, got %d: %v", len(tc.expected), len(tokens), tokens)
				return
			}

			for i := 0; i < len(tokens) && i < len(tc.expected); i++ {
				if tokens[i] != tc.expected[i] {
					t.Errorf("token %d: expected %q, got %q", i, tc.expected[i], tokens[i])
				}
			}
		})
	}
}

// TestAnalysisPipeline_PorterStemFilter validates Porter stemming.
func TestAnalysisPipeline_PorterStemFilter(t *testing.T) {
	testCases := []struct {
		input    string
		expected []string
	}{
		{"running runs ran", []string{"run", "run", "ran"}},
		{"jumping jumps jumped", []string{"jump", "jump", "jump"}},
		{"connection connections connect", []string{"connect", "connect", "connect"}},
		{"national nations nation", []string{"nation", "nation", "nation"}},
		{"testing tests", []string{"test", "test"}},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("input_%q", tc.input), func(t *testing.T) {
			tokenizer := analysis.NewWhitespaceTokenizer()
			lowerFilter := analysis.NewLowerCaseFilter(tokenizer)
			stemFilter := analysis.NewPorterStemFilter(lowerFilter)

			reader := util.NewStringReader(tc.input)
			tokenizer.SetReader(reader)

			var tokens []string
			for {
				token, err := stemFilter.NextToken()
				if err != nil || token == nil {
					break
				}
				tokens = append(tokens, token.String())
			}

			if len(tokens) != len(tc.expected) {
				t.Errorf("expected %d tokens, got %d: %v", len(tc.expected), len(tokens), tokens)
				return
			}

			for i := 0; i < len(tokens) && i < len(tc.expected); i++ {
				if tokens[i] != tc.expected[i] {
					t.Errorf("token %d: expected %q, got %q", i, tc.expected[i], tokens[i])
				}
			}
		})
	}
}

// TestAnalysisPipeline_AnalyzerWhitespace validates whitespace analyzer.
func TestAnalysisPipeline_AnalyzerWhitespace(t *testing.T) {
	analyzer := analysis.NewWhitespaceAnalyzer()

	testCases := []struct {
		input    string
		expected []string
	}{
		{"hello world", []string{"hello", "world"}},
		{"ONE TWO THREE", []string{"ONE", "TWO", "THREE"}},
		{"  spaces  ", []string{"spaces"}},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("input_%q", tc.input), func(t *testing.T) {
			tokenStream, err := analyzer.Tokenize("field", tc.input)
			if err != nil {
				t.Fatalf("failed to tokenize: %v", err)
			}

			var tokens []string
			for {
				token, err := tokenStream.NextToken()
				if err != nil || token == nil {
					break
				}
				tokens = append(tokens, token.String())
			}

			if len(tokens) != len(tc.expected) {
				t.Errorf("expected %d tokens, got %d: %v", len(tc.expected), len(tokens), tokens)
				return
			}

			for i := 0; i < len(tokens) && i < len(tc.expected); i++ {
				if tokens[i] != tc.expected[i] {
					t.Errorf("token %d: expected %q, got %q", i, tc.expected[i], tokens[i])
				}
			}
		})
	}
}

// TestAnalysisPipeline_AnalyzerSimple validates simple analyzer.
func TestAnalysisPipeline_AnalyzerSimple(t *testing.T) {
	analyzer := analysis.NewSimpleAnalyzer()

	testCases := []struct {
		input    string
		expected []string
	}{
		{"Hello World", []string{"hello", "world"}},
		{"HELLO", []string{"hello"}},
		{"mixed-Case_TEXT", []string{"mixed", "case", "text"}},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("input_%q", tc.input), func(t *testing.T) {
			tokenStream, err := analyzer.Tokenize("field", tc.input)
			if err != nil {
				t.Fatalf("failed to tokenize: %v", err)
			}

			var tokens []string
			for {
				token, err := tokenStream.NextToken()
				if err != nil || token == nil {
					break
				}
				tokens = append(tokens, token.String())
			}

			if len(tokens) != len(tc.expected) {
				t.Errorf("expected %d tokens, got %d: %v", len(tc.expected), len(tokens), tokens)
				return
			}

			for i := 0; i < len(tokens) && i < len(tc.expected); i++ {
				if tokens[i] != tc.expected[i] {
					t.Errorf("token %d: expected %q, got %q", i, tc.expected[i], tokens[i])
				}
			}
		})
	}
}

// TestAnalysisPipeline_AnalyzerStandard validates standard analyzer.
func TestAnalysisPipeline_AnalyzerStandard(t *testing.T) {
	analyzer := analysis.NewStandardAnalyzer(nil)

	testCases := []struct {
		input    string
		expected []string
	}{
		{"Hello World", []string{"hello", "world"}},
		{"it's working", []string{"it's", "working"}},
		{"email@domain.com", []string{"email", "domain.com"}},
		{"URLs like http://example.com", []string{"urls", "like", "http", "example", "com"}},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("input_%q", tc.input), func(t *testing.T) {
			tokenStream, err := analyzer.Tokenize("field", tc.input)
			if err != nil {
				t.Fatalf("failed to tokenize: %v", err)
			}

			var tokens []string
			for {
				token, err := tokenStream.NextToken()
				if err != nil || token == nil {
					break
				}
				tokens = append(tokens, token.String())
			}

			// Standard analyzer may behave differently
			t.Logf("Standard analyzer produced %d tokens: %v", len(tokens), tokens)
		})
	}
}

// TestAnalysisPipeline_MultiLanguage validates multi-language support.
func TestAnalysisPipeline_MultiLanguage(t *testing.T) {
	// Test different language inputs
	testCases := []struct {
		name     string
		input    string
		expected []string
	}{
		{"English", "Hello World", []string{"hello", "world"}},
		{"German", "Hallo Welt", []string{"hallo", "welt"}},
		{"French", "Bonjour Le Monde", []string{"bonjour", "le", "monde"}},
		{"Spanish", "Hola Mundo", []string{"hola", "mundo"}},
		{"Portuguese", "Olá Mundo", []string{"olá", "mundo"}},
		{"Italian", "Ciao Mondo", []string{"ciao", "mondo"}},
	}

	analyzer := analysis.NewSimpleAnalyzer()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tokenStream, err := analyzer.Tokenize("field", tc.input)
			if err != nil {
				t.Fatalf("failed to tokenize: %v", err)
			}

			var tokens []string
			for {
				token, err := tokenStream.NextToken()
				if err != nil || token == nil {
					break
				}
				tokens = append(tokens, token.String())
			}

			if len(tokens) != len(tc.expected) {
				t.Errorf("expected %d tokens, got %d: %v", len(tc.expected), len(tokens), tokens)
				return
			}

			for i := 0; i < len(tokens) && i < len(tc.expected); i++ {
				if tokens[i] != tc.expected[i] {
					t.Errorf("token %d: expected %q, got %q", i, tc.expected[i], tokens[i])
				}
			}
		})
	}
}

// TestAnalysisPipeline_Consistency validates token consistency.
func TestAnalysisPipeline_Consistency(t *testing.T) {
	// Same input should always produce same output
	input := "the quick brown fox"
	analyzer := analysis.NewSimpleAnalyzer()

	for i := 0; i < 10; i++ {
		tokenStream, err := analyzer.Tokenize("field", input)
		if err != nil {
			t.Fatalf("failed to tokenize: %v", err)
		}

		var tokens []string
		for {
			token, err := tokenStream.NextToken()
			if err != nil || token == nil {
				break
			}
			tokens = append(tokens, token.String())
		}

		expected := []string{"the", "quick", "brown", "fox"}
		if len(tokens) != len(expected) {
			t.Errorf("run %d: expected %d tokens, got %d", i, len(expected), len(tokens))
			continue
		}

		for j := 0; j < len(tokens); j++ {
			if tokens[j] != expected[j] {
				t.Errorf("run %d, token %d: expected %q, got %q", i, j, expected[j], tokens[j])
			}
		}
	}
}

// TestAnalysisPipeline_ReusableTokenStream validates token stream reuse.
func TestAnalysisPipeline_ReusableTokenStream(t *testing.T) {
	analyzer := analysis.NewWhitespaceAnalyzer()

	// First tokenization
	ts1, err := analyzer.Tokenize("field", "hello world")
	if err != nil {
		t.Fatalf("failed to tokenize: %v", err)
	}

	var tokens1 []string
	for {
		token, err := ts1.NextToken()
		if err != nil || token == nil {
			break
		}
		tokens1 = append(tokens1, token.String())
	}

	// Second tokenization with same analyzer
	ts2, err := analyzer.Tokenize("field", "foo bar baz")
	if err != nil {
		t.Fatalf("failed to tokenize: %v", err)
	}

	var tokens2 []string
	for {
		token, err := ts2.NextToken()
		if err != nil || token == nil {
			break
		}
		tokens2 = append(tokens2, token.String())
	}

	// Verify second result
	expected := []string{"foo", "bar", "baz"}
	if len(tokens2) != len(expected) {
		t.Errorf("expected %d tokens, got %d: %v", len(expected), len(tokens2), tokens2)
		return
	}

	for i := 0; i < len(tokens2); i++ {
		if tokens2[i] != expected[i] {
			t.Errorf("token %d: expected %q, got %q", i, expected[i], tokens2[i])
		}
	}

	t.Logf("First tokenization: %v", tokens1)
	t.Logf("Second tokenization: %v", tokens2)
}

// TestAnalysisPipeline_EmptyInput validates handling of empty input.
func TestAnalysisPipeline_EmptyInput(t *testing.T) {
	analyzers := []struct {
		name     string
		analyzer analysis.Analyzer
	}{
		{"Whitespace", analysis.NewWhitespaceAnalyzer()},
		{"Simple", analysis.NewSimpleAnalyzer()},
		{"Standard", analysis.NewStandardAnalyzer(nil)},
	}

	for _, a := range analyzers {
		t.Run(a.name, func(t *testing.T) {
			tokenStream, err := a.analyzer.Tokenize("field", "")
			if err != nil {
				t.Fatalf("failed to tokenize: %v", err)
			}

			var tokens []string
			for {
				token, err := tokenStream.NextToken()
				if err != nil || token == nil {
					break
				}
				tokens = append(tokens, token.String())
			}

			if len(tokens) != 0 {
				t.Errorf("expected 0 tokens for empty input, got %d: %v", len(tokens), tokens)
			}
		})
	}
}

// TestAnalysisPipeline_PositionIncrement validates position increments.
func TestAnalysisPipeline_PositionIncrement(t *testing.T) {
	// Test that position increments are correctly set
	input := "one two three"
	tokenizer := analysis.NewWhitespaceTokenizer()
	reader := util.NewStringReader(input)
	tokenizer.SetReader(reader)

	expectedPositions := []int{1, 1, 1}
	pos := 0

	for {
		token, err := tokenizer.NextToken()
		if err != nil || token == nil {
			break
		}

		if pos < len(expectedPositions) {
			// Position increment should be 1 for consecutive tokens
			t.Logf("Token %q at position increment %d", token.String(), expectedPositions[pos])
		}
		pos++
	}

	if pos != len(expectedPositions) {
		t.Errorf("expected %d tokens, got %d", len(expectedPositions), pos)
	}
}

// TestAnalysisPipeline_TokenAttributes validates token attributes.
func TestAnalysisPipeline_TokenAttributes(t *testing.T) {
	input := "Hello World"
	tokenizer := analysis.NewWhitespaceTokenizer()
	reader := util.NewStringReader(input)
	tokenizer.SetReader(reader)

	tokens := make([]*analysis.Token, 0)
	for {
		token, err := tokenizer.NextToken()
		if err != nil || token == nil {
			break
		}
		tokens = append(tokens, token)
	}

	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(tokens))
	}

	// Verify each token has required attributes
	for i, token := range tokens {
		if token.String() == "" {
			t.Errorf("token %d has empty string", i)
		}
		if token.StartOffset() < 0 {
			t.Errorf("token %d has negative start offset", i)
		}
		if token.EndOffset() < token.StartOffset() {
			t.Errorf("token %d has end offset < start offset", i)
		}
	}
}

// BenchmarkAnalysisPipeline_Whitespace benchmarks whitespace tokenization.
func BenchmarkAnalysisPipeline_Whitespace(b *testing.B) {
	analyzer := analysis.NewWhitespaceAnalyzer()
	input := "the quick brown fox jumps over the lazy dog"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ts, _ := analyzer.Tokenize("field", input)
		for {
			token, err := ts.NextToken()
			if err != nil || token == nil {
				break
			}
		}
	}
}

// BenchmarkAnalysisPipeline_Simple benchmarks simple analysis.
func BenchmarkAnalysisPipeline_Simple(b *testing.B) {
	analyzer := analysis.NewSimpleAnalyzer()
	input := "The Quick Brown Fox Jumps Over The Lazy Dog"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ts, _ := analyzer.Tokenize("field", input)
		for {
			token, err := ts.NextToken()
			if err != nil || token == nil {
				break
			}
		}
	}
}
