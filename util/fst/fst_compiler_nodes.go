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
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// Node is the marker interface for the in-flight nodes referenced by
// FSTCompiler.Arc.target. It is the Go port of the package-private
// org.apache.lucene.util.fst.FSTCompiler.Node.
type Node interface {
	// IsCompiled reports whether the node has been written to the FST
	// byte stream.
	IsCompiled() bool
}

// CompiledNode is the frozen counterpart to UnCompiledNode. Once a
// node has been written to the FST byte stream via FSTCompiler.addNode,
// the only thing the rest of the algorithm needs from it is the byte
// address; that address is stored here. Mirrors
// FSTCompiler.CompiledNode.
type CompiledNode struct {
	node int64
}

// IsCompiled reports true for CompiledNode.
func (*CompiledNode) IsCompiled() bool { return true }

// NodeAddress returns the FST byte-stream address of this frozen
// node, or one of the virtual constants FinalEndNode / NonFinalEndNode.
func (n *CompiledNode) NodeAddress() int64 { return n.node }

// CompilerArc is the pending arc carried by an UnCompiledNode during
// FST construction. It is the Go port of FSTCompiler.Arc. Once the
// owning node is frozen via addNode, the arc data is consumed and the
// receiver may be reused for the next traversal.
type CompilerArc[T any] struct {
	label           int
	target          Node
	isFinal         bool
	output          T
	nextFinalOutput T
}

// Label returns the arc's input label.
func (a *CompilerArc[T]) Label() int { return a.label }

// Target returns the destination node (Node interface).
func (a *CompilerArc[T]) Target() Node { return a.target }

// IsFinal reports whether the arc carries a final marker.
func (a *CompilerArc[T]) IsFinal() bool { return a.isFinal }

// Output returns the arc output.
func (a *CompilerArc[T]) Output() T { return a.output }

// NextFinalOutput returns the final-output value attached to the arc.
func (a *CompilerArc[T]) NextFinalOutput() T { return a.nextFinalOutput }

// UnCompiledNode is the mutable, in-flight version of a node. It holds
// a growing arcs list and the per-node output/isFinal state until the
// compiler decides to freeze it. Mirrors FSTCompiler.UnCompiledNode.
type UnCompiledNode[T any] struct {
	owner   *FSTCompiler[T]
	numArcs int
	arcs    []CompilerArc[T]
	output  T
	isFinal bool
	depth   int
}

// newUnCompiledNode allocates a fresh node with capacity for one arc.
// The depth is the distance from the automaton root and must remain
// stable across reuse cycles.
func newUnCompiledNode[T any](owner *FSTCompiler[T], depth int) *UnCompiledNode[T] {
	return &UnCompiledNode[T]{
		owner:  owner,
		arcs:   make([]CompilerArc[T], 1),
		output: owner.noOutput,
		depth:  depth,
	}
}

// IsCompiled reports false for UnCompiledNode.
func (*UnCompiledNode[T]) IsCompiled() bool { return false }

// Depth returns the node's depth as set at construction time.
func (n *UnCompiledNode[T]) Depth() int { return n.depth }

// NumArcs returns the count of pending arcs.
func (n *UnCompiledNode[T]) NumArcs() int { return n.numArcs }

// Output returns the node-level output value.
func (n *UnCompiledNode[T]) Output() T { return n.output }

// IsFinal reports the final marker.
func (n *UnCompiledNode[T]) IsFinalFlag() bool { return n.isFinal }

// Arcs returns the slice of pending arcs.
func (n *UnCompiledNode[T]) Arcs() []CompilerArc[T] { return n.arcs[:n.numArcs] }

// clear resets the per-node mutable state for reuse. depth is
// preserved.
func (n *UnCompiledNode[T]) clear() {
	n.numArcs = 0
	n.isFinal = false
	n.output = n.owner.noOutput
}

// getLastOutput returns the output stored on the most recently added
// arc. The caller passes labelToMatch only for an internal sanity
// check, mirroring Lucene's `assert arcs[numArcs - 1].label == labelToMatch`.
func (n *UnCompiledNode[T]) getLastOutput(labelToMatch int) T {
	if n.numArcs == 0 {
		panic("fst: getLastOutput on empty node")
	}
	if n.arcs[n.numArcs-1].label != labelToMatch {
		panic(fmt.Sprintf("fst: getLastOutput label mismatch: arc.label=%d match=%d",
			n.arcs[n.numArcs-1].label, labelToMatch))
	}
	return n.arcs[n.numArcs-1].output
}

