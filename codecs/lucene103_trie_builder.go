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
	"bytes"
	"errors"
	"fmt"
	"math/bits"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Reference: lucene/core/src/java/org/apache/lucene/codecs/lucene103/blocktree/
// TrieBuilder.java (Apache Lucene 10.4.0).
//
// This file ports both the TrieBuilder (used by Lucene103BlockTreeTermsWriter
// to assemble the per-field prefix trie that is persisted to the .tip file)
// and the ChildSaveStrategy enum (which decides how a multi-child node packs
// its children labels on disk).

// On-disk sign codes for trie nodes. These two bits live in the low part of
// the node's first header byte and dispatch to one of four serialisation
// shapes. The numeric values are part of the wire format and must not be
// renumbered.
const (
	trieSignNoChildren                  = 0x00
	trieSignSingleChildWithOutput       = 0x01
	trieSignSingleChildNoOutput         = 0x02
	trieSignMultiChildren               = 0x03
	trieLeafNodeHasTerms                = 1 << 5
	trieLeafNodeHasFloor                = 1 << 6
	trieNonLeafNodeHasTerms       int64 = 1 << 1
	trieNonLeafNodeHasFloor       int64 = 1 << 0
)

// TrieOutput describes the term block a trie leaf points to.
// Mirrors TrieBuilder.Output (a Java record) field-for-field.
type TrieOutput struct {
	// FP is the file pointer to the on-disk terms block this node refers
	// to, relative to the start of the .tim file.
	FP int64
	// HasTerms is false when the on-disk block consists entirely of
	// pointers to child blocks (no actual terms live in it).
	HasTerms bool
	// FloorData is non-nil when a large block of terms sharing a single
	// trie prefix was split into multiple on-disk floor blocks. It
	// encodes the first-byte labels and relative file pointers of every
	// floor sub-block.
	FloorData *util.BytesRef
}

// NewTrieOutput is the Go equivalent of {@code new TrieBuilder.Output(fp,
// hasTerms, floorData)}. Callers must treat the returned pointer as
// immutable: TrieBuilder hangs it off interior nodes.
func NewTrieOutput(fp int64, hasTerms bool, floorData *util.BytesRef) *TrieOutput {
	return &TrieOutput{FP: fp, HasTerms: hasTerms, FloorData: floorData}
}

// trieBuilderStatus tracks the lifecycle of a TrieBuilder. Operations on a
// trie that is not in the BUILDING state panic, matching the Java
// IllegalStateException semantics.
type trieBuilderStatus uint8

const (
	trieStatusBuilding trieBuilderStatus = iota
	trieStatusSaved
	trieStatusDestroyed
)

func (s trieBuilderStatus) String() string {
	switch s {
	case trieStatusBuilding:
		return "BUILDING"
	case trieStatusSaved:
		return "SAVED"
	case trieStatusDestroyed:
		return "DESTROYED"
	default:
		return fmt.Sprintf("trieBuilderStatus(%d)", uint8(s))
	}
}

// trieNode is the in-memory representation of a single trie node during
// construction. Mirrors TrieBuilder.Node in Java.
type trieNode struct {
	// label is the UTF-8 byte (0..255) on the edge leading to this node.
	// The root node uses 0.
	label int
	// output is the term-block pointer when this node terminates a key,
	// or nil when this node is purely structural.
	output *TrieOutput
	// childrenNum is the number of children directly attached to this
	// node. Stored explicitly to avoid linked-list walks during merge.
	childrenNum int
	// next is the next sibling in the parent's children list. Children
	// are kept in strictly increasing label order.
	next *trieNode
	// firstChild / lastChild are the head and tail of the children list.
	firstChild *trieNode
	lastChild  *trieNode
	// fp is the file pointer where this node was serialised, relative to
	// the start of the index slice. -1 means the node has not been
	// written yet.
	fp int64
	// savedTo records the most recent child that has been pushed to the
	// save stack. Used by the post-order DFS in saveNodes.
	savedTo *trieNode
}

func newTrieNode(label int, output *TrieOutput) *trieNode {
	return &trieNode{label: label, output: output, fp: -1}
}

