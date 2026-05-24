// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of
// org.apache.lucene.sandbox.facet.plain.histograms.PointTreeBulkCollector.
package histograms

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/internal/hppc"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// PointTreeRelation mirrors PointValues.Relation for the histogram traversal.
type PointTreeRelation int

const (
	// PointTreeCellInsideQuery — entire cell is within the query range.
	PointTreeCellInsideQuery PointTreeRelation = iota
	// PointTreeCellCrossesQuery — cell partially overlaps the query range.
	PointTreeCellCrossesQuery
	// PointTreeCellOutsideQuery — cell is completely outside the query range.
	PointTreeCellOutsideQuery
)

// PointTree is the minimal interface over the BKD point tree that
// PointTreeBulkCollector needs. It mirrors the PointTree inner type of
// org.apache.lucene.index.PointValues.
//
// Callers that hold an index.PointValues must provide a bridge that
// implements this interface.
type PointTree interface {
	// GetMinPackedValue returns the minimum packed value for this node.
	GetMinPackedValue() []byte
	// GetMaxPackedValue returns the maximum packed value for this node.
	GetMaxPackedValue() []byte
	// Size returns the number of point values in this subtree.
	Size() int64
	// MoveToChild descends to the leftmost child. Returns true if the move
	// succeeded (i.e. this is not a leaf node).
	MoveToChild() (bool, error)
	// MoveToSibling moves to the next sibling. Returns true if the move
	// succeeded (i.e. another sibling exists).
	MoveToSibling() (bool, error)
	// MoveToParent ascends to the parent node.
	MoveToParent() error
	// VisitDocValues visits all leaf documents under this node, calling
	// visitor for each (docID, packedValue) pair.
	VisitDocValues(visitor PointDocValuesVisitor) error
}

// PointDocValuesVisitor is invoked for each (docID, packedValue) leaf entry
// when PointTreeBulkCollector traverses the PointTree manually.
type PointDocValuesVisitor interface {
	// VisitDocValue is called for each document in a leaf node.
	// Returning an error stops traversal; returning
	// ErrCollectionTerminated stops traversal and signals early
	// termination to the outer loop.
	VisitDocValue(docID int, packedValue []byte) error
}

// ErrCollectionTerminated is returned from leaf visitors (and propagated
// through PointTreeBulkCollector) to signal early termination in the same
// way as org.apache.lucene.search.CollectionTerminatedException.
var ErrCollectionTerminated = errors.New("collection terminated")

// PointRangeFilter carries the optional lower/upper bounds from a
// PointRangeQuery so that PointTreeBulkCollector can narrow collection.
// Both slices are packed big-endian sortable-bytes representations.
type PointRangeFilter struct {
	LowerPoint []byte
	UpperPoint []byte
}

// HistogramPointValues is the interface that the PointTree BKD root must
// satisfy in addition to PointTree traversal.
//
// Mirrors the subset of org.apache.lucene.index.PointValues used by
// PointTreeBulkCollector.
type HistogramPointValues interface {
	// GetNumDimensions returns the number of indexed dimensions.
	GetNumDimensions() int
	// GetDocCount returns the number of documents that have point values.
	GetDocCount() int64
	// Size returns the total number of point values across all documents.
	Size() int64
	// GetBytesPerDimension returns bytes per dimension (4 = int, 8 = long).
	GetBytesPerDimension() int
	// GetMinPackedValue returns the global minimum packed value.
	GetMinPackedValue() []byte
	// GetMaxPackedValue returns the global maximum packed value.
	GetMaxPackedValue() []byte
	// GetPointTree returns the traversable BKD tree root.
	GetPointTree() (PointTree, error)
}

