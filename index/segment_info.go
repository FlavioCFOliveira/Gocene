// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// This file is the index-side facade for the SegmentInfo type family
// after the SPI unification (rmp #4669 / Sprint 117 phase 1). The
// canonical declaration site lives in schema/; index/ re-exports the
// types as Go aliases so callers that historically reached for
// index.SegmentInfo, index.NewSegmentInfo, index.Sort, etc. keep
// compiling without churn.
//
// Aliasing a struct with `type X = schema.X` makes the index-package
// identifier indistinguishable from its schema counterpart at the type
// system level: methods declared on *schema.SegmentInfo are visible via
// *index.SegmentInfo, and instances are interchangeable across package
// boundaries. Constants and free functions cannot be aliased; they are
// re-exported below as wrappers or var-redeclarations.

// SegmentInfo is an alias of schema.SegmentInfo.
type SegmentInfo = schema.SegmentInfo

// Sort is an alias of schema.Sort.
type Sort = schema.Sort

// SortField is an alias of schema.SortField.
type SortField = schema.SortField

// SortType is an alias of schema.SortType.
type SortType = schema.SortType

// SortedNumericSortField is an alias of schema.SortedNumericSortField.
type SortedNumericSortField = schema.SortedNumericSortField

// SortedSetSortField is an alias of schema.SortedSetSortField.
type SortedSetSortField = schema.SortedSetSortField

// SegmentInfoList is an alias of schema.SegmentInfoList.
type SegmentInfoList = schema.SegmentInfoList

// SortType constants — Go does not allow aliasing typed constants, so
// they are re-declared as values of the aliased type.
const (
	SortTypeString = schema.SortTypeString
	SortTypeLong   = schema.SortTypeLong
	SortTypeInt    = schema.SortTypeInt
	SortTypeFloat  = schema.SortTypeFloat
	SortTypeDouble = schema.SortTypeDouble
)

// SortRELEVANCE re-exports schema.SortRELEVANCE.
var SortRELEVANCE = schema.SortRELEVANCE

// NewSegmentInfo re-exports schema.NewSegmentInfo.
func NewSegmentInfo(name string, docCount int, dir store.Directory) *SegmentInfo {
	return schema.NewSegmentInfo(name, docCount, dir)
}

// NewSort re-exports schema.NewSort.
func NewSort(fields ...SortField) *Sort {
	return schema.NewSort(fields...)
}

// NewSortField re-exports schema.NewSortField.
func NewSortField(name string, sortType SortType) SortField {
	return schema.NewSortField(name, sortType)
}

// NewSortFieldFull re-exports schema.NewSortFieldFull.
func NewSortFieldFull(name string, sortType SortType, descending bool) SortField {
	return schema.NewSortFieldFull(name, sortType, descending)
}

// NewSortFromFields re-exports schema.NewSortFromFields.
func NewSortFromFields(fields []SortField) *Sort {
	return schema.NewSortFromFields(fields)
}

// NewSortedNumericSortField re-exports schema.NewSortedNumericSortField.
func NewSortedNumericSortField(name string, sortType SortType) *SortedNumericSortField {
	return schema.NewSortedNumericSortField(name, sortType)
}

// NewSortedSetSortField re-exports schema.NewSortedSetSortField.
func NewSortedSetSortField(name string, reverse bool) *SortedSetSortField {
	return schema.NewSortedSetSortField(name, reverse)
}
