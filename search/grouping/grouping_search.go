// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"errors"
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/queries/function"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// GroupingSearch is a convenience class to perform grouping in a non
// distributed environment.
//
// Mirrors org.apache.lucene.search.grouping.GroupingSearch (Apache Lucene
// 10.5.0). Java's class is not generic: it holds a GroupSelector<?> and
// exposes the generic methods <T> search(...) and <T>
// getAllMatchingGroups(). Go has no generic methods, so those two are the
// free functions GroupingSearchSearch and GroupingSearchGetAllMatchingGroups,
// which take the group value type T that Java's call sites infer; the
// selector is held type-erased, exactly as GroupSelector<?> is.
//
// lucene.experimental
type GroupingSearch struct {
	// grouper is the GroupSelector<?>; nil when grouping by doc block.
	grouper      any
	groupEndDocs search.Query

	groupSort       *search.Sort
	sortWithinGroup *search.Sort

	groupDocsOffset int
	groupDocsLimit  int
	includeMaxScore bool

	// maxCacheRAMMB and maxDocsToCache render the nullable Double and
	// Integer fields.
	maxCacheRAMMB  *float64
	maxDocsToCache *int
	cacheScores    bool

	allGroups                   bool
	allGroupHeads               bool
	ignoreDocsWithoutGroupField bool

	// matchingGroups renders Collection<?>: a []T for the T of the last
	// search.
	matchingGroups     any
	matchingGroupHeads util.Bits
}

// NewGroupingSearch constructs a GroupingSearch instance that groups
// documents by index terms using DocValues. The group field can only have
// one token per document. This means that the field must not be analysed.
//
// Mirrors GroupingSearch(String groupField).
func NewGroupingSearch(groupField string) *GroupingSearch {
	return newGroupingSearch(NewTermGroupSelector(groupField), nil)
}

// NewGroupingSearchGroupSelector constructs a GroupingSearch instance that
// groups documents using a GroupSelector.
//
// Mirrors GroupingSearch(GroupSelector<?> groupSelector).
func NewGroupingSearchGroupSelector[T any](groupSelector GroupSelector[T]) *GroupingSearch {
	return newGroupingSearch(groupSelector, nil)
}

// NewGroupingSearchValueSource constructs a GroupingSearch instance that
// groups documents by function using a ValueSource instance; groupFunction is
// the function to group by and valueSourceContext its context.
//
// Mirrors GroupingSearch(ValueSource groupFunction, Map<Object, Object>
// valueSourceContext).
func NewGroupingSearchValueSource(groupFunction function.ValueSource, valueSourceContext function.Context) *GroupingSearch {
	return newGroupingSearch(NewValueSourceGroupSelector(groupFunction, valueSourceContext), nil)
}

// NewGroupingSearchQuery is the constructor for grouping documents by doc
// block. This constructor can only be used when documents belonging in a
// group are indexed in one block; groupEndDocs is the query that marks the
// last document in all doc blocks.
//
// Mirrors GroupingSearch(Query groupEndDocs).
func NewGroupingSearchQuery(groupEndDocs search.Query) *GroupingSearch {
	return newGroupingSearch(nil, groupEndDocs)
}

// newGroupingSearch mirrors the private GroupingSearch(GroupSelector<?>,
// Query) together with the field initialisers.
func newGroupingSearch(grouper any, groupEndDocs search.Query) *GroupingSearch {
	return &GroupingSearch{
		grouper:         grouper,
		groupEndDocs:    groupEndDocs,
		groupSort:       search.RELEVANCE,
		sortWithinGroup: search.RELEVANCE,
		groupDocsLimit:  1,
		includeMaxScore: true,
	}
}

// GroupingSearchSearch executes a grouped search. Both the first pass and
// second pass are executed on the specified searcher.
//
// searcher is the IndexSearcher to execute the grouped search on, query the
// query to execute with the grouping, groupOffset the group offset and
// groupLimit the number of groups to return from the specified group offset.
//
// Mirrors the generic method <T> TopGroups<T> search(IndexSearcher, Query,
// int, int).
func GroupingSearchSearch[T any](gs *GroupingSearch, searcher *search.IndexSearcher, query search.Query,
	groupOffset, groupLimit int) (*TopGroups[T], error) {
	if gs.grouper != nil {
		return groupByFieldOrFunction[T](gs, searcher, query, groupOffset, groupLimit)
	} else if gs.groupEndDocs != nil {
		return groupByDocBlock[T](gs, searcher, query, groupOffset, groupLimit)
	}
	// This can't happen...
	return nil, errors.New("Either groupField, groupFunction or groupEndDocs must be set.")
}