// CanCollectEfficiently reports whether PointTreeBulkCollector can be used
// efficiently for the given point values and bucket width.
//
// Mirrors PointTreeBulkCollector.canCollectEfficiently.
func CanCollectEfficiently(pv HistogramPointValues, bucketWidth int64) bool {
	if pv == nil {
		return false
	}
	if pv.GetNumDimensions() != 1 {
		return false
	}
	if pv.GetDocCount() != pv.Size() {
		return false
	}
	conv := bytesToLong(pv.GetBytesPerDimension())
	if conv == nil {
		return false
	}
	leafMinBucket := floorDiv(conv(pv.GetMinPackedValue()), bucketWidth)
	leafMaxBucket := floorDiv(conv(pv.GetMaxPackedValue()), bucketWidth)
	// Only efficient when the leaf density is higher than the bucket count.
	return (pv.Size() / 512) >= (leafMaxBucket - leafMinBucket)
}

// Collect traverses the BKD PointTree rooted at pv, counting documents per
// histogram bucket. prq may be nil to collect all documents.
//
// Mirrors PointTreeBulkCollector.collect.
func Collect(
	pv HistogramPointValues,
	prq *PointRangeFilter,
	bucketWidth int64,
	counts hppc.LongIntHashMap,
	maxBuckets int,
) error {
	conv := bytesToLong(pv.GetBytesPerDimension())
	if conv == nil {
		return fmt.Errorf("PointTreeBulkCollector.Collect: unsupported bytes per dimension %d", pv.GetBytesPerDimension())
	}

	leafMin := conv(pv.GetMinPackedValue())
	leafMax := conv(pv.GetMaxPackedValue())
	if prq != nil {
		if lo := conv(prq.LowerPoint); lo > leafMin {
			leafMin = lo
		}
		if hi := conv(prq.UpperPoint); hi < leafMax {
			leafMax = hi
		}
	}

	mgr := newBucketManager(counts, leafMin, leafMax+1, bucketWidth, conv, maxBuckets)
	visitor := newIntersectVisitor(mgr)

	tree, err := pv.GetPointTree()
	if err != nil {
		return fmt.Errorf("PointTreeBulkCollector.Collect: get point tree: %w", err)
	}

	if err := intersectWithRanges(visitor, tree, mgr); err != nil && !errors.Is(err, ErrCollectionTerminated) {
		return err
	}
	mgr.finalizePreviousBucket(nil)
	return nil
}

// intersectWithRanges recursively traverses the tree.
func intersectWithRanges(visitor *histIntersectVisitor, tree PointTree, mgr *bucketManager) error {
	rel := visitor.compare(tree.GetMinPackedValue(), tree.GetMaxPackedValue())
	switch rel {
	case PointTreeCellInsideQuery:
		mgr.countNode(int(tree.Size()))
	case PointTreeCellCrossesQuery:
		moved, err := tree.MoveToChild()
		if err != nil {
			return err
		}
		if moved {
			for {
				if err := intersectWithRanges(visitor, tree, mgr); err != nil {
					return err
				}
				hasNext, err := tree.MoveToSibling()
				if err != nil {
					return err
				}
				if !hasNext {
					break
				}
			}
			if err := tree.MoveToParent(); err != nil {
				return err
			}
		} else {
			if err := tree.VisitDocValues(visitor); err != nil && !errors.Is(err, ErrCollectionTerminated) {
				return err
			}
			if errors.Is(err, ErrCollectionTerminated) {
				return ErrCollectionTerminated
			}
		}
	case PointTreeCellOutsideQuery:
		// nothing to do
	}
	return nil
}

// histIntersectVisitor handles the visitor callbacks from the tree traversal.
type histIntersectVisitor struct {
	mgr *bucketManager
}

func newIntersectVisitor(mgr *bucketManager) *histIntersectVisitor {
	return &histIntersectVisitor{mgr: mgr}
}

// VisitDocValue implements PointDocValuesVisitor for leaf nodes.
func (v *histIntersectVisitor) VisitDocValue(_ int, packedValue []byte) error {
	if !v.mgr.withinUpperBound(packedValue) {
		v.mgr.finalizePreviousBucket(packedValue)
		if !v.mgr.withinUpperBound(packedValue) {
			return ErrCollectionTerminated
		}
	}
	if v.mgr.withinRange(packedValue) {
		v.mgr.count()
	}
	return nil
}

