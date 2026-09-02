// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

// Port note:
//
// This file is the Go port of
// lucene/core/src/java/org/apache/lucene/index/NumericDocValuesFieldUpdates.java
// (Apache Lucene 10.4.0). It is the numeric sibling of
// [BinaryDocValuesFieldUpdates]; both embed the shared
// [BaseDocValuesFieldUpdates] and supply subtype-specific
// Swap/Grow/Resize hooks, mirroring Java's virtual dispatch out of the
// abstract parent.
//
// The values storage is held as a *packed.AbstractPagedMutable so the
// two Java constructors (PagedGrowableWriter when the range is unknown,
// PagedMutable when min/max bounds are supplied) collapse into one Go
// field; both concrete types embed *AbstractPagedMutable.
//
// SingleValueNumericDocValuesFieldUpdates, the Java nested static
// class, is ported as [SingleValueNumericDocValuesFieldUpdates] in the
// same package.

// NumericDocValuesFieldUpdates accumulates a packet of numeric
// doc-values updates for a single field within one segment. Add(),
// Reset() and Finish() are safe for concurrent use through the embedded
// [BaseDocValuesFieldUpdates] mutex; Iterator() is not, and callers
// MUST call Finish() before iterating, matching Lucene.
type NumericDocValuesFieldUpdates struct {
	BaseDocValuesFieldUpdates

	values   *packed.AbstractPagedMutable
	minValue int64
}

// NewNumericDocValuesFieldUpdates creates a fresh, empty packet for the
// given field at the given delete generation, with an unknown value
// range. A PagedGrowableWriter backs the values storage so the bit
// width adapts as updates arrive. Mirrors the two-arg Java constructor.
//
// Returns an error when the underlying PagedMutable allocations fail,
// which in practice can only happen for invalid maxDoc.
func NewNumericDocValuesFieldUpdates(delGen int64, field string, maxDoc int) (*NumericDocValuesFieldUpdates, error) {
	n := &NumericDocValuesFieldUpdates{}
	if err := InitBaseDocValuesFieldUpdates(&n.BaseDocValuesFieldUpdates, maxDoc, delGen, field, DocValuesTypeNumeric); err != nil {
		return nil, err
	}
	// We don't know the min/max range so we use the growable writer
	// here to adjust as we go.
	values, err := packed.NewPagedGrowableWriter(1, docValuesFieldUpdatesPageSize, 1, packed.Fast)
	if err != nil {
		return nil, fmt.Errorf("numeric doc values field updates: values: %w", err)
	}
	n.values = values.AbstractPagedMutable
	n.minValue = 0

	n.HookSwap = n.swap
	n.HookGrow = n.grow
	n.HookResize = n.resize
	return n, nil
}

// NewNumericDocValuesFieldUpdatesBounded creates a packet whose values
// storage is sized exactly for the [minValue, maxValue] range, using a
// fixed-width PagedMutable. Mirrors the five-arg Java constructor.
//
// Returns an error when minValue exceeds maxValue or when the
// underlying PagedMutable allocations fail.
func NewNumericDocValuesFieldUpdatesBounded(delGen int64, field string, minValue, maxValue int64, maxDoc int) (*NumericDocValuesFieldUpdates, error) {
	if minValue > maxValue {
		return nil, fmt.Errorf("numeric doc values field updates: minValue must be <= maxValue [%d > %d]", minValue, maxValue)
	}
	n := &NumericDocValuesFieldUpdates{}
	if err := InitBaseDocValuesFieldUpdates(&n.BaseDocValuesFieldUpdates, maxDoc, delGen, field, DocValuesTypeNumeric); err != nil {
		return nil, err
	}
	bitsPerValue := packed.UnsignedBitsRequired(uint64(maxValue - minValue))
	values, err := packed.NewPagedMutable(1, docValuesFieldUpdatesPageSize, bitsPerValue, packed.Fast)
	if err != nil {
		return nil, fmt.Errorf("numeric doc values field updates: values: %w", err)
	}
	n.values = values.AbstractPagedMutable
	n.minValue = minValue

	n.HookSwap = n.swap
	n.HookGrow = n.grow
	n.HookResize = n.resize
	return n, nil
}

