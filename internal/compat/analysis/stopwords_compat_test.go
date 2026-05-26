// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// stopwords_compat_test.go addresses the audit row (verbatim from
// docs/compat-coverage.tsv):
//
//	"Stop-word/keyword set persistence"
//	    lucene_class: org.apache.lucene.analysis.WordlistLoader
//	    gap_notes:    "No fixture-based check against Lucene-shipped
//	                   wordlists."
//
// Lucene ships the canonical English stop-word set inside
// lucene-analysis-common as a static field on EnglishAnalyzer (same
// 33-word list also exposed by StandardAnalyzer.STOP_WORDS_SET). The set
// is NOT serialised as a binary blob in the index; the audit row exists
// because the textual wordlist is part of Lucene's persisted contract
// (analyzer factories deserialise these wordlists from text resources at
// open-time and the resulting set must match Lucene's byte-for-byte to
// produce equivalent indexes).
//
// We therefore close this row with a TEXT-FIXTURE parity check: this
// test pins Gocene's analysis.EnglishStopWords against the exact same
// 33-word list Lucene 10.4 hard-codes. If either side drifts, this test
// fails. The list is small enough to inline; the audit-row contract is
// "both engines agree on the canonical wordlist", and inlining the
// expected list is the most direct way to enforce that. The test does
// NOT require the Java harness jar; it is compat-tagged so it runs in
// the same suite as the JAR-backed scenarios.
//
// Why not call the harness for this row? A "dump-stopwords" Java
// subcommand would add ~50 LOC of CLI plumbing to dump a list that is
// statically defined in lucene-analysis-common as a literal initialiser;
// the Java-side ScenarioDeterminismTest contract (byte-determinism
// across seeds) is meaningless for a constant set. The row's gap_notes
// asks only for a "fixture-based check"; the inlined byte-identical
// list IS a text fixture. If a future task wants the JAR-side dump it
// can be added without touching this file.
package analysis

import (
	"strings"
	"testing"

	gocene "github.com/FlavioCFOliveira/Gocene/analysis"
)

// luceneEnglishStopWords is the canonical 33-word English stop set
// shipped by Apache Lucene 10.4.0 inside lucene-analysis-common as
//
//	EnglishAnalyzer.ENGLISH_STOP_WORDS_SET
//	(== StandardAnalyzer.STOP_WORDS_SET)
//
// Reproduced verbatim from lucene/analysis/common/src/java/org/apache/
// lucene/analysis/en/EnglishAnalyzer.java (release tag
// releases/lucene/10.4.0, commit 9983b7c).
var luceneEnglishStopWords = []string{
	"a", "an", "and", "are", "as", "at", "be", "but", "by",
	"for", "if", "in", "into", "is", "it", "no", "not", "of",
	"on", "or", "such", "that", "the", "their", "then", "there",
	"these", "they", "this", "to", "was", "will", "with",
}

// TestEnglishStopWords_LuceneParity is the lone test class for this
// audit row. It compares Gocene's analysis.EnglishStopWords against the
// pinned Lucene 10.4 list, position-by-position. Order matters because
// every CharArraySet downstream consumer (CharArraySet.copy /
// WordlistLoader.getWordSet) preserves insertion order; a re-ordering
// would change CharArraySet's internal probe sequence and is a
// silently-observable divergence.
func TestEnglishStopWords_LuceneParity(t *testing.T) {
	got := gocene.EnglishStopWords
	if len(got) != len(luceneEnglishStopWords) {
		t.Fatalf("EnglishStopWords length mismatch: got %d, want %d (Lucene 10.4 ENGLISH_STOP_WORDS_SET)",
			len(got), len(luceneEnglishStopWords))
	}
	for i, want := range luceneEnglishStopWords {
		if got[i] != want {
			t.Errorf("EnglishStopWords[%d] = %q; want %q (Lucene 10.4 parity)", i, got[i], want)
		}
	}
}

// TestEnglishStopWords_SetMembership is a second-axis sanity check that
// exercises the CharArraySet built from EnglishStopWords (Gocene's
// idiomatic consumer pattern) and confirms membership semantics match
// Lucene's case-insensitive behaviour for the canonical set.
func TestEnglishStopWords_SetMembership(t *testing.T) {
	set := gocene.GetWordSetFromStrings(gocene.EnglishStopWords, true)
	if set == nil {
		t.Fatalf("GetWordSetFromStrings returned nil")
	}
	for _, word := range luceneEnglishStopWords {
		if !set.ContainsString(word) {
			t.Errorf("stop set missing %q", word)
		}
		// Case-insensitive contract: Lucene's StopFilter folds the input
		// to lower-case before lookup; the upper-case form MUST also be
		// reported as present when the set is built with ignoreCase=true.
		upper := strings.ToUpper(word)
		if !set.ContainsString(upper) {
			t.Errorf("stop set missing case-folded %q", upper)
		}
	}
}
