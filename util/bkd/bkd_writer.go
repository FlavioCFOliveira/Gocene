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
	"errors"
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

// Port of org.apache.lucene.util.bkd.BKDWriter (Lucene 10.4.0).
//
// BKDWriter recursively builds a block KD-tree to assign all incoming
// points in N-dimensional space to smaller and smaller N-dimensional
// rectangles (cells) until the number of points in a given rectangle
// is <= config.MaxPointsInLeafNode(). The tree is partially balanced:
// the leaf nodes will have the requested MaxPointsInLeafNode except
// one that might have less. Leaf nodes may straddle the two bottom
// levels of the binary tree. Values that fall exactly on a cell
// boundary may be in either cell.
//
// The number of dimensions can be 1 to 8, but every byte[] value is
// fixed length.
//
// This consumes heap during writing: it allocates a long[numLeaves],
// a byte[numLeaves*(1+config.BytesPerDim())] and then uses up to the
// specified maxMBSortInHeap heap space for writing.
//
// NOTE: this can write at most math.MaxInt32 *
// config.MaxPointsInLeafNode() / config.BytesPerDim() total points.
//
// Lucene divergence note: the Java reference wraps the temp directory
// in a TrackingDirectoryWrapper to track temp files for best-effort
// cleanup on exception. The Go port relies on the offline pipeline
// (BKDRadixSelector / OfflinePointWriter) to destroy its own temp
// files on the happy path and exposes the initial spilled file for
// cleanup via tempInput; no separate tracking layer is required.

// Codec / version constants exposed to readers. Keep these in sync
// with Lucene 10.4.0's BKDWriter.VERSION_* / CODEC_NAME.
const (
	// BKDCodecName is the codec header used for the BKD meta file.
	BKDCodecName = "BKD"

	// BKDVersionStart is the version used by Lucene 7.0.
	BKDVersionStart = 4

	// BKDVersionLeafStoresBounds (=5) was introduced when leaf blocks
	// started storing per-cell bounds.
	BKDVersionLeafStoresBounds = 5

	// BKDVersionSelectiveIndexing (=6) introduces selective indexing
	// (numIndexDims < numDims).
	BKDVersionSelectiveIndexing = 6

	// BKDVersionLowCardinalityLeaves (=7) introduces the low-
	// cardinality leaf compression path.
	BKDVersionLowCardinalityLeaves = 7

	// SPLITS_BEFORE_EXACT_BOUNDS controls how often the exact bounds of
	// an inner node are recomputed when the number of indexed
	// dimensions is greater than two.
	SPLITS_BEFORE_EXACT_BOUNDS = 4

	// DefaultMaxMBSortInHeap is the default amount of heap (in MiB)
	// used for in-memory point sorting before spilling to disk.
	DefaultMaxMBSortInHeap = 16.0
)

// IORunnable is a closure that performs the final on-disk index
// write. Port of Lucene's IORunnable functional interface: it returns
// an error rather than throwing IOException.
type IORunnable func() error

// BKDWriter incrementally collects points and, on finish, emits the
// serialised tree across the meta/index/data outputs.
//
// A BKDWriter is not safe for concurrent use.
type BKDWriter struct {
	config             BKDConfig
	tempDir            store.Directory
	tempFileNamePrefix string

	comparator             ByteArrayComparator // unsigned comparator on BytesPerDim
	equalsPredicate        ByteArrayPredicate  // per-dim equality
	commonPrefixComparator ByteArrayComparator // prefix-length comparator
	docIdsWriter           *DocIdsWriter       // leaf docID encoder

	maxMBSortInHeap     float64
	maxPointsSortInHeap int
	maxDoc              int
	version             int

	scratchDiff      []byte
	scratch          []byte
	scratchBytesRef1 *util.BytesRef
	scratchBytesRef2 *util.BytesRef
	commonPrefixLens []int

	docsSeen *util.FixedBitSet

	pointWriter PointWriter
	tempInput   store.IndexOutput
	finished    bool

	minPackedValue []byte
	maxPackedValue []byte

	pointCount      int64
	totalPointCount int64
}

