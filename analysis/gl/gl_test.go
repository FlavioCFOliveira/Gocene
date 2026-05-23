// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package gl

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

// tokenize runs input through a WhitespaceTokenizer + filter and returns
// the resulting token strings.
func tokenize(t *testing.T, filter analysis.TokenFilter) []string {
	t.Helper()
	src := filter.(interface {
		GetAttributeSource() *util.AttributeSource
	}).GetAttributeSource()

	termAttr := src.GetAttribute(analysis.CharTermAttributeType)
	if termAttr == nil {
		t.Fatal("CharTermAttribute not found on filter")
	}
	cta := termAttr.(analysis.CharTermAttribute)

	var result []string
	for {
		ok, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken: %v", err)
		}
		if !ok {
			break
		}
		result = append(result, cta.String())
	}
	return result
}

// newMinimalFilter creates a WhitespaceTokenizer + GalicianMinimalStemFilter
// from a single-sentence input string.
func newMinimalFilter(t *testing.T, input string) analysis.TokenFilter {
	t.Helper()
	tok := analysis.NewWhitespaceTokenizer()
	if err := tok.SetReader(strings.NewReader(input)); err != nil {
		t.Fatalf("SetReader: %v", err)
	}
	f, err := NewGalicianMinimalStemFilter(tok)
	if err != nil {
		t.Fatalf("NewGalicianMinimalStemFilter: %v", err)
	}
	return f
}

// newFullFilter creates a WhitespaceTokenizer + GalicianStemFilter from
// a single-sentence input string.
func newFullFilter(t *testing.T, input string) analysis.TokenFilter {
	t.Helper()
	tok := analysis.NewWhitespaceTokenizer()
	if err := tok.SetReader(strings.NewReader(input)); err != nil {
		t.Fatalf("SetReader: %v", err)
	}
	f, err := NewGalicianStemFilter(tok)
	if err != nil {
		t.Fatalf("NewGalicianStemFilter: %v", err)
	}
	return f
}

// checkOneTerm verifies that input stems to expected under the given filter
// constructor.
func checkMinimalOneTerm(t *testing.T, input, expected string) {
	t.Helper()
	tokens := tokenize(t, newMinimalFilter(t, input))
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d: %v", len(tokens), tokens)
	}
	if tokens[0] != expected {
		t.Errorf("input=%q: expected %q, got %q", input, expected, tokens[0])
	}
}

func checkFullOneTerm(t *testing.T, input, expected string) {
	t.Helper()
	tokens := tokenize(t, newFullFilter(t, input))
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d: %v", len(tokens), tokens)
	}
	if tokens[0] != expected {
		t.Errorf("input=%q: expected %q, got %q", input, expected, tokens[0])
	}
}

// ─── GalicianMinimalStemmer tests ────────────────────────────────────────────

// TestGalicianMinimalStemmer_Plural mirrors TestGalicianMinimalStemFilter.testPlural.
func TestGalicianMinimalStemmer_Plural(t *testing.T) {
	tests := []struct{ in, want string }{
		{"elefantes", "elefante"},
		{"elefante", "elefante"},
		{"kalóres", "kalór"},
		{"kalór", "kalór"},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			checkMinimalOneTerm(t, tc.in, tc.want)
		})
	}
}

// TestGalicianMinimalStemmer_Exceptions mirrors TestGalicianMinimalStemFilter.testExceptions.
func TestGalicianMinimalStemmer_Exceptions(t *testing.T) {
	tests := []struct{ in, want string }{
		{"mas", "mas"},
		{"barcelonês", "barcelonês"},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			checkMinimalOneTerm(t, tc.in, tc.want)
		})
	}
}

// TestGalicianMinimalStemmer_EmptyTerm mirrors TestGalicianMinimalStemFilter.testEmptyTerm.
func TestGalicianMinimalStemmer_EmptyTerm(t *testing.T) {
	stemmer, err := NewGalicianMinimalStemmer()
	if err != nil {
		t.Fatalf("NewGalicianMinimalStemmer: %v", err)
	}
	buf := make([]rune, 1)
	if got := stemmer.Stem(buf, 0); got != 0 {
		t.Errorf("empty term: expected length 0, got %d", got)
	}
}

// ─── GalicianStemmer (full) tests ────────────────────────────────────────────

// TestGalicianStemmer_Basic exercises the full stemmer on a handful of
// canonical Galician forms derived from the RSLP vocabulary.
// Expected values verified against the Go stemmer implementation.
func TestGalicianStemmer_Basic(t *testing.T) {
	tests := []struct{ in, want string }{
		// Plural → singular + vowel step removes trailing 'e'
		{"elefantes", "elefant"},
		// Singular form: vowel step removes trailing 'e'
		{"elefante", "elefant"},
		// Plural 'lúas' → vowel removal + accent strip → 'lua'
		{"lúas", "lua"},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			checkFullOneTerm(t, tc.in, tc.want)
		})
	}
}

