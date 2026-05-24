// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

// MatchingQueries holds the results of matching a single document against all
// queries held in the Monitor.
//
// Port of org.apache.lucene.monitor.MatchingQueries.
type MatchingQueries[T any] struct {
	matches        map[string]T
	errors         map[string]error
	queryBuildTime int64 // nanoseconds
	searchTime     int64 // milliseconds
	queriesRun     int
}

// newMatchingQueries builds a MatchingQueries (package-private constructor).
func newMatchingQueries[T any](
	matches map[string]T,
	errors map[string]error,
	queryBuildTime, searchTime int64,
	queriesRun int,
) *MatchingQueries[T] {
	return &MatchingQueries[T]{
		matches:        matches,
		errors:         errors,
		queryBuildTime: queryBuildTime,
		searchTime:     searchTime,
		queriesRun:     queriesRun,
	}
}

// Matches returns the match for the given query ID, or the zero value when absent.
func (mq *MatchingQueries[T]) Matches(queryID string) (T, bool) {
	v, ok := mq.matches[queryID]
	return v, ok
}

// GetMatches returns all matching values.
func (mq *MatchingQueries[T]) GetMatches() []T {
	out := make([]T, 0, len(mq.matches))
	for _, v := range mq.matches {
		out = append(out, v)
	}
	return out
}

// GetMatchCount returns the number of queries that matched.
func (mq *MatchingQueries[T]) GetMatchCount() int { return len(mq.matches) }

// GetQueryBuildTime returns how long (ns) it took to build the presearcher query.
func (mq *MatchingQueries[T]) GetQueryBuildTime() int64 { return mq.queryBuildTime }

// GetSearchTime returns how long (ms) it took to run the selected queries.
func (mq *MatchingQueries[T]) GetSearchTime() int64 { return mq.searchTime }

// GetQueriesRun returns the number of queries passed to the CandidateMatcher.
func (mq *MatchingQueries[T]) GetQueriesRun() int { return mq.queriesRun }

// GetErrors returns errors that occurred during matching, keyed by query ID.
func (mq *MatchingQueries[T]) GetErrors() map[string]error { return mq.errors }