// NewBKDWriter constructs a writer using the current BKD version.
func NewBKDWriter(
	maxDoc int,
	tempDir store.Directory,
	tempFileNamePrefix string,
	config BKDConfig,
	maxMBSortInHeap float64,
	totalPointCount int64,
) (*BKDWriter, error) {
	return NewBKDWriterWithVersion(
		maxDoc, tempDir, tempFileNamePrefix, config,
		maxMBSortInHeap, totalPointCount, BKDVersionCurrent,
	)
}

// NewBKDWriterWithVersion constructs a writer producing the requested
// on-disk version. Only versions in [BKDVersionStart, BKDVersionCurrent]
// are accepted. Mirrors the Java constructor exposed for tests.
func NewBKDWriterWithVersion(
	maxDoc int,
	tempDir store.Directory,
	tempFileNamePrefix string,
	config BKDConfig,
	maxMBSortInHeap float64,
	totalPointCount int64,
	version int,
) (*BKDWriter, error) {
	if version < BKDVersionStart || version > BKDVersionCurrent {
		return nil, fmt.Errorf("bkd: version out of range: %d", version)
	}
	if maxMBSortInHeap < 0.0 {
		return nil, fmt.Errorf("bkd: maxMBSortInHeap must be >= 0.0 (got: %v)", maxMBSortInHeap)
	}
	if totalPointCount < 0 {
		return nil, fmt.Errorf("bkd: totalPointCount must be >=0 (got: %d)", totalPointCount)
	}
	if tempDir == nil {
		return nil, errors.New("bkd: tempDir cannot be nil")
	}
	if maxDoc < 0 {
		return nil, fmt.Errorf("bkd: maxDoc must be >= 0 (got %d)", maxDoc)
	}

	docsSeen, err := util.NewFixedBitSet(maxDoc)
	if err != nil {
		return nil, fmt.Errorf("bkd: creating docsSeen: %w", err)
	}

	maxPointsSortInHeap := int((maxMBSortInHeap * 1024 * 1024) / float64(config.BytesPerDoc()))
	if maxPointsSortInHeap < config.MaxPointsInLeafNode() {
		return nil, fmt.Errorf(
			"bkd: maxMBSortInHeap=%v only allows for maxPointsSortInHeap=%d, but this is less "+
				"than maxPointsInLeafNode=%d; either increase maxMBSortInHeap or decrease "+
				"maxPointsInLeafNode",
			maxMBSortInHeap, maxPointsSortInHeap, config.MaxPointsInLeafNode(),
		)
	}

	return &BKDWriter{
		config:                 config,
		tempDir:                tempDir,
		tempFileNamePrefix:     tempFileNamePrefix,
		comparator:             GetUnsignedComparator(config.BytesPerDim()),
		equalsPredicate:        GetEqualsPredicate(config.BytesPerDim()),
		commonPrefixComparator: GetPrefixLengthComparator(config.BytesPerDim()),
		docIdsWriter:           NewDocIdsWriter(config.MaxPointsInLeafNode(), version),
		maxMBSortInHeap:        maxMBSortInHeap,
		maxPointsSortInHeap:    maxPointsSortInHeap,
		maxDoc:                 maxDoc,
		version:                version,
		scratchDiff:            make([]byte, config.BytesPerDim()),
		scratch:                make([]byte, config.PackedBytesLength()),
		scratchBytesRef1:       &util.BytesRef{},
		scratchBytesRef2:       &util.BytesRef{},
		commonPrefixLens:       make([]int, config.NumDims()),
		docsSeen:               docsSeen,
		minPackedValue:         make([]byte, config.PackedIndexBytesLength()),
		maxPackedValue:         make([]byte, config.PackedIndexBytesLength()),
		totalPointCount:        totalPointCount,
	}, nil
}

// Config returns the BKD configuration of this writer.
func (w *BKDWriter) Config() BKDConfig { return w.config }

// Version returns the BKD on-disk version this writer emits.
func (w *BKDWriter) Version() int { return w.version }

// PointCount returns how many points have been added through Add so
// far. Used by tests and by the merge path.
func (w *BKDWriter) PointCount() int64 { return w.pointCount }

