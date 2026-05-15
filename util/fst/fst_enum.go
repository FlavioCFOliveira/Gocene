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

// FSTEnum.java port — common machinery shared by every typed enumerator
// over an FST (BytesRefFSTEnum, IntsRefFSTEnum, etc.).
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/util/fst/FSTEnum.java
//
// The Java class is abstract and exposes four template-method hooks to
// its subclasses: getTargetLabel, getCurrentLabel, setCurrentLabel,
// grow. In Go we cannot rely on inheritance, so the hooks are modelled
// as a small package-private interface (fstEnumLabels). The concrete
// enumerators (BytesRefFSTEnum below; an eventual IntsRefFSTEnum)
// embed *fstEnum[T] and implement fstEnumLabels themselves; they then
// call back through enum.labels in a single virtual dispatch per hook,
// which is functionally equivalent to Java's polymorphic call.

// fstEnumLabels is the package-private interface that concrete
// FSTEnum subclasses implement, mirroring the four Java abstract
// methods of FSTEnum<T>:
//
//   - GetTargetLabel: read the label at position upto-1 of the target
//     key, or END_LABEL when upto-1 == target length.
//   - GetCurrentLabel: read the label at position upto of the current
//     enumerator state.
//   - SetCurrentLabel: write label into the current enumerator state
//     at position upto.
//   - Grow: ensure the current-state buffer can hold at least upto+1
//     labels.
type fstEnumLabels interface {
	GetTargetLabel() int
	GetCurrentLabel() int
	SetCurrentLabel(label int)
	Grow()
}

// fstEnum is the Go port of Lucene's abstract FSTEnum<T>. It holds the
// shared seek/next state and delegates the four polymorphic label
// hooks to a labels value supplied at construction time. Concrete
// enumerators (BytesRefFSTEnum, IntsRefFSTEnum) embed *fstEnum[T] and
// pass themselves as the labels argument.
//
// fstEnum is package-private; callers should use the typed
// enumerators.
type fstEnum[T any] struct {
	fst       *FST[T]
	labels    fstEnumLabels
	fstReader BytesReader

	// arcs holds, at each level i, the arc that produced the current
	// label at position i-1 of the enumerator state. arcs[0] is the
	// virtual start arc (filled in by GetFirstArc).
	arcs []*Arc[T]

	// output holds the cumulative output at each level. output[i] is
	// the sum of the outputs of arcs[1..i] plus NO_OUTPUT.
	output []T

	noOutput T

	// upto is the current depth of the enumerator (number of labels
	// consumed). upto == 0 means "before any label" / EOF.
	upto int

	// targetLength is set by the typed seekX wrappers to the length of
	// the target key.
	targetLength int
}

// newFSTEnum constructs a fresh fstEnum bound to fst and labels.
// Mirrors the Java FSTEnum<T>(FST<T>) constructor: it allocates the
// initial arcs/output buffers, primes arcs[0] with the virtual start
// arc, and sets output[0] to NO_OUTPUT.
func newFSTEnum[T any](fst *FST[T], labels fstEnumLabels) *fstEnum[T] {
	e := &fstEnum[T]{
		fst:       fst,
		labels:    labels,
		fstReader: fst.GetBytesReader(),
		arcs:      make([]*Arc[T], 10),
		output:    make([]T, 10),
		noOutput:  fst.Outputs().GetNoOutput(),
	}
	fst.GetFirstArc(e.getArc(0))
	e.output[0] = e.noOutput
	return e
}

// getArc lazily allocates the i-th arc, mirroring the private
// FSTEnum.getArc helper.
func (e *fstEnum[T]) getArc(idx int) *Arc[T] {
	if e.arcs[idx] == nil {
		e.arcs[idx] = &Arc[T]{}
	}
	return e.arcs[idx]
}

