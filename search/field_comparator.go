// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// NumericDocValuesIterator and SortedDocValuesIterator are the public aliases for
// the per-document DocValues iterators a custom comparator DV source returns.
// They alias the index-package iterator surfaces so the join package (and other
// callers) can implement NumericDocValuesSource / SortedDocValuesSource without
// re-declaring the contracts.
type (
	// NumericDocValuesIterator is the iterator a NumericDocValuesSource returns.
	NumericDocValuesIterator = index.NumericDocValues
	// SortedDocValuesIterator is the iterator a SortedDocValuesSource returns.
	SortedDocValuesIterator = index.SortedDocValues
)

// Ported from Apache Lucene 10.4.0:
//
//	lucene/core/src/java/org/apache/lucene/search/FieldComparator.java
//
// sortFieldComparator is the cross-segment comparator that the field-sorted
// collector maintains for a single sort key. It owns the per-slot value cache
// (compare/value) and produces a per-segment LeafFieldComparator view
// (setReader rebinds the same instance to a new leaf, mirroring how Lucene's
// SimpleFieldComparator returns `this` from getLeafComparator). Numeric
// comparators and termOrdValComparator both satisfy this interface directly.
type sortFieldComparator interface {
	LeafFieldComparator

	// compare orders two queue slots using the cached per-slot values. Returns a
	// Java-style sign: negative if slot1 sorts before slot2.
	compare(slot1, slot2 int) int

	// value returns the sort value cached in the slot, typed per the field
	// (int32, int64, float32, float64, or a []byte term / nil for missing).
	value(slot int) any

	// setReader rebinds the comparator to a new leaf reader, resolving the
	// segment's DocValues iterator for the sort field.
	setReader(reader IndexReader) error
}

// newSortFieldComparator builds the comparator for a single SortField, mapping
// SortField.Type to the matching DocValues-backed comparator. numHits sizes the
// per-slot value cache (one slot per queue entry).
//
// Mirrors org.apache.lucene.search.SortField.getComparator. Supported types are
// INT, LONG, FLOAT, DOUBLE (NumericDocValues) and STRING (SortedDocValues).
// SCORE/DOC are handled by the collector directly (they need no DocValues) and
// are reported here as unsupported so callers route them explicitly. CUSTOM is
// resolved through SortField's FieldComparatorSource when present.
func newSortFieldComparator(sf *SortField, numHits int) (sortFieldComparator, error) {
	sortMissingLast := missingSortsLast(sf)
	switch sf.Type {
	case SortFieldTypeInt:
		c := newIntComparator(numHits, sf.Field, missingInt32(sf))
		c.dvSource = sf.numericDVSource
		return c, nil
	case SortFieldTypeLong:
		c := newLongComparator(numHits, sf.Field, missingInt64(sf))
		c.dvSource = sf.numericDVSource
		return c, nil
	case SortFieldTypeFloat:
		c := newFloatComparator(numHits, sf.Field, missingFloat32(sf))
		c.dvSource = sf.numericDVSource
		return c, nil
	case SortFieldTypeDouble:
		c := newDoubleComparator(numHits, sf.Field, missingFloat64(sf))
		c.dvSource = sf.numericDVSource
		return c, nil
	case SortFieldTypeString:
		c := newTermOrdValComparator(numHits, sf.Field, sortMissingLast)
		c.dvSource = sf.sortedDVSource
		return c, nil
	default:
		return nil, fmt.Errorf("search: SortField type %d is not supported by the DocValues comparator factory (field=%q)", sf.Type, sf.Field)
	}
}

// missingSortsLast reports whether missing values should sort after present
// values for the given SortField. The STRING_FIRST/STRING_LAST sentinels and the
// MissingValueStrategy both feed into this decision; STRING_FIRST forces
// missing-first regardless of reverse, matching Lucene.
func missingSortsLast(sf *SortField) bool {
	switch sf.MissingValue {
	case STRING_FIRST:
		return false
	case STRING_LAST:
		return true
	}
	return sf.Missing != MissingValueFirst
}

// missingInt32 resolves the int missing value (default 0, matching Lucene's
// IntComparator which substitutes 0 when missingValue is null).
func missingInt32(sf *SortField) int32 {
	switch v := sf.MissingValue.(type) {
	case int32:
		return v
	case int:
		return int32(v)
	case int64:
		return int32(v)
	}
	return 0
}

func missingInt64(sf *SortField) int64 {
	switch v := sf.MissingValue.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case int32:
		return int64(v)
	}
	return 0
}

func missingFloat32(sf *SortField) float32 {
	switch v := sf.MissingValue.(type) {
	case float32:
		return v
	case float64:
		return float32(v)
	}
	return 0
}

func missingFloat64(sf *SortField) float64 {
	switch v := sf.MissingValue.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	}
	return 0
}

// reverseMul returns +1 for an ascending sort field and -1 for a descending
// one. This is multiplied into each comparator's result so the collector can
// keep a single "weakest at the top" ordering convention.
//
// Mirrors org.apache.lucene.search.FieldValueHitQueue.reverseMul.
func reverseMul(sf *SortField) int {
	if sf.Reverse {
		return -1
	}
	return 1
}

// Compile-time guarantees that every comparator satisfies the interface.
var (
	_ sortFieldComparator = (*intComparator)(nil)
	_ sortFieldComparator = (*longComparator)(nil)
	_ sortFieldComparator = (*floatComparator)(nil)
	_ sortFieldComparator = (*doubleComparator)(nil)
	_ sortFieldComparator = (*termOrdValComparator)(nil)
)