// initPointWriter allocates the in-memory or offline writer the first
// time Add is invoked. Mirrors Java's initPointWriter.
func (w *BKDWriter) initPointWriter() error {
	if w.pointWriter != nil {
		return errors.New("bkd: point writer is already initialized")
	}
	if w.totalPointCount > int64(w.maxPointsSortInHeap) {
		off, err := NewOfflinePointWriter(w.config, w.tempDir, w.tempFileNamePrefix, "spill", 0)
		if err != nil {
			return err
		}
		w.pointWriter = off
		w.tempInput = off.out
	} else {
		w.pointWriter = NewHeapPointWriter(w.config, int(w.totalPointCount))
	}
	return nil
}

// Add accumulates a new point composed of packedValue and docID.
// Mirrors Java's BKDWriter.add(byte[], int).
func (w *BKDWriter) Add(packedValue []byte, docID int) error {
	if len(packedValue) != w.config.PackedBytesLength() {
		return fmt.Errorf("bkd: packedValue should be length=%d (got: %d)",
			w.config.PackedBytesLength(), len(packedValue))
	}
	if w.pointCount >= w.totalPointCount {
		return fmt.Errorf(
			"bkd: totalPointCount=%d was passed when we were created, but we just hit %d values",
			w.totalPointCount, w.pointCount+1,
		)
	}
	if w.pointCount == 0 {
		if err := w.initPointWriter(); err != nil {
			return err
		}
		copy(w.minPackedValue, packedValue[:w.config.PackedIndexBytesLength()])
		copy(w.maxPackedValue, packedValue[:w.config.PackedIndexBytesLength()])
	} else {
		bytesPerDim := w.config.BytesPerDim()
		for dim := 0; dim < w.config.NumIndexDims(); dim++ {
			offset := dim * bytesPerDim
			if w.comparator(packedValue, offset, w.minPackedValue, offset) < 0 {
				copy(w.minPackedValue[offset:offset+bytesPerDim], packedValue[offset:offset+bytesPerDim])
			} else if w.comparator(packedValue, offset, w.maxPackedValue, offset) > 0 {
				copy(w.maxPackedValue[offset:offset+bytesPerDim], packedValue[offset:offset+bytesPerDim])
			}
		}
	}
	if err := w.pointWriter.Append(packedValue, docID); err != nil {
		return err
	}
	w.pointCount++
	if docID >= 0 && docID < w.maxDoc {
		w.docsSeen.Set(docID)
	}
	return nil
}

// Finish writes the BKD tree to the provided IndexOutputs and returns
// a runnable that writes the index of the tree if at least one point
// has been added, or nil otherwise. Port of Java's BKDWriter.finish.
func (w *BKDWriter) Finish(metaOut, indexOut, dataOut store.IndexOutput) (IORunnable, error) {
	// Catch user silliness.
	if w.finished {
		return nil, errors.New("bkd: already finished")
	}
	if w.pointCount == 0 {
		return nil, nil
	}
	// Mark as finished.
	w.finished = true

	if err := w.pointWriter.Close(); err != nil {
		return nil, err
	}
	points := PathSlice{Writer: w.pointWriter, Start: 0, Count: w.pointCount}
	// Clean up pointers.
	w.tempInput = nil
	w.pointWriter = nil

	numLeaves := int((w.pointCount + int64(w.config.MaxPointsInLeafNode()) - 1) /
		int64(w.config.MaxPointsInLeafNode()))
	numSplits := numLeaves - 1

	if err := w.checkMaxLeafNodeCount(numLeaves); err != nil {
		return nil, err
	}

	splitPackedValues := make([]byte, numSplits*w.config.BytesPerDim())
	splitDimensionValues := make([]byte, numSplits)

	leafBlockFPs, err := packed.PackedBuilder(packed.PackedLongValuesDefaultPageSize, packed.Compact)
	if err != nil {
		return nil, err
	}

	// We re-use the selector so we do not need to create an object every time.
	radixSelector, err := NewBKDRadixSelector(w.config, w.maxPointsSortInHeap, w.tempDir, w.tempFileNamePrefix)
	if err != nil {
		return nil, err
	}

	dataStartFP := dataOut.GetFilePointer()

	parentSplits := make([]int, w.config.NumIndexDims())
	minClone := cloneBytes(w.minPackedValue)
	maxClone := cloneBytes(w.maxPackedValue)
	if err := w.buildOffline(
		0, numLeaves, points, dataOut, radixSelector,
		minClone, maxClone, parentSplits,
		splitPackedValues, splitDimensionValues,
		leafBlockFPs, numLeaves,
		make([]int32, w.config.MaxPointsInLeafNode()),
	); err != nil {
		return nil, err
	}
	if int64(leafBlockFPs.Size()) != int64(numLeaves) {
		return nil, fmt.Errorf("bkd: built %d leaves, expected %d", leafBlockFPs.Size(), numLeaves)
	}

	leafBlockLongValues := leafBlockFPs.Build()
	w.scratchBytesRef1.Bytes = splitPackedValues
	w.scratchBytesRef1.Length = w.config.BytesPerDim()
	return w.makeWriter(metaOut, indexOut, splitDimensionValues, leafBlockLongValues,
		int(leafBlockFPs.Size()), dataStartFP), nil
}

