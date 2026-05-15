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

package fst

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// FSTCompiler builds a minimal FST from pre-sorted (IntsRef, output)
// pairs. It is the Go port of org.apache.lucene.util.fst.FSTCompiler
// and produces a byte stream that is byte-for-byte identical to
// Apache Lucene 10.4.0 for the same inputs.
//
// Usage:
//
//	compiler := fst.NewFSTCompilerBuilder[int64](
//	    fst.InputTypeByte1, fst.PositiveIntOutputs()).Build()
//	if err := compiler.Add(input, 17); err != nil { ... }
//	metadata, err := compiler.Compile()
//	fst, err := fst.FromFSTReader(metadata, compiler.GetFSTReader())
type FSTCompiler[T any] struct {
	dedupHash *NodeHash[T]
	fst       *FST[T]
	noOutput  T

	lastInput *util.IntsRefBuilder

	// paddingBytePending tracks whether the leading 0x00 byte has
	// already been written. The byte is emitted lazily on the first
	// addNode invocation (or in Compile when the FST is empty but has
	// an empty-output) so that the empty FST corner case can still
	// validly resolve to numBytesWritten == 0.
	paddingBytePending bool

	// frontier is the chain of "in flight" nodes; index i holds the
	// node at depth i along the path of the most recent input.
	frontier []*UnCompiledNode[T]

	lastFrozenNode int64

	// Scratch state for fixed-length arc emission.
	numBytesPerArc        []int
	numLabelBytesPerArc   []int
	fixedLengthArcsBuffer *fixedLengthArcsBuffer

	// Accountancy counters mirroring FSTCompiler.
	ArcCount                  int64
	NodeCount                 int64
	BinarySearchNodeCount     int64
	DirectAddressingNodeCount int64
	ContinuousNodeCount       int64

	allowFixedLengthArcs                bool
	directAddressingMaxOversizingFactor float32
	version                             int
	directAddressingExpansionCredit     int64

	// dataOutput receives the (post-reversal) bytes of each frozen
	// node, in node order. It doubles as an FSTReader when the
	// in-memory ReadWriteDataOutput is used.
	dataOutput store.DataOutput

	scratchBytes    *GrowableByteArrayDataOutput
	numBytesWritten int64
}

// FSTCompiler tuning constants. The defaults match Lucene's static
// finals.
const (
	// DirectAddressingMaxOversizingFactor is the default factor of the
	// permitted oversize for direct addressing relative to binary
	// search.
	DirectAddressingMaxOversizingFactor float32 = 1.0
	// FixedLengthArcShallowDepth is the maximum depth at which a node
	// is force-expanded to fixed-length arcs when its fan-out exceeds
	// FixedLengthArcShallowNumArcs.
	FixedLengthArcShallowDepth = 3
	// FixedLengthArcShallowNumArcs is the fan-out threshold at
	// shallow depths.
	FixedLengthArcShallowNumArcs = 5
	// FixedLengthArcDeepNumArcs is the fan-out threshold beyond which
	// fixed-length arcs are always preferred regardless of depth.
	FixedLengthArcDeepNumArcs = 10
	// directAddressingMaxOversizeWithCreditFactor caps the runaway
	// oversizing while expansion credit is available. See Lucene's
	// internal constant of the same name.
	directAddressingMaxOversizeWithCreditFactor float64 = 1.66
)

// FSTCompilerBuilder is the Go port of FSTCompiler.Builder. It carries
// the fluent configuration knobs and produces a fully wired
// FSTCompiler via Build.
type FSTCompilerBuilder[T any] struct {
	inputType                           InputType
	outputs                             Outputs[T]
	suffixRAMLimitMB                    float64
	allowFixedLengthArcs                bool
	dataOutput                          store.DataOutput
	directAddressingMaxOversizingFactor float32
	version                             int
}

// NewFSTCompilerBuilder returns a builder pre-populated with Lucene's
// default settings.
func NewFSTCompilerBuilder[T any](inputType InputType, outputs Outputs[T]) *FSTCompilerBuilder[T] {
	return &FSTCompilerBuilder[T]{
		inputType:                           inputType,
		outputs:                             outputs,
		suffixRAMLimitMB:                    32.0,
		allowFixedLengthArcs:                true,
		directAddressingMaxOversizingFactor: DirectAddressingMaxOversizingFactor,
		version:                             VERSION_CURRENT,
	}
}

