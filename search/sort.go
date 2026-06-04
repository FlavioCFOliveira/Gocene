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
	// SortFieldTypeCustom sorts using a custom FieldComparatorSource. Mirrors
	// Lucene's SortField.Type.CUSTOM.
	SortFieldTypeCustom
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

// stringSentinel is the opaque type used for the STRING_FIRST / STRING_LAST
// sentinels. Identity comparison (pointer equality) is the only valid test.
type stringSentinel struct{ name string }

// STRING_FIRST is the missing-value sentinel that sorts missing values first
// (before any present value). Mirrors SortField.STRING_FIRST.
var STRING_FIRST = &stringSentinel{"STRING_FIRST"}

// STRING_LAST is the missing-value sentinel that sorts missing values last
// (after any present value). Mirrors SortField.STRING_LAST.
var STRING_LAST = &stringSentinel{"STRING_LAST"}

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

	// numericDVSource, when non-nil, overrides the default per-leaf
	// NumericDocValues resolution used by the INT/LONG/FLOAT/DOUBLE comparators.
	// It is the Go counterpart of a SortField subclass that overrides
	// getNumericDocValues (e.g. ToParentBlockJoinSortField). Set via
	// SetNumericDocValuesSource; nil means the comparator reads the field's
	// NumericDocValues directly from the leaf.
	numericDVSource NumericDocValuesSource

	// sortedDVSource, when non-nil, overrides the default per-leaf
	// SortedDocValues resolution used by the STRING (TermOrdVal) comparator. It is
	// the Go counterpart of overriding getSortedDocValues. Set via
	// SetSortedDocValuesSource; nil means the comparator reads the field's
	// SortedDocValues directly from the leaf.
	sortedDVSource SortedDocValuesSource

	// optimizeSortWithIndexedData mirrors SortField.optimizeSortWithIndexedData
	// (Lucene 10.4.0). When true (the default) Lucene is permitted to use the
	// points / DocValues-skipper index to skip non-competitive hits and to
	// type-validate the sort against the indexed point type. Gocene records the
	// flag but does not yet act on it: the sort-optimization feature it gates
	// (competitive-hit skipping, GREATER_THAN_OR_EQUAL_TO totalHits, and the
	// point-type validation) is tracked by rmp #130. The accessor exists so the
	// public SortField API matches Lucene and callers can opt out in advance of
	// the optimization landing.
	optimizeSortWithIndexedData bool

	// optimizeSet records whether optimizeSortWithIndexedData was explicitly set,
	// so the zero value of a struct-literal SortField still defaults to true.
	optimizeSet bool

	// comparatorSource holds the custom FieldComparatorSource for a
	// SortFieldTypeCustom sort. It is the Go counterpart of the
	// FieldComparatorSource a Lucene SortField carries when constructed with
	// new SortField(field, FieldComparatorSource, reverse). nil for the built-in
	// types. Set via NewSortFieldCustom / SetComparatorSource.
	comparatorSource FieldComparatorSource
}

// NewSortFieldCustom creates a SortField (SortFieldTypeCustom) that orders
// documents using the given custom FieldComparatorSource, mirroring Lucene's
// SortField(String, FieldComparatorSource, boolean) constructor. The source's
// NewComparator is invoked once per search to build the comparator the
// TopFieldCollector drives.
func NewSortFieldCustom(field string, source FieldComparatorSource, reverse bool) *SortField {
	return &SortField{
		Field:                       field,
		Type:                        SortFieldTypeCustom,
		Reverse:                     reverse,
		Missing:                     MissingValueLast,
		optimizeSortWithIndexedData: true,
		optimizeSet:                 true,
		comparatorSource:            source,
	}
}

// GetComparatorSource returns the custom FieldComparatorSource for a CUSTOM sort
// field, or nil for the built-in types.
func (sf *SortField) GetComparatorSource() FieldComparatorSource { return sf.comparatorSource }

// NewSortField creates a new SortField.
func NewSortField(field string, fieldType SortFieldType) *SortField {
	return &SortField{
		Field:                       field,
		Type:                        fieldType,
		Reverse:                     false,
		Missing:                     MissingValueLast,
		optimizeSortWithIndexedData: true,
		optimizeSet:                 true,
	}
}

