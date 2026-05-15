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
	"strings"
)

// Arc represents a single transition out of a node in an FST. It is
// the Go port of the inner class org.apache.lucene.util.fst.FST.Arc.
//
// Arcs are mutated in-place by the read methods on FST so callers can
// pool them and avoid allocations.
type Arc[T any] struct {
	label           int
	output          T
	target          int64
	flags           byte
	nextFinalOutput T
	nextArc         int64
	nodeFlags       byte

	// Fields for arcs belonging to a node with fixed-length arcs;
	// only valid when bytesPerArc != 0 (binary search, direct
	// addressing, or continuous).
	bytesPerArc  int
	posArcsStart int64
	arcIdx       int
	numArcs      int

	// Fields specific to a direct-addressing node.
	// bitTableStart is the absolute position of the bit-table in the
	// underlying BytesReader stream.
	bitTableStart int64
	firstLabel    int
	// presenceIndex is the count of presence bits set strictly before
	// the bit at arcIdx in the bit-table. Cached to avoid re-counting
	// while iterating the arcs of a direct-addressing node.
	presenceIndex int
}

// CopyFrom copies all observable fields from other into the receiver,
// returning the receiver. Mirrors Arc.copyFrom.
func (a *Arc[T]) CopyFrom(other *Arc[T]) *Arc[T] {
	a.label = other.label
	a.target = other.target
	a.flags = other.flags
	a.output = other.output
	a.nextFinalOutput = other.nextFinalOutput
	a.nextArc = other.nextArc
	a.nodeFlags = other.nodeFlags
	a.bytesPerArc = other.bytesPerArc
	a.posArcsStart = other.posArcsStart
	a.arcIdx = other.arcIdx
	a.numArcs = other.numArcs
	a.bitTableStart = other.bitTableStart
	a.firstLabel = other.firstLabel
	a.presenceIndex = other.presenceIndex
	return a
}

// Label returns the input label of this arc.
func (a *Arc[T]) Label() int { return a.label }

// Output returns the output value of this arc.
func (a *Arc[T]) Output() T { return a.output }

// Target returns the address of the target node, or -1/0 for the
// virtual end nodes (FINAL_END_NODE / NON_FINAL_END_NODE).
func (a *Arc[T]) Target() int64 { return a.target }

// Flags returns the raw arc flags byte.
func (a *Arc[T]) Flags() byte { return a.flags }

// NextFinalOutput returns the final output attached to this arc, if
// any.
func (a *Arc[T]) NextFinalOutput() T { return a.nextFinalOutput }

// NextArc returns the address of the next arc in a list of
// variable-length arcs, or the address/ord of the next node if
// label == END_LABEL.
func (a *Arc[T]) NextArc() int64 { return a.nextArc }

// ArcIdx returns the index of this arc within a fixed-length array
// node (only valid when bytesPerArc != 0).
func (a *Arc[T]) ArcIdx() int { return a.arcIdx }

// NodeFlags returns the node-header flags. Compare against
// ARCS_FOR_BINARY_SEARCH / ARCS_FOR_DIRECT_ADDRESSING /
// ARCS_FOR_CONTINUOUS; other values are meaningful only when
// bytesPerArc == 0.
func (a *Arc[T]) NodeFlags() byte { return a.nodeFlags }

// PosArcsStart returns the absolute address at which the arc array
// starts (only valid when bytesPerArc != 0).
func (a *Arc[T]) PosArcsStart() int64 { return a.posArcsStart }

// BytesPerArc is non-zero when this arc belongs to a node with
// fixed-length arcs (binary search, direct addressing, or continuous).
func (a *Arc[T]) BytesPerArc() int { return a.bytesPerArc }

// NumArcs is the number of arcs in a fixed-length-arc node, or for a
// direct-addressing node the label range.
func (a *Arc[T]) NumArcs() int { return a.numArcs }

// FirstLabel returns the first label of a direct-addressing or
// continuous node.
func (a *Arc[T]) FirstLabel() int { return a.firstLabel }

// BitTableStart returns the absolute address of the presence
// bit-table for a direct-addressing node.
func (a *Arc[T]) BitTableStart() int64 { return a.bitTableStart }