// AddBinary is unsupported on a numeric packet. Mirrors the Java
// {@code NumericDocValuesFieldUpdates#add(int, BytesRef)} which throws
// UnsupportedOperationException.
func (n *NumericDocValuesFieldUpdates) AddBinary(doc int, value *util.BytesRef) error {
	return fmt.Errorf("numeric doc values field updates: AddBinary unsupported")
}

// AddLong records a numeric update for doc. The stored value is
// rebased by minValue so the backing PagedMutable only ever holds
// non-negative offsets. Mirrors {@code add(int, long)}.
func (n *NumericDocValuesFieldUpdates) AddLong(doc int, value int64) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	index, err := n.addInternalLocked(doc, docValuesFieldUpdatesHasValueMask)
	if err != nil {
		return err
	}
	n.values.Set(int64(index), value-n.minValue)
	return nil
}

// AddFromIterator copies the long value the iterator currently exposes
// into a new entry for doc. Mirrors
// {@code add(int, DocValuesFieldUpdates.Iterator)}.
func (n *NumericDocValuesFieldUpdates) AddFromIterator(doc int, it DocValuesFieldUpdatesIterator) error {
	if it == nil {
		return fmt.Errorf("numeric doc values field updates: iterator must not be nil")
	}
	return n.AddLong(doc, it.LongValue())
}

// swap is the HookSwap callback. It first delegates to SwapBase to keep
// the docs storage in sync, then swaps the numeric values array.
func (n *NumericDocValuesFieldUpdates) swap(i, j int) {
	n.SwapBase(i, j)

	tmpVal := n.values.Get(int64(j))
	n.values.Set(int64(j), n.values.Get(int64(i)))
	n.values.Set(int64(i), tmpVal)
}

// grow is the HookGrow callback. It first delegates to GrowBase, then
// grows the values array.
func (n *NumericDocValuesFieldUpdates) grow(size int) {
	n.GrowBase(size)
	n.values = n.values.Grow(int64(size))
}

// resize is the HookResize callback. Mirrors the parallel logic in grow
// for the trim path.
func (n *NumericDocValuesFieldUpdates) resize(size int) {
	n.ResizeBase(size)
	n.values = n.values.Resize(int64(size))
}

// Iterator returns a fresh iterator over the packet's numeric updates.
// The packet MUST have been finished first; Iterator panics otherwise
// (via EnsureFinished), matching the Java contract.
//
// The returned iterator shares the underlying values storage with the
// packet, mirroring Java's {@code values.get()} call — callers must not
// mutate the packet while an iterator is live.
func (n *NumericDocValuesFieldUpdates) Iterator() DocValuesFieldUpdatesIterator {
	n.EnsureFinished()
	it := &numericDocValuesFieldUpdatesIterator{
		values:   n.values,
		minValue: n.minValue,
	}
	InitBaseDocValuesFieldUpdatesIterator(&it.BaseDocValuesFieldUpdatesIterator, n.Size, n.Docs, n.DelGen())
	it.SetIdx = it.setIdx
	return it
}

// RamBytesUsed reports an approximate footprint of the packet,
// including the base accounting and the numeric-specific extras.
// Mirrors {@code NumericDocValuesFieldUpdates#ramBytesUsed()} with the
// same best-effort caveats as [BaseDocValuesFieldUpdates.RamBytesUsedBase].
func (n *NumericDocValuesFieldUpdates) RamBytesUsed() int64 {
	const longBytes = 8
	return n.values.RamBytesUsed() +
		n.RamBytesUsedBase() +
		int64(longBytes) +
		int64(util.NumBytesObjectRef)
}

// numericDocValuesFieldUpdatesIterator is the iterator returned by
// [NumericDocValuesFieldUpdates.Iterator]. It embeds the shared
// iterator base and provides the LongValue/BinaryValue overrides.
type numericDocValuesFieldUpdatesIterator struct {
	BaseDocValuesFieldUpdatesIterator
	values   *packed.AbstractPagedMutable
	minValue int64
	value    int64
}

// LongValue returns the long value for the current entry, already
// rebased by minValue. Mirrors {@code Iterator#longValue()}.
func (it *numericDocValuesFieldUpdatesIterator) LongValue() int64 {
	return it.value
}

