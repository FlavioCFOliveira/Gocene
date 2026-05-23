package vectorhighlight

// abstractTestCase provides shared test fixtures and factory helpers for
// vectorhighlight tests.  It mirrors the role of
// org.apache.lucene.search.vectorhighlight.AbstractTestCase in the Java tree.
//
// In Go the abstract base is implemented as a plain struct that concrete test
// files embed (or use directly) rather than inherit.  There are no setup/
// teardown methods — each test function constructs only what it needs.

import (
	"testing"
)

// Field name constants mirror AbstractTestCase.F, F1, F2.
const (
	testFieldF  = "f"
	testFieldF1 = "f1"
	testFieldF2 = "f2"
)

// Short multi-valued field test data (mirrors shortMVValues).
var shortMVValues = []string{"", "", "a b c", "", "d e"}

// Long multi-valued field test data (mirrors longMVValues).
var longMVValues = []string{
	"Followings are the examples of customizable parameters and actual examples of customization:",
	"The most search engines use only one of these methods. Even the search engines that says they can use the both methods basically",
}

// biMVValues is the test data for the LUCENE-1448 bigram multi-valued case.
var biMVValues = []string{
	"\nLucene/Solr does not require such additional hardware.",
	"\nWhen you talk about processing speed, the",
}

// strMVValues is the multi-valued string (keyword) field test data.
var strMVValues = []string{"abc", "defg", "hijkl"}

// bigramTokenize produces bigram tokens from text using the same logic as
// AbstractTestCase.BigramAnalyzer / BasicNGramTokenizer (n=2, delimiters=" \t\n.,").
// Returns token strings (not BytesRef slices).
func bigramTokenize(text string) []string {
	const n = 2
	delims := " \t\n.,"
	isDelim := func(c rune) bool {
		for _, d := range delims {
			if c == d {
				return true
			}
		}
		return false
	}

	runes := []rune(text)
	var tokens []string
	snippet := []rune{}
	for _, ch := range runes {
		if isDelim(ch) {
			snippet = snippet[:0]
		} else {
			snippet = append(snippet, ch)
			if len(snippet) >= n {
				// emit the last n runes
				tok := string(snippet[len(snippet)-n:])
				tokens = append(tokens, tok)
			}
		}
	}
	return tokens
}

// TestAbstractTestCase_Constants verifies that the shared field name constants
// are defined as expected by the concrete test peers.
func TestAbstractTestCase_Constants(t *testing.T) {
	if testFieldF != "f" {
		t.Errorf("expected F=%q, got %q", "f", testFieldF)
	}
	if testFieldF1 != "f1" {
		t.Errorf("expected F1=%q, got %q", "f1", testFieldF1)
	}
	if testFieldF2 != "f2" {
		t.Errorf("expected F2=%q, got %q", "f2", testFieldF2)
	}
}

// TestAbstractTestCase_ShortMVValues verifies the shape of shortMVValues.
func TestAbstractTestCase_ShortMVValues(t *testing.T) {
	if len(shortMVValues) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(shortMVValues))
	}
	// Non-empty entries at indices 2 and 4.
	cases := []struct {
		idx  int
		want string
	}{
		{2, "a b c"},
		{4, "d e"},
	}
	for _, tc := range cases {
		if shortMVValues[tc.idx] != tc.want {
			t.Errorf("shortMVValues[%d]: want %q, got %q", tc.idx, tc.want, shortMVValues[tc.idx])
		}
	}
}

// TestAbstractTestCase_BigramTokenize verifies the bigram tokenizer produces
// the expected n-grams for the first entry of biMVValues.
func TestAbstractTestCase_BigramTokenize(t *testing.T) {
	// "Lucene/Solr does not require such additional hardware."
	// First token: "Lu", second "uc", etc.
	toks := bigramTokenize("Lucene")
	expected := []string{"Lu", "uc", "ce", "en", "ne"}
	if len(toks) != len(expected) {
		t.Fatalf("bigramTokenize(%q): want %v, got %v", "Lucene", expected, toks)
	}
	for i, want := range expected {
		if toks[i] != want {
			t.Errorf("token[%d]: want %q, got %q", i, want, toks[i])
		}
	}
}

// TestAbstractTestCase_BigramTokenize_WithDelimiters verifies that delimiters
// reset the bigram window.
func TestAbstractTestCase_BigramTokenize_WithDelimiters(t *testing.T) {
	toks := bigramTokenize("ab cd")
	// "ab": {ab}, then delimiter " " resets, "cd": {cd}
	expected := []string{"ab", "cd"}
	if len(toks) != len(expected) {
		t.Fatalf("bigramTokenize(%q): want %v, got %v", "ab cd", expected, toks)
	}
	for i, want := range expected {
		if toks[i] != want {
			t.Errorf("token[%d]: want %q, got %q", i, want, toks[i])
		}
	}
}