// TrieBuilder assembles the prefix trie that indexes a single field's term
// blocks. Build with [BytesRefToTrie] (one builder per term), merge with
// [TrieBuilder.Append], and persist with [TrieBuilder.Save]. After Save the
// builder is sealed and any further use panics.
//
// Mirrors org.apache.lucene.codecs.lucene103.blocktree.TrieBuilder.
type TrieBuilder struct {
	root   *trieNode
	minKey *util.BytesRef
	maxKey *util.BytesRef
	status trieBuilderStatus
}

// BytesRefToTrie creates a TrieBuilder containing a single (key, output)
// pair. Mirrors TrieBuilder.bytesRefToTrie(BytesRef, Output) in Java.
//
// Empty keys attach the output to the root node; non-empty keys spin out a
// linear chain of label-only interior nodes plus a leaf carrying the output.
func BytesRefToTrie(k *util.BytesRef, v *TrieOutput) *TrieBuilder {
	deep := util.BytesRefDeepCopyOf(k)
	b := &TrieBuilder{
		root:   newTrieNode(0, nil),
		minKey: deep,
		maxKey: deep,
		status: trieStatusBuilding,
	}
	if k.Length == 0 {
		b.root.output = v
		return b
	}
	parent := b.root
	for i := 0; i < k.Length; i++ {
		lbl := int(k.Bytes[k.Offset+i]) & 0xFF
		var out *TrieOutput
		if i == k.Length-1 {
			out = v
		}
		n := newTrieNode(lbl, out)
		parent.firstChild = n
		parent.lastChild = n
		parent.childrenNum = 1
		parent = n
	}
	return b
}

// Append merges all (key, output) pairs from other into the receiver. The
// caller must ensure other.minKey is strictly greater than the receiver's
// current maxKey, mirroring the Java assertion. other is left in the
// DESTROYED state once Append returns.
//
// Mirrors TrieBuilder.append(TrieBuilder) in Java.
func (b *TrieBuilder) Append(other *TrieBuilder) error {
	if b.status != trieStatusBuilding || other.status != trieStatusBuilding {
		return fmt.Errorf("tries have wrong status, got this: %s, append: %s", b.status, other.status)
	}
	if b.maxKey.BytesRefCompareTo(other.minKey) >= 0 {
		return fmt.Errorf("TrieBuilder.Append: incoming minKey must be strictly greater than current maxKey")
	}

	// Walk the existing maxKey path and the incoming minKey path in
	// lockstep until they diverge; while their leading labels still match,
	// pull the new node's *other* children into the existing tree.
	mismatch := bytesMismatch(
		b.maxKey.Bytes[b.maxKey.Offset:b.maxKey.Offset+b.maxKey.Length],
		other.minKey.Bytes[other.minKey.Offset:other.minKey.Offset+other.minKey.Length],
	)
	if mismatch < 0 {
		// One is a prefix of the other. Both keys come from BytesRefs of
		// distinct length here, but Lucene's Arrays.mismatch returns the
		// shorter length when one is a prefix; replicate that semantics.
		mismatch = minInt(b.maxKey.Length, other.minKey.Length)
	}
	a := b.root
	c := other.root

	for i := 0; i < mismatch; i++ {
		aLast := a.lastChild
		cFirst := c.firstChild
		if aLast == nil || cFirst == nil || aLast.label != cFirst.label {
			return errors.New("TrieBuilder.Append: invariant violation walking matching prefix")
		}

		if c.childrenNum > 1 {
			aLast.next = cFirst.next
			a.childrenNum += c.childrenNum - 1
			a.lastChild = c.lastChild
		}

		a = aLast
		c = cFirst
	}

	if c.childrenNum == 0 {
		return errors.New("TrieBuilder.Append: divergence node in incoming trie unexpectedly has no children")
	}
	if a.childrenNum == 0 {
		a.firstChild = c.firstChild
		a.lastChild = c.lastChild
		a.childrenNum = c.childrenNum
	} else {
		if a.lastChild.label >= c.firstChild.label {
			return fmt.Errorf("TrieBuilder.Append: tail label %d not strictly less than incoming head label %d",
				a.lastChild.label, c.firstChild.label)
		}
		a.lastChild.next = c.firstChild
		a.lastChild = c.lastChild
		a.childrenNum += c.childrenNum
	}

	b.maxKey = other.maxKey
	other.status = trieStatusDestroyed
	return nil
}