// BinaryValue panics: numeric iterators do not expose binary values.
// Mirrors {@code NumericDocValuesFieldUpdates.Iterator#binaryValue()}.
func (it *numericDocValuesFieldUpdatesIterator) BinaryValue() *util.BytesRef {
	panic("numeric doc values field updates: iterator has no binary value")
}

// setIdx is wired into the base iterator as the SetIdx hook so that
// every NextDoc that lands on a value-bearing entry refreshes the
// cached, rebased long value. Mirrors {@code Iterator#set(long)}.
func (it *numericDocValuesFieldUpdatesIterator) setIdx(idx int64) {
	it.value = it.values.Get(idx) + it.minValue
}

// SingleValueNumericDocValuesFieldUpdates is a space-optimised packet
// that stores a single shared long value for every updated doc, backed
// by a SparseFixedBitSet rather than a per-doc values array. Mirrors
// the Java nested static class of the same name.
//
// It is created by the buffered-updates machinery when every doc in a
// packet receives the identical value; Add asserts that contract.
type SingleValueNumericDocValuesFieldUpdates struct {
	BaseDocValuesFieldUpdates

	value            int64
	bitSet           *util.SparseFixedBitSet
	hasNoValue       *util.SparseFixedBitSet
	hasAtLeastOneVal bool
}

// NewSingleValueNumericDocValuesFieldUpdates creates a single-value
// packet for the given field at the given delete generation. Every doc
// added later must carry exactly value. Mirrors the Java constructor.
func NewSingleValueNumericDocValuesFieldUpdates(delGen int64, field string, maxDoc int, value int64) (*SingleValueNumericDocValuesFieldUpdates, error) {
	s := &SingleValueNumericDocValuesFieldUpdates{}
	if err := InitBaseDocValuesFieldUpdates(&s.BaseDocValuesFieldUpdates, maxDoc, delGen, field, DocValuesTypeNumeric); err != nil {
		return nil, err
	}
	bitSet, err := util.NewSparseFixedBitSet(maxDoc)
	if err != nil {
		return nil, fmt.Errorf("single value numeric doc values field updates: bit set: %w", err)
	}
	s.bitSet = bitSet
	s.value = value
	return s, nil
}

// LongValue returns the shared value carried by every doc in the
// packet. Package-private in Java, exported here for testing parity.
func (s *SingleValueNumericDocValuesFieldUpdates) LongValue() int64 { return s.value }

// AddLong records doc as carrying the packet's shared value. It is an
// error to pass a value other than the one the packet was created
// with, mirroring the Java {@code assert this.value == value}.
func (s *SingleValueNumericDocValuesFieldUpdates) AddLong(doc int, value int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if value != s.value {
		return fmt.Errorf("single value numeric doc values field updates: value %d does not match packet value %d", value, s.value)
	}
	if doc < 0 || doc >= s.maxDoc {
		return fmt.Errorf("single value numeric doc values field updates: doc %d out of range [0,%d)", doc, s.maxDoc)
	}
	s.bitSet.Set(doc)
	s.hasAtLeastOneVal = true
	if s.hasNoValue != nil {
		s.hasNoValue.Clear(doc)
	}
	return nil
}

// AddBinary is unsupported on a numeric packet. Mirrors the Java
// {@code add(int, BytesRef)} which throws UnsupportedOperationException.
func (s *SingleValueNumericDocValuesFieldUpdates) AddBinary(doc int, value *util.BytesRef) error {
	return fmt.Errorf("single value numeric doc values field updates: AddBinary unsupported")
}

// AddFromIterator is unsupported on a single-value packet. Mirrors the
// Java {@code add(int, Iterator)} which throws
// UnsupportedOperationException.
func (s *SingleValueNumericDocValuesFieldUpdates) AddFromIterator(doc int, it DocValuesFieldUpdatesIterator) error {
	return fmt.Errorf("single value numeric doc values field updates: AddFromIterator unsupported")
}

