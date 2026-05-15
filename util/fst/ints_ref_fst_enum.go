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

	"github.com/FlavioCFOliveira/Gocene/util"
)

// IntsRefFSTEnum.java port — enumerator over the (IntsRef, output)
// pairs of an FST whose input type may be BYTE1, BYTE2 or BYTE4.
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/util/fst/IntsRefFSTEnum.java
//
// The Go port keeps the offset/length semantics of the Lucene
// reference: the current IntsRef is exposed with Offset==1 and
// Length==upto-1, so callers can read it via current.Ints[1:upto]
// (or by going through *IntsRef helpers).
//
// Java IntsRefFSTEnum extends FSTEnum<T> and reuses the protected
// state (arcs, output, upto, targetLength) of the base class. In Go
// we model the inheritance by composition: IntsRefFSTEnum embeds an
// *fstEnum[T] and implements the fstEnumLabels interface that the
// base uses for label-buffer access — same pattern as
// BytesRefFSTEnum.

// IntsRefInputOutput holds a single (input IntsRef, output T) pair,
// the Go counterpart to Lucene's IntsRefFSTEnum.InputOutput<T> inner
// class. The Input pointer is stable across calls — it always points
// at the enumerator's internal current IntsRef, so callers that need
// a copy should clone it themselves.
//
// Naming rationale: Java disambiguates via nested types
// (BytesRefFSTEnum.InputOutput<T> vs IntsRefFSTEnum.InputOutput<T>).
// Go has no inner classes, so each struct is prefixed with the input
// kind it carries. See also [[BytesRefInputOutput]].
type IntsRefInputOutput[T any] struct {
	Input  *util.IntsRef
	Output T
}

// IntsRefFSTEnum enumerates all (input IntsRef, output T) pairs of
// an FST. Mirrors org.apache.lucene.util.fst.IntsRefFSTEnum.
//
// Unlike BytesRefFSTEnum, this enumerator accepts FSTs of any input
// type — BYTE1, BYTE2 or BYTE4 — because the label buffer is a slice
// of int large enough to hold labels of any width.
//
// The zero value is not usable; construct via NewIntsRefFSTEnum.
type IntsRefFSTEnum[T any] struct {
	enum    *fstEnum[T]
	current *util.IntsRef
	result  *IntsRefInputOutput[T]
	target  *util.IntsRef
}

// NewIntsRefFSTEnum constructs a fresh IntsRefFSTEnum bound to fst.
// Mirrors the IntsRefFSTEnum<T>(FST<T>) constructor. The returned
// enumerator starts positioned "before" the first term; Next or one
// of the Seek methods must be called to advance it.
//
// Any FST input width (BYTE1, BYTE2, BYTE4) is accepted because the
// IntsRef labels are wide enough for all three.
func NewIntsRefFSTEnum[T any](fst *FST[T]) (*IntsRefFSTEnum[T], error) {
	if fst == nil {
		return nil, errors.New("fst.NewIntsRefFSTEnum: fst is nil")
	}
	current := &util.IntsRef{Ints: make([]int, 10), Offset: 1, Length: 0}
	ie := &IntsRefFSTEnum[T]{
		current: current,
		result:  &IntsRefInputOutput[T]{Input: current},
	}
	ie.enum = newFSTEnum(fst, ie)
	return ie, nil
}

// Current returns the (input, output) pair the enumerator is
// positioned at, or nil before the first advance / after EOF.
// Mirrors IntsRefFSTEnum.current().
//
// The pointer returned aliases the enumerator's internal state; the
// Input IntsRef in particular is mutated by the next call. Callers
// who need a stable copy must clone it.
func (ie *IntsRefFSTEnum[T]) Current() *IntsRefInputOutput[T] {
	if ie.enum.upto == 0 {
		return nil
	}
	return ie.result
}

// Next advances the enumerator to the next (input, output) pair in
// ascending input order. Returns (nil, nil) when there are no more
// terms (EOF), mirroring Lucene's null return.
func (ie *IntsRefFSTEnum[T]) Next() (*IntsRefInputOutput[T], error) {
	if err := ie.enum.doNext(); err != nil {
		return nil, err
	}
	return ie.setResult(), nil
}

// SeekCeil seeks to the smallest term >= target. Returns (nil, nil)
// when there is no such term (i.e. every term in the FST is strictly
// less than target). Mirrors IntsRefFSTEnum.seekCeil.
func (ie *IntsRefFSTEnum[T]) SeekCeil(target *util.IntsRef) (*IntsRefInputOutput[T], error) {
	if target == nil {
		return nil, errors.New("fst.IntsRefFSTEnum.SeekCeil: target is nil")
	}
	ie.target = target
	ie.enum.targetLength = target.Length
	if err := ie.enum.doSeekCeil(); err != nil {
		return nil, err
	}
	return ie.setResult(), nil
}

