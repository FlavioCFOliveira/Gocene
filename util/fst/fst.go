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
)

// FST is a compact byte[] representation of a finite-state
// transducer. It is the Go port of the final class
// org.apache.lucene.util.fst.FST. The byte stream produced by Save is
// byte-for-byte identical to Apache Lucene 10.4.0 for the same
// inputs.
//
// File-system wrappers around Save/ReadMetadata (the Java save(Path)
// and read(Path, Outputs) convenience methods) are intentionally
// omitted from this port; callers can wrap their own Path I/O around
// any store.DataOutput / store.DataInput pair.
type FST[T any] struct {
	metadata  *FSTMetadata[T]
	outputs   Outputs[T]
	fstReader FSTReader
}

// defaultMaxBlockBits matches Lucene's DEFAULT_MAX_BLOCK_BITS for
// 64-bit JREs. The Go port stores the FST as a single byte slice, so
// this constant is only consulted by the OnHeapFSTStore validator.
const defaultMaxBlockBits = 30

// NewFSTFromDataInput builds an FST from the supplied metadata,
// reading metadata.NumBytes() bytes from in into a fresh on-heap
// store. Mirrors the public Lucene constructor
// FST(FSTMetadata<T>, DataInput).
func NewFSTFromDataInput[T any](metadata *FSTMetadata[T], in store.DataInput) (*FST[T], error) {
	store, err := NewOnHeapFSTStoreFromDataInput(defaultMaxBlockBits, in, metadata.numBytes)
	if err != nil {
		return nil, err
	}
	return NewFSTFromReader(metadata, store)
}

// NewFSTFromReader builds an FST from a pre-constructed FSTReader.
// Mirrors the package-private Java constructor FST(FSTMetadata<T>,
// FSTReader). Returns an error if either argument is nil.
func NewFSTFromReader[T any](metadata *FSTMetadata[T], fstReader FSTReader) (*FST[T], error) {
	if metadata == nil {
		return nil, errors.New("fst: FSTMetadata cannot be nil")
	}
	if fstReader == nil {
		return nil, errors.New("fst: FSTReader cannot be nil")
	}
	return &FST[T]{
		metadata:  metadata,
		outputs:   metadata.outputs,
		fstReader: fstReader,
	}, nil
}

// FromFSTReader is the Lucene static factory FST.fromFSTReader. It
// returns (nil, nil) when metadata is nil (Lucene returns null in
// that case to signal "no node accepted by the FST"), and an error
// when the reader is nil.
func FromFSTReader[T any](metadata *FSTMetadata[T], fstReader FSTReader) (*FST[T], error) {
	if metadata == nil {
		return nil, nil
	}
	if fstReader == nil {
		return nil, errors.New("fst: FSTReader cannot be nil")
	}
	return NewFSTFromReader(metadata, fstReader)
}

// Metadata returns the FST's metadata block.
func (f *FST[T]) Metadata() *FSTMetadata[T] { return f.metadata }

// Outputs returns the outputs algebra used by this FST.
func (f *FST[T]) Outputs() Outputs[T] { return f.outputs }

// NumBytes returns the size of the FST byte stream.
func (f *FST[T]) NumBytes() int64 { return f.metadata.numBytes }

// GetEmptyOutput returns the output for the empty input. The boolean
// reports whether this FST accepts the empty input at all; when it
// is false the T value is the zero value of T.
func (f *FST[T]) GetEmptyOutput() (T, bool) {
	return f.metadata.emptyOutput, f.metadata.hasEmptyOutput
}

// RAMBytesUsed reports the approximate heap footprint of this FST,
// mirroring Accountable.ramBytesUsed.
func (f *FST[T]) RAMBytesUsed() int64 {
	// Mirrors RamUsageEstimator.shallowSizeOfInstance(FST.class) +
	// reader.ramBytesUsed(); shallow size is approximated as a small
	// constant in the Go port.
	const shallow = 32
	return shallow + f.fstReader.RAMBytesUsed()
}

// String returns the same shape as Lucene's FST.toString.
func (f *FST[T]) String() string {
	return fmt.Sprintf("FST(input=%s,output=%v", f.metadata.inputType, f.outputs)
}

// Save writes this FST to two DataOutputs. metaOut receives the
// metadata block; out receives the byte stream backing the FST. The
// byte stream emitted here is byte-for-byte identical to what
// Lucene 10.4.0 produces.
//
// File-system wrappers (Java's save(Path)) are not part of this
// port: callers can compose their own Path I/O around any
// store.DataOutput pair, including using the same DataOutput for
// both metaOut and out to match Lucene's single-file layout.
func (f *FST[T]) Save(metaOut, out store.DataOutput) error {
	if err := f.metadata.Save(metaOut); err != nil {
		return err
	}
	return f.fstReader.WriteTo(out)
}

