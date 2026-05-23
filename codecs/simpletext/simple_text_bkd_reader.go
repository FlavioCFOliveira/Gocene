// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package simpletext

import (
	"bytes"
	"fmt"
	"math/bits"
	"strconv"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/bkd"
)

// SimpleTextBKDReader reads a BKD tree written by SimpleTextBKDWriter.
// It is a specialised, simplified version of BKDReader that understands
// the plain-text on-disk format.
//
// Port of org.apache.lucene.codecs.simpletext.SimpleTextBKDReader
// (Lucene 10.4.0).
type SimpleTextBKDReader struct {
	// splitPackedValues encodes the inner-node split dimension and value for
	// every inner node in the full binary tree (1-indexed, prefix-free).
	splitPackedValues []byte

	// leafBlockFPs holds the file pointer for each leaf node's data block.
	leafBlockFPs []int64

	// leafNodeOffset is the index of the first leaf node (== len(leafBlockFPs)).
	leafNodeOffset int

	// config holds the BKD dimensionality parameters.
	config bkd.BKDConfig

	// bytesPerIndexEntry is (bytesPerDim + 1) for multi-dim, or bytesPerDim for 1-dim.
	bytesPerIndexEntry int

	// in is the underlying data input (shared, cloned on tree creation).
	in store.IndexInput

	// minPackedValue and maxPackedValue are the global extremes of the field.
	minPackedValue []byte
	maxPackedValue []byte

	// pointCount is the total number of indexed points.
	pointCount int64

	// docCount is the number of distinct documents with points.
	docCount int

	// version is the writer version (always VersionCurrent for SimpleText).
	version int
}

// NewSimpleTextBKDReader constructs a reader from the decoded index metadata.
//
// Port of SimpleTextBKDReader constructor.
func NewSimpleTextBKDReader(
	in store.IndexInput,
	numDims, numIndexDims, maxPointsInLeafNode, bytesPerDim int,
	leafBlockFPs []int64,
	splitPackedValues []byte,
	minPackedValue, maxPackedValue []byte,
	pointCount int64,
	docCount int,
) (*SimpleTextBKDReader, error) {
	cfg, err := bkd.NewBKDConfig(numDims, numIndexDims, bytesPerDim, maxPointsInLeafNode)
	if err != nil {
		return nil, fmt.Errorf("NewSimpleTextBKDReader: invalid BKDConfig: %w", err)
	}
	bpie := bytesPerDim
	if numIndexDims != 1 {
		bpie = bytesPerDim + 1
	}
	return &SimpleTextBKDReader{
		splitPackedValues:  splitPackedValues,
		leafBlockFPs:       leafBlockFPs,
		leafNodeOffset:     len(leafBlockFPs),
		config:             cfg,
		bytesPerIndexEntry: bpie,
		in:                 in,
		minPackedValue:     minPackedValue,
		maxPackedValue:     maxPackedValue,
		pointCount:         pointCount,
		docCount:           docCount,
		version:            VersionCurrent,
	}, nil
}

// GetPointTree returns a new PointTree positioned at the root.
//
// Port of SimpleTextBKDReader.getPointTree().
func (r *SimpleTextBKDReader) GetPointTree() bkd.PointTree {
	return newSimpleTextPointTree(r, r.in.Clone(), 1, 1, r.minPackedValue, r.maxPackedValue)
}

// Intersect walks all points in the BKD tree and dispatches to visitor.
//
// Implements codecs.PointValues.
func (r *SimpleTextBKDReader) Intersect(visitor codecs.IntersectVisitor) error {
	tree := r.GetPointTree()
	return intersectSimpleText(tree, visitor)
}

// EstimatePointCount estimates the number of points matching visitor.
//
// Implements codecs.PointValues.
func (r *SimpleTextBKDReader) EstimatePointCount(visitor codecs.IntersectVisitor) int64 {
	tree := r.GetPointTree()
	return estimateSimpleText(tree, visitor)
}

// GetMinPackedValue returns the global minimum packed value.
func (r *SimpleTextBKDReader) GetMinPackedValue() []byte { return r.minPackedValue }

// GetMaxPackedValue returns the global maximum packed value.
func (r *SimpleTextBKDReader) GetMaxPackedValue() []byte { return r.maxPackedValue }

// GetNumDimensions returns the number of data dimensions.
func (r *SimpleTextBKDReader) GetNumDimensions() int { return r.config.NumDims() }

