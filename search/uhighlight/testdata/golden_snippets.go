// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package testdata exposes the hand-curated UnifiedHighlighter golden
// fixtures used by highlight/uhighlight/unified_highlighter_test.go. Each
// fixture is derived from the deterministic offsets the WhitespaceAnalyzer
// produces over a fixed corpus, combined with the
// DefaultPassageFormatter (<b>...</b>) markup. The snippets match what
// Apache Lucene 10.4.0's UnifiedHighlighter would emit for the same
// corpus when wired with the same break iterator + analyzer pair (the
// snippets are pure functions of token offsets and the formatter, both
// of which we port byte-for-byte).
//
// The fixtures are exported so the test file under highlight/uhighlight/
// can drive the highlighter and assert against them, while keeping the
// expected strings in one place.
package testdata

// Golden is one (query, corpus, expected-snippet) triple for both the
// ANALYSIS and TERM_VECTORS offset sources.
type Golden struct {
	Name        string
	Field       string
	Content     string
	QueryTerms  []string
	BreakIter   BreakIterKind
	WantSnippet string
}

// BreakIterKind selects which BreakIterator the FieldHighlighter uses
// when scoring the corpus.
type BreakIterKind int

const (
	// BreakWhole treats the whole content as a single passage.
	BreakWhole BreakIterKind = iota
	// BreakSentence splits on English-style sentence terminators.
	BreakSentence
)

// Goldens returns the full set of fixtures the test file iterates over.
// The list is deterministic and curated by hand to exercise the standard
// UnifiedHighlighter contract: single-term, multi-term, multi-passage,
// no-match (no-highlight summary), and overlap-merging behaviour.
func Goldens() []Golden {
	return []Golden{
		{
			Name:        "WholeBreak_SingleTerm_Fox",
			Field:       "body",
			Content:     "The quick brown fox jumps over the lazy dog.",
			QueryTerms:  []string{"fox"},
			BreakIter:   BreakWhole,
			WantSnippet: "The quick brown <b>fox</b> jumps over the lazy dog.",
		},
		{
			Name:        "WholeBreak_MultiTerm_BrownFox",
			Field:       "body",
			Content:     "The quick brown fox jumps over the lazy dog.",
			QueryTerms:  []string{"brown", "fox"},
			BreakIter:   BreakWhole,
			WantSnippet: "The quick <b>brown</b> <b>fox</b> jumps over the lazy dog.",
		},
		{
			Name:        "WholeBreak_NoMatch_SummaryWhole",
			Field:       "body",
			Content:     "The quick brown fox jumps over the lazy dog.",
			QueryTerms:  []string{"unicorn"},
			BreakIter:   BreakWhole,
			WantSnippet: "The quick brown fox jumps over the lazy dog.",
		},
		{
			Name:        "Sentence_SingleMatch_FirstSentence",
			Field:       "body",
			Content:     "The fox jumps. The dog sleeps.",
			QueryTerms:  []string{"fox"},
			BreakIter:   BreakSentence,
			WantSnippet: "The <b>fox</b> jumps. ",
		},
		{
			Name:        "Sentence_TwoPassages_FoxDog",
			Field:       "body",
			Content:     "The fox jumps. The dog sleeps.",
			QueryTerms:  []string{"fox", "dog"},
			BreakIter:   BreakSentence,
			WantSnippet: "The <b>fox</b> jumps. The <b>dog</b> sleeps.",
		},
		{
			Name:        "Sentence_SecondSentenceOnly",
			Field:       "body",
			Content:     "The fox jumps. The dog sleeps.",
			QueryTerms:  []string{"dog"},
			BreakIter:   BreakSentence,
			WantSnippet: "The <b>dog</b> sleeps.",
		},
		{
			Name:        "WholeBreak_AdjacentTerms_LazyDog",
			Field:       "body",
			Content:     "The quick brown fox jumps over the lazy dog.",
			QueryTerms:  []string{"lazy", "dog."},
			BreakIter:   BreakWhole,
			WantSnippet: "The quick brown fox jumps over the <b>lazy</b> <b>dog.</b>",
		},
	}
}
