// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"fmt"
	"math"
	"reflect"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// Ported from Apache Lucene 10.5.0:
//   lucene/grouping/src/java/org/apache/lucene/search/grouping/TopGroups.java

// TopGroups represents the result returned by a grouping search.
//
// Mirrors org.apache.lucene.search.grouping.TopGroups<T>.
type TopGroups[T any] struct {
	// TotalHitCount is the number of documents matching the search.
	TotalHitCount int

	// TotalGroupedHitCount is the number of documents grouped into the topN
	// groups.
	TotalGroupedHitCount int

	// TotalGroupCount is the total number of unique groups. A nil value means
	// it was not computed, rendering Java's null Integer.
	TotalGroupCount *int

	// Groups are the group results in groupSort order.
	Groups []*GroupDocs[T]

	// GroupSort is how groups are sorted against each other.
	GroupSort []*search.SortField

	// WithinGroupSort is how docs are sorted within each group.
	WithinGroupSort []*search.SortField

	// MaxScore is the highest score across all hits, or NaN if scores were not
	// computed.
	MaxScore float32
}

// NewTopGroups mirrors TopGroups(SortField[], SortField[], int, int,
// GroupDocs[], float), which leaves totalGroupCount null.
func NewTopGroups[T any](groupSort, withinGroupSort []*search.SortField, totalHitCount, totalGroupedHitCount int, groups []*GroupDocs[T], maxScore float32) *TopGroups[T] {
	return &TopGroups[T]{
		GroupSort:            groupSort,
		WithinGroupSort:      withinGroupSort,
		TotalHitCount:        totalHitCount,
		TotalGroupedHitCount: totalGroupedHitCount,
		Groups:               groups,
		MaxScore:             maxScore,
	}
}