// GetBytesReader returns a BytesReader positioned at the FST's start
// for reverse iteration over the byte stream. Mirrors
// FST.getBytesReader.
func (f *FST[T]) GetBytesReader() BytesReader {
	return f.fstReader.GetReverseBytesReader()
}

// ReadLabel decodes one input label from in according to the FST's
// declared input type. Mirrors FST.readLabel including the back-compat
// byte-reversal for BYTE2 in pre-versionLittleEndian streams.
func (f *FST[T]) ReadLabel(in store.DataInput) (int, error) {
	switch f.metadata.inputType {
	case InputTypeByte1:
		b, err := in.ReadByte()
		if err != nil {
			return 0, err
		}
		return int(b) & 0xFF, nil
	case InputTypeByte2:
		v, err := in.ReadShort()
		if err != nil {
			return 0, err
		}
		if f.metadata.version < versionLittleEndian {
			// Pre-LE: reverse the two bytes to match the written order.
			u := uint16(v)
			u = (u >> 8) | (u << 8)
			return int(u) & 0xFFFF, nil
		}
		return int(uint16(v)) & 0xFFFF, nil
	case InputTypeByte4:
		vli, ok := in.(store.VariableLengthInput)
		if !ok {
			return 0, errNotVariableLengthInput
		}
		v, err := vli.ReadVInt()
		if err != nil {
			return 0, err
		}
		return int(v), nil
	default:
		return 0, fmt.Errorf("fst: unknown input type %v", f.metadata.inputType)
	}
}

// TargetHasArcs reports whether the target of follow points to a node
// with outgoing arcs. Mirrors FST.targetHasArcs.
func TargetHasArcs[T any](arc *Arc[T]) bool { return arc.target > 0 }

// getNumPresenceBytes returns the number of bytes required to hold
// labelRange presence bits, one per arc. Mirrors FST.getNumPresenceBytes.
func getNumPresenceBytes(labelRange int) int {
	return (labelRange + 7) >> 3
}

// readPresenceBytes records the presence-bit-table start address in
// arc and advances in past the table. Mirrors FST.readPresenceBytes.
func readPresenceBytes[T any](arc *Arc[T], in BytesReader) error {
	arc.bitTableStart = in.GetPosition()
	return in.SkipBytes(int64(getNumPresenceBytes(arc.numArcs)))
}

// flag tests whether bit is set in flags. Mirrors the static
// FST.flag helper.
func flag(flags, bit int) bool { return (flags & bit) != 0 }

// GetFirstArc fills arc with the virtual "start" arc — the empty
// incoming arc to the start node. Returns arc. Mirrors FST.getFirstArc.
func (f *FST[T]) GetFirstArc(arc *Arc[T]) *Arc[T] {
	noOutput := f.outputs.GetNoOutput()
	if f.metadata.hasEmptyOutput {
		arc.flags = byte(BIT_FINAL_ARC | BIT_LAST_ARC)
		arc.nextFinalOutput = f.metadata.emptyOutput
		// Lucene uses reference equality (emptyOutput != NO_OUTPUT) to
		// decide whether to set BIT_ARC_HAS_FINAL_OUTPUT. In Go we
		// cannot match arbitrary T by == reliably; we rely on the
		// Outputs contract that GetNoOutput returns a singleton and on
		// the fact that PositiveIntOutputs uses value equality (0 ==
		// 0 detected as no-output anyway). Use isNoOutput which is
		// reflect-free for pointer/interface types and value-equal for
		// comparable scalars.
		if !isNoOutput[T](f.outputs, f.metadata.emptyOutput, noOutput) {
			arc.flags |= byte(BIT_ARC_HAS_FINAL_OUTPUT)
		}
	} else {
		arc.flags = byte(BIT_LAST_ARC)
		arc.nextFinalOutput = noOutput
	}
	arc.output = noOutput
	arc.target = f.metadata.startNode
	return arc
}