// GetNumIndexDimensions returns the number of index dimensions.
func (r *SimpleTextBKDReader) GetNumIndexDimensions() int { return r.config.NumIndexDims() }

// GetBytesPerDimension returns the number of bytes per dimension.
func (r *SimpleTextBKDReader) GetBytesPerDimension() int { return r.config.BytesPerDim() }

// Size returns the total number of indexed point values.
func (r *SimpleTextBKDReader) Size() int64 { return r.pointCount }

// GetDocCount returns the number of documents with at least one point.
func (r *SimpleTextBKDReader) GetDocCount() int { return r.docCount }

// compile-time assertion.
var _ codecs.PointValues = (*SimpleTextBKDReader)(nil)

// ---------------------------------------------------------------------------
// intersect and estimate helpers
// ---------------------------------------------------------------------------

func intersectSimpleText(tree bkd.PointTree, visitor codecs.IntersectVisitor) error {
	rel := visitor.Compare(tree.GetMinPackedValue(), tree.GetMaxPackedValue())
	switch rel {
	case codecs.RelationCellOutsideQuery:
		return nil
	case codecs.RelationCellInsideQuery:
		return tree.VisitDocValues(bkdVisitorAdapter{visitor})
	default: // CROSSES
		if ok, err := tree.MoveToChild(); err != nil {
			return err
		} else if ok {
			if err := intersectSimpleText(tree, visitor); err != nil {
				return err
			}
			if ok2, err := tree.MoveToSibling(); err != nil {
				return err
			} else if ok2 {
				if err := intersectSimpleText(tree, visitor); err != nil {
					return err
				}
			}
			if _, err := tree.MoveToParent(); err != nil {
				return err
			}
		} else {
			// leaf
			return tree.VisitDocValues(bkdVisitorAdapter{visitor})
		}
	}
	return nil
}

func estimateSimpleText(tree bkd.PointTree, visitor codecs.IntersectVisitor) int64 {
	rel := visitor.Compare(tree.GetMinPackedValue(), tree.GetMaxPackedValue())
	switch rel {
	case codecs.RelationCellOutsideQuery:
		return 0
	case codecs.RelationCellInsideQuery:
		return tree.Size()
	default: // CROSSES
		if ok, err := tree.MoveToChild(); err != nil || !ok {
			return (tree.Size() + 1) / 2
		}
		left := estimateSimpleText(tree, visitor)
		if ok2, err := tree.MoveToSibling(); err == nil && ok2 {
			left += estimateSimpleText(tree, visitor)
		}
		_, _ = tree.MoveToParent()
		return left
	}
}

// bkdVisitorAdapter wraps a codecs.IntersectVisitor into the bkd.IntersectVisitor
// interface expected by bkd.PointTree.
type bkdVisitorAdapter struct{ v codecs.IntersectVisitor }

func (a bkdVisitorAdapter) Visit(docID int) error { return a.v.Visit(docID) }
func (a bkdVisitorAdapter) VisitByPackedValue(docID int, pv []byte) error {
	return a.v.VisitByPackedValue(docID, pv)
}
func (a bkdVisitorAdapter) Compare(min, max []byte) codecs.Relation { return a.v.Compare(min, max) }
func (a bkdVisitorAdapter) Grow(count int)                          { a.v.Grow(count) }

var _ bkd.IntersectVisitor = bkdVisitorAdapter{}

// ---------------------------------------------------------------------------
// simpleTextPointTree
// ---------------------------------------------------------------------------

// simpleTextPointTree is the PointTree implementation for SimpleTextBKDReader.
// It walks the packed binary tree stored in splitPackedValues and dispatches
// leaf I/O to the plain-text block format.
//
// Port of SimpleTextBKDReader.SimpleTextPointTree.
type simpleTextPointTree struct {
	reader *SimpleTextBKDReader

	in store.IndexInput

	scratchDocIDs    []int
	scratchPackedVal []byte

	nodeID   int
	level    int
	rootNode int

	minPackedValue []byte
	maxPackedValue []byte

	// splitDimValueStack[l] saves the dimension byte overwritten at level l so
	// it can be restored on pop.
	splitDimValueStack [][]byte
	// splitDims[l] records the split dimension chosen at level l.
	splitDims []int
}