// SuffixRAMLimitMB mirrors FSTCompiler.Builder.suffixRAMLimitMB.
// Setting 0 disables suffix sharing; setting +Inf retains all
// suffixes. The Go port currently retains all suffixes regardless of
// the configured limit; see the package-level NodeHash comment for
// details.
func (b *FSTCompilerBuilder[T]) SuffixRAMLimitMB(mb float64) *FSTCompilerBuilder[T] {
	if mb < 0 {
		panic(fmt.Sprintf("fst: suffixRAMLimitMB must be >= 0; got %f", mb))
	}
	b.suffixRAMLimitMB = mb
	return b
}

// AllowFixedLengthArcs mirrors FSTCompiler.Builder.allowFixedLengthArcs.
func (b *FSTCompilerBuilder[T]) AllowFixedLengthArcs(allow bool) *FSTCompilerBuilder[T] {
	b.allowFixedLengthArcs = allow
	return b
}

// DataOutput mirrors FSTCompiler.Builder.dataOutput. When unset, the
// compiler allocates a fresh in-memory ReadWriteDataOutput.
func (b *FSTCompilerBuilder[T]) DataOutput(out store.DataOutput) *FSTCompilerBuilder[T] {
	if out == nil {
		panic("fst: DataOutput cannot be nil")
	}
	b.dataOutput = out
	return b
}

// DirectAddressingMaxOversizingFactor mirrors the builder setter of
// the same name. Negative values disable direct addressing.
func (b *FSTCompilerBuilder[T]) DirectAddressingMaxOversizingFactor(factor float32) *FSTCompilerBuilder[T] {
	b.directAddressingMaxOversizingFactor = factor
	return b
}

// SetVersion mirrors FSTCompiler.Builder.setVersion.
func (b *FSTCompilerBuilder[T]) SetVersion(version int) *FSTCompilerBuilder[T] {
	if version < VERSION_90 || version > VERSION_CURRENT {
		panic(fmt.Sprintf("fst: version %d out of range [%d, %d]", version, VERSION_90, VERSION_CURRENT))
	}
	b.version = version
	return b
}

// Build allocates and returns an FSTCompiler configured per this
// builder. Equivalent to FSTCompiler.Builder.build.
func (b *FSTCompilerBuilder[T]) Build() *FSTCompiler[T] {
	out := b.dataOutput
	if out == nil {
		out = NewReadWriteDataOutput(15)
	}
	return newFSTCompiler[T](
		b.inputType,
		b.suffixRAMLimitMB,
		b.outputs,
		b.allowFixedLengthArcs,
		out,
		b.directAddressingMaxOversizingFactor,
		b.version,
	)
}

// newFSTCompiler is the unexported constructor; callers should go via
// NewFSTCompilerBuilder. Mirrors FSTCompiler's private constructor.
func newFSTCompiler[T any](
	inputType InputType,
	suffixRAMLimitMB float64,
	outputs Outputs[T],
	allowFixedLengthArcs bool,
	dataOutput store.DataOutput,
	directAddressingMaxOversizingFactor float32,
	version int,
) *FSTCompiler[T] {
	noOut := outputs.GetNoOutput()
	c := &FSTCompiler[T]{
		noOutput:                            noOut,
		lastInput:                           util.NewIntsRefBuilder(),
		paddingBytePending:                  true,
		numBytesWritten:                     1, // reserve byte 0 for the padding sentinel
		dataOutput:                          dataOutput,
		scratchBytes:                        NewGrowableByteArrayDataOutput(),
		allowFixedLengthArcs:                allowFixedLengthArcs,
		directAddressingMaxOversizingFactor: directAddressingMaxOversizingFactor,
		version:                             version,
		numBytesPerArc:                      make([]int, 4),
		numLabelBytesPerArc:                 make([]int, 4),
		fixedLengthArcsBuffer:               newFixedLengthArcsBuffer(),
	}
	// Build the placeholder FST that the compiler uses internally for
	// re-parsing frozen nodes (NodeHash relies on it). startNode = -1
	// means "still building".
	meta := NewFSTMetadata[T](inputType, outputs, noOut, false, -1, version, 0)
	fstInst, err := NewFSTFromReader[T](meta, nullFSTReader{})
	if err != nil {
		// NewFSTFromReader only rejects nil arguments; both are non-nil
		// here so this is genuinely unreachable.
		panic(fmt.Sprintf("fst: unexpected internal error wiring FSTCompiler.fst: %v", err))
	}
	c.fst = fstInst
	// Seed the frontier with empty nodes.
	c.frontier = make([]*UnCompiledNode[T], 10)
	for i := range c.frontier {
		c.frontier[i] = newUnCompiledNode[T](c, i)
	}
	if suffixRAMLimitMB > 0 {
		bytesLimit := int64(suffixRAMLimitMB * 1024 * 1024)
		if bytesLimit < 0 { // overflow guard
			bytesLimit = 0
		}
		nh, err := NewNodeHash[T](c, bytesLimit)
		if err != nil {
			panic(fmt.Sprintf("fst: unexpected internal error wiring NodeHash: %v", err))
		}
		c.dedupHash = nh
	}
	return c
}

