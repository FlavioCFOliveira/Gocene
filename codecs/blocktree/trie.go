// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package blocktree provides trie-based indexing for blocktree postings format.
// This is a Go port of Apache Lucene's lucene103 blocktree implementation.
package blocktree

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ChildSaveStrategy represents the strategy for saving child node labels.
// This is the Go port of TrieBuilder.ChildSaveStrategy.
type ChildSaveStrategy int

const (
	// ChildSaveStrategyBits stores children labels in a bitset.
	// This is the most efficient storage as we can compute position with bitCount.
	ChildSaveStrategyBits ChildSaveStrategy = iota

	// ChildSaveStrategyArray stores labels in an array and lookup with binary search.
	ChildSaveStrategyArray

	// ChildSaveStrategyReverseArray stores labels that don't exist within the range.
	ChildSaveStrategyReverseArray
)

// String returns the string representation of the ChildSaveStrategy.
func (s ChildSaveStrategy) String() string {
	switch s {
	case ChildSaveStrategyBits:
		return "BITS"
	case ChildSaveStrategyArray:
		return "ARRAY"
	case ChildSaveStrategyReverseArray:
		return "REVERSE_ARRAY"
	default:
		return "UNKNOWN"
	}
}

// ChildSaveStrategyChoose selects the best strategy based on byte usage.
// This is the Go port of TrieBuilder.ChildSaveStrategy.choose().
func ChildSaveStrategyChoose(minLabel, maxLabel, labelCnt int) ChildSaveStrategy {
	var bestStrategy ChildSaveStrategy
	minBytes := int(^uint(0) >> 1) // MaxInt

	strategies := []ChildSaveStrategy{ChildSaveStrategyBits, ChildSaveStrategyArray, ChildSaveStrategyReverseArray}
	for _, strategy := range strategies {
		cost := strategyNeedBytes(strategy, minLabel, maxLabel, labelCnt)
		if cost < minBytes {
			bestStrategy = strategy
			minBytes = cost
		}
	}

	return bestStrategy
}

// strategyNeedBytes calculates the number of bytes needed for a strategy.
func strategyNeedBytes(strategy ChildSaveStrategy, minLabel, maxLabel, labelCnt int) int {
	switch strategy {
	case ChildSaveStrategyBits:
		byteDistance := maxLabel - minLabel + 1
		return (byteDistance + 7) / 8
	case ChildSaveStrategyArray:
		return labelCnt - 1 // min label is saved separately
	case ChildSaveStrategyReverseArray:
		byteDistance := maxLabel - minLabel + 1
		return byteDistance - labelCnt + 1
	default:
		return 0
	}
}

// TrieOutput represents the output associated with a trie node.
// This is the Go port of TrieBuilder.Output.
type TrieOutput struct {
	fp        int64
	hasTerms  bool
	floorData *util.BytesRef
}

// NewTrieOutput creates a new TrieOutput.
func NewTrieOutput(fp int64, hasTerms bool, floorData *util.BytesRef) *TrieOutput {
	return &TrieOutput{
		fp:        fp,
		hasTerms:  hasTerms,
		floorData: floorData,
	}
}

// Fp returns the file pointer to the on-disk terms block.
func (o *TrieOutput) Fp() int64 {
	return o.fp
}

// HasTerms returns true if this block has terms (not just pointers to child blocks).
func (o *TrieOutput) HasTerms() bool {
	return o.hasTerms
}

// FloorData returns the floor data for split blocks, or nil if not a floor block.
func (o *TrieOutput) FloorData() *util.BytesRef {
	return o.floorData
}

// TrieBuilder builds a prefix tree (trie) as the index of block tree.
// This is the Go port of TrieBuilder.
type TrieBuilder struct {
	// TODO: Implement the actual trie builder structure
	status    trieStatus
	root      *trieNode
	minKey    *util.BytesRef
	maxKey    *util.BytesRef
	emptyOutput *TrieOutput
}

type trieStatus int

const (
	trieStatusBuilding trieStatus = iota
	trieStatusSaved
	trieStatusDestroyed
)

type trieNode struct {
	label      int
	output     *TrieOutput
	childrenNum int
	next       *trieNode
	firstChild *trieNode
	lastChild  *trieNode
	fp         int64
	savedTo    *trieNode
}

