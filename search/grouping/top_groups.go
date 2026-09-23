// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"errors"
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// TopGroups represents result returned by a grouping search.
//
// Mirrors org.apache.lucene.search.grouping.TopGroups<T>.
//
// lucene.experimental
type TopGroups[T any] struct {
	// TotalHitCount is the number of documents matching the search.
	TotalHitCount int

	// TotalGroupedHitCount is the number of documents grouped into the topN
	// groups.
	TotalGroupedHitCount int

	// TotalGroupCount is the total number of unique groups. If nil this value
	// is not computed. It renders Java's nullable Integer.
	TotalGroupCount *int

	// Groups holds the group results in groupSort order.
	Groups []*GroupDocs[T]

	// GroupSort is how groups are sorted against each other.
	GroupSort []*search.SortField

	// WithinGroupSort is how docs are sorted within each group.
	WithinGroupSort []*search.SortField

	// MaxScore is the highest score across all hits, or NaN if scores were
	// not computed.
	MaxScore float32
}

// NewTopGroups mirrors TopGroups(SortField[], SortField[], int, int,
// GroupDocs[], float).
func NewTopGroups[T any](
	groupSort []*search.SortField,
	withinGroupSort []*search.SortField,
	totalHitCount int,
	totalGroupedHitCount int,
	groups []*GroupDocs[T],
	maxScore float32,
) *TopGroups[T] {
	return &TopGroups[T]{
		GroupSort:            groupSort,
		WithinGroupSort:      withinGroupSort,
		TotalHitCount:        totalHitCount,
		TotalGroupedHitCount: totalGroupedHitCount,
		Groups:               groups,
		TotalGroupCount:      nil,
		MaxScore:             maxScore,
	}
}

// NewTopGroupsWithTotalGroupCount mirrors TopGroups(TopGroups<T>, Integer).
func NewTopGroupsWithTotalGroupCount[T any](oldTopGroups *TopGroups[T], totalGroupCount *int) *TopGroups[T] {
	return &TopGroups[T]{
		GroupSort:            oldTopGroups.GroupSort,
		WithinGroupSort:      oldTopGroups.WithinGroupSort,
		TotalHitCount:        oldTopGroups.TotalHitCount,
		TotalGroupedHitCount: oldTopGroups.TotalGroupedHitCount,
		Groups:               oldTopGroups.Groups,
		MaxScore:             oldTopGroups.MaxScore,
		TotalGroupCount:      totalGroupCount,
	}
}

// ScoreMergeMode renders the nested enum TopGroups.ScoreMergeMode: how the
// GroupDocs score (if any) should be merged.
type ScoreMergeMode int

const (
	// ScoreMergeModeNone sets the score to NaN.
	ScoreMergeModeNone ScoreMergeMode = iota
	// ScoreMergeModeTotal sums the score across all shards for this group.
	ScoreMergeModeTotal
	// ScoreMergeModeAvg averages the score across all shards for this group.
	ScoreMergeModeAvg
)

// String returns the Java enum constant name.
func (m ScoreMergeMode) String() string {
	switch m {
	case ScoreMergeModeNone:
		return "None"
	case ScoreMergeModeTotal:
		return "Total"
	case ScoreMergeModeAvg:
		return "Avg"
	default:
		return "UNKNOWN"
	}
}

// nonNANmax returns the other value if either is NaN, otherwise the greater
// of the two.
//
// Mirrors the package-private static float nonNANmax(float, float).
func nonNANmax(a, b float32) float32 {
	if math.IsNaN(float64(a)) {
		return b
	}
	if math.IsNaN(float64(b)) {
		return a
	}
	if a > b {
		return a
	}
	return b
}

