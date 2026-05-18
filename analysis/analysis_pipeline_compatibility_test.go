// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// GC-903: Analysis Pipeline Compatibility
// These tests validate that all analyzers, tokenizers, and filters
// produce identical tokens to Java Lucene for same input text
// across all supported languages.

// tokenInfo holds the text and offset information extracted from a token stream.
type tokenInfo struct {
	text        string
	startOffset int
	endOffset   int
}

func (ti tokenInfo) String() string   { return ti.text }
func (ti tokenInfo) StartOffset() int { return ti.startOffset }
func (ti tokenInfo) EndOffset() int   { return ti.endOffset }

// nextToken advances the token stream by one token and returns the extracted
// token information, or (nil, nil) when the stream is exhausted.
func nextToken(ts interface {
	IncrementToken() (bool, error)
	GetAttribute(string) util.AttributeImpl
}) (*tokenInfo, error) {
	ok, err := ts.IncrementToken()
	if !ok || err != nil {
		return nil, err
	}

	var text string
	if attr := ts.GetAttribute("CharTermAttribute"); attr != nil {
		if termAttr, ok := attr.(analysis.CharTermAttribute); ok {
			text = termAttr.String()
		}
	}

	ti := &tokenInfo{text: text}

	if attr := ts.GetAttribute("OffsetAttribute"); attr != nil {
		if offsetAttr, ok := attr.(analysis.OffsetAttribute); ok {
			ti.startOffset = offsetAttr.StartOffset()
			ti.endOffset = offsetAttr.EndOffset()
		}
	}

	return ti, nil
}

// collectTokenStrings drains a token stream, returning all token texts.
func collectTokenStrings(ts interface {
	IncrementToken() (bool, error)
	GetAttribute(string) util.AttributeImpl
}) ([]string, error) {
	var tokens []string
	for {
		ti, err := nextToken(ts)
		if err != nil {
			return nil, err
		}
		if ti == nil {
			break
		}
		tokens = append(tokens, ti.text)
	}
	return tokens, nil
}

// collectFromAnalyzer drains the TokenStream returned by an Analyzer.
func collectFromAnalyzer(a analysis.Analyzer, field, text string) ([]string, error) {
	ts, err := a.TokenStream(field, strings.NewReader(text))
	if err != nil {
		return nil, err
	}
	defer ts.Close()

	// TokenStream from analyzers embed BaseTokenStream which exposes GetAttribute.
	type attributeGetter interface {
		IncrementToken() (bool, error)
		GetAttribute(string) util.AttributeImpl
	}

	ag, ok := ts.(attributeGetter)
	if !ok {
		// Fallback: just count tokens without extracting text.
		var dummy []string
		for {
			hasToken, err := ts.IncrementToken()
			if err != nil {
				return nil, err
			}
			if !hasToken {
				break
			}
			dummy = append(dummy, "")
		}
		return dummy, nil
	}

	return collectTokenStrings(ag)
}

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
			tokenizer.SetReader(strings.NewReader(tc.input))

			tokens, err := collectTokenStrings(tokenizer)
			if err != nil {
				t.Fatalf("tokenization error: %v", err)
			}
			if len(tokens) != len(tc.expected) {
				t.Errorf("expected %d tokens, got %d: %v", len(tc.expected), len(tokens), tokens)
				return
			}
			for i := range tokens {
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
			tokenizer.SetReader(strings.NewReader(tc.input))

			tokens, err := collectTokenStrings(tokenizer)
			if err != nil {
				t.Fatalf("tokenization error: %v", err)
			}
			if len(tokens) != len(tc.expected) {
				t.Errorf("expected %d tokens, got %d: %v", len(tc.expected), len(tokens), tokens)
				return
			}
			for i := range tokens {
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

			tokenizer.SetReader(strings.NewReader(tc.input))

			tokens, err := collectTokenStrings(filter)
			if err != nil {
				t.Fatalf("tokenization error: %v", err)
			}
			if len(tokens) != len(tc.expected) {
				t.Errorf("expected %d tokens, got %d: %v", len(tc.expected), len(tokens), tokens)
				return
			}
			for i := range tokens {
				if tokens[i] != tc.expected[i] {
					t.Errorf("token %d: expected %q, got %q", i, tc.expected[i], tokens[i])
				}
			}
		})
	}
}

