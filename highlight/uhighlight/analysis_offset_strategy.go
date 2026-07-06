// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package uhighlight

import (
	"fmt"
	"sort"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// AnalysisDocContext is the docContext payload understood by
// AnalysisOffsetStrategy: it carries the Analyzer that should be re-run
// over the raw field text plus the text itself.
//
// The strategy walks the resulting TokenStream, matches each token's
// term against the query's term set (as exposed by UHComponents.Matchers
// plus the literal matchers seeded by the strategy constructor), and
// records the matching (start, end) offsets.
type AnalysisDocContext struct {
	// Analyzer is the field-level analyzer used to (re-)tokenise the
	// content. Must not be nil.
	Analyzer analysis.Analyzer

	// Content is the raw field value to highlight.
	Content string

	// TermFreqsInDoc records the per-term frequency observed in the
	// surrounding document. When the map is nil or a term is missing,
	// the strategy substitutes 1 (the matched occurrence itself), which
	// matches Lucene's "single-doc analysis" fallback.
	TermFreqsInDoc map[string]int
}

// AnalysisOffsetStrategy resolves offsets by re-running the field
// analyzer over the raw content and matching tokens against the query's
// literal + automaton term set.
//
// Mirrors org.apache.lucene.search.uhighlight.AnalysisOffsetStrategy at
// the level of abstraction the Go uhighlight package consumes (the
// Lucene reference also handles MemoryIndex-backed positional queries,
// which the Go port defers to a follow-up sprint).
type AnalysisOffsetStrategy struct {
	BaseFieldOffsetStrategy
	literals []string
	matchers []CharArrayMatcher
	phrases  []*PhraseInfo
}

// NewAnalysisOffsetStrategy returns the analysis strategy for field.
// The optional second argument is the list of single-term matchers
// (exact match against the analyzer-produced token text); the optional
// third argument is the list of automaton-based matchers (for
// MultiTermQuery support). Either list may be empty; if both are empty
// the resulting strategy always returns an empty enum (which the
// FieldHighlighter renders as the no-highlight summary).
//
// The single-argument form is retained for back-compat with the
// scaffolding tests that only exercise GetOffsetSource.
func NewAnalysisOffsetStrategy(field string, sel ...AnalysisSelector) *AnalysisOffsetStrategy {
	s := &AnalysisOffsetStrategy{
		BaseFieldOffsetStrategy: NewBaseFieldOffsetStrategy(field),
	}
	for _, opt := range sel {
		opt(s)
	}
	return s
}

// AnalysisSelector configures the matcher set of an AnalysisOffsetStrategy.
// Use WithAnalysisLiterals / WithAnalysisMatchers at the call site.
type AnalysisSelector func(*AnalysisOffsetStrategy)

// WithAnalysisLiterals registers a list of literal-term matchers.
func WithAnalysisLiterals(literals ...string) AnalysisSelector {
	return func(s *AnalysisOffsetStrategy) {
		s.literals = append(s.literals, literals...)
	}
}

// WithAnalysisMatchers registers a list of CharArrayMatcher (typically
// automaton-backed) matchers.
func WithAnalysisMatchers(matchers ...CharArrayMatcher) AnalysisSelector {
	return func(s *AnalysisOffsetStrategy) {
		s.matchers = append(s.matchers, matchers...)
	}
}

// WithAnalysisPhrases registers phrase queries that should be highlighted as
// contiguous spans rather than as individual term matches.
func WithAnalysisPhrases(phrases ...*PhraseInfo) AnalysisSelector {
	return func(s *AnalysisOffsetStrategy) {
		s.phrases = append(s.phrases, phrases...)
	}
}

// GetOffsetSource returns OffsetSourceAnalysis.
func (s *AnalysisOffsetStrategy) GetOffsetSource() OffsetSource { return OffsetSourceAnalysis }

// tokenOffset holds the per-token information we need to detect phrase
// matches while walking a re-analyzed field.
type tokenOffset struct {
	term     string
	start    int
	end      int
	position int
}

// GetOffsetsEnum re-tokenises docContext.Content and returns an
// OffsetsEnum walking the matched tokens in document order.
func (s *AnalysisOffsetStrategy) GetOffsetsEnum(docContext any) (OffsetsEnum, error) {
	ctx, ok := docContext.(*AnalysisDocContext)
	if !ok || ctx == nil {
		return nil, fmt.Errorf("uhighlight: AnalysisOffsetStrategy expects *AnalysisDocContext, got %T", docContext)
	}
	if ctx.Analyzer == nil {
		return nil, fmt.Errorf("uhighlight: AnalysisOffsetStrategy requires a non-nil Analyzer")
	}
	if len(s.literals) == 0 && len(s.matchers) == 0 && len(s.phrases) == 0 {
		return NewSliceOffsetsEnum(nil), nil
	}

	stream, err := ctx.Analyzer.TokenStream(s.Field(), strings.NewReader(ctx.Content))
	if err != nil {
		return nil, fmt.Errorf("uhighlight: analyzer TokenStream: %w", err)
	}
	defer func() { _ = stream.Close() }()

	src := attributeSourceFor(stream)
	if src == nil {
		return nil, fmt.Errorf("uhighlight: TokenStream %T does not expose an AttributeSource", stream)
	}
	termAttr, _ := src.GetAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	offsetAttr, _ := src.GetAttribute(analysis.OffsetAttributeType).(analysis.OffsetAttribute)
	posIncrAttr, _ := src.GetAttribute(analysis.PositionIncrementAttributeType).(analysis.PositionIncrementAttribute)
	if termAttr == nil || offsetAttr == nil {
		return nil, fmt.Errorf("uhighlight: TokenStream missing CharTerm/Offset attributes (term=%T offset=%T)", termAttr, offsetAttr)
	}

	var tokens []tokenOffset
	pos := -1
	for {
		more, err := stream.IncrementToken()
		if err != nil {
			return nil, fmt.Errorf("uhighlight: IncrementToken: %w", err)
		}
		if !more {
			break
		}
		incr := 1
		if posIncrAttr != nil {
			incr = posIncrAttr.GetPositionIncrement()
		}
		if pos < 0 {
			pos = incr
		} else {
			pos += incr
		}
		tokens = append(tokens, tokenOffset{
			term:     termAttr.String(),
			start:    offsetAttr.StartOffset(),
			end:      offsetAttr.EndOffset(),
			position: pos,
		})
	}
	_ = stream.End()

	consumed := make([]bool, len(tokens))
	var entries []OffsetEntry

	// Emit phrase spans first so overlapping single-term matches for the
	// same offsets are skipped.
	for _, phrase := range s.phrases {
		if len(phrase.Terms) == 0 || len(phrase.Positions) != len(phrase.Terms) {
			continue
		}
		for i := 0; i < len(tokens); i++ {
			start := tokens[i].position
			endPos := start
			match := true
			for j, want := range phrase.Terms {
				idx := i + j
				if idx >= len(tokens) {
					match = false
					break
				}
				got := tokens[idx].term
				if got != want {
					match = false
					break
				}
				if j == 0 {
					endPos = start + phrase.Positions[j]
				} else {
					expectedPos := start + phrase.Positions[j]
					if tokens[idx].position != expectedPos {
						match = false
						break
					}
					if tokens[idx].position > endPos {
						endPos = tokens[idx].position
					}
				}
			}
			if !match {
				continue
			}
			spanStart := tokens[i].start
			spanEnd := tokens[i+len(phrase.Terms)-1].end
			for j := 0; j < len(phrase.Terms); j++ {
				consumed[i+j] = true
			}
			entries = append(entries, OffsetEntry{
				Term:        "",
				StartOffset: spanStart,
				EndOffset:   spanEnd,
				Weight:      1,
			})
		}
	}

	for i, tok := range tokens {
		if consumed[i] {
			continue
		}
		if !s.termMatches(tok.term) {
			continue
		}
		entries = append(entries, OffsetEntry{
			Term:        tok.term,
			StartOffset: tok.start,
			EndOffset:   tok.end,
			Weight:      lookupFreq(ctx.TermFreqsInDoc, tok.term, 1),
		})
	}

	// Defensive sort: in-order by construction, but graph synonyms can
	// re-order. FieldHighlighter requires ascending start-offsets.
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].StartOffset < entries[j].StartOffset
	})
	return NewSliceOffsetsEnum(entries), nil
}

