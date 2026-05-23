// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"fmt"
	"sort"
)

// SearchGroup represents a single group encountered by the first-pass
// grouping collector. Mirrors org.apache.lucene.search.grouping.SearchGroup.
//
// GroupValue is the group's key (string for term-based groupings; for the
// other typed group selectors it is the value cast to interface{}).
// SortValues hold the comparator state used by the first-pass collector to
// order groups for top-N selection.
type SearchGroup[T comparable] struct {
	GroupValue T
	SortValues []any
}

// NewSearchGroup builds a SearchGroup with the supplied value and sort state.
func NewSearchGroup[T comparable](value T, sortValues []any) *SearchGroup[T] {
	cloned := make([]any, len(sortValues))
	copy(cloned, sortValues)
	return &SearchGroup[T]{GroupValue: value, SortValues: cloned}
}

// String returns a human-readable representation of the group, matching
// the Java toString() contract.
func (g *SearchGroup[T]) String() string {
	return fmt.Sprintf("SearchGroup(groupValue=%v sortValues=%v)", g.GroupValue, g.SortValues)
}

// Equal reports whether g and other represent the same group.  Equality is
// defined solely by GroupValue, mirroring the Java equals() implementation.
func (g *SearchGroup[T]) Equal(other *SearchGroup[T]) bool {
	if other == nil {
		return false
	}
	return g.GroupValue == other.GroupValue
}

// mergedGroup is the internal representation of a group being assembled
// from multiple shards during MergeSearchGroups.
type mergedGroup[T comparable] struct {
	groupValue    T
	topValues     []any
	minShardIndex int
	shardIters    []*shardIter[T]
	processed     bool
	inQueue       bool
}

// shardIter iterates over one shard's search groups.
type shardIter[T comparable] struct {
	groups     []*SearchGroup[T]
	pos        int
	shardIndex int
}

func (si *shardIter[T]) hasNext() bool { return si.pos < len(si.groups) }

func (si *shardIter[T]) next() *SearchGroup[T] {
	g := si.groups[si.pos]
	si.pos++
	return g
}

// MergeSearchGroups merges multiple collections of top groups from separate
// shards and returns at most topN groups starting at offset.  compare must
// be consistent with the sort used to produce each shard's groups (negative
// means "a before b").  Returns nil when no groups survive.
//
// Mirrors org.apache.lucene.search.grouping.SearchGroup.merge.
func MergeSearchGroups[T comparable](
	shards [][](*SearchGroup[T]),
	offset, topN int,
	compare func(a, b []any) int,
) []*SearchGroup[T] {
	if len(shards) == 0 {
		return nil
	}

	maxQueueSize := offset + topN
	seen := make(map[T]*mergedGroup[T])

	// queue ordered by compare; we use a slice and sort after mutations.
	var queue []*mergedGroup[T]

	var updateNextGroup func(iter *shardIter[T])
	updateNextGroup = func(iter *shardIter[T]) {
		for iter.hasNext() {
			sg := iter.next()
			if sg.SortValues == nil {
				panic("SearchGroup.SortValues is nil; fillFields=true must have been passed to the first-pass collector")
			}
			mg, exists := seen[sg.GroupValue]
			if !exists {
				mg = &mergedGroup[T]{
					groupValue:    sg.GroupValue,
					topValues:     sg.SortValues,
					minShardIndex: iter.shardIndex,
				}
				seen[sg.GroupValue] = mg
				mg.inQueue = true
				queue = append(queue, mg)
				sortQueue(queue, compare)
			} else if mg.processed {
				continue
			} else {
				competes := false
				for i, v := range sg.SortValues {
					var prev any
					if i < len(mg.topValues) {
						prev = mg.topValues[i]
					}
					c := compare([]any{v}, []any{prev})
					if c < 0 {
						competes = true
						break
					} else if c > 0 {
						break
					} else if i == len(sg.SortValues)-1 {
						if iter.shardIndex < mg.minShardIndex {
							competes = true
						}
					}
				}
				if competes {
					if mg.inQueue {
						removeFromQueue(&queue, mg)
					}
					mg.topValues = sg.SortValues
					mg.minShardIndex = iter.shardIndex
					queue = append(queue, mg)
					sortQueue(queue, compare)
					mg.inQueue = true
				}
			}
			mg.shardIters = append(mg.shardIters, iter)
			break
		}

		// Prune queue to maxQueueSize.
		for len(queue) > maxQueueSize {
			tail := queue[len(queue)-1]
			queue = queue[:len(queue)-1]
			tail.inQueue = false
		}
	}

	// Initialise: seed queue from every shard.
	for i, shard := range shards {
		if len(shard) == 0 {
			continue
		}
		iter := &shardIter[T]{groups: shard, shardIndex: i}
		updateNextGroup(iter)
	}

	// Pull merged top-N groups in order.
	var result []*SearchGroup[T]
	count := 0
	for len(queue) > 0 {
		mg := queue[0]
		queue = queue[1:]
		mg.processed = true
		if count >= offset {
			result = append(result, &SearchGroup[T]{
				GroupValue: mg.groupValue,
				SortValues: mg.topValues,
			})
			if len(result) == topN {
				break
			}
		}
		count++
		for _, iter := range mg.shardIters {
			updateNextGroup(iter)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// sortQueue sorts the merged-group queue so that the best (lowest by compare)
// group is at index 0.  Ties break on minShardIndex.
func sortQueue[T comparable](q []*mergedGroup[T], compare func(a, b []any) int) {
	sort.SliceStable(q, func(i, j int) bool {
		c := compare(q[i].topValues, q[j].topValues)
		if c != 0 {
			return c < 0
		}
		return q[i].minShardIndex < q[j].minShardIndex
	})
}

// removeFromQueue removes mg from q by value.
func removeFromQueue[T comparable](q *[]*mergedGroup[T], mg *mergedGroup[T]) {
	for i, m := range *q {
		if m == mg {
			*q = append((*q)[:i], (*q)[i+1:]...)
			return
		}
	}
}
