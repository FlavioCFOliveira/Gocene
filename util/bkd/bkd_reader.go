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
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Port of org.apache.lucene.util.bkd.BKDReader (Lucene 10.4.0).
//
// BKDReader handles reading a block KD-tree previously written with
// BKDWriter. The on-disk format is partitioned across three streams
// (meta, index, data) and BKDReader stitches them together into a
// PointTree-style cursor that callers walk to answer point queries.
//
// Local interface surface
//
// The Java reference returns its tree through PointValues.PointTree
// and dispatches visitor callbacks through PointValues.IntersectVisitor.
// Those types are owned by the lucene.index PointValues hierarchy
// which is not yet ported in Gocene; the only existing peer,
// codecs.IntersectVisitor, lacks the IntsRef bulk-visit path that
// BKDReader uses on CELL_INSIDE_QUERY leaves.
//
// To keep this port self-contained — and to follow the precedent set
// by [MutablePointTree] in mutable_point_tree_reader_utils.go — we
// expose:
//
//   - [IntersectVisitor]: the minimum visitor contract used by the
//     reader; it carries Visit(docID), Visit(docID, packedValue),
//     Compare(min, max), and Grow(count). It deliberately omits the
//     IntsRef and DocIdSetIterator bulk-visit hooks: the few sites
//     that would benefit from them degrade to a per-doc loop, which
//     is functionally identical (just one extra method-dispatch per
//     doc on full-cell-inside matches). This will be replaced by the
//     canonical lucene.index PointValues.IntersectVisitor when the
//     broader PointValues port lands.
//
//   - [PointTree]: the cursor returned by GetPointTree(). It mirrors
//     PointValues.PointTree (clone / size / moveToChild / moveToSibling
//     / moveToParent / visitDocIDs / visitDocValues / getMin / getMax).
//
// The reader uses codecs.Relation for the visitor compare() result.

// IntersectVisitor is the narrow visitor surface used by BKDReader to
// emit results during point intersection. It mirrors the subset of
// org.apache.lucene.index.PointValues.IntersectVisitor that BKDReader
// actually invokes for indices written by versions >= VERSION_META_FILE.
//
// Implementations must be ready to be called from any of the three
// methods below in any order during a single Intersect() pass.
//
// This interface is a temporary local surface; it will be unified
// with — or replaced by — the canonical type once the
// lucene.index PointValues port lands. See the file-level docstring.
type IntersectVisitor interface {
	// Visit is called for each doc that matches the query, when
	// individual packed values are not available (e.g. visitDocIDs).
	Visit(docID int) error

	// VisitByPackedValue is called for each doc that matches the query,
	// with the doc's packed value. The packedValue buffer is reused
	// across calls; visitors that need to retain it must copy.
	VisitByPackedValue(docID int, packedValue []byte) error

	// Compare returns the relation between [minPackedValue, maxPackedValue]
	// and the query cell.
	Compare(minPackedValue, maxPackedValue []byte) codecs.Relation

	// Grow is a hint that the visitor will receive at least count more
	// matches in the current sub-walk. Implementations are free to
	// ignore it; most use it to size output buffers.
	Grow(count int)
}

// PointTree is the cursor returned by [BKDReader.GetPointTree]. It
// represents a current position in the BKD tree (root by default) and
// supports navigation (moveToChild / moveToSibling / moveToParent),
// cloning (to fan-out a recursive intersection), size queries, and
// leaf consumption (visitDocIDs for the docID-only path, visitDocValues
// for the full packed-value path).
//
// Mirrors org.apache.lucene.index.PointValues.PointTree.
type PointTree interface {
	// Clone returns a deep-enough copy of this cursor that the original
	// and the clone can be walked independently. They share the
	// underlying immutable index buffers but each carries its own
	// position stack.
	Clone() PointTree

	// MoveToChild attempts to descend to the left child of the current
	// node. Returns false when the cursor is on a leaf.
	MoveToChild() (bool, error)

	// MoveToSibling attempts to move to the right sibling of the current
	// node. Returns false when the cursor is on a right child or on the
	// root.
	MoveToSibling() (bool, error)

	// MoveToParent attempts to move to the parent of the current node.
	// Returns false when the cursor is on the (cloned-from) root.
	MoveToParent() (bool, error)

	// GetMinPackedValue returns the per-dim minimum packed value of
	// the current subtree. The returned slice is owned by the cursor;
	// callers must copy if they need to retain it across moves.
	GetMinPackedValue() []byte

	// GetMaxPackedValue returns the per-dim maximum packed value of
	// the current subtree. The returned slice is owned by the cursor;
	// callers must copy if they need to retain it across moves.
	GetMaxPackedValue() []byte

	// Size returns the number of points in the current subtree.
	Size() int64

	// VisitDocIDs emits the docIDs of every point in the current
	// subtree to visitor.Visit. The visitor's per-value hook is not
	// invoked; visitors that need packed values should call
	// VisitDocValues instead.
	VisitDocIDs(visitor IntersectVisitor) error

	// VisitDocValues walks the current subtree leaf-by-leaf and emits
	// each (docID, packedValue) pair to visitor.VisitByPackedValue when
	// the query crosses the cell, or filters the leaf entirely when
	// the query is outside the cell.
	VisitDocValues(visitor IntersectVisitor) error
}

