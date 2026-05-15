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

// This file ports the inner class BKDReader.BKDPointTree (Lucene
// 10.4.0). The packed index format is the inverse of what
// BKDWriter.recursePackIndex emits:
//
//   - At each inner node:
//       (optional) leafBlockFP delta as VLong (only on right children;
//                  the left-child FP is inherited from the parent),
//       code as VInt = ((firstDiffByteDelta * (1 + bytesPerDim)) + prefix)
//                       * numIndexDims + splitDim,
//       suffix bytes (length = bytesPerDim - prefix) carrying the
//                    remaining bytes of the split value,
//       leftNumBytes as VInt (only when the left child is itself an
//                    inner node; absent for leaf-aligned splits).
//   - At each leaf: the VInt count followed by the docID block and
//     the packed-value payload in `dataIn` at the inherited leaf FP.
//
// All navigation in BKDPointTree is performed by reading these bytes
// on demand; nothing is materialised eagerly.

// bkdPointTree is the concrete PointTree implementation for BKDReader.
// It maintains a stack of per-level state that the navigation methods
// (moveToChild / moveToSibling / moveToParent) push and pop in sync
// with the position in `innerNodes`.
//
// Field naming follows the Java reference closely so that the line-by-
// line port stays auditable; comments explain the divergences forced
// by Go's semantics (notably the missing inner-class capture of the
// enclosing BKDReader's state).
type bkdPointTree struct {
	nodeID   int
	nodeRoot int
	// level is 1-based so that level-1 indexing works without
	// underflow on the root.
	level int

	innerNodes store.IndexInput
	leafNodes  store.IndexInput

	leafBlockFPStack      []int64
	readNodeDataPositions []int
	rightNodePositions    []int
	splitDimsPos          []int
	negativeDeltas        []bool
	splitValuesStack      [][]byte
	splitDimValueStack    [][]byte

	minPackedValue []byte
	maxPackedValue []byte

	config         BKDConfig
	leafNodeOffset int // == numLeaves
	version        int

	pointCount             int64
	lastLeafNodePointCount int
	rightMostLeafNode      int

	scratchDataPackedValue     []byte
	scratchMinIndexPackedValue []byte
	scratchMaxIndexPackedValue []byte
	commonPrefixLengths        []int

	scratchIterator *bkdReaderDocIDSetIterator
	docIdsWriter    *DocIdsWriter

	isTreeBalanced bool
}

// newBKDPointTree constructs a fresh PointTree rooted at the BKD index
// root (nodeID = 1, level = 1) and reads the root node's data. Mirrors
// the public BKDPointTree(...) constructor in the Java reference.
func newBKDPointTree(
	innerNodes, leafNodes store.IndexInput,
	config BKDConfig,
	numLeaves int,
	version int,
	pointCount int64,
	minPackedValue, maxPackedValue []byte,
	isTreeBalanced bool,
) (*bkdPointTree, error) {
	tree := allocBKDPointTree(
		innerNodes, leafNodes, config, numLeaves, version, pointCount,
		1, 1, // nodeID = 1, level = 1
		minPackedValue, maxPackedValue,
		newBKDReaderDocIDSetIterator(config.MaxPointsInLeafNode(), version),
		make([]byte, config.PackedBytesLength()),
		make([]byte, config.PackedIndexBytesLength()),
		make([]byte, config.PackedIndexBytesLength()),
		make([]int, config.NumDims()),
		isTreeBalanced,
	)
	// Root: read the node data once so callers can compare bounds
	// immediately. mirrors the {} initialiser + readNodeData(false)
	// at the end of the Java constructor.
	if err := tree.readNodeData(false); err != nil {
		return nil, err
	}
	return tree, nil
}