// WriteField writes a field from a MutablePointTree. This way of
// writing points is faster than regular writes with BKDWriter.Add
// since there is opportunity for reordering points before writing
// them to disk. This method does not use transient disk in order to
// reorder points. Mirrors BKDWriter.writeField.
func (w *BKDWriter) WriteField(
	metaOut, indexOut, dataOut store.IndexOutput,
	fieldName string,
	reader MutablePointTree,
	readerSize int,
) (IORunnable, error) {
	if w.config.NumDims() == 1 {
		return w.writeField1Dim(metaOut, indexOut, dataOut, fieldName, reader, readerSize)
	}
	return w.writeFieldNDims(metaOut, indexOut, dataOut, fieldName, reader, readerSize)
}

// Close releases temp files held by an in-progress writer. Safe to
// call after Finish has run (it is idempotent in that case).
func (w *BKDWriter) Close() error {
	w.finished = true
	if w.tempInput != nil {
		// NOTE: should only happen on exception, e.g. caller calls Close
		// without calling Finish.
		closeErr := w.tempInput.Close()
		delErr := w.tempDir.DeleteFile(w.tempInput.GetName())
		w.tempInput = nil
		if closeErr != nil {
			return closeErr
		}
		return delErr
	}
	return nil
}

// checkMaxLeafNodeCount mirrors the Java method of the same name and
// returns a typed error rather than throwing.
func (w *BKDWriter) checkMaxLeafNodeCount(numLeaves int) error {
	if int64(w.config.BytesPerDim())*int64(numLeaves) > int64(util.MaxArrayLength) {
		return fmt.Errorf(
			"bkd: too many nodes; increase config.MaxPointsInLeafNode() (currently %d) and reindex",
			w.config.MaxPointsInLeafNode(),
		)
	}
	return nil
}

// getNumLeftLeafNodes returns how many of the supplied leaves go to
// the left subtree given a semi-balanced binary tree. Mirrors Java's
// getNumLeftLeafNodes.
func getNumLeftLeafNodes(numLeaves int) int {
	if numLeaves <= 1 {
		panic(fmt.Sprintf("bkd: getNumLeftLeafNodes called with %d", numLeaves))
	}
	// the level that can be filled with this number of leaves
	lastFullLevel := 31 - leadingZeros32(uint32(numLeaves))
	// how many leaf nodes are in the full level
	leavesFullLevel := 1 << lastFullLevel
	// half of the leaf nodes from the full level goes to the left
	numLeftLeafNodes := leavesFullLevel / 2
	// leaf nodes that do not fit in the full level
	unbalancedLeafNodes := numLeaves - leavesFullLevel
	// distribute unbalanced leaf nodes
	if unbalancedLeafNodes < numLeftLeafNodes {
		numLeftLeafNodes += unbalancedLeafNodes
	} else {
		numLeftLeafNodes += numLeftLeafNodes
	}
	return numLeftLeafNodes
}

// leadingZeros32 mirrors Java's Integer.numberOfLeadingZeros.
func leadingZeros32(v uint32) int {
	if v == 0 {
		return 32
	}
	n := 0
	if v&0xFFFF0000 == 0 {
		n += 16
		v <<= 16
	}
	if v&0xFF000000 == 0 {
		n += 8
		v <<= 8
	}
	if v&0xF0000000 == 0 {
		n += 4
		v <<= 4
	}
	if v&0xC0000000 == 0 {
		n += 2
		v <<= 2
	}
	if v&0x80000000 == 0 {
		n += 1
	}
	return n
}

