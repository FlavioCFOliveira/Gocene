// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package uhighlight

import (
	"fmt"
	"sort"
)

// TermVectorDocContext is the docContext payload understood by
// TermVectorOffsetStrategy: it carries the per-document term-vector
// entries that the strategy walks to produce offsets.
//
// The entries must include StartOffsets/EndOffsets (i.e. the underlying
// field was indexed with TermVectors WITH_OFFSETS). When offsets are
// absent, the strategy returns an error so the caller can fall back to
// the analysis path.
type TermVectorDocContext struct {
	// Entries is the per-document term-vector list. Each entry's
	// occurrence count (Frequency) must match len(StartOffsets) and
	// len(EndOffsets).
	Entries []TermVectorEntry

	// TermFreqsInDoc records the per-term frequency observed in the
	// surrounding document. Defaults to the per-entry Frequency when a
	// term is missing from the map.
	TermFreqsInDoc map[string]int
}

// TermVectorOffsetStrategy reads offsets straight from stored term
// vectors. Mirrors org.apache.lucene.search.uhighlight.TermVectorOffsetStrategy.
type TermVectorOffsetStrategy struct {
	BaseFieldOffsetStrategy
	literals []string
	matchers []CharArrayMatcher
}

// NewTermVectorOffsetStrategy returns the term-vector strategy for
// field. Optional [TermVectorSelector] values configure the matcher set
// the same way [AnalysisSelector] configures [AnalysisOffsetStrategy]
// (see [NewAnalysisOffsetStrategy] for the back-compat rationale).
func NewTermVectorOffsetStrategy(field string, sel ...TermVectorSelector) *TermVectorOffsetStrategy {
	s := &TermVectorOffsetStrategy{
		BaseFieldOffsetStrategy: NewBaseFieldOffsetStrategy(field),
	}
	for _, opt := range sel {
		opt(s)
	}
	return s
}

// TermVectorSelector configures the matcher set of a
// TermVectorOffsetStrategy.
type TermVectorSelector func(*TermVectorOffsetStrategy)

// WithTermVectorLiterals registers a list of literal-term matchers.
func WithTermVectorLiterals(literals ...string) TermVectorSelector {
	return func(s *TermVectorOffsetStrategy) {
		s.literals = append(s.literals, literals...)
	}
}

// WithTermVectorMatchers registers a list of CharArrayMatcher matchers.
func WithTermVectorMatchers(matchers ...CharArrayMatcher) TermVectorSelector {
	return func(s *TermVectorOffsetStrategy) {
		s.matchers = append(s.matchers, matchers...)
	}
}

// GetOffsetSource returns OffsetSourceTermVectors.
func (s *TermVectorOffsetStrategy) GetOffsetSource() OffsetSource { return OffsetSourceTermVectors }

// GetOffsetsEnum walks the term-vector entries and returns the matching
// offsets in document order.
func (s *TermVectorOffsetStrategy) GetOffsetsEnum(docContext any) (OffsetsEnum, error) {
	ctx, ok := docContext.(*TermVectorDocContext)
	if !ok || ctx == nil {
		return nil, fmt.Errorf("uhighlight: TermVectorOffsetStrategy expects *TermVectorDocContext, got %T", docContext)
	}
	if len(s.literals) == 0 && len(s.matchers) == 0 {
		return NewSliceOffsetsEnum(nil), nil
	}
	var entries []OffsetEntry
	for _, e := range ctx.Entries {
		if !s.termMatches(e.Term) {
			continue
		}
		if len(e.StartOffsets) == 0 || len(e.EndOffsets) == 0 {
			return nil, fmt.Errorf(
				"uhighlight: term-vector entry %q has no offsets; field must be indexed WITH_OFFSETS",
				e.Term)
		}
		if len(e.StartOffsets) != len(e.EndOffsets) {
			return nil, fmt.Errorf(
				"uhighlight: term-vector entry %q has mismatched offset arrays (starts=%d, ends=%d)",
				e.Term, len(e.StartOffsets), len(e.EndOffsets))
		}
		weight := lookupFreq(ctx.TermFreqsInDoc, e.Term, float32(e.Frequency))
		for i := range e.StartOffsets {
			entries = append(entries, OffsetEntry{
				Term:        e.Term,
				StartOffset: e.StartOffsets[i],
				EndOffset:   e.EndOffsets[i],
				Weight:      weight,
			})
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].StartOffset < entries[j].StartOffset
	})
	return NewSliceOffsetsEnum(entries), nil
}

// termMatches reports whether any literal or automaton matcher accepts
// the supplied term.
func (s *TermVectorOffsetStrategy) termMatches(term string) bool {
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

var _ FieldOffsetStrategy = (*TermVectorOffsetStrategy)(nil)
