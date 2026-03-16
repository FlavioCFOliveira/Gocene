package grouping

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// TopGroups represents the result of a grouped search.
// It contains the top groups and their top documents.
//
// This is the Go port of Lucene's org.apache.lucene.search.grouping.TopGroups.
type TopGroups struct {
	// GroupSort is the sort used for groups
	GroupSort search.Sort

	// DocSort is the sort used for documents within groups
	DocSort search.Sort

	// TotalGroupCount is the total number of groups (may be -1 if unknown)
	TotalGroupCount int

	// TotalHitCount is the total number of hits across all groups
	TotalHitCount int

	// Groups contains the top groups
	Groups []*GroupDocs

	// groupOffset is the offset for groups
	groupOffset int

	// groupLimit is the maximum number of groups
	groupLimit int
}

// NewTopGroups creates a new TopGroups.
func NewTopGroups(groupSort search.Sort, docSort search.Sort, groupOffset int, groupLimit int) *TopGroups {
	return &TopGroups{
		GroupSort:       groupSort,
		DocSort:         docSort,
		Groups:          make([]*GroupDocs, 0),
		TotalGroupCount: -1, // Unknown by default
		groupOffset:     groupOffset,
		groupLimit:      groupLimit,
	}
}

// AddGroup adds a GroupDocs to this TopGroups.
func (tg *TopGroups) AddGroup(group *GroupDocs) {
	tg.Groups = append(tg.Groups, group)
}

// GetGroup returns the GroupDocs at the given index.
func (tg *TopGroups) GetGroup(index int) *GroupDocs {
	if index < 0 || index >= len(tg.Groups) {
		return nil
	}
	return tg.Groups[index]
}

// GetGroupCount returns the number of groups.
func (tg *TopGroups) GetGroupCount() int {
	return len(tg.Groups)
}

// GetTotalGroupCount returns the total number of groups.
// Returns -1 if the total is unknown.
func (tg *TopGroups) GetTotalGroupCount() int {
	return tg.TotalGroupCount
}

// SetTotalGroupCount sets the total number of groups.
func (tg *TopGroups) SetTotalGroupCount(count int) {
	tg.TotalGroupCount = count
}

// GetTotalHitCount returns the total number of hits across all groups.
func (tg *TopGroups) GetTotalHitCount() int {
	return tg.TotalHitCount
}

// SetTotalHitCount sets the total number of hits.
func (tg *TopGroups) SetTotalHitCount(count int) {
	tg.TotalHitCount = count
}

// GetGroupOffset returns the group offset.
func (tg *TopGroups) GetGroupOffset() int {
	return tg.groupOffset
}

// GetGroupLimit returns the group limit.
func (tg *TopGroups) GetGroupLimit() int {
	return tg.groupLimit
}

// IsTotalGroupCountExact returns true if the total group count is exact.
func (tg *TopGroups) IsTotalGroupCountExact() bool {
	return tg.TotalGroupCount != -1
}

// Merge merges another TopGroups into this one.
// This is useful for combining results from multiple shards.
func (tg *TopGroups) Merge(other *TopGroups) error {
	if other == nil {
		return nil
	}

	// Check that sorts are compatible
	if !tg.areSortsCompatible(other) {
		return fmt.Errorf("cannot merge TopGroups with incompatible sorts")
	}

	// Merge groups
	for _, group := range other.Groups {
		tg.AddGroup(group)
	}

	// Update counts
	if other.TotalGroupCount > 0 {
		if tg.TotalGroupCount < 0 {
			tg.TotalGroupCount = other.TotalGroupCount
		} else {
			tg.TotalGroupCount += other.TotalGroupCount
		}
	}
	tg.TotalHitCount += other.TotalHitCount

	return nil
}

// areSortsCompatible checks if two TopGroups have compatible sorts for merging.
func (tg *TopGroups) areSortsCompatible(other *TopGroups) bool {
	// In a full implementation, this would compare the actual sort fields
	// For now, assume they are compatible
	return true
}

// Copy copies the content from another TopGroups.
func (tg *TopGroups) Copy(other *TopGroups) {
	if other == nil {
		return
	}

	tg.GroupSort = other.GroupSort
	tg.DocSort = other.DocSort
	tg.TotalGroupCount = other.TotalGroupCount
	tg.TotalHitCount = other.TotalHitCount
	tg.groupOffset = other.groupOffset
	tg.groupLimit = other.groupLimit

	// Copy groups
	tg.Groups = make([]*GroupDocs, len(other.Groups))
	for i, group := range other.Groups {
		tg.Groups[i] = group
	}
}

// String returns a string representation of this TopGroups.
func (tg *TopGroups) String() string {
	return fmt.Sprintf("TopGroups(groups=%d, totalGroups=%d, totalHits=%d)",
		len(tg.Groups), tg.TotalGroupCount, tg.TotalHitCount)
}

// TopGroupsMerger merges multiple TopGroups results.
type TopGroupsMerger struct {
	// groupSort is the sort for groups
	groupSort search.Sort

	// docSort is the sort for documents
	docSort search.Sort

	// groupOffset is the offset for groups
	groupOffset int

	// groupLimit is the limit for groups
	groupLimit int
}

// NewTopGroupsMerger creates a new TopGroupsMerger.
func NewTopGroupsMerger(groupSort search.Sort, docSort search.Sort, groupOffset int, groupLimit int) *TopGroupsMerger {
	return &TopGroupsMerger{
		groupSort:   groupSort,
		docSort:     docSort,
		groupOffset: groupOffset,
		groupLimit:  groupLimit,
	}
}

// Merge merges multiple TopGroups into one.
func (tgm *TopGroupsMerger) Merge(topGroupsList []*TopGroups) (*TopGroups, error) {
	if len(topGroupsList) == 0 {
		return NewTopGroups(tgm.groupSort, tgm.docSort, tgm.groupOffset, tgm.groupLimit), nil
	}

	// Start with the first result
	result := NewTopGroups(tgm.groupSort, tgm.docSort, tgm.groupOffset, tgm.groupLimit)
	result.Copy(topGroupsList[0])

	// Merge remaining results
	for i := 1; i < len(topGroupsList); i++ {
		if err := result.Merge(topGroupsList[i]); err != nil {
			return nil, err
		}
	}

	return result, nil
}

// GroupingCollector collects documents and groups them.
type GroupingCollector struct {
	// groupField is the field to group by
	groupField string

	// groups maps group values to GroupDocs
	groups map[string]*GroupDocs

	// docSort is the sort for documents within groups
	docSort search.Sort

	// docLimit is the maximum number of documents per group
	docLimit int
}

// NewGroupingCollector creates a new GroupingCollector.
func NewGroupingCollector(groupField string, docSort search.Sort, docLimit int) *GroupingCollector {
	return &GroupingCollector{
		groupField: groupField,
		groups:     make(map[string]*GroupDocs),
		docSort:    docSort,
		docLimit:   docLimit,
	}
}

// GetGroup returns the GroupDocs for the given group value.
func (gc *GroupingCollector) GetGroup(groupValue string) *GroupDocs {
	return gc.groups[groupValue]
}

// GetGroupCount returns the number of groups.
func (gc *GroupingCollector) GetGroupCount() int {
	return len(gc.groups)
}

// GetAllGroups returns all groups.
func (gc *GroupingCollector) GetAllGroups() []*GroupDocs {
	groups := make([]*GroupDocs, 0, len(gc.groups))
	for _, group := range gc.groups {
		groups = append(groups, group)
	}
	return groups
}