// Reset records an explicit "clear this doc" update: doc is marked as
// present in the packet but flagged as carrying no value. Mirrors the
// Java {@code synchronized void reset(int)} override.
func (s *SingleValueNumericDocValuesFieldUpdates) Reset(doc int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if doc < 0 || doc >= s.maxDoc {
		return fmt.Errorf("single value numeric doc values field updates: doc %d out of range [0,%d)", doc, s.maxDoc)
	}
	s.bitSet.Set(doc)
	s.hasAtLeastOneVal = true
	if s.hasNoValue == nil {
		hasNoValue, err := util.NewSparseFixedBitSet(s.maxDoc)
		if err != nil {
			return fmt.Errorf("single value numeric doc values field updates: hasNoValue bit set: %w", err)
		}
		s.hasNoValue = hasNoValue
	}
	s.hasNoValue.Set(doc)
	return nil
}

// Any reports whether the packet has any updates. It augments the base
// check with the single-value bookkeeping. Mirrors the Java
// {@code synchronized boolean any()} override.
func (s *SingleValueNumericDocValuesFieldUpdates) Any() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Size > 0 || s.hasAtLeastOneVal
}

// RamBytesUsed reports an approximate footprint of the packet. Mirrors
// {@code SingleValueNumericDocValuesFieldUpdates#ramBytesUsed()}.
func (s *SingleValueNumericDocValuesFieldUpdates) RamBytesUsed() int64 {
	bytes := s.RamBytesUsedBase() + s.bitSet.RamBytesUsed()
	if s.hasNoValue != nil {
		bytes += s.hasNoValue.RamBytesUsed()
	}
	return bytes
}

// Iterator returns a fresh iterator over the packet's updated docs, in
// increasing doc-id order, every entry exposing the shared value.
// Mirrors the anonymous {@code DocValuesFieldUpdates.Iterator} returned
// by the Java {@code iterator()} override.
//
// Unlike the multi-value packets this iterator does not require
// Finish() to have been called, matching Lucene which builds it
// straight off the bit set.
func (s *SingleValueNumericDocValuesFieldUpdates) Iterator() DocValuesFieldUpdatesIterator {
	return &singleValueNumericDocValuesFieldUpdatesIterator{
		inner:      util.NewBitSetIterator(s.bitSet, int64(s.maxDoc)),
		value:      s.value,
		delGen:     s.DelGen(),
		hasNoValue: s.hasNoValue,
	}
}

// singleValueNumericDocValuesFieldUpdatesIterator walks the packet's
// SparseFixedBitSet and surfaces the shared value for each set bit.
type singleValueNumericDocValuesFieldUpdatesIterator struct {
	inner      *util.BitSetIterator
	value      int64
	delGen     int64
	hasNoValue *util.SparseFixedBitSet
}

// NextDoc advances to the next updated doc id, or [util.NO_MORE_DOCS]
// when the bit set is exhausted.
func (it *singleValueNumericDocValuesFieldUpdatesIterator) NextDoc() int {
	doc, _ := it.inner.NextDoc()
	return doc
}

// DocID returns the current doc id, -1 before the first NextDoc, or
// [util.NO_MORE_DOCS] once exhausted.
func (it *singleValueNumericDocValuesFieldUpdatesIterator) DocID() int {
	return it.inner.DocID()
}

// LongValue returns the packet's shared value.
func (it *singleValueNumericDocValuesFieldUpdatesIterator) LongValue() int64 {
	return it.value
}

// BinaryValue panics: this is a numeric packet. Mirrors the anonymous
// iterator's {@code binaryValue()} which throws
// UnsupportedOperationException.
func (it *singleValueNumericDocValuesFieldUpdatesIterator) BinaryValue() *util.BytesRef {
	panic("single value numeric doc values field updates: iterator has no binary value")
}

// DelGen returns the packet's delete generation.
func (it *singleValueNumericDocValuesFieldUpdatesIterator) DelGen() int64 {
	return it.delGen
}

// HasValue reports whether the current doc carries the shared value
// (true) or was cleared by Reset (false). Mirrors the anonymous
// iterator's {@code hasValue()}.
func (it *singleValueNumericDocValuesFieldUpdatesIterator) HasValue() bool {
	if it.hasNoValue != nil {
		return !it.hasNoValue.Get(it.DocID())
	}
	return true
}

// Compile-time checks that both iterator types satisfy the contract.
var (
	_ DocValuesFieldUpdatesIterator = (*numericDocValuesFieldUpdatesIterator)(nil)
	_ DocValuesFieldUpdatesIterator = (*singleValueNumericDocValuesFieldUpdatesIterator)(nil)
)
