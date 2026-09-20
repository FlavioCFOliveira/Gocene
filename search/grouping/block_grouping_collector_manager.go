// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// Ported from Apache Lucene 10.5.0:
//   lucene/grouping/src/java/org/apache/lucene/search/grouping/BlockGroupingCollectorManager.java

// BlockGroupingCollectorManager is a CollectorManager for
// [BlockGroupingCollector] that merges results from multiple collectors into a
// single [TopGroups]. It is intended for use with concurrent search, where each
// slice is searched by a separate BlockGroupingCollector.
//
// Documents must be indexed as blocks. All documents in a group block must be
// processed by the same BlockGroupingCollector instance, so the searcher's
// slices must not split a block across slices.
//
// Mirrors org.apache.lucene.search.grouping.BlockGroupingCollectorManager<T>.
type BlockGroupingCollectorManager[T any] struct {
	groupSort   *search.Sort
	groupOffset int
	topNGroups  int
	needsScores bool

	lastDocPerGroup search.Weight

	withinGroupSort   *search.Sort
	withinGroupOffset int
	maxDocsPerGroup   int
}

// NewBlockGroupingCollectorManager creates a new BlockGroupingCollectorManager.
//
// groupSort ranks groups, groupOffset is the offset into the groups to start
// returning from, topNGroups the number of top groups to collect, needsScores
// says whether scores are needed (it must be true when groupSort or
// withinGroupSort uses scores), lastDocPerGroup a Weight matching the last
// document in each group block, withinGroupSort ranks documents within each
// group, withinGroupOffset is the offset into each group's documents, and
// maxDocsPerGroup the maximum number of documents to return per group.
//
// Java raises IllegalArgumentException for every violated precondition; the Go
// rendering panics, as that exception is unchecked.
func NewBlockGroupingCollectorManager[T any](groupSort *search.Sort, groupOffset, topNGroups int, needsScores bool, lastDocPerGroup search.Weight, withinGroupSort *search.Sort, withinGroupOffset, maxDocsPerGroup int) *BlockGroupingCollectorManager[T] {
	if groupSort == nil {
		panic("groupSort must not be null")
	}
	if withinGroupSort == nil {
		panic("withinGroupSort must not be null")
	}
	if groupOffset < 0 {
		panic("groupOffset must be >= 0")
	}
	if topNGroups < 1 {
		panic("topNGroups must be >= 1")
	}
	if withinGroupOffset < 0 {
		panic("withinGroupOffset must be >= 0")
	}
	if maxDocsPerGroup < 1 {
		panic("maxDocsPerGroup must be >= 1")
	}
	return &BlockGroupingCollectorManager[T]{
		groupSort:         groupSort,
		groupOffset:       groupOffset,
		topNGroups:        topNGroups,
		needsScores:       needsScores,
		lastDocPerGroup:   lastDocPerGroup,
		withinGroupSort:   withinGroupSort,
		withinGroupOffset: withinGroupOffset,
		maxDocsPerGroup:   maxDocsPerGroup,
	}
}

// NewCollector mirrors CollectorManager.newCollector().
func (m *BlockGroupingCollectorManager[T]) NewCollector() (search.Collector, error) {
	return NewBlockGroupingCollector(m.groupSort, m.groupOffset+m.topNGroups, m.needsScores, m.lastDocPerGroup), nil
}

// Reduce mirrors CollectorManager.reduce(Collection).
func (m *BlockGroupingCollectorManager[T]) Reduce(collectors []search.Collector) (*TopGroups[T], error) {
	shardGroupsList := make([]*TopGroups[T], 0, len(collectors))
	for _, c := range collectors {
		collector, ok := c.(*BlockGroupingCollector)
		if !ok {
			continue
		}
		topGroups, err := collector.GetTopGroups(m.withinGroupSort, 0, m.withinGroupOffset, m.maxDocsPerGroup)
		if err != nil {
			return nil, err
		}
		if topGroups != nil && len(topGroups.Groups) > 0 {
			shardGroupsList = append(shardGroupsList, retypeTopGroups[T](topGroups))
		}
	}

	return MergeBlockGroups(shardGroupsList, m.groupSort, m.groupOffset, m.topNGroups, m.withinGroupSort), nil
}

// retypeTopGroups renders Java's unchecked cast (TopGroups<T>) applied to the
// TopGroups<?> that BlockGroupingCollector returns: the collector cannot
// compute a group value, so every GroupDocs carries a null one and the type
// parameter is free. The Go rendering rebuilds the value with the zero group
// value of T, which is the Go spelling of that null.
func retypeTopGroups[T any](in *TopGroups[any]) *TopGroups[T] {
	groups := make([]*GroupDocs[T], len(in.Groups))
	for i, g := range in.Groups {
		var zero T
		groups[i] = &GroupDocs[T]{
			Score:           g.Score,
			MaxScore:        g.MaxScore,
			TotalHits:       g.TotalHits,
			ScoreDocs:       g.ScoreDocs,
			FieldDocs:       g.FieldDocs,
			GroupValue:      zero,
			GroupSortValues: g.GroupSortValues,
		}
	}
	out := NewTopGroups(in.GroupSort, in.WithinGroupSort, in.TotalHitCount, in.TotalGroupedHitCount, groups, in.MaxScore)
	out.TotalGroupCount = in.TotalGroupCount
	return out
}
