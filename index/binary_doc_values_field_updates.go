// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"sync"

	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

// Port note:
//
// This file is the Go port of
// lucene/core/src/java/org/apache/lucene/index/BinaryDocValuesFieldUpdates.java
// (Apache Lucene 10.5.0). It handles updates to binary doc values.
//
// Translation only scope: faithful Java→Go conversion.

// BinaryDocValuesFieldUpdates holds updates for a single binary doc values field.
// Mirrors the Java {@code BinaryDocValuesFieldUpdates} class.
type BinaryDocValuesFieldUpdates struct {
	BaseDocValuesFieldUpdates

	mu      sync.Mutex
	offsets *packed.AbstractPagedMutable
	lengths *packed.AbstractPagedMutable
	values  *util.BytesRefBuilder
}

// NewBinaryDocValuesFieldUpdates initialises a new BinaryDocValuesFieldUpdates packet.
func NewBinaryDocValuesFieldUpdates(delGen int64, field string, maxDoc int) *BinaryDocValuesFieldUpdates {
	b := &BinaryDocValuesFieldUpdates{}
	if err := InitBaseDocValuesFieldUpdates(&b.BaseDocValuesFieldUpdates, maxDoc, delGen, field, DocValuesTypeBinary); err != nil {
		panic(err)
	}

	offsets, _ := packed.NewPagedGrowableWriter(1, docValuesFieldUpdatesPageSize, 1, packed.Fast)
	lengths, _ := packed.NewPagedGrowableWriter(1, docValuesFieldUpdatesPageSize, 1, packed.Fast)
	b.offsets = offsets.AbstractPagedMutable
	b.lengths = lengths.AbstractPagedMutable
	b.values = util.NewBytesRefBuilder()

	b.HookSwap = b.swap
	b.HookGrow = b.grow
	b.HookResize = b.resize

	return b
}

// AddBinary records a binary value update for the given doc.
// Mirrors {@code BinaryDocValuesFieldUpdates#add(int, BytesRef)}.
func (b *BinaryDocValuesFieldUpdates) AddBinary(doc int, value *util.BytesRef) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	index, err := b.AddDoc(doc)
	if err != nil {
		return err
	}

	b.offsets.Set(int64(index), int64(b.values.Length()))
	b.lengths.Set(int64(index), int64(value.Length))
	b.values.Append(value)

	return nil
}

// AddIterator records a binary value update from an iterator.
// Mirrors {@code BinaryDocValuesFieldUpdates#add(int, Iterator)}.
func (b *BinaryDocValuesFieldUpdates) AddIterator(doc int, iterator DocValuesFieldUpdatesIterator) error {
	return b.AddBinary(doc, iterator.BinaryValue())
}

// AddLong reports an unsupported operation for binary fields.
// Mirrors {@code BinaryDocValuesFieldUpdates#add(int, long)}.
func (b *BinaryDocValuesFieldUpdates) AddLong(doc int, value int64) {
	panic("binary doc values field updates: add(int, long) is unsupported")
}

func (b *BinaryDocValuesFieldUpdates) swap(i, j int) {
	b.SwapBase(i, j)

	tmpOffset := b.offsets.Get(int64(j))
	b.offsets.Set(int64(j), b.offsets.Get(int64(i)))
	b.offsets.Set(int64(i), tmpOffset)

	tmpLength := b.lengths.Get(int64(j))
	b.lengths.Set(int64(j), b.lengths.Get(int64(i)))
	b.lengths.Set(int64(i), tmpLength)
}

func (b *BinaryDocValuesFieldUpdates) grow(size int) {
	b.GrowBase(size)
	b.offsets = b.offsets.Grow(int64(size))
	b.lengths = b.lengths.Grow(int64(size))
}

func (b *BinaryDocValuesFieldUpdates) resize(size int) {
	b.ResizeBase(size)
	b.offsets = b.offsets.Resize(int64(size))
	b.lengths = b.lengths.Resize(int64(size))
}

// Iterator returns an iterator over the updates.
func (b *BinaryDocValuesFieldUpdates) Iterator() DocValuesFieldUpdatesIterator {
	b.EnsureFinished()
	return NewBinaryDocValuesFieldUpdatesIterator(b.Size, b.Docs, b.DelGen(), b.offsets, b.lengths, util.NewBytesRef(b.values.Bytes()))
}

// RamBytesUsed reports the total RAM footprint of this packet.
func (b *BinaryDocValuesFieldUpdates) RamBytesUsed() int64 {
	const objectHeader = 16
	const intBytes = 4
	const longBytes = 8

	return b.RamBytesUsedBase() +
		b.offsets.RamBytesUsed() +
		b.lengths.RamBytesUsed() +
		int64(objectHeader) +
		2*int64(intBytes) +
		3*int64(util.NumBytesObjectRef) +
		int64(len(b.values.Bytes()))
}

// BinaryDocValuesFieldUpdatesIterator iterates over binary updates.
type BinaryDocValuesFieldUpdatesIterator struct {
	BaseDocValuesFieldUpdatesIterator

	offsets *packed.AbstractPagedMutable
	lengths *packed.AbstractPagedMutable
	value   *util.BytesRef
	offset  int
	length  int
}

// NewBinaryDocValuesFieldUpdatesIterator initialises a new binary update iterator.
func NewBinaryDocValuesFieldUpdatesIterator(size int, docs *packed.AbstractPagedMutable, delGen int64, offsets, lengths *packed.AbstractPagedMutable, values *util.BytesRef) DocValuesFieldUpdatesIterator {
	it := &BinaryDocValuesFieldUpdatesIterator{
		offsets: offsets,
		lengths: lengths,
		value:   util.NewBytesRef(values.ValidBytes()),
	}
	InitBaseDocValuesFieldUpdatesIterator(&it.BaseDocValuesFieldUpdatesIterator, size, docs, delGen)

	it.SetIdx = func(idx int64) {
		it.offset = int(it.offsets.Get(idx))
		it.length = int(it.lengths.Get(idx))
	}

	return it
}

// BinaryValue returns the binary value for the current doc.
func (it *BinaryDocValuesFieldUpdatesIterator) BinaryValue() *util.BytesRef {
	it.value.Offset = it.offset
	it.value.Length = it.length
	return it.value
}

// LongValue reports an unsupported operation for binary iterators.
func (it *BinaryDocValuesFieldUpdatesIterator) LongValue() int64 {
	panic("binary doc values field updates iterator: longValue() is unsupported")
}