// isNoOutput compares two output values for "is the no-output
// singleton". For interface or pointer T this is pointer equality;
// for comparable value types it is value equality. T is unconstrained
// here so we fall back to a runtime type assertion of comparability.
func isNoOutput[T any](_ Outputs[T], a, b T) bool {
	// We can't compare T directly without a constraint. Use the
	// any-wrapping trick: convert both to any and compare. Go's ==
	// on interface values is defined as pointer-equal for pointer
	// types and value-equal for comparable concrete types; for
	// non-comparable types (slices, maps, funcs) the == panics, but
	// no Outputs implementation in Lucene uses such types — outputs
	// are always pointer (BytesRef*), int64, or similar.
	return any(a) == any(b)
}

// readLastTargetArc follows follow and reads the last arc of its
// target into arc. Mirrors FST.readLastTargetArc.
func (f *FST[T]) readLastTargetArc(follow, arc *Arc[T], in BytesReader) (*Arc[T], error) {
	if !TargetHasArcs(follow) {
		if !follow.IsFinal() {
			return nil, errors.New("fst: readLastTargetArc on non-final dead-end arc")
		}
		arc.label = END_LABEL
		arc.target = FinalEndNode
		arc.output = follow.nextFinalOutput
		arc.flags = byte(BIT_LAST_ARC)
		arc.nodeFlags = arc.flags
		return arc, nil
	}
	in.SetPosition(follow.target)
	flagsByte, err := in.ReadByte()
	if err != nil {
		return nil, err
	}
	arc.nodeFlags = flagsByte
	if flagsByte == ARCS_FOR_BINARY_SEARCH ||
		flagsByte == ARCS_FOR_DIRECT_ADDRESSING ||
		flagsByte == ARCS_FOR_CONTINUOUS {
		numArcs, err := in.ReadVInt()
		if err != nil {
			return nil, err
		}
		arc.numArcs = int(numArcs)
		bytesPerArc, err := in.ReadVInt()
		if err != nil {
			return nil, err
		}
		arc.bytesPerArc = int(bytesPerArc)
		switch flagsByte {
		case ARCS_FOR_DIRECT_ADDRESSING:
			if err := readPresenceBytes(arc, in); err != nil {
				return nil, err
			}
			fl, err := f.ReadLabel(in)
			if err != nil {
				return nil, err
			}
			arc.firstLabel = fl
			arc.posArcsStart = in.GetPosition()
			return f.ReadLastArcByDirectAddressing(arc, in)
		case ARCS_FOR_BINARY_SEARCH:
			arc.arcIdx = arc.numArcs - 2
			arc.posArcsStart = in.GetPosition()
			return f.ReadNextRealArc(arc, in)
		default:
			// ARCS_FOR_CONTINUOUS
			fl, err := f.ReadLabel(in)
			if err != nil {
				return nil, err
			}
			arc.firstLabel = fl
			arc.posArcsStart = in.GetPosition()
			return f.ReadLastArcByContinuous(arc, in)
		}
	}
	arc.flags = flagsByte
	arc.bytesPerArc = 0
	// Linear scan to the last arc.
	for !arc.IsLast() {
		if _, err := f.ReadLabel(in); err != nil {
			return nil, err
		}
		if arc.flag(BIT_ARC_HAS_OUTPUT) {
			if err := f.outputs.SkipOutput(in); err != nil {
				return nil, err
			}
		}
		if arc.flag(BIT_ARC_HAS_FINAL_OUTPUT) {
			if err := f.outputs.SkipFinalOutput(in); err != nil {
				return nil, err
			}
		}
		if arc.flag(BIT_STOP_NODE) {
			// nothing
		} else if arc.flag(BIT_TARGET_NEXT) {
			// nothing
		} else {
			if _, err := readUnpackedNodeTarget(in); err != nil {
				return nil, err
			}
		}
		nextFlags, err := in.ReadByte()
		if err != nil {
			return nil, err
		}
		arc.flags = nextFlags
	}
	// Undo the byte flags we read.
	if err := in.SkipBytes(-1); err != nil {
		return nil, err
	}
	arc.nextArc = in.GetPosition()
	return f.ReadNextRealArc(arc, in)
}

// readUnpackedNodeTarget reads an arc's unpacked target address.
// Mirrors FST.readUnpackedNodeTarget.
func readUnpackedNodeTarget(in BytesReader) (int64, error) {
	return in.ReadVLong()
}

// ReadFirstTargetArc follows follow and reads the first arc of its
// target into arc. Mirrors FST.readFirstTargetArc.
func (f *FST[T]) ReadFirstTargetArc(follow, arc *Arc[T], in BytesReader) (*Arc[T], error) {
	if follow.IsFinal() {
		arc.label = END_LABEL
		arc.output = follow.nextFinalOutput
		arc.flags = byte(BIT_FINAL_ARC)
		if follow.target <= 0 {
			arc.flags |= byte(BIT_LAST_ARC)
		} else {
			arc.nextArc = follow.target
		}
		arc.target = FinalEndNode
		arc.nodeFlags = arc.flags
		return arc, nil
	}
	return f.ReadFirstRealTargetArc(follow.target, arc, in)
}

