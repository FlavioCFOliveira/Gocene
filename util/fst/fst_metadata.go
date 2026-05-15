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

// InputType is the Go counterpart of FST.INPUT_TYPE, declaring the
// width of one input label in the FST byte stream.
type InputType int

const (
	// InputTypeByte1 declares 1-byte labels (BYTE1 in Lucene).
	InputTypeByte1 InputType = iota
	// InputTypeByte2 declares 2-byte (unsigned short) labels (BYTE2).
	InputTypeByte2
	// InputTypeByte4 declares 4-byte (VInt-encoded int) labels (BYTE4).
	InputTypeByte4
)

// String returns the Lucene enum-style name.
func (t InputType) String() string {
	switch t {
	case InputTypeByte1:
		return "BYTE1"
	case InputTypeByte2:
		return "BYTE2"
	case InputTypeByte4:
		return "BYTE4"
	default:
		return fmt.Sprintf("InputType(%d)", int(t))
	}
}

// FST byte-layout constants. These mirror the static finals in
// org.apache.lucene.util.fst.FST and must keep their exact numeric
// values for byte-for-byte compatibility with Apache Lucene 10.4.0.
const (
	// BIT_FINAL_ARC marks an arc as final.
	BIT_FINAL_ARC = 1 << 0
	// BIT_LAST_ARC marks the last arc out of its source node.
	BIT_LAST_ARC = 1 << 1
	// BIT_TARGET_NEXT marks that this arc's target is the very next
	// node in the byte stream (saves writing a VLong target address).
	BIT_TARGET_NEXT = 1 << 2
	// BIT_STOP_NODE marks an arc whose target node has no outgoing
	// arcs. Kept for back-compat; Lucene plans to free this bit.
	BIT_STOP_NODE = 1 << 3
	// BIT_ARC_HAS_OUTPUT is set when the arc carries a non-empty
	// output value.
	BIT_ARC_HAS_OUTPUT = 1 << 4
	// BIT_ARC_HAS_FINAL_OUTPUT is set when the arc carries a non-empty
	// final output value.
	BIT_ARC_HAS_FINAL_OUTPUT = 1 << 5
)

// Arc-array node-header markers. These three constants act as
// "flags" stored as the first byte of a fixed-length-arc node; they
// were chosen as bit combinations that cannot legally appear on a
// real arc, which lets FST traversal distinguish array nodes from
// list nodes by inspecting the first byte alone.
const (
	// ARCS_FOR_BINARY_SEARCH = BIT_ARC_HAS_FINAL_OUTPUT (1 << 5).
	ARCS_FOR_BINARY_SEARCH byte = BIT_ARC_HAS_FINAL_OUTPUT
	// ARCS_FOR_DIRECT_ADDRESSING = (1 << 6).
	ARCS_FOR_DIRECT_ADDRESSING byte = 1 << 6
	// ARCS_FOR_CONTINUOUS = ARCS_FOR_DIRECT_ADDRESSING + ARCS_FOR_BINARY_SEARCH
	// = (1 << 6) | (1 << 5).
	ARCS_FOR_CONTINUOUS byte = ARCS_FOR_DIRECT_ADDRESSING + ARCS_FOR_BINARY_SEARCH
)

// File-format identifier and supported version range.
const (
	// fileFormatName is the codec name written in the header.
	fileFormatName = "FST"

	// VERSION_START is the first supported version (Lucene 7.0).
	VERSION_START = 6

	// versionLittleEndian marks the introduction of LE on-disk
	// integer encoding. Used at read time to detect older 2-byte
	// labels that need byte reversal.
	versionLittleEndian = 8

	// VERSION_CONTINUOUS_ARCS marks the introduction of the
	// continuous-arc node layout.
	VERSION_CONTINUOUS_ARCS = 9

	// VERSION_CURRENT is the version this implementation writes.
	VERSION_CURRENT = VERSION_CONTINUOUS_ARCS

	// VERSION_90 is the format released with Lucene 9.0.
	VERSION_90 = versionLittleEndian
)

