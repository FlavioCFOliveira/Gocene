// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"fmt"
	"reflect"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// Ported from Apache Lucene 10.5.0:
//   lucene/grouping/src/java/org/apache/lucene/search/grouping/SearchGroup.java

// SearchGroup represents a group that is found during the first pass search.
//
// Mirrors org.apache.lucene.search.grouping.SearchGroup<T>.
type SearchGroup[T any] struct {
	// GroupValue is the value that defines this group.
	GroupValue T

	// SortValues are the sort values used during sorting: the groupSort field
	// values of the highest ranked document (by the groupSort) within the
	// group. They can be nil when fillFields=false was passed to
	// FirstPassGroupingCollector.GetTopGroups.
	SortValues []any
}

// String mirrors SearchGroup.toString().
func (sg SearchGroup[T]) String() string {
	return fmt.Sprintf("SearchGroup(groupValue=%v sortValues=%v)", sg.GroupValue, sg.SortValues)
}

// Equals reports whether sg and other define the same group. Equality is
// determined solely by the group value.
//
// Mirrors SearchGroup.equals(Object), whose body is
// Objects.equals(groupValue, other.groupValue).
func (sg SearchGroup[T]) Equals(other SearchGroup[T]) bool {
	return reflect.DeepEqual(sg.GroupValue, other.GroupValue)
}

// shardIter renders the private static nested class SearchGroup.ShardIter<T>.
type shardIter[T any] struct {
	groups     []SearchGroup[T]
	pos        int
	shardIndex int
}

func newShardIter[T any](shard []SearchGroup[T], shardIndex int) *shardIter[T] {
	return &shardIter[T]{groups: shard, shardIndex: shardIndex}
}

func (si *shardIter[T]) hasNext() bool { return si.pos < len(si.groups) }

// next mirrors ShardIter.next(), which raises IllegalArgumentException when a
// group carries no sort values.
func (si *shardIter[T]) next() SearchGroup[T] {
	group := si.groups[si.pos]
	si.pos++
	if group.SortValues == nil {
		panic("group.sortValues is null; you must pass fillFields=true to the first pass collector")
	}
	return group
}

func (si *shardIter[T]) String() string {
	return fmt.Sprintf("ShardIter(shard=%d)", si.shardIndex)
}

// mergedGroup renders the private static nested class
// SearchGroup.MergedGroup<T>: it holds all shards currently on the same group.
type mergedGroup[T any] struct {
	// groupValue may be the zero value of T, standing for Java's null.
	groupValue T

	topValues     []any
	shards        []*shardIter[T]
	minShardIndex int
	processed     bool
	inQueue       bool
}

// groupComparator renders the private static nested class
// SearchGroup.GroupComparator<T>.
type groupComparator[T any] struct {
	comparators []search.FieldComparator
	reversed    []int
}

func newGroupComparator[T any](groupSort *search.Sort) *groupComparator[T] {
	sortFields := groupSort.GetSort()
	gc := &groupComparator[T]{
		comparators: make([]search.FieldComparator, len(sortFields)),
		reversed:    make([]int, len(sortFields)),
	}
	for compIDX := range sortFields {
		sortField := sortFields[compIDX]
		gc.comparators[compIDX] = search.SortFieldGetComparator(sortField, 1, search.PruningNone)
		if sortField.GetReverse() {
			gc.reversed[compIDX] = -1
		} else {
			gc.reversed[compIDX] = 1
		}
	}
	return gc
}

// compare mirrors GroupComparator.compare(MergedGroup, MergedGroup), which
// tie-breaks on the minimum shard index.
func (gc *groupComparator[T]) compare(group, other *mergedGroup[T]) int {
	if group == other {
		return 0
	}
	groupValues := group.topValues
	otherValues := other.topValues
	for compIDX := range gc.comparators {
		c := gc.reversed[compIDX] * gc.comparators[compIDX].CompareValues(groupValues[compIDX], otherValues[compIDX])
		if c != 0 {
			return c
		}
	}
	// Tie break by min shard index:
	return group.minShardIndex - other.minShardIndex
}

// groupMerger renders the private static nested class
// SearchGroup.GroupMerger<T>. Java's queue is a TreeSet ordered by
// GroupComparator; because that comparator is a total order over distinct
// groups (it tie-breaks on the shard index), the Go rendering is a slice kept
// sorted by the same comparator, with the same first/last/add/remove
// operations.
type groupMerger[T any] struct {
	groupComp  *groupComparator[T]
	queue      []*mergedGroup[T]
	groupsSeen map[any]*mergedGroup[T]
}

func newGroupMerger[T any](groupSort *search.Sort) *groupMerger[T] {
	return &groupMerger[T]{
		groupComp:  newGroupComparator[T](groupSort),
		groupsSeen: make(map[any]*mergedGroup[T]),
	}
}