// readFirstArcInfo positions in at nodeAddress and reads the node
// header (without reading the first real arc). Mirrors
// FST.readFirstArcInfo.
func (f *FST[T]) readFirstArcInfo(nodeAddress int64, arc *Arc[T], in BytesReader) error {
	in.SetPosition(nodeAddress)
	flagsByte, err := in.ReadByte()
	if err != nil {
		return err
	}
	arc.nodeFlags = flagsByte
	if flagsByte == ARCS_FOR_BINARY_SEARCH ||
		flagsByte == ARCS_FOR_DIRECT_ADDRESSING ||
		flagsByte == ARCS_FOR_CONTINUOUS {
		numArcs, err := in.ReadVInt()
		if err != nil {
			return err
		}
		arc.numArcs = int(numArcs)
		bytesPerArc, err := in.ReadVInt()
		if err != nil {
			return err
		}
		arc.bytesPerArc = int(bytesPerArc)
		arc.arcIdx = -1
		switch flagsByte {
		case ARCS_FOR_DIRECT_ADDRESSING:
			if err := readPresenceBytes(arc, in); err != nil {
				return err
			}
			fl, err := f.ReadLabel(in)
			if err != nil {
				return err
			}
			arc.firstLabel = fl
			arc.presenceIndex = -1
		case ARCS_FOR_CONTINUOUS:
			fl, err := f.ReadLabel(in)
			if err != nil {
				return err
			}
			arc.firstLabel = fl
		}
		arc.posArcsStart = in.GetPosition()
	} else {
		arc.nextArc = nodeAddress
		arc.bytesPerArc = 0
	}
	return nil
}

// ReadFirstRealTargetArc seeks to nodeAddress, parses the node
// header, and reads the first real arc. Mirrors
// FST.readFirstRealTargetArc.
func (f *FST[T]) ReadFirstRealTargetArc(nodeAddress int64, arc *Arc[T], in BytesReader) (*Arc[T], error) {
	if err := f.readFirstArcInfo(nodeAddress, arc, in); err != nil {
		return nil, err
	}
	return f.ReadNextRealArc(arc, in)
}

// IsExpandedTarget reports whether follow's target is a fixed-length
// arc node. Mirrors FST.isExpandedTarget.
func (f *FST[T]) IsExpandedTarget(follow *Arc[T], in BytesReader) (bool, error) {
	if !TargetHasArcs(follow) {
		return false, nil
	}
	in.SetPosition(follow.target)
	flagsByte, err := in.ReadByte()
	if err != nil {
		return false, err
	}
	return flagsByte == ARCS_FOR_BINARY_SEARCH ||
		flagsByte == ARCS_FOR_DIRECT_ADDRESSING ||
		flagsByte == ARCS_FOR_CONTINUOUS, nil
}

// ReadNextArc advances arc to its successor. If arc is the inserted
// "final" arc (label == END_LABEL) then reading the next arc means
// reading the first real arc of arc.nextArc. Mirrors FST.readNextArc.
func (f *FST[T]) ReadNextArc(arc *Arc[T], in BytesReader) (*Arc[T], error) {
	if arc.label == END_LABEL {
		if arc.nextArc <= 0 {
			return nil, errors.New("fst: cannot readNextArc when arc.isLast() is true")
		}
		return f.ReadFirstRealTargetArc(arc.nextArc, arc, in)
	}
	return f.ReadNextRealArc(arc, in)
}