// GetEmptyOutput returns the output stored on the root node, which a writer
// stamps when the field contains the empty term. Mirrors
// TrieBuilder.getEmptyOutput() in Java.
func (b *TrieBuilder) GetEmptyOutput() *TrieOutput {
	return b.root.output
}

// Visit walks the trie in lexicographic key order and invokes consumer for
// every key/output pair. Intended for tests only — the Java reference
// declares the recursive implementation explicitly off-limits for hot paths.
//
// Mirrors TrieBuilder.visit(BiConsumer<BytesRef, Output>) in Java.
func (b *TrieBuilder) Visit(consumer func(key *util.BytesRef, output *TrieOutput)) {
	if b.status != trieStatusBuilding {
		return
	}
	if b.root.output != nil {
		consumer(util.NewBytesRefEmpty(), b.root.output)
	}
	scratch := util.NewBytesRefBuilder()
	b.visit(b.root.firstChild, scratch, consumer)
}

func (b *TrieBuilder) visit(n *trieNode, key *util.BytesRefBuilder, consumer func(key *util.BytesRef, output *TrieOutput)) {
	for n != nil {
		key.AppendByte(byte(n.label))
		if n.output != nil {
			consumer(key.ToBytesRef(), n.output)
		}
		b.visit(n.firstChild, key, consumer)
		key.SetLength(key.Length() - 1)
		n = n.next
	}
}

// Save serialises the trie into index and records the (indexStart, rootFP,
// indexEnd) triple inside meta. The trie transitions to the SAVED state and
// any further mutation panics.
//
// Mirrors TrieBuilder.save(DataOutput meta, IndexOutput index) in Java.
//
// Wire format (per node, written in post-order):
//   - SIGN_NO_CHILDREN:        [header byte] [n bytes output fp] [floor data]
//   - SIGN_SINGLE_CHILD_*:     [header byte] [label] [n bytes child delta]
//     [m bytes encoded output fp]? [floor data]?
//   - SIGN_MULTI_CHILDREN:     [3 bytes header] [m bytes encoded output fp]?
//     [1 byte (childrenNum-1)]? [strategy bytes]
//     [n*childrenNum bytes child deltas] [floor data]?
//
// The meta block stores three VLongs: indexStart (file pointer where the
// trie data starts), rootFP (offset of the root node within that slice),
// indexEnd (file pointer just past the trailing 8 over-read pad bytes).
func (b *TrieBuilder) Save(meta store.DataOutput, index store.IndexOutput) error {
	if b.status != trieStatusBuilding {
		return fmt.Errorf("only unsaved trie can be saved, got: %s", b.status)
	}
	if err := store.WriteVLong(meta, index.GetFilePointer()); err != nil {
		return err
	}
	if err := b.saveNodes(index); err != nil {
		return err
	}
	if err := store.WriteVLong(meta, b.root.fp); err != nil {
		return err
	}
	// 8 extra bytes so the read side can over-read a long without risking
	// an EOF when the last node is shorter than 8 bytes.
	if err := index.WriteLong(0); err != nil {
		return err
	}
	if err := store.WriteVLong(meta, index.GetFilePointer()); err != nil {
		return err
	}
	b.status = trieStatusSaved
	return nil
}

