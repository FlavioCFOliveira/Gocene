// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"bytes"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// Ported from Apache Lucene 10.5.0:
//
//	lucene/core/src/java/org/apache/lucene/search/FieldComparator.java

// FieldComparator compares hits so as to determine their sort order when
// collecting the top results with [TopFieldCollector]. The concrete comparators
// correspond to the [SortField] types.
//
// The document IDs passed to these methods must only move forwards, since they
// are using doc values iterators to retrieve sort values.
//
// This API is designed to achieve high performance sorting, by exposing a tight
// interaction with the field-value hit queue as it visits hits. Whenever a hit
// is competitive, it is enrolled into a virtual slot, which is an int ranging
// from 0 to numHits-1. Segment transitions are handled by creating a dedicated
// per-segment [LeafFieldComparator].
//
// Mirrors org.apache.lucene.search.FieldComparator<T>. Java's type parameter is
// rendered as any: every Lucene consumer of the class holds the wildcard type
// FieldComparator<?> (SortField.getComparator returns it, and
// FieldValueHitQueue stores an array of it), so the erased form is what the
// port actually needs. Java's three concrete members — compareValues,
// setSingleSort and disableSkipping — are supplied by embedding
// [BaseFieldComparator], which stands in for inheriting them from the abstract
// class.
type FieldComparator interface {
	// Compare compares the hit at slot1 with the hit at slot2, returning any
	// N < 0 if slot2's value is sorted after slot1's, any N > 0 if slot2's
	// value is sorted before slot1's, and 0 if they are equal.
	//
	// Mirrors int compare(int slot1, int slot2).
	Compare(slot1, slot2 int) int

	// SetTopValue records the top value, for future calls to
	// LeafFieldComparator.CompareTop. This is only called for searches that use
	// searchAfter (deep paging), and is called before any call to
	// GetLeafComparator.
	//
	// Mirrors void setTopValue(T value).
	SetTopValue(value any)

	// Value returns the actual value in the slot.
	//
	// Mirrors T value(int slot).
	Value(slot int) any

	// GetLeafComparator returns a per-segment [LeafFieldComparator] to collect
	// the given leaf reader context. All docIDs supplied to that comparator are
	// relative to the current reader (the caller must add docBase to map one to
	// a top-level docID).
	//
	// Mirrors
	// LeafFieldComparator getLeafComparator(LeafReaderContext) throws IOException.
	GetLeafComparator(context *index.LeafReaderContext) (LeafFieldComparator, error)

	// CompareValues returns a negative integer if first is less than second, 0
	// if they are equal and a positive integer otherwise.
	//
	// Mirrors int compareValues(T first, T second).
	CompareValues(first, second any) int

	// SetSingleSort informs the comparator that the sort is done on this single
	// field. This is useful to enable some optimizations for skipping
	// non-competitive documents.
	//
	// Mirrors void setSingleSort().
	SetSingleSort()

	// DisableSkipping informs the comparator that the skipping of documents
	// should be disabled.
	//
	// Mirrors void disableSkipping().
	DisableSkipping()
}

// BaseFieldComparator carries the three members that
// org.apache.lucene.search.FieldComparator declares as concrete methods on the
// abstract class — compareValues, setSingleSort and disableSkipping — so that a
// comparator inherits them by embedding, as a Java subclass inherits them by
// extending. A comparator that overrides one of them in Java simply declares
// its own method here, which shadows the embedded one.
//
// SetSingleSort and DisableSkipping have empty bodies in Java, and empty bodies
// here.
type BaseFieldComparator struct{}

// CompareValues is the default body of FieldComparator.compareValues: nulls
// sort first, and otherwise the two values are compared with the natural
// ordering of their type.
func (BaseFieldComparator) CompareValues(first, second any) int {
	return compareFieldValues(first, second)
}

// SetSingleSort mirrors the empty FieldComparator.setSingleSort().
func (BaseFieldComparator) SetSingleSort() {}

// DisableSkipping mirrors the empty FieldComparator.disableSkipping().
func (BaseFieldComparator) DisableSkipping() {}

// compareFieldValues renders the body of FieldComparator.compareValues:
//
//	if (first == null) { if (second == null) return 0; else return -1; }
//	else if (second == null) return 1;
//	else return ((Comparable<T>) first).compareTo(second);
//
// The cast to Comparable is unchecked in Java and raises ClassCastException at
// run time for a value that is not comparable, or whose type does not match the
// other operand; the Go rendering panics, which is how Gocene already renders
// the unchecked exceptions raised by SortField.validateField and by
// BinarySortField's constructor.
//
// The supported types are exactly the ones Lucene's own comparators use as T:
// Integer (int32, and Go's int as produced by the DOC comparator), Long
// (int64), Float (float32), Double (float64) and BytesRef ([]byte). Java's
// String.compareTo orders by UTF-16 code unit, which is not the order of Go's
// string comparison, so a string operand is refused rather than compared under
// a different ordering — no Lucene comparator uses String as its T.
func compareFieldValues(first, second any) int {
	if first == nil {
		if second == nil {
			return 0
		}
		return -1
	} else if second == nil {
		return 1
	}
	switch a := first.(type) {
	case int:
		if b, ok := second.(int); ok {
			return cmpInt(a, b)
		}
	case int32:
		if b, ok := second.(int32); ok {
			return cmpInt32(a, b)
		}
	case int64:
		if b, ok := second.(int64); ok {
			return cmpInt64(a, b)
		}
	case float32:
		if b, ok := second.(float32); ok {
			return cmpFloat32(a, b)
		}
	case float64:
		if b, ok := second.(float64); ok {
			return cmpFloat64(a, b)
		}
	case []byte:
		if b, ok := second.([]byte); ok {
			return bytes.Compare(a, b)
		}
	}
	panic(fmt.Sprintf("search: FieldComparator.CompareValues cannot compare %T with %T", first, second))
}

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