// ReadNextArcLabel peeks at the next arc's label without altering
// arc. Caller must ensure !arc.IsLast(). Mirrors FST.readNextArcLabel.
func (f *FST[T]) ReadNextArcLabel(arc *Arc[T], in BytesReader) (int, error) {
	if arc.IsLast() {
		return 0, errors.New("fst: readNextArcLabel called on last arc")
	}
	if arc.label == END_LABEL {
		in.SetPosition(arc.nextArc)
		flagsByte, err := in.ReadByte()
		if err != nil {
			return 0, err
		}
		if flagsByte == ARCS_FOR_BINARY_SEARCH ||
			flagsByte == ARCS_FOR_DIRECT_ADDRESSING ||
			flagsByte == ARCS_FOR_CONTINUOUS {
			numArcs, err := in.ReadVInt()
			if err != nil {
				return 0, err
			}
			if _, err := in.ReadVInt(); err != nil { // skip bytesPerArc
				return 0, err
			}
			switch flagsByte {
			case ARCS_FOR_BINARY_SEARCH:
				if _, err := in.ReadByte(); err != nil { // skip arc flags
					return 0, err
				}
			case ARCS_FOR_DIRECT_ADDRESSING:
				if err := in.SkipBytes(int64(getNumPresenceBytes(int(numArcs)))); err != nil {
					return 0, err
				}
			}
			// Nothing to skip for ARCS_FOR_CONTINUOUS.
		}
	} else {
		switch arc.nodeFlags {
		case ARCS_FOR_BINARY_SEARCH:
			in.SetPosition(arc.posArcsStart - int64(1+arc.arcIdx)*int64(arc.bytesPerArc) - 1)
		case ARCS_FOR_DIRECT_ADDRESSING:
			nextIndex, err := bitTableNextBitSetArc(arc.arcIdx, arc, in)
			if err != nil {
				return 0, err
			}
			if nextIndex == -1 {
				return 0, errors.New("fst: readNextArcLabel: no next presence bit")
			}
			return arc.firstLabel + nextIndex, nil
		case ARCS_FOR_CONTINUOUS:
			return arc.firstLabel + arc.arcIdx + 1, nil
		default:
			// Variable-length arcs - linear search.
			in.SetPosition(arc.nextArc - 1)
		}
	}
	return f.ReadLabel(in)
}

// ReadArcByIndex reads the idx-th arc of a binary-search node into
// arc. Mirrors FST.readArcByIndex.
func (f *FST[T]) ReadArcByIndex(arc *Arc[T], in BytesReader, idx int) (*Arc[T], error) {
	if arc.bytesPerArc <= 0 {
		return nil, errors.New("fst: ReadArcByIndex requires bytesPerArc > 0")
	}
	if arc.nodeFlags != ARCS_FOR_BINARY_SEARCH {
		return nil, errors.New("fst: ReadArcByIndex requires binary-search node")
	}
	if idx < 0 || idx >= arc.numArcs {
		return nil, fmt.Errorf("fst: ReadArcByIndex: idx %d out of range [0,%d)", idx, arc.numArcs)
	}
	in.SetPosition(arc.posArcsStart - int64(idx)*int64(arc.bytesPerArc))
	arc.arcIdx = idx
	flagsByte, err := in.ReadByte()
	if err != nil {
		return nil, err
	}
	arc.flags = flagsByte
	return f.readArc(arc, in)
}

// ReadArcByContinuous reads the rangeIndex-th arc of a continuous
// node. Mirrors FST.readArcByContinuous.
func (f *FST[T]) ReadArcByContinuous(arc *Arc[T], in BytesReader, rangeIndex int) (*Arc[T], error) {
	if rangeIndex < 0 || rangeIndex >= arc.numArcs {
		return nil, fmt.Errorf("fst: ReadArcByContinuous: rangeIndex %d out of range [0,%d)", rangeIndex, arc.numArcs)
	}
	in.SetPosition(arc.posArcsStart - int64(rangeIndex)*int64(arc.bytesPerArc))
	arc.arcIdx = rangeIndex
	flagsByte, err := in.ReadByte()
	if err != nil {
		return nil, err
	}
	arc.flags = flagsByte
	return f.readArc(arc, in)
}

// ReadArcByDirectAddressing reads the rangeIndex-th arc of a
// direct-addressing node. Computes the presence index by counting
// bits first. Mirrors FST.readArcByDirectAddressing(Arc, BytesReader,
// int).
func (f *FST[T]) ReadArcByDirectAddressing(arc *Arc[T], in BytesReader, rangeIndex int) (*Arc[T], error) {
	if rangeIndex < 0 || rangeIndex >= arc.numArcs {
		return nil, fmt.Errorf("fst: ReadArcByDirectAddressing: rangeIndex %d out of range [0,%d)", rangeIndex, arc.numArcs)
	}
	set, err := bitTableIsBitSetArc(rangeIndex, arc, in)
	if err != nil {
		return nil, err
	}
	if !set {
		return nil, fmt.Errorf("fst: ReadArcByDirectAddressing: bit not set at rangeIndex %d", rangeIndex)
	}
	presenceIndex, err := bitTableCountBitsUpToArc(rangeIndex, arc, in)
	if err != nil {
		return nil, err
	}
	return f.readArcByDirectAddressingIdx(arc, in, rangeIndex, presenceIndex)
}

