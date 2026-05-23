package grouping

// abstractGroupingTestCase contains test helpers shared by grouping-related
// test suites. It is the Go counterpart of
// org.apache.lucene.search.grouping.AbstractGroupingTestCase from Lucene 10.4.0.
//
// Java's LuceneTestCase infrastructure (RandomIndexWriter, random Directory, etc.)
// is not yet available in Gocene, so this file provides lightweight equivalents
// that exercise the same assertion semantics.

import (
	"math/rand/v2"
	"strings"
	"testing"
	"unicode/utf8"
)

// generateRandomNonEmptyString returns a random non-empty Unicode string.
// Mirrors AbstractGroupingTestCase.generateRandomNonEmptyString.
func generateRandomNonEmptyString() string {
	const maxLen = 20
	for {
		n := rand.IntN(maxLen) + 1
		var sb strings.Builder
		for i := 0; i < n; i++ {
			// Generate a random printable Unicode code point in the BMP.
			cp := rune(rand.IntN(0x7F-0x20) + 0x20)
			sb.WriteRune(cp)
		}
		s := sb.String()
		if utf8.RuneCountInString(s) > 0 && s != "" {
			return s
		}
	}
}

// scorePair is a lightweight analogue of Lucene's ScoreDoc used in
// assertGroupScoreDocsEqual.
type scorePair struct {
	doc   int
	score float32
}

// assertGroupScoreDocsEqual verifies that two slices of scorePair are equal
// in doc ID and score, mirroring
// AbstractGroupingTestCase.assertScoreDocsEquals.
func assertGroupScoreDocsEqual(t *testing.T, expected, actual []scorePair) {
	t.Helper()
	if len(expected) != len(actual) {
		t.Fatalf("assertGroupScoreDocsEqual: len expected=%d actual=%d",
			len(expected), len(actual))
	}
	for i := range expected {
		if expected[i].doc != actual[i].doc {
			t.Errorf("[%d] doc: expected=%d actual=%d", i, expected[i].doc, actual[i].doc)
		}
		if expected[i].score != actual[i].score {
			t.Errorf("[%d] score: expected=%f actual=%f", i, expected[i].score, actual[i].score)
		}
	}
}

// TestAbstractGroupingTestCase_GenerateRandomNonEmptyString verifies that
// generateRandomNonEmptyString never returns an empty string over many calls.
// Mirrors the intent of the Java base-class contract.
func TestAbstractGroupingTestCase_GenerateRandomNonEmptyString(t *testing.T) {
	for i := 0; i < 1000; i++ {
		s := generateRandomNonEmptyString()
		if s == "" {
			t.Fatalf("iteration %d: generateRandomNonEmptyString returned empty string", i)
		}
		if utf8.RuneCountInString(s) == 0 {
			t.Fatalf("iteration %d: generated string has zero runes: %q", i, s)
		}
	}
}

// TestAbstractGroupingTestCase_AssertGroupScoreDocsEqual verifies the
// assertGroupScoreDocsEqual helper passes on equal input, mirroring
// AbstractGroupingTestCase.assertScoreDocsEquals.
func TestAbstractGroupingTestCase_AssertGroupScoreDocsEqual(t *testing.T) {
	// Equal sets — must not fail.
	a := []scorePair{{doc: 1, score: 1.0}, {doc: 2, score: 0.5}}
	b := []scorePair{{doc: 1, score: 1.0}, {doc: 2, score: 0.5}}
	assertGroupScoreDocsEqual(t, a, b)

	// Empty equal sets.
	assertGroupScoreDocsEqual(t, []scorePair{}, []scorePair{})

	// Single element equal.
	assertGroupScoreDocsEqual(t,
		[]scorePair{{doc: 7, score: 0.9}},
		[]scorePair{{doc: 7, score: 0.9}},
	)
}
