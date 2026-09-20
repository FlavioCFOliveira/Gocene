// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// PointValues gives access to indexed numeric values.
//
// Points represent numeric values and are indexed differently than ordinary
// text. Instead of an inverted index, points are indexed with datastructures
// such as KD-trees. These structures are optimized for operations such as
// range, distance, nearest-neighbor, and point-in-polygon queries.
//
// This is the Go port of the abstract class
// org.apache.lucene.index.PointValues from Apache Lucene 10.5.0.
//
// Lucene declares it in org.apache.lucene.index; index.PointValues is an
// alias of this type, so the Lucene name stays available in the package
// Lucene declares it in. The canonical declaration lives here because
// [LeafReader.GetPointValues] returns it and org.apache.lucene.util.bkd and
// org.apache.lucene.codecs both consume it, so neither index nor codecs can
// own it without a Go import cycle.
//
// Intersect, EstimatePointCount and EstimateDocCount are `public final` on
// the Java class rather than abstract. They are part of the interface because
// every Java PointValues reference carries them; implementations obtain the
// Java bodies by embedding [BasePointValues], exactly as the codec writers
// obtain PointsWriter.merge from codecs.BasePointsWriter.
//
// The static members size(IndexReader, String), getDocCount(IndexReader,
// String), getMinPackedValue(IndexReader, String) and
// getMaxPackedValue(IndexReader, String), and the constants MAX_NUM_BYTES,
// MAX_DIMENSIONS and MAX_INDEX_DIMENSIONS, are declared in package index —
// the package Lucene declares them in — because they reference IndexReader
// and org.apache.lucene.util.bkd.BKDConfig, which this package cannot import.
type PointValues interface {
	// GetPointTree creates a new [PointTree] to navigate the index.
	// Renders `public abstract PointTree getPointTree()`.
	GetPointTree() (PointTree, error)

	// Intersect finds all documents and points matching the provided
	// visitor. This method does not enforce live documents, so it is up to
	// the caller to test whether each document is deleted, if necessary.
	// Renders `public final void intersect(IntersectVisitor)`; the body is
	// [BasePointValues.Intersect].
	Intersect(visitor IntersectVisitor) error

	// EstimatePointCount estimates the number of points that would be
	// visited by [PointValues.Intersect] with the given visitor. This runs
	// many times faster than Intersect. Renders `public final long
	// estimatePointCount(IntersectVisitor)`; the body is
	// [BasePointValues.EstimatePointCount]. Java wraps the IOException in an
	// UncheckedIOException; Go reports it through the error result.
	EstimatePointCount(visitor IntersectVisitor) (int64, error)

	// EstimateDocCount estimates the number of documents that would be
	// matched by [PointValues.Intersect] with the given visitor. This runs
	// many times faster than Intersect. Renders `public final long
	// estimateDocCount(IntersectVisitor)`; the body is
	// [BasePointValues.EstimateDocCount].
	EstimateDocCount(visitor IntersectVisitor) (int64, error)

	// GetMinPackedValue returns the minimum value for each dimension,
	// packed, or nil if [PointValues.Size] is 0. Renders `public abstract
	// byte[] getMinPackedValue()`.
	GetMinPackedValue() ([]byte, error)

	// GetMaxPackedValue returns the maximum value for each dimension,
	// packed, or nil if [PointValues.Size] is 0. Renders `public abstract
	// byte[] getMaxPackedValue()`.
	GetMaxPackedValue() ([]byte, error)

	// GetNumDimensions returns how many dimensions are represented in the
	// values. Renders `public abstract int getNumDimensions()`.
	GetNumDimensions() (int, error)

	// GetNumIndexDimensions returns how many dimensions are used for the
	// index. Renders `public abstract int getNumIndexDimensions()`.
	GetNumIndexDimensions() (int, error)

	// GetBytesPerDimension returns the number of bytes per dimension.
	// Renders `public abstract int getBytesPerDimension()`.
	GetBytesPerDimension() (int, error)

	// Size returns the total number of indexed points across all documents.
	// Renders `public abstract long size()`.
	Size() int64

	// GetDocCount returns the total number of documents that have indexed at
	// least one point. Renders `public abstract int getDocCount()`.
	GetDocCount() int
}