// allocBKDPointTree is the inner-private allocator that fills the per-
// level scratch arrays. Used by both newBKDPointTree (for the root)
// and Clone() (which reuses the caller's scratch buffers).
func allocBKDPointTree(
	innerNodes, leafNodes store.IndexInput,
	config BKDConfig,
	numLeaves int,
	version int,
	pointCount int64,
	nodeID, level int,
	minPackedValue, maxPackedValue []byte,
	scratchIterator *bkdReaderDocIDSetIterator,
	scratchDataPackedValue, scratchMinIndexPackedValue, scratchMaxIndexPackedValue []byte,
	commonPrefixLengths []int,
	isTreeBalanced bool,
) *bkdPointTree {
	treeDepth := getTreeDepth(numLeaves)
	tree := &bkdPointTree{
		nodeID:         nodeID,
		nodeRoot:       nodeID,
		level:          level,
		innerNodes:     innerNodes,
		leafNodes:      leafNodes,
		config:         config,
		leafNodeOffset: numLeaves,
		version:        version,
		pointCount:     pointCount,
		isTreeBalanced: isTreeBalanced,
		// scratch slices.
		splitDimValueStack:    make([][]byte, treeDepth),
		splitValuesStack:      make([][]byte, treeDepth),
		leafBlockFPStack:      make([]int64, treeDepth+1),
		readNodeDataPositions: make([]int, treeDepth+1),
		rightNodePositions:    make([]int, treeDepth),
		splitDimsPos:          make([]int, treeDepth),
		negativeDeltas:        make([]bool, config.NumIndexDims()*treeDepth),

		scratchIterator:            scratchIterator,
		scratchDataPackedValue:     scratchDataPackedValue,
		scratchMinIndexPackedValue: scratchMinIndexPackedValue,
		scratchMaxIndexPackedValue: scratchMaxIndexPackedValue,
		commonPrefixLengths:        commonPrefixLengths,
		docIdsWriter:               scratchIterator.docIdsWriter,
	}
	// Deep-copy the root bounds so callers can keep their pristine
	// copy. Mirrors `minPackedValue.clone()` / `maxPackedValue.clone()`
	// in the Java constructor.
	tree.minPackedValue = append([]byte(nil), minPackedValue...)
	tree.maxPackedValue = append([]byte(nil), maxPackedValue...)
	// The root's split-values buffer always exists, even though the
	// root itself has no parent split value; subsequent push operations
	// clone from this slot.
	tree.splitValuesStack[0] = make([]byte, config.PackedIndexBytesLength())

	// rightMostLeafNode mirrors `(1 << (treeDepth - 1)) - 1` from Java.
	tree.rightMostLeafNode = (1 << uint(treeDepth-1)) - 1
	last := int(pointCount % int64(config.MaxPointsInLeafNode()))
	if last == 0 {
		last = config.MaxPointsInLeafNode()
	}
	tree.lastLeafNodePointCount = last
	return tree
}

// getTreeDepth mirrors the private getTreeDepth helper from
// BKDPointTree (Java).
func getTreeDepth(numLeaves int) int {
	return util.MathLogIntBase(int64(numLeaves), 2) + 2
}

// Clone returns an independent cursor that shares the underlying
// (immutable) input data but maintains its own position stack.
func (t *bkdPointTree) Clone() PointTree {
	clone := allocBKDPointTree(
		t.innerNodes.Clone(), t.leafNodes.Clone(),
		t.config, t.leafNodeOffset, t.version, t.pointCount,
		t.nodeID, t.level,
		t.minPackedValue, t.maxPackedValue,
		t.scratchIterator,
		t.scratchDataPackedValue, t.scratchMinIndexPackedValue, t.scratchMaxIndexPackedValue,
		t.commonPrefixLengths,
		t.isTreeBalanced,
	)
	clone.leafBlockFPStack[clone.level] = t.leafBlockFPStack[t.level]
	if !t.isLeafNode() {
		clone.rightNodePositions[clone.level] = t.rightNodePositions[t.level]
		clone.readNodeDataPositions[clone.level] = t.readNodeDataPositions[t.level]
		clone.splitValuesStack[clone.level] = append([]byte(nil), t.splitValuesStack[t.level]...)
		startNeg := t.level * t.config.NumIndexDims()
		copy(
			clone.negativeDeltas[startNeg:startNeg+t.config.NumIndexDims()],
			t.negativeDeltas[startNeg:startNeg+t.config.NumIndexDims()],
		)
		clone.splitDimsPos[t.level] = t.splitDimsPos[t.level]
	}
	return clone
}

// GetMinPackedValue / GetMaxPackedValue return the cursor's current
// bounds. The slice aliases the cursor's internal buffer; the cursor
// itself does not invalidate it during a single navigation step, but
// callers that walk children must copy if they need to retain.
func (t *bkdPointTree) GetMinPackedValue() []byte { return t.minPackedValue }
func (t *bkdPointTree) GetMaxPackedValue() []byte { return t.maxPackedValue }

// MoveToChild attempts to descend into the left child. Returns false
// when the cursor is on a leaf.
func (t *bkdPointTree) MoveToChild() (bool, error) {
	if t.isLeafNode() {
		return false, nil
	}
	if err := t.resetNodeDataPosition(); err != nil {
		return false, err
	}
	t.pushBoundsLeft()
	if err := t.pushLeft(); err != nil {
		return false, err
	}
	return true, nil
}