// MergeTopGroups merges an array of TopGroups, for example obtained from the
// second-pass collector across multiple shards. Each TopGroups must have been
// sorted by the same groupSort and docSort, and the top groups passed to all
// second-pass collectors must be the same.
//
// NOTE: We can't always compute an exact totalGroupCount. Documents belonging
// to a group may occur on more than one shard and thus the merged
// totalGroupCount can be higher than the actual totalGroupCount. In this case
// the totalGroupCount represents an upper bound. If the documents of one
// group do only reside in one shard then the totalGroupCount is exact.
//
// Mirrors the static method TopGroups.merge. The Java method signals bad
// input with IllegalArgumentException; the Go rendering returns that as an
// error.
func MergeTopGroups[T any](
	shardGroups []*TopGroups[T],
	groupSort *search.Sort,
	docSort *search.Sort,
	docOffset int,
	docTopN int,
	scoreMergeMode ScoreMergeMode,
) (*TopGroups[T], error) {

	if len(shardGroups) == 0 {
		return nil, nil
	}

	totalHitCount := 0
	totalGroupedHitCount := 0
	// Optionally merge the totalGroupCount.
	var totalGroupCount *int

	numGroups := len(shardGroups[0].Groups)
	for _, shard := range shardGroups {
		if numGroups != len(shard.Groups) {
			return nil, errors.New(
				"number of groups differs across shards; you must pass same top groups to all shards' second-pass collector")
		}
		totalHitCount += shard.TotalHitCount
		totalGroupedHitCount += shard.TotalGroupedHitCount
		if shard.TotalGroupCount != nil {
			if totalGroupCount == nil {
				zero := 0
				totalGroupCount = &zero
			}

			*totalGroupCount += *shard.TotalGroupCount
		}
	}

	mergedGroupDocs := make([]*GroupDocs[T], numGroups)

	sortByRelevance := docSort.Equals(search.RELEVANCE)
	var shardTopDocs []*search.TopDocs
	var shardTopFieldDocs []*search.TopFieldDocs
	if sortByRelevance {
		shardTopDocs = make([]*search.TopDocs, len(shardGroups))
	} else {
		shardTopFieldDocs = make([]*search.TopFieldDocs, len(shardGroups))
	}
	totalMaxScore := float32(math.NaN())

	for groupIDX := 0; groupIDX < numGroups; groupIDX++ {
		groupValue := shardGroups[0].Groups[groupIDX].GroupValue
		groupValueKey := groupKey(any(groupValue))
		maxScore := float32(math.NaN())
		totalHits := int64(0)
		scoreSum := 0.0
		for shardIDX := 0; shardIDX < len(shardGroups); shardIDX++ {
			shard := shardGroups[shardIDX]
			shardGroupDocs := shard.Groups[groupIDX]
			if groupValueKey != groupKey(any(shardGroupDocs.GroupValue)) {
				return nil, errors.New(
					"group values differ across shards; you must pass same top groups to all shards' second-pass collector")
			}

			var current *search.TopDocs
			if sortByRelevance {
				shardTopDocs[shardIDX] = search.NewTopDocs(shardGroupDocs.TotalHits, shardGroupDocs.ScoreDocs)
				current = shardTopDocs[shardIDX]
			} else {
				shardTopFieldDocs[shardIDX] = search.NewTopFieldDocs(
					shardGroupDocs.TotalHits, shardGroupDocs.ScoreDocs, docSort.GetSort())
				current = shardTopFieldDocs[shardIDX].TopDocs
			}

			for i := 0; i < len(current.ScoreDocs); i++ {
				current.ScoreDocs[i].ShardIndex = shardIDX
			}

			if !sortByRelevance {
				maxScore = nonNANmax(maxScore, shardGroupDocs.MaxScore)
			}
			// Java asserts shardGroupDocs.totalHits().relation() == EQUAL_TO here.
			totalHits += shardGroupDocs.TotalHits.Value
			scoreSum += float64(shardGroupDocs.Score)
		}

		var mergedTopDocs *search.TopDocs
		if sortByRelevance {
			mergedTopDocs = search.MergeSimple(docOffset+docTopN, shardTopDocs)
			// When sorting by relevance, the highest-scoring doc is first, so we can
			// derive maxScore directly instead of accumulating across shards.
			if len(mergedTopDocs.ScoreDocs) == 0 {
				maxScore = float32(math.NaN())
			} else {
				maxScore = mergedTopDocs.ScoreDocs[0].Score
			}
		} else {
			merged, err := search.MergeSort(docSort, 0, docOffset+docTopN, shardTopFieldDocs)
			if err != nil {
				return nil, err
			}
			mergedTopDocs = merged.TopDocs
		}

		// Slice;
		var mergedScoreDocs []*search.ScoreDoc
		if docOffset == 0 {
			mergedScoreDocs = mergedTopDocs.ScoreDocs
		} else if docOffset >= len(mergedTopDocs.ScoreDocs) {
			mergedScoreDocs = []*search.ScoreDoc{}
		} else {
			mergedScoreDocs = make([]*search.ScoreDoc, len(mergedTopDocs.ScoreDocs)-docOffset)
			copy(mergedScoreDocs, mergedTopDocs.ScoreDocs[docOffset:])
		}

		var groupScore float32
		switch scoreMergeMode {
		case ScoreMergeModeNone:
			groupScore = float32(math.NaN())
		case ScoreMergeModeAvg:
			if totalHits > 0 {
				groupScore = float32(scoreSum / float64(totalHits))
			} else {
				groupScore = float32(math.NaN())
			}
		case ScoreMergeModeTotal:
			groupScore = float32(scoreSum)
		default:
			return nil, fmt.Errorf("can't handle ScoreMergeMode %v", scoreMergeMode)
		}

		mergedGroupDocs[groupIDX] = NewGroupDocs(
			groupScore,
			maxScore,
			search.NewTotalHits(totalHits, spi.EQUAL_TO),
			mergedScoreDocs,
			groupValue,
			shardGroups[0].Groups[groupIDX].GroupSortValues,
		)
		totalMaxScore = nonNANmax(totalMaxScore, maxScore)
	}

	if totalGroupCount != nil {
		result := NewTopGroups(
			groupSort.GetSort(),
			docSort.GetSort(),
			totalHitCount,
			totalGroupedHitCount,
			mergedGroupDocs,
			totalMaxScore)
		return NewTopGroupsWithTotalGroupCount(result, totalGroupCount), nil
	}
	return NewTopGroups(
		groupSort.GetSort(),
		docSort.GetSort(),
		totalHitCount,
		totalGroupedHitCount,
		mergedGroupDocs,
		totalMaxScore), nil
}