// Virtual node addresses. These constants never appear in serialised
// FST bytes — they are sentinels used by the in-memory traversal API.
const (
	// FinalEndNode is the virtual final node with no outgoing arcs.
	FinalEndNode int64 = -1
	// NonFinalEndNode is the virtual non-final node with no outgoing
	// arcs.
	NonFinalEndNode int64 = 0
	// END_LABEL is the label of the synthetic "final" arc inserted
	// when traversing a node that accepts the empty string.
	END_LABEL = -1
)

// CODEC_MAGIC is the four-byte magic number written at the start of
// every codec file, including FSTs. It is written and read in
// big-endian order, regardless of the rest of the byte stream.
// Mirrors org.apache.lucene.codecs.CodecUtil.CODEC_MAGIC.
const codecMagic int32 = 0x3FD76C17

// FSTMetadata captures the small descriptor block written before the
// FST byte array. It is the Go port of FST.FSTMetadata.
type FSTMetadata[T any] struct {
	inputType   InputType
	outputs     Outputs[T]
	emptyOutput T
	// hasEmptyOutput is true iff the FST accepts the empty input. We
	// cannot use a typed nil because T is a generic type parameter
	// and may be a value type (e.g. int64).
	hasEmptyOutput bool
	startNode      int64
	version        int
	numBytes       int64
}

// NewFSTMetadata builds a metadata descriptor. Pass hasEmptyOutput=false
// when the FST does not accept the empty string; the emptyOutput value
// is ignored in that case.
func NewFSTMetadata[T any](
	inputType InputType,
	outputs Outputs[T],
	emptyOutput T,
	hasEmptyOutput bool,
	startNode int64,
	version int,
	numBytes int64,
) *FSTMetadata[T] {
	return &FSTMetadata[T]{
		inputType:      inputType,
		outputs:        outputs,
		emptyOutput:    emptyOutput,
		hasEmptyOutput: hasEmptyOutput,
		startNode:      startNode,
		version:        version,
		numBytes:       numBytes,
	}
}

// InputType returns the label-width of this FST.
func (m *FSTMetadata[T]) InputType() InputType { return m.inputType }

// Outputs returns the Outputs algebra used by this FST.
func (m *FSTMetadata[T]) Outputs() Outputs[T] { return m.outputs }

// HasEmptyOutput reports whether the FST accepts the empty input.
func (m *FSTMetadata[T]) HasEmptyOutput() bool { return m.hasEmptyOutput }

// EmptyOutput returns the output produced for the empty input. It is
// only meaningful when HasEmptyOutput() is true.
func (m *FSTMetadata[T]) EmptyOutput() T { return m.emptyOutput }

// StartNode returns the address of the FST's start node.
func (m *FSTMetadata[T]) StartNode() int64 { return m.startNode }

// Version returns the binary-format version this metadata describes.
func (m *FSTMetadata[T]) Version() int { return m.version }

// NumBytes returns the size in bytes of the FST byte stream that
// follows this metadata block.
func (m *FSTMetadata[T]) NumBytes() int64 { return m.numBytes }

