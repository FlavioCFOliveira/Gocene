// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

package bkd

import (
	"bytes"

	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

// MutablePointTree captures the methods that
// MutablePointTreeReaderUtils calls on a mutable view of buffered
// points.
//
// This interface is the local minimum surface required by the sort /
// sortByDim / partition helpers. It is a temporary shim: the canonical
// type in Lucene is org.apache.lucene.codecs.MutablePointTree (an inner
// type of PointValues), and once the broader PointValues port lands
// this interface will be unified with — or replaced by — the central
// definition. Until then, it lives here so that
// MutablePointTreeReaderUtils can compile and be tested in isolation
// from the not-yet-ported PointValues/codec surface.
//
// Method contracts mirror the Java reference (Lucene 10.4.0):
//   - Swap(i, j): exchange the entries at slots i and j.
//   - GetValue(i, dst): fill dst with the packed value at slot i; the
//     implementation typically aliases the underlying byte storage
//     rather than copying.
//   - GetByteAt(i, k): return the k-th byte (as a signed Go byte) of
//     the packed value at slot i.
//   - GetDocID(i): return the docID associated with the point at slot i.
//   - Save(i, j) / Restore(i, j): scratch-storage hooks used by
//     StableMSBRadixSorter; semantics match
//     [util.StableMSBRadixSorterImpl].
type MutablePointTree interface {
	// Swap exchanges the entries at slots i and j.
	Swap(i, j int)
	// GetValue fills dst with the packed value at slot i. The
	// implementation is allowed to alias its underlying storage.
	GetValue(i int, dst *util.BytesRef)
	// GetByteAt returns the k-th byte of the packed value at slot i.
	GetByteAt(i, k int) byte
	// GetDocID returns the docID associated with the point at slot i.
	GetDocID(i int) int
	// Save writes the value at slot i into the j-th scratch position.
	Save(i, j int)
	// Restore copies the scratch values back into slots [i, j) of the
	// primary storage.
	Restore(i, j int)
}

// SortMutablePointTree sorts the given MutablePointTree first by packed
// value then by doc ID. Port of
// org.apache.lucene.util.bkd.MutablePointTreeReaderUtils.sort.
//
// The implementation matches the Java reference: if the [from, to)
// range is already ordered by docID it skips the docID tie-breaker
// bytes (the radix sort is stable, so equal-key entries preserve their
// input order); otherwise it appends ceil(bitsPerDocID/8) bytes of
// docID to the radix key.
func SortMutablePointTree(config BKDConfig, maxDoc int, reader MutablePointTree, from, to int) {
	sortedByDocID := true
	prevDoc := 0
	for i := from; i < to; i++ {
		doc := reader.GetDocID(i)
		if doc < prevDoc {
			sortedByDocID = false
			break
		}
		prevDoc = doc
	}

	// No need to tie break on doc IDs if already sorted by doc ID, since we use a stable sort.
	bitsPerDocID := 0
	if !sortedByDocID {
		bitsPerDocID = packed.UnsignedBitsRequired(uint64(maxDoc - 1))
	}

	packedBytesLen := config.PackedBytesLength()
	maxLength := packedBytesLen + (bitsPerDocID+7)/8

	adapter := &mptSortAdapter{
		reader:         reader,
		packedBytesLen: packedBytesLen,
		bitsPerDocID:   bitsPerDocID,
	}
	util.NewStableMSBRadixSorter(adapter, maxLength).Sort(from, to)
}

// SortMutablePointTreeByDim sorts points on the given dimension. Port of
// org.apache.lucene.util.bkd.MutablePointTreeReaderUtils.sortByDim.
//
// The commonPrefixLengths slice is accepted for parity with the Java
// signature even though the Java implementation does not use it (the
// argument was already vestigial there). It is retained so callers
// share a single signature.
//
// scratch1 and scratch2 are caller-provided reusable [util.BytesRef]
// buffers; this avoids per-call allocations on the BKD hot path. They
// may be empty (zero value) [util.BytesRef] instances.
func SortMutablePointTreeByDim(
	config BKDConfig,
	sortedDim int,
	_ []int, // commonPrefixLengths - kept for parity with Lucene; unused.
	reader MutablePointTree,
	from, to int,
	scratch1, scratch2 *util.BytesRef,
) {
	start := sortedDim * config.BytesPerDim()
	end := start + config.BytesPerDim()
	dataDimOffset := config.PackedIndexBytesLength()
	dataDimEnd := config.PackedBytesLength()

	adapter := &mptSortByDimAdapter{
		reader:        reader,
		pivot:         scratch1,
		scratch:       scratch2,
		start:         start,
		end:           end,
		dataDimOffset: dataDimOffset,
		dataDimEnd:    dataDimEnd,
	}
	// No need for a fancy radix sort here: this is called on the leaves
	// only so there are not many values to sort.
	util.NewIntroSorter(adapter).Sort(from, to)
}

// PartitionMutablePointTree partitions points around mid. All values on
// the left must be less than or equal to it and all values on the right
// must be greater than or equal to it. Port of
// org.apache.lucene.util.bkd.MutablePointTreeReaderUtils.partition.
//
// scratch1 and scratch2 are accepted for parity with the Java signature
// (Lucene uses them inside its custom IntroSelector fallback). Our
// [util.RadixSelector] currently uses its own built-in fallback that
// reads via [util.RadixSelectorInterface.ByteAt], so these scratch
// buffers are not consulted; they are retained on the API to avoid
// changing the signature when a fallback-override hook is added to
// util.RadixSelector. See the parity note on
// [mptPartitionAdapter.ByteAt] for the correctness argument.
func PartitionMutablePointTree(
	config BKDConfig,
	maxDoc int,
	splitDim int,
	commonPrefixLen int,
	reader MutablePointTree,
	from, to, mid int,
	_, _ *util.BytesRef,
) {
	bytesPerDim := config.BytesPerDim()
	packedIndexBytesLen := config.PackedIndexBytesLength()
	numDataDimsBytes := (config.NumDims() - config.NumIndexDims()) * bytesPerDim

	dimOffset := splitDim*bytesPerDim + commonPrefixLen
	dimCmpBytes := bytesPerDim - commonPrefixLen
	dataCmpBytes := numDataDimsBytes + dimCmpBytes
	bitsPerDocID := packed.UnsignedBitsRequired(uint64(maxDoc - 1))
	maxLength := dataCmpBytes + (bitsPerDocID+7)/8

	adapter := &mptPartitionAdapter{
		reader:              reader,
		packedIndexBytesLen: packedIndexBytesLen,
		dimOffset:           dimOffset,
		dimCmpBytes:         dimCmpBytes,
		dataCmpBytes:        dataCmpBytes,
		bitsPerDocID:        bitsPerDocID,
	}
	util.NewRadixSelector(adapter, maxLength).Select(from, to, mid)
}

// ---------------------------------------------------------------------
// Sort adapter: drives StableMSBRadixSorter for SortMutablePointTree.
// ---------------------------------------------------------------------

// mptSortAdapter satisfies [util.StableMSBRadixSorterImpl]. The byte
// stream exposed by ByteAt is:
//   - bytes [0, packedBytesLen) -> packed value bytes via reader.GetByteAt;
//   - bytes [packedBytesLen, packedBytesLen + ceil(bitsPerDocID/8)) ->
//     big-endian high-order bytes of the docID, with the low end of the
//     trailing byte zero-padded so that the comparison stays correct
//     even when the docID is shorter than a full byte (e.g. maxDoc=1).
type mptSortAdapter struct {
	reader         MutablePointTree
	packedBytesLen int
	bitsPerDocID   int
}

// ByteAt returns the k-th byte of the radix key for slot i.
func (a *mptSortAdapter) ByteAt(i, k int) int {
	if k < a.packedBytesLen {
		return int(a.reader.GetByteAt(i, k)) & 0xFF
	}
	// Big-endian docID bytes, with the trailing byte zero-padded to the
	// right when bitsPerDocID is not a multiple of 8 — matches the Java
	// "shift = bitsPerDocId - ((k - packedBytesLength + 1) << 3)" plus
	// Math.max(0, shift).
	shift := a.bitsPerDocID - ((k - a.packedBytesLen + 1) << 3)
	if shift < 0 {
		shift = 0
	}
	return (a.reader.GetDocID(i) >> uint(shift)) & 0xFF
}

// Swap delegates to the underlying reader.
func (a *mptSortAdapter) Swap(i, j int) { a.reader.Swap(i, j) }

// Save delegates to the underlying reader for the stable radix sorter
// scratch storage.
func (a *mptSortAdapter) Save(i, j int) { a.reader.Save(i, j) }

// Restore delegates to the underlying reader for the stable radix
// sorter scratch storage.
func (a *mptSortAdapter) Restore(i, j int) { a.reader.Restore(i, j) }

// ---------------------------------------------------------------------
// SortByDim adapter: drives IntroSorter for SortMutablePointTreeByDim.
// ---------------------------------------------------------------------

// mptSortByDimAdapter satisfies [util.IntroSorterInterface]. It pivots
// on a single point and compares two scratch BytesRef refs through the
// reader.GetValue alias.
type mptSortByDimAdapter struct {
	reader        MutablePointTree
	pivot         *util.BytesRef
	scratch       *util.BytesRef
	start         int
	end           int
	dataDimOffset int
	dataDimEnd    int
	pivotDoc      int
}

// Swap delegates to the underlying reader.
func (a *mptSortByDimAdapter) Swap(i, j int) { a.reader.Swap(i, j) }

// SetPivot remembers the value (via GetValue alias) and docID of slot i.
func (a *mptSortByDimAdapter) SetPivot(i int) {
	a.reader.GetValue(i, a.pivot)
	a.pivotDoc = a.reader.GetDocID(i)
}

// ComparePivot compares the cached pivot against slot j using the
// indexed-dim primary key, the data-dim tie-breaker, and finally the
// docID. Mirrors the Java IntroSorter.comparePivot in
// MutablePointTreeReaderUtils.sortByDim.
func (a *mptSortByDimAdapter) ComparePivot(j int) int {
	a.reader.GetValue(j, a.scratch)
	cmp := bytes.Compare(
		a.pivot.Bytes[a.pivot.Offset+a.start:a.pivot.Offset+a.end],
		a.scratch.Bytes[a.scratch.Offset+a.start:a.scratch.Offset+a.end],
	)
	if cmp != 0 {
		return cmp
	}
	cmp = bytes.Compare(
		a.pivot.Bytes[a.pivot.Offset+a.dataDimOffset:a.pivot.Offset+a.dataDimEnd],
		a.scratch.Bytes[a.scratch.Offset+a.dataDimOffset:a.scratch.Offset+a.dataDimEnd],
	)
	if cmp != 0 {
		return cmp
	}
	return a.pivotDoc - a.reader.GetDocID(j)
}

// Compare is required by [util.SorterInterface]; for IntroSorter the
// dispatcher routes through SetPivot/ComparePivot when the impl also
// satisfies [util.IntroSorterInterface], so this method is unreachable.
func (a *mptSortByDimAdapter) Compare(i, j int) int {
	a.SetPivot(i)
	return a.ComparePivot(j)
}

// Sort is required by [util.SorterInterface] but the IntroSorter
// dispatcher drives the algorithm directly, so this method is
// unreachable.
func (a *mptSortByDimAdapter) Sort(_, _ int) {
	panic("mptSortByDimAdapter.Sort: unreachable; use util.IntroSorter.Sort")
}

// ---------------------------------------------------------------------
// Partition adapter: drives RadixSelector for PartitionMutablePointTree.
// ---------------------------------------------------------------------

// mptPartitionAdapter satisfies [util.RadixSelectorInterface]. The byte
// stream consumed by the radix selector is:
//   - bytes [0, dimCmpBytes) -> trailing bytes of the split dimension
//     starting at the common prefix offset;
//   - bytes [dimCmpBytes, dataCmpBytes) -> data-dim bytes;
//   - bytes [dataCmpBytes, dataCmpBytes + ceil(bitsPerDocID/8)) ->
//     big-endian high-order bytes of the docID, padded as in
//     [mptSortAdapter].
type mptPartitionAdapter struct {
	reader              MutablePointTree
	packedIndexBytesLen int
	dimOffset           int
	dimCmpBytes         int
	dataCmpBytes        int
	bitsPerDocID        int
}

// Swap delegates to the underlying reader.
func (a *mptPartitionAdapter) Swap(i, j int) { a.reader.Swap(i, j) }

// ByteAt returns the k-th byte of the radix key for slot i.
//
// Parity note on the fallback selector: Lucene's RadixSelector exposes a
// protected getFallbackSelector hook, and MutablePointTreeReaderUtils
// overrides it with an IntroSelector that compares by split-dim suffix,
// then data-dim bytes, then docID. Our [util.RadixSelector] does not
// currently expose such a hook; it falls back to a built-in
// comparison-based selector (radixFallbackImpl) that consults the same
// [util.RadixSelectorInterface.ByteAt] stream byte-by-byte. Because the
// byte stream is laid out as <dim suffix> | <data dims> | <docID big-
// endian high bytes>, lexicographic comparison over that stream
// produces the same total ordering as the Java fallback: any tie on the
// split-dim suffix forces continuation into the data-dim bytes, and any
// further tie forces continuation into the docID bytes. The Java
// fallback's structure is therefore an optimisation, not a behaviour
// difference; correctness is preserved by relying on the built-in
// fallback. Should util.RadixSelector grow a getFallbackSelector hook,
// this adapter is the right place to plug a custom selector in.
func (a *mptPartitionAdapter) ByteAt(i, k int) int {
	switch {
	case k < a.dimCmpBytes:
		return int(a.reader.GetByteAt(i, a.dimOffset+k)) & 0xFF
	case k < a.dataCmpBytes:
		return int(a.reader.GetByteAt(i, a.packedIndexBytesLen+k-a.dimCmpBytes)) & 0xFF
	default:
		shift := a.bitsPerDocID - ((k - a.dataCmpBytes + 1) << 3)
		if shift < 0 {
			shift = 0
		}
		return (a.reader.GetDocID(i) >> uint(shift)) & 0xFF
	}
}
