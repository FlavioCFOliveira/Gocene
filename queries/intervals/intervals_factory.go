// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/intervals/Intervals.java

package intervals

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// Intervals provides factory functions for creating IntervalsSource instances.
// These sources implement minimum-interval algorithms from the paper
// "Efficient Optimally Lazy Algorithms for Minimal-Interval Semantics".
//
// Mirrors org.apache.lucene.queries.intervals.Intervals.
//
// Deviations from Java:
//   - Automaton-based factory methods (Prefix, Wildcard, Range, FuzzyTerm) that depend
//     on PrefixQuery.toAutomaton, WildcardQuery.toAutomaton, TermRangeQuery.toAutomaton are
//     not present; the caller should construct a CompiledAutomaton and call Multiterm directly.
//   - AnalyzedText uses *analysis.CachingTokenFilter (no Analyzer overload).

// Term returns an IntervalsSource exposing intervals for a term.
func Term(term string) IntervalsSource {
	return NewTermIntervalsSource([]byte(term))
}

// TermBytes returns an IntervalsSource exposing intervals for a term given as bytes.
func TermBytes(term []byte) IntervalsSource {
	return NewTermIntervalsSource(term)
}

// TermWithPayloadFilter returns an IntervalsSource for a term filtered by payload predicate.
func TermWithPayloadFilter(term string, filter func([]byte) bool) IntervalsSource {
	return NewPayloadFilteredTermIntervalsSource([]byte(term), filter)
}

// Phrase returns an IntervalsSource exposing intervals for a phrase of terms.
func Phrase(terms ...string) IntervalsSource {
	if len(terms) == 1 {
		return Term(terms[0])
	}
	sources := make([]IntervalsSource, len(terms))
	for i, t := range terms {
		sources[i] = Term(t)
	}
	return BuildBlockIntervalsSource(sources)
}

// PhraseOf returns an IntervalsSource for a phrase of IntervalsSource objects.
func PhraseOf(subSources ...IntervalsSource) IntervalsSource {
	return BuildBlockIntervalsSource(subSources)
}

// Or returns an IntervalsSource over the disjunction of sub-sources, with automatic rewriting.
func Or(subSources ...IntervalsSource) IntervalsSource {
	return OrList(true, subSources)
}

// OrNoRewrite returns an IntervalsSource over the disjunction without rewriting.
func OrNoRewrite(subSources ...IntervalsSource) IntervalsSource {
	return OrList(false, subSources)
}

// OrList returns an IntervalsSource over the disjunction of a list.
// If rewrite is false, the result is not rewritten for gap-sensitive sources.
func OrList(rewrite bool, subSources []IntervalsSource) IntervalsSource {
	return createDisjunction(subSources, rewrite)
}

// createDisjunction builds a DisjunctionIntervalsSource or returns the single source.
func createDisjunction(subSources []IntervalsSource, pullUp bool) IntervalsSource {
	simplified := deduplicateOr(subSources, pullUp)
	if len(simplified) == 1 {
		return simplified[0]
	}
	return NewDisjunctionIntervalsSource(simplified, pullUp)
}

// deduplicateOr simplifies a disjunction list by flattening nested disjunctions and
// removing duplicates, preserving insertion order.
func deduplicateOr(sources []IntervalsSource, _ bool) []IntervalsSource {
	seen := make(map[string]bool)
	var result []IntervalsSource
	var add func(s IntervalsSource)
	add = func(s IntervalsSource) {
		if d, ok := s.(*DisjunctionIntervalsSource); ok {
			for _, sub := range d.subSources {
				add(sub)
			}
		} else {
			key := s.String()
			if !seen[key] {
				seen[key] = true
				result = append(result, s)
			}
		}
	}
	for _, s := range sources {
		add(s)
	}
	return result
}

// Ordered returns an ordered IntervalsSource.
func Ordered(subSources ...IntervalsSource) IntervalsSource {
	return BuildOrderedIntervalsSource(subSources)
}

// Unordered returns an unordered IntervalsSource.
func Unordered(subSources ...IntervalsSource) IntervalsSource {
	return BuildUnorderedIntervalsSource(subSources)
}

// UnorderedNoOverlaps returns intervals where both sources appear without overlapping.
func UnorderedNoOverlaps(a, b IntervalsSource) IntervalsSource {
	return Or(Ordered(a, b), Ordered(b, a))
}

// FixField returns an IntervalsSource that always uses a specific field.
func FixField(field string, source IntervalsSource) IntervalsSource {
	return NewFixedFieldIntervalsSource(field, source)
}

// NonOverlapping returns intervals of the minuend that do not overlap with the subtrahend.
func NonOverlapping(minuend, subtrahend IntervalsSource) IntervalsSource {
	return NewNonOverlappingIntervalsSource(minuend, subtrahend)
}

// Overlapping returns intervals of the source that overlap with the reference.
func Overlapping(source, reference IntervalsSource) IntervalsSource {
	return NewOverlappingIntervalsSource(source, reference)
}

