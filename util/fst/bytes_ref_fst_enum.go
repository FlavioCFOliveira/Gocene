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

// BytesRefFSTEnum.java port — enumerator over the (BytesRef, output)
// pairs of an FST whose input type is BYTE1.
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/util/fst/BytesRefFSTEnum.java
//
// The Go port keeps the offset/length semantics of the Lucene reference:
// the current BytesRef is exposed with Offset==1 and Length==upto-1, so
// callers can read it via current.ValidBytes() or by indexing
// Bytes[1:upto].
//
// Java BytesRefFSTEnum extends FSTEnum<T> and reuses the protected
// state (arcs, output, upto, targetLength) of the base class. In Go we
// model the inheritance by composition: BytesRefFSTEnum embeds an
// *fstEnum[T] and implements the fstEnumLabels interface that the base
// uses for label-buffer access.

// InputOutput holds a single (input BytesRef, output T) pair, the Go
// counterpart to Lucene's BytesRefFSTEnum.InputOutput<T> inner class.
// The Input pointer is stable across calls — it always points at the
// enumerator's internal current BytesRef, so callers that need a copy
// should clone it themselves.
type InputOutput[T any] struct {
	Input  *util.BytesRef
	Output T
}

// BytesRefFSTEnum enumerates all (input BytesRef, output T) pairs of
// an FST. Mirrors org.apache.lucene.util.fst.BytesRefFSTEnum.
//
// The zero value is not usable; construct via NewBytesRefFSTEnum.
type BytesRefFSTEnum[T any] struct {
	enum    *fstEnum[T]
	current *util.BytesRef
	result  *InputOutput[T]
	target  *util.BytesRef
}

// NewBytesRefFSTEnum constructs a fresh BytesRefFSTEnum bound to fst.
// Mirrors the BytesRefFSTEnum<T>(FST<T>) constructor. The returned
// enumerator starts positioned "before" the first term; Next or one
// of the Seek methods must be called to advance it.
//
// The FST's input type must be BYTE1; passing an FST built with a
// different input type returns an error so the caller does not get
// silently incorrect behaviour at first call.
func NewBytesRefFSTEnum[T any](fst *FST[T]) (*BytesRefFSTEnum[T], error) {
	if fst == nil {
		return nil, errors.New("fst.NewBytesRefFSTEnum: fst is nil")
	}
	if fst.metadata.inputType != InputTypeByte1 {
		return nil, errors.New("fst.NewBytesRefFSTEnum: FST input type must be BYTE1")
	}
	current := &util.BytesRef{Bytes: make([]byte, 10), Offset: 1, Length: 0}
	be := &BytesRefFSTEnum[T]{
		current: current,
		result:  &InputOutput[T]{Input: current},
	}
	be.enum = newFSTEnum(fst, be)
	return be, nil
}

// Current returns the (input, output) pair the enumerator is
// positioned at, or nil before the first advance / after EOF.
// Mirrors BytesRefFSTEnum.current().
//
// The pointer returned aliases the enumerator's internal state; the
// Input BytesRef in particular is mutated by the next call. Callers
// who need a stable copy must clone it.
func (be *BytesRefFSTEnum[T]) Current() *InputOutput[T] {
	if be.enum.upto == 0 {
		return nil
	}
	return be.result
}

// Next advances the enumerator to the next (input, output) pair in
// ascending input order. Returns (nil, nil) when there are no more
// terms (EOF), mirroring Lucene's null return.
func (be *BytesRefFSTEnum[T]) Next() (*InputOutput[T], error) {
	if err := be.enum.doNext(); err != nil {
		return nil, err
	}
	return be.setResult(), nil
}

// SeekCeil seeks to the smallest term >= target. Returns (nil, nil)
// when there is no such term (i.e. every term in the FST is strictly
// less than target). Mirrors BytesRefFSTEnum.seekCeil.
func (be *BytesRefFSTEnum[T]) SeekCeil(target *util.BytesRef) (*InputOutput[T], error) {
	if target == nil {
		return nil, errors.New("fst.BytesRefFSTEnum.SeekCeil: target is nil")
	}
	be.target = target
	be.enum.targetLength = target.Length
	if err := be.enum.doSeekCeil(); err != nil {
		return nil, err
	}
	return be.setResult(), nil
}

