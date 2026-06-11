// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

import (
	"sync"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// pendingParallelQuery holds a queued query for async execution.
type pendingParallelQuery[T any] struct {
	queryID  string
	query    search.Query
	metadata map[string]string
}

// ParallelMatcher runs each query in the monitor on a separate goroutine.
//
// Port of org.apache.lucene.monitor.ParallelMatcher. Each call to MatchQuery
// submits the query to an internal queue; Finish drains the queue by spawning
// one goroutine per queued query, waiting for all completions, and merging the
// results into the base matcher's result set.
type ParallelMatcher[T any] struct {
	BaseCandidateMatcher[T]
	factory MatcherFactory[T]
	mu      sync.Mutex
	pending []*pendingParallelQuery[T]
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

// MatchQuery enqueues the query for concurrent execution in Finish.
//
// Port of ParallelMatcher.matchQuery: the Java implementation submits an async
// task to an ExecutorService. Gocene enqueues here and drains in Finish.
func (p *ParallelMatcher[T]) MatchQuery(queryID string, matchQuery search.Query, metadata map[string]string) error {
	p.mu.Lock()
	p.pending = append(p.pending, &pendingParallelQuery[T]{queryID, matchQuery, metadata})
	p.mu.Unlock()
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

// Finish drains the pending queue by running each query concurrently and
// returns the merged results. Mirrors ParallelMatcher.finish in Java.
func (p *ParallelMatcher[T]) Finish(buildTime int64, queryCount int) *MultiMatchingQueries[T] {
	p.mu.Lock()
	pending := p.pending
	p.pending = nil
	p.mu.Unlock()

	if len(pending) == 0 {
		return p.BaseCandidateMatcher.Finish(buildTime, queryCount)
	}

	// Spawn one goroutine per pending query; each creates its own delegate so
	// there is no shared mutable state between goroutines.
	type result struct {
		queryID  string
		err      error
		delegate *BaseCandidateMatcher[T]
	}
	results := make(chan result, len(pending))

	for _, pq := range pending {
		pq := pq
		go func() {
			delegate := p.factory.CreateMatcher(p.Searcher)
			err := delegate.MatchQuery(pq.queryID, pq.query, pq.metadata)
			// Extract delegate's BaseCandidateMatcher to merge results.
			var base *BaseCandidateMatcher[T]
			if bm, ok := interface{}(delegate).(*BaseCandidateMatcher[T]); ok {
				base = bm
			}
			results <- result{pq.queryID, err, base}
		}()
	}

	// Collect and merge all results back into the parent matcher.
	for range pending {
		r := <-results
		if r.err != nil {
			p.ReportError(r.queryID, r.err)
		}
		if r.delegate != nil {
			p.BaseCandidateMatcher.MergeFrom(r.delegate, p.Resolve)
		}
	}
	close(results)

	return p.BaseCandidateMatcher.Finish(buildTime, queryCount)
}