// NotWithin returns intervals of the minuend not within positions of the subtrahend.
func NotWithin(minuend IntervalsSource, positions int, subtrahend IntervalsSource) IntervalsSource {
	return NewNonOverlappingIntervalsSource(minuend, Extend(subtrahend, positions, positions))
}

// Within returns intervals of the source within positions of the reference.
func Within(source IntervalsSource, positions int, reference IntervalsSource) IntervalsSource {
	return ContainedBy(source, Extend(reference, positions, positions))
}

// NotContaining returns intervals from the minuend that do not contain intervals of the subtrahend.
func NotContaining(minuend, subtrahend IntervalsSource) IntervalsSource {
	pullUp := disjunctionsPullUpSingle(minuend, func(s IntervalsSource) IntervalsSource {
		return NewNotContainingIntervalsSource(s, subtrahend)
	})
	return OrList(true, pullUp)
}

// Containing returns intervals from big that contain intervals from small.
func Containing(big, small IntervalsSource) IntervalsSource {
	pullUp := disjunctionsPullUpSingle(big, func(s IntervalsSource) IntervalsSource {
		return NewContainingIntervalsSource(s, small)
	})
	return OrList(true, pullUp)
}

// NotContainedBy returns intervals from small that do not appear within big.
func NotContainedBy(small, big IntervalsSource) IntervalsSource {
	pullUp := disjunctionsPullUpSingle(big, func(s IntervalsSource) IntervalsSource {
		return NewNotContainedByIntervalsSource(small, s)
	})
	return OrList(true, pullUp)
}

// ContainedBy returns intervals from small that appear within big.
func ContainedBy(small, big IntervalsSource) IntervalsSource {
	pullUp := disjunctionsPullUpSingle(big, func(s IntervalsSource) IntervalsSource {
		return NewContainedByIntervalsSource(small, s)
	})
	return OrList(true, pullUp)
}

// AtLeast returns intervals spanning combinations from at least minShouldMatch sources.
func AtLeast(minShouldMatch int, sources ...IntervalsSource) IntervalsSource {
	if minShouldMatch == len(sources) {
		return Unordered(sources...)
	}
	if minShouldMatch > len(sources) {
		return NewNoMatchIntervalsSource(
			"Too few sources to match minimum of [" + intToStr(minShouldMatch) + "]: " + formatSources(sources))
	}
	return NewMinimumShouldMatchIntervalsSource(sources, minShouldMatch)
}

// Before returns intervals from source that appear before intervals from reference.
func Before(source, reference IntervalsSource) IntervalsSource {
	return ContainedBy(source, Extend(NewOffsetIntervalsSource(reference, true), math.MaxInt32, 0))
}

// After returns intervals from source that appear after intervals from reference.
func After(source, reference IntervalsSource) IntervalsSource {
	return ContainedBy(source, Extend(NewOffsetIntervalsSource(reference, false), 0, math.MaxInt32))
}

// Extend wraps a source, extending intervals by before and after positions.
func Extend(source IntervalsSource, before, after int) IntervalsSource {
	return NewExtendedIntervalsSource(source, before, after)
}

// MaxGaps filters a source keeping only intervals with at most maxGaps gaps.
func MaxGaps(gaps int, subSource IntervalsSource) IntervalsSource {
	return MaxGapsIntervalsSource(subSource, gaps)
}

// MaxWidth filters a source keeping only intervals with at most maxWidth positions.
func MaxWidth(width int, subSource IntervalsSource) IntervalsSource {
	return MaxWidthIntervalsSource(subSource, width)
}

// NoIntervals returns an IntervalsSource that matches no intervals.
func NoIntervals(reason string) IntervalsSource {
	return NewNoMatchIntervalsSource(reason)
}

// Multiterm returns an IntervalsSource over the disjunction of all terms accepted by the automaton.
func Multiterm(ca *automaton.CompiledAutomaton, pattern string) IntervalsSource {
	return MultitermWithMaxExpansions(ca, DefaultMaxExpansions, pattern)
}

// MultitermWithMaxExpansions returns a Multiterm source with a custom expansion limit.
func MultitermWithMaxExpansions(ca *automaton.CompiledAutomaton, maxExpansions int, pattern string) IntervalsSource {
	return NewMultiTermIntervalsSource(ca, maxExpansions, pattern)
}

// AnalyzedText returns intervals for analyzed text from a CachingTokenFilter.
func AnalyzedText(stream *analysis.CachingTokenFilter, maxGaps int, ordered bool) (IntervalsSource, error) {
	return AnalyzeText(stream, maxGaps, ordered)
}

// formatSources formats a slice of sources as "[s1, s2, ...]".
func formatSources(sources []IntervalsSource) string {
	s := "["
	for i, src := range sources {
		if i > 0 {
			s += ", "
		}
		s += src.String()
	}
	return s + "]"
}