// GetNodeCount returns the number of frozen nodes including the
// implicit -1 stop node; mirrors FSTCompiler.getNodeCount.
func (c *FSTCompiler[T]) GetNodeCount() int64 { return 1 + c.NodeCount }

// GetArcCount returns the number of arcs across all frozen nodes.
func (c *FSTCompiler[T]) GetArcCount() int64 { return c.ArcCount }

// GetDirectAddressingMaxOversizingFactor exposes the configured
// oversizing factor; mirrors the Java method.
func (c *FSTCompiler[T]) GetDirectAddressingMaxOversizingFactor() float32 {
	return c.directAddressingMaxOversizingFactor
}

// FSTSizeInBytes reports the number of bytes the FST byte stream
// currently occupies, mirroring fstSizeInBytes.
func (c *FSTCompiler[T]) FSTSizeInBytes() int64 { return c.numBytesWritten }

// GetFSTReader returns the configured DataOutput cast to FSTReader, so
// callers can build an FST directly from the in-memory bytes. Panics
// if the configured DataOutput is not an FSTReader (Lucene throws
// IllegalStateException in the same situation).
func (c *FSTCompiler[T]) GetFSTReader() FSTReader {
	if r, ok := c.dataOutput.(FSTReader); ok {
		return r
	}
	panic("fst: FSTCompiler.GetFSTReader: configured DataOutput does not implement FSTReader")
}

// Add appends a sorted (input, output) pair. input must compare
// strictly greater than or equal to the previous input under
// util.IntsRefCompare. Re-adding the same input with a different
// output is supported only when the Outputs algebra implements Merge.
//
// Mirrors FSTCompiler.add(IntsRef, T).
func (c *FSTCompiler[T]) Add(input *util.IntsRef, output T) error {
	// De-dup NO_OUTPUT singleton.
	if c.outputsEqualToNoOutput(output) {
		output = c.noOutput
	}
	// Empty input: only allowed as the very first input.
	if input.Length == 0 {
		c.frontier[0].isFinal = true
		c.setEmptyOutput(output)
		return nil
	}
	if c.lastInput.Length() > 0 {
		cmp := util.IntsRefCompare(input, c.lastInput.Get())
		if cmp < 0 {
			return fmt.Errorf(
				"fst: inputs are added out of order: lastInput=%v vs input=%v",
				c.lastInput.Get(), input,
			)
		}
	}
	// Compute the shared prefix length with the previous input.
	pos1 := 0
	pos2 := input.Offset
	pos1Stop := c.lastInput.Length()
	if input.Length < pos1Stop {
		pos1Stop = input.Length
	}
	for pos1 < pos1Stop && c.lastInput.IntAt(pos1) == input.Ints[pos2] {
		pos1++
		pos2++
	}
	prefixLenPlus1 := pos1 + 1

	// Grow the frontier if needed.
	if len(c.frontier) < input.Length+1 {
		grow := util.Oversize(input.Length+1, 8)
		next := make([]*UnCompiledNode[T], grow)
		copy(next, c.frontier)
		for i := len(c.frontier); i < grow; i++ {
			next[i] = newUnCompiledNode[T](c, i)
		}
		c.frontier = next
	}

	// Minimise/compile states from the previous input's orphaned suffix.
	if err := c.freezeTail(prefixLenPlus1); err != nil {
		return err
	}

	// Initialise tail states for the current input.
	for idx := prefixLenPlus1; idx <= input.Length; idx++ {
		c.frontier[idx-1].addArc(input.Ints[input.Offset+idx-1], c.frontier[idx])
	}

	lastNode := c.frontier[input.Length]
	if c.lastInput.Length() != input.Length || prefixLenPlus1 != input.Length+1 {
		lastNode.isFinal = true
		lastNode.output = c.noOutput
	}

	// Push conflicting outputs forward, only as far as needed.
	for idx := 1; idx < prefixLenPlus1; idx++ {
		node := c.frontier[idx]
		parent := c.frontier[idx-1]
		lastOutput := parent.getLastOutput(input.Ints[input.Offset+idx-1])
		var commonOutputPrefix T
		if !c.outputsEqualToNoOutput(lastOutput) {
			commonOutputPrefix = c.fst.outputs.Common(output, lastOutput)
			wordSuffix := c.fst.outputs.Subtract(lastOutput, commonOutputPrefix)
			parent.setLastOutput(input.Ints[input.Offset+idx-1], commonOutputPrefix)
			node.prependOutput(wordSuffix)
		} else {
			commonOutputPrefix = c.noOutput
		}
		output = c.fst.outputs.Subtract(output, commonOutputPrefix)
	}

	if c.lastInput.Length() == input.Length && prefixLenPlus1 == 1+input.Length {
		// Same input as last time: merge outputs.
		merged, err := c.fst.outputs.Merge(lastNode.output, output)
		if err != nil {
			return err
		}
		lastNode.output = merged
	} else {
		c.frontier[prefixLenPlus1-1].setLastOutput(
			input.Ints[input.Offset+prefixLenPlus1-1], output,
		)
	}

	// Save last input for the next call.
	c.lastInput.CopyIntsRef(input)
	return nil
}