// MoveToSibling moves to the right sibling if the cursor is on a left
// child; returns false otherwise.
func (t *bkdPointTree) MoveToSibling() (bool, error) {
	if !t.isLeftNode() || t.isRootNode() {
		return false, nil
	}
	t.pop()
	t.popBounds(t.maxPackedValue)
	t.pushBoundsRight()
	if err := t.pushRight(); err != nil {
		return false, err
	}
	if !t.nodeExists() {
		return false, fmt.Errorf("bkd: moveToSibling produced non-existent node %d", t.nodeID)
	}
	return true, nil
}

// MoveToParent returns to the parent node, restoring the bounds
// corresponding to the split that produced the current child.
func (t *bkdPointTree) MoveToParent() (bool, error) {
	if t.isRootNode() {
		return false, nil
	}
	var packed []byte
	if t.isLeftNode() {
		packed = t.maxPackedValue
	} else {
		packed = t.minPackedValue
	}
	t.pop()
	t.popBounds(packed)
	return true, nil
}

// Size returns the number of points reachable from the current
// subtree. Mirrors BKDPointTree.size().
func (t *bkdPointTree) Size() int64 {
	leftMostLeafNode := t.nodeID
	for leftMostLeafNode < t.leafNodeOffset {
		leftMostLeafNode *= 2
	}
	rightMostLeafNode := t.nodeID
	for rightMostLeafNode < t.leafNodeOffset {
		rightMostLeafNode = rightMostLeafNode*2 + 1
	}
	var numLeaves int
	if rightMostLeafNode >= leftMostLeafNode {
		numLeaves = rightMostLeafNode - leftMostLeafNode + 1
	} else {
		numLeaves = rightMostLeafNode - leftMostLeafNode + 1 + t.leafNodeOffset
	}
	if t.isTreeBalanced {
		// BKDWriter never emits balanced trees (version >= 9), so this
		// branch is only reached when reading externally-produced
		// legacy indices. We preserve the Java semantics for
		// completeness.
		return t.sizeFromBalancedTree(leftMostLeafNode, rightMostLeafNode)
	}
	if rightMostLeafNode == t.rightMostLeafNode {
		return int64(numLeaves-1)*int64(t.config.MaxPointsInLeafNode()) + int64(t.lastLeafNodePointCount)
	}
	return int64(numLeaves) * int64(t.config.MaxPointsInLeafNode())
}

// sizeFromBalancedTree mirrors the Java helper of the same name. It
// counts how many leaves carry maxPointsInLeafNode versus
// maxPointsInLeafNode-1.
func (t *bkdPointTree) sizeFromBalancedTree(leftMostLeafNode, rightMostLeafNode int) int64 {
	extraPoints := int(int64(t.config.MaxPointsInLeafNode())*int64(t.leafNodeOffset) - t.pointCount)
	nodeOffset := t.leafNodeOffset - extraPoints
	var count int64
	for node := leftMostLeafNode; node <= rightMostLeafNode; node++ {
		if balanceTreeNodePosition(0, t.leafNodeOffset, node-t.leafNodeOffset, 0, 0) < nodeOffset {
			count += int64(t.config.MaxPointsInLeafNode())
		} else {
			count += int64(t.config.MaxPointsInLeafNode() - 1)
		}
	}
	return count
}

// balanceTreeNodePosition mirrors the Java helper.
func balanceTreeNodePosition(minNode, maxNode, node, position, level int) int {
	if maxNode-minNode == 1 {
		return position
	}
	mid := (minNode + maxNode + 1) >> 1
	if mid > node {
		return balanceTreeNodePosition(minNode, mid, node, position, level+1)
	}
	return balanceTreeNodePosition(mid, maxNode, node, position+(1<<uint(level)), level+1)
}

// VisitDocIDs walks the subtree and delivers each leaf's docIDs through
// visitor.Visit. The packed values themselves are not decoded.
func (t *bkdPointTree) VisitDocIDs(visitor IntersectVisitor) error {
	if err := t.resetNodeDataPosition(); err != nil {
		return err
	}
	return t.addAll(visitor, false)
}

