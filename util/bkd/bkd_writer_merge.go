// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package bkd

import (
	"errors"
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// This file ports the bulk-merge members of
// org.apache.lucene.util.bkd.BKDWriter (Apache Lucene 10.5.0): the merge
// method and the private nested classes MergeReader, MergeIntersectsVisitor
// and BKDMergeQueue that it drives.

// errMergeIntersectsVisitorVisitDocID renders the
// UnsupportedOperationException thrown by MergeIntersectsVisitor.visit(int).
var errMergeIntersectsVisitorVisitDocID = errors.New("bkd: MergeIntersectsVisitor.visit(int) is unsupported")

// Merge is a more efficient bulk-add for incoming PointValues. It does a merge
// sort of the already sorted values and currently only works when numDims==1.
// It returns a nil runnable if all documents containing dimensional values
// were deleted.
//
// docMaps may be nil; otherwise it holds one DocMap per reader, in the same
// order.
//
// Port of BKDWriter.merge(IndexOutput, IndexOutput, IndexOutput,
// List<MergeState.DocMap>, List<PointValues>).
func (w *BKDWriter) Merge(
	metaOut, indexOut, dataOut store.IndexOutput,
	docMaps []spi.DocMap,
	readers []spi.PointValues,
) (IORunnable, error) {
	queue, err := newBKDMergeQueue(w.config.BytesPerDim(), len(readers))
	if err != nil {
		return nil, err
	}

	for i, pointValues := range readers {
		var docMap spi.DocMap
		if docMaps != nil {
			docMap = docMaps[i]
		}
		reader, err := newMergeReader(pointValues, docMap)
		if err != nil {
			return nil, err
		}
		hasNext, err := reader.next()
		if err != nil {
			return nil, err
		}
		if hasNext {
			queue.Add(reader)
		}
	}

	oneDimWriter := newOneDimensionBKDWriter(w, metaOut, indexOut, dataOut)

	for queue.Size() != 0 {
		reader := queue.Top()

		if err := oneDimWriter.add(reader.packedValue, reader.docID); err != nil {
			return nil, err
		}

		hasNext, err := reader.next()
		if err != nil {
			return nil, err
		}
		if hasNext {
			queue.UpdateTop()
		} else {
			// This segment was exhausted
			queue.Pop()
		}
	}

	return oneDimWriter.finish()
}

// mergeReader is the port of the private static class BKDWriter.MergeReader.
type mergeReader struct {
	pointTree              PointTree
	packedBytesLength      int
	docMap                 spi.DocMap
	mergeIntersectsVisitor *mergeIntersectsVisitor

	// docBlockUpto is which doc in this block we are up to.
	docBlockUpto int

	// docID is the current doc ID.
	docID int

	// packedValue is the current packed value.
	packedValue []byte
}

// newMergeReader renders the MergeReader(PointValues, MergeState.DocMap)
// constructor.
func newMergeReader(pointValues spi.PointValues, docMap spi.DocMap) (*mergeReader, error) {
	bytesPerDim, err := pointValues.GetBytesPerDimension()
	if err != nil {
		return nil, err
	}
	numDims, err := pointValues.GetNumDimensions()
	if err != nil {
		return nil, err
	}
	packedBytesLength := bytesPerDim * numDims
	pointTree, err := pointValues.GetPointTree()
	if err != nil {
		return nil, err
	}
	visitor := newMergeIntersectsVisitor(packedBytesLength)
	// move to first child of the tree and collect docs
	for {
		moved, err := pointTree.MoveToChild()
		if err != nil {
			return nil, err
		}
		if !moved {
			break
		}
	}
	if err := pointTree.VisitDocValues(visitor); err != nil {
		return nil, err
	}
	return &mergeReader{
		pointTree:              pointTree,
		packedBytesLength:      packedBytesLength,
		docMap:                 docMap,
		mergeIntersectsVisitor: visitor,
		packedValue:            make([]byte, packedBytesLength),
	}, nil
}

// next renders MergeReader.next().
func (r *mergeReader) next() (bool, error) {
	for {
		if r.docBlockUpto == r.mergeIntersectsVisitor.docsInBlock {
			collected, err := r.collectNextLeaf()
			if err != nil {
				return false, err
			}
			if !collected {
				return false, nil
			}
			r.docBlockUpto = 0
		}

		index := r.docBlockUpto
		r.docBlockUpto++
		oldDocID := r.mergeIntersectsVisitor.docIDs[index]

		var mappedDocID int
		if r.docMap == nil {
			mappedDocID = oldDocID
		} else {
			mappedDocID = r.docMap.Get(oldDocID)
		}

		if mappedDocID != -1 {
			// Not deleted!
			r.docID = mappedDocID
			copy(r.packedValue,
				r.mergeIntersectsVisitor.packedValues[index*r.packedBytesLength:(index+1)*r.packedBytesLength])
			return true, nil
		}
	}
}

// collectNextLeaf renders MergeReader.collectNextLeaf().
func (r *mergeReader) collectNextLeaf() (bool, error) {
	r.mergeIntersectsVisitor.reset()
	for {
		moved, err := r.pointTree.MoveToSibling()
		if err != nil {
			return false, err
		}
		if moved {
			// move to first child of this node and collect docs
			for {
				child, err := r.pointTree.MoveToChild()
				if err != nil {
					return false, err
				}
				if !child {
					break
				}
			}
			if err := r.pointTree.VisitDocValues(r.mergeIntersectsVisitor); err != nil {
				return false, err
			}
			return true, nil
		}
		up, err := r.pointTree.MoveToParent()
		if err != nil {
			return false, err
		}
		if !up {
			return false, nil
		}
	}
}

// mergeIntersectsVisitor is the port of the private static class
// BKDWriter.MergeIntersectsVisitor.
type mergeIntersectsVisitor struct {
	docsInBlock       int
	packedValues      []byte
	docIDs            []int
	packedBytesLength int
}

func newMergeIntersectsVisitor(packedBytesLength int) *mergeIntersectsVisitor {
	return &mergeIntersectsVisitor{
		docIDs:            []int{},
		packedValues:      []byte{},
		packedBytesLength: packedBytesLength,
	}
}

func (v *mergeIntersectsVisitor) reset() {
	v.docsInBlock = 0
}

// Grow renders MergeIntersectsVisitor.grow(int). Java throws
// ArithmeticException (Math.toIntExact) and IllegalStateException, both
// unchecked, from this void method; they are rendered as panics.
func (v *mergeIntersectsVisitor) Grow(count int) {
	if len(v.docIDs) < count {
		// ArrayUtil.grow(docIDs, count)
		v.docIDs = util.GrowExact(v.docIDs, util.Oversize(count, 4))
		size := int64(len(v.docIDs)) * int64(v.packedBytesLength)
		if size > math.MaxInt32 {
			panic("integer overflow")
		}
		packedValuesSize := int(size)
		if packedValuesSize > util.MaxArrayLength {
			panic(fmt.Sprintf("array length must be <= to %d but was: %d", util.MaxArrayLength, packedValuesSize))
		}
		v.packedValues = util.GrowExact(v.packedValues, packedValuesSize)
	}
}

func (v *mergeIntersectsVisitor) Visit(docID int) error {
	return errMergeIntersectsVisitorVisitDocID
}

func (v *mergeIntersectsVisitor) VisitByPackedValue(docID int, packedValue []byte) error {
	copy(v.packedValues[v.docsInBlock*v.packedBytesLength:(v.docsInBlock+1)*v.packedBytesLength],
		packedValue[:v.packedBytesLength])
	v.docIDs[v.docsInBlock] = docID
	v.docsInBlock++
	return nil
}

func (v *mergeIntersectsVisitor) Compare(minPackedValue, maxPackedValue []byte) spi.Relation {
	return spi.CellCrossesQuery
}

// bkdMergeQueue is the port of the private static class BKDWriter.BKDMergeQueue,
// a PriorityQueue<MergeReader> ordered by packed value, then by docID.
type bkdMergeQueue struct {
	*util.PriorityQueue[*mergeReader]
	comparator ByteArrayComparator
}

func newBKDMergeQueue(bytesPerDim, maxSize int) (*bkdMergeQueue, error) {
	q := &bkdMergeQueue{comparator: GetUnsignedComparator(bytesPerDim)}
	pq, err := util.NewPriorityQueue[*mergeReader](maxSize, q.lessThan)
	if err != nil {
		return nil, err
	}
	q.PriorityQueue = pq
	return q, nil
}

// lessThan renders BKDMergeQueue.lessThan(MergeReader, MergeReader).
func (q *bkdMergeQueue) lessThan(a, b *mergeReader) bool {
	cmp := q.comparator(a.packedValue, 0, b.packedValue, 0)

	if cmp < 0 {
		return true
	} else if cmp > 0 {
		return false
	}

	// Tie break by sorting smaller docIDs earlier:
	return a.docID < b.docID
}

var _ IntersectVisitor = (*mergeIntersectsVisitor)(nil)

// VisitByDocIDSetIterator renders the default body of
// PointValues.IntersectVisitor.visit(DocIdSetIterator), which v does not
// override.
func (v *mergeIntersectsVisitor) VisitByDocIDSetIterator(iterator spi.DocIdSetIterator) error {
	return spi.DefaultVisitByDocIDSetIterator(v, iterator)
}

// VisitByIntsRef renders the default body of
// PointValues.IntersectVisitor.visit(IntsRef), which v does not override.
func (v *mergeIntersectsVisitor) VisitByIntsRef(ref *util.IntsRef) error {
	return spi.DefaultVisitByIntsRef(v, ref)
}

// VisitByDocIDSetIteratorAndPackedValue renders the default body of
// PointValues.IntersectVisitor.visit(DocIdSetIterator, byte[]), which v
// does not override.
func (v *mergeIntersectsVisitor) VisitByDocIDSetIteratorAndPackedValue(iterator spi.DocIdSetIterator, packedValue []byte) error {
	return spi.DefaultVisitByDocIDSetIteratorAndPackedValue(v, iterator, packedValue)
}
