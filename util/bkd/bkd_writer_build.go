// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package bkd

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

// This file ports the recursive build functions of
// org.apache.lucene.util.bkd.BKDWriter (Lucene 10.4.0): the two
// variants (MutablePointTree-driven and PathSlice-driven), the
// per-leaf emission helpers, and the dim-cardinality scan.

// writeField1Dim handles the numIndexDims == 1 fast path of
// WriteField: sort by docID/value via the mutable reader utils, then
// stream through the one-dim writer.
func (w *BKDWriter) writeField1Dim(
	metaOut, indexOut, dataOut store.IndexOutput,
	fieldName string,
	reader MutablePointTree,
	readerSize int,
) (IORunnable, error) {
	SortMutablePointTree(w.config, w.maxDoc, reader, 0, readerSize)

	oneDim := newOneDimensionBKDWriter(w, metaOut, indexOut, dataOut)
	visitor := &add1DVisitor{oneDim: oneDim}
	if err := visitMutablePointTreeAll(reader, readerSize, visitor); err != nil {
		return nil, err
	}
	return oneDim.finish()
}

// visitMutablePointTreeAll walks every point in the mutable reader
// in order and feeds (docID, packedValue) to the visitor. Mirrors
// Java's reader.visitDocValues with an IntersectVisitor.
func visitMutablePointTreeAll(reader MutablePointTree, size int, visitor mutablePointVisitor) error {
	scratch := &util.BytesRef{}
	for i := 0; i < size; i++ {
		reader.GetValue(i, scratch)
		docID := reader.GetDocID(i)
		// Materialise a defensive copy of the packed value because the
		// scratch BytesRef aliases the reader's storage and the
		// downstream OneDim leaf buffer holds onto bytes across calls.
		bytes := make([]byte, scratch.Length)
		copy(bytes, scratch.Bytes[scratch.Offset:scratch.Offset+scratch.Length])
		if err := visitor.visit(docID, bytes); err != nil {
			return err
		}
	}
	return nil
}

// mutablePointVisitor is the narrow surface required by the mutable
// reader walk: only the per-doc visit hook used by the BKD writer.
type mutablePointVisitor interface {
	visit(docID int, packedValue []byte) error
}

type add1DVisitor struct {
	oneDim *oneDimensionBKDWriter
}

func (v *add1DVisitor) visit(docID int, packedValue []byte) error {
	return v.oneDim.add(packedValue, docID)
}

// writeFieldNDims handles the numIndexDims > 1 path: recursively pick
// the split dimension and partition values around it using
// MutablePointTreeReaderUtils.partition.
func (w *BKDWriter) writeFieldNDims(
	metaOut, indexOut, dataOut store.IndexOutput,
	fieldName string,
	values MutablePointTree,
	readerSize int,
) (IORunnable, error) {
	if w.pointCount != 0 {
		return nil, fmt.Errorf("bkd: cannot mix Add and WriteField")
	}
	if w.finished {
		return nil, fmt.Errorf("bkd: already finished")
	}

	// Mark that we already finished.
	w.finished = true
	w.pointCount = int64(readerSize)

	if w.pointCount == 0 {
		return nil, nil
	}

	numLeaves := int((w.pointCount + int64(w.config.MaxPointsInLeafNode()) - 1) /
		int64(w.config.MaxPointsInLeafNode()))
	numSplits := numLeaves - 1

	if err := w.checkMaxLeafNodeCount(numLeaves); err != nil {
		return nil, err
	}

	splitPackedValues := make([]byte, numSplits*w.config.BytesPerDim())
	splitDimensionValues := make([]byte, numSplits)
	leafBlockFPs := make([]int64, numLeaves)

	// Compute the min/max for this slice.
	w.computePackedValueBounds_mutable(values, 0, readerSize, w.minPackedValue, w.maxPackedValue, w.scratchBytesRef1)
	for i := 0; i < readerSize; i++ {
		docID := values.GetDocID(i)
		if docID >= 0 && docID < w.maxDoc {
			w.docsSeen.Set(docID)
		}
	}

	dataStartFP := dataOut.GetFilePointer()
	parentSplits := make([]int, w.config.NumIndexDims())

	if err := w.buildMutable(
		0, numLeaves, values, 0, readerSize, dataOut,
		cloneBytes(w.minPackedValue), cloneBytes(w.maxPackedValue),
		parentSplits, splitPackedValues, splitDimensionValues,
		leafBlockFPs, make([]int32, w.config.MaxPointsInLeafNode()),
	); err != nil {
		return nil, err
	}

	w.scratchBytesRef1.Length = w.config.BytesPerDim()
	w.scratchBytesRef1.Bytes = splitPackedValues

	leafFPs := newSliceLongValues(leafBlockFPs)
	return w.makeWriter(metaOut, indexOut, splitDimensionValues, leafFPs, len(leafBlockFPs), dataStartFP), nil
}