// queueAdd inserts group at the position the comparator dictates.
func (m *groupMerger[T]) queueAdd(group *mergedGroup[T]) {
	pos := len(m.queue)
	for i := range m.queue {
		if m.groupComp.compare(group, m.queue[i]) < 0 {
			pos = i
			break
		}
	}
	m.queue = append(m.queue, nil)
	copy(m.queue[pos+1:], m.queue[pos:])
	m.queue[pos] = group
}

// queueRemove removes group from the queue by identity.
func (m *groupMerger[T]) queueRemove(group *mergedGroup[T]) {
	for i, g := range m.queue {
		if g == group {
			m.queue = append(m.queue[:i], m.queue[i+1:]...)
			return
		}
	}
}

func (m *groupMerger[T]) pollFirst() *mergedGroup[T] {
	group := m.queue[0]
	m.queue = m.queue[1:]
	return group
}

func (m *groupMerger[T]) pollLast() *mergedGroup[T] {
	group := m.queue[len(m.queue)-1]
	m.queue = m.queue[:len(m.queue)-1]
	return group
}

// updateNextGroup mirrors GroupMerger.updateNextGroup(int, ShardIter).
func (m *groupMerger[T]) updateNextGroup(topN int, shard *shardIter[T]) {
	for shard.hasNext() {
		group := shard.next()
		key := getComparableKey(group.GroupValue)
		mergedGrp, found := m.groupsSeen[key]
		isNew := !found

		if isNew {
			// Start a new group:
			mergedGrp = &mergedGroup[T]{groupValue: group.GroupValue}
			mergedGrp.minShardIndex = shard.shardIndex
			mergedGrp.topValues = group.SortValues
			m.groupsSeen[key] = mergedGrp
			mergedGrp.inQueue = true
			m.queueAdd(mergedGrp)
		} else if mergedGrp.processed {
			// This shard produced a group that we already processed; move on to
			// the next group.
			continue
		} else {
			competes := false
			for compIDX := range m.groupComp.comparators {
				cmp := m.groupComp.reversed[compIDX] *
					m.groupComp.comparators[compIDX].CompareValues(group.SortValues[compIDX], mergedGrp.topValues[compIDX])
				if cmp < 0 {
					// Definitely competes.
					competes = true
					break
				} else if cmp > 0 {
					// Definitely does not compete.
					break
				} else if compIDX == len(m.groupComp.comparators)-1 {
					if shard.shardIndex < mergedGrp.minShardIndex {
						competes = true
					}
				}
			}

			if competes {
				// Group's sort changed -- remove and re-insert, updating the
				// first group in place for efficiency.
				skipHeavyOps := len(m.queue) > 0 && m.queue[0] == mergedGrp
				if mergedGrp.inQueue && !skipHeavyOps {
					m.queueRemove(mergedGrp)
				}
				mergedGrp.topValues = group.SortValues
				mergedGrp.minShardIndex = shard.shardIndex
				if !skipHeavyOps {
					m.queueAdd(mergedGrp)
				}
				mergedGrp.inQueue = true
			}
		}

		mergedGrp.shards = append(mergedGrp.shards, shard)
		break
	}

	// Prune un-competitive groups:
	for len(m.queue) > topN {
		group := m.pollLast()
		group.inQueue = false
	}
}

// merge mirrors GroupMerger.merge(List<Collection<SearchGroup<T>>>, int, int).
func (m *groupMerger[T]) merge(shards [][]SearchGroup[T], offset, topN int) []SearchGroup[T] {
	maxQueueSize := offset + topN

	// Init queue:
	for shardIDX := range shards {
		shard := shards[shardIDX]
		if len(shard) != 0 {
			m.updateNextGroup(maxQueueSize, newShardIter(shard, shardIDX))
		}
	}

	// Pull merged topN groups:
	newTopGroups := make([]SearchGroup[T], 0, topN)

	count := 0

	for len(m.queue) != 0 {
		group := m.pollFirst()
		group.processed = true
		if count >= offset {
			newTopGroups = append(newTopGroups, SearchGroup[T]{
				GroupValue: group.groupValue,
				SortValues: group.topValues,
			})
			if len(newTopGroups) == topN {
				break
			}
		}
		count++

		// Advance all iters in this group:
		for _, shardIter := range group.shards {
			m.updateNextGroup(maxQueueSize, shardIter)
		}
	}

	return newTopGroups
}

// MergeSearchGroups merges multiple collections of top groups, for example
// obtained from separate index shards. groupSort must match how the groups were
// sorted, and the provided SearchGroups must have been computed with
// fillFields=true passed to FirstPassGroupingCollector.GetTopGroups.
//
// Mirrors the static SearchGroup.merge(List, int, int, Sort); the Go name
// distinguishes it from TopGroups.merge, which is [Merge] in this package,
// because Go cannot overload.
func MergeSearchGroups[T any](topGroups [][]SearchGroup[T], offset, topN int, groupSort *search.Sort) []SearchGroup[T] {
	return newGroupMerger[T](groupSort).merge(topGroups, offset, topN)
}
