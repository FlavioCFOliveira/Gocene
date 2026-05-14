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

package util

import (
	"errors"
	"fmt"
)

// RoaringDocIdSet is a DocIdSet inspired by http://roaringbitmap.org/.
// The space is split into blocks of 2^16 bits and each block is encoded
// independently:
//
//   - if less than 2^12 bits are set, doc ids are stored in a uint16 slice
//     (sparse encoding);
//   - if more than 2^16 - 2^12 bits are set, the inverse of the set is
//     stored in a uint16 slice and wrapped by [NotDocIdSet]
//     (super-dense encoding);
//   - otherwise the block is stored as a [FixedBitSet] wrapped by
//     [BitDocIdSet].
//
// Port of org.apache.lucene.util.RoaringDocIdSet.
type RoaringDocIdSet struct {
	docIdSets   []DocIdSet
	cardinality int
}

const (
	roaringBlockSize      = 1 << 16
	roaringMaxArrayLength = 1 << 12
)

// Cardinality returns the exact number of documents in the set.
func (r *RoaringDocIdSet) Cardinality() int { return r.cardinality }

// String returns a human-readable description.
func (r *RoaringDocIdSet) String() string {
	return fmt.Sprintf("RoaringDocIdSet(cardinality=%d)", r.cardinality)
}

// Iterator returns a DocIdSetIterator over the set, or nil when the
// set is empty (matching the Java semantics where the iterator is
// nullable for empty sets).
func (r *RoaringDocIdSet) Iterator() DocIdSetIterator {
	if r.cardinality == 0 {
		return nil
	}
	return &roaringIterator{
		owner: r,
		block: -1,
		sub:   EmptyDocIdSetIterator(),
		doc:   -1,
	}
}

// RoaringDocIdSetBuilder builds a RoaringDocIdSet. Doc ids must be
// added in strictly increasing order.
type RoaringDocIdSetBuilder struct {
	maxDoc                  int
	sets                    []DocIdSet
	cardinality             int
	lastDocID               int
	currentBlock            int
	currentBlockCardinality int
	buffer                  []uint16
	denseBuffer             *FixedBitSet
}

// NewRoaringDocIdSetBuilder returns a builder targeting a doc-id range
// of [0, maxDoc).
func NewRoaringDocIdSetBuilder(maxDoc int) *RoaringDocIdSetBuilder {
	nBlocks := (maxDoc + roaringBlockSize - 1) >> 16
	return &RoaringDocIdSetBuilder{
		maxDoc:       maxDoc,
		sets:         make([]DocIdSet, nBlocks),
		lastDocID:    -1,
		currentBlock: -1,
		buffer:       make([]uint16, roaringMaxArrayLength),
	}
}

// Add records docID. Returns an error if doc ids are not in strictly
// increasing order; mirrors Lucene's IllegalArgumentException.
func (b *RoaringDocIdSetBuilder) Add(docID int) error {
	if docID <= b.lastDocID {
		return fmt.Errorf("doc ids must be added in-order, got %d which is <= lastDocID=%d", docID, b.lastDocID)
	}
	block := docID >> 16
	if block != b.currentBlock {
		b.flush()
		b.currentBlock = block
	}
	if b.currentBlockCardinality < roaringMaxArrayLength {
		b.buffer[b.currentBlockCardinality] = uint16(docID)
	} else {
		if b.denseBuffer == nil {
			numBits := roaringBlockSize
			remaining := b.maxDoc - (block << 16)
			if remaining < numBits {
				numBits = remaining
			}
			bs, err := NewFixedBitSet(numBits)
			if err != nil {
				return err
			}
			b.denseBuffer = bs
			for _, d := range b.buffer {
				b.denseBuffer.Set(int(d) & 0xFFFF)
			}
		}
		b.denseBuffer.Set(docID & 0xFFFF)
	}
	b.lastDocID = docID
	b.currentBlockCardinality++
	return nil
}

// AddIterator drains the iterator, calling Add for each emitted doc id.
func (b *RoaringDocIdSetBuilder) AddIterator(it DocIdSetIterator) error {
	if it == nil {
		return nil
	}
	for {
		doc, err := it.NextDoc()
		if err != nil {
			return err
		}
		if doc == NO_MORE_DOCS {
			return nil
		}
		if err := b.Add(doc); err != nil {
			return err
		}
	}
}

func (b *RoaringDocIdSetBuilder) flush() {
	if b.currentBlockCardinality <= roaringMaxArrayLength {
		if b.currentBlockCardinality > 0 {
			copyBuf := make([]uint16, b.currentBlockCardinality)
			copy(copyBuf, b.buffer[:b.currentBlockCardinality])
			b.sets[b.currentBlock] = newShortArrayDocIdSet(copyBuf)
		}
	} else {
		if b.denseBuffer.Length() == roaringBlockSize &&
			roaringBlockSize-b.currentBlockCardinality < roaringMaxArrayLength {
			excluded := make([]uint16, roaringBlockSize-b.currentBlockCardinality)
			b.denseBuffer.FlipRange(0, b.denseBuffer.Length())
			pos := -1
			for i := range excluded {
				pos = b.denseBuffer.NextSetBit(pos + 1)
				excluded[i] = uint16(pos)
			}
			b.sets[b.currentBlock] = NewNotDocIdSet(roaringBlockSize, newShortArrayDocIdSet(excluded))
		} else {
			ds, err := NewBitDocIdSet(b.denseBuffer, int64(b.currentBlockCardinality))
			if err != nil {
				// cost is non-negative by construction here; panic
				// surfaces the programming error if the invariant ever
				// breaks.
				panic(err)
			}
			b.sets[b.currentBlock] = ds
		}
	}
	b.cardinality += b.currentBlockCardinality
	b.denseBuffer = nil
	b.currentBlockCardinality = 0
}

