// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// SearchGroup represents a group that is found during the first pass search.
//
// Mirrors org.apache.lucene.search.grouping.SearchGroup<T>.
type SearchGroup[T any] struct {
	// GroupValue is the value that defines this group.
	GroupValue T

	// SortValues holds the sort values used during sorting. These are the
	// groupSort field values of the highest rank document (by the groupSort)
	// within the group. Can be nil if fillFields=false had been passed to
	// FirstPassGroupingCollector.GetTopGroups.
	SortValues []any
}

// String mirrors SearchGroup.toString().
func (g *SearchGroup[T]) String() string {
	return "SearchGroup(groupValue=" + fmt.Sprint(g.GroupValue) +
		" sortValues=" + javaArraysToString(g.SortValues) + ")"
}

// Equals mirrors SearchGroup.equals(Object): two groups are equal when their
// group values are.
func (g *SearchGroup[T]) Equals(o *SearchGroup[T]) bool {
	if g == o {
		return true
	}
	if o == nil {
		return false
	}
	return groupKey(any(g.GroupValue)) == groupKey(any(o.GroupValue))
}

// HashCode mirrors SearchGroup.hashCode().
func (g *SearchGroup[T]) HashCode() int {
	return javaHashCode(any(g.GroupValue))
}

// searchGroupShardIter mirrors the private static class
// SearchGroup.ShardIter<T>.
type searchGroupShardIter[T any] struct {
	groups     []*SearchGroup[T]
	pos        int
	shardIndex int
}

// newSearchGroupShardIter mirrors ShardIter(Collection, int).
func newSearchGroupShardIter[T any](shard []*SearchGroup[T], shardIndex int) *searchGroupShardIter[T] {
	return &searchGroupShardIter[T]{groups: shard, shardIndex: shardIndex}
}

// hasNext mirrors iter.hasNext().
func (s *searchGroupShardIter[T]) hasNext() bool { return s.pos < len(s.groups) }

// next mirrors ShardIter.next(). It fails the same way Java does when the
// first pass was run with fillFields=false.
func (s *searchGroupShardIter[T]) next() *SearchGroup[T] {
	group := s.groups[s.pos]
	s.pos++
	if group.SortValues == nil {
		panic("group.sortValues is null; you must pass fillFields=true to the first pass collector")
	}
	return group
}

// String mirrors ShardIter.toString().
func (s *searchGroupShardIter[T]) String() string {
	return fmt.Sprintf("ShardIter(shard=%d)", s.shardIndex)
}

// searchGroupMergedGroup mirrors the private static class
// SearchGroup.MergedGroup<T>: it holds all shards currently on the same group.
type searchGroupMergedGroup[T any] struct {
	// groupValue may be null!
	groupValue T

	topValues     []any
	shards        []*searchGroupShardIter[T]
	minShardIndex int
	processed     bool
	inQueue       bool
}

// newSearchGroupMergedGroup mirrors MergedGroup(T).
func newSearchGroupMergedGroup[T any](groupValue T) *searchGroupMergedGroup[T] {
	return &searchGroupMergedGroup[T]{groupValue: groupValue}
}

// equals mirrors MergedGroup.equals(Object).
func (m *searchGroupMergedGroup[T]) equals(other *searchGroupMergedGroup[T]) bool {
	if other == nil {
		return false
	}
	return groupKey(any(m.groupValue)) == groupKey(any(other.groupValue))
}

// hashCode mirrors MergedGroup.hashCode().
func (m *searchGroupMergedGroup[T]) hashCode() int {
	return javaHashCode(any(m.groupValue))
}

// searchGroupComparator mirrors the private static class
// SearchGroup.GroupComparator<T>.
type searchGroupComparator[T any] struct {
	comparators []search.FieldComparator
	reversed    []int
}

// newSearchGroupComparator mirrors GroupComparator(Sort).
func newSearchGroupComparator[T any](groupSort *search.Sort) *searchGroupComparator[T] {
	sortFields := groupSort.GetSort()
	c := &searchGroupComparator[T]{
		comparators: make([]search.FieldComparator, len(sortFields)),
		reversed:    make([]int, len(sortFields)),
	}
	for compIDX := 0; compIDX < len(sortFields); compIDX++ {
		sortField := sortFields[compIDX]
		c.comparators[compIDX] = search.SortFieldGetComparator(sortField, 1, search.PruningNone)
		if sortField.GetReverse() {
			c.reversed[compIDX] = -1
		} else {
			c.reversed[compIDX] = 1
		}
	}
	return c
}

// compare mirrors GroupComparator.compare(MergedGroup, MergedGroup).
func (c *searchGroupComparator[T]) compare(group, other *searchGroupMergedGroup[T]) int {
	if group == other {
		return 0
	}
	groupValues := group.topValues
	otherValues := other.topValues
	for compIDX := 0; compIDX < len(c.comparators); compIDX++ {
		cmp := c.reversed[compIDX] * c.comparators[compIDX].CompareValues(groupValues[compIDX], otherValues[compIDX])
		if cmp != 0 {
			return cmp
		}
	}

	// Tie break by min shard index:
	return group.minShardIndex - other.minShardIndex
}