// addAll mirrors BKDPointTree.addAll; descends recursively, calling
// grow(size) at the top of each subtree small enough to fit in int.
func (t *bkdPointTree) addAll(visitor IntersectVisitor, grown bool) error {
	if !grown {
		size := t.Size()
		if size <= int64(^uint32(0)>>1) {
			visitor.Grow(int(size))
			grown = true
		}
	}
	if t.isLeafNode() {
		if err := t.leafNodes.SetPosition(t.getLeafBlockFP()); err != nil {
			return err
		}
		count32, err := store.ReadVInt(t.leafNodes)
		if err != nil {
			return err
		}
		count := int(count32)
		return t.docIdsWriter.ReadIntsVisitor(t.leafNodes, count, &visitDocIDsAdapter{v: visitor}, t.scratchIterator.docIDs)
	}
	if err := t.pushLeftForRecurse(); err != nil {
		return err
	}
	if err := t.addAll(visitor, grown); err != nil {
		return err
	}
	t.pop()
	if err := t.pushRightForRecurse(); err != nil {
		return err
	}
	if err := t.addAll(visitor, grown); err != nil {
		return err
	}
	t.pop()
	return nil
}

// visitDocIDsAdapter bridges the local IntersectVisitor.Visit(int) to
// the DocIdsWriter.DocIDVisitor narrow contract.
type visitDocIDsAdapter struct{ v IntersectVisitor }

func (a *visitDocIDsAdapter) Visit(docID int) error { return a.v.Visit(docID) }

// VisitDocValues walks the subtree leaf-by-leaf and dispatches each
// (docID, packedValue) pair through visitor.VisitByPackedValue. The
// per-leaf prefix-decoding follows the same layout BKDWriter emits.
func (t *bkdPointTree) VisitDocValues(visitor IntersectVisitor) error {
	if err := t.resetNodeDataPosition(); err != nil {
		return err
	}
	return t.visitLeavesOneByOne(visitor)
}

func (t *bkdPointTree) visitLeavesOneByOne(visitor IntersectVisitor) error {
	if t.isLeafNode() {
		return t.visitDocValues(visitor, t.getLeafBlockFP())
	}
	if err := t.pushLeftForRecurse(); err != nil {
		return err
	}
	if err := t.visitLeavesOneByOne(visitor); err != nil {
		return err
	}
	t.pop()
	if err := t.pushRightForRecurse(); err != nil {
		return err
	}
	if err := t.visitLeavesOneByOne(visitor); err != nil {
		return err
	}
	t.pop()
	return nil
}

// visitDocValues reads one leaf at blockFP, populates scratchIterator,
// and dispatches its docIDs / packed values through visitor. Mirrors
// BKDPointTree.visitDocValues(IntersectVisitor, long).
func (t *bkdPointTree) visitDocValues(visitor IntersectVisitor, blockFP int64) error {
	count, err := t.readDocIDs(t.leafNodes, blockFP, t.scratchIterator)
	if err != nil {
		return err
	}
	if t.version >= BKDVersionLowCardinalityLeaves {
		return t.visitDocValuesWithCardinality(visitor, count)
	}
	return t.visitDocValuesNoCardinality(visitor, count)
}

// readDocIDs decodes the leaf's docID block at blockFP into the
// supplied iterator's docIDs slice. Returns the count.
func (t *bkdPointTree) readDocIDs(in store.IndexInput, blockFP int64, iter *bkdReaderDocIDSetIterator) (int, error) {
	if err := in.SetPosition(blockFP); err != nil {
		return 0, err
	}
	count32, err := store.ReadVInt(in)
	if err != nil {
		return 0, err
	}
	count := int(count32)
	if err := iter.docIdsWriter.ReadInts(in, count, iter.docIDs); err != nil {
		return 0, err
	}
	return count, nil
}