// PresenceIndex returns the cached count of presence bits set before
// the bit at arcIdx, for a direct-addressing node.
func (a *Arc[T]) PresenceIndex() int { return a.presenceIndex }

// flag reports whether the given flag bit is set in the arc flags.
func (a *Arc[T]) flag(bit int) bool { return flag(int(a.flags), bit) }

// IsLast reports whether this is the last arc of its source node.
func (a *Arc[T]) IsLast() bool { return a.flag(BIT_LAST_ARC) }

// IsFinal reports whether this arc is final.
func (a *Arc[T]) IsFinal() bool { return a.flag(BIT_FINAL_ARC) }

// String returns a debugging representation matching the
// Java Arc.toString output.
func (a *Arc[T]) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, " target=%d", a.target)
	fmt.Fprintf(&b, " label=0x%x", a.label)
	if a.flag(BIT_FINAL_ARC) {
		b.WriteString(" final")
	}
	if a.flag(BIT_LAST_ARC) {
		b.WriteString(" last")
	}
	if a.flag(BIT_TARGET_NEXT) {
		b.WriteString(" targetNext")
	}
	if a.flag(BIT_STOP_NODE) {
		b.WriteString(" stop")
	}
	if a.flag(BIT_ARC_HAS_OUTPUT) {
		fmt.Fprintf(&b, " output=%v", a.output)
	}
	if a.flag(BIT_ARC_HAS_FINAL_OUTPUT) {
		fmt.Fprintf(&b, " nextFinalOutput=%v", a.nextFinalOutput)
	}
	if a.bytesPerArc != 0 {
		kind := "bs"
		switch a.nodeFlags {
		case ARCS_FOR_DIRECT_ADDRESSING:
			kind = "da"
		case ARCS_FOR_CONTINUOUS:
			kind = "cs"
		}
		fmt.Fprintf(&b, " arcArray(idx=%d of %d)(%s)", a.arcIdx, a.numArcs, kind)
	}
	return b.String()
}

// bitTableIsBitSetArc is the entry point used by FST traversal code
// when querying a single presence bit. It positions the reader at the
// bit-table start, then delegates. Mirrors FST.Arc.BitTable.isBitSet.
func bitTableIsBitSetArc[T any](bitIndex int, arc *Arc[T], in BytesReader) (bool, error) {
	in.SetPosition(arc.bitTableStart)
	return bitTableIsBitSet(bitIndex, in)
}

// bitTableCountBitsArc counts all bits set in the arc's bit-table.
// Mirrors FST.Arc.BitTable.countBits.
func bitTableCountBitsArc[T any](arc *Arc[T], in BytesReader) (int, error) {
	in.SetPosition(arc.bitTableStart)
	return bitTableCountBits(getNumPresenceBytes(arc.numArcs), in)
}

// bitTableCountBitsUpToArc counts bits set strictly before bitIndex.
// Mirrors FST.Arc.BitTable.countBitsUpTo.
func bitTableCountBitsUpToArc[T any](bitIndex int, arc *Arc[T], in BytesReader) (int, error) {
	in.SetPosition(arc.bitTableStart)
	return bitTableCountBitsUpTo(bitIndex, in)
}

// bitTableNextBitSetArc returns the index of the next bit set after
// bitIndex, or -1. Mirrors FST.Arc.BitTable.nextBitSet.
func bitTableNextBitSetArc[T any](bitIndex int, arc *Arc[T], in BytesReader) (int, error) {
	in.SetPosition(arc.bitTableStart)
	return bitTableNextBitSet(bitIndex, getNumPresenceBytes(arc.numArcs), in)
}

// bitTablePreviousBitSetArc returns the index of the previous bit set
// before bitIndex, or -1. Mirrors FST.Arc.BitTable.previousBitSet.
func bitTablePreviousBitSetArc[T any](bitIndex int, arc *Arc[T], in BytesReader) (int, error) {
	in.SetPosition(arc.bitTableStart)
	return bitTablePreviousBitSet(bitIndex, in)
}
