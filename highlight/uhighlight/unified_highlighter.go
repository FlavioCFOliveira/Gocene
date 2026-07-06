// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package uhighlight

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// UnifiedHighlighter is the top-level entry point that consumes a
// per-field offsets source (re-analysis or term-vector) and emits a
// formatted snippet for a single document at a time.
//
// This is the Gocene-internal slice of
// org.apache.lucene.search.uhighlight.UnifiedHighlighter that supports
// OffsetSourceAnalysis and OffsetSourceTermVectors. The POSTINGS and
// MEMORY_INDEX paths remain deferred (the corresponding strategies in
// strategies.go return errNotImplemented).
//
// A UnifiedHighlighter is configured with literal + automaton matchers
// derived from a query and an Analyzer used to re-tokenise content for
// the ANALYSIS path. Callers drive it through one call per document via
// Highlight or HighlightTermVector.
type UnifiedHighlighter struct {
	field            string
	analyzer         analysis.Analyzer
	literals         []string
	matchers         []CharArrayMatcher
	phrases          []*PhraseInfo
	scorer           *PassageScorer
	formatter        PassageFormatter
	breakIter        BreakIterator
	maxPassages      int
	maxNoHighlight   int
}

// NewUnifiedHighlighter constructs a highlighter for field using the
// supplied analyzer (used only when ANALYSIS is selected). Literals and
// matchers form the term selection set; either may be empty.
func NewUnifiedHighlighter(field string, analyzer analysis.Analyzer, literals []string, matchers []CharArrayMatcher) *UnifiedHighlighter {
	return &UnifiedHighlighter{
		field:          field,
		analyzer:       analyzer,
		literals:       append([]string(nil), literals...),
		matchers:       append([]CharArrayMatcher(nil), matchers...),
		scorer:         NewPassageScorer(),
		formatter:      NewDefaultPassageFormatter(),
		breakIter:      SplittingBreakIterator{},
		maxPassages:    3,
		maxNoHighlight: 1,
	}
}

// SetMaxPassages overrides the default (3) top-K passage count.
func (h *UnifiedHighlighter) SetMaxPassages(n int) {
	if n < 1 {
		n = 1
	}
	h.maxPassages = n
}

// SetMaxNoHighlightPassages overrides the default (1) no-highlight
// summary count. A value of -1 means "use MaxPassages".
func (h *UnifiedHighlighter) SetMaxNoHighlightPassages(n int) { h.maxNoHighlight = n }

// SetPhrases registers phrase queries that should be rendered as
// contiguous highlighted spans rather than individual term matches.
func (h *UnifiedHighlighter) SetPhrases(phrases []*PhraseInfo) { h.phrases = phrases }

// SetPassageScorer overrides the default PassageScorer.
func (h *UnifiedHighlighter) SetPassageScorer(s *PassageScorer) {
	if s != nil {
		h.scorer = s
	}
}

// SetPassageFormatter overrides the default DefaultPassageFormatter.
func (h *UnifiedHighlighter) SetPassageFormatter(f PassageFormatter) {
	if f != nil {
		h.formatter = f
	}
}

// SetBreakIterator overrides the default SplittingBreakIterator.
func (h *UnifiedHighlighter) SetBreakIterator(bi BreakIterator) {
	if bi != nil {
		h.breakIter = bi
	}
}

// Field returns the target field name.
func (h *UnifiedHighlighter) Field() string { return h.field }

// Highlight renders a snippet for the supplied content using the
// re-analysis path (OffsetSourceAnalysis). termFreqs is optional and
// carries the per-term doc-frequency counters used by the PassageScorer;
// passing nil substitutes the value 1 for every match.
func (h *UnifiedHighlighter) Highlight(content string, termFreqs map[string]int) (string, error) {
	if h.analyzer == nil {
		return "", errors.New("uhighlight: UnifiedHighlighter requires an Analyzer for the ANALYSIS path")
	}
	strat := NewAnalysisOffsetStrategy(
		h.field,
		WithAnalysisLiterals(h.literals...),
		WithAnalysisMatchers(h.matchers...),
		WithAnalysisPhrases(h.phrases...),
	)
	fh := NewFieldHighlighter(
		h.field, strat, h.breakIter, h.scorer,
		h.maxPassages, h.maxNoHighlight, h.formatter,
	)
	ctx := &AnalysisDocContext{
		Analyzer:       h.analyzer,
		Content:        content,
		TermFreqsInDoc: termFreqs,
	}
	snippet, err := fh.HighlightFieldForDoc(ctx, content)
	if err != nil {
		return "", fmt.Errorf("uhighlight: highlight field %q: %w", h.field, err)
	}
	return snippet, nil
}

// HighlightTermVector renders a snippet for the supplied content using
// stored term vectors WITH_OFFSETS. The entries argument is the
// per-document term-vector list produced by the codec; it must include
// StartOffsets / EndOffsets for every entry.
func (h *UnifiedHighlighter) HighlightTermVector(content string, entries []TermVectorEntry, termFreqs map[string]int) (string, error) {
	strat := NewTermVectorOffsetStrategy(
		h.field,
		WithTermVectorLiterals(h.literals...),
		WithTermVectorMatchers(h.matchers...),
	)
	fh := NewFieldHighlighter(
		h.field, strat, h.breakIter, h.scorer,
		h.maxPassages, h.maxNoHighlight, h.formatter,
	)
	ctx := &TermVectorDocContext{
		Entries:        entries,
		TermFreqsInDoc: termFreqs,
	}
	snippet, err := fh.HighlightFieldForDoc(ctx, content)
	if err != nil {
		return "", fmt.Errorf("uhighlight: highlight field %q (term-vectors): %w", h.field, err)
	}
	return snippet, nil
}
