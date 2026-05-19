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
// lucene/core/src/java/org/apache/lucene/index/BinaryDocValuesFieldUpdates.java
// (Apache Lucene 10.4.0). The shared per-(field, delGen) state lives
// in [BaseDocValuesFieldUpdates]; only the binary-specific
// offsets/lengths/values storage and the matching iterator are
// here. The HookSwap / HookGrow / HookResize callbacks installed by
// the constructor mirror Java's virtual dispatch from the abstract
// parent into BinaryDocValuesFieldUpdates.

// BinaryDocValuesFieldUpdates accumulates a packet of binary
// doc-values updates for a single field within one segment. Add(),
// Reset() and Finish() are safe for concurrent use through the
// embedded [BaseDocValuesFieldUpdates] mutex; Iterator() is not, and
// callers MUST call Finish() before iterating, matching Lucene.
type BinaryDocValuesFieldUpdates struct {
	BaseDocValuesFieldUpdates

	offsets *packed.PagedGrowableWriter
	lengths *packed.PagedGrowableWriter
	values  *util.BytesRefBuilder
}

// NewBinaryDocValuesFieldUpdates creates a fresh, empty packet for
// the given field at the given delete generation. maxDoc is the
// segment-wide maxDoc and bounds the doc ids accepted by Add and
// Reset.
//
// Returns an error when the underlying PagedMutable allocations
// fail, which in practice can only happen for invalid maxDoc.
func NewBinaryDocValuesFieldUpdates(delGen int64, field string, maxDoc int) (*BinaryDocValuesFieldUpdates, error) {
	b := &BinaryDocValuesFieldUpdates{}
	if err := InitBaseDocValuesFieldUpdates(&b.BaseDocValuesFieldUpdates, maxDoc, delGen, field, DocValuesTypeBinary); err != nil {
		return nil, err
	}
	offsets, err := packed.NewPagedGrowableWriter(1, docValuesFieldUpdatesPageSize, 1, packed.Fast)
	if err != nil {
		return nil, fmt.Errorf("binary doc values field updates: offsets: %w", err)
	}
	lengths, err := packed.NewPagedGrowableWriter(1, docValuesFieldUpdatesPageSize, 1, packed.Fast)
	if err != nil {
		return nil, fmt.Errorf("binary doc values field updates: lengths: %w", err)
	}
	b.offsets = offsets
	b.lengths = lengths
	b.values = &util.BytesRefBuilder{}

	b.HookSwap = b.swap
	b.HookGrow = b.grow
	b.HookResize = b.resize
	return b, nil
}

// AddLong is unsupported on a binary packet. Mirrors the Java
// {@code BinaryDocValuesFieldUpdates#add(int, long)} which throws
// UnsupportedOperationException.
func (b *BinaryDocValuesFieldUpdates) AddLong(doc int, value int64) error {
	return fmt.Errorf("binary doc values field updates: AddLong unsupported")
}