// buildMutable is the MutablePointTree-driven recursion. It exactly
// mirrors the structure of the Java build method with the same name.
//
// would obscure the line-by-line correspondence with the reference.
//
//nolint:gocyclo // direct port of the Java recursion; restructuring
func (w *BKDWriter) buildMutable(
	leavesOffset int,
	numLeaves int,
	reader MutablePointTree,
	from, to int,
	out store.IndexOutput,
	minPackedValue, maxPackedValue []byte,
	parentSplits []int,
	splitPackedValues, splitDimensionValues []byte,
	leafBlockFPs []int64,
	spareDocIds []int32,
) error {

	if numLeaves == 1 {
		// Leaf node.
		count := to - from
		if count > w.config.MaxPointsInLeafNode() {
			return fmt.Errorf("bkd: leaf overflow: count=%d max=%d", count, w.config.MaxPointsInLeafNode())
		}

		// Compute common prefixes.
		for i := range w.commonPrefixLens {
			w.commonPrefixLens[i] = w.config.BytesPerDim()
		}
		reader.GetValue(from, w.scratchBytesRef1)
		bytesPerDim := w.config.BytesPerDim()
		for i := from + 1; i < to; i++ {
			reader.GetValue(i, w.scratchBytesRef2)
			for dim := 0; dim < w.config.NumDims(); dim++ {
				offset := dim * bytesPerDim
				prefixHere := w.commonPrefixComparator(
					w.scratchBytesRef1.Bytes, w.scratchBytesRef1.Offset+offset,
					w.scratchBytesRef2.Bytes, w.scratchBytesRef2.Offset+offset,
				)
				if prefixHere < w.commonPrefixLens[dim] {
					w.commonPrefixLens[dim] = prefixHere
				}
			}
		}

		// Find the dimension that has the least number of unique bytes at
		// commonPrefixLengths[dim].
		usedBytes := make([]*util.FixedBitSet, w.config.NumDims())
		for dim := 0; dim < w.config.NumDims(); dim++ {
			if w.commonPrefixLens[dim] < bytesPerDim {
				bs, err := util.NewFixedBitSet(256)
				if err != nil {
					return err
				}
				usedBytes[dim] = bs
			}
		}
		for i := from + 1; i < to; i++ {
			for dim := 0; dim < w.config.NumDims(); dim++ {
				if usedBytes[dim] != nil {
					b := reader.GetByteAt(i, dim*bytesPerDim+w.commonPrefixLens[dim])
					usedBytes[dim].Set(int(b) & 0xff)
				}
			}
		}
		sortedDim := 0
		sortedDimCardinality := int(^uint(0) >> 1) // math.MaxInt
		for dim := 0; dim < w.config.NumDims(); dim++ {
			if usedBytes[dim] != nil {
				card := usedBytes[dim].Cardinality()
				if card < sortedDimCardinality {
					sortedDim = dim
					sortedDimCardinality = card
				}
			}
		}

		// Sort by sortedDim.
		SortMutablePointTreeByDim(w.config, sortedDim, w.commonPrefixLens, reader, from, to,
			w.scratchBytesRef1, w.scratchBytesRef2)

		// Compute leaf cardinality.
		comparator := w.scratchBytesRef1
		collector := w.scratchBytesRef2
		reader.GetValue(from, comparator)
		leafCardinality := 1
		for i := from + 1; i < to; i++ {
			reader.GetValue(i, collector)
			for dim := 0; dim < w.config.NumDims(); dim++ {
				start := dim * bytesPerDim
				if !w.equalsPredicate(
					collector.Bytes, collector.Offset+start,
					comparator.Bytes, comparator.Offset+start,
				) {
					leafCardinality++
					tmp := collector
					collector = comparator
					comparator = tmp
					break
				}
			}
		}

		// Save the block file pointer.
		leafBlockFPs[leavesOffset] = out.GetFilePointer()

		// Write doc IDs.
		docIDs := spareDocIds
		for i := from; i < to; i++ {
			docIDs[i-from] = int32(reader.GetDocID(i))
		}
		if err := w.writeLeafBlockDocs(out, docIDs, 0, count); err != nil {
			return err
		}

		// Write the common prefixes.
		reader.GetValue(from, w.scratchBytesRef1)
		copy(w.scratch[:w.config.PackedBytesLength()],
			w.scratchBytesRef1.Bytes[w.scratchBytesRef1.Offset:w.scratchBytesRef1.Offset+w.config.PackedBytesLength()])
		if err := w.writeCommonPrefixes(out, w.commonPrefixLens, w.scratch); err != nil {
			return err
		}

		// Write the full values.
		packedScratch := &util.BytesRef{}
		packedValues := func(i int) *util.BytesRef {
			reader.GetValue(from+i, packedScratch)
			return packedScratch
		}
		return w.writeLeafBlockPackedValues(out, w.commonPrefixLens, count, sortedDim, packedValues, leafCardinality)
	}

	// Inner node.
	splitDim := 0
	if w.config.NumIndexDims() == 1 {
		splitDim = 0
	} else {
		// For dimensions > 2 we recompute the bounds for the current
		// inner node to help the algorithm choose best split
		// dimensions. Because it is an expensive operation, the
		// frequency we recompute the bounds is given by
		// SPLITS_BEFORE_EXACT_BOUNDS.
		totalSplits := 0
		for _, n := range parentSplits {
			totalSplits += n
		}
		if numLeaves != len(leafBlockFPs) && w.config.NumIndexDims() > 2 &&
			totalSplits%SPLITS_BEFORE_EXACT_BOUNDS == 0 {
			w.computePackedValueBounds_mutable(reader, from, to, minPackedValue, maxPackedValue, w.scratchBytesRef1)
		}
		splitDim = w.split(minPackedValue, maxPackedValue, parentSplits)
	}

	numLeftLeafNodes := getNumLeftLeafNodes(numLeaves)
	mid := from + numLeftLeafNodes*w.config.MaxPointsInLeafNode()

	commonPrefixLen := w.commonPrefixComparator(
		minPackedValue, splitDim*w.config.BytesPerDim(),
		maxPackedValue, splitDim*w.config.BytesPerDim(),
	)

	PartitionMutablePointTree(w.config, w.maxDoc, splitDim, commonPrefixLen, reader, from, to, mid,
		w.scratchBytesRef1, w.scratchBytesRef2)

	rightOffset := leavesOffset + numLeftLeafNodes
	splitOffset := rightOffset - 1
	address := splitOffset * w.config.BytesPerDim()
	splitDimensionValues[splitOffset] = byte(splitDim)
	reader.GetValue(mid, w.scratchBytesRef1)
	copy(splitPackedValues[address:address+w.config.BytesPerDim()],
		w.scratchBytesRef1.Bytes[w.scratchBytesRef1.Offset+splitDim*w.config.BytesPerDim():w.scratchBytesRef1.Offset+splitDim*w.config.BytesPerDim()+w.config.BytesPerDim()])

	minSplitPackedValue := make([]byte, w.config.PackedIndexBytesLength())
	maxSplitPackedValue := make([]byte, w.config.PackedIndexBytesLength())
	copy(minSplitPackedValue, minPackedValue[:w.config.PackedIndexBytesLength()])
	copy(maxSplitPackedValue, maxPackedValue[:w.config.PackedIndexBytesLength()])
	copy(minSplitPackedValue[splitDim*w.config.BytesPerDim():splitDim*w.config.BytesPerDim()+w.config.BytesPerDim()],
		w.scratchBytesRef1.Bytes[w.scratchBytesRef1.Offset+splitDim*w.config.BytesPerDim():w.scratchBytesRef1.Offset+splitDim*w.config.BytesPerDim()+w.config.BytesPerDim()])
	copy(maxSplitPackedValue[splitDim*w.config.BytesPerDim():splitDim*w.config.BytesPerDim()+w.config.BytesPerDim()],
		w.scratchBytesRef1.Bytes[w.scratchBytesRef1.Offset+splitDim*w.config.BytesPerDim():w.scratchBytesRef1.Offset+splitDim*w.config.BytesPerDim()+w.config.BytesPerDim()])

	// Recurse.
	parentSplits[splitDim]++
	if err := w.buildMutable(
		leavesOffset, numLeftLeafNodes, reader, from, mid, out,
		minPackedValue, maxSplitPackedValue, parentSplits,
		splitPackedValues, splitDimensionValues, leafBlockFPs, spareDocIds,
	); err != nil {
		return err
	}
	if err := w.buildMutable(
		rightOffset, numLeaves-numLeftLeafNodes, reader, mid, to, out,
		minSplitPackedValue, maxPackedValue, parentSplits,
		splitPackedValues, splitDimensionValues, leafBlockFPs, spareDocIds,
	); err != nil {
		return err
	}
	parentSplits[splitDim]--
	return nil
}

