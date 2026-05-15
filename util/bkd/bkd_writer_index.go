// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package bkd

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// This file ports the index-packing helpers of
// org.apache.lucene.util.bkd.BKDWriter (Lucene 10.4.0):
// packIndex, recursePackIndex, writeIndex, and the makeWriter helper.

// bkdTreeLeafNodes is the flat-tree view consumed by the index
// packer. Mirrors the Java private interface BKDTreeLeafNodes.
type bkdTreeLeafNodes interface {
	// NumLeaves returns the number of leaf nodes.
	NumLeaves() int
	// GetLeafLP returns the on-disk pointer to the leaf at index.
	GetLeafLP(index int) int64
	// GetSplitValue returns the split bytes for the inner node between
	// leaf indices (index-1) and index.
	GetSplitValue(index int) *util.BytesRef
	// GetSplitDimension returns the split dimension for the inner
	// node at the given split index.
	GetSplitDimension(index int) int
}

// makeWriter packages the split byte arrays + leaf block FPs into the
// BKDTreeLeafNodes view and returns the IORunnable that emits the
// final index. Mirrors Java's makeWriter.
func (w *BKDWriter) makeWriter(
	metaOut, indexOut store.IndexOutput,
	splitDimensionValues []byte,
	leafBlockFPs longValuesGet,
	numLeaves int,
	dataStartFP int64,
) IORunnable {
	leafNodes := &splitArraysLeafNodes{
		owner:                w,
		splitDimensionValues: splitDimensionValues,
		leafBlockFPs:         leafBlockFPs,
		splitPackedBytes:     w.scratchBytesRef1.Bytes,
		numLeaves:            numLeaves,
	}
	return func() error {
		return w.writeIndex(metaOut, indexOut, w.config.MaxPointsInLeafNode(), leafNodes, dataStartFP)
	}
}

// splitArraysLeafNodes is the BKDTreeLeafNodes implementation for the
// general (numDims, numIndexDims) case where split values live in a
// contiguous byte array and split dimensions in their own byte array.
type splitArraysLeafNodes struct {
	owner                *BKDWriter
	splitDimensionValues []byte
	leafBlockFPs         longValuesGet
	splitPackedBytes     []byte
	numLeaves            int
	scratch              util.BytesRef
}

func (n *splitArraysLeafNodes) NumLeaves() int { return n.numLeaves }

func (n *splitArraysLeafNodes) GetLeafLP(index int) int64 {
	return n.leafBlockFPs.Get(int64(index))
}

func (n *splitArraysLeafNodes) GetSplitValue(index int) *util.BytesRef {
	n.scratch.Bytes = n.splitPackedBytes
	n.scratch.Offset = index * n.owner.config.BytesPerDim()
	n.scratch.Length = n.owner.config.BytesPerDim()
	return &n.scratch
}

func (n *splitArraysLeafNodes) GetSplitDimension(index int) int {
	return int(n.splitDimensionValues[index]) & 0xff
}

// writeIndex packs the BKD tree index and emits it through metaOut /
// indexOut. Mirrors the Java writeIndex(MetaOut, IndexOut, countPerLeaf,
// BKDTreeLeafNodes, dataStartFP) overload.
func (w *BKDWriter) writeIndex(
	metaOut, indexOut store.IndexOutput,
	countPerLeaf int,
	leafNodes bkdTreeLeafNodes,
	dataStartFP int64,
) error {
	packed, err := w.packIndex(leafNodes)
	if err != nil {
		return err
	}
	return w.writeIndexFinal(metaOut, indexOut, countPerLeaf, leafNodes.NumLeaves(), packed, dataStartFP)
}