// BytesRefToTrie creates a TrieBuilder from a single key-value pair.
// This is the Go port of TrieBuilder.bytesRefToTrie().
func BytesRefToTrie(key *util.BytesRef, output *TrieOutput) *TrieBuilder {
	builder := &TrieBuilder{
		status:      trieStatusBuilding,
		minKey:      key.Clone(),
		maxKey:      key.Clone(),
		emptyOutput: output,
	}

	// Create root node
	builder.root = &trieNode{label: 0}

	if key.Length == 0 {
		builder.root.output = output
		return builder
	}

	// Build the trie path for the key
	parent := builder.root
	keyBytes := key.ValidBytes()
	for i := 0; i < len(keyBytes); i++ {
		b := int(keyBytes[i] & 0xFF)
		var nodeOutput *TrieOutput
		if i == len(keyBytes)-1 {
			nodeOutput = output
		}
		node := &trieNode{label: b, output: nodeOutput}
		parent.firstChild = node
		parent.lastChild = node
		parent.childrenNum = 1
		parent = node
	}

	return builder
}

// Append appends all (K, V) pairs from the given trie into this one.
// The given trie will be destroyed after appending.
// This is the Go port of TrieBuilder.append().
func (tb *TrieBuilder) Append(other *TrieBuilder) error {
	if tb.status != trieStatusBuilding || other.status != trieStatusBuilding {
		return fmt.Errorf("tries have wrong status: this=%d, other=%d", tb.status, other.status)
	}

	// TODO: Implement the actual append logic
	// For now, just mark the other trie as destroyed
	other.status = trieStatusDestroyed
	return nil
}

// Visit traverses the trie and calls the consumer for each key-value pair.
// This is the Go port of TrieBuilder.visit().
func (tb *TrieBuilder) Visit(consumer func(key *util.BytesRef, output *TrieOutput)) {
	if tb.status != trieStatusBuilding {
		return
	}

	if tb.root.output != nil {
		consumer(util.NewBytesRefEmpty(), tb.root.output)
	}

	tb.visitNode(tb.root.firstChild, util.NewBytesRefEmpty(), consumer)
}

func (tb *TrieBuilder) visitNode(node *trieNode, key *util.BytesRef, consumer func(key *util.BytesRef, output *TrieOutput)) {
	for node != nil {
		// Append the label to the key
		newKey := util.NewBytesRef(append(key.ValidBytes(), byte(node.label)))

		if node.output != nil {
			consumer(newKey, node.output)
		}

		tb.visitNode(node.firstChild, newKey, consumer)
		node = node.next
	}
}

// Save saves the trie to disk.
// This is the Go port of TrieBuilder.save().
func (tb *TrieBuilder) Save(meta store.DataOutput, index store.IndexOutput) error {
	if tb.status != trieStatusBuilding {
		return fmt.Errorf("only unsaved trie can be saved, got status: %d", tb.status)
	}

	// TODO: Implement the actual save logic
	// Write metadata
	startFP := index.GetFilePointer()
	if err := store.WriteVLong(meta, startFP); err != nil {
		return err
	}

	// Write root file pointer
	if err := store.WriteVLong(meta, 0); err != nil {
		return err
	}

	// Write end position
	endFP := index.GetFilePointer()
	if err := store.WriteVLong(meta, endFP); err != nil {
		return err
	}

	tb.status = trieStatusSaved
	return nil
}

// TrieNode represents a node in the trie.
// This is the Go port of TrieReader.Node.
type TrieNode struct {
	// single child
	ChildDeltaFp int64

	// multi children
	StrategyFp          int64
	ChildSaveStrategy   int
	StrategyBytes       int
	ChildrenDeltaFpBytes int

	// common
	Sign            int
	Fp              int64
	MinChildrenLabel int
	Label           int
	OutputFp        int64
	HasTerms        bool
	FloorDataFp     int64
}

// NewTrieNode creates a new empty TrieNode.
func NewTrieNode() *TrieNode {
	return &TrieNode{
		OutputFp:    -1, // NO_OUTPUT
		FloorDataFp: -1, // NO_FLOOR_DATA
	}
}

// HasOutput returns true if this node has an output.
func (n *TrieNode) HasOutput() bool {
	return n.OutputFp != -1
}

// IsFloor returns true if this node has floor data.
func (n *TrieNode) IsFloor() bool {
	return n.FloorDataFp != -1
}

