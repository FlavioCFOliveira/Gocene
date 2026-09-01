package grouping

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// GroupingSearch is a convenience class to perform grouping in a non distributed environment.
type GroupingSearch[T any] struct {
	grouper                    GroupSelector[T]
	groupEndDocs               search.Query
	groupSort                  *search.Sort
	sortWithinGroup            *search.Sort
	groupDocsOffset            int
	groupDocsLimit             int
	includeMaxScore            bool
	ignoreDocsWithoutGroupField bool
	allGroups                  bool
	allGroupHeads              bool
	matchingGroups             []T
	matchingGroupHeads         []int
}

func NewGroupingSearch[T any](grouper GroupSelector[T]) *GroupingSearch[T] {
	return &GroupingSearch[T]{
		grouper:           grouper,
		groupSort:         search.DefaultSort,
		sortWithinGroup:   search.DefaultSort,
		groupDocsLimit:    1,
		includeMaxScore:   true,
	}
}

func NewGroupingSearchByDocBlock[T any](groupEndDocs search.Query) *GroupingSearch[T] {
	return &GroupingSearch[T]{
		groupEndDocs: groupEndDocs,
		groupSort:     search.DefaultSort,
		sortWithinGroup: search.DefaultSort,
		groupDocsLimit: 1,
		includeMaxScore: true,
	}
}

func (gs *GroupingSearch[T]) SetGroupSort(sort *search.Sort) *GroupingSearch[T] {
	gs.groupSort = sort
	return gs
}

func (gs *GroupingSearch[T]) SetSortWithinGroup(sort *search.Sort) *GroupingSearch[T] {
	gs.sortWithinGroup = sort
	return gs
}

func (gs *GroupingSearch[T]) SetGroupDocsOffset(offset int) *GroupingSearch[T] {
	gs.groupDocsOffset = offset
	return gs
}

func (gs *GroupingSearch[T]) SetGroupDocsLimit(limit int) *GroupingSearch[T] {
	gs.groupDocsLimit = limit
	return gs
}

func (gs *GroupingSearch[T]) SetIncludeMaxScore(include bool) *GroupingSearch[T] {
	gs.includeMaxScore = include
	return gs
}

func (gs *GroupingSearch[T]) SetAllGroups(all bool) *GroupingSearch[T] {
	gs.allGroups = all
	return gs
}

func (gs *GroupingSearch[T]) SetAllGroupHeads(all bool) *GroupingSearch[T] {
	gs.allGroupHeads = all
	return gs
}

func (gs *GroupingSearch[T]) SetIgnoreDocsWithoutGroupField(ignore bool) *GroupingSearch[T] {
	gs.ignoreDocsWithoutGroupField = ignore
	return gs
}

func (gs *GroupingSearch[T]) Search(searcher *search.IndexSearcher, query search.Query, groupOffset, groupLimit int) (*TopGroups[T], error) {
	if gs.grouper != nil {
		return gs.groupByFieldOrFunction(searcher, query, groupOffset, groupLimit)
	} else if gs.groupEndDocs != nil {
		return gs.groupByDocBlock(searcher, query, groupOffset, groupLimit)
	}
	return nil, nil
}

func (gs *GroupingSearch[T]) groupByFieldOrFunction(searcher *search.IndexSearcher, query search.Query, groupOffset, groupLimit int) (*TopGroups[T], error) {
	topN := groupOffset + groupLimit

	firstPassCollector := NewFirstPassGroupingCollector(gs.grouper, gs.groupSort, topN, gs.ignoreDocsWithoutGroupField)

	var firstRound search.Collector = firstPassCollector

	// Simplified: No CachingCollector for now.
	if err := searcher.Search(query, firstRound); err != nil {
		return nil, err
	}

	topSearchGroups, err := firstPassCollector.GetTopGroups(groupOffset)
	if err != nil {
		return nil, err
	}
	if topSearchGroups == nil {
		return NewTopGroups(gs.groupSort.Fields, gs.sortWithinGroup.Fields, 0, 0, nil, 0), nil
	}

	topNInsideGroup := gs.groupDocsOffset + gs.groupDocsLimit
	secondPassCollector := NewTopGroupsCollector(
		gs.grouper, topSearchGroups, gs.groupSort, gs.sortWithinGroup, topNInsideGroup, gs.includeMaxScore,
	)

	if err := searcher.Search(query, secondPassCollector); err != nil {
		return nil, err
	}

	return secondPassCollector.GetTopGroups(gs.groupDocsOffset), nil
}

func (gs *GroupingSearch[T]) groupByDocBlock(searcher *search.IndexSearcher, query search.Query, groupOffset, groupLimit int) (*TopGroups[T], error) {
	// This would use BlockGroupingCollectorManager.
	// For now, we'll return nil or implement a simplified version.
	return nil, nil
}

func (gs *GroupingSearch[T]) GetAllMatchingGroups() []T {
	return gs.matchingGroups
}

func (gs *GroupingSearch[T]) GetAllGroupHeads() []int {
	return gs.matchingGroupHeads
}