// AddBinary records a binary update for doc. The value bytes are
// copied into the internal builder; the caller may mutate the input
// after the call returns. Mirrors {@code add(int, BytesRef)}.
func (b *BinaryDocValuesFieldUpdates) AddBinary(doc int, value *util.BytesRef) error {
	if value == nil {
		return fmt.Errorf("binary doc values field updates: value must not be nil")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	index, err := b.addInternalLocked(doc, docValuesFieldUpdatesHasValueMask)
	if err != nil {
		return err
	}
	b.offsets.Set(int64(index), int64(b.values.Length()))
	b.lengths.Set(int64(index), int64(value.Length))
	b.values.AppendBytesRef(value)
	return nil
}

// AddFromIterator copies the binary value the iterator currently
// exposes into a new entry for doc. Mirrors
// {@code add(int, DocValuesFieldUpdates.Iterator)}.
func (b *BinaryDocValuesFieldUpdates) AddFromIterator(doc int, it DocValuesFieldUpdatesIterator) error {
	if it == nil {
		return fmt.Errorf("binary doc values field updates: iterator must not be nil")
	}
	return b.AddBinary(doc, it.BinaryValue())
}

// swap is the HookSwap callback. It first delegates to SwapBase to
// keep the docs storage in sync, then swaps the binary-specific
// offsets and lengths arrays.
func (b *BinaryDocValuesFieldUpdates) swap(i, j int) {
	b.SwapBase(i, j)

	tmpOffset := b.offsets.Get(int64(j))
	b.offsets.Set(int64(j), b.offsets.Get(int64(i)))
	b.offsets.Set(int64(i), tmpOffset)

	tmpLength := b.lengths.Get(int64(j))
	b.lengths.Set(int64(j), b.lengths.Get(int64(i)))
	b.lengths.Set(int64(i), tmpLength)
}

// grow is the HookGrow callback. It first delegates to GrowBase,
// then grows the offsets and lengths arrays.
func (b *BinaryDocValuesFieldUpdates) grow(size int) {
	b.GrowBase(size)
	b.offsets.AbstractPagedMutable = b.offsets.Grow(int64(size))
	b.lengths.AbstractPagedMutable = b.lengths.Grow(int64(size))
}

// resize is the HookResize callback. Mirrors the parallel logic in
// grow for the trim path.
func (b *BinaryDocValuesFieldUpdates) resize(size int) {
	b.ResizeBase(size)
	b.offsets.AbstractPagedMutable = b.offsets.Resize(int64(size))
	b.lengths.AbstractPagedMutable = b.lengths.Resize(int64(size))
}

// Iterator returns a fresh iterator over the packet's binary
// updates. The packet MUST have been finished first; Iterator
// panics otherwise (via EnsureFinished), matching the Java contract.
//
// The returned iterator shares the underlying values storage with
// the packet, mirroring Java's {@code values.get()} call — callers
// must not mutate the packet while an iterator is live.
func (b *BinaryDocValuesFieldUpdates) Iterator() DocValuesFieldUpdatesIterator {
	b.EnsureFinished()
	it := &binaryDocValuesFieldUpdatesIterator{
		offsets: b.offsets,
		lengths: b.lengths,
		value:   b.values.Get().ShallowClone(),
	}
	InitBaseDocValuesFieldUpdatesIterator(&it.BaseDocValuesFieldUpdatesIterator, b.Size, b.Docs, b.DelGen())
	it.SetIdx = it.setIdx
	return it
}

// RamBytesUsed reports an approximate footprint of the packet,
// including the base accounting and the binary-specific extras.
// Mirrors {@code BinaryDocValuesFieldUpdates#ramBytesUsed()} with
// the same best-effort caveats as [BaseDocValuesFieldUpdates.RamBytesUsedBase].
func (b *BinaryDocValuesFieldUpdates) RamBytesUsed() int64 {
	const objectHeader = 16
	const intBytes = 4
	bytes := b.RamBytesUsedBase() +
		b.offsets.RamBytesUsed() +
		b.lengths.RamBytesUsed() +
		int64(objectHeader) +
		2*int64(intBytes) +
		3*int64(util.NumBytesObjectRef) +
		int64(len(b.values.Bytes()))
	return bytes
}

// binaryDocValuesFieldUpdatesIterator is the iterator returned by
// [BinaryDocValuesFieldUpdates.Iterator]. It embeds the shared
// iterator base and provides the BinaryValue/LongValue overrides.
type binaryDocValuesFieldUpdatesIterator struct {
	BaseDocValuesFieldUpdatesIterator
	offsets *packed.PagedGrowableWriter
	lengths *packed.PagedGrowableWriter
	value   *util.BytesRef
	offset  int
	length  int
}

// BinaryValue refreshes the iterator's view over the shared values
// buffer to the current entry and returns the ref. The returned ref
// is owned by the iterator and must not be retained across NextDoc
// calls, matching the Java contract.
func (it *binaryDocValuesFieldUpdatesIterator) BinaryValue() *util.BytesRef {
	it.value.Offset = it.offset
	it.value.Length = it.length
	return it.value
}

// LongValue panics: binary iterators do not expose long values.
// Mirrors {@code BinaryDocValuesFieldUpdates.Iterator#longValue()}.
func (it *binaryDocValuesFieldUpdatesIterator) LongValue() int64 {
	panic("binary doc values field updates: iterator has no long value")
}

// setIdx is wired into the base iterator as the SetIdx hook so that
// every NextDoc that lands on a value-bearing entry refreshes the
// cached offset and length.
func (it *binaryDocValuesFieldUpdatesIterator) setIdx(idx int64) {
	it.offset = int(it.offsets.Get(idx))
	it.length = int(it.lengths.Get(idx))
}
