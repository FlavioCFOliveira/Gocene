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
	"io"
	"strconv"
)

// Util.java port — Graphviz DOT export.
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/util/fst/Util.java

// ToDot writes a Graphviz "dot" description of fst to out, mirroring
// {@code Util.toDot(FST<T>, Writer, boolean, boolean)}. When sameRank
// is true the output groups states in layers of breadth-first
// traversal; when labelStates is true states are labelled with their
// byte-stream offsets.
//
// The byte stream emitted here is byte-for-byte identical to
// {@code Util.toDot} for the same FST: same prologue, same ordering of
// transitions (each level is consumed in last-in/first-out order, as
// the Java source uses {@code remove(size-1)}), same escape rules in
// {@code printableLabel}, and the same terminating dummy state -1.
func ToDot[T any](fst *FST[T], out io.Writer, sameRank, labelStates bool) error {
	const expandedNodeColor = "blue"

	bw := newDotWriter(out)

	startArc := fst.GetFirstArc(&Arc[T]{})

	// A queue of transitions to consider for the next level. We use
	// slices because Java's ArrayList.remove(size-1) maps directly to
	// truncating the tail; this preserves the LIFO traversal order.
	var thisLevelQueue []*Arc[T]
	var nextLevelQueue []*Arc[T]
	nextLevelQueue = append(nextLevelQueue, startArc)

	// Per-level same-rank state list (Java uses IntArrayList).
	var sameLevelStates []int64

	// Set of states already emitted.
	seen := make(map[int64]struct{})
	seen[startArc.target] = struct{}{}

	const (
		stateShape      = "circle"
		finalStateShape = "doublecircle"
	)

	// Prologue.
	bw.writeString("digraph FST {\n")
	bw.writeString("  rankdir = LR; splines=true; concentrate=true; ordering=out; ranksep=2.5; \n")

	if !labelStates {
		bw.writeString("  node [shape=circle, width=.2, height=.2, style=filled]\n")
	}

	emitDotState(bw, "initial", "point", "white", "")

	noOutput := fst.outputs.GetNoOutput()
	r := fst.GetBytesReader()

	// Emit the first state (the start arc's target).
	{
		expanded, err := fst.IsExpandedTarget(startArc, r)
		if err != nil {
			return err
		}
		stateColor := ""
		if expanded {
			stateColor = expandedNodeColor
		}
		var (
			isFinal     bool
			finalOutput string
		)
		if startArc.IsFinal() {
			isFinal = true
			// "!= NO_OUTPUT" in Java is by-reference; here we compare
			// via the Outputs algebra. PositiveIntOutputs returns the
			// same value (0) for NO_OUTPUT so any(0)==any(0) holds; for
			// reference-typed outputs the singleton's pointer matches.
			if !isNoOutput[T](fst.outputs, startArc.nextFinalOutput, noOutput) {
				finalOutput = fst.outputs.OutputToString(startArc.nextFinalOutput)
			}
		}
		shape := stateShape
		if isFinal {
			shape = finalStateShape
		}
		emitDotState(bw, strconv.FormatInt(startArc.target, 10), shape, stateColor, finalOutput)
	}

	bw.writeString("  initial -> ")
	bw.writeString(strconv.FormatInt(startArc.target, 10))
	bw.writeString("\n")

	level := 0
	for len(nextLevelQueue) > 0 {
		// Move next-level queue into this-level queue.
		thisLevelQueue = append(thisLevelQueue, nextLevelQueue...)
		nextLevelQueue = nextLevelQueue[:0]

		level++
		bw.writeString("\n  // Transitions and states at level: ")
		bw.writeString(strconv.Itoa(level))
		bw.writeString("\n")

		for len(thisLevelQueue) > 0 {
			// Java pops the tail: ArrayList.remove(size-1). We do the same.
			lastIdx := len(thisLevelQueue) - 1
			arc := thisLevelQueue[lastIdx]
			thisLevelQueue = thisLevelQueue[:lastIdx]

			if !TargetHasArcs(arc) {
				continue
			}

			node := arc.target
			if _, err := fst.ReadFirstRealTargetArc(arc.target, arc, r); err != nil {
				return err
			}

			for {
				// Emit the unseen state and add it to the queue for the next level.
				if arc.target >= 0 {
					if _, ok := seen[arc.target]; !ok {
						expanded, err := fst.IsExpandedTarget(arc, r)
						if err != nil {
							return err
						}
						stateColor := ""
						if expanded {
							stateColor = expandedNodeColor
						}
						finalOutput := ""
						// Java: arc.nextFinalOutput() != null && != NO_OUTPUT.
						// We map "!= null" to true (Go's T has no null
						// concept for value types) and rely on the
						// no-output sentinel comparison.
						if !isNoOutput[T](fst.outputs, arc.nextFinalOutput, noOutput) {
							finalOutput = fst.outputs.OutputToString(arc.nextFinalOutput)
						}
						emitDotState(bw, strconv.FormatInt(arc.target, 10), stateShape, stateColor, finalOutput)
						seen[arc.target] = struct{}{}
						// Add a *copy* of arc so subsequent mutation does
						// not corrupt the queued entry.
						queued := &Arc[T]{}
						queued.CopyFrom(arc)
						nextLevelQueue = append(nextLevelQueue, queued)
						sameLevelStates = append(sameLevelStates, arc.target)
					}
				}

				outs := ""
				if !isNoOutput[T](fst.outputs, arc.output, noOutput) {
					outs = "/" + fst.outputs.OutputToString(arc.output)
				}
				if !TargetHasArcs(arc) && arc.IsFinal() && !isNoOutput[T](fst.outputs, arc.nextFinalOutput, noOutput) {
					outs += "/[" + fst.outputs.OutputToString(arc.nextFinalOutput) + "]"
				}
				arcColor := "black"
				if arc.flag(BIT_TARGET_NEXT) {
					arcColor = "red"
				}

				if arc.label == END_LABEL {
					return fmt.Errorf("fst.ToDot: unexpected END_LABEL arc in adjacency list")
				}

				bw.writeString("  ")
				bw.writeString(strconv.FormatInt(node, 10))
				bw.writeString(" -> ")
				bw.writeString(strconv.FormatInt(arc.target, 10))
				bw.writeString(" [label=\"")
				bw.writeString(printableLabel(arc.label))
				bw.writeString(outs)
				bw.writeString("\"")
				if arc.IsFinal() {
					bw.writeString(" style=\"bold\"")
				}
				bw.writeString(" color=\"")
				bw.writeString(arcColor)
				bw.writeString("\"]\n")

				if arc.IsLast() {
					break
				}
				if _, err := fst.ReadNextRealArc(arc, r); err != nil {
					return err
				}
			}
		}

		if sameRank && len(sameLevelStates) > 1 {
			bw.writeString("  {rank=same; ")
			for _, s := range sameLevelStates {
				bw.writeString(strconv.FormatInt(s, 10))
				bw.writeString("; ")
			}
			bw.writeString(" }\n")
		}
		sameLevelStates = sameLevelStates[:0]
	}

	bw.writeString("  -1 [style=filled, color=black, shape=doublecircle, label=\"\"]\n\n")
	bw.writeString("  {rank=sink; -1 }\n")
	bw.writeString("}\n")
	return bw.err
}

