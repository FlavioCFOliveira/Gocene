package grouping

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// TopGroups represents result returned by a grouping search.
type TopGroups[T any] struct {
	TotalHitCount        int
	TotalGroupedHitCount int
	TotalGroupCount      *int
	Groups              []*GroupDocs[T]
	GroupSort            []search.SortField
	WithinGroupSort      []search.SortField
	MaxScore             float32
}

func NewTopGroups[T any](groupSort, withinGroupSort []search.SortField, totalHitCount, totalGroupedHitCount int, groups []*GroupDocs[T], maxScore float32) *TopGroups[T] {
	return &TopGroups[T]{
		GroupSort:            groupSort,
		WithinGroupSort:      withinGroupSort,
		TotalHitCount:        totalHitCount,
		TotalGroupedHitCount: totalGroupedHitCount,
		Groups:               groups,
		MaxScore:             maxScore,
	}
}

func nonNANmax(a, b float32) float32 {
	if math.IsNaN(float32(a)) {
		return b
	}
	if math.IsNaN(float32(b)) {
		return a
	}
	if a > b {
		return a
	}
	return b
}

// Merge merges an array of TopGroups, for example obtained from the second-pass collector across
// multiple shards.
func Merge[T any](shardGroups []*TopGroups[T], groupSort, docSort *search.Sort, docOffset, docTopN, scoreMergeMode int) *TopGroups[T] {
	if len(shardGroups) == 0 {
		return nil
	}

	totalHitCount := 0
	totalGroupedHitCount := 0
	var totalGroupCount *int

	numGroups := len(shardGroups[0].Groups)
	for _, shard := range shardGroups {
		if len(shard.Groups) != numGroups {
			panic("number of groups differs across shards")
		}
		totalHitCount += shard.TotalHitCount
		totalGroupedHitCount += shard.TotalGroupedHitCount
		if shard.TotalGroupCount != nil {
			if totalGroupCount == nil {
				tgc := 0
				totalGroupCount = &tgc
			}
			*totalGroupCount += *shard.TotalGroupCount
		}
	}

	mergedGroupDocs := make([]*GroupDocs[T], numGroups)
	sortByRelevance := docSort.IsRelevance()
	totalMaxScore := float32(math.NaN())

	for groupIDX := 0; groupIDX < numGroups; groupIDX++ {
		groupValue := shardGroups[0].Groups[groupIDX].GroupValue
		maxScore := float32(math.NaN())
		totalHits := 0
		scoreSum := 0.0

		shardTopDocs := make([]search.TopDocs, len(shardGroups))
		for shardIDX, shard := range shardGroups {
			shardGroupDocs := shard.Groups[groupIDX]
			if groupValue != nil && (shardGroupDocs.GroupValue == nil || *groupValue != *shardGroupDocs.GroupValue) {
				panic("group values differ across shards")
			}

			if sortByRelevance {
				shardTopDocs[shardIDX] = search.TopDocs{
					TotalHits:  shardGroupDocs.TotalHits.Value,
					ScoreDocs: shardGroupDocs.ScoreDocs,
				}
			} else {
				shardTopDocs[shardIDX] = search.TopDocs{
					TotalHits:  shardGroupDocs.TotalHits.Value,
					ScoreDocs: shardGroupDocs.ScoreDocs,
				}
			}

			if !sortByRelevance {
				maxScore = nonNANmax(maxScore, shardGroupDocs.MaxScore)
			}
			totalHits += shardGroupDocs.TotalHits.Value
			for _, sd := range shardGroupDocs.ScoreDocs {
				scoreSum += float64(sd.Score)
			}
		}

		var mergedTopDocs search.TopDocs
		if sortByRelevance {
			mergedTopDocs = search.MergeTopDocs(docOffset+docTopN, shardTopDocs)
			if len(mergedTopDocs.ScoreDocs) > 0 {
				maxScore = mergedTopDocs.ScoreDocs[0].Score
			} else {
				maxScore = float32(math.NaN())
			}
		} else {
			mergedTopDocs = search.MergeTopFieldDocs(docSort, docOffset+docTopN, shardTopDocs)
		}

		var mergedScoreDocs []search.ScoreDoc
		if docOffset == 0 {
			mergedScoreDocs = mergedTopDocs.ScoreDocs
		} else if docOffset >= len(mergedTopDocs.ScoreDocs) {
			mergedScoreDocs = []search.ScoreDoc{}
		} else {
			end := docOffset + docTopN
			if end > len(mergedTopDocs.ScoreDocs) {
				end = len(mergedTopDocs.ScoreDocs)
			}
			mergedScoreDocs = mergedTopDocs.ScoreDocs[docOffset:end]
		}

		var groupScore float32
		switch scoreMergeMode {
		case 0: // None
			groupScore = float32(math.NaN())
		case 1: // Total
			groupScore = float32(scoreSum)
		case 2: // Avg
			if totalHits > 0 {
				groupScore = float32(scoreSum / float64(totalHits))
			} else {
				groupScore = float32(math.NaN())
			}
		}

		mergedGroupDocs[groupIDX] = &GroupDocs[T]{
			Score:           groupScore,
			MaxScore:       maxScore,
			TotalHits:       search.TotalHits{Value: totalHits, Relation: search.TotalHitsEqual},
			ScoreDocs:       mergedScoreDocs,
			GroupValue:      groupValue,
			GroupSortValues: shardGroups[0].Groups[groupIDX].GroupSortValues,
		}
		totalMaxScore = nonNANmax(totalMaxScore, maxScore)
	}

	return NewTopGroups(groupSort.Fields, docSort.Fields, totalHitCount, totalGroupedHitCount, mergedGroupDocs, totalMaxScore)
}