// FloorData returns an IndexInput for reading the floor data.
func (n *TrieNode) FloorData(reader *TrieReader) (store.IndexInput, error) {
	if !n.IsFloor() {
		return nil, errors.New("node does not have floor data")
	}
	// Seek to floor data position
	if err := reader.input.SetPosition(n.FloorDataFp); err != nil {
		return nil, err
	}
	return reader.input, nil
}

// TrieReader reads a trie from disk.
// This is the Go port of TrieReader.
type TrieReader struct {
	Access store.RandomAccessInput
	input  store.IndexInput
	Root   *TrieNode
}

// NewTrieReader creates a new TrieReader from the given input and root file pointer.
// This is the Go port of TrieReader constructor.
func NewTrieReader(input store.IndexInput, rootFP int64) (*TrieReader, error) {
	// Check if input supports random access
	ra, ok := input.(store.RandomAccessInput)
	if !ok {
		return nil, errors.New("input does not support random access")
	}

	reader := &TrieReader{
		Access: ra,
		input:  input,
		Root:   NewTrieNode(),
	}

	// Load root node
	if err := reader.load(reader.Root, rootFP); err != nil {
		return nil, err
	}

	return reader, nil
}

// load loads a node from the given file pointer.
func (tr *TrieReader) load(node *TrieNode, fp int64) error {
	node.Fp = fp

	// Read the term flags
	termFlagsLong, err := tr.Access.ReadLongAt(fp)
	if err != nil {
		return err
	}

	termFlags := int(termFlagsLong & 0xFF)
	sign := termFlags & 0x03
	node.Sign = sign

	switch sign {
	case 0x00: // SIGN_NO_CHILDREN
		return tr.loadLeafNode(node, fp, termFlags, termFlagsLong)
	case 0x03: // SIGN_MULTI_CHILDREN
		return tr.loadMultiChildrenNode(node, fp, termFlags, termFlagsLong)
	default: // SIGN_SINGLE_CHILD_WITH_OUTPUT or SIGN_SINGLE_CHILD_WITHOUT_OUTPUT
		return tr.loadSingleChildNode(node, fp, sign, termFlags, termFlagsLong)
	}
}

// loadLeafNode loads a leaf node.
func (tr *TrieReader) loadLeafNode(node *TrieNode, fp int64, termFlags int, termFlagsLong int64) error {
	fpBytesMinus1 := (termFlags >> 2) & 0x07

	if fpBytesMinus1 <= 6 {
		mask := bytesMinus1Mask(fpBytesMinus1)
		node.OutputFp = (termFlagsLong >> 8) & mask
	} else {
		val, err := tr.Access.ReadLongAt(fp + 1)
		if err != nil {
			return err
		}
		node.OutputFp = val
	}

	node.HasTerms = (termFlags & 0x20) != 0 // LEAF_NODE_HAS_TERMS

	if (termFlags & 0x40) != 0 { // LEAF_NODE_HAS_FLOOR
		node.FloorDataFp = fp + 2 + int64(fpBytesMinus1)
	}

	return nil
}

// loadSingleChildNode loads a single child node.
func (tr *TrieReader) loadSingleChildNode(node *TrieNode, fp int64, sign int, termFlags int, termFlagsLong int64) error {
	childDeltaFpBytesMinus1 := (termFlags >> 2) & 0x07

	var l int64
	if childDeltaFpBytesMinus1 <= 5 {
		mask := bytesMinus1Mask(childDeltaFpBytesMinus1)
		l = (termFlagsLong >> 16) & mask
	} else {
		val, err := tr.Access.ReadLongAt(fp + 2)
		if err != nil {
			return err
		}
		l = val
	}

	node.ChildDeltaFp = l
	node.MinChildrenLabel = (termFlags >> 8) & 0xFF

	if sign == 0x02 { // SIGN_SINGLE_CHILD_WITHOUT_OUTPUT
		node.OutputFp = -1
	} else {
		// SIGN_SINGLE_CHILD_WITH_OUTPUT
		encodedOutputFpBytesMinus1 := (termFlags >> 5) & 0x07
		offset := fp + 3 + int64(childDeltaFpBytesMinus1)
		if encodedOutputFpBytesMinus1 > 6 {
			offset = fp + 2
		}

		encodedFp, err := tr.Access.ReadLongAt(offset)
		if err != nil {
			return err
		}

		mask := bytesMinus1Mask(int(encodedOutputFpBytesMinus1))
		encodedFp &= mask

		node.OutputFp = encodedFp >> 2
		node.HasTerms = (encodedFp & 0x02) != 0 // NON_LEAF_NODE_HAS_TERMS

		if (encodedFp & 0x01) != 0 { // NON_LEAF_NODE_HAS_FLOOR
			node.FloorDataFp = offset + 1 + int64(encodedOutputFpBytesMinus1)
		}
	}

	return nil
}

