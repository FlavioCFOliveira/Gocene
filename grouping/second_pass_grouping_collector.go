// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

// SecondPassGroupingCollector consumes the SearchGroups elected by the
// first-pass collector and collects every per-group hit (typically up to a
// per-group docs-per-group budget). Mirrors
// org.apache.lucene.search.grouping.SecondPassGroupingCollector.
type SecondPassGroupingCollector[T comparable] struct {
	target       map[T]bool
	docsPerGroup int
	groups       map[T][]int
}

// NewSecondPassGroupingCollector builds the collector for the set of group
// keys returned by the first pass, with a per-group document budget.
func NewSecondPassGroupingCollector[T comparable](targetGroups []T, docsPerGroup int) *SecondPassGroupingCollector[T] {
	if docsPerGroup < 1 {
		docsPerGroup = 1
	}
	target := make(map[T]bool, len(targetGroups))
	for _, g := range targetGroups {
		target[g] = true
	}
	return &SecondPassGroupingCollector[T]{
		target:       target,
		docsPerGroup: docsPerGroup,
		groups:       make(map[T][]int, len(targetGroups)),
	}
}

// Collect records docID under the supplied group, ignoring groups outside
// the target set and bounded by the per-group document budget.
func (c *SecondPassGroupingCollector[T]) Collect(group T, docID int) {
	if !c.target[group] {
		return
	}
	cur := c.groups[group]
	if len(cur) >= c.docsPerGroup {
		return
	}
	c.groups[group] = append(cur, docID)
}

// GetDocs returns the collected docIDs for group, in collection order.
func (c *SecondPassGroupingCollector[T]) GetDocs(group T) []int {
	src := c.groups[group]
	out := make([]int, len(src))
	copy(out, src)
	return out
}

// GroupCount returns the number of groups holding at least one document.
func (c *SecondPassGroupingCollector[T]) GroupCount() int { return len(c.groups) }