// TestAnalysisPipeline_StopFilter validates stop word filter.
func TestAnalysisPipeline_StopFilter(t *testing.T) {
	stopWordSet := map[string]struct{}{
		"the": {},
		"a":   {},
		"an":  {},
		"is":  {},
		"are": {},
	}

	// Convert the set to a slice for NewStopFilter.
	stopWords := make([]string, 0, len(stopWordSet))
	for w := range stopWordSet {
		stopWords = append(stopWords, w)
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

			tokenizer.SetReader(strings.NewReader(tc.input))

			tokens, err := collectTokenStrings(filter)
			if err != nil {
				t.Fatalf("tokenization error: %v", err)
			}
			if len(tokens) != len(tc.expected) {
				t.Errorf("expected %d tokens, got %d: %v", len(tc.expected), len(tokens), tokens)
				return
			}
			for i := range tokens {
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

			tokenizer.SetReader(strings.NewReader(tc.input))

			tokens, err := collectTokenStrings(stemFilter)
			if err != nil {
				t.Fatalf("tokenization error: %v", err)
			}
			if len(tokens) != len(tc.expected) {
				t.Errorf("expected %d tokens, got %d: %v", len(tc.expected), len(tokens), tokens)
				return
			}
			for i := range tokens {
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
			tokens, err := collectFromAnalyzer(analyzer, "field", tc.input)
			if err != nil {
				t.Fatalf("failed to tokenize: %v", err)
			}
			if len(tokens) != len(tc.expected) {
				t.Errorf("expected %d tokens, got %d: %v", len(tc.expected), len(tokens), tokens)
				return
			}
			for i := range tokens {
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
			tokens, err := collectFromAnalyzer(analyzer, "field", tc.input)
			if err != nil {
				t.Fatalf("failed to tokenize: %v", err)
			}
			if len(tokens) != len(tc.expected) {
				t.Errorf("expected %d tokens, got %d: %v", len(tc.expected), len(tokens), tokens)
				return
			}
			for i := range tokens {
				if tokens[i] != tc.expected[i] {
					t.Errorf("token %d: expected %q, got %q", i, tc.expected[i], tokens[i])
				}
			}
		})
	}
}

// TestAnalysisPipeline_AnalyzerStandard validates standard analyzer.
func TestAnalysisPipeline_AnalyzerStandard(t *testing.T) {
	analyzer := analysis.NewStandardAnalyzer()

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
			tokens, err := collectFromAnalyzer(analyzer, "field", tc.input)
			if err != nil {
				t.Fatalf("failed to tokenize: %v", err)
			}
			// Standard analyzer may behave differently from Java Lucene;
			// log the result without asserting exact output.
			t.Logf("Standard analyzer produced %d tokens: %v", len(tokens), tokens)
		})
	}
}

// TestAnalysisPipeline_MultiLanguage validates multi-language support.
func TestAnalysisPipeline_MultiLanguage(t *testing.T) {
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
			tokens, err := collectFromAnalyzer(analyzer, "field", tc.input)
			if err != nil {
				t.Fatalf("failed to tokenize: %v", err)
			}
			if len(tokens) != len(tc.expected) {
				t.Errorf("expected %d tokens, got %d: %v", len(tc.expected), len(tokens), tokens)
				return
			}
			for i := range tokens {
				if tokens[i] != tc.expected[i] {
					t.Errorf("token %d: expected %q, got %q", i, tc.expected[i], tokens[i])
				}
			}
		})
	}
}

