// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

import "strings"

// PresearcherMatch wraps a QueryMatch with information about which presearcher
// terms were selected.
//
// Port of org.apache.lucene.monitor.PresearcherMatch.
type PresearcherMatch[T any] struct {
	// PresearcherMatches contains the presearcher terms that triggered this match.
	PresearcherMatches string

	// QueryMatch is the underlying match.
	QueryMatch T

	// QueryID is the ID of the matching query.
	QueryID string
}

// newPresearcherMatch builds a PresearcherMatch (package-private constructor).
func newPresearcherMatch[T any](id, presearcherMatches string, queryMatch T) *PresearcherMatch[T] {
	return &PresearcherMatch[T]{
		PresearcherMatches: presearcherMatches,
		QueryMatch:         queryMatch,
		QueryID:            id,
	}
}

// PresearcherMatches wraps a MultiMatchingQueries with per-query presearcher
// term information.
//
// Port of org.apache.lucene.monitor.PresearcherMatches.
type PresearcherMatchesResult[T any] struct {
	matchingTerms map[string]*strings.Builder
	// Matcher is the wrapped MultiMatchingQueries.
	Matcher *MultiMatchingQueries[T]
}

// newPresearcherMatches builds a PresearcherMatchesResult (package-private constructor).
func newPresearcherMatches[T any](
	matchingTerms map[string]*strings.Builder,
	matcher *MultiMatchingQueries[T],
) *PresearcherMatchesResult[T] {
	return &PresearcherMatchesResult[T]{
		matchingTerms: matchingTerms,
		Matcher:       matcher,
	}
}

// Match returns match information for a given query and document, or nil if no match.
func (pm *PresearcherMatchesResult[T]) Match(queryID string, doc int) *PresearcherMatch[T] {
	sb, ok := pm.matchingTerms[queryID]
	if !ok {
		return nil
	}
	m, matched := pm.Matcher.Matches(queryID, doc)
	if !matched {
		return nil
	}
	return newPresearcherMatch(queryID, sb.String(), m)
}