// Save serialises this metadata to metaOut. The byte stream produced
// here is identical to what Lucene 10.4.0's FSTMetadata.save emits.
//
// Layout (in order):
//  1. Codec header: 4 big-endian magic bytes, then writeString("FST"),
//     then 4 big-endian version bytes (writeBEInt). Magic and version
//     are explicitly big-endian regardless of the rest of the stream
//     (see CodecUtil.writeBEInt).
//  2. One byte: 0x01 if the FST accepts the empty input, else 0x00.
//  3. If accepting the empty input: writeVInt(emptyLen) followed by the
//     reversed serialised emptyOutput bytes.
//  4. One byte for the input type: 0=BYTE1, 1=BYTE2, 2=BYTE4.
//  5. writeVLong(startNode).
//  6. writeVLong(numBytes).
func (m *FSTMetadata[T]) Save(metaOut store.DataOutput) error {
	if err := writeCodecHeader(metaOut, fileFormatName, VERSION_CURRENT); err != nil {
		return err
	}
	if m.hasEmptyOutput {
		if err := metaOut.WriteByte(1); err != nil {
			return err
		}
		// Serialise the empty-string output into a buffer; then
		// reverse the byte order so that, when the FST is read back,
		// the reverse reader produces the exact same bytes in writer
		// order. Mirrors the in-place reversal in Lucene's save.
		buf := store.NewByteArrayDataOutput(16)
		if err := m.outputs.WriteFinalOutput(m.emptyOutput, buf); err != nil {
			return err
		}
		emptyBytes := buf.GetBytes()
		emptyLen := len(emptyBytes)
		reversed := make([]byte, emptyLen)
		for i := 0; i < emptyLen; i++ {
			reversed[i] = emptyBytes[emptyLen-1-i]
		}
		if err := store.WriteVInt(metaOut, int32(emptyLen)); err != nil {
			return err
		}
		if emptyLen > 0 {
			if err := metaOut.WriteBytesN(reversed, emptyLen); err != nil {
				return err
			}
		}
	} else {
		if err := metaOut.WriteByte(0); err != nil {
			return err
		}
	}
	var t byte
	switch m.inputType {
	case InputTypeByte1:
		t = 0
	case InputTypeByte2:
		t = 1
	case InputTypeByte4:
		t = 2
	default:
		return fmt.Errorf("fst: unknown input type %v", m.inputType)
	}
	if err := metaOut.WriteByte(t); err != nil {
		return err
	}
	if err := store.WriteVLong(metaOut, m.startNode); err != nil {
		return err
	}
	return store.WriteVLong(metaOut, m.numBytes)
}

// ReadMetadata parses an FST metadata block from metaIn. The supplied
// Outputs determines the output type and how the empty-output bytes
// are decoded. Mirrors FST.readMetadata.
func ReadMetadata[T any](metaIn store.DataInput, outputs Outputs[T]) (*FSTMetadata[T], error) {
	version, err := checkCodecHeader(metaIn, fileFormatName, VERSION_START, VERSION_CURRENT)
	if err != nil {
		return nil, err
	}
	hasEmpty, err := metaIn.ReadByte()
	if err != nil {
		return nil, err
	}
	var emptyOutput T
	var hasEmptyOutput bool
	if hasEmpty == 1 {
		vli, ok := metaIn.(store.VariableLengthInput)
		if !ok {
			return nil, errNotVariableLengthInput
		}
		emptyLen32, err := vli.ReadVInt()
		if err != nil {
			return nil, err
		}
		if emptyLen32 < 0 {
			return nil, fmt.Errorf("fst: corrupt metadata: negative empty-output length %d", emptyLen32)
		}
		emptyLen := int(emptyLen32)
		emptyBytes := make([]byte, emptyLen)
		if emptyLen > 0 {
			if err := metaIn.ReadBytes(emptyBytes); err != nil {
				return nil, err
			}
		}
		// The bytes are stored reversed in the metadata stream so the
		// reverse reader hands them back to outputs.readFinalOutput
		// in writer order.
		emptyStore := NewOnHeapFSTStoreFromBytes(emptyBytes)
		reader := emptyStore.GetReverseBytesReader()
		if emptyLen > 0 {
			reader.SetPosition(int64(emptyLen - 1))
		}
		emptyOutput, err = outputs.ReadFinalOutput(reader)
		if err != nil {
			return nil, err
		}
		hasEmptyOutput = true
	} else if hasEmpty != 0 {
		return nil, fmt.Errorf("fst: corrupt metadata: empty-output flag %d", hasEmpty)
	}
	tByte, err := metaIn.ReadByte()
	if err != nil {
		return nil, err
	}
	var inputType InputType
	switch tByte {
	case 0:
		inputType = InputTypeByte1
	case 1:
		inputType = InputTypeByte2
	case 2:
		inputType = InputTypeByte4
	default:
		return nil, fmt.Errorf("fst: corrupt metadata: invalid input type %d", tByte)
	}
	vli, ok := metaIn.(store.VariableLengthInput)
	if !ok {
		return nil, errNotVariableLengthInput
	}
	startNode, err := vli.ReadVLong()
	if err != nil {
		return nil, err
	}
	numBytes, err := vli.ReadVLong()
	if err != nil {
		return nil, err
	}
	return &FSTMetadata[T]{
		inputType:      inputType,
		outputs:        outputs,
		emptyOutput:    emptyOutput,
		hasEmptyOutput: hasEmptyOutput,
		startNode:      startNode,
		version:        int(version),
		numBytes:       numBytes,
	}, nil
}

