// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "fmt"

// DocValuesType specifies the type of per-document values stored in the index.
// DocValues are stored in a columnar format for efficient sorting, faceting,
// and value retrieval.
type DocValuesType int

const (
	// DocValuesTypeNone means no doc values are stored for this field.
	DocValuesTypeNone DocValuesType = iota

	// DocValuesTypeNumeric stores a single numeric value per document.
	// The value can be a signed 64-bit integer.
	DocValuesTypeNumeric

	// DocValuesTypeBinary stores a variable-length binary value per document.
	DocValuesTypeBinary

	// DocValuesTypeSorted stores a sorted set of bytes per document.
	// Values are deduplicated and sorted.
	DocValuesTypeSorted

	// DocValuesTypeSortedSet stores multiple sorted values per document.
	// This is useful for multi-valued fields.
	DocValuesTypeSortedSet

	// DocValuesTypeSortedNumeric stores multiple sorted numeric values per document.
	// This is useful for multi-valued numeric fields.
	DocValuesTypeSortedNumeric
)

// String returns the string representation of the DocValuesType.
func (dvt DocValuesType) String() string {
	switch dvt {
	case DocValuesTypeNone:
		return "NONE"
	case DocValuesTypeNumeric:
		return "NUMERIC"
	case DocValuesTypeBinary:
		return "BINARY"
	case DocValuesTypeSorted:
		return "SORTED"
	case DocValuesTypeSortedSet:
		return "SORTED_SET"
	case DocValuesTypeSortedNumeric:
		return "SORTED_NUMERIC"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", dvt)
	}
}

// HasDocValues returns true if the field has doc values.
func (dvt DocValuesType) HasDocValues() bool {
	return dvt != DocValuesTypeNone
}

// IsSorted returns true if the doc values type is sorted.
func (dvt DocValuesType) IsSorted() bool {
	return dvt == DocValuesTypeSorted || dvt == DocValuesTypeSortedSet || dvt == DocValuesTypeSortedNumeric
}

// IsMultiValued returns true if the doc values type supports multiple values per document.
func (dvt DocValuesType) IsMultiValued() bool {
	return dvt == DocValuesTypeSortedSet || dvt == DocValuesTypeSortedNumeric
}
