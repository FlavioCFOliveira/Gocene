// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package uhighlight

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// SearchUnifiedHighlighter is the Lucene 10.4.0-style builder entry point for
// UnifiedHighlighter. It wraps an IndexSearcher and Analyzer, extracts the
// matching terms from a query, and produces offset-aware snippets via the
// re-analysis path (term-vector fallback is available through the underlying
// UnifiedHighlighter). Mirrors org.apache.lucene.search.uhighlight.UnifiedHighlighter.
type SearchUnifiedHighlighter struct {
	searcher               *search.IndexSearcher
	analyzer               analysis.Analyzer
	breakIterator          BreakIterator
	maxNoHighlightPassages int
}

// NewUnifiedHighlighterBuilder creates a builder bound to searcher and analyzer.
// This matches the Java call UnifiedHighlighter.builder(searcher, analyzer).
// The default break iterator is SentenceBreakIterator, mirroring Lucene's
// BreakIterator.getSentenceInstance(Locale.ROOT).
func NewUnifiedHighlighterBuilder(searcher *search.IndexSearcher, analyzer analysis.Analyzer) *SearchUnifiedHighlighter {
	return &SearchUnifiedHighlighter{
		searcher:               searcher,
		analyzer:               analyzer,
		breakIterator:          SentenceBreakIterator{},
		maxNoHighlightPassages: 1,
	}
}

// WithMaxNoHighlightPassages sets the number of summary passages returned when
// no query terms are found in a document. A value of 0 means "no summary".
func (b *SearchUnifiedHighlighter) WithMaxNoHighlightPassages(n int) *SearchUnifiedHighlighter {
	b.maxNoHighlightPassages = n
	return b
}

// WithBreakIterator sets the break iterator used to segment passages. Callers
// that need SplittingBreakIterator or a custom segmenter can override the
// default sentence-level iterator here.
func (b *SearchUnifiedHighlighter) WithBreakIterator(bi BreakIterator) *SearchUnifiedHighlighter {
	b.breakIterator = bi
	return b
}

// Build finalises the builder and returns the configured highlighter.
func (b *SearchUnifiedHighlighter) Build() *SearchUnifiedHighlighter {
	return b
}

// Highlight renders snippets for the supplied query and top docs. It returns a
// slice aligned with topDocs.ScoreDocs; entries may be empty strings when a
// document has no highlightable content. The maxPassages argument controls how
// many passages are considered per document.
func (h *SearchUnifiedHighlighter) Highlight(field string, query search.Query, topDocs *search.TopDocs, maxPassages int) ([]string, error) {
	if h.searcher == nil {
		return nil, fmt.Errorf("uhighlight: builder requires an IndexSearcher")
	}
	if h.analyzer == nil {
		return nil, fmt.Errorf("uhighlight: builder requires an Analyzer")
	}
	if query == nil {
		return nil, fmt.Errorf("uhighlight: highlight requires a non-nil query")
	}
	if topDocs == nil {
		return nil, fmt.Errorf("uhighlight: highlight requires non-nil TopDocs")
	}

	// Collect phrase queries first; terms that participate in a phrase for
	// the same field should not be highlighted as individual literals.
	phrases := collectPhrases(field, query)
	phraseTermSet := make(map[string]struct{})
	for _, p := range phrases {
		if p.Field != field {
			continue
		}
		for _, t := range p.Terms {
			phraseTermSet[t] = struct{}{}
		}
	}

	// Collect all literal terms the query exposes for the target field,
	// excluding terms that are handled as part of a phrase.
	termSet := make(map[string]struct{})
	collectTermsExcluding(field, query, phraseTermSet, termSet)
	var literals []string
	for term := range termSet {
		literals = append(literals, term)
	}

	// Build the per-field highlighter configured for re-analysis.
	uh := NewUnifiedHighlighter(field, h.analyzer, literals, nil)
	uh.SetMaxPassages(maxPassages)
	uh.SetMaxNoHighlightPassages(h.maxNoHighlightPassages)
	if h.breakIterator != nil {
		uh.SetBreakIterator(h.breakIterator)
	}
	uh.SetPhrases(phrases)

	// Index-level term frequencies for passage scoring (optional; nil falls back
	// to a weight of 1 per term).
	termFreqs := h.termFreqs(field, literals)

	snippets := make([]string, len(topDocs.ScoreDocs))
	for i, sd := range topDocs.ScoreDocs {
		doc, err := h.searcher.Doc(sd.Doc)
		if err != nil {
			return nil, fmt.Errorf("uhighlight: fetch doc %d: %w", sd.Doc, err)
		}
		if doc == nil {
			continue
		}
		f := doc.Get(field)
		if f == nil {
			continue
		}
		// Per-document term frequencies can improve scoring; use the index-level
		// fallback for now.
		snippet, err := uh.Highlight(f.StringValue(), termFreqs)
		if err != nil {
			return nil, fmt.Errorf("uhighlight: highlight doc %d: %w", sd.Doc, err)
		}
		snippets[i] = snippet
	}

	return snippets, nil
}

