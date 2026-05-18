// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

// GroupFacetCollector is the abstract base shared by the per-type group-facet
// counters. Mirrors org.apache.lucene.search.grouping.GroupFacetCollector.
//
// The contract: each implementation produces a list of (facet value, group
// count) entries, where the count reflects how many distinct groups produced
// at least one hit for the value.
type GroupFacetCollector interface {
	// Collect observes a (group, facet) tuple. The (group, facet) uniqueness
	// is captured by the implementation.
	Collect(group, facet string)

	// GetCounts returns the facet → distinct-group-count entries in
	// insertion order (or any order the caller can sort later).
	GetCounts() []GroupFacetEntry

	// TotalCount returns the number of distinct (group, facet) tuples seen.
	TotalCount() int
}

// GroupFacetEntry is a single (facet, distinct-group-count) pair returned by
// GetCounts.
type GroupFacetEntry struct {
	Facet string
	Count int
}

// MapGroupFacetCollector is the default implementation backed by a
// (facet, group) set used to count distinct groups per facet.
type MapGroupFacetCollector struct {
	counts map[string]map[string]struct{}
	order  []string
	total  int
}

// NewMapGroupFacetCollector builds the default collector.
func NewMapGroupFacetCollector() *MapGroupFacetCollector {
	return &MapGroupFacetCollector{counts: make(map[string]map[string]struct{})}
}

// Collect records a unique (group, facet) tuple.
func (c *MapGroupFacetCollector) Collect(group, facet string) {
	groups, ok := c.counts[facet]
	if !ok {
		groups = make(map[string]struct{})
		c.counts[facet] = groups
		c.order = append(c.order, facet)
	}
	if _, exists := groups[group]; exists {
		return
	}
	groups[group] = struct{}{}
	c.total++
}

// GetCounts returns the per-facet distinct group counts in insertion order.
func (c *MapGroupFacetCollector) GetCounts() []GroupFacetEntry {
	out := make([]GroupFacetEntry, 0, len(c.order))
	for _, f := range c.order {
		out = append(out, GroupFacetEntry{Facet: f, Count: len(c.counts[f])})
	}
	return out
}

// TotalCount returns the number of distinct (group, facet) pairs collected.
func (c *MapGroupFacetCollector) TotalCount() int { return c.total }

var _ GroupFacetCollector = (*MapGroupFacetCollector)(nil)
