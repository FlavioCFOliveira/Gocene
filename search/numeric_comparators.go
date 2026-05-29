// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/comparators/NumericComparator.java
//   lucene/core/src/java/org/apache/lucene/search/comparators/IntComparator.java
//   lucene/core/src/java/org/apache/lucene/search/comparators/LongComparator.java
//   lucene/core/src/java/org/apache/lucene/search/comparators/FloatComparator.java
//   lucene/core/src/java/org/apache/lucene/search/comparators/DoubleComparator.java
//
// These are the per-segment NumericDocValues-backed field comparators that the
// field-sorted search path drives. Unlike Lucene, this port does not yet
// implement the BKD-index skipping (competitive-iterator) optimisation; it
// reproduces the value semantics — advanceExact(doc) to read the per-document
// long, fall back to the missing value when the document has no value, decode
// the long according to the field type, and compare with the natural ordering
// for that primitive. Skipping is a pure performance optimisation; omitting it
// changes nothing observable about the produced order (Lucene's
// NumericComparator with pruning disabled behaves identically).

// numericDocValuesReader is the narrow view of a leaf reader needed to obtain
// the per-field NumericDocValues iterator. SegmentReader and LeafReader both
// satisfy it. It is declared here (rather than imported from index) to keep the
// comparator decoupled from the concrete reader type, mirroring how
// index_searcher.go type-asserts for GetLiveDocs.
type numericDocValuesReader interface {
	GetNumericDocValues(field string) (index.NumericDocValues, error)
}

// numericLeafState holds the docValues iterator bound to the current leaf, used
// by every numeric comparator's getValueForDoc.
//
// Mirrors NumericComparator.NumericLeafComparator (minus the skipping state).
type numericLeafState struct {
	docValues index.NumericDocValues
	// valueAvailable mirrors Lucene's NumericLeafComparator.docExists optimisation
	// indirectly: when the segment has no values for the field at all
	// (docValues == nil), every document is treated as missing.
}

// bindNumericLeaf resolves the NumericDocValues iterator for field on the given
// reader. A nil iterator (no producer / no field) is not an error: every
// document is then treated as missing, matching Lucene's DocValues.getNumeric
// returning an empty iterator.
func bindNumericLeaf(reader IndexReader, field string) (*numericLeafState, error) {
	r, ok := reader.(numericDocValuesReader)
	if !ok {
		return &numericLeafState{}, nil
	}
	dv, err := r.GetNumericDocValues(field)
	if err != nil {
		return nil, err
	}
	return &numericLeafState{docValues: dv}, nil
}

// getRawValueForDoc positions the iterator on doc and returns its long value,
// or (missing, false) when the document has no value. Callers advance with
// monotonically increasing doc ids, satisfying the AdvanceExact contract.
func (s *numericLeafState) getRawValueForDoc(doc int, missing int64) (int64, error) {
	if s.docValues == nil {
		return missing, nil
	}
	exists, err := s.docValues.AdvanceExact(doc)
	if err != nil {
		return 0, err
	}
	if !exists {
		return missing, nil
	}
	v, err := s.docValues.LongValue()
	if err != nil {
		return 0, err
	}
	return v, nil
}

// --- IntComparator -----------------------------------------------------------

// intComparator is the SortFieldTypeInt field comparator.
//
// Mirrors org.apache.lucene.search.comparators.IntComparator.
type intComparator struct {
	values  []int32
	bottom  int32
	missing int32
	field   string
	leaf    *numericLeafState
}

func newIntComparator(numHits int, field string, missing int32) *intComparator {
	return &intComparator{values: make([]int32, numHits), field: field, missing: missing}
}

func (c *intComparator) compare(slot1, slot2 int) int {
	return cmpInt32(c.values[slot1], c.values[slot2])
}

func (c *intComparator) value(slot int) any { return c.values[slot] }

func (c *intComparator) setReader(reader IndexReader) error {
	leaf, err := bindNumericLeaf(reader, c.field)
	if err != nil {
		return err
	}
	c.leaf = leaf
	return nil
}

func (c *intComparator) getValueForDoc(doc int) (int32, error) {
	v, err := c.leaf.getRawValueForDoc(doc, int64(c.missing))
	return int32(v), err
}