// outputsEqualToNoOutput returns true when output equals the
// configured Outputs no-output singleton. Mirrors the Lucene
// dedup test "output.equals(NO_OUTPUT)".
func (c *FSTCompiler[T]) outputsEqualToNoOutput(output T) bool {
	return any(output) == any(c.noOutput) || outputsDeepEqual[T](output, c.noOutput)
}

// freezeTail walks the frontier from the deepest node down to
// prefixLenPlus1 and compiles every node it traverses. Mirrors
// FSTCompiler.freezeTail.
func (c *FSTCompiler[T]) freezeTail(prefixLenPlus1 int) error {
	downTo := prefixLenPlus1
	if downTo < 1 {
		downTo = 1
	}
	for idx := c.lastInput.Length(); idx >= downTo; idx-- {
		node := c.frontier[idx]
		prevIdx := idx - 1
		parent := c.frontier[prevIdx]
		nextFinalOutput := node.output
		isFinal := node.isFinal
		compiled, err := c.compileNode(node)
		if err != nil {
			return err
		}
		parent.replaceLast(c.lastInput.IntAt(prevIdx), compiled, nextFinalOutput, isFinal)
	}
	return nil
}

// compileNode freezes a single UnCompiledNode and returns the
// corresponding CompiledNode wrapper. Mirrors FSTCompiler.compileNode.
//
// The lastFrozenNode field is updated in two situations, matching
// Lucene exactly:
//  1. With dedupHash enabled and numArcs==0, lastFrozenNode is set
//     to the just-returned stop-node sentinel even though addNode
//     wrote no bytes. This causes downstream arcs whose target is
//     also a stop node to set BIT_TARGET_NEXT; the reader ignores
//     that bit when BIT_STOP_NODE is already set, so the behaviour
//     is byte-identical to Lucene and reader-equivalent.
//  2. Whenever addNode actually appended bytes (the only case for
//     real, non-stop nodes).
func (c *FSTCompiler[T]) compileNode(nodeIn *UnCompiledNode[T]) (*CompiledNode, error) {
	bytesPosStart := c.numBytesWritten
	var node int64
	var err error
	if c.dedupHash != nil {
		if nodeIn.numArcs == 0 {
			node, err = c.addNode(nodeIn)
			if err != nil {
				return nil, err
			}
			c.lastFrozenNode = node
		} else {
			node, err = c.dedupHash.Add(nodeIn)
			if err != nil {
				return nil, err
			}
		}
	} else {
		node, err = c.addNode(nodeIn)
		if err != nil {
			return nil, err
		}
	}
	bytesPosEnd := c.numBytesWritten
	if bytesPosEnd != bytesPosStart {
		c.lastFrozenNode = node
	}
	nodeIn.clear()
	return &CompiledNode{node: node}, nil
}