// mergedBlockGroup mirrors the private record
// TopGroups.MergedBlockGroup(Object[] topValues, int shardIndex, int groupIndex).
type mergedBlockGroup struct {
	topValues  []any
	shardIndex int
	groupIndex int
}

// topGroupsGroupComparator mirrors the private static class
// TopGroups.GroupComparator implements Comparator<MergedBlockGroup>.
type topGroupsGroupComparator struct {
	comparators []search.FieldComparator
	reversed    []int
}

// newTopGroupsGroupComparator mirrors GroupComparator(Sort).
func newTopGroupsGroupComparator(groupSort *search.Sort) *topGroupsGroupComparator {
	sortFields := groupSort.GetSort()
	c := &topGroupsGroupComparator{
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

// compare mirrors GroupComparator.compare(MergedBlockGroup, MergedBlockGroup).
func (c *topGroupsGroupComparator) compare(group, other *mergedBlockGroup) int {
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

	return group.shardIndex - other.shardIndex
}

// MergeBlockGroups merges TopGroups that are partitioned into blocks per
// shard. This function assumes that within each shard, the groups are sorted
// according to the groupSort.
//
// shardGroups is the list of TopGroups, one per shard. groupSort is the Sort
// used to sort the groups: the top sorted document within each group
// according to groupSort determines how that group sorts against other
// groups; it must be non-nil, i.e. if you want to groupSort by relevance use
// search.RELEVANCE. groupOffset says which group to start from, topNGroups
// how many top groups to keep, and docSort is the sort to use within each
// group.
//
// Mirrors the static method TopGroups.mergeBlockGroups.
func MergeBlockGroups[T any](
	shardGroups []*TopGroups[T],
	groupSort *search.Sort,
	groupOffset int,
	topNGroups int,
	docSort *search.Sort,
) *TopGroups[T] {
	if len(shardGroups) == 0 {
		return NewTopGroups(
			groupSort.GetSort(),
			docSort.GetSort(),
			0,
			0,
			[]*GroupDocs[T]{},
			float32(math.NaN()))
	}

	var totalGroupCount *int
	totalHitCount := 0
	totalGroupedHitCount := 0
	for _, sg := range shardGroups {
		totalHitCount += sg.TotalHitCount
		if sg.TotalGroupCount != nil {
			if totalGroupCount == nil {
				zero := 0
				totalGroupCount = &zero
			}
			*totalGroupCount += *sg.TotalGroupCount
		}
	}

	// k-way merge
	groupComp := newTopGroupsGroupComparator(groupSort)
	queue := newTreeSet(groupComp.compare)

	totalMaxScore := float32(math.NaN())
	groupSortByRelevance := groupSort.Equals(search.RELEVANCE)
	// init queue
	for idx := 0; idx < len(shardGroups); idx++ {
		topGroups := shardGroups[idx]
		if len(topGroups.Groups) == 0 {
			continue
		}
		if !groupSortByRelevance {
			totalMaxScore = nonNANmax(totalMaxScore, topGroups.MaxScore)
		}
		firstGroupDocs := topGroups.Groups[0]
		queue.add(&mergedBlockGroup{topValues: firstGroupDocs.GroupSortValues, shardIndex: idx, groupIndex: 0})
	}

	if groupSortByRelevance && !queue.isEmpty() {
		totalMaxScore = shardGroups[queue.first().shardIndex].MaxScore
	}

	groupDocsList := make([]*GroupDocs[T], 0)
	count := 0
	for !queue.isEmpty() {
		mbg, _ := queue.pollFirst()
		shardGroup := shardGroups[mbg.shardIndex]

		currentGroupIndex := mbg.groupIndex
		currentGroupDocs := shardGroup.Groups[currentGroupIndex]
		if count >= groupOffset {
			groupDocsList = append(groupDocsList, currentGroupDocs)
			totalGroupedHitCount += int(currentGroupDocs.TotalHits.Value)
			if len(groupDocsList) == topNGroups {
				break
			}
		}
		count++

		nextGroupIndex := currentGroupIndex + 1
		if nextGroupIndex < len(shardGroup.Groups) {
			nextGroupDocs := shardGroup.Groups[nextGroupIndex]
			queue.add(&mergedBlockGroup{
				topValues:  nextGroupDocs.GroupSortValues,
				shardIndex: mbg.shardIndex,
				groupIndex: nextGroupIndex,
			})
		}
	}

	return NewTopGroupsWithTotalGroupCount(
		NewTopGroups(
			groupSort.GetSort(),
			docSort.GetSort(),
			totalHitCount,
			totalGroupedHitCount,
			groupDocsList,
			totalMaxScore),
		totalGroupCount)
}