// groupByFieldOrFunction mirrors the protected groupByFieldOrFunction(
// IndexSearcher, Query, int, int).
func groupByFieldOrFunction[T any](gs *GroupingSearch, searcher *search.IndexSearcher, query search.Query,
	groupOffset, groupLimit int) (*TopGroups[T], error) {
	grouper, ok := gs.grouper.(GroupSelector[T])
	if !ok {
		return nil, fmt.Errorf("ClassCastException: %T is not a GroupSelector of %T", gs.grouper, *new(T))
	}

	topN := groupOffset + groupLimit
	firstPassCollector, err := NewFirstPassGroupingCollectorIgnoringDocsWithoutGroupField(
		grouper, gs.groupSort, topN, gs.ignoreDocsWithoutGroupField)
	if err != nil {
		return nil, err
	}
	var allGroupsCollector *AllGroupsCollector[T]
	if gs.allGroups {
		allGroupsCollector = NewAllGroupsCollector(grouper)
	}
	var allGroupHeadsCollector *AllGroupHeadsCollector[T]
	if gs.allGroupHeads {
		allGroupHeadsCollector = NewAllGroupHeadsCollector(grouper, gs.sortWithinGroup)
	}
	// MultiCollector.wrap drops the null collectors; typed nil pointers would
	// survive Go's nil check, so only the present ones are passed.
	collectors := []search.Collector{firstPassCollector}
	if allGroupsCollector != nil {
		collectors = append(collectors, allGroupsCollector)
	}
	if allGroupHeadsCollector != nil {
		collectors = append(collectors, allGroupHeadsCollector)
	}
	firstRound, err := search.MultiCollectorWrap(collectors...)
	if err != nil {
		return nil, err
	}

	var cachedCollector search.CachingCollector
	if gs.maxCacheRAMMB != nil || gs.maxDocsToCache != nil {
		if gs.maxCacheRAMMB != nil {
			cachedCollector = search.CreateCachingCollectorWithOther(firstRound, gs.cacheScores, *gs.maxCacheRAMMB)
		} else {
			cachedCollector = search.CreateCachingCollectorWithOtherInt(firstRound, gs.cacheScores, *gs.maxDocsToCache)
		}
		if err := searcher.SearchWithCollector(query, cachedCollector); err != nil {
			return nil, err
		}
	} else {
		if err := searcher.SearchWithCollector(query, firstRound); err != nil {
			return nil, err
		}
	}

	if gs.allGroups {
		gs.matchingGroups = allGroupsCollector.GetGroups()
	} else {
		gs.matchingGroups = []T{}
	}
	maxDoc := searcher.GetIndexReader().MaxDoc()
	if gs.allGroupHeads {
		heads, err := allGroupHeadsCollector.RetrieveGroupHeadsBitSet(maxDoc)
		if err != nil {
			return nil, err
		}
		gs.matchingGroupHeads = heads
	} else {
		gs.matchingGroupHeads = util.NewMatchNoBits(maxDoc)
	}

	topSearchGroups, err := firstPassCollector.GetTopGroups(groupOffset)
	if err != nil {
		return nil, err
	}
	if topSearchGroups == nil {
		return NewTopGroups([]*search.SortField{}, []*search.SortField{}, 0, 0, []*GroupDocs[T]{},
			float32(math.NaN())), nil
	}

	topNInsideGroup := gs.groupDocsOffset + gs.groupDocsLimit
	secondPassCollector, err := NewTopGroupsCollector(
		grouper, topSearchGroups, gs.groupSort, gs.sortWithinGroup, topNInsideGroup, gs.includeMaxScore)
	if err != nil {
		return nil, err
	}

	if cachedCollector != nil && cachedCollector.IsCached() {
		if err := cachedCollector.Replay(secondPassCollector); err != nil {
			return nil, err
		}
	} else {
		if err := searcher.SearchWithCollector(query, secondPassCollector); err != nil {
			return nil, err
		}
	}

	topGroups, err := secondPassCollector.GetTopGroups(gs.groupDocsOffset)
	if err != nil {
		return nil, err
	}
	if gs.allGroups {
		matchingGroupsSize := len(gs.matchingGroups.([]T))
		return NewTopGroupsWithTotalGroupCount(topGroups, &matchingGroupsSize), nil
	}
	return topGroups, nil
}

// groupByDocBlock mirrors the protected <T> groupByDocBlock(IndexSearcher,
// Query, int, int).
func groupByDocBlock[T any](gs *GroupingSearch, searcher *search.IndexSearcher, query search.Query,
	groupOffset, groupLimit int) (*TopGroups[T], error) {
	endDocsQuery, err := searcher.Rewrite(gs.groupEndDocs)
	if err != nil {
		return nil, err
	}
	groupEndDocs, err := searcher.CreateWeight(endDocsQuery, search.COMPLETE_NO_SCORES, 1)
	if err != nil {
		return nil, err
	}
	bcm, err := NewBlockGroupingCollectorManager[T](
		gs.groupSort,
		groupOffset,
		groupLimit,
		gs.groupSort.NeedsScores() || gs.sortWithinGroup.NeedsScores(),
		groupEndDocs,
		gs.sortWithinGroup,
		gs.groupDocsOffset,
		gs.groupDocsOffset+gs.groupDocsLimit)
	if err != nil {
		return nil, err
	}
	return search.SearchWithCollectorManager[*BlockGroupingCollector, *TopGroups[T]](searcher, query, bcm)
}