// addNode serialises nodeIn to scratchBytes, reverses the buffer in
// place, then flushes it to dataOutput. Returns the FST address of
// the node (the offset of its last byte in the byte stream).
//
// Mirrors FSTCompiler.addNode.
func (c *FSTCompiler[T]) addNode(nodeIn *UnCompiledNode[T]) (int64, error) {
	if nodeIn.numArcs == 0 {
		if nodeIn.isFinal {
			return FinalEndNode, nil
		}
		return NonFinalEndNode, nil
	}
	c.scratchBytes.SetPosition(0)
	doFixedLengthArcs := c.shouldExpandNodeWithFixedLengthArcs(nodeIn)
	if doFixedLengthArcs && len(c.numBytesPerArc) < nodeIn.numArcs {
		grow := util.Oversize(nodeIn.numArcs, 4)
		c.numBytesPerArc = make([]int, grow)
		c.numLabelBytesPerArc = make([]int, grow)
	}
	c.ArcCount += int64(nodeIn.numArcs)
	lastArc := nodeIn.numArcs - 1
	lastArcStart := 0
	maxBytesPerArc := 0
	maxBytesPerArcWithoutLabel := 0
	for arcIdx := 0; arcIdx < nodeIn.numArcs; arcIdx++ {
		arc := &nodeIn.arcs[arcIdx]
		target := arc.target.(*CompiledNode)
		flags := 0
		if arcIdx == lastArc {
			flags |= BIT_LAST_ARC
		}
		if c.lastFrozenNode == target.node && !doFixedLengthArcs {
			flags |= BIT_TARGET_NEXT
		}
		if arc.isFinal {
			flags |= BIT_FINAL_ARC
			if !c.outputsEqualToNoOutput(arc.nextFinalOutput) {
				flags |= BIT_ARC_HAS_FINAL_OUTPUT
			}
		}
		targetHasArcs := target.node > 0
		if !targetHasArcs {
			flags |= BIT_STOP_NODE
		}
		if !c.outputsEqualToNoOutput(arc.output) {
			flags |= BIT_ARC_HAS_OUTPUT
		}
		if err := c.scratchBytes.WriteByte(byte(flags)); err != nil {
			return 0, err
		}
		labelStart := c.scratchBytes.GetPosition()
		if err := c.writeLabel(c.scratchBytes, arc.label); err != nil {
			return 0, err
		}
		numLabelBytes := c.scratchBytes.GetPosition() - labelStart
		if flags&BIT_ARC_HAS_OUTPUT != 0 {
			if err := c.fst.outputs.Write(arc.output, c.scratchBytes); err != nil {
				return 0, err
			}
		}
		if flags&BIT_ARC_HAS_FINAL_OUTPUT != 0 {
			if err := c.fst.outputs.WriteFinalOutput(arc.nextFinalOutput, c.scratchBytes); err != nil {
				return 0, err
			}
		}
		if targetHasArcs && (flags&BIT_TARGET_NEXT) == 0 {
			if err := c.scratchBytes.WriteVLong(target.node); err != nil {
				return 0, err
			}
		}
		if doFixedLengthArcs {
			numArcBytes := c.scratchBytes.GetPosition() - lastArcStart
			c.numBytesPerArc[arcIdx] = numArcBytes
			c.numLabelBytesPerArc[arcIdx] = numLabelBytes
			lastArcStart = c.scratchBytes.GetPosition()
			if numArcBytes > maxBytesPerArc {
				maxBytesPerArc = numArcBytes
			}
			if numArcBytes-numLabelBytes > maxBytesPerArcWithoutLabel {
				maxBytesPerArcWithoutLabel = numArcBytes - numLabelBytes
			}
		}
	}

	if doFixedLengthArcs {
		labelRange := nodeIn.arcs[nodeIn.numArcs-1].label - nodeIn.arcs[0].label + 1
		continuousLabel := labelRange == nodeIn.numArcs
		switch {
		case continuousLabel && c.version >= VERSION_CONTINUOUS_ARCS:
			c.writeNodeForDirectAddressingOrContinuous(nodeIn, maxBytesPerArcWithoutLabel, labelRange, true)
			c.ContinuousNodeCount++
		case c.shouldExpandNodeWithDirectAddressing(nodeIn, maxBytesPerArc, maxBytesPerArcWithoutLabel, labelRange):
			c.writeNodeForDirectAddressingOrContinuous(nodeIn, maxBytesPerArcWithoutLabel, labelRange, false)
			c.DirectAddressingNodeCount++
		default:
			c.writeNodeForBinarySearch(nodeIn, maxBytesPerArc)
			c.BinarySearchNodeCount++
		}
	}

	c.reverseScratchBytes()
	if c.paddingBytePending {
		if err := c.writePaddingByte(); err != nil {
			return 0, err
		}
	}
	if err := c.scratchBytes.WriteTo(c.dataOutput); err != nil {
		return 0, err
	}
	c.numBytesWritten += int64(c.scratchBytes.GetPosition())
	c.NodeCount++
	return c.numBytesWritten - 1, nil
}