// addArc appends a new arc with the given label and target.
func (n *UnCompiledNode[T]) addArc(label int, target Node) {
	if label < 0 {
		panic(fmt.Sprintf("fst: addArc: negative label %d", label))
	}
	if n.numArcs > 0 && label <= n.arcs[n.numArcs-1].label {
		panic(fmt.Sprintf(
			"fst: addArc: label %d must be strictly greater than previous %d",
			label, n.arcs[n.numArcs-1].label,
		))
	}
	if n.numArcs == len(n.arcs) {
		grow := util.Oversize(n.numArcs+1, 8)
		next := make([]CompilerArc[T], grow)
		copy(next, n.arcs)
		n.arcs = next
	}
	arc := &n.arcs[n.numArcs]
	arc.label = label
	arc.target = target
	arc.output = n.owner.noOutput
	arc.nextFinalOutput = n.owner.noOutput
	arc.isFinal = false
	n.numArcs++
}

// replaceLast mutates the most recently added arc's target/final
// output. Used by freezeTail to plug in the freshly-compiled child.
func (n *UnCompiledNode[T]) replaceLast(labelToMatch int, target Node, nextFinalOutput T, isFinal bool) {
	if n.numArcs == 0 {
		panic("fst: replaceLast on empty node")
	}
	arc := &n.arcs[n.numArcs-1]
	if arc.label != labelToMatch {
		panic(fmt.Sprintf("fst: replaceLast label mismatch: arc.label=%d match=%d", arc.label, labelToMatch))
	}
	arc.target = target
	arc.nextFinalOutput = nextFinalOutput
	arc.isFinal = isFinal
}

// setLastOutput overwrites the output of the most recently added arc.
func (n *UnCompiledNode[T]) setLastOutput(labelToMatch int, newOutput T) {
	if n.numArcs == 0 {
		panic("fst: setLastOutput on empty node")
	}
	arc := &n.arcs[n.numArcs-1]
	if arc.label != labelToMatch {
		panic(fmt.Sprintf("fst: setLastOutput label mismatch: arc.label=%d match=%d", arc.label, labelToMatch))
	}
	arc.output = newOutput
}

// prependOutput shifts an output prefix onto every arc of this node.
// Used while pushing conflicting outputs back along the shared
// prefix. Mirrors UnCompiledNode.prependOutput.
func (n *UnCompiledNode[T]) prependOutput(outputPrefix T) {
	for i := 0; i < n.numArcs; i++ {
		n.arcs[i].output = n.owner.fst.outputs.Add(outputPrefix, n.arcs[i].output)
	}
	if n.isFinal {
		n.output = n.owner.fst.outputs.Add(outputPrefix, n.output)
	}
}

// fixedLengthArcsBuffer is the small reusable buffer used while
// emitting fixed-length arcs (binary search or direct addressing).
// Mirrors FSTCompiler.FixedLengthArcsBuffer.
//
// The buffer combines a single contiguous byte slice with a
// monotonically-increasing write head; ensureCapacity grows the
// slice while preserving existing bytes, and resetPosition rewinds
// the head to zero. The header is written via writeByte/writeVInt at
// the start of the buffer; bytes laid out for arc bodies are written
// at higher offsets directly into getBytes by the caller. Both views
// share the same backing slice.
type fixedLengthArcsBuffer struct {
	bytes []byte
	pos   int
}

func newFixedLengthArcsBuffer() *fixedLengthArcsBuffer {
	// Header byte + 2 VInt fields fit in 11 bytes worst-case.
	return &fixedLengthArcsBuffer{bytes: make([]byte, 11)}
}

func (b *fixedLengthArcsBuffer) resetPosition() *fixedLengthArcsBuffer {
	b.pos = 0
	return b
}

func (b *fixedLengthArcsBuffer) writeByte(by byte) *fixedLengthArcsBuffer {
	b.ensureSize(b.pos + 1)
	b.bytes[b.pos] = by
	b.pos++
	return b
}

func (b *fixedLengthArcsBuffer) writeVInt(v int32) *fixedLengthArcsBuffer {
	for v&^int32(0x7F) != 0 {
		b.writeByte(byte((v & 0x7F) | 0x80))
		v = int32(uint32(v) >> 7)
	}
	return b.writeByte(byte(v))
}

func (b *fixedLengthArcsBuffer) getPosition() int { return b.pos }

// getBytes returns the underlying byte slice. The slice is mutable
// and aliases the buffer's storage; the caller must not retain it
// across an ensureCapacity call.
func (b *fixedLengthArcsBuffer) getBytes() []byte { return b.bytes }

// ensureCapacity grows the buffer so that callers can address byte
// offsets up to (but not including) capacity. The buffer's logical
// position is preserved.
func (b *fixedLengthArcsBuffer) ensureCapacity(capacity int) *fixedLengthArcsBuffer {
	b.ensureSize(capacity)
	return b
}

// ensureSize is the internal helper that grows the buffer to at least
// minLen using util.Oversize for amortised growth.
func (b *fixedLengthArcsBuffer) ensureSize(minLen int) {
	if minLen <= len(b.bytes) {
		return
	}
	grow := util.Oversize(minLen, 1)
	if grow < minLen {
		grow = minLen
	}
	nb := make([]byte, grow)
	copy(nb, b.bytes)
	b.bytes = nb
}
