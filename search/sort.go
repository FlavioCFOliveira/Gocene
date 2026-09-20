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

var (
	// RELEVANCE represents sorting by computed relevance. Using this sort
	// criteria returns the same results as calling IndexSearcher.Search
	// without a sort criteria, only with slightly more overhead.
	//
	// Mirrors the static field Sort.RELEVANCE, which Java builds with the
	// no-argument constructor, whose body is this(SortField.FIELD_SCORE).
	RELEVANCE = NewSort(FieldScore)

	// INDEXORDER represents sorting by index order.
	//
	// Mirrors the static field Sort.INDEXORDER, built as
	// new Sort(SortField.FIELD_DOC).
	INDEXORDER = NewSort(FIELD_DOC)
)

// GetSort returns the representation of the sort criteria: the SortField
// values used in this sort criteria.
//
// Mirrors Sort.getSort().
func (s *Sort) GetSort() []*SortField { return s.Fields }

// Equals reports whether o is equal to this Sort, which holds when both carry
// the same sort fields in the same order.
//
// Mirrors Sort.equals(Object), whose body is Arrays.equals(this.fields,
// other.fields) — an element-wise SortField.equals comparison.
func (s *Sort) Equals(o *Sort) bool {
	if s == o {
		return true
	}
	if s == nil || o == nil {
		return false
	}
	if len(s.Fields) != len(o.Fields) {
		return false
	}
	for i := range s.Fields {
		a, b := s.Fields[i], o.Fields[i]
		if a == b {
			continue
		}
		if a == nil || b == nil {
			return false
		}
		if !a.Equals(b) {
			return false
		}
	}
	return true
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

// FieldComparator is declared in field_comparator.go, the file named for
// FieldComparator.java, and LeafFieldComparator in leaf_field_comparator.go.
// Sort.java declares neither.

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