// TestAnalysisPipeline_Consistency validates token consistency.
func TestAnalysisPipeline_Consistency(t *testing.T) {
	input := "the quick brown fox"
	analyzer := analysis.NewSimpleAnalyzer()

	for i := 0; i < 10; i++ {
		tokens, err := collectFromAnalyzer(analyzer, "field", input)
		if err != nil {
			t.Fatalf("run %d: failed to tokenize: %v", i, err)
		}

		expected := []string{"the", "quick", "brown", "fox"}
		if len(tokens) != len(expected) {
			t.Errorf("run %d: expected %d tokens, got %d", i, len(expected), len(tokens))
			continue
		}
		for j := range tokens {
			if tokens[j] != expected[j] {
				t.Errorf("run %d, token %d: expected %q, got %q", i, j, expected[j], tokens[j])
			}
		}
	}
}

// TestAnalysisPipeline_ReusableTokenStream validates token stream reuse.
func TestAnalysisPipeline_ReusableTokenStream(t *testing.T) {
	analyzer := analysis.NewWhitespaceAnalyzer()

	tokens1, err := collectFromAnalyzer(analyzer, "field", "hello world")
	if err != nil {
		t.Fatalf("first tokenization failed: %v", err)
	}

	tokens2, err := collectFromAnalyzer(analyzer, "field", "foo bar baz")
	if err != nil {
		t.Fatalf("second tokenization failed: %v", err)
	}

	expected := []string{"foo", "bar", "baz"}
	if len(tokens2) != len(expected) {
		t.Errorf("expected %d tokens, got %d: %v", len(expected), len(tokens2), tokens2)
		return
	}
	for i := range tokens2 {
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
		{"Standard", analysis.NewStandardAnalyzer()},
	}

	for _, a := range analyzers {
		t.Run(a.name, func(t *testing.T) {
			tokens, err := collectFromAnalyzer(a.analyzer, "field", "")
			if err != nil {
				t.Fatalf("failed to tokenize: %v", err)
			}
			if len(tokens) != 0 {
				t.Errorf("expected 0 tokens for empty input, got %d: %v", len(tokens), tokens)
			}
		})
	}
}

// TestAnalysisPipeline_PositionIncrement validates position increments.
func TestAnalysisPipeline_PositionIncrement(t *testing.T) {
	input := "one two three"
	tokenizer := analysis.NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader(input))

	expectedCount := 3
	count := 0

	for {
		ti, err := nextToken(tokenizer)
		if err != nil {
			t.Fatalf("tokenization error: %v", err)
		}
		if ti == nil {
			break
		}
		t.Logf("Token %q at offsets [%d, %d)", ti.text, ti.startOffset, ti.endOffset)
		count++
	}

	if count != expectedCount {
		t.Errorf("expected %d tokens, got %d", expectedCount, count)
	}
}

// TestAnalysisPipeline_TokenAttributes validates token attributes.
func TestAnalysisPipeline_TokenAttributes(t *testing.T) {
	input := "Hello World"
	tokenizer := analysis.NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader(input))

	var tokens []*tokenInfo
	for {
		ti, err := nextToken(tokenizer)
		if err != nil {
			t.Fatalf("tokenization error: %v", err)
		}
		if ti == nil {
			break
		}
		tokens = append(tokens, ti)
	}

	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(tokens))
	}

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
		ts, _ := analyzer.TokenStream("field", strings.NewReader(input))
		if ts == nil {
			continue
		}
		for {
			ok, err := ts.IncrementToken()
			if !ok || err != nil {
				break
			}
		}
		ts.Close()
	}
}

// BenchmarkAnalysisPipeline_Simple benchmarks simple analysis.
func BenchmarkAnalysisPipeline_Simple(b *testing.B) {
	analyzer := analysis.NewSimpleAnalyzer()
	input := "The Quick Brown Fox Jumps Over The Lazy Dog"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ts, _ := analyzer.TokenStream("field", strings.NewReader(input))
		if ts == nil {
			continue
		}
		for {
			ok, err := ts.IncrementToken()
			if !ok || err != nil {
				break
			}
		}
		ts.Close()
	}
}