// loadMultiChildrenNode loads a multi-children node.
func (tr *TrieReader) loadMultiChildrenNode(node *TrieNode, fp int64, termFlags int, termFlagsLong int64) error {
	node.ChildrenDeltaFpBytes = ((termFlags >> 2) & 0x07) + 1
	node.ChildSaveStrategy = (termFlags >> 9) & 0x03
	node.StrategyBytes = ((termFlags >> 11) & 0x1F) + 1
	node.MinChildrenLabel = (termFlags >> 16) & 0xFF

	hasOutput := (termFlags & 0x20) != 0

	if hasOutput {
		encodedOutputFpBytesMinus1 := (termFlags >> 6) & 0x07

		var l int64
		if encodedOutputFpBytesMinus1 <= 4 {
			mask := bytesMinus1Mask(int(encodedOutputFpBytesMinus1))
			l = (termFlagsLong >> 24) & mask
		} else {
			val, err := tr.Access.ReadLongAt(fp + 3)
			if err != nil {
				return err
			}
			l = val
		}

		encodedFp := l & bytesMinus1Mask(int(encodedOutputFpBytesMinus1))
		node.OutputFp = encodedFp >> 2
		node.HasTerms = (encodedFp & 0x02) != 0

		if (encodedFp & 0x01) != 0 {
			offset := fp + 4 + int64(encodedOutputFpBytesMinus1)
			childrenNum, err := tr.Access.ReadByteAt(offset)
			if err != nil {
				return err
			}
			node.StrategyFp = offset + 1
			node.FloorDataFp = node.StrategyFp + int64(node.StrategyBytes) + int64(childrenNum+1)*int64(node.ChildrenDeltaFpBytes)
		} else {
			node.FloorDataFp = -1
			node.StrategyFp = fp + 4 + int64(encodedOutputFpBytesMinus1)
		}
	} else {
		node.OutputFp = -1
		node.StrategyFp = fp + 3
	}

	return nil
}

// LookupChild looks up a child node by label.
// Returns the child node or nil if not found.
// This is the Go port of TrieReader.lookupChild().
func (tr *TrieReader) LookupChild(targetLabel int, parent, child *TrieNode) (*TrieNode, error) {
	sign := parent.Sign

	if sign == 0x00 { // SIGN_NO_CHILDREN
		return nil, nil
	}

	if sign != 0x03 { // Not SIGN_MULTI_CHILDREN
		// Single child
		if targetLabel != parent.MinChildrenLabel {
			return nil, nil
		}
		child.Label = targetLabel
		if err := tr.load(child, parent.Fp-parent.ChildDeltaFp); err != nil {
			return nil, err
		}
		return child, nil
	}

	// Multi children
	strategyBytesStartFp := parent.StrategyFp
	minLabel := parent.MinChildrenLabel
	strategyBytes := parent.StrategyBytes

	position := -1
	if targetLabel == minLabel {
		position = 0
	} else if targetLabel > minLabel {
		pos, err := tr.lookupChildPosition(targetLabel, parent.ChildSaveStrategy, strategyBytesStartFp, strategyBytes, minLabel)
		if err != nil {
			return nil, err
		}
		position = pos
	}

	if position < 0 {
		return nil, nil
	}

	bytesPerEntry := parent.ChildrenDeltaFpBytes
	pos := strategyBytesStartFp + int64(strategyBytes) + int64(bytesPerEntry)*int64(position)

	childDeltaFpVal, err := tr.Access.ReadLongAt(pos)
	if err != nil {
		return nil, err
	}

	mask := bytesMinus1Mask(bytesPerEntry - 1)
	childDeltaFp := childDeltaFpVal & mask

	childFp := parent.Fp - childDeltaFp
	child.Label = targetLabel
	if err := tr.load(child, childFp); err != nil {
		return nil, err
	}

	return child, nil
}

