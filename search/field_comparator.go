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
	case SortFieldTypeCustom:
		if sf.comparatorSource == nil {
			return nil, fmt.Errorf("search: CUSTOM SortField %q has no FieldComparatorSource", sf.Field)
		}
		inner := sf.comparatorSource.NewComparator(sf, numHits)
		if inner == nil {
			return nil, fmt.Errorf("search: FieldComparatorSource for %q returned a nil comparator", sf.Field)
		}
		return newCustomFieldComparator(inner), nil
	default:
		return nil, fmt.Errorf("search: SortField type %d is not supported by the DocValues comparator factory (field=%q)", sf.Type, sf.Field)
	}
}

// customFieldComparator adapts a public FieldComparator (produced by a
// FieldComparatorSource, e.g. Lucene's ElevationComparatorSource) to the
// internal sortFieldComparator the TopFieldCollector drives. It bridges the two
// shapes: the public comparator owns its per-slot value cache and ordering
// (Compare/SetBottom/CompareBottom/Copy/SetScorer); this adapter supplies the
// extra surface the collector needs (setReader leaf binding, CompareTop,
// CompetitiveIterator, SetHitsThresholdReached, value).
//
// Leaf binding: when the wrapped comparator implements the optional
// leafBindingComparator interface, setReader forwards the leaf reader so the
// comparator can resolve the segment's DocValues (mirroring Lucene's
// FieldComparator.getLeafComparator(LeafReaderContext)). A comparator that does
// not implement it is simply not leaf-bound.
type customFieldComparator struct {
	inner FieldComparator
}

// leafBindingComparator is the optional hook a public FieldComparator implements
// to receive the per-leaf reader before Copy/CompareBottom are called for that
// segment. It is the Go analogue of FieldComparator.getLeafComparator binding a
// LeafReaderContext, expressed without changing the stable public
// FieldComparator interface.
type leafBindingComparator interface {
	SetReader(reader IndexReader) error
}

// valueComparator is the optional hook a public FieldComparator implements to
// expose the per-slot sort value for FieldDoc.Fields. A comparator that does not
// implement it reports nil values (the order is still correct).
type valueComparator interface {
	Value(slot int) any
}

func newCustomFieldComparator(inner FieldComparator) *customFieldComparator {
	return &customFieldComparator{inner: inner}
}

func (c *customFieldComparator) compare(slot1, slot2 int) int { return c.inner.Compare(slot1, slot2) }

func (c *customFieldComparator) value(slot int) any {
	if v, ok := c.inner.(valueComparator); ok {
		return v.Value(slot)
	}
	return nil
}

func (c *customFieldComparator) setReader(reader IndexReader) error {
	if lb, ok := c.inner.(leafBindingComparator); ok {
		return lb.SetReader(reader)
	}
	return nil
}

func (c *customFieldComparator) SetBottom(slot int) error { c.inner.SetBottom(slot); return nil }

func (c *customFieldComparator) CompareBottom(doc int) (int, error) {
	return c.inner.CompareBottom(doc), nil
}

func (c *customFieldComparator) CompareTop(int) (int, error) { return 0, nil }

func (c *customFieldComparator) Copy(slot, doc int) error { c.inner.Copy(slot, doc); return nil }

// SetScorer is a no-op for the custom comparator. The public FieldComparator
// expects a search.Scorer, while the collector hands the internal comparators a
// Scorable; the only built-in consumer of the scorer is the SCORE sort key,
// which the collector routes to its own relevanceComparator, not to a custom
// comparator. DocValues-backed custom comparators (e.g. the elevation
// comparator) ignore the scorer entirely (their setScorer is empty), so not
// forwarding it preserves their behaviour.
func (c *customFieldComparator) SetScorer(Scorable) error { return nil }

func (c *customFieldComparator) CompetitiveIterator() (DocIdSetIterator, error) { return nil, nil }

func (c *customFieldComparator) SetHitsThresholdReached() {}

var _ sortFieldComparator = (*customFieldComparator)(nil)

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