// writeIndexFinal mirrors the lower-level Java writeIndex overload
// that accepts the already-packed index bytes.
func (w *BKDWriter) writeIndexFinal(
	metaOut, indexOut store.IndexOutput,
	countPerLeaf int,
	numLeaves int,
	packedIndex []byte,
	dataStartFP int64,
) error {
	if err := codecs.WriteHeader(metaOut, BKDCodecName, int32(w.version)); err != nil {
		return err
	}
	if err := store.WriteVInt(metaOut, int32(w.config.NumDims())); err != nil {
		return err
	}
	if err := store.WriteVInt(metaOut, int32(w.config.NumIndexDims())); err != nil {
		return err
	}
	if err := store.WriteVInt(metaOut, int32(countPerLeaf)); err != nil {
		return err
	}
	if err := store.WriteVInt(metaOut, int32(w.config.BytesPerDim())); err != nil {
		return err
	}
	if numLeaves <= 0 {
		return fmt.Errorf("bkd: writeIndex numLeaves=%d", numLeaves)
	}
	if err := store.WriteVInt(metaOut, int32(numLeaves)); err != nil {
		return err
	}
	if err := metaOut.WriteBytes(w.minPackedValue[:w.config.PackedIndexBytesLength()]); err != nil {
		return err
	}
	if err := metaOut.WriteBytes(w.maxPackedValue[:w.config.PackedIndexBytesLength()]); err != nil {
		return err
	}
	if err := store.WriteVLong(metaOut, w.pointCount); err != nil {
		return err
	}
	if err := store.WriteVInt(metaOut, int32(w.docsSeen.Cardinality())); err != nil {
		return err
	}
	if err := store.WriteVInt(metaOut, int32(len(packedIndex))); err != nil {
		return err
	}
	if err := store.WriteInt64(metaOut, dataStartFP); err != nil {
		return err
	}
	// If metaOut and indexOut are the same file, we account for the
	// fact that writing a long makes the index start 8 bytes later.
	bias := int64(0)
	if metaOut == indexOut {
		bias = 8
	}
	if err := store.WriteInt64(metaOut, indexOut.GetFilePointer()+bias); err != nil {
		return err
	}
	return indexOut.WriteBytes(packedIndex)
}

// packIndex serialises the BKD tree's flat representation into a
// compact byte[] using a recursive scheme. Mirrors Java's packIndex.
func (w *BKDWriter) packIndex(leafNodes bkdTreeLeafNodes) ([]byte, error) {
	writeBuffer := store.NewByteBuffersDataOutput()
	blocks := make([][]byte, 0, 64)
	lastSplitValues := make([]byte, w.config.BytesPerDim()*w.config.NumIndexDims())

	totalSize, err := w.recursePackIndex(
		writeBuffer, leafNodes, 0, &blocks,
		lastSplitValues,
		make([]bool, w.config.NumIndexDims()),
		false, 0, leafNodes.NumLeaves(),
	)
	if err != nil {
		return nil, err
	}

	index := make([]byte, totalSize)
	upto := 0
	for _, block := range blocks {
		copy(index[upto:upto+len(block)], block)
		upto += len(block)
	}
	if upto != totalSize {
		return nil, fmt.Errorf("bkd: packIndex size mismatch: %d vs %d", upto, totalSize)
	}
	return index, nil
}

// appendBlock copies writeBuffer's current contents into a fresh
// block, appends it to blocks, and resets writeBuffer. Mirrors Java's
// appendBlock helper.
func appendBlock(writeBuffer *store.ByteBuffersDataOutput, blocks *[][]byte) int {
	block := writeBuffer.ToArrayCopy()
	*blocks = append(*blocks, block)
	writeBuffer.Reset()
	return len(block)
}

