// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

import "github.com/FlavioCFOliveira/Gocene/search"

// CollectingMatcher is a CandidateMatcher that uses a Collector to gather results.
//
// Port of org.apache.lucene.monitor.CollectingMatcher (package-private in Java).
//
// Deviation: CollectingMatcher in Java is abstract and backed by a Lucene
// SimpleCollector that calls doMatch.  In Gocene, with no live IndexSearcher,
// it is a concrete stub that stores matches added via AddMatchDirect.
// Full scorer integration is deferred to backlog #2693.
type CollectingMatcher[T any] struct {
	BaseCandidateMatcher[T]
	resolve func(T, T) T
}

// NewCollectingMatcher creates a CollectingMatcher backed by the given searcher.
func NewCollectingMatcher[T any](
	searcher *search.IndexSearcher,
	resolve func(T, T) T,
) *CollectingMatcher[T] {
	return &CollectingMatcher[T]{
		BaseCandidateMatcher: NewBaseCandidateMatcher[T](searcher),
		resolve:              resolve,
	}
}

// MatchQuery is a no-op stub.  Subclasses (or callers) should populate matches via
// AddMatchDirect.
func (c *CollectingMatcher[T]) MatchQuery(_ string, _ search.Query, _ map[string]string) error {
	return nil
}

// Resolve combines two matches for the same query.
func (c *CollectingMatcher[T]) Resolve(match1, match2 T) T {
	return c.resolve(match1, match2)
}

// ReportError delegates to BaseCandidateMatcher.
func (c *CollectingMatcher[T]) ReportError(queryID string, err error) {
	c.BaseCandidateMatcher.ReportError(queryID, err)
}

// Finish delegates to BaseCandidateMatcher.
func (c *CollectingMatcher[T]) Finish(buildTime int64, queryCount int) *MultiMatchingQueries[T] {
	return c.BaseCandidateMatcher.Finish(buildTime, queryCount)
}

// AddMatchDirect records a match for the given document using the resolver.
func (c *CollectingMatcher[T]) AddMatchDirect(match T, doc int) {
	c.BaseCandidateMatcher.AddMatch(match, doc, c.resolve)
}