// termMatches reports whether any literal or automaton matcher accepts
// the supplied token.
func (s *AnalysisOffsetStrategy) termMatches(term string) bool {
	for _, lit := range s.literals {
		if term == lit {
			return true
		}
	}
	if len(s.matchers) == 0 {
		return false
	}
	chars := []rune(term)
	for _, m := range s.matchers {
		if m == nil {
			continue
		}
		if m.Match(chars, 0, len(chars)) {
			return true
		}
	}
	return false
}

// lookupFreq returns m[term] cast to float32 or fallback if absent.
func lookupFreq(m map[string]int, term string, fallback float32) float32 {
	if v, ok := m[term]; ok {
		return float32(v)
	}
	return fallback
}

// attributeSourceFor returns the *util.AttributeSource carried by the
// supplied TokenStream. Every Gocene analyzer output embeds
// *analysis.BaseTokenStream and inherits GetAttributeSource() via method
// promotion; the helper accepts either the embedding pointer directly or
// any wrapper that publishes the same accessor.
func attributeSourceFor(stream analysis.TokenStream) *util.AttributeSource {
	type sourceProvider interface {
		GetAttributeSource() *util.AttributeSource
	}
	if sp, ok := stream.(sourceProvider); ok {
		return sp.GetAttributeSource()
	}
	return nil
}

// Ensure AnalysisOffsetStrategy satisfies the contract.
var _ FieldOffsetStrategy = (*AnalysisOffsetStrategy)(nil)