// readArcByDirectAddressingIdx is the package-private overload that
// takes both rangeIndex and the precomputed presenceIndex. Mirrors
// the private FST.readArcByDirectAddressing(Arc, BytesReader, int, int).
func (f *FST[T]) readArcByDirectAddressingIdx(arc *Arc[T], in BytesReader, rangeIndex, presenceIndex int) (*Arc[T], error) {
	in.SetPosition(arc.posArcsStart - int64(presenceIndex)*int64(arc.bytesPerArc))
	arc.arcIdx = rangeIndex
	arc.presenceIndex = presenceIndex
	flagsByte, err := in.ReadByte()
	if err != nil {
		return nil, err
	}
	arc.flags = flagsByte
	return f.readArc(arc, in)
}

// ReadLastArcByDirectAddressing reads the last present arc of a
// direct-addressing node. Mirrors FST.readLastArcByDirectAddressing.
func (f *FST[T]) ReadLastArcByDirectAddressing(arc *Arc[T], in BytesReader) (*Arc[T], error) {
	count, err := bitTableCountBitsArc(arc, in)
	if err != nil {
		return nil, err
	}
	presenceIndex := count - 1
	return f.readArcByDirectAddressingIdx(arc, in, arc.numArcs-1, presenceIndex)
}

// ReadLastArcByContinuous reads the last arc of a continuous node.
// Mirrors FST.readLastArcByContinuous.
func (f *FST[T]) ReadLastArcByContinuous(arc *Arc[T], in BytesReader) (*Arc[T], error) {
	return f.ReadArcByContinuous(arc, in, arc.numArcs-1)
}

// ReadNextRealArc reads the arc that follows arc. Mirrors
// FST.readNextRealArc. Callers must ensure !arc.IsLast() except when
// called from a routine that has just primed arc.arcIdx = -1.
func (f *FST[T]) ReadNextRealArc(arc *Arc[T], in BytesReader) (*Arc[T], error) {
	switch arc.nodeFlags {
	case ARCS_FOR_BINARY_SEARCH, ARCS_FOR_CONTINUOUS:
		if arc.bytesPerArc <= 0 {
			return nil, errors.New("fst: ReadNextRealArc requires bytesPerArc > 0 for fixed-length arc nodes")
		}
		arc.arcIdx++
		if arc.arcIdx < 0 || arc.arcIdx >= arc.numArcs {
			return nil, fmt.Errorf("fst: ReadNextRealArc: arcIdx %d out of range [0,%d)", arc.arcIdx, arc.numArcs)
		}
		in.SetPosition(arc.posArcsStart - int64(arc.arcIdx)*int64(arc.bytesPerArc))
		flagsByte, err := in.ReadByte()
		if err != nil {
			return nil, err
		}
		arc.flags = flagsByte
	case ARCS_FOR_DIRECT_ADDRESSING:
		nextIndex, err := bitTableNextBitSetArc(arc.arcIdx, arc, in)
		if err != nil {
			return nil, err
		}
		return f.readArcByDirectAddressingIdx(arc, in, nextIndex, arc.presenceIndex+1)
	default:
		// Variable-length arcs - linear search.
		if arc.bytesPerArc != 0 {
			return nil, errors.New("fst: ReadNextRealArc: variable-length path entered with bytesPerArc != 0")
		}
		in.SetPosition(arc.nextArc)
		flagsByte, err := in.ReadByte()
		if err != nil {
			return nil, err
		}
		arc.flags = flagsByte
	}
	return f.readArc(arc, in)
}

