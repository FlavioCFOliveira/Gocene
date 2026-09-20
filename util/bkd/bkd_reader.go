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

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Port of org.apache.lucene.util.bkd.BKDReader (Lucene 10.5.0).
//
// BKDReader handles reading a block KD-tree previously written with
// BKDWriter. The on-disk format is partitioned across three streams
// (meta, index, data) and BKDReader stitches them together into a
// PointTree cursor that callers walk to answer point queries.
//
// Java's BKDReader returns its tree through PointValues.PointTree and
// dispatches visitor callbacks through PointValues.IntersectVisitor.
// [PointTree] and [IntersectVisitor] are aliases of the canonical Gocene
// renderings of those two nested types, spi.PointTree and
// spi.IntersectVisitor, so the names Java uses here stay available in this
// package.

// IntersectVisitor is an alias of [spi.IntersectVisitor], the Go port of
// org.apache.lucene.index.PointValues.IntersectVisitor. BKDReader names it
// unqualified exactly as the Java reader does.
type IntersectVisitor = spi.IntersectVisitor

// PointTree is an alias of [spi.PointTree], the Go port of
// org.apache.lucene.index.PointValues.PointTree: the cursor returned by
// [BKDReader.GetPointTree]. BKDReader names it unqualified exactly as the
// Java reader does.
type PointTree = spi.PointTree

// BKDReader reads a BKD tree previously written by BKDWriter from the
// provided (meta, index, data) IndexInputs. The reader does NOT own
// the inputs; closing the reader does not close them.
//
// A reader is safe for concurrent use only through clones of the
// PointTree returned by GetPointTree(). The reader itself caches
// only immutable metadata after construction.
type BKDReader struct {
	*spi.BasePointValues
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

	version, err := store.CheckHeader(metaIn, BKDCodecName, BKDVersionStart, BKDVersionCurrent)
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
	if err := metaIn.ReadBytes(minPackedValue, 0, pibl); err != nil {
		return nil, fmt.Errorf("bkd: read minPackedValue: %w", err)
	}
	maxPackedValue := make([]byte, pibl)
	if err := metaIn.ReadBytes(maxPackedValue, 0, pibl); err != nil {
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

	pointCount, err := metaIn.ReadVLong()
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
		minLeafBlockFP, err = indexIn.ReadVLong()
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
	r.BasePointValues = spi.NewBasePointValues(r)
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
func (r *BKDReader) GetNumDimensions() (int, error) { return r.config.NumDims(), nil }

// GetNumIndexDimensions returns the number of indexed dimensions
// (numIndexDims; may be less than getNumDimensions for selective
// indexing).
func (r *BKDReader) GetNumIndexDimensions() (int, error) { return r.config.NumIndexDims(), nil }

// GetBytesPerDimension returns the byte width of each dimension's
// packed value.
func (r *BKDReader) GetBytesPerDimension() (int, error) { return r.config.BytesPerDim(), nil }

// GetMinPackedValue returns a fresh copy of the per-dim minimum packed
// values across all points in this tree. Java's body is
// `return minPackedValue.clone()`.
func (r *BKDReader) GetMinPackedValue() ([]byte, error) {
	out := make([]byte, len(r.minPackedValue))
	copy(out, r.minPackedValue)
	return out, nil
}

// GetMaxPackedValue returns a fresh copy of the per-dim maximum packed
// values across all points in this tree. Java's body is
// `return maxPackedValue.clone()`.
func (r *BKDReader) GetMaxPackedValue() ([]byte, error) {
	out := make([]byte, len(r.maxPackedValue))
	copy(out, r.maxPackedValue)
	return out, nil
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

// Java's BKDReader does not override PointValues.intersect or
// PointValues.estimatePointCount: both are `public final` on PointValues, so
// the reader inherits them. The embedded [spi.BasePointValues] supplies those
// bodies, replacing two BKD-local re-implementations that diverged from Java
// (no upperBound early termination, and a crossing leaf contributed its whole
// size instead of Java's (size + 1) / 2).