// lookupChildPosition looks up the position of a child label using the given strategy.
func (tr *TrieReader) lookupChildPosition(targetLabel int, strategy int, strategyBytesStartFp int64, strategyBytes int, minLabel int) (int, error) {
	switch strategy {
	case 0: // REVERSE_ARRAY
		return tr.lookupReverseArray(targetLabel, strategyBytesStartFp, strategyBytes, minLabel)
	case 1: // ARRAY
		return tr.lookupArray(targetLabel, strategyBytesStartFp, strategyBytes, minLabel)
	case 2: // BITS
		return tr.lookupBits(targetLabel, strategyBytesStartFp, strategyBytes, minLabel)
	default:
		return -1, fmt.Errorf("unknown child save strategy: %d", strategy)
	}
}

// lookupBits looks up a child position using the BITS strategy.
func (tr *TrieReader) lookupBits(targetLabel int, strategyBytesStartFp int64, strategyBytes int, minLabel int) (int, error) {
	bitIndex := targetLabel - minLabel
	if bitIndex >= strategyBytes*8 {
		return -1, nil
	}

	wordIndex := bitIndex / 64
	wordFp := strategyBytesStartFp + int64(wordIndex)*8

	word, err := tr.Access.ReadLongAt(wordFp)
	if err != nil {
		return -1, err
	}

	mask := int64(1) << uint(bitIndex%64)
	if (word & mask) == 0 {
		return -1, nil
	}

	// Count bits before this position
	pos := 0
	for fp := strategyBytesStartFp; fp < wordFp; fp += 8 {
		w, err := tr.Access.ReadLongAt(fp)
		if err != nil {
			return -1, err
		}
		pos += popcount(uint64(w))
	}

	// Count bits in the current word up to the target bit
	pos += popcount(uint64(word & (mask - 1)))

	return pos, nil
}

// lookupArray looks up a child position using the ARRAY strategy.
func (tr *TrieReader) lookupArray(targetLabel int, strategyBytesStartFp int64, strategyBytes int, minLabel int) (int, error) {
	// Binary search
	low := 0
	high := strategyBytes - 1

	for low <= high {
		mid := (low + high) / 2
		midLabel, err := tr.Access.ReadByteAt(strategyBytesStartFp + int64(mid))
		if err != nil {
			return -1, err
		}

		if int(midLabel)&0xFF < targetLabel {
			low = mid + 1
		} else if int(midLabel)&0xFF > targetLabel {
			high = mid - 1
		} else {
			return mid + 1, nil // min label not included, plus 1
		}
	}

	return -1, nil
}

// lookupReverseArray looks up a child position using the REVERSE_ARRAY strategy.
func (tr *TrieReader) lookupReverseArray(targetLabel int, strategyBytesStartFp int64, strategyBytes int, minLabel int) (int, error) {
	maxLabelByte, err := tr.Access.ReadByteAt(strategyBytesStartFp)
	if err != nil {
		return -1, err
	}
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

	// Binary search
	low := 0
	high := strategyBytes - 2

	for low <= high {
		mid := (low + high) / 2
		midLabel, err := tr.Access.ReadByteAt(strategyBytesStartFp + 1 + int64(mid))
		if err != nil {
			return -1, err
		}

		if int(midLabel)&0xFF < targetLabel {
			low = mid + 1
		} else if int(midLabel)&0xFF > targetLabel {
			high = mid - 1
		} else {
			return -1, nil // Found in absent list
		}
	}

	return targetLabel - minLabel - low, nil
}

// bytesMinus1Mask returns a mask with n+1 bytes set to 1.
func bytesMinus1Mask(n int) int64 {
	masks := []int64{
		0xFF,
		0xFFFF,
		0xFFFFFF,
		0xFFFFFFFF,
		0xFFFFFFFFFF,
		0xFFFFFFFFFFFF,
		0xFFFFFFFFFFFFFF,
		-1, // 0xFFFFFFFFFFFFFFFF as int64
	}
	if n >= 0 && n < len(masks) {
		return masks[n]
	}
	return -1
}

// popcount returns the number of set bits in x.
func popcount(x uint64) int {
	// Using the standard bit counting algorithm
	count := 0
	for x != 0 {
		count++
		x &= x - 1
	}
	return count
}