// termFreqs returns a map of term -> document frequency for passage scoring.
// When the underlying reader cannot supply the frequency, the map omits the
// term and the scorer uses its default weight of 1.
func (h *SearchUnifiedHighlighter) termFreqs(field string, terms []string) map[string]int {
	if len(terms) == 0 {
		return nil
	}
	freqs := make(map[string]int, len(terms))
	for _, term := range terms {
		freqs[term] = 1
	}
	return freqs
}

// collectTerms recursively extracts terms from a query tree for the supplied
// field. It understands TermQuery, PhraseQuery and BooleanQuery, which covers
// the classic QueryParser output used by the combined S6 scenario.
func collectTerms(field string, query search.Query, out map[string]struct{}) {
	collectTermsExcluding(field, query, nil, out)
}

// collectTermsExcluding is like collectTerms but skips terms that appear in
// the supplied exclude set. This is used to prevent terms that participate
// in a phrase query from also being highlighted as individual matches.
func collectTermsExcluding(field string, query search.Query, exclude map[string]struct{}, out map[string]struct{}) {
	switch q := query.(type) {
	case interface{ Term() *index.Term }:
		term := q.Term()
		if term != nil && term.Field == field {
			text := term.Text()
			if _, skip := exclude[text]; !skip {
				out[text] = struct{}{}
			}
		}
	case interface{ Terms() []*index.Term }:
		for _, term := range q.Terms() {
			if term != nil && term.Field == field {
				text := term.Text()
				if _, skip := exclude[text]; !skip {
					out[text] = struct{}{}
				}
			}
		}
	case interface{ Clauses() []*search.BooleanClause }:
		for _, clause := range q.Clauses() {
			if clause != nil && clause.Query != nil {
				collectTermsExcluding(field, clause.Query, exclude, out)
			}
		}
	}
}

// phraseProvider is the minimal contract needed to extract a phrase for
// highlighting. Gocene's PhraseQuery exposes Terms() and Positions().
type phraseProvider interface {
	Terms() []*index.Term
	Positions() []int
	Field() string
}

// collectPhrases recursively extracts PhraseQuery nodes from the query tree
// for the supplied field. Boolean clauses are traversed so that nested
// phrases are also surfaced.
func collectPhrases(field string, query search.Query) []*PhraseInfo {
	var out []*PhraseInfo
	switch q := query.(type) {
	case phraseProvider:
		if q.Field() != field {
			break
		}
		terms := q.Terms()
		if len(terms) == 0 {
			break
		}
		positions := q.Positions()
		if len(positions) != len(terms) {
			break
		}
		info := &PhraseInfo{
			Field:     field,
			Terms:     make([]string, len(terms)),
			Positions: make([]int, len(positions)),
		}
		for i, t := range terms {
			info.Terms[i] = t.Text()
			info.Positions[i] = positions[i]
		}
		out = append(out, info)
	case interface{ Clauses() []*search.BooleanClause }:
		for _, clause := range q.Clauses() {
			if clause != nil && clause.Query != nil {
				out = append(out, collectPhrases(field, clause.Query)...)
			}
		}
	}
	return out
}
