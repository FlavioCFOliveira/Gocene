// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package sortedset

import "sync"

// ConcurrentSortedSetDocValuesFacetCounts is the thread-safe variant of
// SortedSetDocValuesFacetCounts: it backs the per-ord counters with a mutex
// so multiple goroutines (or Lucene's IntraQueryExecutor) can increment in
// parallel. Mirrors
// org.apache.lucene.facet.sortedset.ConcurrentSortedSetDocValuesFacetCounts.
type ConcurrentSortedSetDocValuesFacetCounts struct {
	state  SortedSetDocValuesReaderState
	mu     sync.Mutex
	counts []int
	total  int
}

// NewConcurrentSortedSetDocValuesFacetCounts builds the counter for the
// supplied reader state.
func NewConcurrentSortedSetDocValuesFacetCounts(state SortedSetDocValuesReaderState) *ConcurrentSortedSetDocValuesFacetCounts {
	return &ConcurrentSortedSetDocValuesFacetCounts{
		state:  state,
		counts: make([]int, state.GetSize()),
	}
}

// IncrementOrd safely increases the count for ord.
func (c *ConcurrentSortedSetDocValuesFacetCounts) IncrementOrd(ord int) {
	if ord < 0 || ord >= len(c.counts) {
		return
	}
	c.mu.Lock()
	c.counts[ord]++
	c.total++
	c.mu.Unlock()
}

// CountForOrd returns the count recorded for ord.
func (c *ConcurrentSortedSetDocValuesFacetCounts) CountForOrd(ord int) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ord < 0 || ord >= len(c.counts) {
		return 0
	}
	return c.counts[ord]
}

// GetTotalCount returns the sum of all per-ord counts.
func (c *ConcurrentSortedSetDocValuesFacetCounts) GetTotalCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.total
}

// CountsForDim returns a copy of the counts in the dim's ordinal range.
func (c *ConcurrentSortedSetDocValuesFacetCounts) CountsForDim(dim string) []int {
	start, end := c.state.GetOrdRange(dim)
	if start < 0 {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]int, end-start)
	copy(out, c.counts[start:end])
	return out
}
