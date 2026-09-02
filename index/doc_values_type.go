// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "github.com/FlavioCFOliveira/Gocene/schema"

// DocValuesType is the Go port of org.apache.lucene.index.DocValuesType from
// Apache Lucene 10.5.0: the type of per-document values a field carries, if
// any. DocValues are strongly typed, so a field cannot have different types
// across different documents.
//
// The declaration lives in the schema package so that packages below index in
// the dependency graph (codecs, spi, search) can name the type without
// importing index; index re-exports it here under its Lucene name, matching
// the convention already used for FieldInfo, FieldInfos, IndexOptions and
// PostingsEnum.
//
// The constant ordinals are the on-disk byte encoding written by
// FieldInfosFormat, so they MUST match the Java enum ordinals exactly:
// NONE=0, NUMERIC=1, BINARY=2, SORTED=3, SORTED_NUMERIC=4, SORTED_SET=5.
type DocValuesType = schema.DocValuesType

const (
	// DocValuesTypeNone means no doc values are stored for this field.
	DocValuesTypeNone = schema.DocValuesTypeNone

	// DocValuesTypeNumeric stores a single numeric value per document.
	DocValuesTypeNumeric = schema.DocValuesTypeNumeric

	// DocValuesTypeBinary stores a variable-length binary value per document.
	// Values may be larger than 32766 bytes, but different codecs may enforce
	// their own limits.
	DocValuesTypeBinary = schema.DocValuesTypeBinary

	// DocValuesTypeSorted stores a pre-sorted byte slice per document. Fields
	// with this type only store distinct byte values plus an offset pointer
	// per document to dereference the shared values. Values must be <= 32766
	// bytes.
	DocValuesTypeSorted = schema.DocValuesTypeSorted

	// DocValuesTypeSortedNumeric stores a pre-sorted list of numbers per
	// document, ordered as by Long.compare in Java.
	DocValuesTypeSortedNumeric = schema.DocValuesTypeSortedNumeric

	// DocValuesTypeSortedSet stores a pre-sorted set of byte slices per
	// document. Values must be <= 32766 bytes.
	DocValuesTypeSortedSet = schema.DocValuesTypeSortedSet
)