// Relation is used by [PointValues.Intersect] to check how each recursive
// cell corresponds to the query. It is the Go port of the nested enum
// org.apache.lucene.index.PointValues.Relation from Apache Lucene 10.5.0
// (PointValues.java:223-230). index.Relation is an alias of this type, so the
// Lucene name stays available in the package Lucene declares it in.
//
// The canonical declaration lives here, next to [PointValues], [PointTree] and
// [IntersectVisitor], and for the same reason: org.apache.lucene.geo,
// org.apache.lucene.util.bkd, org.apache.lucene.codecs and
// org.apache.lucene.index all consume it, so no one of those Go packages can
// own it without an import cycle.
//
// The declaration order is Lucene's, so the ordinals are Lucene's:
// CELL_INSIDE_QUERY is 0, CELL_OUTSIDE_QUERY is 1, CELL_CROSSES_QUERY is 2.
// The order is load-bearing — org.apache.lucene.geo.GeoEncodingUtils caches
// relations as ordinals in its sub-box grid (GeoEncodingUtils.java:273-276 and
// :379-380).
type Relation int

const (
	// CellInsideQuery is returned if the cell is fully contained by the
	// query. Renders CELL_INSIDE_QUERY.
	CellInsideQuery Relation = iota
	// CellOutsideQuery is returned if the cell and query do not overlap.
	// Renders CELL_OUTSIDE_QUERY.
	CellOutsideQuery
	// CellCrossesQuery is returned if the cell partially overlaps the query.
	// Renders CELL_CROSSES_QUERY.
	CellCrossesQuery
)

// String renders the implicit java.lang.Enum#toString of
// org.apache.lucene.index.PointValues.Relation, which returns the constant's
// declared name.
func (r Relation) String() string {
	switch r {
	case CellInsideQuery:
		return "CELL_INSIDE_QUERY"
	case CellOutsideQuery:
		return "CELL_OUTSIDE_QUERY"
	case CellCrossesQuery:
		return "CELL_CROSSES_QUERY"
	default:
		return "UNKNOWN"
	}
}

// PointTree carries the basic operations to read the KD-tree. It is the Go
// port of the nested interface
// org.apache.lucene.index.PointValues.PointTree from Apache Lucene 10.5.0
// (which extends Cloneable). index.PointTree and bkd.PointTree are aliases of
// this type.
type PointTree interface {
	// Clone clones the tree; the current node becomes the root of the new
	// tree. Renders `PointTree clone()`.
	Clone() PointTree

	// MoveToChild moves to the first child node and returns true upon
	// success. Returns false for leaf nodes and true otherwise.
	MoveToChild() (bool, error)

	// MoveToSibling moves to the next sibling node and returns true upon
	// success. Returns false if the current node has no more siblings.
	MoveToSibling() (bool, error)

	// MoveToParent moves to the parent node and returns true upon success.
	// Returns false for the root node and true otherwise.
	MoveToParent() (bool, error)

	// GetMinPackedValue returns the minimum packed value of the current node.
	GetMinPackedValue() []byte

	// GetMaxPackedValue returns the maximum packed value of the current node.
	GetMaxPackedValue() []byte

	// Size returns the number of points below the current node.
	Size() int64

	// VisitDocIDs visits all the docs below the current node.
	VisitDocIDs(visitor IntersectVisitor) error

	// VisitDocValues visits all the docs and values below the current node.
	VisitDocValues(visitor IntersectVisitor) error
}

// IntersectVisitor guides the recursion of a [PointTree] walk. It is the Go
// port of the nested interface
// org.apache.lucene.index.PointValues.IntersectVisitor from Apache Lucene
// 10.5.0. index.IntersectVisitor, codecs.IntersectVisitor and
// bkd.IntersectVisitor are aliases of this type.
//
// Java overloads visit five ways; Go has no overloading, so each overload is
// named after the parameters it takes. Three of the five are `default`
// methods: Go has no inherited default, so an implementation that does not
// override them delegates to [DefaultVisitByDocIDSetIterator],
// [DefaultVisitByIntsRef] and [DefaultVisitByDocIDSetIteratorAndPackedValue]
// below, which carry the Java bodies verbatim. Grow is likewise `default`,
// with an empty body.
type IntersectVisitor interface {
	// Visit is called for all documents in a leaf cell that is fully
	// contained by the query. The consumer should blindly accept the docID.
	// Renders `void visit(int docID)`.
	Visit(docID int) error

	// VisitByDocIDSetIterator is a bulk visit, and implementations may have
	// their optimizations. It is guaranteed that the given iterator is not
	// positioned. Renders `default void visit(DocIdSetIterator iterator)`.
	VisitByDocIDSetIterator(iterator DocIdSetIterator) error

	// VisitByIntsRef is a bulk visit, and implementations may have their
	// optimizations. Even if the implementation does the same thing as
	// [IntersectVisitor.Visit], this may be a speed improvement due to fewer
	// virtual calls. Renders `default void visit(IntsRef ref)`.
	VisitByIntsRef(ref *util.IntsRef) error

	// VisitByPackedValue is called for all documents in a leaf cell that
	// crosses the query. The consumer should scrutinize the packedValue to
	// decide whether to accept it. In the 1D case, values are visited in
	// increasing order, and in the case of ties, in increasing docID order.
	// Renders `void visit(int docID, byte[] packedValue)`.
	VisitByPackedValue(docID int, packedValue []byte) error

	// VisitByDocIDSetIteratorAndPackedValue is like
	// [IntersectVisitor.VisitByPackedValue], but here the packedValue can
	// have more than one docID associated to it. The provided iterator
	// should not escape the scope of this method so that implementations of
	// PointValues are free to reuse it. Renders `default void
	// visit(DocIdSetIterator iterator, byte[] packedValue)`.
	VisitByDocIDSetIteratorAndPackedValue(iterator DocIdSetIterator, packedValue []byte) error

	// Compare is called for non-leaf cells to test how the cell relates to
	// the query, to determine how to further recurse down the tree. Renders
	// `Relation compare(byte[] minPackedValue, byte[] maxPackedValue)`.
	Compare(minPackedValue, maxPackedValue []byte) Relation

	// Grow notifies the caller that this many documents are about to be
	// visited. Renders `default void grow(int count)`, whose body is empty.
	Grow(count int)
}