// SeekFloor seeks to the largest term <= target. Returns (nil, nil)
// when there is no such term (i.e. every term in the FST is strictly
// greater than target). Mirrors IntsRefFSTEnum.seekFloor.
func (ie *IntsRefFSTEnum[T]) SeekFloor(target *util.IntsRef) (*IntsRefInputOutput[T], error) {
	if target == nil {
		return nil, errors.New("fst.IntsRefFSTEnum.SeekFloor: target is nil")
	}
	ie.target = target
	ie.enum.targetLength = target.Length
	if err := ie.enum.doSeekFloor(); err != nil {
		return nil, err
	}
	return ie.setResult(), nil
}

// SeekExact seeks to exactly target. Returns (nil, nil) when target
// is not in the FST. Faster than SeekCeil / SeekFloor because it
// short-circuits as soon as the match is not found. Mirrors
// IntsRefFSTEnum.seekExact.
func (ie *IntsRefFSTEnum[T]) SeekExact(target *util.IntsRef) (*IntsRefInputOutput[T], error) {
	if target == nil {
		return nil, errors.New("fst.IntsRefFSTEnum.SeekExact: target is nil")
	}
	ie.target = target
	ie.enum.targetLength = target.Length
	ok, err := ie.enum.doSeekExact()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	if ie.enum.upto != 1+target.Length {
		// The Java reference asserts upto == 1 + target.length on a
		// successful match. Surface a corrupt-state error rather than
		// crashing.
		return nil, errors.New("fst.IntsRefFSTEnum.SeekExact: post-condition upto == 1 + target.length violated")
	}
	return ie.setResult(), nil
}

// GetTargetLabel implements fstEnumLabels — read the label at
// position upto-1 of the target key. Mirrors
// IntsRefFSTEnum.getTargetLabel.
func (ie *IntsRefFSTEnum[T]) GetTargetLabel() int {
	if ie.enum.upto-1 == ie.target.Length {
		return END_LABEL
	}
	return ie.target.Ints[ie.target.Offset+ie.enum.upto-1]
}

// GetCurrentLabel implements fstEnumLabels — read the label at
// position upto of the current key. Mirrors
// IntsRefFSTEnum.getCurrentLabel. current.Offset is fixed at 1, so
// position upto in user-facing terms maps to int index upto in the
// underlying buffer.
func (ie *IntsRefFSTEnum[T]) GetCurrentLabel() int {
	return ie.current.Ints[ie.enum.upto]
}

// SetCurrentLabel implements fstEnumLabels — write label into the
// current key at position upto. Mirrors
// IntsRefFSTEnum.setCurrentLabel.
func (ie *IntsRefFSTEnum[T]) SetCurrentLabel(label int) {
	ie.current.Ints[ie.enum.upto] = label
}

// Grow implements fstEnumLabels — ensure current.Ints can hold at
// least upto+1 labels (accounting for the fixed Offset==1 slot).
// Mirrors IntsRefFSTEnum.grow which calls ArrayUtil.grow(ints,
// upto+1). Lucene's ArrayUtil.grow on int[] passes 4 as bytesPerElement
// (the Java int width); util.IntsRefBuilder.Grow also uses 4 for the
// same reason. We reuse that literal here to keep the oversize curve
// identical to the Java reference and consistent with IntsRefBuilder.
func (ie *IntsRefFSTEnum[T]) Grow() {
	need := ie.enum.upto + 1
	if len(ie.current.Ints) >= need {
		return
	}
	const bytesPerInt = 4
	newCap := util.Oversize(need, bytesPerInt)
	if newCap < need {
		newCap = need
	}
	ni := make([]int, newCap)
	copy(ni, ie.current.Ints)
	ie.current.Ints = ni
}

// setResult finishes the seek/next call: if the enumerator is at EOF
// (upto == 0) it returns nil; otherwise it adjusts current.Length to
// the active prefix and stores the cumulative output. Mirrors
// IntsRefFSTEnum.setResult.
func (ie *IntsRefFSTEnum[T]) setResult() *IntsRefInputOutput[T] {
	if ie.enum.upto == 0 {
		return nil
	}
	ie.current.Length = ie.enum.upto - 1
	ie.result.Output = ie.enum.output[ie.enum.upto]
	return ie.result
}