// saveNodes performs a post-order DFS over the trie, emitting each node
// after all its children. The DFS is iterative to avoid blowing the Go
// stack on adversarial inputs.
func (b *TrieBuilder) saveNodes(index store.IndexOutput) error {
	startFP := index.GetFilePointer()
	stack := make([]*trieNode, 0, 32)
	stack = append(stack, b.root)

	for len(stack) > 0 {
		n := stack[len(stack)-1]
		if n.fp != -1 {
			return errors.New("TrieBuilder.saveNodes: node visited twice")
		}

		childrenNum := n.childrenNum
		if childrenNum == 0 {
			// Leaf node.
			if n.output == nil {
				return errors.New("TrieBuilder.saveNodes: leaf node has no output")
			}
			n.fp = index.GetFilePointer() - startFP
			stack = stack[:len(stack)-1]

			out := n.output
			outBytes := bytesRequiredVLong(out.FP)
			header := trieSignNoChildren | ((outBytes - 1) << 2)
			if out.HasTerms {
				header |= trieLeafNodeHasTerms
			}
			if out.FloorData != nil {
				header |= trieLeafNodeHasFloor
			}
			if err := index.WriteByte(byte(header)); err != nil {
				return err
			}
			if err := writeLongNBytes(out.FP, outBytes, index); err != nil {
				return err
			}
			if out.FloorData != nil {
				if err := index.WriteBytesN(
					out.FloorData.Bytes[out.FloorData.Offset:out.FloorData.Offset+out.FloorData.Length],
					out.FloorData.Length,
				); err != nil {
					return err
				}
			}
			continue
		}

		// Drive the post-order traversal: push the first not-yet-saved
		// child, advance the savedTo cursor across siblings, and only
		// write the parent after every child has flushed.
		if n.savedTo == nil {
			n.savedTo = n.firstChild
			stack = append(stack, n.savedTo)
			continue
		}
		if n.savedTo.next != nil {
			n.savedTo = n.savedTo.next
			stack = append(stack, n.savedTo)
			continue
		}

		// All children written; emit this internal node.
		n.fp = index.GetFilePointer() - startFP
		stack = stack[:len(stack)-1]

		if childrenNum == 1 {
			childDeltaFP := n.fp - n.firstChild.fp
			if childDeltaFP <= 0 {
				return fmt.Errorf("TrieBuilder.saveNodes: parent (fp=%d) must be written after child (fp=%d)", n.fp, n.firstChild.fp)
			}
			childFPBytes := bytesRequiredVLong(childDeltaFP)
			var encodedOutputFPBytes int
			if n.output == nil {
				encodedOutputFPBytes = 0
			} else {
				encodedOutputFPBytes = bytesRequiredVLong(n.output.FP << 2)
			}

			sign := trieSignSingleChildNoOutput
			if n.output != nil {
				sign = trieSignSingleChildWithOutput
			}
			header := sign | ((childFPBytes - 1) << 2) | ((encodedOutputFPBytes - 1) << 5)
			if err := index.WriteByte(byte(header)); err != nil {
				return err
			}
			if err := index.WriteByte(byte(n.firstChild.label)); err != nil {
				return err
			}
			if err := writeLongNBytes(childDeltaFP, childFPBytes, index); err != nil {
				return err
			}
			if n.output != nil {
				out := n.output
				if err := writeLongNBytes(encodeOutputFP(out), encodedOutputFPBytes, index); err != nil {
					return err
				}
				if out.FloorData != nil {
					if err := index.WriteBytesN(
						out.FloorData.Bytes[out.FloorData.Offset:out.FloorData.Offset+out.FloorData.Length],
						out.FloorData.Length,
					); err != nil {
						return err
					}
				}
			}
			continue
		}

		// childrenNum > 1
		minLabel := n.firstChild.label
		maxLabel := n.lastChild.label
		if maxLabel <= minLabel {
			return fmt.Errorf("TrieBuilder.saveNodes: maxLabel %d <= minLabel %d on multi-child node", maxLabel, minLabel)
		}
		strategy := chooseChildSaveStrategy(minLabel, maxLabel, childrenNum)
		strategyBytes := strategy.needBytes(minLabel, maxLabel, childrenNum)
		if strategyBytes <= 0 || strategyBytes > 32 {
			return fmt.Errorf("TrieBuilder.saveNodes: strategy bytes %d outside [1, 32]", strategyBytes)
		}

		// Children file pointers are in increasing fp order; the first
		// child has the largest delta because it was emitted earliest.
		maxChildDeltaFP := n.fp - n.firstChild.fp
		if maxChildDeltaFP <= 0 {
			return fmt.Errorf("TrieBuilder.saveNodes: parent (fp=%d) must be written after first child (fp=%d)", n.fp, n.firstChild.fp)
		}

		childrenFPBytes := bytesRequiredVLong(maxChildDeltaFP)
		var encodedOutputFPBytes int
		if n.output == nil {
			encodedOutputFPBytes = 1
		} else {
			encodedOutputFPBytes = bytesRequiredVLong(n.output.FP << 2)
		}

		hasOutputBit := 0
		if n.output != nil {
			hasOutputBit = 1
		}
		header := int64(trieSignMultiChildren) |
			(int64(childrenFPBytes-1) << 2) |
			(int64(hasOutputBit) << 5) |
			(int64(encodedOutputFPBytes-1) << 6) |
			(int64(strategy.code) << 9) |
			(int64(strategyBytes-1) << 11) |
			(int64(minLabel) << 16)
		if err := writeLongNBytes(header, 3, index); err != nil {
			return err
		}

		if n.output != nil {
			out := n.output
			if err := writeLongNBytes(encodeOutputFP(out), encodedOutputFPBytes, index); err != nil {
				return err
			}
			if out.FloorData != nil {
				if err := index.WriteByte(byte(childrenNum - 1)); err != nil {
					return err
				}
			}
		}

		strategyStartFP := index.GetFilePointer()
		if err := strategy.save(n, childrenNum, strategyBytes, index); err != nil {
			return err
		}
		if got := index.GetFilePointer() - strategyStartFP; got != int64(strategyBytes) {
			return fmt.Errorf("TrieBuilder.saveNodes: strategy %s wrote %d bytes, expected %d",
				strategy.name, got, strategyBytes)
		}

		for child := n.firstChild; child != nil; child = child.next {
			if n.fp <= child.fp {
				return fmt.Errorf("TrieBuilder.saveNodes: parent fp %d must exceed child fp %d", n.fp, child.fp)
			}
			if err := writeLongNBytes(n.fp-child.fp, childrenFPBytes, index); err != nil {
				return err
			}
		}

		if n.output != nil && n.output.FloorData != nil {
			fd := n.output.FloorData
			if err := index.WriteBytesN(fd.Bytes[fd.Offset:fd.Offset+fd.Length], fd.Length); err != nil {
				return err
			}
		}
	}
	return nil
}