// BKDReader reads a BKD tree previously written by BKDWriter from the
// provided (meta, index, data) IndexInputs. The reader does NOT own
// the inputs; closing the reader does not close them.
//
// A reader is safe for concurrent use only through clones of the
// PointTree returned by GetPointTree(). The reader itself caches
// only immutable metadata after construction.
type BKDReader struct {
	config         BKDConfig
	numLeaves      int
	in             store.IndexInput
	minPackedValue []byte
	maxPackedValue []byte
	pointCount     int64
	docCount       int
	version        int
	minLeafBlockFP int64

	indexStartPointer int64
	numIndexBytes     int
	indexIn           store.IndexInput

	// isTreeBalanced is always false for indices we read here: only
	// pre-VERSION_META_FILE indices can be balanced, and BKDWriter
	// emits version >= 9 exclusively. Kept for parity with the Java
	// reference and to gate the few code paths that check it.
	isTreeBalanced bool
}

// NewBKDReader opens a BKD tree from the supplied meta, index and data
// inputs. The caller must have positioned metaIn at the start of the
// BKD codec header (BKDWriter emits the codec header as the first
// bytes of metaIn); indexIn and dataIn must point at the bases the
// writer's makeWriter returned (i.e. opened from their respective
// files in the directory).
//
// Mirrors the Java BKDReader(IndexInput metaIn, IndexInput indexIn,
// IndexInput dataIn) constructor.
func NewBKDReader(metaIn, indexIn, dataIn store.IndexInput) (*BKDReader, error) {
	if metaIn == nil {
		return nil, errors.New("bkd: metaIn cannot be nil")
	}
	if indexIn == nil {
		return nil, errors.New("bkd: indexIn cannot be nil")
	}
	if dataIn == nil {
		return nil, errors.New("bkd: dataIn cannot be nil")
	}

	version, err := codecs.CheckHeader(metaIn, BKDCodecName, BKDVersionStart, BKDVersionCurrent)
	if err != nil {
		return nil, fmt.Errorf("bkd: meta codec header: %w", err)
	}

	numDims32, err := store.ReadVInt(metaIn)
	if err != nil {
		return nil, fmt.Errorf("bkd: read numDims: %w", err)
	}
	numDims := int(numDims32)

	var numIndexDims int
	if version >= BKDVersionSelectiveIndexing {
		v, err := store.ReadVInt(metaIn)
		if err != nil {
			return nil, fmt.Errorf("bkd: read numIndexDims: %w", err)
		}
		numIndexDims = int(v)
	} else {
		numIndexDims = numDims
	}

	maxPointsInLeafNode32, err := store.ReadVInt(metaIn)
	if err != nil {
		return nil, fmt.Errorf("bkd: read maxPointsInLeafNode: %w", err)
	}
	bytesPerDim32, err := store.ReadVInt(metaIn)
	if err != nil {
		return nil, fmt.Errorf("bkd: read bytesPerDim: %w", err)
	}

	config, err := Of(numDims, numIndexDims, int(bytesPerDim32), int(maxPointsInLeafNode32))
	if err != nil {
		return nil, fmt.Errorf("bkd: invalid config: %w", err)
	}

	numLeaves32, err := store.ReadVInt(metaIn)
	if err != nil {
		return nil, fmt.Errorf("bkd: read numLeaves: %w", err)
	}
	if numLeaves32 <= 0 {
		return nil, fmt.Errorf("bkd: numLeaves must be > 0, got %d", numLeaves32)
	}
	numLeaves := int(numLeaves32)

	pibl := config.PackedIndexBytesLength()
	minPackedValue := make([]byte, pibl)
	if err := metaIn.ReadBytes(minPackedValue); err != nil {
		return nil, fmt.Errorf("bkd: read minPackedValue: %w", err)
	}
	maxPackedValue := make([]byte, pibl)
	if err := metaIn.ReadBytes(maxPackedValue); err != nil {
		return nil, fmt.Errorf("bkd: read maxPackedValue: %w", err)
	}

	comparator := GetUnsignedComparator(config.BytesPerDim())
	for dim := 0; dim < config.NumIndexDims(); dim++ {
		off := dim * config.BytesPerDim()
		if comparator(minPackedValue, off, maxPackedValue, off) > 0 {
			return nil, fmt.Errorf(
				"bkd: corrupt index: minPackedValue %x > maxPackedValue %x at dim=%d",
				minPackedValue, maxPackedValue, dim,
			)
		}
	}
	// Save heap for the single-value edge case (mirrors Java's
	// Arrays.equals shortcut on lines 102-107 of BKDReader.java).
	if bytes.Equal(minPackedValue, maxPackedValue) {
		maxPackedValue = minPackedValue
	}

	pointCount, err := store.ReadVLong(metaIn)
	if err != nil {
		return nil, fmt.Errorf("bkd: read pointCount: %w", err)
	}
	docCount32, err := store.ReadVInt(metaIn)
	if err != nil {
		return nil, fmt.Errorf("bkd: read docCount: %w", err)
	}
	numIndexBytes32, err := store.ReadVInt(metaIn)
	if err != nil {
		return nil, fmt.Errorf("bkd: read numIndexBytes: %w", err)
	}

	var minLeafBlockFP, indexStartPointer int64
	if int(version) >= BKDVersionMetaFile {
		minLeafBlockFP, err = store.ReadInt64(metaIn)
		if err != nil {
			return nil, fmt.Errorf("bkd: read minLeafBlockFP: %w", err)
		}
		indexStartPointer, err = store.ReadInt64(metaIn)
		if err != nil {
			return nil, fmt.Errorf("bkd: read indexStartPointer: %w", err)
		}
	} else {
		// Pre-VERSION_META_FILE layout: the index lives inline with the
		// data stream. We do not emit this format from BKDWriter, but we
		// keep the legacy decode path so externally-produced indices
		// (or future test fixtures) can be opened.
		indexStartPointer = indexIn.GetFilePointer()
		minLeafBlockFP, err = store.ReadVLong(indexIn)
		if err != nil {
			return nil, fmt.Errorf("bkd: read legacy minLeafBlockFP: %w", err)
		}
		if err := indexIn.SetPosition(indexStartPointer); err != nil {
			return nil, fmt.Errorf("bkd: seek legacy indexStart: %w", err)
		}
	}

	r := &BKDReader{
		config:            config,
		numLeaves:         numLeaves,
		in:                dataIn,
		minPackedValue:    minPackedValue,
		maxPackedValue:    maxPackedValue,
		pointCount:        pointCount,
		docCount:          int(docCount32),
		version:           int(version),
		minLeafBlockFP:    minLeafBlockFP,
		indexStartPointer: indexStartPointer,
		numIndexBytes:     int(numIndexBytes32),
		indexIn:           indexIn,
	}
	// Trees emitted by BKDWriter (version >= 9) are always unbalanced.
	// For one leaf, balanced and unbalanced share the same code path.
	r.isTreeBalanced = numLeaves != 1 && r.computeIsTreeBalanced()
	return r, nil
}