func newSimpleTextPointTree(
	r *SimpleTextBKDReader,
	in store.IndexInput,
	nodeID, level int,
	minPV, maxPV []byte,
) *simpleTextPointTree {
	treeDepth := bkdTreeDepth(r.leafNodeOffset)
	t := &simpleTextPointTree{
		reader:             r,
		in:                 in,
		scratchDocIDs:      make([]int, r.config.MaxPointsInLeafNode()),
		scratchPackedVal:   make([]byte, r.config.PackedBytesLength()),
		nodeID:             nodeID,
		rootNode:           nodeID,
		level:              level,
		minPackedValue:     append([]byte(nil), minPV...),
		maxPackedValue:     append([]byte(nil), maxPV...),
		splitDimValueStack: make([][]byte, treeDepth+1),
		splitDims:          make([]int, treeDepth+1),
	}
	return t
}

// bkdTreeDepth computes the required stack depth for a tree with numLeaves
// leaves. Mirrors Java's MathUtil.log(numLeaves, 2) + 2.
func bkdTreeDepth(numLeaves int) int {
	if numLeaves <= 0 {
		return 2
	}
	return bits.Len(uint(numLeaves)) + 1 // floor(log2) + 1 + 1
}

// Clone returns an independent copy of this cursor.
//
// Port of SimpleTextPointTree.clone().
func (t *simpleTextPointTree) Clone() bkd.PointTree {
	c := newSimpleTextPointTree(t.reader, t.in.Clone(), t.nodeID, t.level, t.minPackedValue, t.maxPackedValue)
	if !t.isLeafNode() {
		c.splitDims[t.level] = t.splitDims[t.level]
		c.splitDimValueStack[t.level] = t.splitDimValueStack[t.level]
	}
	return c
}

// MoveToChild descends to the left child.
func (t *simpleTextPointTree) MoveToChild() (bool, error) {
	if t.isLeafNode() {
		return false, nil
	}
	t.pushLeft()
	return true, nil
}

// MoveToSibling moves to the right sibling (only valid after MoveToChild).
func (t *simpleTextPointTree) MoveToSibling() (bool, error) {
	if t.nodeID != t.rootNode && (t.nodeID&1) == 0 {
		t.pop(true)
		t.pushRight()
		return true, nil
	}
	return false, nil
}

// MoveToParent ascends to the parent.
func (t *simpleTextPointTree) MoveToParent() (bool, error) {
	if t.nodeID == t.rootNode {
		return false, nil
	}
	t.pop((t.nodeID & 1) == 0)
	return true, nil
}

// GetMinPackedValue returns the current node's min packed value.
func (t *simpleTextPointTree) GetMinPackedValue() []byte { return t.minPackedValue }

// GetMaxPackedValue returns the current node's max packed value.
func (t *simpleTextPointTree) GetMaxPackedValue() []byte { return t.maxPackedValue }

// Size returns the number of points in the current subtree.
//
// Port of SimpleTextPointTree.size().
func (t *simpleTextPointTree) Size() int64 {
	leftmost := t.nodeID
	for leftmost < t.reader.leafNodeOffset {
		leftmost *= 2
	}
	rightmost := t.nodeID
	for rightmost < t.reader.leafNodeOffset {
		rightmost = 2*rightmost + 1
	}
	return t.sizeFromBalancedTree(leftmost, rightmost)
}

func (t *simpleTextPointTree) sizeFromBalancedTree(leftmost, rightmost int) int64 {
	leafNodeOffset := t.reader.leafNodeOffset
	maxPtsPerLeaf := int64(t.reader.config.MaxPointsInLeafNode())
	extraPoints := int64(maxPtsPerLeaf)*int64(leafNodeOffset) - t.reader.pointCount
	nodeOffset := int64(leafNodeOffset) - extraPoints
	var count int64
	for node := leftmost; node <= rightmost; node++ {
		if t.balanceTreeNodePosition(0, leafNodeOffset, node-leafNodeOffset, 0, 0) < int(nodeOffset) {
			count += maxPtsPerLeaf
		} else {
			count += maxPtsPerLeaf - 1
		}
	}
	return count
}

func (t *simpleTextPointTree) balanceTreeNodePosition(minNode, maxNode, node, position, level int) int {
	if maxNode-minNode == 1 {
		return position
	}
	mid := (minNode + maxNode + 1) >> 1
	if mid > node {
		return t.balanceTreeNodePosition(minNode, mid, node, position, level+1)
	}
	return t.balanceTreeNodePosition(mid, maxNode, node, position+(1<<level), level+1)
}

