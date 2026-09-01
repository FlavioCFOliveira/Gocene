package grouping

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// GroupDocs represents one group in the results.
type GroupDocs[T any] struct {
	Score         float32
	MaxScore      float32
	TotalHits     search.TotalHits
	ScoreDocs     []search.ScoreDoc
	GroupValue    T
	GroupSortValues []any
}