// readNodeData mirrors BKDPointTree.readNodeData; runs every time the
// cursor descends so that the per-level scratch arrays carry the
// invariants needed for subsequent navigation.
func (t *bkdPointTree) readNodeData(isLeft bool) error {
	t.leafBlockFPStack[t.level] = t.leafBlockFPStack[t.level-1]
	if !isLeft {
		delta, err := store.ReadVLong(t.innerNodes)
		if err != nil {
			return err
		}
		t.leafBlockFPStack[t.level] += delta
	}

	if !t.isLeafNode() {
		numIndexDims := t.config.NumIndexDims()
		bytesPerDim := t.config.BytesPerDim()

		// negativeDeltas[level] = negativeDeltas[level-1] copied, then
		// the bit for the previous split dim is updated to isLeft.
		copy(
			t.negativeDeltas[t.level*numIndexDims:(t.level+1)*numIndexDims],
			t.negativeDeltas[(t.level-1)*numIndexDims:t.level*numIndexDims],
		)
		t.negativeDeltas[t.level*numIndexDims+(t.splitDimsPos[t.level-1]/bytesPerDim)] = isLeft

		// splitValuesStack[level] mirrors lazy allocation in Java: if
		// the slot is nil clone from the parent; otherwise overwrite
		// in place.
		pibl := t.config.PackedIndexBytesLength()
		if t.splitValuesStack[t.level] == nil {
			t.splitValuesStack[t.level] = append([]byte(nil), t.splitValuesStack[t.level-1]...)
		} else {
			copy(t.splitValuesStack[t.level], t.splitValuesStack[t.level-1][:pibl])
		}

		code32, err := store.ReadVInt(t.innerNodes)
		if err != nil {
			return err
		}
		code := int(code32)
		splitDim := code % numIndexDims
		t.splitDimsPos[t.level] = splitDim * bytesPerDim
		code /= numIndexDims
		prefix := code % (1 + bytesPerDim)
		suffix := bytesPerDim - prefix

		if suffix > 0 {
			firstDiffByteDelta := code / (1 + bytesPerDim)
			if t.negativeDeltas[t.level*numIndexDims+splitDim] {
				firstDiffByteDelta = -firstDiffByteDelta
			}
			startPos := t.splitDimsPos[t.level] + prefix
			oldByte := int(t.splitValuesStack[t.level][startPos]) & 0xFF
			t.splitValuesStack[t.level][startPos] = byte(oldByte + firstDiffByteDelta)
			if suffix-1 > 0 {
				if err := t.innerNodes.ReadBytes(t.splitValuesStack[t.level][startPos+1 : startPos+suffix]); err != nil {
					return err
				}
			}
		}

		var leftNumBytes int
		if t.nodeID*2 < t.leafNodeOffset {
			v, err := store.ReadVInt(t.innerNodes)
			if err != nil {
				return err
			}
			leftNumBytes = int(v)
		}
		t.rightNodePositions[t.level] = int(t.innerNodes.GetFilePointer()) + leftNumBytes
		t.readNodeDataPositions[t.level] = int(t.innerNodes.GetFilePointer())
	}
	return nil
}

// resetNodeDataPosition rewinds the inner-nodes input to the
// position recorded at the current level the last time readNodeData
// ran. Used before iterating the children once.
func (t *bkdPointTree) resetNodeDataPosition() error {
	pos := int64(t.readNodeDataPositions[t.level])
	if pos > t.innerNodes.GetFilePointer() {
		return fmt.Errorf("bkd: readNodeDataPosition=%d > currentPos=%d", pos, t.innerNodes.GetFilePointer())
	}
	return t.innerNodes.SetPosition(pos)
}

// pushBoundsLeft / pushBoundsRight save the current split-dim slice
// and overlay the level's split value onto the matching dimension of
// max (or min) packed value.
func (t *bkdPointTree) pushBoundsLeft() {
	splitDimPos := t.splitDimsPos[t.level]
	bytesPerDim := t.config.BytesPerDim()
	if t.splitDimValueStack[t.level] == nil {
		t.splitDimValueStack[t.level] = make([]byte, bytesPerDim)
	}
	copy(t.splitDimValueStack[t.level], t.maxPackedValue[splitDimPos:splitDimPos+bytesPerDim])
	copy(
		t.maxPackedValue[splitDimPos:splitDimPos+bytesPerDim],
		t.splitValuesStack[t.level][splitDimPos:splitDimPos+bytesPerDim],
	)
}

func (t *bkdPointTree) pushBoundsRight() {
	splitDimPos := t.splitDimsPos[t.level]
	bytesPerDim := t.config.BytesPerDim()
	// splitDimValueStack[level] was populated by the matching push-left.
	copy(t.splitDimValueStack[t.level], t.minPackedValue[splitDimPos:splitDimPos+bytesPerDim])
	copy(
		t.minPackedValue[splitDimPos:splitDimPos+bytesPerDim],
		t.splitValuesStack[t.level][splitDimPos:splitDimPos+bytesPerDim],
	)
}

// pushLeft / pushRight advance the cursor to the matching child and
// re-read the node data at the new level.
func (t *bkdPointTree) pushLeft() error {
	t.nodeID *= 2
	t.level++
	return t.readNodeData(true)
}

func (t *bkdPointTree) pushRight() error {
	pos := int64(t.rightNodePositions[t.level])
	if pos < t.innerNodes.GetFilePointer() {
		return fmt.Errorf("bkd: rightNodePosition=%d < currentPos=%d", pos, t.innerNodes.GetFilePointer())
	}
	if err := t.innerNodes.SetPosition(pos); err != nil {
		return err
	}
	t.nodeID = 2*t.nodeID + 1
	t.level++
	return t.readNodeData(false)
}