// NewTopGroupsWithCount mirrors TopGroups(TopGroups, Integer), the copy
// constructor that attaches a total group count.
func NewTopGroupsWithCount[T any](oldTopGroups *TopGroups[T], totalGroupCount *int) *TopGroups[T] {
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

// ScoreMergeMode says how the GroupDocs score (if any) should be merged.
//
// Mirrors the nested enum TopGroups.ScoreMergeMode.
type ScoreMergeMode int

const (
	// ScoreMergeModeNone sets the score to NaN.
	ScoreMergeModeNone ScoreMergeMode = iota
	// ScoreMergeModeTotal sums the score across all shards for this group.
	ScoreMergeModeTotal
	// ScoreMergeModeAvg averages the score across all shards for this group.
	ScoreMergeModeAvg
)

// nonNANmax returns the other value if either is NaN, and otherwise the greater
// of the two.
//
// Mirrors the package-private static TopGroups.nonNANmax(float, float).
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

// Merge merges a slice of TopGroups, for example obtained from the second-pass
// collector across multiple shards. Each TopGroups must have been sorted by the
// same groupSort and docSort, and the top groups passed to all second-pass
// collectors must be the same.
//
// NOTE: an exact totalGroupCount cannot always be computed. Documents belonging
// to a group may occur on more than one shard, so the merged totalGroupCount
// can be higher than the actual one; in that case it is an upper bound. When
// the documents of one group reside in a single shard the totalGroupCount is
// exact.
//
// Mirrors the static TopGroups.merge(TopGroups[], Sort, Sort, int, int,
// ScoreMergeMode). Java's IllegalArgumentException for mismatched shards is
// unchecked and is rendered as a panic; the returned error carries the failure
// of the underlying field-docs merge, which Gocene's TopDocs merge reports
// rather than throwing.
func Merge[T any](shardGroups []*TopGroups[T], groupSort, docSort *search.Sort, docOffset, docTopN int, scoreMergeMode ScoreMergeMode) (*TopGroups[T], error) {
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
			panic("number of groups differs across shards; you must pass same top groups to all shards' second-pass collector")
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
	shardTopDocs := make([]*search.TopDocs, len(shardGroups))
	shardTopFieldDocs := make([]*search.TopFieldDocs, len(shardGroups))
	totalMaxScore := float32(math.NaN())

	for groupIDX := 0; groupIDX < numGroups; groupIDX++ {
		groupValue := shardGroups[0].Groups[groupIDX].GroupValue
		maxScore := float32(math.NaN())
		totalHits := int64(0)
		scoreSum := 0.0
		for shardIDX := range shardGroups {
			shard := shardGroups[shardIDX]
			shardGroupDocs := shard.Groups[groupIDX]
			if !reflect.DeepEqual(groupValue, shardGroupDocs.GroupValue) {
				panic("group values differ across shards; you must pass same top groups to all shards' second-pass collector")
			}

			if sortByRelevance {
				shardTopDocs[shardIDX] = search.NewTopDocs(shardGroupDocs.TotalHits, shardGroupDocs.ScoreDocs)
			} else {
				if len(shardGroupDocs.FieldDocs) != len(shardGroupDocs.ScoreDocs) {
					return nil, fmt.Errorf("grouping: merging a non-relevance within-group sort needs the per-hit FieldDocs of group %d in shard %d", groupIDX, shardIDX)
				}
				shardTopFieldDocs[shardIDX] = search.NewTopFieldDocsWithFieldDocs(shardGroupDocs.TotalHits, shardGroupDocs.FieldDocs, docSort.GetSort())
				shardTopDocs[shardIDX] = shardTopFieldDocs[shardIDX].TopDocs
			}

			for i := range shardTopDocs[shardIDX].ScoreDocs {
				shardTopDocs[shardIDX].ScoreDocs[i].ShardIndex = shardIDX
			}

			if !sortByRelevance {
				maxScore = nonNANmax(maxScore, shardGroupDocs.MaxScore)
			}
			totalHits += shardGroupDocs.TotalHits.Value
			scoreSum += float64(shardGroupDocs.Score)
		}

		var mergedTopDocs *search.TopDocs
		if sortByRelevance {
			mergedTopDocs = search.Merge(0, docOffset+docTopN, shardTopDocs, nil)
			// When sorting by relevance, the highest-scoring doc is first, so we
			// can derive maxScore directly instead of accumulating across shards.
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

		// Slice:
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
			panic(fmt.Sprintf("can't handle ScoreMergeMode %d", int(scoreMergeMode)))
		}

		mergedGroupDocs[groupIDX] = NewGroupDocs(
			groupScore,
			maxScore,
			search.NewTotalHits(totalHits, search.EQUAL_TO),
			mergedScoreDocs,
			groupValue,
			shardGroups[0].Groups[groupIDX].GroupSortValues,
		)
		totalMaxScore = nonNANmax(totalMaxScore, maxScore)
	}

	result := NewTopGroups(
		groupSort.GetSort(),
		docSort.GetSort(),
		totalHitCount,
		totalGroupedHitCount,
		mergedGroupDocs,
		totalMaxScore,
	)
	if totalGroupCount != nil {
		return NewTopGroupsWithCount(result, totalGroupCount), nil
	}
	return result, nil
}

// mergedBlockGroup renders the private record TopGroups.MergedBlockGroup.
type mergedBlockGroup struct {
	topValues  []any
	shardIndex int
	groupIndex int
}

// blockGroupComparator renders the private static nested class
// TopGroups.GroupComparator, which orders MergedBlockGroup values and
// tie-breaks on the shard index.
type blockGroupComparator struct {
	comparators []search.FieldComparator
	reversed    []int
}

func newBlockGroupComparator(groupSort *search.Sort) *blockGroupComparator {
	sortFields := groupSort.GetSort()
	gc := &blockGroupComparator{
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

func (gc *blockGroupComparator) compare(group, other *mergedBlockGroup) int {
	if group == other {
		return 0
	}
	for compIDX := range gc.comparators {
		c := gc.reversed[compIDX] * gc.comparators[compIDX].CompareValues(group.topValues[compIDX], other.topValues[compIDX])
		if c != 0 {
			return c
		}
	}
	return group.shardIndex - other.shardIndex
}

// MergeBlockGroups merges TopGroups that are partitioned into blocks per shard.
// It assumes that within each shard the groups are sorted according to
// groupSort.
//
// shardGroups is one TopGroups per shard. groupSort is the Sort used to sort
// the groups: the top sorted document within each group according to groupSort
// determines how that group sorts against other groups, and it must be
// non-nil — use [search.RELEVANCE] to group-sort by relevance. groupOffset is
// which group to start from, topNGroups how many top groups to keep, and
// docSort the sort used within each group.
//
// Mirrors the static TopGroups.mergeBlockGroups(List, Sort, int, int, Sort).
// Java's queue is a TreeSet over the GroupComparator, a total order because it
// tie-breaks on the shard index; the Go rendering keeps a slice sorted by the
// same comparator.
func MergeBlockGroups[T any](shardGroups []*TopGroups[T], groupSort *search.Sort, groupOffset, topNGroups int, docSort *search.Sort) *TopGroups[T] {
	if len(shardGroups) == 0 {
		return NewTopGroups(groupSort.GetSort(), docSort.GetSort(), 0, 0, []*GroupDocs[T]{}, float32(math.NaN()))
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
	groupComp := newBlockGroupComparator(groupSort)
	var queue []*mergedBlockGroup
	queueAdd := func(group *mergedBlockGroup) {
		pos := len(queue)
		for i := range queue {
			if groupComp.compare(group, queue[i]) < 0 {
				pos = i
				break
			}
		}
		queue = append(queue, nil)
		copy(queue[pos+1:], queue[pos:])
		queue[pos] = group
	}

	totalMaxScore := float32(math.NaN())
	groupSortByRelevance := groupSort.Equals(search.RELEVANCE)
	// init queue
	for idx := range shardGroups {
		topGroups := shardGroups[idx]
		if len(topGroups.Groups) == 0 {
			continue
		}
		if !groupSortByRelevance {
			totalMaxScore = nonNANmax(totalMaxScore, topGroups.MaxScore)
		}
		firstGroupDocs := topGroups.Groups[0]
		queueAdd(&mergedBlockGroup{topValues: firstGroupDocs.GroupSortValues, shardIndex: idx, groupIndex: 0})
	}

	if groupSortByRelevance && len(queue) != 0 {
		totalMaxScore = shardGroups[queue[0].shardIndex].MaxScore
	}

	groupDocsList := make([]*GroupDocs[T], 0, topNGroups)
	count := 0
	for len(queue) != 0 {
		mergedBlock := queue[0]
		queue = queue[1:]
		shardGroup := shardGroups[mergedBlock.shardIndex]

		currentGroupIndex := mergedBlock.groupIndex
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
			queueAdd(&mergedBlockGroup{topValues: nextGroupDocs.GroupSortValues, shardIndex: mergedBlock.shardIndex, groupIndex: nextGroupIndex})
		}
	}

	return NewTopGroupsWithCount(
		NewTopGroups(
			groupSort.GetSort(),
			docSort.GetSort(),
			totalHitCount,
			totalGroupedHitCount,
			groupDocsList,
			totalMaxScore,
		),
		totalGroupCount,
	)
}
