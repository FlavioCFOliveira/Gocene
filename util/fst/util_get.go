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

	"github.com/FlavioCFOliveira/Gocene/util"
)

// Util.java port — single-input lookup helpers and the readCeilArc /
// binarySearch primitives.
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/util/fst/Util.java

// Get looks up the output for input in fst. The second return value is
// true iff the FST accepts input; when false the T zero value is
// returned. Mirrors {@code Util.get(FST<T>, IntsRef)}.
//
// The Java reference returns the output (or null when the input is not
// accepted); Go does not have a typed-null in generic position, so we
// return (T, bool, error). Errors come exclusively from the BytesReader
// or the Outputs algebra (e.g. truncated stream); the value/found
// channel mirrors Java's value/null channel.
func Get[T any](fst *FST[T], input *util.IntsRef) (T, bool, error) {
	var zero T
	if fst == nil {
		return zero, false, errors.New("fst.Get: fst is nil")
	}
	if input == nil {
		return zero, false, errors.New("fst.Get: input is nil")
	}
	arc := fst.GetFirstArc(&Arc[T]{})
	reader := fst.GetBytesReader()
	output := fst.outputs.GetNoOutput()
	end := input.Offset + input.Length
	for i := input.Offset; i < end; i++ {
		found, err := fst.FindTargetArc(input.Ints[i], arc, arc, reader)
		if err != nil {
			return zero, false, err
		}
		if found == nil {
			return zero, false, nil
		}
		output = fst.outputs.Add(output, arc.Output())
	}
	if !arc.IsFinal() {
		return zero, false, nil
	}
	return fst.outputs.Add(output, arc.NextFinalOutput()), true, nil
}

// GetBytesRef looks up the output for input in an FST whose input type
// is BYTE1. Mirrors {@code Util.get(FST<T>, BytesRef)}.
//
// The Java reference asserts inputType == BYTE1; we return an error
// instead of panicking when the FST was not built with BYTE1 labels.
func GetBytesRef[T any](fst *FST[T], input *util.BytesRef) (T, bool, error) {
	var zero T
	if fst == nil {
		return zero, false, errors.New("fst.GetBytesRef: fst is nil")
	}
	if input == nil {
		return zero, false, errors.New("fst.GetBytesRef: input is nil")
	}
	if fst.metadata.inputType != InputTypeByte1 {
		return zero, false, fmt.Errorf("fst.GetBytesRef: FST input type must be BYTE1, got %s", fst.metadata.inputType)
	}
	reader := fst.GetBytesReader()
	arc := fst.GetFirstArc(&Arc[T]{})
	output := fst.outputs.GetNoOutput()
	end := input.Offset + input.Length
	for i := input.Offset; i < end; i++ {
		label := int(input.Bytes[i]) & 0xFF
		found, err := fst.FindTargetArc(label, arc, arc, reader)
		if err != nil {
			return zero, false, err
		}
		if found == nil {
			return zero, false, nil
		}
		output = fst.outputs.Add(output, arc.Output())
	}
	if !arc.IsFinal() {
		return zero, false, nil
	}
	return fst.outputs.Add(output, arc.NextFinalOutput()), true, nil
}

