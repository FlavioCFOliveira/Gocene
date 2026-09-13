package search

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
)

type SortFieldType = spi.SortFieldType
type MissingValueStrategy = spi.MissingValueStrategy

var STRING_FIRST = spi.STRING_FIRST
var STRING_LAST = spi.STRING_LAST

// SortField is declared in sort_field.go, the file named for SortField.java.
// The five accessors that used to be re-declared here are already provided by
// spi.SortField itself, and Go does not allow a package to add methods to a
// type it does not define.

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
			{Type: spi.SortFieldTypeScore, Reverse: true},
		},
	}
}

// NewSortByDoc creates a sort by document ID (ascending).
func NewSortByDoc() *Sort {
	return &Sort{
		Fields: []*SortField{
			{Type: spi.SortFieldTypeDoc},
		},
	}
}

// NeedsScores returns true if any sort field needs scores.
func (s *Sort) NeedsScores() bool {
	for _, field := range s.Fields {
		if field.Type == spi.SortFieldTypeScore {
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
	// Mirrors LeafFieldComparator.setScorer(Scorable) of Apache Lucene 10.5.0.
	SetScorer(scorer Scorable) error
}

// Rewrite rewrites this Sort, returning a new Sort if any of the sort fields
// changed during their rewriting, or this Sort otherwise.
//
// Mirrors Sort.rewrite(IndexSearcher) of Apache Lucene 10.5.0.
func (s *Sort) Rewrite(searcher *IndexSearcher) (*Sort, error) {
	changed := false
	rewrittenSortFields := make([]*SortField, len(s.Fields))
	for i := 0; i < len(s.Fields); i++ {
		rewritten, err := RewriteSortField(s.Fields[i], searcher)
		if err != nil {
			return nil, err
		}
		rewrittenSortFields[i] = rewritten
		if s.Fields[i] != rewrittenSortFields[i] {
			changed = true
		}
	}

	if changed {
		return NewSort(rewrittenSortFields...), nil
	}
	return s, nil
}
