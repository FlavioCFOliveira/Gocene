// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

// MultiMatchingQueries holds the results of matching a batch of documents against
// all queries held in the Monitor.
//
// Port of org.apache.lucene.monitor.MultiMatchingQueries.
type MultiMatchingQueries[T any] struct {
	matches        []map[string]T // one map per document
	errors         map[string]error
	queryBuildTime int64
	searchTime     int64
	queriesRun     int
	batchSize      int
}

// newMultiMatchingQueries builds a MultiMatchingQueries (package-private constructor).
func newMultiMatchingQueries[T any](
	matches []map[string]T,
	errors map[string]error,
	queryBuildTime, searchTime int64,
	queriesRun, batchSize int,
) *MultiMatchingQueries[T] {
	return &MultiMatchingQueries[T]{
		matches:        matches,
		errors:         errors,
		queryBuildTime: queryBuildTime,
		searchTime:     searchTime,
		queriesRun:     queriesRun,
		batchSize:      batchSize,
	}
}

// Matches returns the match for the given query and document IDs, or the zero value.
func (mq *MultiMatchingQueries[T]) Matches(queryID string, docID int) (T, bool) {
	if docID < 0 || docID >= len(mq.matches) {
		var zero T
		return zero, false
	}
	v, ok := mq.matches[docID][queryID]
	return v, ok
}

// GetMatches returns all matches for a given document.
func (mq *MultiMatchingQueries[T]) GetMatches(docID int) []T {
	if docID < 0 || docID >= len(mq.matches) {
		return nil
	}
	out := make([]T, 0, len(mq.matches[docID]))
	for _, v := range mq.matches[docID] {
		out = append(out, v)
	}
	return out
}

// GetMatchCount returns the number of queries that matched for the given document.
func (mq *MultiMatchingQueries[T]) GetMatchCount(docID int) int {
	if docID < 0 || docID >= len(mq.matches) {
		return 0
	}
	return len(mq.matches[docID])
}

// GetQueryBuildTime returns how long (ns) it took to build the presearcher query.
func (mq *MultiMatchingQueries[T]) GetQueryBuildTime() int64 { return mq.queryBuildTime }

// GetSearchTime returns how long (ms) it took to run the selected queries.
func (mq *MultiMatchingQueries[T]) GetSearchTime() int64 { return mq.searchTime }

// GetQueriesRun returns the number of queries passed to the CandidateMatcher.
func (mq *MultiMatchingQueries[T]) GetQueriesRun() int { return mq.queriesRun }

// GetBatchSize returns the number of documents in the batch.
func (mq *MultiMatchingQueries[T]) GetBatchSize() int { return mq.batchSize }

// GetErrors returns errors that occurred during matching, keyed by query ID.
func (mq *MultiMatchingQueries[T]) GetErrors() map[string]error { return mq.errors }

// singleton extracts a MatchingQueries for a batch of exactly one document.
func (mq *MultiMatchingQueries[T]) singleton() *MatchingQueries[T] {
	if len(mq.matches) != 1 {
		panic("singleton called on a MultiMatchingQueries with != 1 document")
	}
	return newMatchingQueries(mq.matches[0], mq.errors, mq.queryBuildTime, mq.searchTime, mq.queriesRun)
}
