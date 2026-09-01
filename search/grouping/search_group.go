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