// searchGroupMerger mirrors the private static class
// SearchGroup.GroupMerger<T>.
type searchGroupMerger[T any] struct {
	groupComp  *searchGroupComparator[T]
	queue      *treeSet[*searchGroupMergedGroup[T]]
	groupsSeen *groupMap[T, *searchGroupMergedGroup[T]]
}

// newSearchGroupMerger mirrors GroupMerger(Sort).
func newSearchGroupMerger[T any](groupSort *search.Sort) *searchGroupMerger[T] {
	groupComp := newSearchGroupComparator[T](groupSort)
	return &searchGroupMerger[T]{
		groupComp:  groupComp,
		queue:      newTreeSet(groupComp.compare),
		groupsSeen: newGroupMap[T, *searchGroupMergedGroup[T]](),
	}
}

// updateNextGroup mirrors GroupMerger.updateNextGroup(int, ShardIter).
func (g *searchGroupMerger[T]) updateNextGroup(topN int, shard *searchGroupShardIter[T]) {
	for shard.hasNext() {
		group := shard.next()
		mergedGroup, found := g.groupsSeen.get(group.GroupValue)
		isNew := !found

		if isNew {
			// Start a new group:
			mergedGroup = newSearchGroupMergedGroup(group.GroupValue)
			mergedGroup.minShardIndex = shard.shardIndex
			mergedGroup.topValues = group.SortValues
			g.groupsSeen.put(group.GroupValue, mergedGroup)
			mergedGroup.inQueue = true
			g.queue.add(mergedGroup)
		} else if mergedGroup.processed {
			// This shard produced a group that we already
			// processed; move on to next group...
			continue
		} else {
			competes := false
			for compIDX := 0; compIDX < len(g.groupComp.comparators); compIDX++ {
				cmp := g.groupComp.reversed[compIDX] *
					g.groupComp.comparators[compIDX].CompareValues(
						group.SortValues[compIDX], mergedGroup.topValues[compIDX])
				if cmp < 0 {
					// Definitely competes
					competes = true
					break
				} else if cmp > 0 {
					// Definitely does not compete
					break
				} else if compIDX == len(g.groupComp.comparators)-1 {
					if shard.shardIndex < mergedGroup.minShardIndex {
						competes = true
					}
				}
			}

			if competes {
				// Group's sort changed -- remove & re-insert, update first group in place for
				// efficiency
				skipHeavyOps := g.queue.first() == mergedGroup
				if mergedGroup.inQueue && !skipHeavyOps {
					g.queue.remove(mergedGroup)
				}
				mergedGroup.topValues = group.SortValues
				mergedGroup.minShardIndex = shard.shardIndex
				if !skipHeavyOps {
					g.queue.add(mergedGroup)
				}
				mergedGroup.inQueue = true
			}
		}

		mergedGroup.shards = append(mergedGroup.shards, shard)
		break
	}

	// Prune un-competitive groups:
	for g.queue.size() > topN {
		group, _ := g.queue.pollLast()
		group.inQueue = false
	}
}

// merge mirrors GroupMerger.merge(List, int, int).
func (g *searchGroupMerger[T]) merge(shards [][]*SearchGroup[T], offset, topN int) []*SearchGroup[T] {
	maxQueueSize := offset + topN

	// Init queue:
	for shardIDX := 0; shardIDX < len(shards); shardIDX++ {
		shard := shards[shardIDX]
		if len(shard) != 0 {
			g.updateNextGroup(maxQueueSize, newSearchGroupShardIter(shard, shardIDX))
		}
	}

	// Pull merged topN groups:
	newTopGroups := make([]*SearchGroup[T], 0, topN)

	count := 0

	for !g.queue.isEmpty() {
		group, _ := g.queue.pollFirst()
		group.processed = true
		if count >= offset {
			newGroup := &SearchGroup[T]{}
			newGroup.GroupValue = group.groupValue
			newGroup.SortValues = group.topValues
			newTopGroups = append(newTopGroups, newGroup)
			if len(newTopGroups) == topN {
				break
			}
		}
		count++

		// Advance all iters in this group:
		for _, shardIter := range group.shards {
			g.updateNextGroup(maxQueueSize, shardIter)
		}
	}

	return newTopGroups
}

// MergeSearchGroups merges multiple collections of top groups, for example
// obtained from separate index shards. The provided groupSort must match how
// the groups were sorted, and the provided SearchGroups must have been
// computed with fillFields=true passed to
// FirstPassGroupingCollector.GetTopGroups.
//
// Mirrors the static method
// org.apache.lucene.search.grouping.SearchGroup.merge.
func MergeSearchGroups[T any](topGroups [][]*SearchGroup[T], offset, topN int, groupSort *search.Sort) []*SearchGroup[T] {
	return newSearchGroupMerger[T](groupSort).merge(topGroups, offset, topN)
}
