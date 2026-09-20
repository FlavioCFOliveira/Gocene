package grouping

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// GroupingSearch is a convenience class to perform grouping in a non distributed environment.
type GroupingSearch[T any] struct {
	grouper                     GroupSelector[T]
	groupEndDocs                search.Query
	groupSort                   *search.Sort
	sortWithinGroup             *search.Sort
	groupDocsOffset             int
	groupDocsLimit              int
	includeMaxScore             bool
	ignoreDocsWithoutGroupField bool
	allGroups                   bool
	allGroupHeads               bool
	matchingGroups              []T
	matchingGroupHeads          []int
}

func NewGroupingSearch[T any](grouper GroupSelector[T]) *GroupingSearch[T] {
	return &GroupingSearch[T]{
		grouper:         grouper,
		groupSort:       search.RELEVANCE,
		sortWithinGroup: search.RELEVANCE,
		groupDocsLimit:  1,
		includeMaxScore: true,
	}
}

func NewGroupingSearchByDocBlock[T any](groupEndDocs search.Query) *GroupingSearch[T] {
	return &GroupingSearch[T]{
		groupEndDocs:    groupEndDocs,
		groupSort:       search.RELEVANCE,
		sortWithinGroup: search.RELEVANCE,
		groupDocsLimit:  1,
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
	if err := searcher.SearchWithCollector(query, firstRound); err != nil {
		return nil, err
	}

	topSearchGroups, err := firstPassCollector.GetTopGroups(groupOffset)
	if err != nil {
		return nil, err
	}
	if topSearchGroups == nil {
		// Mirrors the empty result Java returns here:
		// new TopGroups(new SortField[0], new SortField[0], 0, 0, new GroupDocs[0], Float.NaN).
		return NewTopGroups([]*search.SortField{}, []*search.SortField{}, 0, 0, []*GroupDocs[T]{}, float32(math.NaN())), nil
	}

	topNInsideGroup := gs.groupDocsOffset + gs.groupDocsLimit
	secondPassCollector := NewTopGroupsCollector(
		gs.grouper, topSearchGroups, gs.groupSort, gs.sortWithinGroup, topNInsideGroup, gs.includeMaxScore,
	)

	if err := searcher.SearchWithCollector(query, secondPassCollector); err != nil {
		return nil, err
	}

	return secondPassCollector.GetTopGroups(gs.groupDocsOffset), nil
}

// groupByDocBlock groups documents that were indexed as blocks, using the
// groupEndDocs query to mark the last document of each block.
//
// Mirrors GroupingSearch.groupByDocBlock(IndexSearcher, Query, int, int).
// Java's last statement is searcher.search(query, bcm), the CollectorManager
// entry point; Gocene's IndexSearcher does not declare it, so the manager is
// driven directly — newCollector, search, reduce — which is exactly what that
// entry point does for a single slice, and Gocene's search is sequential.
func (gs *GroupingSearch[T]) groupByDocBlock(searcher *search.IndexSearcher, query search.Query, groupOffset, groupLimit int) (*TopGroups[T], error) {
	endDocsQuery, err := searcher.Rewrite(gs.groupEndDocs)
	if err != nil {
		return nil, err
	}
	groupEndDocs, err := searcher.CreateWeight(endDocsQuery, search.COMPLETE_NO_SCORES, 1)
	if err != nil {
		return nil, err
	}
	bcm := NewBlockGroupingCollectorManager[T](
		gs.groupSort,
		groupOffset,
		groupLimit,
		gs.groupSort.NeedsScores() || gs.sortWithinGroup.NeedsScores(),
		groupEndDocs,
		gs.sortWithinGroup,
		gs.groupDocsOffset,
		gs.groupDocsOffset+gs.groupDocsLimit,
	)

	collector, err := bcm.NewCollector()
	if err != nil {
		return nil, err
	}
	if err := searcher.SearchWithCollector(query, collector); err != nil {
		return nil, err
	}
	return bcm.Reduce([]search.Collector{collector})
}

func (gs *GroupingSearch[T]) GetAllMatchingGroups() []T {
	return gs.matchingGroups
}

func (gs *GroupingSearch[T]) GetAllGroupHeads() []int {
	return gs.matchingGroupHeads
}