func (c *intComparator) SetBottom(slot int) error { c.bottom = c.values[slot]; return nil }

func (c *intComparator) CompareBottom(doc int) (int, error) {
	v, err := c.getValueForDoc(doc)
	if err != nil {
		return 0, err
	}
	return cmpInt32(c.bottom, v), nil
}

func (c *intComparator) CompareTop(doc int) (int, error) { return 0, nil }

func (c *intComparator) Copy(slot, doc int) error {
	v, err := c.getValueForDoc(doc)
	if err != nil {
		return err
	}
	c.values[slot] = v
	return nil
}

func (c *intComparator) SetScorer(Scorable) error                       { return nil }
func (c *intComparator) CompetitiveIterator() (DocIdSetIterator, error) { return nil, nil }
func (c *intComparator) SetHitsThresholdReached()                       {}

// --- LongComparator ----------------------------------------------------------

// longComparator is the SortFieldTypeLong field comparator.
//
// Mirrors org.apache.lucene.search.comparators.LongComparator.
type longComparator struct {
	values  []int64
	bottom  int64
	missing int64
	field   string
	leaf    *numericLeafState
}

func newLongComparator(numHits int, field string, missing int64) *longComparator {
	return &longComparator{values: make([]int64, numHits), field: field, missing: missing}
}

func (c *longComparator) compare(slot1, slot2 int) int {
	return cmpInt64(c.values[slot1], c.values[slot2])
}

func (c *longComparator) value(slot int) any { return c.values[slot] }

func (c *longComparator) setReader(reader IndexReader) error {
	leaf, err := bindNumericLeaf(reader, c.field)
	if err != nil {
		return err
	}
	c.leaf = leaf
	return nil
}

func (c *longComparator) getValueForDoc(doc int) (int64, error) {
	return c.leaf.getRawValueForDoc(doc, c.missing)
}

func (c *longComparator) SetBottom(slot int) error { c.bottom = c.values[slot]; return nil }

func (c *longComparator) CompareBottom(doc int) (int, error) {
	v, err := c.getValueForDoc(doc)
	if err != nil {
		return 0, err
	}
	return cmpInt64(c.bottom, v), nil
}

func (c *longComparator) CompareTop(doc int) (int, error) { return 0, nil }

func (c *longComparator) Copy(slot, doc int) error {
	v, err := c.getValueForDoc(doc)
	if err != nil {
		return err
	}
	c.values[slot] = v
	return nil
}

func (c *longComparator) SetScorer(Scorable) error                       { return nil }
func (c *longComparator) CompetitiveIterator() (DocIdSetIterator, error) { return nil, nil }
func (c *longComparator) SetHitsThresholdReached()                       {}

// --- FloatComparator ---------------------------------------------------------

// floatComparator is the SortFieldTypeFloat field comparator. The per-document
// long stored in NumericDocValues is the IEEE-754 bit pattern of the float
// (Lucene encodes FloatDocValuesField as Float.floatToIntBits).
//
// Mirrors org.apache.lucene.search.comparators.FloatComparator, whose
// getValueForDoc does Float.intBitsToFloat((int) docValues.longValue()).
type floatComparator struct {
	values  []float32
	bottom  float32
	missing float32
	field   string
	leaf    *numericLeafState
}

func newFloatComparator(numHits int, field string, missing float32) *floatComparator {
	return &floatComparator{values: make([]float32, numHits), field: field, missing: missing}
}

func (c *floatComparator) compare(slot1, slot2 int) int {
	return cmpFloat32(c.values[slot1], c.values[slot2])
}

func (c *floatComparator) value(slot int) any { return c.values[slot] }

func (c *floatComparator) setReader(reader IndexReader) error {
	leaf, err := bindNumericLeaf(reader, c.field)
	if err != nil {
		return err
	}
	c.leaf = leaf
	return nil
}

func (c *floatComparator) getValueForDoc(doc int) (float32, error) {
	if c.leaf.docValues == nil {
		return c.missing, nil
	}
	exists, err := c.leaf.docValues.AdvanceExact(doc)
	if err != nil {
		return 0, err
	}
	if !exists {
		return c.missing, nil
	}
	bits, err := c.leaf.docValues.LongValue()
	if err != nil {
		return 0, err
	}
	return math.Float32frombits(uint32(bits)), nil
}