// readArc parses the body of an arc after the flags byte has been
// consumed and assigned to arc.flags. Mirrors the private FST.readArc.
func (f *FST[T]) readArc(arc *Arc[T], in BytesReader) (*Arc[T], error) {
	if arc.nodeFlags == ARCS_FOR_DIRECT_ADDRESSING || arc.nodeFlags == ARCS_FOR_CONTINUOUS {
		arc.label = arc.firstLabel + arc.arcIdx
	} else {
		lbl, err := f.ReadLabel(in)
		if err != nil {
			return nil, err
		}
		arc.label = lbl
	}
	if arc.flag(BIT_ARC_HAS_OUTPUT) {
		o, err := f.outputs.Read(in)
		if err != nil {
			return nil, err
		}
		arc.output = o
	} else {
		arc.output = f.outputs.GetNoOutput()
	}
	if arc.flag(BIT_ARC_HAS_FINAL_OUTPUT) {
		o, err := f.outputs.ReadFinalOutput(in)
		if err != nil {
			return nil, err
		}
		arc.nextFinalOutput = o
	} else {
		arc.nextFinalOutput = f.outputs.GetNoOutput()
	}
	if arc.flag(BIT_STOP_NODE) {
		if arc.flag(BIT_FINAL_ARC) {
			arc.target = FinalEndNode
		} else {
			arc.target = NonFinalEndNode
		}
		arc.nextArc = in.GetPosition()
	} else if arc.flag(BIT_TARGET_NEXT) {
		arc.nextArc = in.GetPosition()
		if !arc.flag(BIT_LAST_ARC) {
			if arc.bytesPerArc == 0 {
				if err := f.seekToNextNode(in); err != nil {
					return nil, err
				}
			} else {
				var numArcs int
				if arc.nodeFlags == ARCS_FOR_DIRECT_ADDRESSING {
					n, err := bitTableCountBitsArc(arc, in)
					if err != nil {
						return nil, err
					}
					numArcs = n
				} else {
					numArcs = arc.numArcs
				}
				// Prevent integer overflow from crafted FST nodes where bytesPerArc*numArcs exceeds MaxInt64.
	pos := arc.posArcsStart - int64(arc.bytesPerArc)*int64(numArcs)
	if int64(arc.bytesPerArc) > 0 && numArcs > 0 && int64(numArcs) > (1<<62)/int64(arc.bytesPerArc) {
		return nil, fmt.Errorf("fst: integer overflow in readArc position (bytesPerArc=%d, numArcs=%d)", arc.bytesPerArc, numArcs)
	}
	in.SetPosition(pos)
			}
		}
		arc.target = in.GetPosition()
	} else {
		t, err := readUnpackedNodeTarget(in)
		if err != nil {
			return nil, err
		}
		arc.target = t
		arc.nextArc = in.GetPosition()
	}
	return arc, nil
}

// ReadEndArc fills arc with the synthetic end arc derived from follow
// when follow is final. Returns nil when follow is not final.
// Mirrors the package-private FST.readEndArc.
func ReadEndArc[T any](follow, arc *Arc[T]) *Arc[T] {
	if !follow.IsFinal() {
		return nil
	}
	if follow.target <= 0 {
		arc.flags = byte(BIT_LAST_ARC)
	} else {
		arc.flags = 0
		arc.nextArc = follow.target
	}
	arc.output = follow.nextFinalOutput
	arc.label = END_LABEL
	return arc
}

