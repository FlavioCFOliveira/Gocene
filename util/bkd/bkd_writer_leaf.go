// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package bkd

import (
	"bytes"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

// This file ports the leaf-block emission helpers of
// org.apache.lucene.util.bkd.BKDWriter:
//   - writeLeafBlockPackedValues + its high / low cardinality variants
//   - writeActualBounds
//   - the OneDimensionBKDWriter inner class
//   - small helpers (runLen, computeMinMax).

// writeLeafBlockPackedValues emits the packed-value payload of a leaf
// block following the per-dim common prefixes, choosing between three
// formats: all-equal (-1), low-cardinality (-2), or high-cardinality
// (sortedDim byte). Mirrors Java's writeLeafBlockPackedValues.
func (w *BKDWriter) writeLeafBlockPackedValues(
	out store.DataOutput,
	commonPrefixLengths []int,
	count int,
	sortedDim int,
	packedValues func(int) *util.BytesRef,
	leafCardinality int,
) error {
	prefixLenSum := 0
	for _, v := range commonPrefixLengths {
		prefixLenSum += v
	}
	if prefixLenSum == w.config.PackedBytesLength() {
		// All values in this block are equal.
		return out.WriteByte(byte(0xFF)) // (byte) -1
	}

	if commonPrefixLengths[sortedDim] >= w.config.BytesPerDim() {
		return fmt.Errorf("bkd: commonPrefixLengths[sortedDim]=%d >= bytesPerDim=%d",
			commonPrefixLengths[sortedDim], w.config.BytesPerDim())
	}
	compressedByteOffset := sortedDim*w.config.BytesPerDim() + commonPrefixLengths[sortedDim]
	var highCardinalityCost, lowCardinalityCost int
	if count == leafCardinality {
		// All values in this block are different.
		highCardinalityCost = 0
		lowCardinalityCost = 1
	} else {
		// Compute cost of runLen compression.
		numRunLens := 0
		i := 0
		for i < count {
			end := i + 0xff
			if end > count {
				end = count
			}
			r := runLen(packedValues, i, end, compressedByteOffset)
			if r > 0xff {
				return fmt.Errorf("bkd: runLen=%d > 255", r)
			}
			numRunLens++
			i += r
		}
		// Add cost of runLen compression.
		highCardinalityCost = count*(w.config.PackedBytesLength()-prefixLenSum-1) + 2*numRunLens
		// +1 is the byte needed for storing the cardinality.
		lowCardinalityCost = leafCardinality * (w.config.PackedBytesLength() - prefixLenSum + 1)
	}
	if lowCardinalityCost <= highCardinalityCost {
		if err := out.WriteByte(byte(0xFE)); err != nil { // (byte) -2
			return err
		}
		return w.writeLowCardinalityLeafBlockPackedValues(out, commonPrefixLengths, count, packedValues)
	}
	if err := out.WriteByte(byte(sortedDim)); err != nil {
		return err
	}
	return w.writeHighCardinalityLeafBlockPackedValues(
		out, commonPrefixLengths, count, sortedDim, packedValues, compressedByteOffset)
}

func (w *BKDWriter) writeLowCardinalityLeafBlockPackedValues(
	out store.DataOutput,
	commonPrefixLengths []int,
	count int,
	packedValues func(int) *util.BytesRef,
) error {
	if w.config.NumIndexDims() != 1 {
		if err := w.writeActualBounds(out, commonPrefixLengths, count, packedValues); err != nil {
			return err
		}
	}
	value := packedValues(0)
	copy(w.scratch[:w.config.PackedBytesLength()],
		value.Bytes[value.Offset:value.Offset+w.config.PackedBytesLength()])
	cardinality := 1
	bytesPerDim := w.config.BytesPerDim()
	for i := 1; i < count; i++ {
		value = packedValues(i)
		broken := false
		for dim := 0; dim < w.config.NumDims(); dim++ {
			start := dim * bytesPerDim
			if !w.equalsPredicate(value.Bytes, value.Offset+start, w.scratch, start) {
				if err := store.WriteVInt(out, int32(cardinality)); err != nil {
					return err
				}
				for j := 0; j < w.config.NumDims(); j++ {
					off := j*bytesPerDim + commonPrefixLengths[j]
					if err := out.WriteBytes(w.scratch[off : off+bytesPerDim-commonPrefixLengths[j]]); err != nil {
						return err
					}
				}
				copy(w.scratch[:w.config.PackedBytesLength()],
					value.Bytes[value.Offset:value.Offset+w.config.PackedBytesLength()])
				cardinality = 1
				broken = true
				break
			} else if dim == w.config.NumDims()-1 {
				cardinality++
			}
		}
		_ = broken // suppress lint warnings; control already broke out of inner loop.
	}
	if err := store.WriteVInt(out, int32(cardinality)); err != nil {
		return err
	}
	for i := 0; i < w.config.NumDims(); i++ {
		off := i*bytesPerDim + commonPrefixLengths[i]
		if err := out.WriteBytes(w.scratch[off : off+bytesPerDim-commonPrefixLengths[i]]); err != nil {
			return err
		}
	}
	return nil
}

func (w *BKDWriter) writeHighCardinalityLeafBlockPackedValues(
	out store.DataOutput,
	commonPrefixLengths []int,
	count int,
	sortedDim int,
	packedValues func(int) *util.BytesRef,
	compressedByteOffset int,
) error {
	if w.config.NumIndexDims() != 1 {
		if err := w.writeActualBounds(out, commonPrefixLengths, count, packedValues); err != nil {
			return err
		}
	}
	commonPrefixLengths[sortedDim]++
	i := 0
	for i < count {
		end := i + 0xff
		if end > count {
			end = count
		}
		r := runLen(packedValues, i, end, compressedByteOffset)
		if r > 0xff {
			return fmt.Errorf("bkd: runLen=%d > 255", r)
		}
		first := packedValues(i)
		prefixByte := first.Bytes[first.Offset+compressedByteOffset]
		if err := out.WriteByte(prefixByte); err != nil {
			return err
		}
		if err := out.WriteByte(byte(r)); err != nil {
			return err
		}
		if err := w.writeLeafBlockPackedValuesRange(out, commonPrefixLengths, i, i+r, packedValues); err != nil {
			return err
		}
		i += r
		if i > count {
			return fmt.Errorf("bkd: writeHighCardinality overrun: i=%d count=%d", i, count)
		}
	}
	return nil
}

func (w *BKDWriter) writeLeafBlockPackedValuesRange(
	out store.DataOutput,
	commonPrefixLengths []int,
	start, end int,
	packedValues func(int) *util.BytesRef,
) error {
	bytesPerDim := w.config.BytesPerDim()
	for i := start; i < end; i++ {
		ref := packedValues(i)
		if ref.Length != w.config.PackedBytesLength() {
			return fmt.Errorf("bkd: ref.length=%d != packedBytesLength=%d",
				ref.Length, w.config.PackedBytesLength())
		}
		for dim := 0; dim < w.config.NumDims(); dim++ {
			prefix := commonPrefixLengths[dim]
			begin := ref.Offset + dim*bytesPerDim + prefix
			tail := bytesPerDim - prefix
			if tail > 0 {
				if err := out.WriteBytes(ref.Bytes[begin : begin+tail]); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// writeActualBounds emits the per-index-dim suffix bounds at the head
// of the packed-value payload (when numIndexDims != 1). Mirrors the
// Java helper of the same name.
func (w *BKDWriter) writeActualBounds(
	out store.DataOutput,
	commonPrefixLengths []int,
	count int,
	packedValues func(int) *util.BytesRef,
) error {
	bytesPerDim := w.config.BytesPerDim()
	for dim := 0; dim < w.config.NumIndexDims(); dim++ {
		commonPrefixLength := commonPrefixLengths[dim]
		suffixLength := bytesPerDim - commonPrefixLength
		if suffixLength > 0 {
			minBytes, maxBytes := computeMinMax(count, packedValues,
				dim*bytesPerDim+commonPrefixLength, suffixLength)
			if err := out.WriteBytes(minBytes); err != nil {
				return err
			}
			if err := out.WriteBytes(maxBytes); err != nil {
				return err
			}
		}
	}
	return nil
}

// runLen returns the maximum prefix length (capped by end-start) over
// which packedValues[start..].bytes[byteOffset] stays equal.
func runLen(packedValues func(int) *util.BytesRef, start, end, byteOffset int) int {
	first := packedValues(start)
	b := first.Bytes[first.Offset+byteOffset]
	for i := start + 1; i < end; i++ {
		ref := packedValues(i)
		b2 := ref.Bytes[ref.Offset+byteOffset]
		if b != b2 {
			return i - start
		}
	}
	return end - start
}

// computeMinMax returns fresh copies of the min and max bytes for the
// [offset, offset+length) window of the supplied packed values.
func computeMinMax(count int, packedValues func(int) *util.BytesRef, offset, length int) ([]byte, []byte) {
	minB := make([]byte, length)
	maxB := make([]byte, length)
	first := packedValues(0)
	copy(minB, first.Bytes[first.Offset+offset:first.Offset+offset+length])
	copy(maxB, first.Bytes[first.Offset+offset:first.Offset+offset+length])
	for i := 1; i < count; i++ {
		cand := packedValues(i)
		c := cand.Bytes[cand.Offset+offset : cand.Offset+offset+length]
		if bytes.Compare(minB, c) > 0 {
			copy(minB, c)
		} else if bytes.Compare(maxB, c) < 0 {
			copy(maxB, c)
		}
	}
	return minB, maxB
}

// ---------------------------------------------------------------------
// OneDimensionBKDWriter: streaming writer used by the numDims == 1
// fast path of WriteField. Mirrors Java's BKDWriter.OneDimensionBKDWriter.
// ---------------------------------------------------------------------

type oneDimensionBKDWriter struct {
	owner    *BKDWriter
	metaOut  store.IndexOutput
	indexOut store.IndexOutput
	dataOut  store.IndexOutput

	dataStartFP int64

	leafBlockFPs *packed.PackedLongValuesBuilder
	// leafBlockStartValues stores the first packedIndexBytes of each
	// leaf block except the first, in insertion order, concatenated as
	// a single contiguous slab so that GetSplitValue is O(1). The Java
	// reference uses FixedLengthBytesRefArray which exposes random
	// access; the Go port keeps its own flat buffer to avoid the
	// per-access Iterator walk.
	leafBlockStartValues []byte
	startValuesCount     int
	leafValues           []byte
	leafDocs             []int32
	valueCount           int64
	leafCount            int
	leafCardinality      int
}

func newOneDimensionBKDWriter(owner *BKDWriter, metaOut, indexOut, dataOut store.IndexOutput) *oneDimensionBKDWriter {
	if owner.config.NumIndexDims() != 1 {
		panic(fmt.Sprintf("bkd: oneDimensionBKDWriter requires numIndexDims == 1 (got %d)", owner.config.NumIndexDims()))
	}
	if owner.pointCount != 0 {
		panic("bkd: cannot mix Add and merge")
	}
	if owner.finished {
		panic("bkd: already finished")
	}
	owner.finished = true

	leafFPs, err := packed.PackedBuilder(packed.PackedLongValuesDefaultPageSize, packed.Compact)
	if err != nil {
		panic(err)
	}
	return &oneDimensionBKDWriter{
		owner:        owner,
		metaOut:      metaOut,
		indexOut:     indexOut,
		dataOut:      dataOut,
		dataStartFP:  dataOut.GetFilePointer(),
		leafBlockFPs: leafFPs,
		leafValues:   make([]byte, owner.config.MaxPointsInLeafNode()*owner.config.PackedBytesLength()),
		leafDocs:     make([]int32, owner.config.MaxPointsInLeafNode()),
	}
}

func (o *oneDimensionBKDWriter) add(packedValue []byte, docID int) error {
	cfg := o.owner.config
	if o.leafCount == 0 || !o.owner.equalsPredicate(
		o.leafValues, (o.leafCount-1)*cfg.BytesPerDim(),
		packedValue, 0) {
		o.leafCardinality++
	}
	copy(o.leafValues[o.leafCount*cfg.PackedBytesLength():o.leafCount*cfg.PackedBytesLength()+cfg.PackedBytesLength()],
		packedValue)
	o.leafDocs[o.leafCount] = int32(docID)
	if docID >= 0 && docID < o.owner.maxDoc {
		o.owner.docsSeen.Set(docID)
	}
	o.leafCount++

	if o.valueCount+int64(o.leafCount) > o.owner.totalPointCount {
		return fmt.Errorf(
			"bkd: totalPointCount=%d was passed when we were created, but we just hit %d values",
			o.owner.totalPointCount, o.valueCount+int64(o.leafCount),
		)
	}

	if o.leafCount == cfg.MaxPointsInLeafNode() {
		if err := o.writeLeafBlock(o.leafCardinality); err != nil {
			return err
		}
		o.leafCardinality = 0
		o.leafCount = 0
	}
	return nil
}

func (o *oneDimensionBKDWriter) finish() (IORunnable, error) {
	if o.leafCount > 0 {
		if err := o.writeLeafBlock(o.leafCardinality); err != nil {
			return nil, err
		}
		o.leafCardinality = 0
		o.leafCount = 0
	}
	if o.valueCount == 0 {
		return nil, nil
	}
	o.owner.pointCount = o.valueCount

	cfg := o.owner.config

	if int64(o.startValuesCount+1) != o.leafBlockFPs.Size() {
		return nil, fmt.Errorf("bkd: oneDim invariant: start=%d fps=%d",
			o.startValuesCount, o.leafBlockFPs.Size())
	}
	leafFPLongValues := newPackedLongValuesAdapter(o.leafBlockFPs.Build())

	// Build the leaf-nodes view for the index writer. The split
	// values are the per-block first values stored contiguously in
	// leafBlockStartValues; the split dimension is always 0 in the
	// one-dim case.
	leafNodes := &oneDimLeafNodes{
		owner:        o.owner,
		startValues:  o.leafBlockStartValues,
		blockFPs:     leafFPLongValues,
		valueLength:  cfg.PackedIndexBytesLength(),
		scratchValue: util.BytesRef{},
	}

	return func() error {
		return o.owner.writeIndex(o.metaOut, o.indexOut, cfg.MaxPointsInLeafNode(), leafNodes, o.dataStartFP)
	}, nil
}

func (o *oneDimensionBKDWriter) writeLeafBlock(leafCardinality int) error {
	cfg := o.owner.config
	if o.leafCount == 0 {
		return fmt.Errorf("bkd: writeLeafBlock with leafCount=0")
	}
	if o.valueCount == 0 {
		copy(o.owner.minPackedValue, o.leafValues[:cfg.PackedIndexBytesLength()])
	}
	copy(o.owner.maxPackedValue,
		o.leafValues[(o.leafCount-1)*cfg.PackedBytesLength():(o.leafCount-1)*cfg.PackedBytesLength()+cfg.PackedIndexBytesLength()])
	o.valueCount += int64(o.leafCount)

	if o.leafBlockFPs.Size() > 0 {
		// Save the first (minimum) value of each non-first leaf block.
		o.leafBlockStartValues = append(o.leafBlockStartValues, o.leafValues[:cfg.PackedIndexBytesLength()]...)
		o.startValuesCount++
	}
	if err := o.leafBlockFPs.Add(o.dataOut.GetFilePointer()); err != nil {
		return err
	}
	if err := o.owner.checkMaxLeafNodeCount(int(o.leafBlockFPs.Size())); err != nil {
		return err
	}

	// Find per-dim common prefix: numIndexDims is 1 here, so the
	// single dim's common prefix is computed across [first, last].
	o.owner.commonPrefixLens[0] = o.owner.commonPrefixComparator(
		o.leafValues, 0,
		o.leafValues, (o.leafCount-1)*cfg.PackedBytesLength(),
	)

	if err := o.owner.writeLeafBlockDocs(o.dataOut, o.leafDocs, 0, o.leafCount); err != nil {
		return err
	}
	if err := o.owner.writeCommonPrefixes(o.dataOut, o.owner.commonPrefixLens, o.leafValues); err != nil {
		return err
	}

	ref := &util.BytesRef{
		Bytes:  o.leafValues,
		Length: cfg.PackedBytesLength(),
	}
	leafCount := o.leafCount
	packedValues := func(i int) *util.BytesRef {
		ref.Offset = cfg.PackedBytesLength() * i
		return ref
	}
	return o.owner.writeLeafBlockPackedValues(o.dataOut, o.owner.commonPrefixLens, leafCount, 0, packedValues, leafCardinality)
}

// oneDimLeafNodes adapts the leaf-start-values slab + leaf block FPs
// to the BKDTreeLeafNodes contract used by writeIndex.
type oneDimLeafNodes struct {
	owner        *BKDWriter
	startValues  []byte
	blockFPs     *packedLongValuesAdapter
	valueLength  int
	scratchValue util.BytesRef
}

func (o *oneDimLeafNodes) NumLeaves() int { return int(o.blockFPs.Size()) }

func (o *oneDimLeafNodes) GetLeafLP(index int) int64 { return o.blockFPs.Get(int64(index)) }

func (o *oneDimLeafNodes) GetSplitValue(index int) *util.BytesRef {
	o.scratchValue.Bytes = o.startValues
	o.scratchValue.Offset = index * o.valueLength
	o.scratchValue.Length = o.valueLength
	return &o.scratchValue
}

func (o *oneDimLeafNodes) GetSplitDimension(index int) int { return 0 }

// ---------------------------------------------------------------------
// LongValues adapters.
// ---------------------------------------------------------------------

// longValuesGet is the lookup contract consumed by the index writer.
// Mirrors Java's util.LongValues (a single get(long) method).
type longValuesGet interface {
	Get(index int64) int64
}

// sliceLongValues adapts a []int64 to longValuesGet.
type sliceLongValues struct{ s []int64 }

func newSliceLongValues(s []int64) *sliceLongValues { return &sliceLongValues{s: s} }

func (l *sliceLongValues) Get(index int64) int64 { return l.s[index] }

// packedLongValuesAdapter adapts a *packed.PackedLongValues to the
// longValuesGet contract.
type packedLongValuesAdapter struct {
	plv *packed.PackedLongValues
}

func newPackedLongValuesAdapter(plv *packed.PackedLongValues) *packedLongValuesAdapter {
	return &packedLongValuesAdapter{plv: plv}
}

func (a *packedLongValuesAdapter) Get(index int64) int64 { return a.plv.Get(index) }

func (a *packedLongValuesAdapter) Size() int64 { return a.plv.Size() }