func (c *floatComparator) SetBottom(slot int) error { c.bottom = c.values[slot]; return nil }

func (c *floatComparator) CompareBottom(doc int) (int, error) {
	v, err := c.getValueForDoc(doc)
	if err != nil {
		return 0, err
	}
	return cmpFloat32(c.bottom, v), nil
}

func (c *floatComparator) CompareTop(doc int) (int, error) { return 0, nil }

func (c *floatComparator) Copy(slot, doc int) error {
	v, err := c.getValueForDoc(doc)
	if err != nil {
		return err
	}
	c.values[slot] = v
	return nil
}

func (c *floatComparator) SetScorer(Scorable) error                       { return nil }
func (c *floatComparator) CompetitiveIterator() (DocIdSetIterator, error) { return nil, nil }
func (c *floatComparator) SetHitsThresholdReached()                       {}

// --- DoubleComparator --------------------------------------------------------

// doubleComparator is the SortFieldTypeDouble field comparator. The
// per-document long is the IEEE-754 bit pattern of the double.
//
// Mirrors org.apache.lucene.search.comparators.DoubleComparator, whose
// getValueForDoc does Double.longBitsToDouble(docValues.longValue()).
type doubleComparator struct {
	values  []float64
	bottom  float64
	missing float64
	field   string
	leaf    *numericLeafState
}

func newDoubleComparator(numHits int, field string, missing float64) *doubleComparator {
	return &doubleComparator{values: make([]float64, numHits), field: field, missing: missing}
}

func (c *doubleComparator) compare(slot1, slot2 int) int {
	return cmpFloat64(c.values[slot1], c.values[slot2])
}

func (c *doubleComparator) value(slot int) any { return c.values[slot] }

func (c *doubleComparator) setReader(reader IndexReader) error {
	leaf, err := bindNumericLeaf(reader, c.field)
	if err != nil {
		return err
	}
	c.leaf = leaf
	return nil
}

func (c *doubleComparator) getValueForDoc(doc int) (float64, error) {
	if c.leaf.docValues == nil {
		return c.missing, nil
	}
	exists, err := c.leaf.docValues.AdvanceExact(doc)
	if err != nil {
		return 0, err
	}
	if !exists {
		return c.missing, nil
	}
	bits, err := c.leaf.docValues.LongValue()
	if err != nil {
		return 0, err
	}
	return math.Float64frombits(uint64(bits)), nil
}

func (c *doubleComparator) SetBottom(slot int) error { c.bottom = c.values[slot]; return nil }

func (c *doubleComparator) CompareBottom(doc int) (int, error) {
	v, err := c.getValueForDoc(doc)
	if err != nil {
		return 0, err
	}
	return cmpFloat64(c.bottom, v), nil
}

func (c *doubleComparator) CompareTop(doc int) (int, error) { return 0, nil }

func (c *doubleComparator) Copy(slot, doc int) error {
	v, err := c.getValueForDoc(doc)
	if err != nil {
		return err
	}
	c.values[slot] = v
	return nil
}

func (c *doubleComparator) SetScorer(Scorable) error                       { return nil }
func (c *doubleComparator) CompetitiveIterator() (DocIdSetIterator, error) { return nil, nil }
func (c *doubleComparator) SetHitsThresholdReached()                       {}

// --- primitive comparison helpers (Java Integer.compare etc. semantics) ------

func cmpInt32(a, b int32) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func cmpInt64(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

// cmpFloat32 mirrors Java Float.compare: it orders -0.0 < 0.0 and NaN as the
// greatest value, which is the ordering Lucene relies on for float sorts.
func cmpFloat32(a, b float32) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	ab := int32(math.Float32bits(a))
	bb := int32(math.Float32bits(b))
	switch {
	case ab < bb:
		return -1
	case ab > bb:
		return 1
	default:
		return 0
	}
}

// cmpFloat64 mirrors Java Double.compare.
func cmpFloat64(a, b float64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	ab := int64(math.Float64bits(a))
	bb := int64(math.Float64bits(b))
	switch {
	case ab < bb:
		return -1
	case ab > bb:
		return 1
	default:
		return 0
	}
}