// SetCachingInMB enables caching for the second pass search. The cache will
// not grow over a specified limit in MB. The cache is filled during the first
// pass searched and then replayed during the second pass searched. If the
// cache grows beyond the specified limit, then the cache is purged and not
// used in the second pass search.
//
// Mirrors setCachingInMB(double, boolean).
func (gs *GroupingSearch) SetCachingInMB(maxCacheRAMMB float64, cacheScores bool) *GroupingSearch {
	gs.maxCacheRAMMB = &maxCacheRAMMB
	gs.maxDocsToCache = nil
	gs.cacheScores = cacheScores
	return gs
}

// SetCaching enables caching for the second pass search. The cache will not
// contain more than the maximum specified documents. The cache is filled
// during the first pass searched and then replayed during the second pass
// searched. If the cache grows beyond the specified limit, then the cache is
// purged and not used in the second pass search.
//
// Mirrors setCaching(int, boolean).
func (gs *GroupingSearch) SetCaching(maxDocsToCache int, cacheScores bool) *GroupingSearch {
	gs.maxDocsToCache = &maxDocsToCache
	gs.maxCacheRAMMB = nil
	gs.cacheScores = cacheScores
	return gs
}

// DisableCaching disables any enabled cache.
//
// Mirrors disableCaching().
func (gs *GroupingSearch) DisableCaching() *GroupingSearch {
	gs.maxCacheRAMMB = nil
	gs.maxDocsToCache = nil
	return gs
}

// SetGroupSort specifies how groups are sorted. Defaults to
// search.RELEVANCE.
func (gs *GroupingSearch) SetGroupSort(groupSort *search.Sort) *GroupingSearch {
	gs.groupSort = groupSort
	return gs
}

// SetSortWithinGroup specifies how documents inside a group are sorted.
// Defaults to search.RELEVANCE.
func (gs *GroupingSearch) SetSortWithinGroup(sortWithinGroup *search.Sort) *GroupingSearch {
	gs.sortWithinGroup = sortWithinGroup
	return gs
}

// SetGroupDocsOffset specifies the offset for documents inside a group.
func (gs *GroupingSearch) SetGroupDocsOffset(groupDocsOffset int) *GroupingSearch {
	gs.groupDocsOffset = groupDocsOffset
	return gs
}

// SetGroupDocsLimit specifies the number of documents to return inside a
// group from the specified groupDocsOffset.
func (gs *GroupingSearch) SetGroupDocsLimit(groupDocsLimit int) *GroupingSearch {
	gs.groupDocsLimit = groupDocsLimit
	return gs
}

// SetIncludeMaxScore says whether to include the score of the most relevant
// document per group.
func (gs *GroupingSearch) SetIncludeMaxScore(includeMaxScore bool) *GroupingSearch {
	gs.includeMaxScore = includeMaxScore
	return gs
}

// SetAllGroups says whether to also compute all groups matching the query.
// This can be used to determine the number of groups, which can be used for
// accurate pagination.
//
// When grouping by doc block the number of groups are automatically included
// in the TopGroups and this option doesn't have any influence.
func (gs *GroupingSearch) SetAllGroups(allGroups bool) *GroupingSearch {
	gs.allGroups = allGroups
	return gs
}

// GroupingSearchGetAllMatchingGroups returns all matching groups if
// SetAllGroups(true) was set, otherwise an empty collection. T is the group
// value type; if grouping by doc block the group value is always nil.
//
// Mirrors the generic method <T> Collection<T> getAllMatchingGroups(); the
// unchecked cast is a type assertion.
func GroupingSearchGetAllMatchingGroups[T any](gs *GroupingSearch) []T {
	if gs.matchingGroups == nil {
		return nil
	}
	return gs.matchingGroups.([]T)
}

// SetAllGroupHeads says whether to compute all group heads (most relevant
// document per group) matching the query.
//
// This feature isn't enabled when grouping by doc block.
func (gs *GroupingSearch) SetAllGroupHeads(allGroupHeads bool) *GroupingSearch {
	gs.allGroupHeads = allGroupHeads
	return gs
}

// GetAllGroupHeads returns the matching group heads if SetAllGroupHeads(true)
// was set, or an empty bit set.
func (gs *GroupingSearch) GetAllGroupHeads() util.Bits {
	return gs.matchingGroupHeads
}

// SetIgnoreDocsWithoutGroupField says whether to ignore documents that don't
// have the group field instead of putting them in a null group.
func (gs *GroupingSearch) SetIgnoreDocsWithoutGroupField(ignoreDocsWithoutGroupField bool) *GroupingSearch {
	gs.ignoreDocsWithoutGroupField = ignoreDocsWithoutGroupField
	return gs
}