// computeIsTreeBalanced is the Go peer of Java's isTreeBalanced().
// Indices emitted by BKDWriter at version >= VERSION_META_FILE are
// always unbalanced (cf. the Java implementation that short-circuits
// to false on that branch). The full pre-8.6 algorithm is not
// re-implemented here because BKDWriter never produces such trees;
// callers reading an externally-produced legacy index would simply
// see a conservative `false` answer, which is safe (it routes through
// the same unbalanced code path).
func (r *BKDReader) computeIsTreeBalanced() bool {
	// Version >= 9 (the only version BKDWriter emits) is never balanced.
	return false
}

// Config returns the BKD configuration recorded in the meta header.
func (r *BKDReader) Config() BKDConfig { return r.config }

// Version returns the on-disk BKD version read from the meta header.
func (r *BKDReader) Version() int { return r.version }

// NumLeaves returns the number of leaf blocks recorded in the meta
// header.
func (r *BKDReader) NumLeaves() int { return r.numLeaves }

// Size returns the total number of points indexed by this BKD tree.
// Mirrors PointValues.size() from Java.
func (r *BKDReader) Size() int64 { return r.pointCount }

// GetDocCount returns the number of distinct documents that contributed
// at least one point. Mirrors PointValues.getDocCount() from Java.
func (r *BKDReader) GetDocCount() int { return r.docCount }

// GetNumDimensions returns the number of stored dimensions.
func (r *BKDReader) GetNumDimensions() int { return r.config.NumDims() }

// GetNumIndexDimensions returns the number of indexed dimensions
// (numIndexDims; may be less than getNumDimensions for selective
// indexing).
func (r *BKDReader) GetNumIndexDimensions() int { return r.config.NumIndexDims() }

// GetBytesPerDimension returns the byte width of each dimension's
// packed value.
func (r *BKDReader) GetBytesPerDimension() int { return r.config.BytesPerDim() }