// writeCodecHeader emits the four big-endian magic bytes, then the
// codec name as a length-prefixed UTF-8 VInt+bytes block, then the
// four big-endian version bytes. The big-endian writes are done by
// hand because store.DataOutput.WriteInt is little-endian on some
// implementations (e.g. ByteArrayDataOutput) and the codec header is
// canonically big-endian.
func writeCodecHeader(out store.DataOutput, codec string, version int32) error {
	if err := checkCodecName(codec); err != nil {
		return err
	}
	if err := writeBEInt32(out, codecMagic); err != nil {
		return err
	}
	if err := store.WriteString(out, codec); err != nil {
		return err
	}
	return writeBEInt32(out, version)
}

// checkCodecHeader reads and validates the four big-endian magic
// bytes, the codec name, and the big-endian version. Mirrors
// CodecUtil.checkHeader.
func checkCodecHeader(in store.DataInput, codec string, minVersion, maxVersion int32) (int32, error) {
	magic, err := readBEInt32(in)
	if err != nil {
		return 0, err
	}
	if magic != codecMagic {
		return 0, fmt.Errorf("fst: invalid codec magic: 0x%08x (expected 0x%08x)", uint32(magic), uint32(codecMagic))
	}
	actualCodec, err := in.ReadString()
	if err != nil {
		return 0, err
	}
	if actualCodec != codec {
		return 0, fmt.Errorf("fst: invalid codec name: %q (expected %q)", actualCodec, codec)
	}
	version, err := readBEInt32(in)
	if err != nil {
		return 0, err
	}
	if version < minVersion || version > maxVersion {
		return 0, fmt.Errorf("fst: unsupported version %d (must be %d..%d)", version, minVersion, maxVersion)
	}
	return version, nil
}

// checkCodecName mirrors the codec-name validation in CodecUtil.
func checkCodecName(codec string) error {
	if len(codec) >= 128 {
		return fmt.Errorf("fst: codec name too long: %d", len(codec))
	}
	for i := 0; i < len(codec); i++ {
		if codec[i] > 127 {
			return errors.New("fst: codec name must be ASCII")
		}
	}
	return nil
}

// writeBEInt32 emits a 32-bit signed value in big-endian order.
func writeBEInt32(out store.DataOutput, v int32) error {
	if err := out.WriteByte(byte(v >> 24)); err != nil {
		return err
	}
	if err := out.WriteByte(byte(v >> 16)); err != nil {
		return err
	}
	if err := out.WriteByte(byte(v >> 8)); err != nil {
		return err
	}
	return out.WriteByte(byte(v))
}

// readBEInt32 reads a 32-bit signed value in big-endian order.
func readBEInt32(in store.DataInput) (int32, error) {
	b0, err := in.ReadByte()
	if err != nil {
		return 0, err
	}
	b1, err := in.ReadByte()
	if err != nil {
		return 0, err
	}
	b2, err := in.ReadByte()
	if err != nil {
		return 0, err
	}
	b3, err := in.ReadByte()
	if err != nil {
		return 0, err
	}
	return int32(b0)<<24 | int32(b1)<<16 | int32(b2)<<8 | int32(b3), nil
}