// split picks the next dimension to split on, mirroring Java's
// BKDWriter.split. The choice prefers a dimension that has not yet
// been split twice less than the most-split dimension; failing that,
// it picks the dimension with the largest span.
func (w *BKDWriter) split(minPackedValue, maxPackedValue []byte, parentSplits []int) int {
	maxNumSplits := 0
	for _, n := range parentSplits {
		if n > maxNumSplits {
			maxNumSplits = n
		}
	}
	bytesPerDim := w.config.BytesPerDim()
	for dim := 0; dim < w.config.NumIndexDims(); dim++ {
		offset := dim * bytesPerDim
		if parentSplits[dim] < maxNumSplits/2 &&
			w.comparator(minPackedValue, offset, maxPackedValue, offset) != 0 {
			return dim
		}
	}

	// Find which dim has the largest span so we can split on it.
	splitDim := -1
	for dim := 0; dim < w.config.NumIndexDims(); dim++ {
		_ = util.Subtract(w.config.BytesPerDim(), dim, maxPackedValue, minPackedValue, w.scratchDiff)
		if splitDim == -1 || w.comparator(w.scratchDiff, 0, w.scratch, 0) > 0 {
			copy(w.scratch[:w.config.BytesPerDim()], w.scratchDiff)
			splitDim = dim
		}
	}
	return splitDim
}

// computePackedValueBounds_mutable recomputes the per-dim min/max for
// the supplied MutablePointTree range [from, to). Mirrors the Java
// overload taking a MutablePointTree.
func (w *BKDWriter) computePackedValueBounds_mutable(
	values MutablePointTree,
	from, to int,
	minPackedValue, maxPackedValue []byte,
	scratch *util.BytesRef,
) {
	if from == to {
		return
	}
	values.GetValue(from, scratch)
	copy(minPackedValue[:w.config.PackedIndexBytesLength()],
		scratch.Bytes[scratch.Offset:scratch.Offset+w.config.PackedIndexBytesLength()])
	copy(maxPackedValue[:w.config.PackedIndexBytesLength()],
		scratch.Bytes[scratch.Offset:scratch.Offset+w.config.PackedIndexBytesLength()])
	bytesPerDim := w.config.BytesPerDim()
	for i := from + 1; i < to; i++ {
		values.GetValue(i, scratch)
		for dim := 0; dim < w.config.NumIndexDims(); dim++ {
			start := dim * bytesPerDim
			if w.comparator(scratch.Bytes, scratch.Offset+start, minPackedValue, start) < 0 {
				copy(minPackedValue[start:start+bytesPerDim],
					scratch.Bytes[scratch.Offset+start:scratch.Offset+start+bytesPerDim])
			} else if w.comparator(scratch.Bytes, scratch.Offset+start, maxPackedValue, start) > 0 {
				copy(maxPackedValue[start:start+bytesPerDim],
					scratch.Bytes[scratch.Offset+start:scratch.Offset+start+bytesPerDim])
			}
		}
	}
}

// computePackedValueBounds_offline recomputes the per-dim min/max by
// reading the supplied PathSlice once. Mirrors the Java overload
// taking a PathSlice.
func (w *BKDWriter) computePackedValueBounds_offline(
	slice PathSlice,
	minPackedValue, maxPackedValue []byte,
) error {
	reader, err := slice.Writer.GetReader(slice.Start, slice.Count)
	if err != nil {
		return err
	}
	defer func() { _ = reader.Close() }()

	hasNext, err := reader.Next()
	if err != nil {
		return err
	}
	if !hasNext {
		return nil
	}
	val := reader.PointValue().PackedValue()
	copy(minPackedValue[:w.config.PackedIndexBytesLength()],
		val.Bytes[val.Offset:val.Offset+w.config.PackedIndexBytesLength()])
	copy(maxPackedValue[:w.config.PackedIndexBytesLength()],
		val.Bytes[val.Offset:val.Offset+w.config.PackedIndexBytesLength()])
	bytesPerDim := w.config.BytesPerDim()
	for {
		hasNext, err = reader.Next()
		if err != nil {
			return err
		}
		if !hasNext {
			break
		}
		val = reader.PointValue().PackedValue()
		for dim := 0; dim < w.config.NumIndexDims(); dim++ {
			startOffset := dim * bytesPerDim
			if w.comparator(val.Bytes, val.Offset+startOffset, minPackedValue, startOffset) < 0 {
				copy(minPackedValue[startOffset:startOffset+bytesPerDim],
					val.Bytes[val.Offset+startOffset:val.Offset+startOffset+bytesPerDim])
			} else if w.comparator(val.Bytes, val.Offset+startOffset, maxPackedValue, startOffset) > 0 {
				copy(maxPackedValue[startOffset:startOffset+bytesPerDim],
					val.Bytes[val.Offset+startOffset:val.Offset+startOffset+bytesPerDim])
			}
		}
	}
	return nil
}