// encodeOutputFP packs the (floorPresent, hasTerms, fp) triple into a single
// long suitable for the multi-child / single-with-output encoding paths.
// Mirrors TrieBuilder.encodeFP(Output) in Java.
func encodeOutputFP(out *TrieOutput) int64 {
	if out.FP >= (int64(1) << 62) {
		panic(fmt.Sprintf("TrieBuilder.encodeOutputFP: file pointer %d does not fit in 62 bits", out.FP))
	}
	v := out.FP << 2
	if out.HasTerms {
		v |= trieNonLeafNodeHasTerms
	}
	if out.FloorData != nil {
		v |= trieNonLeafNodeHasFloor
	}
	return v
}

// bytesRequiredVLong returns the number of bytes required to encode v in the
// trie's tightly packed little-endian form (always at least 1, at most 8).
// Mirrors TrieBuilder.bytesRequiredVLong(long) in Java.
func bytesRequiredVLong(v int64) int {
	// |1 ensures we still return 1 byte when v == 0.
	leading := bits.LeadingZeros64(uint64(v) | 1)
	return 8 - leading>>3
}

// writeLongNBytes emits the lowest n bytes of v in little-endian order. Used
// pervasively in the trie's hand-packed integers. Mirrors
// TrieBuilder.writeLongNBytes(long, int, DataOutput) in Java.
func writeLongNBytes(v int64, n int, out store.DataOutput) error {
	u := uint64(v)
	for i := 0; i < n; i++ {
		if err := out.WriteByte(byte(u)); err != nil {
			return err
		}
		u >>= 8
	}
	if u != 0 {
		return fmt.Errorf("writeLongNBytes: %d bytes are insufficient to encode value %d", n, v)
	}
	return nil
}

// bytesMismatch returns the index of the first differing byte between a and
// b, or -1 if a and b are identical *and* of equal length. When one is a
// prefix of the other Java's Arrays.mismatch returns the common length; we
// return -1 here so callers can short-circuit, matching the equal-length
// fast path.
func bytesMismatch(a, b []byte) int {
	n := minInt(len(a), len(b))
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	if len(a) == len(b) {
		return -1
	}
	return n
}

