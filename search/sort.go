// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// SortFieldType represents the type of a sort field.
type SortFieldType int

const (
	// SortFieldTypeScore sorts by relevance score.
	SortFieldTypeScore SortFieldType = iota
	// SortFieldTypeDoc sorts by document ID.
	SortFieldTypeDoc
	// SortFieldTypeString sorts by string value.
	SortFieldTypeString
	// SortFieldTypeInt sorts by integer value.
	SortFieldTypeInt
	// SortFieldTypeLong sorts by long value.
	SortFieldTypeLong
	// SortFieldTypeFloat sorts by float value.
	SortFieldTypeFloat
	// SortFieldTypeDouble sorts by double value.
	SortFieldTypeDouble
)

// MissingValueStrategy defines how to handle missing values during sorting.
type MissingValueStrategy int

const (
	// MissingValueLast places documents with missing values at the end.
	MissingValueLast MissingValueStrategy = iota
	// MissingValueFirst places documents with missing values at the beginning.
	MissingValueFirst
	// MissingValueUseDefault uses a default value for missing documents.
	MissingValueUseDefault
)

// SortField defines how to sort documents by a specific field.
// This is the Go port of Lucene's org.apache.lucene.search.SortField.
type SortField struct {
	// Field is the name of the field to sort by.
	// Empty string for score sorting.
	Field string

	// Type is the type of the sort field.
	Type SortFieldType

	// Reverse sorts in descending order if true.
	Reverse bool

	// Missing is the strategy for handling missing values.
	Missing MissingValueStrategy

	// MissingValue is the value to use for missing documents (for numeric types).
	MissingValue interface{}
}

// NewSortField creates a new SortField.
func NewSortField(field string, fieldType SortFieldType) *SortField {
	return &SortField{
		Field:   field,
		Type:    fieldType,
		Reverse: false,
		Missing: MissingValueLast,
	}
}

// NewSortFieldReverse creates a new SortField with reverse order.
func NewSortFieldReverse(field string, fieldType SortFieldType) *SortField {
	return &SortField{
		Field:   field,
		Type:    fieldType,
		Reverse: true,
		Missing: MissingValueLast,
	}
}

// GetReverse returns whether this sort is reversed.
func (sf *SortField) GetReverse() bool {
	return sf.Reverse
}

// Sort defines the sort order for search results.
// This is the Go port of Lucene's org.apache.lucene.search.Sort.
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
	// Returns -1 if doc1 < doc2, 0 if equal, 1 if doc1 > doc2.
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

// FieldComparatorSource creates FieldComparators for sorting.
type FieldComparatorSource interface {
	// NewComparator creates a new comparator for the given sort field.
	NewComparator(field *SortField, numHits int) FieldComparator
}
