// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License. You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package codecs

import (
	"errors"
	"fmt"
	"io"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// Reference: lucene/core/src/java/org/apache/lucene/codecs/lucene103/blocktree/
// TrieReader.java (Apache Lucene 10.4.0).
//
// TrieReader is the read-side companion to [TrieBuilder]. It walks the trie
// persisted by TrieBuilder.Save and resolves child labels into file pointers.

// trieReaderNoOutput is the sentinel stored on a trie node when the node
// carries no term-block pointer. Matches TrieReader.NO_OUTPUT in Java.
const trieReaderNoOutput int64 = -1

// trieReaderNoFloorData is the sentinel for "this node has no floor data".
// Matches TrieReader.NO_FLOOR_DATA in Java.
const trieReaderNoFloorData int64 = -1

// trieBytesMinus1Mask is the strict port of TrieReader.BYTES_MINUS_1_MASK.
// Indexed by (bytes-1), it strips everything above the requested byte
// count from a 64-bit raw read.
var trieBytesMinus1Mask = [8]int64{
	0xFF,
	0xFFFF,
	0xFFFFFF,
	0xFFFFFFFF,
	0xFFFFFFFFFF,
	0xFFFFFFFFFFFF,
	0xFFFFFFFFFFFFFF,
	-1, // 0xFFFFFFFFFFFFFFFF as int64
}

// TrieNode mirrors TrieReader.Node from the Java reference. Fields are
// exported so tests and the read-side enumerator (SegmentTermsEnum, future
// backlog work) can inspect them; treat the type as a transient cursor —
// it is reused as the [TrieReader.LookupChild] caller advances down the
// trie.
type TrieNode struct {
	// Single-child storage.
	ChildDeltaFP int64

	// Multi-child storage.
	StrategyFP            int64
	ChildSaveStrategyCode int
	StrategyBytes         int
	ChildrenDeltaFPBytes  int

	// Common fields.
	Sign             int
	FP               int64
	MinChildrenLabel int
	Label            int
	OutputFP         int64
	HasTerms         bool
	FloorDataFP      int64
}

// NewTrieNode returns a TrieNode with the no-output / no-floor sentinels in
// place so callers can pass a fresh node to TrieReader.LookupChild without
// preflight initialisation.
func NewTrieNode() *TrieNode {
	return &TrieNode{OutputFP: trieReaderNoOutput, FloorDataFP: trieReaderNoFloorData}
}

// HasOutput returns true when this node terminates a key and points to a
// terms block. Mirrors TrieReader.Node.hasOutput().
func (n *TrieNode) HasOutput() bool {
	return n.OutputFP != trieReaderNoOutput
}

// IsFloor returns true when this node carries floor-block split data.
// Mirrors TrieReader.Node.isFloor().
func (n *TrieNode) IsFloor() bool {
	return n.FloorDataFP != trieReaderNoFloorData
}

// TrieReader walks the on-disk trie persisted by TrieBuilder.Save. It is
// single-threaded; share the underlying IndexInput between multiple
// TrieReader instances only after Cloning it. Mirrors
// org.apache.lucene.codecs.lucene103.blocktree.TrieReader.
type TrieReader struct {
	access store.RandomAccessInput
	input  store.IndexInput
	root   *TrieNode
}

// NewTrieReader opens a TrieReader on input at the given root file pointer.
// input is expected to either already implement [store.RandomAccessInput] or
// be small enough to slurp into memory; in the latter case a
// [store.ByteArrayRandomAccessInput] copy is made and used for absolute
// reads (mirrors Java's input.randomAccessSlice(0, input.length())).
func NewTrieReader(input store.IndexInput, rootFP int64) (*TrieReader, error) {
	if input == nil {
		return nil, errors.New("NewTrieReader: input must not be nil")
	}
	access, err := obtainRandomAccess(input)
	if err != nil {
		return nil, err
	}
	r := &TrieReader{
		access: access,
		input:  input,
		root:   NewTrieNode(),
	}
	if err := r.load(r.root, rootFP); err != nil {
		return nil, err
	}
	return r, nil
}

// Root returns the trie root node materialised at construction time.
// Mirrors TrieReader.root in Java.
func (r *TrieReader) Root() *TrieNode { return r.root }

// FloorData positions the underlying IndexInput at the node's floor data
// and returns it for sequential reads. The returned IndexInput is the same
// one this TrieReader owns; callers must not advance the file pointer in a
// way that races with subsequent LookupChild calls.
//
// Mirrors TrieReader.Node.floorData(TrieReader).
func (r *TrieReader) FloorData(n *TrieNode) (store.IndexInput, error) {
	if !n.IsFloor() {
		return nil, errors.New("TrieReader.FloorData: node has no floor data")
	}
	if err := r.input.SetPosition(n.FloorDataFP); err != nil {
		return nil, err
	}
	return r.input, nil
}

// LookupChild reads the child of parent whose label matches targetLabel,
// populating child (which the caller supplies to avoid an allocation per
// step). Returns (child, nil) on hit, (nil, nil) when the label is absent,
// and (nil, err) on I/O failure. Mirrors TrieReader.lookupChild.
func (r *TrieReader) LookupChild(targetLabel int, parent, child *TrieNode) (*TrieNode, error) {
	sign := parent.Sign
	if sign == trieSignNoChildren {
		return nil, nil
	}
	if sign != trieSignMultiChildren {
		// Single child path.
		if targetLabel != parent.MinChildrenLabel {
			return nil, nil
		}
		child.Label = targetLabel
		if err := r.load(child, parent.FP-parent.ChildDeltaFP); err != nil {
			return nil, err
		}
		return child, nil
	}

	strategyBytesStartFP := parent.StrategyFP
	minLabel := parent.MinChildrenLabel
	strategyBytes := parent.StrategyBytes

	position := -1
	switch {
	case targetLabel == minLabel:
		position = 0
	case targetLabel > minLabel:
		s, err := ChildSaveStrategyByCode(parent.ChildSaveStrategyCode)
		if err != nil {
			return nil, err
		}
		pos, err := s.lookup(targetLabel, r.access, strategyBytesStartFP, strategyBytes, minLabel)
		if err != nil {
			return nil, err
		}
		position = pos
	}
	if position < 0 {
		return nil, nil
	}

	bytesPerEntry := parent.ChildrenDeltaFPBytes
	pos := strategyBytesStartFP + int64(strategyBytes) + int64(bytesPerEntry)*int64(position)
	raw, err := r.access.ReadLongAt(pos)
	if err != nil {
		return nil, err
	}
	childFP := parent.FP - (raw & trieBytesMinus1Mask[bytesPerEntry-1])
	child.Label = targetLabel
	if err := r.load(child, childFP); err != nil {
		return nil, err
	}
	return child, nil
}

// load materialises the trie node at fp into n. Mirrors TrieReader.load.
func (r *TrieReader) load(n *TrieNode, fp int64) error {
	n.FP = fp
	termFlagsLong, err := r.access.ReadLongAt(fp)
	if err != nil {
		return err
	}
	termFlags := int(termFlagsLong)
	sign := termFlags & 0x03
	n.Sign = sign

	switch sign {
	case trieSignNoChildren:
		return r.loadLeafNode(n, fp, termFlags, termFlagsLong)
	case trieSignMultiChildren:
		return r.loadMultiChildrenNode(n, fp, termFlags, termFlagsLong)
	default:
		return r.loadSingleChildNode(n, fp, sign, termFlags, termFlagsLong)
	}
}

func (r *TrieReader) loadLeafNode(n *TrieNode, fp int64, term int, termLong int64) error {
	fpBytesMinus1 := (term >> 2) & 0x07
	if fpBytesMinus1 <= 6 {
		n.OutputFP = (termLong >> 8) & trieBytesMinus1Mask[fpBytesMinus1]
	} else {
		v, err := r.access.ReadLongAt(fp + 1)
		if err != nil {
			return err
		}
		n.OutputFP = v
	}
	n.HasTerms = term&trieLeafNodeHasTerms != 0
	if term&trieLeafNodeHasFloor != 0 {
		n.FloorDataFP = fp + 2 + int64(fpBytesMinus1)
	} else {
		n.FloorDataFP = trieReaderNoFloorData
	}
	return nil
}

func (r *TrieReader) loadSingleChildNode(n *TrieNode, fp int64, sign, term int, termLong int64) error {
	childDeltaFPBytesMinus1 := (term >> 2) & 0x07
	var l int64
	if childDeltaFPBytesMinus1 <= 5 {
		l = termLong >> 16
	} else {
		v, err := r.access.ReadLongAt(fp + 2)
		if err != nil {
			return err
		}
		l = v
	}
	n.ChildDeltaFP = l & trieBytesMinus1Mask[childDeltaFPBytesMinus1]
	n.MinChildrenLabel = (term >> 8) & 0xFF

	if sign == trieSignSingleChildNoOutput {
		n.OutputFP = trieReaderNoOutput
		n.FloorDataFP = trieReaderNoFloorData
		return nil
	}
	// sign == trieSignSingleChildWithOutput
	encodedOutputFPBytesMinus1 := (term >> 5) & 0x07
	offset := fp + int64(childDeltaFPBytesMinus1) + 3
	encodedRaw, err := r.access.ReadLongAt(offset)
	if err != nil {
		return err
	}
	encodedFP := encodedRaw & trieBytesMinus1Mask[encodedOutputFPBytesMinus1]
	n.OutputFP = encodedFP >> 2
	n.HasTerms = encodedFP&trieNonLeafNodeHasTerms != 0
	if encodedFP&trieNonLeafNodeHasFloor != 0 {
		n.FloorDataFP = offset + int64(encodedOutputFPBytesMinus1) + 1
	} else {
		n.FloorDataFP = trieReaderNoFloorData
	}
	return nil
}

func (r *TrieReader) loadMultiChildrenNode(n *TrieNode, fp int64, term int, termLong int64) error {
	n.ChildrenDeltaFPBytes = ((term >> 2) & 0x07) + 1
	n.ChildSaveStrategyCode = (term >> 9) & 0x03
	n.StrategyBytes = ((term >> 11) & 0x1F) + 1
	n.MinChildrenLabel = (term >> 16) & 0xFF

	hasOutput := term&0x20 != 0
	if hasOutput {
		encodedOutputFPBytesMinus1 := (term >> 6) & 0x07
		var l int64
		if encodedOutputFPBytesMinus1 <= 4 {
			l = termLong >> 24
		} else {
			v, err := r.access.ReadLongAt(fp + 3)
			if err != nil {
				return err
			}
			l = v
		}
		encodedFP := l & trieBytesMinus1Mask[encodedOutputFPBytesMinus1]
		n.OutputFP = encodedFP >> 2
		n.HasTerms = encodedFP&trieNonLeafNodeHasTerms != 0
		if encodedFP&trieNonLeafNodeHasFloor != 0 {
			offset := fp + 4 + int64(encodedOutputFPBytesMinus1)
			cb, err := r.access.ReadByteAt(offset)
			if err != nil {
				return err
			}
			childrenNum := int64(cb)&0xFF + 1
			n.StrategyFP = offset + 1
			n.FloorDataFP = n.StrategyFP + int64(n.StrategyBytes) + childrenNum*int64(n.ChildrenDeltaFPBytes)
		} else {
			n.FloorDataFP = trieReaderNoFloorData
			n.StrategyFP = fp + 4 + int64(encodedOutputFPBytesMinus1)
		}
	} else {
		n.OutputFP = trieReaderNoOutput
		n.StrategyFP = fp + 3
		n.FloorDataFP = trieReaderNoFloorData
	}
	return nil
}

// obtainRandomAccess returns the most direct RandomAccessInput backing the
// supplied IndexInput. If the IndexInput already implements
// RandomAccessInput, it is returned verbatim. Otherwise the entire input is
// copied into memory once and wrapped in a ByteArrayRandomAccessInput.
//
// The Java reference uses input.randomAccessSlice(0, input.length()), which
// every concrete IndexInput in Lucene 10.4.0 supports natively. Gocene's
// IndexInput surface still has some implementations that don't expose
// absolute reads; the fall-back keeps the read side functional at the cost
// of an in-memory copy.
func obtainRandomAccess(input store.IndexInput) (store.RandomAccessInput, error) {
	if ra, ok := input.(store.RandomAccessInput); ok {
		return ra, nil
	}
	length := input.Length()
	if length < 0 {
		return nil, fmt.Errorf("obtainRandomAccess: input length %d invalid", length)
	}
	buf := make([]byte, length)
	if err := input.SetPosition(0); err != nil {
		return nil, fmt.Errorf("obtainRandomAccess: rewind: %w", err)
	}
	if length > 0 {
		if err := input.ReadBytes(buf); err != nil && !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("obtainRandomAccess: read input: %w", err)
		}
	}
	return store.NewByteArrayRandomAccessInput(buf), nil
}