// pushLeftForRecurse / pushRightForRecurse combine the bounds and the
// position push into a single call used inside the recursive walkers.
// The bounds are unconditionally pushed because addAll / visitLeaves
// always wants them tracked.
func (t *bkdPointTree) pushLeftForRecurse() error {
	t.pushBoundsLeft()
	return t.pushLeft()
}

func (t *bkdPointTree) pushRightForRecurse() error {
	t.pushBoundsRight()
	return t.pushRight()
}

// pop / popBounds restore the cursor's state after a recursive walk
// returns to the parent.
func (t *bkdPointTree) pop() {
	t.nodeID /= 2
	t.level--
}

func (t *bkdPointTree) popBounds(packed []byte) {
	splitDimPos := t.splitDimsPos[t.level]
	bytesPerDim := t.config.BytesPerDim()
	copy(packed[splitDimPos:splitDimPos+bytesPerDim], t.splitDimValueStack[t.level])
}

// isRootNode / isLeftNode / isLeafNode / nodeExists / getLeafBlockFP
// mirror the matching private helpers in BKDPointTree.
func (t *bkdPointTree) isRootNode() bool { return t.nodeID == t.nodeRoot }
func (t *bkdPointTree) isLeftNode() bool { return (t.nodeID & 1) == 0 }
func (t *bkdPointTree) isLeafNode() bool { return t.nodeID >= t.leafNodeOffset }
func (t *bkdPointTree) nodeExists() bool { return t.nodeID-t.leafNodeOffset < t.leafNodeOffset }

func (t *bkdPointTree) getLeafBlockFP() int64 {
	if !t.isLeafNode() {
		panic(fmt.Sprintf("bkd: getLeafBlockFP called on inner node %d", t.nodeID))
	}
	return t.leafBlockFPStack[t.level]
}

// ---------------------------------------------------------------------
// Leaf packed-value decoding helpers (visit* family).
// ---------------------------------------------------------------------

// visitDocValuesNoCardinality decodes the leaf payload for
// pre-VERSION_LOW_CARDINALITY indices. BKDWriter emits version
// >= VERSION_LOW_CARDINALITY so this path is only exercised by legacy
// fixtures.
func (t *bkdPointTree) visitDocValuesNoCardinality(visitor IntersectVisitor, count int) error {
	if err := t.readCommonPrefixes(t.commonPrefixLengths, t.scratchDataPackedValue, t.leafNodes); err != nil {
		return err
	}
	pibl := t.config.PackedIndexBytesLength()
	if t.config.NumIndexDims() != 1 && t.version >= BKDVersionLeafStoresBounds {
		copy(t.scratchMinIndexPackedValue[:pibl], t.scratchDataPackedValue[:pibl])
		copy(t.scratchMaxIndexPackedValue[:pibl], t.scratchDataPackedValue[:pibl])
		if err := t.readMinMax(t.commonPrefixLengths, t.scratchMinIndexPackedValue, t.scratchMaxIndexPackedValue, t.leafNodes); err != nil {
			return err
		}
		r := visitor.Compare(t.scratchMinIndexPackedValue[:pibl], t.scratchMaxIndexPackedValue[:pibl])
		if r == codecs.RelationCellOutsideQuery {
			return nil
		}
		visitor.Grow(count)
		if r == codecs.RelationCellInsideQuery {
			return t.visitAllInLeafAsIDs(visitor, count)
		}
	} else {
		visitor.Grow(count)
	}

	compressedDim, err := t.readCompressedDim(t.leafNodes)
	if err != nil {
		return err
	}
	if compressedDim == -1 {
		return t.visitUniqueRawDocValues(t.scratchDataPackedValue, count, visitor)
	}
	return t.visitCompressedDocValues(t.commonPrefixLengths, t.scratchDataPackedValue, t.leafNodes, count, visitor, compressedDim)
}