// NewSortFieldReverse creates a new SortField with reverse order.
func NewSortFieldReverse(field string, fieldType SortFieldType) *SortField {
	return &SortField{
		Field:                       field,
		Type:                        fieldType,
		Reverse:                     true,
		Missing:                     MissingValueLast,
		optimizeSortWithIndexedData: true,
		optimizeSet:                 true,
	}
}

// GetField returns the field name this SortField sorts on.
func (sf *SortField) GetField() string { return sf.Field }

// GetReverse returns whether this sort is reversed.
func (sf *SortField) GetReverse() bool {
	return sf.Reverse
}

// SetMissingValue sets the value used when a document has no value for this
// field during sorting. The sentinel values STRING_FIRST and STRING_LAST are
// supported for string-typed fields.
func (sf *SortField) SetMissingValue(v interface{}) {
	sf.MissingValue = v
}

// SetOptimizeSortWithIndexedData controls whether the sort may use the points /
// DocValues-skipper index to skip non-competitive hits (and to type-validate the
// sort against the indexed point type). It mirrors
// SortField.setOptimizeSortWithIndexedData (Lucene 10.4.0).
//
// Gocene records the flag but does not yet consult it during search: the
// optimization it gates is tracked by rmp #130. Passing false today is therefore
// a no-op on behaviour but keeps the public API faithful to Lucene.
func (sf *SortField) SetOptimizeSortWithIndexedData(v bool) {
	sf.optimizeSortWithIndexedData = v
	sf.optimizeSet = true
}

// GetOptimizeSortWithIndexedData reports whether the sort is allowed to use the
// indexed data for the optimization. It defaults to true (matching Lucene),
// including for struct-literal SortFields that never called the setter.
func (sf *SortField) GetOptimizeSortWithIndexedData() bool {
	if !sf.optimizeSet {
		return true
	}
	return sf.optimizeSortWithIndexedData
}

// SetNumericDocValuesSource installs a custom per-leaf NumericDocValues resolver
// for this SortField, overriding the default (read the field directly from the
// leaf). The INT/LONG/FLOAT/DOUBLE comparators consult it in setReader. This is
// the extension point used by ToParentBlockJoinSortField to feed
// BlockJoinSelector-wrapped child values to a numeric comparator, mirroring the
// getNumericDocValues override in Lucene's ToParentBlockJoinSortField.
func (sf *SortField) SetNumericDocValuesSource(src NumericDocValuesSource) {
	sf.numericDVSource = src
}

// SetSortedDocValuesSource installs a custom per-leaf SortedDocValues resolver
// for this SortField, overriding the default. The STRING comparator consults it
// in setReader. This is the extension point used by ToParentBlockJoinSortField
// for STRING-typed parent sorts, mirroring the getSortedDocValues override in
// Lucene's ToParentBlockJoinSortField.
func (sf *SortField) SetSortedDocValuesSource(src SortedDocValuesSource) {
	sf.sortedDVSource = src
}

// NumericDocValuesSource resolves the NumericDocValues a numeric field comparator
// should read for a given leaf reader and field. A custom source lets callers
// substitute a derived/wrapped iterator (e.g. block-join MIN/MAX selection over a
// parent's children) in place of the field's stored values.
//
// Mirrors the getNumericDocValues(LeafReaderContext, String) hook that Lucene's
// numeric LeafComparators expose for subclassing.
type NumericDocValuesSource interface {
	// NumericDocValues returns the iterator the comparator reads, or nil when the
	// leaf has no values (treated as every document missing). The reader is the
	// leaf reader the comparator was just bound to.
	NumericDocValues(reader IndexReader, field string) (NumericDocValuesIterator, error)
}

// SortedDocValuesSource resolves the SortedDocValues the STRING comparator should
// read for a given leaf reader and field. Mirrors the getSortedDocValues hook in
// Lucene's TermOrdValComparator.
type SortedDocValuesSource interface {
	// SortedDocValues returns the iterator the comparator reads, or nil when the
	// leaf has no values (treated as every document missing).
	SortedDocValues(reader IndexReader, field string) (SortedDocValuesIterator, error)
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