// writePaddingByte writes the leading 0x00 sentinel into dataOutput
// so that no real node ends up at address 0 (which is reserved for
// NON_FINAL_END_NODE).
func (c *FSTCompiler[T]) writePaddingByte() error {
	if err := c.dataOutput.WriteByte(0); err != nil {
		return err
	}
	c.paddingBytePending = false
	return nil
}

// writeLabel emits one input label according to the FST's input type.
func (c *FSTCompiler[T]) writeLabel(out *GrowableByteArrayDataOutput, v int) error {
	if v < 0 {
		return fmt.Errorf("fst: writeLabel: negative label %d", v)
	}
	switch c.fst.metadata.inputType {
	case InputTypeByte1:
		if v > 0xFF {
			return fmt.Errorf("fst: writeLabel BYTE1: label %d > 0xFF", v)
		}
		return out.WriteByte(byte(v))
	case InputTypeByte2:
		if v > 0xFFFF {
			return fmt.Errorf("fst: writeLabel BYTE2: label %d > 0xFFFF", v)
		}
		return out.WriteShort(int16(v))
	case InputTypeByte4:
		return out.WriteVInt(int32(v))
	default:
		return fmt.Errorf("fst: writeLabel: unknown input type %v", c.fst.metadata.inputType)
	}
}

// shouldExpandNodeWithFixedLengthArcs mirrors the Java predicate.
func (c *FSTCompiler[T]) shouldExpandNodeWithFixedLengthArcs(node *UnCompiledNode[T]) bool {
	if !c.allowFixedLengthArcs {
		return false
	}
	shallow := node.depth <= FixedLengthArcShallowDepth && node.numArcs >= FixedLengthArcShallowNumArcs
	deep := node.numArcs >= FixedLengthArcDeepNumArcs
	return shallow || deep
}

// shouldExpandNodeWithDirectAddressing implements the cost/credit
// heuristic exactly as in FSTCompiler.
func (c *FSTCompiler[T]) shouldExpandNodeWithDirectAddressing(
	nodeIn *UnCompiledNode[T], numBytesPerArc, maxBytesPerArcWithoutLabel, labelRange int,
) bool {
	sizeForBinarySearch := numBytesPerArc * nodeIn.numArcs
	sizeForDirectAddressing := getNumPresenceBytes(labelRange) +
		c.numLabelBytesPerArc[0] + maxBytesPerArcWithoutLabel*nodeIn.numArcs
	allowedOversize := int(float32(sizeForBinarySearch) * c.directAddressingMaxOversizingFactor)
	expansionCost := sizeForDirectAddressing - allowedOversize
	if expansionCost <= 0 ||
		(c.directAddressingExpansionCredit >= int64(expansionCost) &&
			float64(sizeForDirectAddressing) <= float64(allowedOversize)*directAddressingMaxOversizeWithCreditFactor) {
		c.directAddressingExpansionCredit -= int64(expansionCost)
		return true
	}
	return false
}

// writeNodeForBinarySearch emits the binary-search node header and
// re-expands the per-arc bytes so each arc occupies exactly
// maxBytesPerArc bytes. Mirrors FSTCompiler.writeNodeForBinarySearch.
func (c *FSTCompiler[T]) writeNodeForBinarySearch(nodeIn *UnCompiledNode[T], maxBytesPerArc int) {
	c.fixedLengthArcsBuffer.resetPosition().
		writeByte(ARCS_FOR_BINARY_SEARCH).
		writeVInt(int32(nodeIn.numArcs)).
		writeVInt(int32(maxBytesPerArc))
	headerLen := c.fixedLengthArcsBuffer.getPosition()

	srcPos := c.scratchBytes.GetPosition()
	destPos := headerLen + nodeIn.numArcs*maxBytesPerArc
	if destPos > srcPos {
		c.scratchBytes.SetPosition(destPos)
		for arcIdx := nodeIn.numArcs - 1; arcIdx >= 0; arcIdx-- {
			destPos -= maxBytesPerArc
			arcLen := c.numBytesPerArc[arcIdx]
			srcPos -= arcLen
			if srcPos != destPos {
				c.writeScratchBytes(destPos, c.scratchBytes.GetBytes(), srcPos, arcLen)
			}
		}
	}
	c.writeScratchBytes(0, c.fixedLengthArcsBuffer.getBytes(), 0, headerLen)
}

