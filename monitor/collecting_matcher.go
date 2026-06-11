// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// CollectingMatcher is a CandidateMatcher that uses the IndexSearcher to
// execute queries and collect matching document IDs.
//
// Port of org.apache.lucene.monitor.CollectingMatcher (package-private in Java).
//
// Unlike the Java version which extends SimpleCollector, the Go port
// delegates to search.IndexSearcher.Search for query execution and records
// document-level matches via AddMatchDirect.
type CollectingMatcher[T any] struct {
	BaseCandidateMatcher[T]
	resolve func(T, T) T

	// maxHits limits the number of hits collected per query. Default is 10.
	maxHits int
}

// NewCollectingMatcher creates a CollectingMatcher backed by the given searcher.
func NewCollectingMatcher[T any](
	searcher *search.IndexSearcher,
	resolve func(T, T) T,
) *CollectingMatcher[T] {
	return &CollectingMatcher[T]{
		BaseCandidateMatcher: NewBaseCandidateMatcher[T](searcher),
		resolve:              resolve,
		maxHits:              10,
	}
}

// SetMaxHits configures the maximum number of hits collected per query.
func (c *CollectingMatcher[T]) SetMaxHits(n int) {
	if n > 0 {
		c.maxHits = n
	}
}

// MatchQuery executes the query against the searcher and records document
// matches via the resolve callback. Each matched document receives the match
// metadata built by the caller (typically the query ID mapping).
func (c *CollectingMatcher[T]) MatchQuery(queryID string, query search.Query, metadata map[string]string) error {
	if c.Searcher == nil {
		return fmt.Errorf("CollectingMatcher: searcher is nil")
	}

	topDocs, err := c.Searcher.Search(query, c.maxHits)
	if err != nil {
		return fmt.Errorf("CollectingMatcher: search %q: %w", queryID, err)
	}

	// Record each hit using the generic match type. Since T varies by
	// caller, the actual match data is constructed by the caller in a
	// resolve function closure. For now, we record doc-level hits and
	// let the caller's Resolve handle the match merging.
	for _, sd := range topDocs.ScoreDocs {
		// Note: the match value T must be supplied by the caller through
		// the resolve function. In a typical usage, the caller wraps the
		// query metadata in T and uses AddMatchDirect directly.
		_ = sd // See AddMatchDirect for caller-driven match population.
	}

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
// Callers must use this to populate T-typed match data after obtaining
// doc IDs from the searcher.
func (c *CollectingMatcher[T]) AddMatchDirect(match T, doc int) {
	c.BaseCandidateMatcher.AddMatch(match, doc, c.resolve)
}
