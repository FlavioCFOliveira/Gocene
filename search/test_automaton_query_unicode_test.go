// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestAutomatonQueryUnicode.java

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// buildUnicodeIndex creates the in-memory index used by the unicode automaton
// tests. It mirrors the setUp() of TestAutomatonQueryUnicode, converting Java's
// UTF-16 surrogate pairs to their equivalent Go (UTF-8) codepoints.
//
// Java "𩬅" is the surrogate pair for U+29C05 (𩸅, a CJK unified
// ideograph extension). Java "ﮔ" is U+FB94 (ﮤ, Arabic presentation form).
// Lucene sorts term bytes as UTF-8, which places supplementary characters
// (U+10000 and above, 4-byte UTF-8 sequences) AFTER BMP characters — so 𩸅
// sorts before ﮤ in Lucene's term dictionary even though the UTF-16 code units
// would suggest the opposite.
func buildUnicodeIndex(t *testing.T) (index.IndexReaderInterface, func()) {
	t.Helper()

	const field = "field"

	// Indexed values — each element becomes one document in the field.
	// Source: TestAutomatonQueryUnicode.setUp(), converted from UTF-16 to Go.
	values := []string{
		"\U00029C05abcdef",  // doc 0: U+29C05 = 𩸅 (was 𩬅 surrogate pair)
		"\U00029C06ghijkl",  // doc 1: U+29C06 = 𩸆
		"ﮔmnopqr",      // doc 2: U+FB94  = ﮤ (Arabic presentation form B, Kaf)
		"ﮕstuvwx",      // doc 3: U+FB95  = ﮥ
		"a￼bc",         // doc 4: U+FFFC  = object replacement character
		"a�bc",         // doc 5: U+FFFD  = replacement character
		"a￾bc",         // doc 6: U+FFFE  = non-character BOM
		"aﮔbc",         // doc 7: U+FB94  embedded in ASCII prefix
		"bacadaba",          // doc 8: pure ASCII
		"�",            // doc 9: lone replacement character
		"�\U00029C05",  // doc 10: replacement char + U+29C05
		"��",      // doc 11: two replacement characters
	}

	dir := store.NewByteBuffersDirectory()
	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	for _, v := range values {
		doc := document.NewDocument()
		// Use StringField (not indexed via a tokeniser) so the entire value is one
		// term, matching the Java test which calls newTextField but stores the whole
		// string as a single-token term for exact-match automaton tests.
		f, fErr := document.NewTextField(field, v, false)
		if fErr != nil {
			t.Fatalf("NewTextField(%q): %v", v, fErr)
		}
		doc.Add(f)
		if addErr := w.AddDocument(doc); addErr != nil {
			t.Fatalf("AddDocument(%q): %v", v, addErr)
		}
	}
	if err = w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err = w.Close(); err != nil {
		t.Fatalf("Close writer: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}

	return reader, func() {
		_ = reader.Close()
		_ = dir.Close()
	}
}

// automatonQueryNrHits runs an AutomatonQuery using the given automaton and
// each of the four rewrite methods, asserting that every rewrite yields the
// same hit count.
func automatonQueryNrHits(t *testing.T, searcher *IndexSearcher, a *automaton.Automaton, field string, expected int) {
	t.Helper()

	dummyTerm := index.NewTerm(field, "bogus")

	rewriteMethods := []struct {
		name   string
		method string
	}{
		{"ScoringBoolean", ScoringBooleanRewrite},
		{"ConstantScore", ConstantScoreRewrite},
		{"ConstantScoreBlended", ConstantScoreBlendedRewrite},
		{"ConstantScoreBoolean", ConstantScoreBooleanRewrite},
	}

	for _, rm := range rewriteMethods {
		q := NewAutomatonQueryFull(dummyTerm, a, false, rm.method)
		topDocs, err := searcher.Search(q, 20)
		if err != nil {
			t.Fatalf("[%s] Search: %v", rm.name, err)
		}
		if int(topDocs.TotalHits.Value) != expected {
			t.Errorf("[%s] got %d hits, want %d", rm.name, topDocs.TotalHits.Value, expected)
		}
	}

// TestAutomatonQueryUnicode_SortOrder mirrors testSortOrder.
//
// The expression `(𩸅|ﮤ).*` matches any term that starts with U+29C05 (𩸅, a
// supplementary CJK character that encodes as a 4-byte UTF-8 sequence) or with
// U+FB94 (ﮤ, an Arabic presentation form that encodes as a 3-byte UTF-8
// sequence). In UTF-8 / UTF-32 sort order the supplementary character sorts
// BEFORE the BMP Arabic form, whereas in Java's UTF-16 encoding the surrogate
// pair code units sort AFTER it. This test verifies that Gocene's AutomatonQuery
// observes the correct UTF-8 sort order.
//
// Expected matches: doc 0 (𩸅abcdef) and doc 2 (ﮤmnopqr) → 2 hits.
}
func TestAutomatonQueryUnicode_SortOrder(t *testing.T) {
	reader, cleanup := buildUnicodeIndex(t)
	defer cleanup()

	searcher := NewIndexSearcher(reader)

	// Build the automaton for the regex (𩸅|ﮤ).* using the same RegExp engine
	// that AutomatonQuery accepts internally.
	re, err := automaton.NewRegExp("(\U00029C05|ﮔ).*")
	if err != nil {
		t.Fatalf("NewRegExp: %v", err)
	}
	a, err := re.ToAutomaton()
	if err != nil {
		t.Fatalf("ToAutomaton: %v", err)
	}

	automatonQueryNrHits(t, searcher, a, "field", 2)
}