// buildOffline is the offline-pipeline variant: every recursion step
// pivots around a PathSlice that may live on disk. Mirrors the Java
// build method that operates on BKDRadixSelector.PathSlice.
//
//nolint:gocyclo // direct port of the Java recursion.
func (w *BKDWriter) buildOffline(
	leavesOffset int,
	numLeaves int,
	points PathSlice,
	out store.IndexOutput,
	radixSelector *BKDRadixSelector,
	minPackedValue, maxPackedValue []byte,
	parentSplits []int,
	splitPackedValues, splitDimensionValues []byte,
	leafBlockFPs *packed.PackedLongValuesBuilder,
	totalNumLeaves int,
	spareDocIds []int32,
) error {

	if numLeaves == 1 {
		// Leaf node: write block.
		var heapSource *HeapPointWriter
		if hps, ok := points.Writer.(*HeapPointWriter); ok {
			heapSource = hps
		} else {
			converted, err := w.switchToHeap(points.Writer)
			if err != nil {
				return err
			}
			// switchToHeap destroys the source for us; mark closed so
			// subsequent code does not try to read from it again.
			heapSource = converted
		}

		from, err := int64ToInt(points.Start, "from")
		if err != nil {
			return err
		}
		toEnd, err := int64ToInt(points.Start+points.Count, "to")
		if err != nil {
			return err
		}

		// Compute per-dim common prefix lengths into w.scratch.
		w.computeCommonPrefixLengthOffline(heapSource, w.scratch, from, toEnd)

		sortedDim := 0
		sortedDimCardinality := int(^uint(0) >> 1)
		usedBytes := make([]*util.FixedBitSet, w.config.NumDims())
		bytesPerDim := w.config.BytesPerDim()
		for dim := 0; dim < w.config.NumDims(); dim++ {
			if w.commonPrefixLens[dim] < bytesPerDim {
				bs, fErr := util.NewFixedBitSet(256)
				if fErr != nil {
					return fErr
				}
				usedBytes[dim] = bs
			}
		}
		// Find the dimension to compress.
		for dim := 0; dim < w.config.NumDims(); dim++ {
			prefix := w.commonPrefixLens[dim]
			if prefix < bytesPerDim {
				offset := dim * bytesPerDim
				for i := from; i < toEnd; i++ {
					pv := heapSource.GetPackedValueSlice(i)
					br := pv.PackedValue()
					bucket := int(br.Bytes[br.Offset+offset+prefix]) & 0xff
					usedBytes[dim].Set(bucket)
				}
				card := usedBytes[dim].Cardinality()
				if card < sortedDimCardinality {
					sortedDim = dim
					sortedDimCardinality = card
				}
			}
		}

		// Sort the chosen dimension.
		radixSelector.HeapRadixSort(heapSource, from, toEnd, sortedDim, w.commonPrefixLens[sortedDim])
		// Compute cardinality.
		leafCardinality := heapSource.ComputeCardinality(from, toEnd, w.commonPrefixLens)

		// Save the block file pointer.
		if err := leafBlockFPs.Add(out.GetFilePointer()); err != nil {
			return err
		}

		// Write doc IDs.
		count := toEnd - from
		if count <= 0 {
			return fmt.Errorf("bkd: empty leaf at offset=%d", leavesOffset)
		}
		if count > len(spareDocIds) {
			return fmt.Errorf("bkd: count=%d > length=%d", count, len(spareDocIds))
		}
		docIDs := spareDocIds
		for i := 0; i < count; i++ {
			docIDs[i] = int32(heapSource.GetPackedValueSlice(from + i).DocID())
		}
		if err := w.writeLeafBlockDocs(out, docIDs, 0, count); err != nil {
			return err
		}

		// Write the common prefixes.
		if err := w.writeCommonPrefixes(out, w.commonPrefixLens, w.scratch); err != nil {
			return err
		}

		// Write the full values.
		packedValuesFn := func(i int) *util.BytesRef {
			return heapSource.GetPackedValueSlice(from + i).PackedValue()
		}
		return w.writeLeafBlockPackedValues(out, w.commonPrefixLens, count, sortedDim, packedValuesFn, leafCardinality)
	}

	// Inner node: partition/recurse.
	var splitDim int
	if w.config.NumIndexDims() == 1 {
		splitDim = 0
	} else {
		totalSplits := 0
		for _, n := range parentSplits {
			totalSplits += n
		}
		if numLeaves != totalNumLeaves && w.config.NumIndexDims() > 2 &&
			totalSplits%SPLITS_BEFORE_EXACT_BOUNDS == 0 {
			if err := w.computePackedValueBounds_offline(points, minPackedValue, maxPackedValue); err != nil {
				return err
			}
		}
		splitDim = w.split(minPackedValue, maxPackedValue, parentSplits)
	}

	if numLeaves > totalNumLeaves {
		return fmt.Errorf("bkd: numLeaves=%d totalNumLeaves=%d", numLeaves, totalNumLeaves)
	}

	numLeftLeafNodes := getNumLeftLeafNodes(numLeaves)
	leftCount := int64(numLeftLeafNodes) * int64(w.config.MaxPointsInLeafNode())

	slices := make([]PathSlice, 2)
	commonPrefixLen := w.commonPrefixComparator(
		minPackedValue, splitDim*w.config.BytesPerDim(),
		maxPackedValue, splitDim*w.config.BytesPerDim(),
	)

	splitValue, err := radixSelector.Select(
		points, slices, points.Start, points.Start+points.Count,
		points.Start+leftCount, splitDim, commonPrefixLen,
	)
	if err != nil {
		return err
	}

	rightOffset := leavesOffset + numLeftLeafNodes
	splitValueOffset := rightOffset - 1
	splitDimensionValues[splitValueOffset] = byte(splitDim)
	address := splitValueOffset * w.config.BytesPerDim()
	copy(splitPackedValues[address:address+w.config.BytesPerDim()], splitValue)

	bytesPerDim := w.config.BytesPerDim()
	minSplitPackedValue := make([]byte, w.config.PackedIndexBytesLength())
	maxSplitPackedValue := make([]byte, w.config.PackedIndexBytesLength())
	copy(minSplitPackedValue, minPackedValue[:w.config.PackedIndexBytesLength()])
	copy(maxSplitPackedValue, maxPackedValue[:w.config.PackedIndexBytesLength()])
	copy(minSplitPackedValue[splitDim*bytesPerDim:splitDim*bytesPerDim+bytesPerDim], splitValue)
	copy(maxSplitPackedValue[splitDim*bytesPerDim:splitDim*bytesPerDim+bytesPerDim], splitValue)

	parentSplits[splitDim]++
	// Recurse on left tree.
	if err := w.buildOffline(
		leavesOffset, numLeftLeafNodes, slices[0], out, radixSelector,
		minPackedValue, maxSplitPackedValue, parentSplits,
		splitPackedValues, splitDimensionValues, leafBlockFPs, totalNumLeaves, spareDocIds,
	); err != nil {
		return err
	}
	// Recurse on right tree.
	if err := w.buildOffline(
		rightOffset, numLeaves-numLeftLeafNodes, slices[1], out, radixSelector,
		minSplitPackedValue, maxPackedValue, parentSplits,
		splitPackedValues, splitDimensionValues, leafBlockFPs, totalNumLeaves, spareDocIds,
	); err != nil {
		return err
	}
	parentSplits[splitDim]--
	return nil
}

