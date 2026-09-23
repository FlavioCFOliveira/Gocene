// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// BlockGroupingCollectorManager is a CollectorManager for
// BlockGroupingCollector that merges results from multiple collectors into a
// single TopGroups. This is intended for use with concurrent search, where
// each slice is searched by a separate BlockGroupingCollector.
//
// Documents must be indexed as blocks using IndexWriter.addDocuments() or
// IndexWriter.updateDocuments().
//
// NOTE: All documents in a group block must be processed by the same
// BlockGroupingCollector instance. This means that the IndexSearcher's slices
// must not split a segment in a way that places documents from the same block
// into different slices. The default IndexSearcher slices implementation
// (inter-segment only) satisfies this constraint. If intra-segment
// concurrency is desired, the caller must override the slices to ensure each
// doc block falls entirely within one slice.
//
// See BlockGroupingCollector for more details.
//
// Mirrors org.apache.lucene.search.grouping.BlockGroupingCollectorManager<T>
// (Apache Lucene 10.5.0).
//
// lucene.experimental
type BlockGroupingCollectorManager[T any] struct {
	groupSort       *search.Sort
	groupOffset     int
	topNGroups      int
	needsScores     bool
	lastDocPerGroup search.Weight

	withinGroupSort   *search.Sort
	withinGroupOffset int
	maxDocsPerGroup   int
}

// NewBlockGroupingCollectorManager creates a new
// BlockGroupingCollectorManager.
//
// groupSort is the sort used to rank groups; groupOffset the offset into the
// groups to start returning from; topNGroups the number of top groups to
// collect; needsScores whether scores are needed (must be true if groupSort
// or withinGroupSort uses scores); lastDocPerGroup a Weight that matches the
// last document in each group block; withinGroupSort the sort used to rank
// documents within each group; withinGroupOffset the offset into each group's
// documents to start returning from; maxDocsPerGroup the maximum number of
// documents to return per group.
//
// Java's IllegalArgumentException is returned as an error.
func NewBlockGroupingCollectorManager[T any](
	groupSort *search.Sort,
	groupOffset int,
	topNGroups int,
	needsScores bool,
	lastDocPerGroup search.Weight,
	withinGroupSort *search.Sort,
	withinGroupOffset int,
	maxDocsPerGroup int,
) (*BlockGroupingCollectorManager[T], error) {
	if groupSort == nil {
		return nil, errors.New("groupSort must not be null")
	}
	if withinGroupSort == nil {
		return nil, errors.New("withinGroupSort must not be null")
	}

	if groupOffset < 0 {
		return nil, fmt.Errorf("groupOffset must be >= 0 (got %d)", groupOffset)
	}

	if topNGroups < 1 {
		return nil, fmt.Errorf("topNGroups must be >= 1 (got %d)", topNGroups)
	}

	if withinGroupOffset < 0 {
		return nil, fmt.Errorf("withinGroupOffset must be >= 0 (got %d)", withinGroupOffset)
	}

	if maxDocsPerGroup < 1 {
		return nil, fmt.Errorf("maxDocsPerGroup must be >= 1 (got %d)", maxDocsPerGroup)
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
	}, nil
}

// NewCollector renders newCollector().
func (m *BlockGroupingCollectorManager[T]) NewCollector() (*BlockGroupingCollector, error) {
	return NewBlockGroupingCollector(m.groupSort, m.groupOffset+m.topNGroups, m.needsScores, m.lastDocPerGroup)
}

// Reduce renders reduce(Collection<BlockGroupingCollector>).
func (m *BlockGroupingCollectorManager[T]) Reduce(collectors []*BlockGroupingCollector) (*TopGroups[T], error) {
	shardGroupsList := make([]*TopGroups[T], 0)
	for _, collector := range collectors {
		topGroups, err := collector.GetTopGroups(m.withinGroupSort, 0, m.withinGroupOffset, m.maxDocsPerGroup)
		if err != nil {
			return nil, err
		}
		if topGroups != nil && len(topGroups.Groups) > 0 {
			shardGroupsList = append(shardGroupsList, uncheckedTopGroupsCast[T](topGroups))
		}
	}

	return MergeBlockGroups(shardGroupsList, m.groupSort, m.groupOffset, m.topNGroups, m.withinGroupSort), nil
}

// uncheckedTopGroupsCast renders Java's unchecked cast (TopGroups<T>)
// topGroups of a BlockGroupingCollector result: every group value is null,
// so the groups carry T's zero value.
func uncheckedTopGroupsCast[T any](topGroups *TopGroups[any]) *TopGroups[T] {
	groups := make([]*GroupDocs[T], len(topGroups.Groups))
	for i, g := range topGroups.Groups {
		var groupValue T
		if g.GroupValue != nil {
			groupValue = g.GroupValue.(T)
		}
		groups[i] = NewGroupDocs(g.Score, g.MaxScore, g.TotalHits, g.ScoreDocs, groupValue, g.GroupSortValues)
	}
	return &TopGroups[T]{
		TotalHitCount:        topGroups.TotalHitCount,
		TotalGroupedHitCount: topGroups.TotalGroupedHitCount,
		TotalGroupCount:      topGroups.TotalGroupCount,
		Groups:               groups,
		GroupSort:            topGroups.GroupSort,
		WithinGroupSort:      topGroups.WithinGroupSort,
		MaxScore:             topGroups.MaxScore,
	}
}
