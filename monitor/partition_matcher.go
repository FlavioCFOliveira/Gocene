// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

import (
	"sync"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// PartitionMatcher collects all possible matches in one pass, then partitions
// them across worker goroutines.
//
// Port of org.apache.lucene.monitor.PartitionMatcher.
//
// Deviation: Java uses an ExecutorService.  Gocene uses goroutines.
// Full parallel dispatch is deferred to backlog #2693.
type PartitionMatcher[T any] struct {
	BaseCandidateMatcher[T]
	factory MatcherFactory[T]
	pending []*pendingQuery[T]
	mu      sync.Mutex
}

type pendingQuery[T any] struct {
	queryID  string
	query    search.Query
	metadata map[string]string
}

// NewPartitionMatcher creates a PartitionMatcher.
func NewPartitionMatcher[T any](
	searcher *search.IndexSearcher,
	factory MatcherFactory[T],
) *PartitionMatcher[T] {
	return &PartitionMatcher[T]{
		BaseCandidateMatcher: NewBaseCandidateMatcher[T](searcher),
		factory:              factory,
	}
}

// MatchQuery accumulates the query for deferred execution.
func (p *PartitionMatcher[T]) MatchQuery(queryID string, matchQuery search.Query, metadata map[string]string) error {
	p.mu.Lock()
	p.pending = append(p.pending, &pendingQuery[T]{queryID, matchQuery, metadata})
	p.mu.Unlock()
	return nil
}

// Resolve delegates to a fresh delegate from the factory.
func (p *PartitionMatcher[T]) Resolve(match1, match2 T) T {
	delegate := p.factory.CreateMatcher(p.Searcher)
	return delegate.Resolve(match1, match2)
}

// ReportError records an error.
func (p *PartitionMatcher[T]) ReportError(queryID string, err error) {
	p.BaseCandidateMatcher.ReportError(queryID, err)
}

// Finish runs all pending queries (single-threaded stub) and returns results.
func (p *PartitionMatcher[T]) Finish(buildTime int64, queryCount int) *MultiMatchingQueries[T] {
	p.mu.Lock()
	pending := p.pending
	p.pending = nil
	p.mu.Unlock()

	for _, pq := range pending {
		delegate := p.factory.CreateMatcher(p.Searcher)
		if err := delegate.MatchQuery(pq.queryID, pq.query, pq.metadata); err != nil {
			p.ReportError(pq.queryID, err)
		}
	}
	return p.BaseCandidateMatcher.Finish(buildTime, queryCount)
}