// GetMinPackedValue returns a fresh copy of the per-dim minimum packed
// values across all points in this tree.
func (r *BKDReader) GetMinPackedValue() []byte {
	out := make([]byte, len(r.minPackedValue))
	copy(out, r.minPackedValue)
	return out
}

// GetMaxPackedValue returns a fresh copy of the per-dim maximum packed
// values across all points in this tree.
func (r *BKDReader) GetMaxPackedValue() []byte {
	out := make([]byte, len(r.maxPackedValue))
	copy(out, r.maxPackedValue)
	return out
}

// GetPointTree returns a PointTree cursor positioned at the root of
// the tree. The cursor reads the packed index from a slice of
// indexIn and clones dataIn so that callers can walk the tree
// concurrently with other consumers of the same reader's inputs.
func (r *BKDReader) GetPointTree() (PointTree, error) {
	innerNodes, err := r.indexIn.Slice("packedIndex", r.indexStartPointer, int64(r.numIndexBytes))
	if err != nil {
		return nil, fmt.Errorf("bkd: slice packedIndex: %w", err)
	}
	leafNodes := r.in.Clone()

	tree, err := newBKDPointTree(
		innerNodes, leafNodes,
		r.config, r.numLeaves, r.version, r.pointCount,
		r.minPackedValue, r.maxPackedValue,
		r.isTreeBalanced,
	)
	if err != nil {
		return nil, err
	}
	return tree, nil
}

// Intersect walks the tree and invokes visitor for every doc whose
// packed value is contained in the query cell described by
// visitor.Compare. This is a convenience wrapper around GetPointTree:
// it performs the recursive descent that
// org.apache.lucene.index.PointValues.intersect(IntersectVisitor) wraps
// around its tree.
func (r *BKDReader) Intersect(visitor IntersectVisitor) error {
	tree, err := r.GetPointTree()
	if err != nil {
		return err
	}
	return intersect(tree, visitor)
}

// intersect implements PointValues.intersect's recursive descent on
// top of a PointTree cursor.
func intersect(tree PointTree, visitor IntersectVisitor) error {
	rel := visitor.Compare(tree.GetMinPackedValue(), tree.GetMaxPackedValue())
	switch rel {
	case codecs.RelationCellOutsideQuery:
		return nil
	case codecs.RelationCellInsideQuery:
		return tree.VisitDocIDs(visitor)
	case codecs.RelationCellCrossesQuery:
		// If we are on a leaf, scan all docs in this leaf with their
		// packed values; otherwise recurse left + right.
		// We do not have direct "isLeaf" on PointTree (Lucene exposes
		// it via size() vs. moveToChild), so we attempt to descend.
		moved, err := tree.MoveToChild()
		if err != nil {
			return err
		}
		if !moved {
			return tree.VisitDocValues(visitor)
		}
		if err := intersect(tree, visitor); err != nil {
			return err
		}
		moved, err = tree.MoveToSibling()
		if err != nil {
			return err
		}
		if moved {
			if err := intersect(tree, visitor); err != nil {
				return err
			}
		}
		if _, err := tree.MoveToParent(); err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("bkd: unexpected relation %v", rel)
	}
}

// EstimatePointCount estimates the number of points that match a query
// by walking the tree once and summing sizes for fully-inside subtrees,
// plus a sample of crossing leaves. Mirrors PointValues.estimatePointCount
// but without the full caching the Java reference performs.
//
// The estimate is rough: crossing leaves contribute their full size,
// which over-counts. This is consistent with the Java reference's
// behaviour for unbalanced trees.
func (r *BKDReader) EstimatePointCount(visitor IntersectVisitor) (int64, error) {
	tree, err := r.GetPointTree()
	if err != nil {
		return 0, err
	}
	return estimatePointCount(tree, visitor)
}

func estimatePointCount(tree PointTree, visitor IntersectVisitor) (int64, error) {
	rel := visitor.Compare(tree.GetMinPackedValue(), tree.GetMaxPackedValue())
	switch rel {
	case codecs.RelationCellOutsideQuery:
		return 0, nil
	case codecs.RelationCellInsideQuery:
		return tree.Size(), nil
	case codecs.RelationCellCrossesQuery:
		moved, err := tree.MoveToChild()
		if err != nil {
			return 0, err
		}
		if !moved {
			// Leaf crossing the query: contribute the whole leaf.
			return tree.Size(), nil
		}
		var total int64
		left, err := estimatePointCount(tree, visitor)
		if err != nil {
			return 0, err
		}
		total += left
		moved, err = tree.MoveToSibling()
		if err != nil {
			return 0, err
		}
		if moved {
			right, err := estimatePointCount(tree, visitor)
			if err != nil {
				return 0, err
			}
			total += right
		}
		if _, err := tree.MoveToParent(); err != nil {
			return 0, err
		}
		return total, nil
	default:
		return 0, fmt.Errorf("bkd: unexpected relation %v", rel)
	}
}
