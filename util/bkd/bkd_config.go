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
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// BKDConfig holds the basic parameters for indexing points on a BKD
// tree. Port of org.apache.lucene.util.bkd.BKDConfig (Lucene 10.4.0),
// which is a Java record; the Go form is a plain struct with the same
// fields and helpers.
type BKDConfig struct {
	numDims             int
	numIndexDims        int
	bytesPerDim         int
	maxPointsInLeafNode int
}

const (
	// DefaultMaxPointsInLeafNode is the default maximum number of
	// points in each leaf block.
	DefaultMaxPointsInLeafNode = 512

	// MaxDims is the maximum number of stored dimensions
	// (2 * MaxIndexDims).
	MaxDims = 16

	// MaxIndexDims is the maximum number of index dimensions.
	MaxIndexDims = 8

	// integerBytes mirrors Java's Integer.BYTES (4).
	integerBytes = 4
)

// defaultConfigs corresponds to BKDConfig.DEFAULT_CONFIGS in Lucene:
// pre-built instances that Of() interns to keep equal configurations
// pointer-identical when possible.
var defaultConfigs = []BKDConfig{
	// cover the most common types for 1 and 2 dimensions.
	{numDims: 1, numIndexDims: 1, bytesPerDim: 2, maxPointsInLeafNode: DefaultMaxPointsInLeafNode},
	{numDims: 1, numIndexDims: 1, bytesPerDim: 4, maxPointsInLeafNode: DefaultMaxPointsInLeafNode},
	{numDims: 1, numIndexDims: 1, bytesPerDim: 8, maxPointsInLeafNode: DefaultMaxPointsInLeafNode},
	{numDims: 1, numIndexDims: 1, bytesPerDim: 16, maxPointsInLeafNode: DefaultMaxPointsInLeafNode},
	{numDims: 2, numIndexDims: 2, bytesPerDim: 2, maxPointsInLeafNode: DefaultMaxPointsInLeafNode},
	{numDims: 2, numIndexDims: 2, bytesPerDim: 4, maxPointsInLeafNode: DefaultMaxPointsInLeafNode},
	{numDims: 2, numIndexDims: 2, bytesPerDim: 8, maxPointsInLeafNode: DefaultMaxPointsInLeafNode},
	{numDims: 2, numIndexDims: 2, bytesPerDim: 16, maxPointsInLeafNode: DefaultMaxPointsInLeafNode},
	// cover lucene shapes
	{numDims: 7, numIndexDims: 4, bytesPerDim: 4, maxPointsInLeafNode: DefaultMaxPointsInLeafNode},
}

// NewBKDConfig validates the four parameters and returns the
// corresponding configuration. The Java record constructor's
// invariants are enforced as typed errors here.
func NewBKDConfig(numDims, numIndexDims, bytesPerDim, maxPointsInLeafNode int) (BKDConfig, error) {
	if numDims < 1 || numDims > MaxDims {
		return BKDConfig{}, fmt.Errorf("numDims must be 1 .. %d (got: %d)", MaxDims, numDims)
	}
	if numIndexDims < 1 || numIndexDims > MaxIndexDims {
		return BKDConfig{}, fmt.Errorf("numIndexDims must be 1 .. %d (got: %d)", MaxIndexDims, numIndexDims)
	}
	if numIndexDims > numDims {
		return BKDConfig{}, fmt.Errorf("numIndexDims cannot exceed numDims (%d) (got: %d)", numDims, numIndexDims)
	}
	if bytesPerDim <= 0 {
		return BKDConfig{}, fmt.Errorf("bytesPerDim must be > 0; got %d", bytesPerDim)
	}
	if maxPointsInLeafNode <= 0 {
		return BKDConfig{}, fmt.Errorf("maxPointsInLeafNode must be > 0; got %d", maxPointsInLeafNode)
	}
	if maxPointsInLeafNode > util.MaxArrayLength {
		return BKDConfig{}, fmt.Errorf("maxPointsInLeafNode must be <= ArrayUtil.MAX_ARRAY_LENGTH (= %d); got %d",
			util.MaxArrayLength, maxPointsInLeafNode)
	}
	return BKDConfig{
		numDims:             numDims,
		numIndexDims:        numIndexDims,
		bytesPerDim:         bytesPerDim,
		maxPointsInLeafNode: maxPointsInLeafNode,
	}, nil
}

// Of returns a BKDConfig with the requested parameters, reusing a
// shared default-configuration instance when the parameters match
// one of the pre-built defaults. Mirrors BKDConfig.of(...) in Java.
func Of(numDims, numIndexDims, bytesPerDim, maxPointsInLeafNode int) (BKDConfig, error) {
	candidate, err := NewBKDConfig(numDims, numIndexDims, bytesPerDim, maxPointsInLeafNode)
	if err != nil {
		return BKDConfig{}, err
	}
	for _, d := range defaultConfigs {
		if d == candidate {
			return d, nil
		}
	}
	return candidate, nil
}

// NumDims returns how many dimensions are stored at the leaf (data) node.
func (c BKDConfig) NumDims() int { return c.numDims }

// NumIndexDims returns how many dimensions are indexed in the internal nodes.
func (c BKDConfig) NumIndexDims() int { return c.numIndexDims }

// BytesPerDim returns the number of bytes per dimension value.
func (c BKDConfig) BytesPerDim() int { return c.bytesPerDim }

// MaxPointsInLeafNode returns the maximum number of points allowed
// on a leaf block.
func (c BKDConfig) MaxPointsInLeafNode() int { return c.maxPointsInLeafNode }

// PackedBytesLength returns numDims * bytesPerDim, the total number
// of bytes per packed value across all stored dimensions.
func (c BKDConfig) PackedBytesLength() int {
	return c.numDims * c.bytesPerDim
}

// PackedIndexBytesLength returns numIndexDims * bytesPerDim, the
// total number of bytes per packed value across the indexed
// dimensions only.
func (c BKDConfig) PackedIndexBytesLength() int {
	return c.numIndexDims * c.bytesPerDim
}

// BytesPerDoc returns (numDims * bytesPerDim) + Integer.BYTES, the
// total number of bytes used per indexed document (packed value plus
// docID).
func (c BKDConfig) BytesPerDoc() int {
	return c.PackedBytesLength() + integerBytes
}