// SeekFloor seeks to the largest term <= target. Returns (nil, nil)
// when there is no such term (i.e. every term in the FST is strictly
// greater than target). Mirrors BytesRefFSTEnum.seekFloor.
func (be *BytesRefFSTEnum[T]) SeekFloor(target *util.BytesRef) (*InputOutput[T], error) {
	if target == nil {
		return nil, errors.New("fst.BytesRefFSTEnum.SeekFloor: target is nil")
	}
	be.target = target
	be.enum.targetLength = target.Length
	if err := be.enum.doSeekFloor(); err != nil {
		return nil, err
	}
	return be.setResult(), nil
}

// SeekExact seeks to exactly target. Returns (nil, nil) when target
// is not in the FST. Faster than SeekCeil / SeekFloor because it
// short-circuits as soon as the match is not found. Mirrors
// BytesRefFSTEnum.seekExact.
func (be *BytesRefFSTEnum[T]) SeekExact(target *util.BytesRef) (*InputOutput[T], error) {
	if target == nil {
		return nil, errors.New("fst.BytesRefFSTEnum.SeekExact: target is nil")
	}
	be.target = target
	be.enum.targetLength = target.Length
	ok, err := be.enum.doSeekExact()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	if be.enum.upto != 1+target.Length {
		// The Java reference asserts upto == 1 + target.length on a
		// successful match. Surface a corrupt-state error rather than
		// crashing.
		return nil, errors.New("fst.BytesRefFSTEnum.SeekExact: post-condition upto == 1 + target.length violated")
	}
	return be.setResult(), nil
}

// GetTargetLabel implements fstEnumLabels — read the label at
// position upto-1 of the target key. Mirrors
// BytesRefFSTEnum.getTargetLabel.
func (be *BytesRefFSTEnum[T]) GetTargetLabel() int {
	if be.enum.upto-1 == be.target.Length {
		return END_LABEL
	}
	return int(be.target.Bytes[be.target.Offset+be.enum.upto-1]) & 0xFF
}

// GetCurrentLabel implements fstEnumLabels — read the label at
// position upto of the current key. Mirrors
// BytesRefFSTEnum.getCurrentLabel. current.Offset is fixed at 1, so
// position upto in user-facing terms maps to byte index upto in the
// underlying buffer.
func (be *BytesRefFSTEnum[T]) GetCurrentLabel() int {
	return int(be.current.Bytes[be.enum.upto]) & 0xFF
}

// SetCurrentLabel implements fstEnumLabels — write label into the
// current key at position upto. Mirrors
// BytesRefFSTEnum.setCurrentLabel.
func (be *BytesRefFSTEnum[T]) SetCurrentLabel(label int) {
	be.current.Bytes[be.enum.upto] = byte(label)
}

// Grow implements fstEnumLabels — ensure current.Bytes can hold at
// least upto+1 labels (accounting for the fixed Offset==1 slot).
// Mirrors BytesRefFSTEnum.grow which calls ArrayUtil.grow(bytes,
// upto+1).
func (be *BytesRefFSTEnum[T]) Grow() {
	need := be.enum.upto + 1
	if len(be.current.Bytes) >= need {
		return
	}
	newCap := util.Oversize(need, 1)
	if newCap < need {
		newCap = need
	}
	nb := make([]byte, newCap)
	copy(nb, be.current.Bytes)
	be.current.Bytes = nb
}

// setResult finishes the seek/next call: if the enumerator is at EOF
// (upto == 0) it returns nil; otherwise it adjusts current.Length to
// the active prefix and stores the cumulative output. Mirrors
// BytesRefFSTEnum.setResult.
func (be *BytesRefFSTEnum[T]) setResult() *InputOutput[T] {
	if be.enum.upto == 0 {
		return nil
	}
	be.current.Length = be.enum.upto - 1
	be.result.Output = be.enum.output[be.enum.upto]
	return be.result
}