// computeCommonPrefixLengthOffline mirrors Java's
// BKDWriter.computeCommonPrefixLength: scan a HeapPointWriter range
// and emit per-dim common prefixes into commonPrefixLens; the first
// point's bytes are also copied into commonPrefix for downstream
// writeCommonPrefixes use.
func (w *BKDWriter) computeCommonPrefixLengthOffline(
	hps *HeapPointWriter, commonPrefix []byte, from, to int,
) {
	bytesPerDim := w.config.BytesPerDim()
	for i := range w.commonPrefixLens {
		w.commonPrefixLens[i] = bytesPerDim
	}
	pv := hps.GetPackedValueSlice(from)
	br := pv.PackedValue()
	for dim := 0; dim < w.config.NumDims(); dim++ {
		copy(commonPrefix[dim*bytesPerDim:dim*bytesPerDim+bytesPerDim],
			br.Bytes[br.Offset+dim*bytesPerDim:br.Offset+dim*bytesPerDim+bytesPerDim])
	}
	for i := from + 1; i < to; i++ {
		pv = hps.GetPackedValueSlice(i)
		br = pv.PackedValue()
		for dim := 0; dim < w.config.NumDims(); dim++ {
			if w.commonPrefixLens[dim] != 0 {
				lp := w.commonPrefixComparator(
					commonPrefix, dim*bytesPerDim,
					br.Bytes, br.Offset+dim*bytesPerDim,
				)
				if lp < w.commonPrefixLens[dim] {
					w.commonPrefixLens[dim] = lp
				}
			}
		}
	}
}
