// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/intervals/MultiTermIntervalsSource.java

package intervals

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// DefaultMaxExpansions is the default maximum number of term expansions.
// Mirrors Intervals.DEFAULT_MAX_EXPANSIONS.
const DefaultMaxExpansions = 128

// MultiTermIntervalsSource is an IntervalsSource over the disjunction of all terms
// matching a compiled automaton.
//
// Mirrors org.apache.lucene.queries.intervals.MultiTermIntervalsSource.
//
// Deviations from Java:
//   - Uses Terms.GetIterator() + CompiledAutomaton.Run([]byte) for term matching
//     instead of automaton.getTermsEnum(terms), since Gocene's CompiledAutomaton
//     does not expose a filtered TermsEnum.
//   - automaton.Visit (QueryVisitor) not delegated; VisitLeaf is used instead.
type MultiTermIntervalsSource struct {
	compiled      *automaton.CompiledAutomaton
	maxExpansions int
	pattern       string
}

// NewMultiTermIntervalsSource creates a MultiTermIntervalsSource.
func NewMultiTermIntervalsSource(compiled *automaton.CompiledAutomaton, maxExpansions int, pattern string) *MultiTermIntervalsSource {
	return &MultiTermIntervalsSource{compiled: compiled, maxExpansions: maxExpansions, pattern: pattern}
}

// Intervals creates an IntervalIterator over the union of matching term iterators.
func (s *MultiTermIntervalsSource) Intervals(field string, ctx *index.LeafReaderContext) (IntervalIterator, error) {
	terms, err := ctx.LeafReader().Terms(field)
	if err != nil {
		return nil, err
	}
	if terms == nil {
		return nil, nil
	}
	var subIters []IntervalIterator
	te, err := terms.GetIterator()
	if err != nil {
		return nil, err
	}
	count := 0
	for {
		t, err := te.Next()
		if err != nil {
			return nil, err
		}
		if t == nil {
			break
		}
		if !s.compiled.Run([]byte(t.Text())) {
			continue
		}
		it, err := termIntervals([]byte(t.Text()), te)
		if err != nil {
			return nil, err
		}
		subIters = append(subIters, it)
		count++
		if count > s.maxExpansions {
			return nil, fmt.Errorf("automaton [%s] expanded to too many terms (limit %d)", s.pattern, s.maxExpansions)
		}
	}
	if len(subIters) == 0 {
		return nil, nil
	}
	if len(subIters) == 1 {
		return subIters[0], nil
	}
	return newDisjunctionIntervalIterator(subIters), nil
}

// Matches creates an IntervalMatchesIterator over the union of matching term match iterators.
func (s *MultiTermIntervalsSource) Matches(field string, ctx *index.LeafReaderContext, doc int) (IntervalMatchesIterator, error) {
	terms, err := ctx.LeafReader().Terms(field)
	if err != nil {
		return nil, err
	}
	if terms == nil {
		return nil, nil
	}
	te, err := terms.GetIterator()
	if err != nil {
		return nil, err
	}
	var subMatches []search.MatchesIterator
	count := 0
	for {
		t, err := te.Next()
		if err != nil {
			return nil, err
		}
		if t == nil {
			break
		}
		if !s.compiled.Run([]byte(t.Text())) {
			continue
		}
		mi, err := termMatches(te, doc, field)
		if err != nil {
			return nil, err
		}
		if mi != nil {
			subMatches = append(subMatches, mi)
			count++
			if count > s.maxExpansions {
				return nil, fmt.Errorf("automaton %s expanded to too many terms (limit %d)", t.Text(), s.maxExpansions)
			}
		}
	}
	mi := search.DisjunctionMatchesIterator(subMatches)
	if mi == nil {
		return nil, nil
	}
	return &multiTermMatchesIterator{inner: mi}, nil
}

// multiTermMatchesIterator wraps a MatchesIterator as an IntervalMatchesIterator.
type multiTermMatchesIterator struct {
	inner search.MatchesIterator
}

func (m *multiTermMatchesIterator) Gaps() int  { return 0 }
func (m *multiTermMatchesIterator) Width() int { return 1 }
func (m *multiTermMatchesIterator) Next() (bool, error) { return m.inner.Next() }
func (m *multiTermMatchesIterator) StartPosition() int  { return m.inner.StartPosition() }
func (m *multiTermMatchesIterator) EndPosition() int    { return m.inner.EndPosition() }
func (m *multiTermMatchesIterator) StartOffset() (int, error) { return m.inner.StartOffset() }
func (m *multiTermMatchesIterator) EndOffset() (int, error)   { return m.inner.EndOffset() }
func (m *multiTermMatchesIterator) GetSubMatches() (search.MatchesIterator, error) {
	return m.inner.GetSubMatches()
}
func (m *multiTermMatchesIterator) GetQuery() search.Query { return m.inner.GetQuery() }

// Visit visits the query using the visitor's leaf path.
func (s *MultiTermIntervalsSource) Visit(field string, visitor search.QueryVisitor) {
	visitor.VisitLeaf(NewIntervalQuery(field, s))
}

// MinExtent returns 1.
func (s *MultiTermIntervalsSource) MinExtent() int { return 1 }

// PullUpDisjunctions returns a singleton list.
func (s *MultiTermIntervalsSource) PullUpDisjunctions() []IntervalsSource {
	return []IntervalsSource{s}
}

// Equals reports structural equality.
func (s *MultiTermIntervalsSource) Equals(other IntervalsSource) bool {
	o, ok := other.(*MultiTermIntervalsSource)
	if !ok {
		return false
	}
	return s.maxExpansions == o.maxExpansions && s.pattern == o.pattern
}

// HashCode returns a hash code.
func (s *MultiTermIntervalsSource) HashCode() int {
	return hashString(s.pattern)*31 + s.maxExpansions
}

// String returns a human-readable representation.
func (s *MultiTermIntervalsSource) String() string {
	return "MultiTerm(" + s.pattern + ")"
}
