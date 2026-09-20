// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "github.com/FlavioCFOliveira/Gocene/spi"

// This file re-exports the sort vocabulary that Apache Lucene 10.5.0 declares
// in org.apache.lucene.search (Sort, SortField, SortField.Type,
// SortedNumericSortField, SortedSetSortField) under the names the index
// package uses.
//
// PORT NOTE: in Java, org.apache.lucene.index freely imports
// org.apache.lucene.search because the two live in the same artefact. In
// Gocene, search depends on index (transitively, through document), so the
// canonical declarations live in package schema — the shared vocabulary layer
// that sits below both — and index and search each re-export them. There is
// exactly one SortField type in the module; these are type aliases, not
// parallel definitions, so a *Sort built by search is the same value an index
// sort consumes.

// Sort is the Go port of org.apache.lucene.search.Sort: an ordered list of
// SortField values defining a total order over documents.
type Sort = spi.Sort

// SortField is the Go port of org.apache.lucene.search.SortField: one sort
// dimension, naming the field, the value type and the direction.
type SortField = spi.SortField

// SortType is the Go port of the org.apache.lucene.search.SortField.Type
// enum, naming the value type a SortField sorts on.
type SortType = spi.SortFieldType

const (
	// SortTypeScore sorts by document score (relevance). Sort values are
	// floats and higher values sort first.
	SortTypeScore = spi.SortFieldTypeScore

	// SortTypeDoc sorts by document number (index order). Lower values sort
	// first.
	SortTypeDoc = spi.SortFieldTypeDoc

	// SortTypeString sorts using term values as strings. Lower values sort
	// first.
	SortTypeString = spi.SortFieldTypeString

	// SortTypeInt sorts using term values as encoded 32-bit integers.
	SortTypeInt = spi.SortFieldTypeInt

	// SortTypeLong sorts using term values as encoded 64-bit integers.
	SortTypeLong = spi.SortFieldTypeLong

	// SortTypeFloat sorts using term values as encoded 32-bit floats.
	SortTypeFloat = spi.SortFieldTypeFloat

	// SortTypeDouble sorts using term values as encoded 64-bit floats.
	SortTypeDouble = spi.SortFieldTypeDouble

	// SortTypeCustom sorts using a custom FieldComparatorSource.
	SortTypeCustom = spi.SortFieldTypeCustom
)

// SortedNumericSortField sorts a multi-valued numeric field, choosing which of
// a document's values participates in the order via a selector. Mirrors
// org.apache.lucene.search.SortedNumericSortField.
type SortedNumericSortField = spi.SortedNumericSortField

// SortedSetSortField sorts a multi-valued string field, choosing which of a
// document's values participates in the order via a selector. Mirrors
// org.apache.lucene.search.SortedSetSortField.
type SortedSetSortField = spi.SortedSetSortField

// SortRELEVANCE sorts by relevance (score). It cannot be used as an index
// sort. Mirrors org.apache.lucene.search.Sort.RELEVANCE.
var SortRELEVANCE = spi.SortRELEVANCE

// NewSort builds a Sort from the given SortFields, applied in order.
func NewSort(fields ...*SortField) *Sort { return spi.NewSort(fields...) }

// NewSortFromFields wraps an existing slice of SortFields in a Sort. The slice
// is taken by reference; callers must not retain or mutate it afterwards.
func NewSortFromFields(fields []SortField) *Sort { return spi.NewSortFromFields(fields) }

// NewSortField builds an ascending SortField over the named field with the
// given sort type.
func NewSortField(field string, sortType SortType) *SortField {
	return spi.NewSortField(field, sortType)
}

// NewSortFieldFull builds a SortField over the named field with the given sort
// type and direction.
func NewSortFieldFull(field string, sortType SortType, descending bool) *SortField {
	return spi.NewSortFieldFull(field, sortType, descending)
}

// NewSortedNumericSortField builds a SortField over a multi-valued numeric
// field, selecting the minimum value of each document by default.
func NewSortedNumericSortField(name string, sortType SortType) *SortedNumericSortField {
	return spi.NewSortedNumericSortField(name, sortType)
}

// NewSortedSetSortField builds a SortField over a multi-valued string field,
// selecting the minimum value of each document by default.
func NewSortedSetSortField(name string, reverse bool) *SortedSetSortField {
	return spi.NewSortedSetSortField(name, reverse)
}