// dotWriter is a thin wrapper around io.Writer that captures the first
// write error. Mirrors the implicit fall-through of Java's
// {@code Writer.write} which throws on the first failure.
type dotWriter struct {
	w   io.Writer
	err error
}

func newDotWriter(w io.Writer) *dotWriter { return &dotWriter{w: w} }

func (d *dotWriter) writeString(s string) {
	if d.err != nil {
		return
	}
	_, d.err = io.WriteString(d.w, s)
}

// emitDotState emits a single state line. Mirrors
// {@code Util.emitDotState}.
//
// Java passes the shape/color/label as optional fragments: when a piece
// is null Java still emits the trailing single space that separates
// fragments, and emits "label=\"\"" when the label is null. We mirror
// that verbatim.
func emitDotState(d *dotWriter, name, shape, color, label string) {
	d.writeString("  ")
	d.writeString(name)
	d.writeString(" [")
	if shape != "" {
		d.writeString("shape=")
		d.writeString(shape)
	}
	d.writeString(" ")
	if color != "" {
		d.writeString("color=")
		d.writeString(color)
	}
	d.writeString(" ")
	d.writeString("label=\"")
	d.writeString(label)
	d.writeString("\"")
	d.writeString(" ")
	d.writeString("]\n")
}

// printableLabel renders an arc label as a single character when it is
// printable US-ASCII (excluding '"' and '\\'), otherwise as a "0xNN..."
// hexadecimal sequence. Mirrors {@code Util.printableLabel}.
func printableLabel(label int) string {
	if label >= 0x20 && label <= 0x7d && label != 0x22 && label != 0x5c {
		return string(rune(label))
	}
	// Java's Integer.toHexString is lower-case and unpadded.
	return "0x" + strconv.FormatInt(int64(uint32(int32(label))), 16)
}
