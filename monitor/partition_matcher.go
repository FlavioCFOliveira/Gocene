// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

import (
	"runtime"
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

// Finish partitions pending queries across goroutines and returns merged results.
//
// Port of PartitionMatcher.finish: Java splits the pending list into N equal
// partitions (where N == number of available CPU threads) and runs each
// partition in a separate executor task. Gocene spawns one goroutine per
// runtime.NumCPU() partition, each executing its slice of pending queries
// through a per-partition delegate. Results are merged after all goroutines
// complete.
func (p *PartitionMatcher[T]) Finish(buildTime int64, queryCount int) *MultiMatchingQueries[T] {
	p.mu.Lock()
	pending := p.pending
	p.pending = nil
	p.mu.Unlock()

	if len(pending) == 0 {
		return p.BaseCandidateMatcher.Finish(buildTime, queryCount)
	}

	// Determine partition count: cap at the number of pending queries.
	nPartitions := runtime.NumCPU()
	if nPartitions > len(pending) {
		nPartitions = len(pending)
	}

	type partResult struct {
		queryID string
		err     error
	}
	results := make(chan partResult, len(pending))

	var wg sync.WaitGroup
	partSize := (len(pending) + nPartitions - 1) / nPartitions
	for i := 0; i < len(pending); i += partSize {
		end := i + partSize
		if end > len(pending) {
			end = len(pending)
		}
		slice := pending[i:end]
		wg.Add(1)
		go func(queries []*pendingQuery[T]) {
			defer wg.Done()
			delegate := p.factory.CreateMatcher(p.Searcher)
			for _, pq := range queries {
				if err := delegate.MatchQuery(pq.queryID, pq.query, pq.metadata); err != nil {
					results <- partResult{pq.queryID, err}
				}
			}
		}(slice)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for r := range results {
		p.ReportError(r.queryID, r.err)
	}

	return p.BaseCandidateMatcher.Finish(buildTime, queryCount)
}