// DefaultVisitByDocIDSetIterator is the body of the default method
// IntersectVisitor.visit(DocIdSetIterator) in Apache Lucene 10.5.0:
//
//	int docID;
//	while ((docID = iterator.nextDoc()) != DocIdSetIterator.NO_MORE_DOCS) {
//	  visit(docID);
//	}
//
// It is a free function rather than a method on an embeddable struct because
// the Java body dispatches back to the abstract visit(int), which an embedded
// Go struct cannot reach.
func DefaultVisitByDocIDSetIterator(v IntersectVisitor, iterator DocIdSetIterator) error {
	for {
		docID, err := iterator.NextDoc()
		if err != nil {
			return err
		}
		if docID == NO_MORE_DOCS {
			return nil
		}
		if err := v.Visit(docID); err != nil {
			return err
		}
	}
}

// DefaultVisitByIntsRef is the body of the default method
// IntersectVisitor.visit(IntsRef) in Apache Lucene 10.5.0:
//
//	for (int i = ref.offset; i < ref.length + ref.offset; i++) {
//	  visit(ref.ints[i]);
//	}
//
// It is a free function for the same reason as
// [DefaultVisitByDocIDSetIterator].
func DefaultVisitByIntsRef(v IntersectVisitor, ref *util.IntsRef) error {
	for i := ref.Offset; i < ref.Length+ref.Offset; i++ {
		if err := v.Visit(ref.Ints[i]); err != nil {
			return err
		}
	}
	return nil
}

// DefaultVisitByDocIDSetIteratorAndPackedValue is the body of the default
// method IntersectVisitor.visit(DocIdSetIterator, byte[]) in Apache Lucene
// 10.5.0:
//
//	int docID;
//	while ((docID = iterator.nextDoc()) != DocIdSetIterator.NO_MORE_DOCS) {
//	  visit(docID, packedValue);
//	}
//
// It is a free function for the same reason as
// [DefaultVisitByDocIDSetIterator].
func DefaultVisitByDocIDSetIteratorAndPackedValue(v IntersectVisitor, iterator DocIdSetIterator, packedValue []byte) error {
	for {
		docID, err := iterator.NextDoc()
		if err != nil {
			return err
		}
		if docID == NO_MORE_DOCS {
			return nil
		}
		if err := v.VisitByPackedValue(docID, packedValue); err != nil {
			return err
		}
	}
}

// BasePointValues carries the `public final` members of the abstract class
// org.apache.lucene.index.PointValues: intersect, estimatePointCount and
// estimateDocCount. Every concrete PointValues embeds a BasePointValues built
// with [NewBasePointValues], passing itself as impl: impl is the receiver on
// which these bodies invoke getPointTree, size and getDocCount, which Java
// dispatches through this.
type BasePointValues struct {
	impl PointValues
}

// NewBasePointValues returns the base of the PointValues impl. Mirrors the
// protected constructor PointValues().
func NewBasePointValues(impl PointValues) *BasePointValues {
	return &BasePointValues{impl: impl}
}

// Intersect renders `public final void intersect(IntersectVisitor visitor)`.
func (b *BasePointValues) Intersect(visitor IntersectVisitor) error {
	pointTree, err := b.impl.GetPointTree()
	if err != nil {
		return err
	}
	return intersectPointTree(visitor, pointTree)
}

