package search

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
)

type SortFieldType = spi.SortFieldType
type MissingValueStrategy = spi.MissingValueStrategy
var STRING_FIRST = spi.STRING_FIRST
var STRING_LAST = spi.STRING_LAST
type SortField = spi.SortField

func (sf *SortField) GetField() string { return sf.Field }
func (sf *SortField) GetReverse() bool { return sf.Reverse }
func (sf *SortField) SetMissingValue(v interface{}) { sf.MissingValue = v }
func (sf *SortField) SetOptimizeSortWithIndexedData(v bool) {
	sf.optimizeSortWithIndexedData = v
	sf.optimizeSet = true
}
func (sf *SortField) GetOptimizeSortWithIndexedData() bool {
	if !sf.optimizeSet {
		return true
	}
	return sf.optimizeSortWithIndexedData
}

// Sort defines the sort order for search results.
type Sort struct {
	Fields []*SortField
}

// NewSort creates a new Sort with the given fields.
func NewSort(fields ...*SortField) *Sort {
	return &Sort{Fields: fields}
}

// NewSortByScore creates a sort by relevance score (descending).
func NewSortByScore() *Sort {
	return &Sort{
		Fields: []*SortField{
			{Type: SortFieldTypeScore, Reverse: true},
		},
	}
}

// NewSortByDoc creates a sort by document ID (ascending).
func NewSortByDoc() *Sort {
	return &Sort{
		Fields: []*SortField{
			{Type: SortFieldTypeDoc},
		},
	}
}

// NeedsScores returns true if any sort field needs scores.
func (s *Sort) NeedsScores() bool {
	for _, field := range s.Fields {
		if field.Type == SortFieldTypeScore {
			return true
		}
	}
	return false
}

// FieldComparator compares two documents based on a sort field.
type FieldComparator interface {
	// Compare compares doc1 and doc2.
	Compare(doc1, doc2 int) int

	// SetBottom sets the bottom document for the priority queue.
	SetBottom(doc int)

	// CompareBottom compares the given doc with the bottom doc.
	CompareBottom(doc int) int

	// Copy copies the value from the given doc to the slot.
	Copy(slot int, doc int)

	// SetScorer sets the scorer.
	SetScorer(scorer Scorer)
}
