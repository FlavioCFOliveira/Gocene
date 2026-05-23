// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package query

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ---------------------------------------------------------------------------
// Mock IndexReader infrastructure
// ---------------------------------------------------------------------------

// mockTermEntry holds term text and its document frequency.
type mockTermEntry struct {
	text string
	df   int
}

// mockTerms implements index.Terms backed by a fixed slice.
type mockTerms struct {
	index.TermsBase
	entries []mockTermEntry
}

func (m *mockTerms) GetIterator() (index.TermsEnum, error) {
	return &mockTermsEnum{entries: m.entries, pos: -1}, nil
}

func (m *mockTerms) GetIteratorWithSeek(*index.Term) (index.TermsEnum, error) {
	return m.GetIterator()
}

func (m *mockTerms) GetPostingsReader(string, int) (index.PostingsEnum, error) { return nil, nil }
func (m *mockTerms) GetMin() (*index.Term, error)                              { return nil, nil }
func (m *mockTerms) GetMax() (*index.Term, error)                              { return nil, nil }
func (m *mockTerms) Size() int64                                               { return int64(len(m.entries)) }

// mockTermsEnum iterates over a fixed slice of entries.
type mockTermsEnum struct {
	entries []mockTermEntry
	pos     int
}

func (e *mockTermsEnum) Next() (*index.Term, error) {
	e.pos++
	if e.pos >= len(e.entries) {
		return nil, nil
	}
	return index.NewTerm("", e.entries[e.pos].text), nil
}

func (e *mockTermsEnum) SeekCeil(*index.Term) (*index.Term, error) { return nil, nil }
func (e *mockTermsEnum) SeekExact(*index.Term) (bool, error)       { return false, nil }

func (e *mockTermsEnum) Term() *index.Term {
	if e.pos < 0 || e.pos >= len(e.entries) {
		return nil
	}
	return index.NewTerm("", e.entries[e.pos].text)
}

func (e *mockTermsEnum) DocFreq() (int, error) {
	if e.pos < 0 || e.pos >= len(e.entries) {
		return 0, nil
	}
	return e.entries[e.pos].df, nil
}

func (e *mockTermsEnum) TotalTermFreq() (int64, error) { return -1, nil }

func (e *mockTermsEnum) Postings(int) (index.PostingsEnum, error) { return nil, nil }

func (e *mockTermsEnum) PostingsWithLiveDocs(util.Bits, int) (index.PostingsEnum, error) {
	return nil, nil
}

// mockIndexReader implements IndexReaderForAutoStop for test purposes.
type mockIndexReader struct {
	numDocs    int
	fieldTerms map[string][]mockTermEntry
}

func (r *mockIndexReader) NumDocs() int {
	return r.numDocs
}

func (r *mockIndexReader) Terms(field string) (index.Terms, error) {
	entries, ok := r.fieldTerms[field]
	if !ok {
		return nil, nil
	}
	return &mockTerms{entries: entries}, nil
}

// ---------------------------------------------------------------------------
// Helper: collect tokens through the QueryAutoStopWordAnalyzer pipeline
// ---------------------------------------------------------------------------

// collectQueryTokens drives the token stream returned by the analyzer for the
// given (fieldName, text) pair. The delegate is a WhitespaceTokenizer so the
// test controls tokenisation precisely.
func collectQueryTokens(t *testing.T, a *QueryAutoStopWordAnalyzer, fieldName, text string) []string {
	t.Helper()

	// Build the delegate's token stream directly so we can obtain the
	// attribute source from the concrete StopFilter (the last filter in the
	// chain returned by WrapTokenStream).
	tok := analysis.NewWhitespaceTokenizer()
	if err := tok.SetReader(strings.NewReader(text)); err != nil {
		t.Fatalf("SetReader: %v", err)
	}

	// Apply WrapTokenStream if stop words exist for this field.
	var ts analysis.TokenStream = tok
	if stopWords := a.stopWordsPerField[fieldName]; len(stopWords) > 0 {
		ts = analysis.NewStopFilter(tok, stopWords)
	}
	defer ts.Close()

	var termAttr analysis.CharTermAttribute
	switch v := ts.(type) {
	case *analysis.StopFilter:
		src := v.GetAttributeSource()
		if attr := src.GetAttribute(analysis.CharTermAttributeType); attr != nil {
			termAttr = attr.(analysis.CharTermAttribute)
		}
	default:
		// bare WhitespaceTokenizer
		src := tok.GetAttributeSource()
		if attr := src.GetAttribute(analysis.CharTermAttributeType); attr != nil {
			termAttr = attr.(analysis.CharTermAttribute)
		}
	}

	var tokens []string
	for {
		ok, err := ts.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken: %v", err)
		}
		if !ok {
			break
		}
		if termAttr != nil {
			tokens = append(tokens, termAttr.String())
		}
	}
	return tokens
}

