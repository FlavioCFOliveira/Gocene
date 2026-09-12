// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

import "fmt"

// IndexOptions controls how much information is stored in the postings lists.
// This determines what information is available for scoring and highlighting.
type IndexOptions int

const (
	// IndexOptionsNone means the field is not indexed at all.
	IndexOptionsNone IndexOptions = iota

	// IndexOptionsDocs means only the document IDs are indexed.
	// This is useful for fields that need to be searchable but don't need
	// scoring information (e.g., filter fields).
	IndexOptionsDocs

	// IndexOptionsDocsAndFreqs means document IDs and term frequencies are indexed.
	// This allows for scoring based on term frequency but doesn't store
	// positional information.
	IndexOptionsDocsAndFreqs

	// IndexOptionsDocsAndFreqsAndPositions means document IDs, term frequencies,
	// and term positions are indexed. This enables phrase queries and positional
	// queries but uses more space.
	IndexOptionsDocsAndFreqsAndPositions

	// IndexOptionsDocsAndFreqsAndPositionsAndOffsets means all information is indexed:
	// document IDs, term frequencies, positions, and character offsets.
	// This enables highlighting and span queries.
	IndexOptionsDocsAndFreqsAndPositionsAndOffsets
)

// String returns the string representation of the IndexOptions.
func (io IndexOptions) String() string {
	switch io {
	case IndexOptionsNone:
		return "NONE"
	case IndexOptionsDocs:
		return "DOCS"
	case IndexOptionsDocsAndFreqs:
		return "DOCS_AND_FREQS"
	case IndexOptionsDocsAndFreqsAndPositions:
		return "DOCS_AND_FREQS_AND_POSITIONS"
	case IndexOptionsDocsAndFreqsAndPositionsAndOffsets:
		return "DOCS_AND_FREQS_AND_POSITIONS_AND_OFFSETS"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", io)
	}
}

// Subsumes returns true if this set of index options includes the given options.
func (io IndexOptions) Subsumes(other IndexOptions) bool {
	return io >= other
}

// IsIndexed returns true if the field is indexed (has any index options other than NONE).
func (io IndexOptions) IsIndexed() bool {
	return io != IndexOptionsNone
}

// HasFreqs returns true if term frequencies are stored.
func (io IndexOptions) HasFreqs() bool {
	return io >= IndexOptionsDocsAndFreqs
}

// HasPositions returns true if term positions are stored.
func (io IndexOptions) HasPositions() bool {
	return io >= IndexOptionsDocsAndFreqsAndPositions
}

// HasOffsets returns true if term offsets are stored.
func (io IndexOptions) HasOffsets() bool {
	return io >= IndexOptionsDocsAndFreqsAndPositionsAndOffsets
}

// DocValuesType specifies the type of per-document values stored in the index.
// DocValues are stored in a columnar format for efficient sorting, faceting,
// and value retrieval.
//
// The constant ordinals MUST match Apache Lucene 10.4.0's
// org.apache.lucene.index.DocValuesType enum, because the ordinal is the
// on-disk byte encoding used by FieldInfosFormat.
//
// Ordinal order:
//
//	NONE=0, NUMERIC=1, BINARY=2, SORTED=3, SORTED_NUMERIC=4, SORTED_SET=5
type DocValuesType int

const (
	// DocValuesTypeNone means no doc values are stored for this field.
	DocValuesTypeNone DocValuesType = iota

	// DocValuesTypeNumeric stores a single numeric value per document.
	// The value is a signed 64-bit integer.
	DocValuesTypeNumeric

	// DocValuesTypeBinary stores a variable-length binary value per document.
	// Values may be larger than 32766 bytes, but different codecs may enforce
	// their own limits.
	DocValuesTypeBinary

	// DocValuesTypeSorted stores a pre-sorted byte[] per document. Fields with
	// this type only store distinct byte values and store an additional offset
	// pointer per document to dereference the shared byte[]. The stored byte[]
	// is presorted and allows access via document id, ordinal and by-value.
	// Values must be <= 32766 bytes.
	DocValuesTypeSorted

	// DocValuesTypeSortedNumeric stores a pre-sorted Number[] per document.
	// Fields with this type store numeric values in sorted order.
	DocValuesTypeSortedNumeric

	// DocValuesTypeSortedSet stores a pre-sorted Set<byte[]> per document.
	// Fields with this type only store distinct byte values and store
	// additional offset pointers per document to dereference the shared byte[]s.
	// The stored byte[] is presorted and allows access via document id,
	// ordinal and by-value. Values must be <= 32766 bytes.
	DocValuesTypeSortedSet
)

// DocValuesSkipIndexType represents the type of doc values skip index.
//
// The constant ordinals MUST match Apache Lucene 10.4.0's
// org.apache.lucene.index.DocValuesSkipIndexType enum.
//
// Ordinal order:
//
//	NONE=0, RANGE=1
type DocValuesSkipIndexType int

const (
	// DocValuesSkipIndexTypeNone means no skip index should be created.
	DocValuesSkipIndexTypeNone DocValuesSkipIndexType = iota

	// DocValuesSkipIndexTypeRange records range of values. Suitable for
	// NUMERIC, SORTED_NUMERIC, SORTED and SORTED_SET doc values; records the
	// min/max values per range of doc IDs.
	DocValuesSkipIndexTypeRange
)

// String returns the string representation of the DocValuesSkipIndexType.
func (dvst DocValuesSkipIndexType) String() string {
	switch dvst {
	case DocValuesSkipIndexTypeNone:
		return "NONE"
	case DocValuesSkipIndexTypeRange:
		return "RANGE"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", dvst)
	}
}

// IsCompatibleWith reports whether this skip-index type is permitted for the
// given doc-values type. Matches Lucene 10.4.0's isCompatibleWith semantics.
func (dvst DocValuesSkipIndexType) IsCompatibleWith(dvt DocValuesType) bool {
	switch dvst {
	case DocValuesSkipIndexTypeNone:
		return true
	case DocValuesSkipIndexTypeRange:
		return dvt == DocValuesTypeNumeric ||
			dvt == DocValuesTypeSortedNumeric ||
			dvt == DocValuesTypeSorted ||
			dvt == DocValuesTypeSortedSet
	default:
		return false
	}
}

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
	case DocValuesTypeSortedNumeric:
		return "SORTED_NUMERIC"
	case DocValuesTypeSortedSet:
		return "SORTED_SET"
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

