package grouping

import (
	"context"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// GroupingSearch performs searches with result grouping.
// This is the Go port of Lucene's org.apache.lucene.search.grouping.GroupingSearch.
type GroupingSearch struct {
	// groupField is the field to group by
	groupField string

	// groupSort is the sort for groups
	groupSort search.Sort

	// docSort is the sort for documents within groups
	docSort search.Sort

	// groupOffset is the offset for groups
	groupOffset int

	// groupLimit is the maximum number of groups to return
	groupLimit int

	// docOffset is the offset for documents within groups
	docOffset int

	// docLimit is the maximum number of documents per group
	docLimit int

	// fillSortFields indicates whether to fill sort field values
	fillSortFields bool

	// includeMaxScore indicates whether to include max score per group
	includeMaxScore bool
}

// NewGroupingSearch creates a new GroupingSearch for the given field.
func NewGroupingSearch(groupField string) *GroupingSearch {
	return &GroupingSearch{
		groupField:      groupField,
		groupSort:       *search.NewSortByScore(),
		docSort:         *search.NewSortByScore(),
		groupOffset:     0,
		groupLimit:      10,
		docOffset:       0,
		docLimit:        1,
		fillSortFields:  false,
		includeMaxScore: false,
	}
}

// SetGroupSort sets the sort for groups.
func (gs *GroupingSearch) SetGroupSort(sort search.Sort) *GroupingSearch {
	gs.groupSort = sort
	return gs
}

// SetGroupSortByField sets the sort for groups by field.
func (gs *GroupingSearch) SetGroupSortByField(field string, reverse bool) *GroupingSearch {
	sortField := search.NewSortField(field, search.SortFieldTypeString)
	if reverse {
		sortField.Reverse = true
	}
	gs.groupSort = *search.NewSort(sortField)
	return gs
}

// SetDocSort sets the sort for documents within groups.
func (gs *GroupingSearch) SetDocSort(sort search.Sort) *GroupingSearch {
	gs.docSort = sort
	return gs
}

// SetDocSortByField sets the sort for documents within groups by field.
func (gs *GroupingSearch) SetDocSortByField(field string, reverse bool) *GroupingSearch {
	sortField := search.NewSortField(field, search.SortFieldTypeString)
	if reverse {
		sortField.Reverse = true
	}
	gs.docSort = *search.NewSort(sortField)
	return gs
}

// SetGroupOffset sets the offset for groups.
func (gs *GroupingSearch) SetGroupOffset(offset int) *GroupingSearch {
	gs.groupOffset = offset
	return gs
}

// SetGroupLimit sets the maximum number of groups to return.
func (gs *GroupingSearch) SetGroupLimit(limit int) *GroupingSearch {
	gs.groupLimit = limit
	return gs
}

// SetDocOffset sets the offset for documents within groups.
func (gs *GroupingSearch) SetDocOffset(offset int) *GroupingSearch {
	gs.docOffset = offset
	return gs
}

// SetDocLimit sets the maximum number of documents per group.
func (gs *GroupingSearch) SetDocLimit(limit int) *GroupingSearch {
	gs.docLimit = limit
	return gs
}

// SetFillSortFields sets whether to fill sort field values.
func (gs *GroupingSearch) SetFillSortFields(fill bool) *GroupingSearch {
	gs.fillSortFields = fill
	return gs
}

// SetIncludeMaxScore sets whether to include max score per group.
func (gs *GroupingSearch) SetIncludeMaxScore(include bool) *GroupingSearch {
	gs.includeMaxScore = include
	return gs
}

// Search performs a search with grouping.
func (gs *GroupingSearch) Search(ctx context.Context, searcher *search.IndexSearcher, query search.Query) (*TopGroups, error) {
	// In a full implementation, this would:
	// 1. Execute the query
	// 2. Group results by the group field
	// 3. Sort groups according to groupSort
	// 4. For each group, sort documents according to docSort
	// 5. Return TopGroups with the results

	// For now, return an empty result
	return NewTopGroups(gs.groupSort, gs.docSort, gs.groupOffset, gs.groupLimit), nil
}

// SearchWithCollector performs a search with grouping using a custom collector.
func (gs *GroupingSearch) SearchWithCollector(ctx context.Context, searcher *search.IndexSearcher, query search.Query, collector search.Collector) (*TopGroups, error) {
	// In a full implementation, this would use the custom collector
	// while still performing grouping
	return gs.Search(ctx, searcher, query)
}

// GetGroupField returns the group field.
func (gs *GroupingSearch) GetGroupField() string {
	return gs.groupField
}

// GetGroupSort returns the group sort.
func (gs *GroupingSearch) GetGroupSort() search.Sort {
	return gs.groupSort
}

// GetDocSort returns the document sort.
func (gs *GroupingSearch) GetDocSort() search.Sort {
	return gs.docSort
}

// GetGroupOffset returns the group offset.
func (gs *GroupingSearch) GetGroupOffset() int {
	return gs.groupOffset
}

// GetGroupLimit returns the group limit.
func (gs *GroupingSearch) GetGroupLimit() int {
	return gs.groupLimit
}

// GetDocOffset returns the document offset.
func (gs *GroupingSearch) GetDocOffset() int {
	return gs.docOffset
}

// GetDocLimit returns the document limit.
func (gs *GroupingSearch) GetDocLimit() int {
	return gs.docLimit
}

// String returns a string representation of this GroupingSearch.
func (gs *GroupingSearch) String() string {
	return fmt.Sprintf("GroupingSearch(field=%s, groupLimit=%d, docLimit=%d)",
		gs.groupField, gs.groupLimit, gs.docLimit)
}

// GroupDocs represents documents within a group.
type GroupDocs struct {
	// GroupValue is the value that defines this group
	GroupValue interface{}

	// Score is the score of this group (based on groupSort)
	Score float32

	// TotalHits is the total number of hits in this group
	TotalHits int

	// ScoreDocs contains the top documents in this group
	ScoreDocs []*search.ScoreDoc

	// MaxScore is the maximum score in this group (if includeMaxScore is true)
	MaxScore float32
}

// NewGroupDocs creates a new GroupDocs.
func NewGroupDocs(groupValue interface{}, score float32) *GroupDocs {
	return &GroupDocs{
		GroupValue: groupValue,
		Score:      score,
		ScoreDocs:  make([]*search.ScoreDoc, 0),
	}
}

// AddScoreDoc adds a ScoreDoc to this group.
func (gd *GroupDocs) AddScoreDoc(doc *search.ScoreDoc) {
	gd.ScoreDocs = append(gd.ScoreDocs, doc)
}

// GetScoreDoc returns the ScoreDoc at the given index.
func (gd *GroupDocs) GetScoreDoc(index int) *search.ScoreDoc {
	if index < 0 || index >= len(gd.ScoreDocs) {
		return nil
	}
	return gd.ScoreDocs[index]
}

// GetScoreDocCount returns the number of ScoreDocs in this group.
func (gd *GroupDocs) GetScoreDocCount() int {
	return len(gd.ScoreDocs)
}
