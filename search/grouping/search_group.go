package grouping

import "fmt"

// SearchGroup represents a group that is found during the first pass search.
type SearchGroup[T any] struct {
	GroupValue T
	SortValues []any
}

func (sg SearchGroup[T]) String() string {
	return fmt.Sprintf("SearchGroup(groupValue=%v sortValues=%v)", sg.GroupValue, sg.SortValues)
}

// CollectedSearchGroup is a representation of a group in FirstPassGroupingCollector,
// tracking the top doc and comparator slot.
type CollectedSearchGroup[T any] struct {
	SearchGroup[T]
	TopDoc         int
	ComparatorSlot int
}
