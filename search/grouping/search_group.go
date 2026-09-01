package grouping

import (
	"fmt"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// SearchGroup represents a group that is found during the first pass search.
type SearchGroup[T any] struct {
	GroupValue *T
	SortValues []any
}

func (sg *SearchGroup[T]) String() string {
	return fmt.Sprintf("SearchGroup(groupValue=%v sortValues=%v)", sg.GroupValue, sg.SortValues)
}

// Merge merges multiple collections of top groups, for example obtained from separate index shards.
// The provided groupSort must match how the groups were sorted, and the provided SearchGroups must
// have been computed with fillFields=true passed to the first pass collector.
func Merge[T any](topGroups [][]SearchGroup[T], offset, topN int, groupSort *search.Sort) []SearchGroup[T] {
	if len(topGroups) == 0 {
		return nil
	}

	// Collect all unique groups across all shards
	type groupKey struct {
		val any
	}
	groupsSeen := make(map[groupKey]*mergedGroup[T])

	for shardIdx, shard := range topGroups {
		for _, group := range shard {
			var key groupKey
			if group.GroupValue != nil {
				key.val = *group.GroupValue
			}

			if mg, ok := groupsSeen[key]; ok {
				// If this shard's representative is better, update topValues
				if group.SortValues != nil {
					if mg.topValues == nil || compareSortValues(group.SortValues, mg.topValues, groupSort) < 0 {
						mg.topValues = group.SortValues
					}
				}
				if shardIdx < mg.minShardIndex {
					mg.minShardIndex = shardIdx
				}
			} else {
				groupsSeen[key] = &mergedGroup[T]{
					groupValue:    group.GroupValue,
					topValues:     group.SortValues,
					minShardIndex: shardIdx,
				}
			}
		}
	}

	// Convert to slice and sort
	merged := make([]*mergedGroup[T], 0, len(groupsSeen))
	for _, mg := range groupsSeen {
		merged = append(merged, mg)
	}

	sort.Slice(merged, func(i, j int) bool {
		cmp := compareSortValues(merged[i].topValues, merged[j].topValues, groupSort)
		if cmp != 0 {
			return cmp < 0
		}
		return merged[i].minShardIndex < merged[j].minShardIndex
	})

	// Handle offset and topN
	if offset >= len(merged) {
		return nil
	}

	end := offset + topN
	if end > len(merged) {
		end = len(merged)
	}

	result := make([]SearchGroup[T], 0, end-offset)
	for i := offset; i < end; i++ {
		mg := merged[i]
		result = append(result, SearchGroup[T]{
			GroupValue: mg.groupValue,
			SortValues: mg.topValues,
		})
	}

	return result
}

type mergedGroup[T any] struct {
	groupValue    *T
	topValues     []any
	minShardIndex int
}

func compareSortValues(v1, v2 []any, groupSort *search.Sort) int {
	if v1 == nil && v2 == nil {
		return 0
	}
	if v1 == nil {
		return 1
	}
	if v2 == nil {
		return -1
	}

	for i, sf := range groupSort.Fields {
		if i >= len(v1) || i >= len(v2) {
			break
		}
		cmp := compareValues(v1[i], v2[i])
		if sf.Reverse {
			cmp = -cmp
		}
		if cmp != 0 {
			return cmp
		}
	}
	return 0
}

func compareValues(v1, v2 any) int {
	if v1 == nil && v2 == nil {
		return 0
	}
	if v1 == nil {
		return -1
	}
	if v2 == nil {
		return 1
	}

	switch val1 := v1.(type) {
	case int:
		if val2, ok := v2.(int); ok {
			if val1 < val2 { return -1 }
			if val1 > val2 { return 1 }
			return 0
		}
	case int64:
		if val2, ok := v2.(int64); ok {
			if val1 < val2 { return -1 }
			if val1 > val2 { return 1 }
			return 0
		}
	case float64:
		if val2, ok := v2.(float64); ok {
			if val1 < val2 { return -1 }
			if val1 > val2 { return 1 }
			return 0
		}
	case string:
		if val2, ok := v2.(string); ok {
			if val1 < val2 { return -1 }
			if val1 > val2 { return 1 }
			return 0
		}
	}
	return 0
}