// rewindPrefix rewinds enum state to match the shared prefix between
// current term and target term. Mirrors FSTEnum.rewindPrefix.
func (e *fstEnum[T]) rewindPrefix() error {
	if e.upto == 0 {
		e.upto = 1
		if _, err := e.fst.ReadFirstTargetArc(e.getArc(0), e.getArc(1), e.fstReader); err != nil {
			return err
		}
		return nil
	}
	currentLimit := e.upto
	e.upto = 1
	for e.upto < currentLimit && e.upto <= e.targetLength+1 {
		cmp := e.labels.GetCurrentLabel() - e.labels.GetTargetLabel()
		if cmp < 0 {
			// seek forward — current is already past the shared prefix.
			break
		} else if cmp > 0 {
			// seek backwards — reset this arc to the first arc of the
			// previous node so the seek scans from the beginning.
			arc := e.getArc(e.upto)
			if _, err := e.fst.ReadFirstTargetArc(e.getArc(e.upto-1), arc, e.fstReader); err != nil {
				return err
			}
			break
		}
		e.upto++
	}
	return nil
}

// doNext advances the enumerator to the next term in input order.
// Mirrors FSTEnum.doNext.
func (e *fstEnum[T]) doNext() error {
	if e.upto == 0 {
		e.upto = 1
		if _, err := e.fst.ReadFirstTargetArc(e.getArc(0), e.getArc(1), e.fstReader); err != nil {
			return err
		}
	} else {
		// Pop until we find an arc that has a successor, or we exhaust
		// the stack (EOF).
		for e.arcs[e.upto].IsLast() {
			e.upto--
			if e.upto == 0 {
				return nil
			}
		}
		if _, err := e.fst.ReadNextArc(e.arcs[e.upto], e.fstReader); err != nil {
			return err
		}
	}
	return e.pushFirst()
}

