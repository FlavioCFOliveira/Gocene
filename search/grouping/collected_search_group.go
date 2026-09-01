package grouping

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// CollectedSearchGroup represents a group in FirstPassGroupingCollector.
type CollectedSearchGroup[T any] struct {
	SearchGroup[T]
	TopDoc          int
	ComparatorSlot  int
}