// switchToHeap pulls a partition back into heap once the point count
// is low enough while recursing. Mirrors Java's switchToHeap.
func (w *BKDWriter) switchToHeap(source PointWriter) (*HeapPointWriter, error) {
	count, err := int64ToInt(source.Count(), "count")
	if err != nil {
		return nil, err
	}
	reader, err := source.GetReader(0, source.Count())
	if err != nil {
		return nil, err
	}
	writer := NewHeapPointWriter(w.config, count)
	for i := 0; i < count; i++ {
		hasNext, nErr := reader.Next()
		if nErr != nil {
			_ = reader.Close()
			return nil, nErr
		}
		if !hasNext {
			_ = reader.Close()
			return nil, errors.New("bkd: source reader exhausted before count")
		}
		if aErr := writer.AppendPointValue(reader.PointValue()); aErr != nil {
			_ = reader.Close()
			return nil, aErr
		}
	}
	if cErr := reader.Close(); cErr != nil {
		return nil, cErr
	}
	if dErr := source.Destroy(); dErr != nil {
		return nil, dErr
	}
	return writer, nil
}

// writeLeafBlockDocs emits the docID block at the start of a leaf
// block: a VInt count followed by the DocIdsWriter-encoded ids.
func (w *BKDWriter) writeLeafBlockDocs(out store.DataOutput, docIDs []int32, start, count int) error {
	if count <= 0 {
		return fmt.Errorf("bkd: writeLeafBlockDocs: count=%d", count)
	}
	if err := store.WriteVInt(out, int32(count)); err != nil {
		return err
	}
	return w.docIdsWriter.WriteDocIds(docIDs, start, count, out)
}

// writeCommonPrefixes emits the per-dim common prefix metadata at
// the head of a leaf block (after the doc IDs). Mirrors Java's
// writeCommonPrefixes.
func (w *BKDWriter) writeCommonPrefixes(out store.DataOutput, commonPrefixes []int, packedValue []byte) error {
	bytesPerDim := w.config.BytesPerDim()
	for dim := 0; dim < w.config.NumDims(); dim++ {
		if err := store.WriteVInt(out, int32(commonPrefixes[dim])); err != nil {
			return err
		}
		if commonPrefixes[dim] > 0 {
			if err := out.WriteBytes(packedValue[dim*bytesPerDim : dim*bytesPerDim+commonPrefixes[dim]]); err != nil {
				return err
			}
		}
	}
	return nil
}

// cloneBytes returns a fresh copy of b. Mirrors Java's b.clone() for
// the byte arrays the build recursion passes down.
func cloneBytes(b []byte) []byte {
	out := make([]byte, len(b))
	copy(out, b)
	return out
}

// int64ToInt narrows an int64 to an int (32-bit safety check) and
// returns a typed error on overflow.
func int64ToInt(v int64, name string) (int, error) {
	if v > math.MaxInt32 || v < math.MinInt32 {
		return 0, fmt.Errorf("bkd: %s overflows int32 (got %d)", name, v)
	}
	return int(v), nil
}

// GetUnsignedComparator returns an unsigned byte-range comparator for
// the supplied dimension width. It mirrors
// org.apache.lucene.util.ArrayUtil.getUnsignedComparator(numBytes) and
// returns the standard sign-extended -1/0/+1 result of a lexicographic
// byte comparison.
func GetUnsignedComparator(numBytes int) ByteArrayComparator {
	n := numBytes
	return func(a []byte, aOffset int, b []byte, bOffset int) int {
		for i := 0; i < n; i++ {
			av := a[aOffset+i]
			bv := b[bOffset+i]
			if av != bv {
				if av < bv {
					return -1
				}
				return 1
			}
		}
		return 0
	}
}
