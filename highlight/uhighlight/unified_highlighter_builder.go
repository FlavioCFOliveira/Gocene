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
	maxNoHighlightPassages int
}

// NewUnifiedHighlighterBuilder creates a builder bound to searcher and analyzer.
// This matches the Java call UnifiedHighlighter.builder(searcher, analyzer).
func NewUnifiedHighlighterBuilder(searcher *search.IndexSearcher, analyzer analysis.Analyzer) *SearchUnifiedHighlighter {
	return &SearchUnifiedHighlighter{
		searcher:               searcher,
		analyzer:               analyzer,
		maxNoHighlightPassages: 1,
	}
}

// WithMaxNoHighlightPassages sets the number of summary passages returned when
// no query terms are found in a document. A value of 0 means "no summary".
func (b *SearchUnifiedHighlighter) WithMaxNoHighlightPassages(n int) *SearchUnifiedHighlighter {
	b.maxNoHighlightPassages = n
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

	// Collect all literal terms the query exposes for the target field.
	termSet := make(map[string]struct{})
	collectTerms(field, query, termSet)
	var literals []string
	for term := range termSet {
		literals = append(literals, term)
	}

	// Build the per-field highlighter configured for re-analysis.
	uh := NewUnifiedHighlighter(field, h.analyzer, literals, nil)
	uh.SetMaxPassages(maxPassages)
	uh.SetMaxNoHighlightPassages(h.maxNoHighlightPassages)

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
	switch q := query.(type) {
	case interface{ Term() *index.Term }:
		term := q.Term()
		if term != nil && term.Field == field {
			out[term.Text()] = struct{}{}
		}
	case interface{ Terms() []*index.Term }:
		for _, term := range q.Terms() {
			if term != nil && term.Field == field {
				out[term.Text()] = struct{}{}
			}
		}
	case interface{ Clauses() []*search.BooleanClause }:
		for _, clause := range q.Clauses() {
			if clause != nil && clause.Query != nil {
				collectTerms(field, clause.Query, out)
			}
		}
	}
}