// doSeekCeil seeks to the smallest term >= target. Mirrors
// FSTEnum.doSeekCeil.
func (e *fstEnum[T]) doSeekCeil() error {
	// Save time by starting at the end of the shared prefix between
	// our current term and the target.
	if err := e.rewindPrefix(); err != nil {
		return err
	}
	arc := e.getArc(e.upto)
	var err error
	for arc != nil {
		targetLabel := e.labels.GetTargetLabel()
		if arc.bytesPerArc != 0 && arc.label != END_LABEL {
			in := e.fst.GetBytesReader()
			switch arc.nodeFlags {
			case ARCS_FOR_DIRECT_ADDRESSING:
				arc, err = e.doSeekCeilArrayDirectAddressing(arc, targetLabel, in)
			case ARCS_FOR_BINARY_SEARCH:
				arc, err = e.doSeekCeilArrayPacked(arc, targetLabel, in)
			case ARCS_FOR_CONTINUOUS:
				arc, err = e.doSeekCeilArrayContinuous(arc, targetLabel, in)
			default:
				return errors.New("fst.FSTEnum: doSeekCeil: unexpected nodeFlags for fixed-length node")
			}
		} else {
			arc, err = e.doSeekCeilList(arc, targetLabel)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// doSeekCeilArrayContinuous handles the ceil seek inside a
// continuous-node arc array. Mirrors
// FSTEnum.doSeekCeilArrayContinuous.
func (e *fstEnum[T]) doSeekCeilArrayContinuous(arc *Arc[T], targetLabel int, in BytesReader) (*Arc[T], error) {
	targetIndex := targetLabel - arc.firstLabel
	if targetIndex >= arc.numArcs {
		return nil, e.rollbackToLastForkThenPush()
	}
	if targetIndex < 0 {
		if _, err := e.fst.ReadArcByContinuous(arc, in, 0); err != nil {
			return nil, err
		}
		return nil, e.pushFirst()
	}
	if _, err := e.fst.ReadArcByContinuous(arc, in, targetIndex); err != nil {
		return nil, err
	}
	// Found — recurse from the matched arc.
	e.output[e.upto] = e.fst.Outputs().Add(e.output[e.upto-1], arc.output)
	if targetLabel == END_LABEL {
		return nil, nil
	}
	e.labels.SetCurrentLabel(arc.label)
	if err := e.incr(); err != nil {
		return nil, err
	}
	return e.fst.ReadFirstTargetArc(arc, e.getArc(e.upto), e.fstReader)
}

// doSeekCeilArrayDirectAddressing handles the ceil seek inside a
// direct-addressing arc array. Mirrors
// FSTEnum.doSeekCeilArrayDirectAddressing.
func (e *fstEnum[T]) doSeekCeilArrayDirectAddressing(arc *Arc[T], targetLabel int, in BytesReader) (*Arc[T], error) {
	targetIndex := targetLabel - arc.firstLabel
	if targetIndex >= arc.numArcs {
		return nil, e.rollbackToLastForkThenPush()
	}
	if targetIndex < 0 {
		targetIndex = -1
	} else {
		set, err := bitTableIsBitSetArc(targetIndex, arc, in)
		if err != nil {
			return nil, err
		}
		if set {
			if _, err := e.fst.ReadArcByDirectAddressing(arc, in, targetIndex); err != nil {
				return nil, err
			}
			// Found — recurse from the matched arc.
			e.output[e.upto] = e.fst.Outputs().Add(e.output[e.upto-1], arc.output)
			if targetLabel == END_LABEL {
				return nil, nil
			}
			e.labels.SetCurrentLabel(arc.label)
			if err := e.incr(); err != nil {
				return nil, err
			}
			return e.fst.ReadFirstTargetArc(arc, e.getArc(e.upto), e.fstReader)
		}
	}
	// Not found — read the next present arc (ceil).
	ceilIndex, err := bitTableNextBitSetArc(targetIndex, arc, in)
	if err != nil {
		return nil, err
	}
	if ceilIndex == -1 {
		return nil, errors.New("fst.FSTEnum: direct-addressing node has no ceil bit")
	}
	if _, err := e.fst.ReadArcByDirectAddressing(arc, in, ceilIndex); err != nil {
		return nil, err
	}
	return nil, e.pushFirst()
}

// doSeekCeilArrayPacked handles the ceil seek inside a
// binary-search-packed arc array. Mirrors FSTEnum.doSeekCeilArrayPacked.
func (e *fstEnum[T]) doSeekCeilArrayPacked(arc *Arc[T], targetLabel int, in BytesReader) (*Arc[T], error) {
	idx, err := BinarySearch(e.fst, arc, targetLabel)
	if err != nil {
		return nil, err
	}
	if idx >= 0 {
		// Match — recurse.
		if _, err := e.fst.ReadArcByIndex(arc, in, idx); err != nil {
			return nil, err
		}
		e.output[e.upto] = e.fst.Outputs().Add(e.output[e.upto-1], arc.output)
		if targetLabel == END_LABEL {
			return nil, nil
		}
		e.labels.SetCurrentLabel(arc.label)
		if err := e.incr(); err != nil {
			return nil, err
		}
		return e.fst.ReadFirstTargetArc(arc, e.getArc(e.upto), e.fstReader)
	}
	idx = -1 - idx
	if idx == arc.numArcs {
		// Dead end (target is after the last arc); rollback to last
		// fork then push.
		if _, err := e.fst.ReadArcByIndex(arc, in, idx-1); err != nil {
			return nil, err
		}
		e.upto--
		for {
			if e.upto == 0 {
				return nil, nil
			}
			prevArc := e.getArc(e.upto)
			if !prevArc.IsLast() {
				if _, err := e.fst.ReadNextArc(prevArc, e.fstReader); err != nil {
					return nil, err
				}
				return nil, e.pushFirst()
			}
			e.upto--
		}
	}
	// Ceil — arc with the least label strictly greater than targetLabel.
	if _, err := e.fst.ReadArcByIndex(arc, in, idx); err != nil {
		return nil, err
	}
	return nil, e.pushFirst()
}

// doSeekCeilList handles the ceil seek inside a variable-length
// (linear) arc list. Mirrors FSTEnum.doSeekCeilList.
func (e *fstEnum[T]) doSeekCeilList(arc *Arc[T], targetLabel int) (*Arc[T], error) {
	if arc.label == targetLabel {
		// Recurse.
		e.output[e.upto] = e.fst.Outputs().Add(e.output[e.upto-1], arc.output)
		if targetLabel == END_LABEL {
			return nil, nil
		}
		e.labels.SetCurrentLabel(arc.label)
		if err := e.incr(); err != nil {
			return nil, err
		}
		return e.fst.ReadFirstTargetArc(arc, e.getArc(e.upto), e.fstReader)
	}
	if arc.label > targetLabel {
		return nil, e.pushFirst()
	}
	if arc.IsLast() {
		// Dead end — rollback to last fork then push.
		e.upto--
		for {
			if e.upto == 0 {
				return nil, nil
			}
			prevArc := e.getArc(e.upto)
			if !prevArc.IsLast() {
				if _, err := e.fst.ReadNextArc(prevArc, e.fstReader); err != nil {
					return nil, err
				}
				return nil, e.pushFirst()
			}
			e.upto--
		}
	}
	// Keep scanning.
	if _, err := e.fst.ReadNextArc(arc, e.fstReader); err != nil {
		return nil, err
	}
	return arc, nil
}

// doSeekFloor seeks to the largest term <= target. Mirrors
// FSTEnum.doSeekFloor.
func (e *fstEnum[T]) doSeekFloor() error {
	if err := e.rewindPrefix(); err != nil {
		return err
	}
	arc := e.getArc(e.upto)
	var err error
	for arc != nil {
		targetLabel := e.labels.GetTargetLabel()
		if arc.bytesPerArc != 0 && arc.label != END_LABEL {
			in := e.fst.GetBytesReader()
			switch arc.nodeFlags {
			case ARCS_FOR_DIRECT_ADDRESSING:
				arc, err = e.doSeekFloorArrayDirectAddressing(arc, targetLabel, in)
			case ARCS_FOR_BINARY_SEARCH:
				arc, err = e.doSeekFloorArrayPacked(arc, targetLabel, in)
			case ARCS_FOR_CONTINUOUS:
				arc, err = e.doSeekFloorContinuous(arc, targetLabel, in)
			default:
				return errors.New("fst.FSTEnum: doSeekFloor: unexpected nodeFlags for fixed-length node")
			}
		} else {
			arc, err = e.doSeekFloorList(arc, targetLabel)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// doSeekFloorContinuous handles the floor seek inside a
// continuous-node arc array. Mirrors FSTEnum.doSeekFloorContinuous.
func (e *fstEnum[T]) doSeekFloorContinuous(arc *Arc[T], targetLabel int, in BytesReader) (*Arc[T], error) {
	targetIndex := targetLabel - arc.firstLabel
	if targetIndex < 0 {
		return e.backtrackToFloorArc(arc, targetLabel, in)
	}
	if targetIndex >= arc.numArcs {
		// After last arc.
		if _, err := e.fst.ReadLastArcByContinuous(arc, in); err != nil {
			return nil, err
		}
		return nil, e.pushLast()
	}
	if _, err := e.fst.ReadArcByContinuous(arc, in, targetIndex); err != nil {
		return nil, err
	}
	// Found — recurse from the matched arc.
	e.output[e.upto] = e.fst.Outputs().Add(e.output[e.upto-1], arc.output)
	if targetLabel == END_LABEL {
		return nil, nil
	}
	e.labels.SetCurrentLabel(arc.label)
	if err := e.incr(); err != nil {
		return nil, err
	}
	return e.fst.ReadFirstTargetArc(arc, e.getArc(e.upto), e.fstReader)
}

// doSeekFloorArrayDirectAddressing handles the floor seek inside a
// direct-addressing arc array. Mirrors
// FSTEnum.doSeekFloorArrayDirectAddressing.
func (e *fstEnum[T]) doSeekFloorArrayDirectAddressing(arc *Arc[T], targetLabel int, in BytesReader) (*Arc[T], error) {
	targetIndex := targetLabel - arc.firstLabel
	if targetIndex < 0 {
		return e.backtrackToFloorArc(arc, targetLabel, in)
	}
	if targetIndex >= arc.numArcs {
		// After last arc.
		if _, err := e.fst.ReadLastArcByDirectAddressing(arc, in); err != nil {
			return nil, err
		}
		return nil, e.pushLast()
	}
	set, err := bitTableIsBitSetArc(targetIndex, arc, in)
	if err != nil {
		return nil, err
	}
	if set {
		if _, err := e.fst.ReadArcByDirectAddressing(arc, in, targetIndex); err != nil {
			return nil, err
		}
		// Found — recurse from the matched arc.
		e.output[e.upto] = e.fst.Outputs().Add(e.output[e.upto-1], arc.output)
		if targetLabel == END_LABEL {
			return nil, nil
		}
		e.labels.SetCurrentLabel(arc.label)
		if err := e.incr(); err != nil {
			return nil, err
		}
		return e.fst.ReadFirstTargetArc(arc, e.getArc(e.upto), e.fstReader)
	}
	// Scan backwards to find a floor arc.
	floorIndex, err := bitTablePreviousBitSetArc(targetIndex, arc, in)
	if err != nil {
		return nil, err
	}
	if floorIndex == -1 {
		return nil, errors.New("fst.FSTEnum: direct-addressing node has no floor bit")
	}
	if _, err := e.fst.ReadArcByDirectAddressing(arc, in, floorIndex); err != nil {
		return nil, err
	}
	return nil, e.pushLast()
}

// rollbackToLastForkThenPush implements the rollback step shared by
// the ceil paths. Target is beyond the last arc; rewind until we find
// an unread sibling, then pushFirst. Mirrors
// FSTEnum.rollbackToLastForkThenPush.
func (e *fstEnum[T]) rollbackToLastForkThenPush() error {
	e.upto--
	for {
		if e.upto == 0 {
			return nil
		}
		prevArc := e.getArc(e.upto)
		if !prevArc.IsLast() {
			if _, err := e.fst.ReadNextArc(prevArc, e.fstReader); err != nil {
				return err
			}
			return e.pushFirst()
		}
		e.upto--
	}
}

// backtrackToFloorArc backtracks until we find a node whose first arc
// is before our target label, then finds the arc just before the
// target on that node. Mirrors FSTEnum.backtrackToFloorArc.
func (e *fstEnum[T]) backtrackToFloorArc(arc *Arc[T], targetLabel int, in BytesReader) (*Arc[T], error) {
	for {
		if _, err := e.fst.ReadFirstTargetArc(e.getArc(e.upto-1), arc, e.fstReader); err != nil {
			return nil, err
		}
		if arc.label < targetLabel {
			if !arc.IsLast() {
				if arc.bytesPerArc != 0 && arc.label != END_LABEL {
					switch arc.nodeFlags {
					case ARCS_FOR_BINARY_SEARCH:
						if err := e.findNextFloorArcBinarySearch(arc, targetLabel, in); err != nil {
							return nil, err
						}
					case ARCS_FOR_DIRECT_ADDRESSING:
						if err := e.findNextFloorArcDirectAddressing(arc, targetLabel, in); err != nil {
							return nil, err
						}
					case ARCS_FOR_CONTINUOUS:
						if err := e.findNextFloorArcContinuous(arc, targetLabel, in); err != nil {
							return nil, err
						}
					default:
						return nil, errors.New("fst.FSTEnum: backtrackToFloorArc: unexpected nodeFlags")
					}
				} else {
					for !arc.IsLast() {
						nextLabel, err := e.fst.ReadNextArcLabel(arc, in)
						if err != nil {
							return nil, err
						}
						if nextLabel >= targetLabel {
							break
						}
						if _, err := e.fst.ReadNextArc(arc, e.fstReader); err != nil {
							return nil, err
						}
					}
				}
			}
			return nil, e.pushLast()
		}
		e.upto--
		if e.upto == 0 {
			return nil, nil
		}
		targetLabel = e.labels.GetTargetLabel()
		arc = e.getArc(e.upto)
	}
}

// findNextFloorArcDirectAddressing finds an arc on the current node
// whose label is strictly less than targetLabel, skipping the first
// arc. Mirrors FSTEnum.findNextFloorArcDirectAddressing.
func (e *fstEnum[T]) findNextFloorArcDirectAddressing(arc *Arc[T], targetLabel int, in BytesReader) error {
	if arc.numArcs > 1 {
		targetIndex := targetLabel - arc.firstLabel
		if targetIndex >= arc.numArcs {
			if _, err := e.fst.ReadLastArcByDirectAddressing(arc, in); err != nil {
				return err
			}
		} else {
			floorIndex, err := bitTablePreviousBitSetArc(targetIndex, arc, in)
			if err != nil {
				return err
			}
			if floorIndex > 0 {
				if _, err := e.fst.ReadArcByDirectAddressing(arc, in, floorIndex); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// findNextFloorArcContinuous is the continuous-node analogue of
// findNextFloorArcDirectAddressing. Mirrors
// FSTEnum.findNextFloorArcContinuous.
func (e *fstEnum[T]) findNextFloorArcContinuous(arc *Arc[T], targetLabel int, in BytesReader) error {
	if arc.numArcs > 1 {
		targetIndex := targetLabel - arc.firstLabel
		if targetIndex >= arc.numArcs {
			if _, err := e.fst.ReadLastArcByContinuous(arc, in); err != nil {
				return err
			}
		} else {
			if _, err := e.fst.ReadArcByContinuous(arc, in, targetIndex-1); err != nil {
				return err
			}
		}
	}
	return nil
}

// findNextFloorArcBinarySearch is the binary-search-node analogue of
// findNextFloorArcDirectAddressing. Mirrors
// FSTEnum.findNextFloorArcBinarySearch.
func (e *fstEnum[T]) findNextFloorArcBinarySearch(arc *Arc[T], targetLabel int, in BytesReader) error {
	if arc.numArcs > 1 {
		idx, err := BinarySearch(e.fst, arc, targetLabel)
		if err != nil {
			return err
		}
		if idx > 1 {
			if _, err := e.fst.ReadArcByIndex(arc, in, idx-1); err != nil {
				return err
			}
		} else if idx < -2 {
			if _, err := e.fst.ReadArcByIndex(arc, in, -2-idx); err != nil {
				return err
			}
		}
	}
	return nil
}

// doSeekFloorArrayPacked handles the floor seek inside a
// binary-search-packed arc array. Mirrors
// FSTEnum.doSeekFloorArrayPacked.
func (e *fstEnum[T]) doSeekFloorArrayPacked(arc *Arc[T], targetLabel int, in BytesReader) (*Arc[T], error) {
	idx, err := BinarySearch(e.fst, arc, targetLabel)
	if err != nil {
		return nil, err
	}
	if idx >= 0 {
		if _, err := e.fst.ReadArcByIndex(arc, in, idx); err != nil {
			return nil, err
		}
		e.output[e.upto] = e.fst.Outputs().Add(e.output[e.upto-1], arc.output)
		if targetLabel == END_LABEL {
			return nil, nil
		}
		e.labels.SetCurrentLabel(arc.label)
		if err := e.incr(); err != nil {
			return nil, err
		}
		return e.fst.ReadFirstTargetArc(arc, e.getArc(e.upto), e.fstReader)
	}
	if idx == -1 {
		return e.backtrackToFloorArc(arc, targetLabel, in)
	}
	// There is a floor arc; idx == -1 - (floor + 1) so floor == -2 -
	// idx.
	if _, err := e.fst.ReadArcByIndex(arc, in, -2-idx); err != nil {
		return nil, err
	}
	return nil, e.pushLast()
}

// doSeekFloorList handles the floor seek inside a variable-length
// (linear) arc list. Mirrors FSTEnum.doSeekFloorList.
func (e *fstEnum[T]) doSeekFloorList(arc *Arc[T], targetLabel int) (*Arc[T], error) {
	if arc.label == targetLabel {
		e.output[e.upto] = e.fst.Outputs().Add(e.output[e.upto-1], arc.output)
		if targetLabel == END_LABEL {
			return nil, nil
		}
		e.labels.SetCurrentLabel(arc.label)
		if err := e.incr(); err != nil {
			return nil, err
		}
		return e.fst.ReadFirstTargetArc(arc, e.getArc(e.upto), e.fstReader)
	}
	if arc.label > targetLabel {
		// Walk backwards to find a first arc that's before our target,
		// then scan forward to the arc just before targetLabel.
		for {
			if _, err := e.fst.ReadFirstTargetArc(e.getArc(e.upto-1), arc, e.fstReader); err != nil {
				return nil, err
			}
			if arc.label < targetLabel {
				for !arc.IsLast() {
					nextLabel, err := e.fst.ReadNextArcLabel(arc, e.fstReader)
					if err != nil {
						return nil, err
					}
					if nextLabel >= targetLabel {
						break
					}
					if _, err := e.fst.ReadNextArc(arc, e.fstReader); err != nil {
						return nil, err
					}
				}
				return nil, e.pushLast()
			}
			e.upto--
			if e.upto == 0 {
				return nil, nil
			}
			targetLabel = e.labels.GetTargetLabel()
			arc = e.getArc(e.upto)
		}
	}
	if !arc.IsLast() {
		nextLabel, err := e.fst.ReadNextArcLabel(arc, e.fstReader)
		if err != nil {
			return nil, err
		}
		if nextLabel > targetLabel {
			return nil, e.pushLast()
		}
		// Keep scanning.
		return e.fst.ReadNextArc(arc, e.fstReader)
	}
	return nil, e.pushLast()
}

// doSeekExact seeks to exactly target. Returns true when target is in
// the FST. Mirrors FSTEnum.doSeekExact.
func (e *fstEnum[T]) doSeekExact() (bool, error) {
	if err := e.rewindPrefix(); err != nil {
		return false, err
	}
	arc := e.getArc(e.upto - 1)
	targetLabel := e.labels.GetTargetLabel()
	in := e.fst.GetBytesReader()
	for {
		nextArc, err := e.fst.FindTargetArc(targetLabel, arc, e.getArc(e.upto), in)
		if err != nil {
			return false, err
		}
		if nextArc == nil {
			// Short circuit: target not found. Reposition arc[upto] to
			// the first real arc of the current node so a subsequent
			// next/seek starts from a consistent state.
			if _, err := e.fst.ReadFirstTargetArc(arc, e.getArc(e.upto), e.fstReader); err != nil {
				return false, err
			}
			return false, nil
		}
		e.output[e.upto] = e.fst.Outputs().Add(e.output[e.upto-1], nextArc.output)
		if targetLabel == END_LABEL {
			return true, nil
		}
		e.labels.SetCurrentLabel(targetLabel)
		if err := e.incr(); err != nil {
			return false, err
		}
		targetLabel = e.labels.GetTargetLabel()
		arc = nextArc
	}
}

// incr grows the arcs/output buffers when upto outgrows them.
// Mirrors FSTEnum.incr — the subclass grow() hook is invoked first
// (it must grow whatever key buffer the concrete enum holds) so that
// subsequent SetCurrentLabel calls land in a sized slot.
func (e *fstEnum[T]) incr() error {
	e.upto++
	e.labels.Grow()
	if len(e.arcs) <= e.upto {
		newLen := util.Oversize(1+e.upto, util.NumBytesObjectRef)
		newArcs := make([]*Arc[T], newLen)
		copy(newArcs, e.arcs)
		e.arcs = newArcs
	}
	if len(e.output) <= e.upto {
		newLen := util.Oversize(1+e.upto, util.NumBytesObjectRef)
		newOutput := make([]T, newLen)
		copy(newOutput, e.output)
		e.output = newOutput
	}
	return nil
}

// pushFirst appends the current arc and recurses from its target,
// appending the first arc all the way to the next final node. Mirrors
// FSTEnum.pushFirst.
func (e *fstEnum[T]) pushFirst() error {
	arc := e.arcs[e.upto]
	if arc == nil {
		return errors.New("fst.FSTEnum: pushFirst: nil arc at upto")
	}
	for {
		e.output[e.upto] = e.fst.Outputs().Add(e.output[e.upto-1], arc.output)
		if arc.label == END_LABEL {
			return nil
		}
		e.labels.SetCurrentLabel(arc.label)
		if err := e.incr(); err != nil {
			return err
		}
		nextArc := e.getArc(e.upto)
		if _, err := e.fst.ReadFirstTargetArc(arc, nextArc, e.fstReader); err != nil {
			return err
		}
		arc = nextArc
	}
}

// pushLast recurses from the current arc, appending the last arc all
// the way to the first final node. Mirrors FSTEnum.pushLast.
func (e *fstEnum[T]) pushLast() error {
	arc := e.arcs[e.upto]
	if arc == nil {
		return errors.New("fst.FSTEnum: pushLast: nil arc at upto")
	}
	for {
		e.labels.SetCurrentLabel(arc.label)
		e.output[e.upto] = e.fst.Outputs().Add(e.output[e.upto-1], arc.output)
		if arc.label == END_LABEL {
			return nil
		}
		if err := e.incr(); err != nil {
			return err
		}
		next, err := e.fst.ReadLastTargetArc(arc, e.getArc(e.upto), e.fstReader)
		if err != nil {
			return err
		}
		arc = next
	}
}