// trieAssertNoCommonPrefix is an internal sanity check used by Append in
// debug builds; left non-public so the production code path is allocation
// free.
//
//lint:ignore U1000 retained for future debug builds.
func trieAssertNoCommonPrefix(a, b []byte) bool {
	return !bytes.HasPrefix(a, b) && !bytes.HasPrefix(b, a)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ChildSaveStrategy enumerates the three serialisation shapes a multi-child
// trie node can pick. The strategy is chosen per-node at save time to
// minimise the on-disk footprint. Mirrors
// TrieBuilder.ChildSaveStrategy in Java.
type ChildSaveStrategy struct {
	// name is the strategy name as exposed by Lucene's enum.name().
	name string
	// code is the on-disk dispatch code; it is part of the wire format
	// and must not be changed.
	code int
	// needBytes returns the number of bytes the strategy would consume
	// for the given child-label distribution.
	needBytes func(minLabel, maxLabel, labelCount int) int
	// save serialises the strategy-specific payload into index, using
	// parent as the source of children labels.
	save func(parent *trieNode, labelCount, strategyBytes int, index store.IndexOutput) error
	// lookup probes the on-disk payload for targetLabel and returns its
	// 0-based position among children, or -1 when absent. Used by the
	// read side (TrieReader).
	lookup func(targetLabel int, in store.RandomAccessInput, offset int64, strategyBytes, minLabel int) (int, error)
}

// On-disk strategy codes. The read side dispatches on these.
const (
	trieStrategyCodeReverseArray = 0
	trieStrategyCodeArray        = 1
	trieStrategyCodeBits         = 2
)

// strategyBits packs children labels into a bitset starting at the first
// child's label. Highest priority because lookup is a single popcount.
var strategyBits = ChildSaveStrategy{
	name: "BITS",
	code: trieStrategyCodeBits,
	needBytes: func(minLabel, maxLabel, _ int) int {
		distance := maxLabel - minLabel + 1
		return (distance + 7) >> 3
	},
	save: func(parent *trieNode, _, _ int, index store.IndexOutput) error {
		var presence byte = 1 // first child is always present
		presenceIndex := 0
		previousLabel := parent.firstChild.label
		for child := parent.firstChild.next; child != nil; child = child.next {
			label := child.label
			if label <= previousLabel {
				return fmt.Errorf("BITS.save: children labels must be strictly increasing, got %d after %d", label, previousLabel)
			}
			presenceIndex += label - previousLabel
			for presenceIndex >= 8 {
				if err := index.WriteByte(presence); err != nil {
					return err
				}
				presence = 0
				presenceIndex -= 8
			}
			presence |= 1 << presenceIndex
			previousLabel = label
		}
		return index.WriteByte(presence)
	},
	lookup: func(targetLabel int, in store.RandomAccessInput, offset int64, strategyBytes, minLabel int) (int, error) {
		bitIndex := targetLabel - minLabel
		if bitIndex >= strategyBytes<<3 {
			return -1, nil
		}
		wordIndex := bitIndex >> 6
		wordFP := offset + int64(wordIndex)<<3
		word, err := in.ReadLongAt(wordFP)
		if err != nil {
			return -1, err
		}
		mask := int64(1) << uint(bitIndex)
		if word&mask == 0 {
			return -1, nil
		}
		pos := 0
		for fp := offset; fp < wordFP; fp += 8 {
			w, err := in.ReadLongAt(fp)
			if err != nil {
				return -1, err
			}
			pos += bits.OnesCount64(uint64(w))
		}
		pos += bits.OnesCount64(uint64(word & (mask - 1)))
		return pos, nil
	},
}

// strategyArray stores every label after the first in an unsorted-but-
// strictly-increasing array; lookup is a binary search.
var strategyArray = ChildSaveStrategy{
	name: "ARRAY",
	code: trieStrategyCodeArray,
	needBytes: func(_, _, labelCount int) int {
		// First label is implicit (kept in the node header), so the
		// strategy block stores N-1 bytes.
		return labelCount - 1
	},
	save: func(parent *trieNode, _, _ int, index store.IndexOutput) error {
		for child := parent.firstChild.next; child != nil; child = child.next {
			if err := index.WriteByte(byte(child.label)); err != nil {
				return err
			}
		}
		return nil
	},
	lookup: func(targetLabel int, in store.RandomAccessInput, offset int64, strategyBytes, minLabel int) (int, error) {
		_ = minLabel
		low, high := 0, strategyBytes-1
		for low <= high {
			mid := (low + high) >> 1
			b, err := in.ReadByteAt(offset + int64(mid))
			if err != nil {
				return -1, err
			}
			lbl := int(b) & 0xFF
			switch {
			case lbl < targetLabel:
				low = mid + 1
			case lbl > targetLabel:
				high = mid - 1
			default:
				// min label not included, plus 1
				return mid + 1, nil
			}
		}
		return -1, nil
	},
}

// strategyReverseArray stores the labels that are *absent* inside the
// [minLabel, maxLabel] range, prefixed by maxLabel. Useful for very dense
// ranges where most labels are present.
var strategyReverseArray = ChildSaveStrategy{
	name: "REVERSE_ARRAY",
	code: trieStrategyCodeReverseArray,
	needBytes: func(minLabel, maxLabel, labelCount int) int {
		distance := maxLabel - minLabel + 1
		return distance - labelCount + 1
	},
	save: func(parent *trieNode, _, _ int, index store.IndexOutput) error {
		if err := index.WriteByte(byte(parent.lastChild.label)); err != nil {
			return err
		}
		lastLabel := parent.firstChild.label
		for child := parent.firstChild.next; child != nil; child = child.next {
			for {
				lastLabel++
				if lastLabel >= child.label {
					break
				}
				if err := index.WriteByte(byte(lastLabel)); err != nil {
					return err
				}
			}
		}
		return nil
	},
	lookup: func(targetLabel int, in store.RandomAccessInput, offset int64, strategyBytes, minLabel int) (int, error) {
		maxLabelByte, err := in.ReadByteAt(offset)
		if err != nil {
			return -1, err
		}
		offset++
		maxLabel := int(maxLabelByte) & 0xFF
		if targetLabel >= maxLabel {
			if targetLabel == maxLabel {
				return maxLabel - minLabel - strategyBytes + 1, nil
			}
			return -1, nil
		}
		if strategyBytes == 1 {
			return targetLabel - minLabel, nil
		}
		low, high := 0, strategyBytes-2
		for low <= high {
			mid := (low + high) >> 1
			b, err := in.ReadByteAt(offset + int64(mid))
			if err != nil {
				return -1, err
			}
			lbl := int(b) & 0xFF
			switch {
			case lbl < targetLabel:
				low = mid + 1
			case lbl > targetLabel:
				high = mid - 1
			default:
				return -1, nil
			}
		}
		return targetLabel - minLabel - low, nil
	},
}

// strategiesByPriority drives chooseChildSaveStrategy: the first strategy
// to tie on byte count wins. The order matches the Java reference (BITS
// first because its lookup is a single popcount).
var strategiesByPriority = [...]*ChildSaveStrategy{&strategyBits, &strategyArray, &strategyReverseArray}

// strategiesByCode is the inverse lookup table used by the read side.
var strategiesByCode = func() [3]*ChildSaveStrategy {
	var arr [3]*ChildSaveStrategy
	for _, s := range strategiesByPriority {
		arr[s.code] = s
	}
	return arr
}()

// ChildSaveStrategyByCode returns the strategy with the given wire-format
// code, or an error for an unknown code. Mirrors
// TrieBuilder.ChildSaveStrategy.byCode(int) in Java.
func ChildSaveStrategyByCode(code int) (*ChildSaveStrategy, error) {
	if code < 0 || code >= len(strategiesByCode) {
		return nil, fmt.Errorf("illegal code for child save strategy: %d", code)
	}
	return strategiesByCode[code], nil
}

// chooseChildSaveStrategy picks the cheapest strategy in priority order.
// Mirrors TrieBuilder.ChildSaveStrategy.choose(int, int, int) in Java.
func chooseChildSaveStrategy(minLabel, maxLabel, labelCount int) *ChildSaveStrategy {
	var best *ChildSaveStrategy
	bestBytes := int(^uint(0) >> 1) // MaxInt
	for _, s := range strategiesByPriority {
		c := s.needBytes(minLabel, maxLabel, labelCount)
		if c < bestBytes {
			best = s
			bestBytes = c
		}
	}
	if best == nil {
		panic("chooseChildSaveStrategy: no strategy selected (empty priority list)")
	}
	return best
}