// NumericDocValuesSource resolves the NumericDocValues a numeric field comparator
// should read for a given leaf reader and field. A custom source lets callers
// substitute a derived/wrapped iterator (e.g. block-join MIN/MAX selection over a
// parent's children) in place of the field's stored values.
//
// Mirrors the getNumericDocValues(LeafReaderContext, String) hook that Lucene's
// numeric LeafComparators expose for subclassing.
type NumericDocValuesSource interface {
	// NumericDocValues returns the iterator the comparator reads, or nil when the
	// leaf has no values (treated as every document missing). The reader is the
	// leaf reader the comparator was just bound to.
	NumericDocValues(reader IndexReader, field string) (NumericDocValuesIterator, error)
}

// SortedDocValuesSource resolves the SortedDocValues the STRING comparator should
// read for a given leaf reader and field. Mirrors the getSortedDocValues hook in
// Lucene's TermOrdValComparator.
type SortedDocValuesSource interface {
	// SortedDocValues returns the iterator the comparator reads, or nil when the
	// leaf has no values (treated as every document missing).
	SortedDocValues(reader IndexReader, field string) (SortedDocValuesIterator, error)
}

// simpleFieldComparator is the shape shared by every comparator in this package
// that serves as its own per-leaf view — the arrangement Lucene expresses with
// SimpleFieldComparator, whose getLeafComparator returns `this`. It is the
// internal contract the DocValues-backed comparators satisfy; the collector
// itself holds the two public interfaces, exactly as Lucene's
// FieldValueHitQueue holds FieldComparator[] and hands out
// LeafFieldComparator[].
type simpleFieldComparator interface {
	FieldComparator
	LeafFieldComparator

	// setReader binds the comparator to a new leaf reader, resolving the
	// segment's DocValues iterator for the sort field. It is the body of
	// GetLeafComparator, factored out because the leaf reader — and not the
	// whole context — is all the DocValues lookup needs.
	setReader(reader IndexReader) error
}

// leafReaderOf returns the leaf reader a context represents, as the narrow
// reader contract the comparators bind to, tolerating a nil context (which
// Gocene's collector passes when it collects without a segment).
func leafReaderOf(context *index.LeafReaderContext) IndexReader {
	if context == nil {
		return nil
	}
	reader := context.LeafReader()
	if reader == nil {
		return nil
	}
	return reader
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
	return sf.Missing != spi.MissingValueFirst
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

// topValueInt32, topValueInt64, topValueFloat32, topValueFloat64 and
// topValueBytes render the unboxing Java performs when a comparator's
// setTopValue(T) assigns its argument to a primitive field: a null raises
// NullPointerException and a value of another type raises ClassCastException,
// both unchecked. The Go rendering panics for the same inputs.
func topValueInt32(v any) int32 {
	switch n := v.(type) {
	case int32:
		return n
	case int:
		return int32(n)
	case int64:
		return int32(n)
	}
	panic(fmt.Sprintf("search: SetTopValue expects an INT sort value, got %T", v))
}

func topValueInt64(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case int32:
		return int64(n)
	}
	panic(fmt.Sprintf("search: SetTopValue expects a LONG sort value, got %T", v))
}

func topValueFloat32(v any) float32 {
	switch n := v.(type) {
	case float32:
		return n
	case float64:
		return float32(n)
	}
	panic(fmt.Sprintf("search: SetTopValue expects a FLOAT sort value, got %T", v))
}

func topValueFloat64(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	}
	panic(fmt.Sprintf("search: SetTopValue expects a DOUBLE sort value, got %T", v))
}

// bytesRefValue renders a BytesRef parameter of the term comparators. Java
// documents null as meaningful for setTopValue ("it means the last doc of the
// prior search was missing this value") and for compareValues (a missing
// value), so nil is accepted; any other type raises ClassCastException in Java
// and panics here. member names the Java member for the message.
func bytesRefValue(v any, member string) []byte {
	if v == nil {
		return nil
	}
	if b, ok := v.([]byte); ok {
		return b
	}
	panic(fmt.Sprintf("search: %s expects a STRING sort value, got %T", member, v))
}

// topValueBytes renders the BytesRef parameter of setTopValue for the term
// comparators.
func topValueBytes(v any) []byte { return bytesRefValue(v, "SetTopValue") }

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

// Compile-time guarantees that every comparator satisfies the interfaces.
var (
	_ simpleFieldComparator = (*intComparator)(nil)
	_ simpleFieldComparator = (*longComparator)(nil)
	_ simpleFieldComparator = (*floatComparator)(nil)
	_ simpleFieldComparator = (*doubleComparator)(nil)
	_ simpleFieldComparator = (*termOrdValComparator)(nil)
)
