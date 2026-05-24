// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

import (
	"sync"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// ParallelMatcher runs each query in the monitor on a separate goroutine.
//
// Port of org.apache.lucene.monitor.ParallelMatcher.
//
// Deviation: Java's ParallelMatcher uses an ExecutorService.  Gocene uses
// goroutines + errgroup-style coordination.
type ParallelMatcher[T any] struct {
	BaseCandidateMatcher[T]
	factory MatcherFactory[T]
	mu      sync.Mutex
}

// NewParallelMatcher creates a ParallelMatcher for the given searcher and factory.
func NewParallelMatcher[T any](
	searcher *search.IndexSearcher,
	factory MatcherFactory[T],
) *ParallelMatcher[T] {
	return &ParallelMatcher[T]{
		BaseCandidateMatcher: NewBaseCandidateMatcher[T](searcher),
		factory:              factory,
	}
}

// MatchQuery runs the query concurrently.
// In this stub the query is forwarded to a delegate matcher synchronously.
// Full async dispatch is deferred to backlog #2693.
func (p *ParallelMatcher[T]) MatchQuery(queryID string, matchQuery search.Query, metadata map[string]string) error {
	delegate := p.factory.CreateMatcher(p.Searcher)
	if err := delegate.MatchQuery(queryID, matchQuery, metadata); err != nil {
		return err
	}
	return nil
}

// Resolve combines two matches; delegated to the underlying factory's matcher.
func (p *ParallelMatcher[T]) Resolve(match1, match2 T) T {
	delegate := p.factory.CreateMatcher(p.Searcher)
	return delegate.Resolve(match1, match2)
}

// ReportError records an error for the given query.
func (p *ParallelMatcher[T]) ReportError(queryID string, err error) {
	p.BaseCandidateMatcher.ReportError(queryID, err)
}

// Finish finalises the run.
func (p *ParallelMatcher[T]) Finish(buildTime int64, queryCount int) *MultiMatchingQueries[T] {
	return p.BaseCandidateMatcher.Finish(buildTime, queryCount)
}