// visitDocValuesWithCardinality decodes the leaf payload for indices
// at version >= VERSION_LOW_CARDINALITY_LEAVES. Mirrors
// BKDPointTree.visitDocValuesWithCardinality.
func (t *bkdPointTree) visitDocValuesWithCardinality(visitor IntersectVisitor, count int) error {
	if err := t.readCommonPrefixes(t.commonPrefixLengths, t.scratchDataPackedValue, t.leafNodes); err != nil {
		return err
	}
	compressedDim, err := t.readCompressedDim(t.leafNodes)
	if err != nil {
		return err
	}
	pibl := t.config.PackedIndexBytesLength()
	if compressedDim == -1 {
		visitor.Grow(count)
		return t.visitUniqueRawDocValues(t.scratchDataPackedValue, count, visitor)
	}
	if t.config.NumIndexDims() != 1 {
		copy(t.scratchMinIndexPackedValue[:pibl], t.scratchDataPackedValue[:pibl])
		copy(t.scratchMaxIndexPackedValue[:pibl], t.scratchDataPackedValue[:pibl])
		if err := t.readMinMax(t.commonPrefixLengths, t.scratchMinIndexPackedValue, t.scratchMaxIndexPackedValue, t.leafNodes); err != nil {
			return err
		}
		r := visitor.Compare(t.scratchMinIndexPackedValue[:pibl], t.scratchMaxIndexPackedValue[:pibl])
		if r == codecs.RelationCellOutsideQuery {
			return nil
		}
		visitor.Grow(count)
		if r == codecs.RelationCellInsideQuery {
			return t.visitAllInLeafAsIDs(visitor, count)
		}
	} else {
		visitor.Grow(count)
	}
	if compressedDim == -2 {
		return t.visitSparseRawDocValues(t.commonPrefixLengths, t.scratchDataPackedValue, t.leafNodes, count, visitor)
	}
	return t.visitCompressedDocValues(t.commonPrefixLengths, t.scratchDataPackedValue, t.leafNodes, count, visitor, compressedDim)
}

// visitAllInLeafAsIDs emits Visit(docID) for every docID in the
// leaf when the entire cell is inside the query. This degrades the
// Java IntsRef bulk-visit path to a per-doc loop, which is
// behaviourally identical from the visitor's perspective (only the
// dispatch cost differs).
func (t *bkdPointTree) visitAllInLeafAsIDs(visitor IntersectVisitor, count int) error {
	for i := 0; i < count; i++ {
		if err := visitor.Visit(int(t.scratchIterator.docIDs[i])); err != nil {
			return err
		}
	}
	return nil
}

// visitUniqueRawDocValues fires Visit(docID, packedValue) for every
// docID in the leaf when all values in the leaf are identical.
func (t *bkdPointTree) visitUniqueRawDocValues(scratchPackedValue []byte, count int, visitor IntersectVisitor) error {
	for i := 0; i < count; i++ {
		if err := visitor.VisitByPackedValue(int(t.scratchIterator.docIDs[i]), scratchPackedValue); err != nil {
			return err
		}
	}
	return nil
}

// visitSparseRawDocValues mirrors the BKDPointTree helper of the same
// name: emits packed values stored as (length, suffix-bytes) tuples,
// each tuple applying to `length` consecutive docIDs.
func (t *bkdPointTree) visitSparseRawDocValues(
	commonPrefixLengths []int,
	scratchPackedValue []byte,
	in store.IndexInput,
	count int,
	visitor IntersectVisitor,
) error {
	bytesPerDim := t.config.BytesPerDim()
	i := 0
	for i < count {
		length32, err := store.ReadVInt(in)
		if err != nil {
			return err
		}
		length := int(length32)
		for dim := 0; dim < t.config.NumDims(); dim++ {
			prefix := commonPrefixLengths[dim]
			if err := in.ReadBytes(scratchPackedValue[dim*bytesPerDim+prefix : dim*bytesPerDim+bytesPerDim]); err != nil {
				return err
			}
		}
		for k := 0; k < length; k++ {
			if err := visitor.VisitByPackedValue(int(t.scratchIterator.docIDs[i+k]), scratchPackedValue); err != nil {
				return err
			}
		}
		i += length
	}
	if i != count {
		return fmt.Errorf("bkd: sparse sub-blocks do not add up to expected count %d != %d", i, count)
	}
	return nil
}

// visitCompressedDocValues mirrors the BKDPointTree helper of the
// same name: runs of equal values on the compressed dim are emitted
// as (prefixByte, runLen, runLen suffix tuples).
func (t *bkdPointTree) visitCompressedDocValues(
	commonPrefixLengths []int,
	scratchPackedValue []byte,
	in store.IndexInput,
	count int,
	visitor IntersectVisitor,
	compressedDim int,
) error {
	bytesPerDim := t.config.BytesPerDim()
	compressedByteOffset := compressedDim*bytesPerDim + commonPrefixLengths[compressedDim]
	// Increment locally so we restore on return — exactly mirroring
	// the Java reference, which mutates commonPrefixLengths in place.
	commonPrefixLengths[compressedDim]++
	defer func() { commonPrefixLengths[compressedDim]-- }()
	i := 0
	for i < count {
		prefixByte, err := in.ReadByte()
		if err != nil {
			return err
		}
		scratchPackedValue[compressedByteOffset] = prefixByte
		runLenByte, err := in.ReadByte()
		if err != nil {
			return err
		}
		runLen := int(runLenByte)
		for j := 0; j < runLen; j++ {
			for dim := 0; dim < t.config.NumDims(); dim++ {
				prefix := commonPrefixLengths[dim]
				if err := in.ReadBytes(scratchPackedValue[dim*bytesPerDim+prefix : dim*bytesPerDim+bytesPerDim]); err != nil {
					return err
				}
			}
			if err := visitor.VisitByPackedValue(int(t.scratchIterator.docIDs[i+j]), scratchPackedValue); err != nil {
				return err
			}
		}
		i += runLen
	}
	if i != count {
		return fmt.Errorf("bkd: compressed sub-blocks do not add up to expected count %d != %d", i, count)
	}
	return nil
}