// writeScratchBytes copies bytes into scratchBytes at destPos. The
// caller must ensure destPos + length lies within the populated
// prefix; mirrors the Java helper of the same name.
func (c *FSTCompiler[T]) writeScratchBytes(destPos int, bytes []byte, offset, length int) {
	if destPos+length > c.scratchBytes.GetPosition() {
		panic(fmt.Sprintf("fst: writeScratchBytes: out of range destPos=%d length=%d nextWrite=%d",
			destPos, length, c.scratchBytes.GetPosition()))
	}
	dst := c.scratchBytes.GetBytes()
	copy(dst[destPos:destPos+length], bytes[offset:offset+length])
}

// writeNodeForDirectAddressingOrContinuous mirrors the Java helper
// that emits either an ARCS_FOR_DIRECT_ADDRESSING or
// ARCS_FOR_CONTINUOUS node. The label bytes are dropped from every
// arc except the first (which carries the firstLabel that other arc
// labels are computed relative to).
func (c *FSTCompiler[T]) writeNodeForDirectAddressingOrContinuous(
	nodeIn *UnCompiledNode[T], maxBytesPerArcWithoutLabel, labelRange int, continuous bool,
) {
	headerMaxLen := 11
	numPresenceBytes := 0
	if !continuous {
		numPresenceBytes = getNumPresenceBytes(labelRange)
	}
	srcPos := c.scratchBytes.GetPosition()
	totalArcBytes := c.numLabelBytesPerArc[0] + nodeIn.numArcs*maxBytesPerArcWithoutLabel
	bufferOffset := headerMaxLen + numPresenceBytes + totalArcBytes
	c.fixedLengthArcsBuffer.ensureCapacity(bufferOffset)
	buffer := c.fixedLengthArcsBuffer.getBytes()
	for arcIdx := nodeIn.numArcs - 1; arcIdx >= 0; arcIdx-- {
		bufferOffset -= maxBytesPerArcWithoutLabel
		srcArcLen := c.numBytesPerArc[arcIdx]
		srcPos -= srcArcLen
		labelLen := c.numLabelBytesPerArc[arcIdx]
		c.scratchBytes.WriteToBytes(srcPos, buffer, bufferOffset, 1)
		remainingArcLen := srcArcLen - 1 - labelLen
		if remainingArcLen != 0 {
			c.scratchBytes.WriteToBytes(srcPos+1+labelLen, buffer, bufferOffset+1, remainingArcLen)
		}
		if arcIdx == 0 {
			bufferOffset -= labelLen
			c.scratchBytes.WriteToBytes(srcPos+1, buffer, bufferOffset, labelLen)
		}
	}
	header := ARCS_FOR_DIRECT_ADDRESSING
	if continuous {
		header = ARCS_FOR_CONTINUOUS
	}
	c.fixedLengthArcsBuffer.resetPosition().
		writeByte(header).
		writeVInt(int32(labelRange)).
		writeVInt(int32(maxBytesPerArcWithoutLabel))
	headerLen := c.fixedLengthArcsBuffer.getPosition()

	c.scratchBytes.SetPosition(0)
	if err := c.scratchBytes.WriteBytesN(c.fixedLengthArcsBuffer.getBytes(), headerLen); err != nil {
		panic(fmt.Sprintf("fst: writing fixed-length-arcs header into scratch: %v", err))
	}
	if !continuous {
		c.writePresenceBits(nodeIn)
	}
	if err := c.scratchBytes.WriteBytesN(c.fixedLengthArcsBuffer.getBytes()[bufferOffset:bufferOffset+totalArcBytes], totalArcBytes); err != nil {
		panic(fmt.Sprintf("fst: writing fixed-length arcs into scratch: %v", err))
	}
}

