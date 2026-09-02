//go:build ignore

// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// DocValuesSkipIndexType defines options for skip indexes on doc values.
// Mirrors org.apache.lucene.index.DocValuesSkipIndexType from Apache Lucene 10.5.0.
type DocValuesSkipIndexType int

const (
	// DocValuesSkipIndexTypeNone: No skip index should be created.
	DocValuesSkipIndexTypeNone DocValuesSkipIndexType = iota
	// DocValuesSkipIndexTypeRange: Record range of values.
	DocValuesSkipIndexTypeRange
)

// IsCompatibleWith reports whether the skip index type is compatible with the given doc values type.
func (t DocValuesSkipIndexType) IsCompatibleWith(dvType DocValuesType) bool {
	switch t {
	case DocValuesSkipIndexTypeNone:
		return true
	case DocValuesSkipIndexTypeRange:
		return dvType == DocValuesTypeNumeric ||
			dvType == DocValuesTypeSortedNumeric ||
			dvType == DocValuesTypeSorted ||
			dvType == DocValuesTypeSortedSet
	default:
		return false
	}
}