// readCompressedDim reads the leaf's compressedDim byte and validates
// it against the configured numDims and the on-disk version.
func (t *bkdPointTree) readCompressedDim(in store.IndexInput) (int, error) {
	b, err := in.ReadByte()
	if err != nil {
		return 0, err
	}
	// The byte is interpreted as a signed 8-bit integer: -1 means
	// "all values equal" and -2 means "low cardinality" (only for
	// version >= VERSION_LOW_CARDINALITY_LEAVES). Other valid values
	// are dim indices in [0, numDims).
	compressedDim := int(int8(b))
	if compressedDim < -2 ||
		compressedDim >= t.config.NumDims() ||
		(t.version < BKDVersionLowCardinalityLeaves && compressedDim == -2) {
		return 0, fmt.Errorf("bkd: invalid compressedDim=%d", compressedDim)
	}
	return compressedDim, nil
}

// readCommonPrefixes mirrors BKDPointTree.readCommonPrefixes: VInt
// prefix length per dim followed by the prefix bytes.
func (t *bkdPointTree) readCommonPrefixes(
	commonPrefixLengths []int,
	scratchPackedValue []byte,
	in store.IndexInput,
) error {
	bytesPerDim := t.config.BytesPerDim()
	for dim := 0; dim < t.config.NumDims(); dim++ {
		prefix32, err := store.ReadVInt(in)
		if err != nil {
			return err
		}
		prefix := int(prefix32)
		commonPrefixLengths[dim] = prefix
		if prefix > 0 {
			if err := in.ReadBytes(scratchPackedValue[dim*bytesPerDim : dim*bytesPerDim+prefix]); err != nil {
				return err
			}
		}
	}
	return nil
}

// readMinMax mirrors BKDPointTree.readMinMax: per indexed dim, read
// the suffix bytes of the leaf's actual min, then the suffix bytes of
// the actual max. The prefix bytes are already in both buffers (copied
// from scratchDataPackedValue by the caller).
func (t *bkdPointTree) readMinMax(
	commonPrefixLengths []int,
	minPackedValue, maxPackedValue []byte,
	in store.IndexInput,
) error {
	bytesPerDim := t.config.BytesPerDim()
	for dim := 0; dim < t.config.NumIndexDims(); dim++ {
		prefix := commonPrefixLengths[dim]
		suffix := bytesPerDim - prefix
		if suffix == 0 {
			continue
		}
		if err := in.ReadBytes(minPackedValue[dim*bytesPerDim+prefix : dim*bytesPerDim+bytesPerDim]); err != nil {
			return err
		}
		if err := in.ReadBytes(maxPackedValue[dim*bytesPerDim+prefix : dim*bytesPerDim+bytesPerDim]); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------
// bkdReaderDocIDSetIterator: helper that reuses the docIDs buffer
// across leaves and exposes a DocIdSetIterator-style sequential
// scan. The Java reference inherits from AbstractDocIdSetIterator and
// is used by visitSparseRawDocValues (CARDINALITY layout). In the Go
// port the sparse-cardinality path emits Visit(docID, packed) for
// each doc directly, so the iterator surface degrades to a thin
// (offset, length) container.
// ---------------------------------------------------------------------

type bkdReaderDocIDSetIterator struct {
	docIDs       []int32
	docIdsWriter *DocIdsWriter
}

func newBKDReaderDocIDSetIterator(maxPointsInLeafNode, version int) *bkdReaderDocIDSetIterator {
	return &bkdReaderDocIDSetIterator{
		docIDs:       make([]int32, maxPointsInLeafNode),
		docIdsWriter: NewDocIdsWriter(maxPointsInLeafNode, version),
	}
}