// VisitDocIDs visits every docID in the current subtree without values.
//
// Port of SimpleTextPointTree.visitDocIDs → addAll(visitor, false).
func (t *simpleTextPointTree) VisitDocIDs(visitor bkd.IntersectVisitor) error {
	return t.addAll(visitor, false)
}

func (t *simpleTextPointTree) addAll(visitor bkd.IntersectVisitor, grown bool) error {
	if !grown {
		sz := t.Size()
		if sz <= int64(^uint(0)>>1) {
			visitor.Grow(int(sz))
			grown = true
		}
	}
	if t.isLeafNode() {
		scratch := util.NewBytesRefBuilder()
		if err := t.in.SetPosition(t.reader.leafBlockFPs[t.nodeID-t.reader.leafNodeOffset]); err != nil {
			return err
		}
		if err := stReadLine(t.in, scratch); err != nil {
			return err
		}
		count, err := stParseInt(scratch.Bytes()[:scratch.Length()], len(PwBlockCount))
		if err != nil {
			return fmt.Errorf("simpleTextPointTree.addAll: parse block count: %w", err)
		}
		for i := 0; i < count; i++ {
			if err := stReadLine(t.in, scratch); err != nil {
				return err
			}
			docID, err := stParseInt(scratch.Bytes()[:scratch.Length()], len(PwBlockDocID))
			if err != nil {
				return fmt.Errorf("simpleTextPointTree.addAll: parse docID: %w", err)
			}
			if err := visitor.Visit(docID); err != nil {
				return err
			}
		}
		return nil
	}
	t.pushLeft()
	if err := t.addAll(visitor, grown); err != nil {
		return err
	}
	t.pop(true)
	t.pushRight()
	if err := t.addAll(visitor, grown); err != nil {
		return err
	}
	t.pop(false)
	return nil
}

// VisitDocValues visits every (docID, packedValue) in the current subtree.
//
// Port of SimpleTextPointTree.visitDocValues().
func (t *simpleTextPointTree) VisitDocValues(visitor bkd.IntersectVisitor) error {
	if t.isLeafNode() {
		leafID := t.nodeID - t.reader.leafNodeOffset
		count, err := t.readDocIDs(t.in, t.reader.leafBlockFPs[leafID], t.scratchDocIDs)
		if err != nil {
			return err
		}
		visitor.Grow(count)
		scratch := util.NewBytesRefBuilder()
		for i := 0; i < count; i++ {
			if err := stReadLine(t.in, scratch); err != nil {
				return err
			}
			line := scratch.Bytes()[:scratch.Length()]
			if !bytes.HasPrefix(line, PwBlockValue) {
				return fmt.Errorf("simpleTextPointTree.VisitDocValues: expected %q, got %q", PwBlockValue, line)
			}
			valueStr := string(line[len(PwBlockValue):])
			decoded, err := fromBytesRefString(valueStr)
			if err != nil {
				return fmt.Errorf("simpleTextPointTree.VisitDocValues: decode value: %w", err)
			}
			copy(t.scratchPackedVal, decoded)
			if err := visitor.VisitByPackedValue(t.scratchDocIDs[i], t.scratchPackedVal); err != nil {
				return err
			}
		}
		return nil
	}
	t.pushLeft()
	if err := t.VisitDocValues(visitor); err != nil {
		return err
	}
	t.pop(true)
	t.pushRight()
	if err := t.VisitDocValues(visitor); err != nil {
		return err
	}
	t.pop(false)
	return nil
}

// readDocIDs seeks to blockFP and reads count doc IDs into docIDs.
func (t *simpleTextPointTree) readDocIDs(in store.IndexInput, blockFP int64, docIDs []int) (int, error) {
	scratch := util.NewBytesRefBuilder()
	if err := in.SetPosition(blockFP); err != nil {
		return 0, err
	}
	if err := stReadLine(in, scratch); err != nil {
		return 0, err
	}
	count, err := stParseInt(scratch.Bytes()[:scratch.Length()], len(PwBlockCount))
	if err != nil {
		return 0, fmt.Errorf("simpleTextPointTree.readDocIDs: parse block count: %w", err)
	}
	for i := 0; i < count; i++ {
		if err := stReadLine(in, scratch); err != nil {
			return 0, err
		}
		docID, err := stParseInt(scratch.Bytes()[:scratch.Length()], len(PwBlockDocID))
		if err != nil {
			return 0, fmt.Errorf("simpleTextPointTree.readDocIDs: parse docID: %w", err)
		}
		docIDs[i] = docID
	}
	return count, nil
}