// TestGalicianStemmer_EmptyTerm verifies graceful handling of empty input.
func TestGalicianStemmer_EmptyTerm(t *testing.T) {
	stemmer, err := NewGalicianStemmer()
	if err != nil {
		t.Fatalf("NewGalicianStemmer: %v", err)
	}
	buf := make([]rune, 1)
	if got := stemmer.Stem(buf, 0); got != 0 {
		t.Errorf("empty term: expected length 0, got %d", got)
	}
}

// TestGalicianStemmer_AccentRemoval verifies the accent-normalisation pass
// at the end of the full stemming pipeline.
// Expected values verified against the Go stemmer implementation.
func TestGalicianStemmer_AccentRemoval(t *testing.T) {
	stemmer, err := NewGalicianStemmer()
	if err != nil {
		t.Fatalf("NewGalicianStemmer: %v", err)
	}
	tests := []struct {
		in   string
		want string
	}{
		// accent strip only (other steps do not fire), á→a
		{"nós", "no"},
		// plural + vowel step + accent strip; cafés → caf (é stripped by vowel step)
		{"cafés", "caf"},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			runes := []rune(tc.in)
			buf := make([]rune, len(runes)+1)
			copy(buf, runes)
			newLen := stemmer.Stem(buf, len(runes))
			got := string(buf[:newLen])
			if got != tc.want {
				t.Errorf("input=%q: expected %q, got %q", tc.in, tc.want, got)
			}
		})
	}
}

// ─── GalicianMinimalStemFilter tests ─────────────────────────────────────────

// TestGalicianMinimalStemFilter_MultiWord verifies that multiple tokens in a
// single stream are all stemmed.
func TestGalicianMinimalStemFilter_MultiWord(t *testing.T) {
	f := newMinimalFilter(t, "elefantes kalóres mas")
	tokens := tokenize(t, f)
	want := []string{"elefante", "kalór", "mas"}
	if len(tokens) != len(want) {
		t.Fatalf("expected %v, got %v", want, tokens)
	}
	for i, tok := range tokens {
		if tok != want[i] {
			t.Errorf("token[%d]: expected %q, got %q", i, want[i], tok)
		}
	}
}

// TestGalicianMinimalStemFilterFactory_Create verifies that the factory
// creates a working filter.
func TestGalicianMinimalStemFilterFactory_Create(t *testing.T) {
	factory, err := NewGalicianMinimalStemFilterFactory(nil)
	if err != nil {
		t.Fatalf("NewGalicianMinimalStemFilterFactory: %v", err)
	}

	tok := analysis.NewWhitespaceTokenizer()
	if err := tok.SetReader(strings.NewReader("elefantes")); err != nil {
		t.Fatalf("SetReader: %v", err)
	}
	f := factory.Create(tok)
	tokens := tokenize(t, f)
	if len(tokens) != 1 || tokens[0] != "elefante" {
		t.Errorf("expected [elefante], got %v", tokens)
	}
}

// ─── GalicianStemFilter tests ─────────────────────────────────────────────────

// TestGalicianStemFilter_MultiWord verifies that multiple tokens are stemmed.
func TestGalicianStemFilter_MultiWord(t *testing.T) {
	f := newFullFilter(t, "elefantes lúas")
	tokens := tokenize(t, f)
	// elefantes: plural+vowel step → "elefant"; lúas: plural+vowel strip → "lua"
	want := []string{"elefant", "lua"}
	if len(tokens) != len(want) {
		t.Fatalf("expected %v, got %v", want, tokens)
	}
	for i, tok := range tokens {
		if tok != want[i] {
			t.Errorf("token[%d]: expected %q, got %q", i, want[i], tok)
		}
	}
}

// TestGalicianStemFilterFactory_Create verifies that the factory creates a
// working filter.
func TestGalicianStemFilterFactory_Create(t *testing.T) {
	factory, err := NewGalicianStemFilterFactory(nil)
	if err != nil {
		t.Fatalf("NewGalicianStemFilterFactory: %v", err)
	}

	tok := analysis.NewWhitespaceTokenizer()
	if err := tok.SetReader(strings.NewReader("elefantes")); err != nil {
		t.Fatalf("SetReader: %v", err)
	}
	f := factory.Create(tok)
	tokens := tokenize(t, f)
	// full stemmer: plural+vowel step → "elefant"
	if len(tokens) != 1 || tokens[0] != "elefant" {
		t.Errorf("expected [elefant], got %v", tokens)
	}
}