// FindTargetArc finds the arc leaving follow with label
// labelToMatch and stores the result in arc. Returns (nil, nil) when
// the label is not present. Mirrors FST.findTargetArc.
func (f *FST[T]) FindTargetArc(labelToMatch int, follow, arc *Arc[T], in BytesReader) (*Arc[T], error) {
	if labelToMatch == END_LABEL {
		if follow.IsFinal() {
			if follow.target <= 0 {
				arc.flags = byte(BIT_LAST_ARC)
			} else {
				arc.flags = 0
				arc.nextArc = follow.target
			}
			arc.output = follow.nextFinalOutput
			arc.label = END_LABEL
			arc.nodeFlags = arc.flags
			return arc, nil
		}
		return nil, nil
	}
	if !TargetHasArcs(follow) {
		return nil, nil
	}
	in.SetPosition(follow.target)
	flagsByte, err := in.ReadByte()
	if err != nil {
		return nil, err
	}
	arc.nodeFlags = flagsByte
	switch flagsByte {
	case ARCS_FOR_DIRECT_ADDRESSING:
		na, err := in.ReadVInt()
		if err != nil {
			return nil, err
		}
		arc.numArcs = int(na)
		bpa, err := in.ReadVInt()
		if err != nil {
			return nil, err
		}
		arc.bytesPerArc = int(bpa)
		if err := readPresenceBytes(arc, in); err != nil {
			return nil, err
		}
		fl, err := f.ReadLabel(in)
		if err != nil {
			return nil, err
		}
		arc.firstLabel = fl
		arc.posArcsStart = in.GetPosition()
		arcIndex := labelToMatch - arc.firstLabel
		if arcIndex < 0 || arcIndex >= arc.numArcs {
			return nil, nil
		}
		set, err := bitTableIsBitSetArc(arcIndex, arc, in)
		if err != nil {
			return nil, err
		}
		if !set {
			return nil, nil
		}
		return f.ReadArcByDirectAddressing(arc, in, arcIndex)
	case ARCS_FOR_BINARY_SEARCH:
		na, err := in.ReadVInt()
		if err != nil {
			return nil, err
		}
		arc.numArcs = int(na)
		bpa, err := in.ReadVInt()
		if err != nil {
			return nil, err
		}
		arc.bytesPerArc = int(bpa)
		arc.posArcsStart = in.GetPosition()
		// Sparse array; binary search.
		low := 0
		high := arc.numArcs - 1
		for low <= high {
			mid := int(uint(low+high) >> 1)
			in.SetPosition(arc.posArcsStart - int64(arc.bytesPerArc*mid+1))
			midLabel, err := f.ReadLabel(in)
			if err != nil {
				return nil, err
			}
			cmp := midLabel - labelToMatch
			switch {
			case cmp < 0:
				low = mid + 1
			case cmp > 0:
				high = mid - 1
			default:
				arc.arcIdx = mid - 1
				return f.ReadNextRealArc(arc, in)
			}
		}
		return nil, nil
	case ARCS_FOR_CONTINUOUS:
		na, err := in.ReadVInt()
		if err != nil {
			return nil, err
		}
		arc.numArcs = int(na)
		bpa, err := in.ReadVInt()
		if err != nil {
			return nil, err
		}
		arc.bytesPerArc = int(bpa)
		fl, err := f.ReadLabel(in)
		if err != nil {
			return nil, err
		}
		arc.firstLabel = fl
		arc.posArcsStart = in.GetPosition()
		arcIndex := labelToMatch - arc.firstLabel
		if arcIndex < 0 || arcIndex >= arc.numArcs {
			return nil, nil
		}
		arc.arcIdx = arcIndex - 1
		return f.ReadNextRealArc(arc, in)
	}
	// Linear scan.
	if err := f.readFirstArcInfo(follow.target, arc, in); err != nil {
		return nil, err
	}
	in.SetPosition(arc.nextArc)
	for {
		if arc.bytesPerArc != 0 {
			return nil, errors.New("fst: FindTargetArc: linear scan with bytesPerArc != 0")
		}
		flagsByte, err := in.ReadByte()
		if err != nil {
			return nil, err
		}
		arc.flags = flagsByte
		pos := in.GetPosition()
		lbl, err := f.ReadLabel(in)
		if err != nil {
			return nil, err
		}
		if lbl == labelToMatch {
			in.SetPosition(pos)
			return f.readArc(arc, in)
		}
		if lbl > labelToMatch {
			return nil, nil
		}
		if arc.IsLast() {
			return nil, nil
		}
		if flag(int(flagsByte), BIT_ARC_HAS_OUTPUT) {
			if err := f.outputs.SkipOutput(in); err != nil {
				return nil, err
			}
		}
		if flag(int(flagsByte), BIT_ARC_HAS_FINAL_OUTPUT) {
			if err := f.outputs.SkipFinalOutput(in); err != nil {
				return nil, err
			}
		}
		if !flag(int(flagsByte), BIT_STOP_NODE) && !flag(int(flagsByte), BIT_TARGET_NEXT) {
			if _, err := readUnpackedNodeTarget(in); err != nil {
				return nil, err
			}
		}
	}
}

// seekToNextNode skips over a single arc in the linear-scan format.
// Mirrors the private FST.seekToNextNode.
func (f *FST[T]) seekToNextNode(in BytesReader) error {
	for {
		flagsByte, err := in.ReadByte()
		if err != nil {
			return err
		}
		flags := int(flagsByte)
		if _, err := f.ReadLabel(in); err != nil {
			return err
		}
		if flag(flags, BIT_ARC_HAS_OUTPUT) {
			if err := f.outputs.SkipOutput(in); err != nil {
				return err
			}
		}
		if flag(flags, BIT_ARC_HAS_FINAL_OUTPUT) {
			if err := f.outputs.SkipFinalOutput(in); err != nil {
				return err
			}
		}
		if !flag(flags, BIT_STOP_NODE) && !flag(flags, BIT_TARGET_NEXT) {
			if _, err := readUnpackedNodeTarget(in); err != nil {
				return err
			}
		}
		if flag(flags, BIT_LAST_ARC) {
			return nil
		}
	}
}

// ReadLastTargetArc is the exported wrapper around readLastTargetArc.
// Lucene exposes this method as package-private, but other Gocene
// packages (FSTEnum, Util) will need it later.
func (f *FST[T]) ReadLastTargetArc(follow, arc *Arc[T], in BytesReader) (*Arc[T], error) {
	return f.readLastTargetArc(follow, arc, in)
}