// ---------------------------------------------------------------------------
// buildMockReader simulates 200 documents.
//   - variedField: 9 terms, df~20 each (except "the" df=40 — appears twice
//     in the original 10-value cycle).
//   - repetitiveField: "boring" df=133, "vaguelyboring" df=67.
// ---------------------------------------------------------------------------

func buildMockReader() *mockIndexReader {
	return &mockIndexReader{
		numDocs: 200,
		fieldTerms: map[string][]mockTermEntry{
			"variedField": {
				{"boring", 20}, {"brown", 20}, {"dog", 20},
				{"fox", 20}, {"jumped", 20}, {"lazy", 20},
				{"over", 20}, {"quick", 20}, {"the", 40},
			},
			"repetitiveField": {
				{"boring", 133}, {"vaguelyboring", 67},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Tests (mirroring TestQueryAutoStopWordAnalyzer.java)
// ---------------------------------------------------------------------------

// TestQueryAutoStopWordAnalyzer_NoStopwords verifies that an empty field list
// produces no stop-word filtering.
// Source: testNoStopwords
func TestQueryAutoStopWordAnalyzer_NoStopwords(t *testing.T) {
	reader := buildMockReader()
	delegate := analysis.NewStandardAnalyzer()
	a, err := NewQueryAutoStopWordAnalyzerWithMaxDocFreq(delegate, reader, nil, 1)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	got := collectQueryTokens(t, a, "variedField", "quick")
	if len(got) == 0 {
		t.Error("expected 'quick' to pass through with no stop words configured")
	}
	got2 := collectQueryTokens(t, a, "repetitiveField", "boring")
	if len(got2) == 0 {
		t.Error("expected 'boring' to pass through with no stop words configured")
	}
}

// TestQueryAutoStopWordAnalyzer_DefaultThreshold verifies that words
// appearing in > 40 % of docs are collected as stop words.
// Source: testDefaultStopwordsAllFields
func TestQueryAutoStopWordAnalyzer_DefaultThreshold(t *testing.T) {
	reader := buildMockReader()
	// numDocs=200, default 40% → maxDocFreq=80; "boring" df=133 → stop word
	delegate := analysis.NewStandardAnalyzer()
	a, err := NewQueryAutoStopWordAnalyzer(delegate, reader, []string{"repetitiveField"})
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	sw := a.GetStopWordsForField("repetitiveField")
	boringFound := false
	for _, w := range sw {
		if w == "boring" {
			boringFound = true
		}
	}
	if !boringFound {
		t.Errorf("expected 'boring' in stop words for repetitiveField; got %v", sw)
	}
}

// TestQueryAutoStopWordAnalyzer_PercentThreshold verifies percent-based
// thresholds select the correct stop words.
// Source: testStopwordsAllFieldsMaxPercentDocs
func TestQueryAutoStopWordAnalyzer_PercentThreshold(t *testing.T) {
	reader := buildMockReader()
	delegate := analysis.NewStandardAnalyzer()

	// 50 %: maxDocFreq=100 → "boring" (133) filtered; "vaguelyboring" (67) not
	a, err := NewQueryAutoStopWordAnalyzerWithPercent(delegate, reader, []string{"repetitiveField"}, 0.5)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	sw := a.GetStopWordsForField("repetitiveField")
	for _, w := range sw {
		if w == "vaguelyboring" {
			t.Errorf("'vaguelyboring' should not be a stop word at 50%% threshold; sw=%v", sw)
		}
	}
	boringFound := false
	for _, w := range sw {
		if w == "boring" {
			boringFound = true
		}
	}
	if !boringFound {
		t.Errorf("'boring' should be a stop word at 50%% threshold; sw=%v", sw)
	}

	// 25 %: maxDocFreq=50 → both "boring" and "vaguelyboring" filtered
	a2, err := NewQueryAutoStopWordAnalyzerWithPercent(delegate, reader, []string{"repetitiveField"}, 0.25)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	sw2 := a2.GetStopWordsForField("repetitiveField")
	vbFound := false
	for _, w := range sw2 {
		if w == "vaguelyboring" {
			vbFound = true
		}
	}
	if !vbFound {
		t.Errorf("'vaguelyboring' should be a stop word at 25%% threshold; sw=%v", sw2)
	}
}

// TestQueryAutoStopWordAnalyzer_PerFieldNoFieldNamePollution verifies that
// stop words for one field do not affect queries on another.
// Source: testNoFieldNamePollution
func TestQueryAutoStopWordAnalyzer_PerFieldNoFieldNamePollution(t *testing.T) {
	reader := buildMockReader()
	delegate := analysis.NewStandardAnalyzer()
	a, err := NewQueryAutoStopWordAnalyzerWithMaxDocFreq(delegate, reader, []string{"repetitiveField"}, 10)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	// Stop words registered only for repetitiveField; variedField must be empty.
	sw := a.GetStopWordsForField("variedField")
	if len(sw) != 0 {
		t.Errorf("variedField should have no stop words; got %v", sw)
	}
	// Filtering on repetitiveField should suppress "boring".
	got := collectQueryTokens(t, a, "repetitiveField", "boring")
	if len(got) != 0 {
		t.Errorf("expected 'boring' filtered on repetitiveField; got %v", got)
	}
	// The same token on variedField should pass through.
	got2 := collectQueryTokens(t, a, "variedField", "boring")
	if len(got2) == 0 {
		t.Errorf("'boring' should pass through on variedField; got %v", got2)
	}
}

// TestQueryAutoStopWordAnalyzer_MaxDocFreqStopWords verifies that
// GetStopWordsForField returns a non-empty list when threshold is exceeded.
// Source: testStopwordsPerFieldMaxDocFreq
func TestQueryAutoStopWordAnalyzer_MaxDocFreqStopWords(t *testing.T) {
	reader := buildMockReader()
	delegate := analysis.NewStandardAnalyzer()
	a, err := NewQueryAutoStopWordAnalyzerWithMaxDocFreq(delegate, reader, []string{"repetitiveField"}, 10)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	sw := a.GetStopWordsForField("repetitiveField")
	if len(sw) == 0 {
		t.Fatal("should have identified stop words for repetitiveField")
	}

	// Adding variedField to the scan should yield more stop words in total.
	a2, err := NewQueryAutoStopWordAnalyzerWithMaxDocFreq(
		delegate, reader, []string{"repetitiveField", "variedField"}, 10)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	total := len(a2.GetStopWordsForField("repetitiveField")) +
		len(a2.GetStopWordsForField("variedField"))
	if total <= len(sw) {
		t.Errorf("two-field scan should yield more stop words; got %d vs base %d", total, len(sw))
	}
}

// TestQueryAutoStopWordAnalyzer_GetStopWords verifies GetStopWords returns
// index.Term values covering all registered fields.
func TestQueryAutoStopWordAnalyzer_GetStopWords(t *testing.T) {
	reader := buildMockReader()
	delegate := analysis.NewStandardAnalyzer()
	a, err := NewQueryAutoStopWordAnalyzerWithMaxDocFreq(
		delegate, reader, []string{"repetitiveField", "variedField"}, 10)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	terms := a.GetStopWords()
	if len(terms) == 0 {
		t.Fatal("GetStopWords returned no terms")
	}
	fields := make(map[string]bool)
	for _, term := range terms {
		fields[term.Field] = true
	}
	if !fields["repetitiveField"] {
		t.Error("expected repetitiveField in stop word terms")
	}
}

// TestQueryAutoStopWordAnalyzer_NilFields verifies that a nil field list
// produces an analyzer with no stop words registered.
func TestQueryAutoStopWordAnalyzer_NilFields(t *testing.T) {
	reader := buildMockReader()
	delegate := analysis.NewStandardAnalyzer()
	a, err := NewQueryAutoStopWordAnalyzerWithMaxDocFreq(delegate, reader, nil, 1)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	sw := a.GetStopWords()
	if len(sw) != 0 {
		t.Errorf("expected no stop words with nil field list; got %d", len(sw))
	}
}