// recursePackIndex packs a subtree of the leaf-node array. Mirrors the
// Java recursePackIndex routine line-by-line.
//
// would obscure the line-by-line correspondence with the reference.
//
//nolint:gocyclo // direct port of the Java recursion; restructuring
func (w *BKDWriter) recursePackIndex(
	writeBuffer *store.ByteBuffersDataOutput,
	leafNodes bkdTreeLeafNodes,
	minBlockFP int64,
	blocks *[][]byte,
	lastSplitValues []byte,
	negativeDeltas []bool,
	isLeft bool,
	leavesOffset, numLeaves int,
) (int, error) {
	if numLeaves == 1 {
		if isLeft {
			if leafNodes.GetLeafLP(leavesOffset)-minBlockFP != 0 {
				return 0, fmt.Errorf("bkd: recursePackIndex left-leaf delta != 0: %d",
					leafNodes.GetLeafLP(leavesOffset)-minBlockFP)
			}
			return 0, nil
		}
		delta := leafNodes.GetLeafLP(leavesOffset) - minBlockFP
		if leafNodes.NumLeaves() != numLeaves && delta <= 0 {
			return 0, fmt.Errorf("bkd: expected delta > 0; got numLeaves=%d delta=%d", numLeaves, delta)
		}
		if err := writeBuffer.WriteVLong(delta); err != nil {
			return 0, err
		}
		return appendBlock(writeBuffer, blocks), nil
	}

	var leftBlockFP int64
	if isLeft {
		if leafNodes.GetLeafLP(leavesOffset) != minBlockFP {
			return 0, fmt.Errorf("bkd: left subtree FP mismatch: lp=%d minBlockFP=%d",
				leafNodes.GetLeafLP(leavesOffset), minBlockFP)
		}
		leftBlockFP = minBlockFP
	} else {
		leftBlockFP = leafNodes.GetLeafLP(leavesOffset)
		delta := leftBlockFP - minBlockFP
		if leafNodes.NumLeaves() != numLeaves && delta <= 0 {
			return 0, fmt.Errorf("bkd: expected delta > 0; got numLeaves=%d delta=%d", numLeaves, delta)
		}
		if err := writeBuffer.WriteVLong(delta); err != nil {
			return 0, err
		}
	}

	numLeftLeafNodes := getNumLeftLeafNodes(numLeaves)
	rightOffset := leavesOffset + numLeftLeafNodes
	splitOffset := rightOffset - 1

	splitDim := leafNodes.GetSplitDimension(splitOffset)
	splitValue := leafNodes.GetSplitValue(splitOffset)
	address := splitValue.Offset

	// Find common prefix with last split value in this dim.
	prefix := w.commonPrefixComparator(
		splitValue.Bytes, address,
		lastSplitValues, splitDim*w.config.BytesPerDim(),
	)

	var firstDiffByteDelta int
	if prefix < w.config.BytesPerDim() {
		firstDiffByteDelta =
			int(splitValue.Bytes[address+prefix])&0xff -
				int(lastSplitValues[splitDim*w.config.BytesPerDim()+prefix])&0xff
		if negativeDeltas[splitDim] {
			firstDiffByteDelta = -firstDiffByteDelta
		}
		if firstDiffByteDelta <= 0 {
			return 0, fmt.Errorf("bkd: firstDiffByteDelta=%d not positive", firstDiffByteDelta)
		}
	} else {
		firstDiffByteDelta = 0
	}

	// Pack the prefix, splitDim and delta first diff byte into a single vInt.
	code := (firstDiffByteDelta*(1+w.config.BytesPerDim())+prefix)*w.config.NumIndexDims() + splitDim
	if err := writeBuffer.WriteVInt(int32(code)); err != nil {
		return 0, err
	}

	// Write the split value, prefix coded vs. our parent's split value.
	suffix := w.config.BytesPerDim() - prefix
	savSplitValue := make([]byte, suffix)
	if suffix > 1 {
		if err := writeBuffer.WriteBytes(splitValue.Bytes[address+prefix+1 : address+prefix+suffix]); err != nil {
			return 0, err
		}
	}

	// Stash the old bytes of lastSplitValues for restoration below.
	copy(savSplitValue,
		lastSplitValues[splitDim*w.config.BytesPerDim()+prefix:splitDim*w.config.BytesPerDim()+prefix+suffix])

	// Copy our split value into lastSplitValues for our children to prefix-code against.
	copy(lastSplitValues[splitDim*w.config.BytesPerDim()+prefix:splitDim*w.config.BytesPerDim()+prefix+suffix],
		splitValue.Bytes[address+prefix:address+prefix+suffix])

	numBytes := appendBlock(writeBuffer, blocks)

	// Placeholder for left-tree numBytes; we need this so that at
	// search time if we only need to recurse into the right subtree we
	// can quickly seek to its starting point.
	idxSav := len(*blocks)
	*blocks = append(*blocks, nil)

	savNegativeDelta := negativeDeltas[splitDim]
	negativeDeltas[splitDim] = true

	leftNumBytes, err := w.recursePackIndex(
		writeBuffer, leafNodes, leftBlockFP, blocks,
		lastSplitValues, negativeDeltas, true,
		leavesOffset, numLeftLeafNodes,
	)
	if err != nil {
		return 0, err
	}

	if numLeftLeafNodes != 1 {
		if err := writeBuffer.WriteVInt(int32(leftNumBytes)); err != nil {
			return 0, err
		}
	} else if leftNumBytes != 0 {
		return 0, fmt.Errorf("bkd: leftNumBytes=%d expected 0", leftNumBytes)
	}

	bytes2 := writeBuffer.ToArrayCopy()
	writeBuffer.Reset()
	(*blocks)[idxSav] = bytes2

	negativeDeltas[splitDim] = false
	rightNumBytes, err := w.recursePackIndex(
		writeBuffer, leafNodes, leftBlockFP, blocks,
		lastSplitValues, negativeDeltas, false,
		rightOffset, numLeaves-numLeftLeafNodes,
	)
	if err != nil {
		return 0, err
	}

	negativeDeltas[splitDim] = savNegativeDelta

	// Restore lastSplitValues to what caller originally passed us.
	copy(lastSplitValues[splitDim*w.config.BytesPerDim()+prefix:splitDim*w.config.BytesPerDim()+prefix+suffix],
		savSplitValue)

	return numBytes + len(bytes2) + leftNumBytes + rightNumBytes, nil
}
