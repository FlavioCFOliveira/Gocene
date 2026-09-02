package schema

import "fmt"

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
	// SortFieldTypeCustom sorts using a custom FieldComparatorSource.
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

// STRING_FIRST is the missing-value sentinel that sorts missing values first.
var STRING_FIRST = &stringSentinel{"STRING_FIRST"}

// STRING_LAST is the missing-value sentinel that sorts missing values last.
var STRING_LAST = &stringSentinel{"STRING_LAST"}

// SortField defines how to sort documents by a specific field.
type SortField struct {
	// Field is the name of the field to sort by.
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
	numericDVSource any // Use any to avoid importing index.NumericDocValuesSource

	// sortedDVSource, when non-nil, overrides the default per-leaf
	// SortedDocValues resolution used by the STRING (TermOrdVal) comparator.
	sortedDVSource any // Use any to avoid importing index.SortedDocValuesSource

	// optimizeSortWithIndexedData mirrors SortField.optimizeSortWithIndexedData
	optimizeSortWithIndexedData bool

	// optimizeSet records whether optimizeSortWithIndexedData was explicitly set.
	optimizeSet bool

	// comparatorSource holds the custom FieldComparatorSource for a
	// SortFieldTypeCustom sort.
	comparatorSource any // Use any to avoid importing search.FieldComparatorSource
}

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