func (t *simpleTextPointTree) isLeafNode() bool {
	return t.nodeID >= t.reader.leafNodeOffset
}

// pushLeft descends to the left child, updating min/max packed values.
//
// Port of SimpleTextPointTree.pushLeft().
func (t *simpleTextPointTree) pushLeft() {
	r := t.reader
	address := t.nodeID * r.bytesPerIndexEntry
	if r.config.NumIndexDims() == 1 {
		t.splitDims[t.level] = 0
	} else {
		t.splitDims[t.level] = int(r.splitPackedValues[address] & 0xff)
		address++
	}
	splitDimPos := t.splitDims[t.level] * r.config.BytesPerDim()
	if t.splitDimValueStack[t.level] == nil {
		t.splitDimValueStack[t.level] = make([]byte, r.config.BytesPerDim())
	}
	// save current max dim value so we can restore on pop
	copy(t.splitDimValueStack[t.level], t.maxPackedValue[splitDimPos:splitDimPos+r.config.BytesPerDim()])
	t.nodeID *= 2
	t.level++
	// overwrite max with the split value
	copy(t.maxPackedValue[splitDimPos:], r.splitPackedValues[address:address+r.config.BytesPerDim()])
}

// pushRight descends to the right child, updating min/max packed values.
//
// Port of SimpleTextPointTree.pushRight().
func (t *simpleTextPointTree) pushRight() {
	r := t.reader
	address := t.nodeID * r.bytesPerIndexEntry
	if r.config.NumIndexDims() == 1 {
		t.splitDims[t.level] = 0
	} else {
		t.splitDims[t.level] = int(r.splitPackedValues[address] & 0xff)
		address++
	}
	splitDimPos := t.splitDims[t.level] * r.config.BytesPerDim()
	// save current min dim value so we can restore on pop
	copy(t.splitDimValueStack[t.level], t.minPackedValue[splitDimPos:splitDimPos+r.config.BytesPerDim()])
	t.nodeID = 2*t.nodeID + 1
	t.level++
	// overwrite min with the split value
	copy(t.minPackedValue[splitDimPos:], r.splitPackedValues[address:address+r.config.BytesPerDim()])
}

// pop ascends from the current node, restoring the overwritten dimension.
//
// Port of SimpleTextPointTree.pop(boolean).
func (t *simpleTextPointTree) pop(isLeft bool) {
	t.nodeID /= 2
	t.level--
	splitDimPos := t.splitDims[t.level] * t.reader.config.BytesPerDim()
	bpd := t.reader.config.BytesPerDim()
	if isLeft {
		copy(t.maxPackedValue[splitDimPos:splitDimPos+bpd], t.splitDimValueStack[t.level])
	} else {
		copy(t.minPackedValue[splitDimPos:splitDimPos+bpd], t.splitDimValueStack[t.level])
	}
}

// compile-time assertion.
var _ bkd.PointTree = (*simpleTextPointTree)(nil)

// ---------------------------------------------------------------------------
// fromBytesRefString — inverse of bytesRefString.
// ---------------------------------------------------------------------------

// fromBytesRefString parses a BytesRef.toString()-format string such as
// "[0 1f a3]" (space-separated lower-case hex bytes without zero-padding)
// back into a byte slice.
//
// Port of SimpleTextUtil.fromBytesRefString (Lucene 10.4.0).
func fromBytesRefString(s string) ([]byte, error) {
	if len(s) < 2 {
		return nil, fmt.Errorf("fromBytesRefString: too short: %q", s)
	}
	if s[0] != '[' || s[len(s)-1] != ']' {
		return nil, fmt.Errorf("fromBytesRefString: not bracketed: %q", s)
	}
	if len(s) == 2 {
		return []byte{}, nil
	}
	inner := s[1 : len(s)-1]
	parts := strings.Split(inner, " ")
	out := make([]byte, len(parts))
	for i, p := range parts {
		v, err := strconv.ParseUint(p, 16, 8)
		if err != nil {
			return nil, fmt.Errorf("fromBytesRefString: parse byte %q: %w", p, err)
		}
		out[i] = byte(v)
	}
	return out, nil
}