// BinarySearch performs a binary search inside a packed
// fixed-length-arc array node. The caller must have positioned arc on
// the first arc of the array (typically via ReadFirstTargetArc). The
// return value is the index of the matching arc, or
// {@code -1 - insertionPoint} when no match is found (mirroring
// Lucene's contract).
//
// Mirrors the package-private static method
// {@code Util.binarySearch(FST<T>, FST.Arc<T>, int)}.
func BinarySearch[T any](fst *FST[T], arc *Arc[T], targetLabel int) (int, error) {
	if arc.nodeFlags != ARCS_FOR_BINARY_SEARCH {
		return 0, fmt.Errorf(
			"fst.BinarySearch: Arc is not encoded as packed array for binary search (nodeFlags=%d)",
			arc.nodeFlags,
		)
	}
	in := fst.GetBytesReader()
	low := arc.arcIdx
	high := arc.numArcs - 1
	for low <= high {
		// Equivalent to (low + high) >>> 1 in Java; cast through uint
		// to avoid signed overflow when low+high is large.
		mid := int(uint(low+high) >> 1)
		in.SetPosition(arc.posArcsStart)
		// Skip mid arcs plus the flags byte to position at the label.
		if err := in.SkipBytes(int64(arc.bytesPerArc)*int64(mid) + 1); err != nil {
			return 0, err
		}
		midLabel, err := fst.ReadLabel(in)
		if err != nil {
			return 0, err
		}
		cmp := midLabel - targetLabel
		switch {
		case cmp < 0:
			low = mid + 1
		case cmp > 0:
			high = mid - 1
		default:
			return mid, nil
		}
	}
	return -1 - low, nil
}

// ReadCeilArc reads the first arc whose label is greater than or equal
// to label into arc, in place, and returns arc. Returns nil when no
// such arc exists (i.e. all sibling labels are strictly less than
// label or the follow target has no arcs at all).
//
// Mirrors {@code Util.readCeilArc(int, FST<T>, FST.Arc<T>, FST.Arc<T>, BytesReader)}.
func ReadCeilArc[T any](label int, fst *FST[T], follow, arc *Arc[T], in BytesReader) (*Arc[T], error) {
	if label == END_LABEL {
		return ReadEndArc(follow, arc), nil
	}
	if !TargetHasArcs(follow) {
		return nil, nil
	}
	if _, err := fst.ReadFirstTargetArc(follow, arc, in); err != nil {
		return nil, err
	}
	if arc.bytesPerArc != 0 && arc.label != END_LABEL {
		switch arc.nodeFlags {
		case ARCS_FOR_DIRECT_ADDRESSING:
			// Fixed-length arcs, direct-addressing node.
			targetIndex := label - arc.label
			if targetIndex >= arc.numArcs {
				return nil, nil
			}
			if targetIndex < 0 {
				return arc, nil
			}
			set, err := bitTableIsBitSetArc(targetIndex, arc, in)
			if err != nil {
				return nil, err
			}
			if set {
				return fst.ReadArcByDirectAddressing(arc, in, targetIndex)
			}
			ceilIndex, err := bitTableNextBitSetArc(targetIndex, arc, in)
			if err != nil {
				return nil, err
			}
			if ceilIndex == -1 {
				return nil, errors.New("fst.ReadCeilArc: direct-addressing node has no ceiling bit")
			}
			return fst.ReadArcByDirectAddressing(arc, in, ceilIndex)
		case ARCS_FOR_CONTINUOUS:
			targetIndex := label - arc.label
			if targetIndex >= arc.numArcs {
				return nil, nil
			}
			if targetIndex < 0 {
				return arc, nil
			}
			return fst.ReadArcByContinuous(arc, in, targetIndex)
		}
		// Fixed-length arcs, binary-search node.
		idx, err := BinarySearch(fst, arc, label)
		if err != nil {
			return nil, err
		}
		if idx >= 0 {
			return fst.ReadArcByIndex(arc, in, idx)
		}
		idx = -1 - idx
		if idx == arc.numArcs {
			// Dead end: all labels are strictly less than the target.
			return nil, nil
		}
		return fst.ReadArcByIndex(arc, in, idx)
	}
	// Variable-length arcs (linear scan) or the special END_LABEL arc.
	if _, err := fst.ReadFirstRealTargetArc(follow.target, arc, in); err != nil {
		return nil, err
	}
	for {
		if arc.label >= label {
			return arc, nil
		}
		if arc.IsLast() {
			return nil, nil
		}
		if _, err := fst.ReadNextRealArc(arc, in); err != nil {
			return nil, err
		}
	}
}