// intersectPointTree renders the private static
// PointValues.intersect(IntersectVisitor, PointTree).
func intersectPointTree(visitor IntersectVisitor, pointTree PointTree) error {
	for {
		compare := visitor.Compare(pointTree.GetMinPackedValue(), pointTree.GetMaxPackedValue())
		if compare == CellInsideQuery {
			// This cell is fully inside the query shape: recursively add all
			// points in this cell without filtering
			if err := pointTree.VisitDocIDs(visitor); err != nil {
				return err
			}
		} else if compare == CellCrossesQuery {
			// The cell crosses the shape boundary, or the cell fully contains
			// the query, so we fall through and do full filtering:
			moved, err := pointTree.MoveToChild()
			if err != nil {
				return err
			}
			if moved {
				continue
			}
			// Leaf node; scan and filter all points in this block:
			if err := pointTree.VisitDocValues(visitor); err != nil {
				return err
			}
		}
		for {
			moved, err := pointTree.MoveToSibling()
			if err != nil {
				return err
			}
			if moved {
				break
			}
			moved, err = pointTree.MoveToParent()
			if err != nil {
				return err
			}
			if !moved {
				return nil
			}
		}
	}
}

// IsEstimatedPointCountGreaterThanOrEqualTo estimates if the point count that
// would be matched by [PointValues.Intersect] with the given visitor is
// greater than or equal to upperBound. Renders `public static boolean
// isEstimatedPointCountGreaterThanOrEqualTo(IntersectVisitor, PointTree,
// long)`.
func IsEstimatedPointCountGreaterThanOrEqualTo(visitor IntersectVisitor, pointTree PointTree, upperBound int64) (bool, error) {
	count, err := estimatePointCount(visitor, pointTree, upperBound)
	if err != nil {
		return false, err
	}
	return count >= upperBound, nil
}

// EstimatePointCount renders `public final long
// estimatePointCount(IntersectVisitor visitor)`.
func (b *BasePointValues) EstimatePointCount(visitor IntersectVisitor) (int64, error) {
	pointTree, err := b.impl.GetPointTree()
	if err != nil {
		return 0, err
	}
	return estimatePointCount(visitor, pointTree, math.MaxInt64)
}

// estimatePointCount renders the private static
// PointValues.estimatePointCount(IntersectVisitor, PointTree, long). The
// estimation terminates when the point count gets greater than or equal to
// upperBound.
func estimatePointCount(visitor IntersectVisitor, pointTree PointTree, upperBound int64) (int64, error) {
	r := visitor.Compare(pointTree.GetMinPackedValue(), pointTree.GetMaxPackedValue())
	switch r {
	case CellOutsideQuery:
		// This cell is fully outside the query shape: no points added
		return 0, nil
	case CellInsideQuery:
		// This cell is fully inside the query shape: add all points
		return pointTree.Size(), nil
	case CellCrossesQuery:
		// The cell crosses the shape boundary: keep recursing
		moved, err := pointTree.MoveToChild()
		if err != nil {
			return 0, err
		}
		if moved {
			var cost int64
			for {
				sub, err := estimatePointCount(visitor, pointTree, upperBound-cost)
				if err != nil {
					return 0, err
				}
				cost += sub
				if cost >= upperBound {
					break
				}
				sibling, err := pointTree.MoveToSibling()
				if err != nil {
					return 0, err
				}
				if !sibling {
					break
				}
			}
			if _, err := pointTree.MoveToParent(); err != nil {
				return 0, err
			}
			return cost, nil
		}
		// Assume half the points matched
		return (pointTree.Size() + 1) / 2, nil
	default:
		return 0, fmt.Errorf("unreachable code")
	}
}

// EstimateDocCount renders `public final long
// estimateDocCount(IntersectVisitor visitor)`.
func (b *BasePointValues) EstimateDocCount(visitor IntersectVisitor) (int64, error) {
	estimatedPointCount, err := b.EstimatePointCount(visitor)
	if err != nil {
		return 0, err
	}
	docCount := b.impl.GetDocCount()
	size := float64(b.impl.Size())
	if float64(estimatedPointCount) >= size {
		// math all docs
		return int64(docCount), nil
	} else if size == float64(docCount) || estimatedPointCount == 0 {
		// if the point count estimate is 0 or we have only single values
		// return this estimate
		return estimatedPointCount, nil
	}
	// in case of multi values estimate the number of docs using the solution
	// provided in
	// https://math.stackexchange.com/questions/1175295/urn-problem-probability-of-drawing-balls-of-k-unique-colors
	// then approximate the solution for points per doc << size() which results
	// in the expression
	// D * (1 - ((N - n) / N)^(N/D))
	// where D is the total number of docs, N the total number of points and n
	// the estimated point count
	docEstimate := int64(float64(docCount) *
		(1.0 - math.Pow((size-float64(estimatedPointCount))/size, size/float64(docCount))))
	if docEstimate == 0 {
		return 1, nil
	}
	return docEstimate, nil
}