// writePresenceBits emits the presence bit-table that accompanies a
// direct-addressing node. Each byte holds up to 8 bits, low-bit first.
// The first arc is always present.
func (c *FSTCompiler[T]) writePresenceBits(nodeIn *UnCompiledNode[T]) {
	presenceBits := byte(1)
	presenceIndex := 0
	previousLabel := nodeIn.arcs[0].label
	for arcIdx := 1; arcIdx < nodeIn.numArcs; arcIdx++ {
		label := nodeIn.arcs[arcIdx].label
		presenceIndex += label - previousLabel
		for presenceIndex >= 8 {
			if err := c.scratchBytes.WriteByte(presenceBits); err != nil {
				panic(fmt.Sprintf("fst: writePresenceBits: %v", err))
			}
			presenceBits = 0
			presenceIndex -= 8
		}
		presenceBits |= 1 << presenceIndex
		previousLabel = label
	}
	if err := c.scratchBytes.WriteByte(presenceBits); err != nil {
		panic(fmt.Sprintf("fst: writePresenceBits trailing: %v", err))
	}
}

// reverseScratchBytes reverses the populated prefix of scratchBytes in
// place.
func (c *FSTCompiler[T]) reverseScratchBytes() {
	pos := c.scratchBytes.GetPosition()
	bytes := c.scratchBytes.GetBytes()
	for i, j := 0, pos-1; i < j; i, j = i+1, j-1 {
		bytes[i], bytes[j] = bytes[j], bytes[i]
	}
}

// setEmptyOutput records the output for the empty input. Subsequent
// calls merge via the Outputs algebra; mirrors FSTCompiler.setEmptyOutput.
func (c *FSTCompiler[T]) setEmptyOutput(v T) {
	if c.fst.metadata.hasEmptyOutput {
		merged, err := c.fst.outputs.Merge(c.fst.metadata.emptyOutput, v)
		if err != nil {
			panic(fmt.Sprintf("fst: setEmptyOutput merge: %v", err))
		}
		c.fst.metadata.emptyOutput = merged
	} else {
		c.fst.metadata.emptyOutput = v
		c.fst.metadata.hasEmptyOutput = true
	}
}

// finish closes the FST byte stream and commits the start-node
// address into the metadata. Mirrors FSTCompiler.finish.
func (c *FSTCompiler[T]) finish(newStartNode int64) error {
	if newStartNode > c.numBytesWritten {
		return fmt.Errorf("fst: finish: newStartNode=%d exceeds numBytesWritten=%d", newStartNode, c.numBytesWritten)
	}
	if c.fst.metadata.startNode != -1 {
		return errors.New("fst: compiler already finished")
	}
	if newStartNode == FinalEndNode && c.fst.metadata.hasEmptyOutput {
		newStartNode = 0
	}
	c.fst.metadata.startNode = newStartNode
	c.fst.metadata.numBytes = c.numBytesWritten
	// If the configured DataOutput is the in-memory writer, freeze it
	// so subsequent reads see a stable byte slice.
	if rw, ok := c.dataOutput.(*ReadWriteDataOutput); ok {
		rw.Freeze()
	}
	return nil
}

// Compile finalises the FST and returns its metadata, or nil if the
// FST accepts nothing. Mirrors FSTCompiler.compile.
func (c *FSTCompiler[T]) Compile() (*FSTMetadata[T], error) {
	root := c.frontier[0]
	if err := c.freezeTail(0); err != nil {
		return nil, err
	}
	if root.numArcs == 0 {
		if !c.fst.metadata.hasEmptyOutput {
			return nil, nil
		}
		// The padding byte has not been emitted yet (no node call has
		// reached addNode); emit it now so the FST is still
		// well-formed.
		if c.paddingBytePending {
			if err := c.writePaddingByte(); err != nil {
				return nil, err
			}
		}
	}
	rootNode, err := c.compileNode(root)
	if err != nil {
		return nil, err
	}
	if err := c.finish(rootNode.node); err != nil {
		return nil, err
	}
	return c.fst.metadata, nil
}

// nullFSTReader is the in-package equivalent of Lucene's
// NullFSTReader: it is wired into the compiler's internal FST while
// building, and is replaced by the real reader once Compile finishes.
type nullFSTReader struct{}

func (nullFSTReader) GetReverseBytesReader() BytesReader { return nil }
func (nullFSTReader) WriteTo(_ store.DataOutput) error   { return nil }
func (nullFSTReader) RAMBytesUsed() int64                { return 0 }