// compare determines the relation of a cell to the current bucket window.
func (v *histIntersectVisitor) compare(minPacked, maxPacked []byte) PointTreeRelation {
	if !v.mgr.withinUpperBound(minPacked) {
		v.mgr.finalizePreviousBucket(minPacked)
		if !v.mgr.withinUpperBound(minPacked) {
			// Signals to caller that traversal is done.
			return PointTreeCellOutsideQuery
		}
	}
	if !v.mgr.withinLowerBound(maxPacked) {
		return PointTreeCellOutsideQuery
	}
	if v.mgr.withinRange(minPacked) && v.mgr.withinRange(maxPacked) {
		return PointTreeCellInsideQuery
	}
	return PointTreeCellCrossesQuery
}

// bucketManager tracks the current bucket window and accumulates counts.
// Mirrors the private BucketManager inner class in PointTreeBulkCollector.
type bucketManager struct {
	counts      hppc.LongIntHashMap
	counter     int
	startValue  int64
	maxValue    int64
	endValue    int64
	nonZero     int
	maxBuckets  int
	byteToLong  func([]byte) int64
	bucketWidth int64
}

func newBucketManager(
	counts hppc.LongIntHashMap,
	minValue, maxValue, bucketWidth int64,
	byteToLong func([]byte) int64,
	maxBuckets int,
) *bucketManager {
	startValue := minValue
	endValue := minInt64(
		(floorDiv(startValue, bucketWidth)+1)*bucketWidth,
		maxValue,
	)
	return &bucketManager{
		counts:      counts,
		bucketWidth: bucketWidth,
		startValue:  startValue,
		endValue:    endValue,
		maxValue:    maxValue,
		byteToLong:  byteToLong,
		maxBuckets:  maxBuckets,
	}
}

func (m *bucketManager) count() {
	m.counter++
}

func (m *bucketManager) countNode(n int) {
	m.counter += n
}

func (m *bucketManager) finalizePreviousBucket(packedValue []byte) {
	if m.counter > 0 {
		key := floorDiv(m.startValue, m.bucketWidth)
		m.counts[key] += int32(m.counter)
		m.nonZero++
		m.counter = 0
		CheckMaxBuckets(m.nonZero, m.maxBuckets)
	}
	if packedValue != nil {
		m.startValue = m.byteToLong(packedValue)
		m.endValue = minInt64(
			(floorDiv(m.startValue, m.bucketWidth)+1)*m.bucketWidth,
			m.maxValue,
		)
	}
}

func (m *bucketManager) withinLowerBound(value []byte) bool {
	return m.byteToLong(value) >= m.startValue
}

func (m *bucketManager) withinUpperBound(value []byte) bool {
	return m.byteToLong(value) < m.endValue
}

func (m *bucketManager) withinRange(value []byte) bool {
	return m.withinLowerBound(value) && m.withinUpperBound(value)
}

// bytesToLong returns the conversion function for the given bytes-per-dimension.
// Returns nil for unsupported sizes (mirrors the Java static method).
func bytesToLong(numBytes int) func([]byte) int64 {
	switch numBytes {
	case 8: // long/double — LongPoint, DoublePoint
		return func(b []byte) int64 { return util.SortableBytesToLong(b, 0) }
	case 4: // int/float — IntPoint, FloatPoint, LatLonPoint
		return func(b []byte) int64 { return int64(util.SortableBytesToInt(b, 0)) }
	}
	return nil
}

// floorDiv mirrors Java's Math.floorDiv for int64.
func floorDiv(a, b int64) int64 {
	q := a / b
	if (a^b) < 0 && q*b != a {
		q--
	}
	return q
}

// minInt64 returns the smaller of a and b.
func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
