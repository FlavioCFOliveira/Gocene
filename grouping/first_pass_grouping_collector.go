// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

// FirstPassGroupingCollector keeps the top-N groups by sort key as documents
// stream through Collect. Mirrors
// org.apache.lucene.search.grouping.FirstPassGroupingCollector.
//
// The Go port is generic over the group key type T (comparable) so callers
// can group by string, int64, *DoubleRange, etc. The compare callback decides
// whether candidate sort values rank higher than the current weakest group.
type FirstPassGroupingCollector[T comparable] struct {
	topN     int
	compare  func(a, b []any) int
	groups   map[T]*CollectedSearchGroup[T]
	order    []*CollectedSearchGroup[T]
}

// NewFirstPassGroupingCollector builds the collector with a top-N budget and
// the comparator over per-group sort values.
func NewFirstPassGroupingCollector[T comparable](topN int, compare func(a, b []any) int) *FirstPassGroupingCollector[T] {
	if topN < 1 {
		topN = 1
	}
	return &FirstPassGroupingCollector[T]{
		topN:    topN,
		compare: compare,
		groups:  make(map[T]*CollectedSearchGroup[T]),
	}
}

// Collect records a (group, sortValues, docID) tuple. When the group is new
// and the queue is full the highest-ranked existing group is evicted only
// when the incoming entry compares lower; otherwise the incoming entry is
// dropped.
func (c *FirstPassGroupingCollector[T]) Collect(group T, sortValues []any, docID int) {
	if existing, ok := c.groups[group]; ok {
		if c.compare(sortValues, existing.SortValues) < 0 {
			existing.SortValues = append(existing.SortValues[:0], sortValues...)
			existing.TopDoc = docID
		}
		return
	}
	if len(c.order) < c.topN {
		entry := NewCollectedSearchGroup(group, sortValues, docID, len(c.order))
		c.groups[group] = entry
		c.order = append(c.order, entry)
		return
	}
	worstIdx := c.findWorst()
	worst := c.order[worstIdx]
	if c.compare(sortValues, worst.SortValues) >= 0 {
		return
	}
	delete(c.groups, worst.GroupValue)
	entry := NewCollectedSearchGroup(group, sortValues, docID, worst.ComparatorSlot)
	c.order[worstIdx] = entry
	c.groups[group] = entry
}

func (c *FirstPassGroupingCollector[T]) findWorst() int {
	worst := 0
	for i := 1; i < len(c.order); i++ {
		if c.compare(c.order[i].SortValues, c.order[worst].SortValues) > 0 {
			worst = i
		}
	}
	return worst
}

// GetTopGroups returns the collected groups in collection order.
func (c *FirstPassGroupingCollector[T]) GetTopGroups() []*CollectedSearchGroup[T] {
	out := make([]*CollectedSearchGroup[T], len(c.order))
	copy(out, c.order)
	return out
}