// Build returns the immutable RoaringDocIdSet.
func (b *RoaringDocIdSetBuilder) Build() *RoaringDocIdSet {
	b.flush()
	return &RoaringDocIdSet{docIdSets: b.sets, cardinality: b.cardinality}
}

// shortArrayDocIdSet is the sparse block encoding: doc ids 0..2^16-1
// stored as uint16. Unexported because it only makes sense inside a
// roaring block.
type shortArrayDocIdSet struct {
	docIDs []uint16
}

func newShortArrayDocIdSet(docIDs []uint16) *shortArrayDocIdSet {
	return &shortArrayDocIdSet{docIDs: docIDs}
}

// Iterator returns a DocIdSetIterator over docIDs.
func (s *shortArrayDocIdSet) Iterator() DocIdSetIterator {
	return &shortArrayIterator{set: s, idx: -1, doc: -1}
}

type shortArrayIterator struct {
	set *shortArrayDocIdSet
	idx int
	doc int
}

func (it *shortArrayIterator) docAt(i int) int {
	return int(it.set.docIDs[i]) & 0xFFFF
}

func (it *shortArrayIterator) DocID() int { return it.doc }

func (it *shortArrayIterator) NextDoc() (int, error) {
	it.idx++
	if it.idx >= len(it.set.docIDs) {
		it.doc = NO_MORE_DOCS
		return it.doc, nil
	}
	it.doc = it.docAt(it.idx)
	return it.doc, nil
}

func (it *shortArrayIterator) Advance(target int) (int, error) {
	lo := it.idx + 1
	hi := len(it.set.docIDs) - 1
	for lo <= hi {
		mid := int(uint(lo+hi) >> 1)
		if it.docAt(mid) < target {
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	if lo >= len(it.set.docIDs) {
		it.idx = len(it.set.docIDs)
		it.doc = NO_MORE_DOCS
		return it.doc, nil
	}
	it.idx = lo
	it.doc = it.docAt(it.idx)
	return it.doc, nil
}

func (it *shortArrayIterator) Cost() int64 { return int64(len(it.set.docIDs)) }

func (it *shortArrayIterator) DocIDRunEnd() int { return it.doc + 1 }

// roaringIterator stitches the per-block iterators together.
type roaringIterator struct {
	owner *RoaringDocIdSet
	block int
	sub   DocIdSetIterator
	doc   int
}

func (it *roaringIterator) DocID() int { return it.doc }

func (it *roaringIterator) NextDoc() (int, error) {
	if it.sub == nil {
		it.doc = NO_MORE_DOCS
		return it.doc, nil
	}
	subNext, err := it.sub.NextDoc()
	if err != nil {
		return 0, err
	}
	if subNext == NO_MORE_DOCS {
		return it.firstDocFromNextBlock()
	}
	it.doc = (it.block << 16) | subNext
	return it.doc, nil
}

func (it *roaringIterator) Advance(target int) (int, error) {
	targetBlock := target >> 16
	if targetBlock != it.block {
		it.block = targetBlock
		if it.block >= len(it.owner.docIdSets) {
			it.sub = nil
			it.doc = NO_MORE_DOCS
			return it.doc, nil
		}
		if it.owner.docIdSets[it.block] == nil {
			return it.firstDocFromNextBlock()
		}
		it.sub = it.owner.docIdSets[it.block].Iterator()
	}
	if it.sub == nil {
		it.doc = NO_MORE_DOCS
		return it.doc, nil
	}
	subNext, err := it.sub.Advance(target & 0xFFFF)
	if err != nil {
		return 0, err
	}
	if subNext == NO_MORE_DOCS {
		return it.firstDocFromNextBlock()
	}
	it.doc = (it.block << 16) | subNext
	return it.doc, nil
}

func (it *roaringIterator) firstDocFromNextBlock() (int, error) {
	for {
		it.block++
		if it.block >= len(it.owner.docIdSets) {
			it.sub = nil
			it.doc = NO_MORE_DOCS
			return it.doc, nil
		}
		if it.owner.docIdSets[it.block] != nil {
			it.sub = it.owner.docIdSets[it.block].Iterator()
			subNext, err := it.sub.NextDoc()
			if err != nil {
				return 0, err
			}
			if subNext == NO_MORE_DOCS {
				return 0, errors.New("roaring: non-empty block produced empty iterator")
			}
			it.doc = (it.block << 16) | subNext
			return it.doc, nil
		}
	}
}

func (it *roaringIterator) Cost() int64 { return int64(it.owner.cardinality) }

func (it *roaringIterator) DocIDRunEnd() int { return it.doc + 1 }

var (
	_ DocIdSet         = (*RoaringDocIdSet)(nil)
	_ DocIdSet         = (*shortArrayDocIdSet)(nil)
	_ DocIdSetIterator = (*shortArrayIterator)(nil)
	_ DocIdSetIterator = (*roaringIterator)(nil)
)
