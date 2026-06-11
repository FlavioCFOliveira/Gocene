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

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// maxOutputBytes limits the size of a single FST output sequence read from a VInt,
// preventing OOM from crafted/corrupted FST files.
const maxOutputBytes = 100_000_000 // 100 MB

// byteSequenceNoOutput is the singleton "no output" BytesRef used by
// ByteSequenceOutputs. Comparisons are performed by pointer identity,
// matching Lucene's reference-equality contract.
var byteSequenceNoOutput = util.NewBytesRefEmpty()

// ByteSequenceOutputsImpl is the Outputs implementation whose values
// are sequences of bytes (BytesRef). It is the Go port of
// org.apache.lucene.util.fst.ByteSequenceOutputs.
type ByteSequenceOutputsImpl struct{}

var byteSequenceOutputsSingleton = &ByteSequenceOutputsImpl{}

// ByteSequenceOutputs returns the ByteSequenceOutputs singleton.
// Equivalent to ByteSequenceOutputs.getSingleton().
func ByteSequenceOutputs() Outputs[*util.BytesRef] { return byteSequenceOutputsSingleton }

// ByteSequenceOutputsSingleton returns the concrete singleton.
func ByteSequenceOutputsSingleton() *ByteSequenceOutputsImpl { return byteSequenceOutputsSingleton }

// mismatchPos returns the index of the first byte that differs between
// the two byte regions, or len if all bytes are equal up to the
// shorter length and -1 if both ranges are fully equal (same length
// and all bytes match). The contract matches java.util.Arrays.mismatch
// with the (array, fromIndex, toIndex, array, fromIndex, toIndex)
// signature used by Lucene.
func mismatchPos(a []byte, ao, ae int, b []byte, bo, be int) int {
	la, lb := ae-ao, be-bo
	n := la
	if lb < n {
		n = lb
	}
	for i := 0; i < n; i++ {
		if a[ao+i] != b[bo+i] {
			return i
		}
	}
	if la == lb {
		return -1
	}
	return n
}

// Common implements Outputs.
func (*ByteSequenceOutputsImpl) Common(o1, o2 *util.BytesRef) *util.BytesRef {
	mp := mismatchPos(
		o1.Bytes, o1.Offset, o1.Offset+o1.Length,
		o2.Bytes, o2.Offset, o2.Offset+o2.Length,
	)
	switch {
	case mp == 0:
		return byteSequenceNoOutput
	case mp == -1:
		return o1
	case mp == o1.Length:
		return o1
	case mp == o2.Length:
		return o2
	default:
		return &util.BytesRef{Bytes: o1.Bytes, Offset: o1.Offset, Length: mp}
	}
}

// Subtract implements Outputs.
func (*ByteSequenceOutputsImpl) Subtract(output, inc *util.BytesRef) *util.BytesRef {
	if inc == byteSequenceNoOutput {
		return output
	}
	if inc.Length == output.Length {
		return byteSequenceNoOutput
	}
	return &util.BytesRef{
		Bytes:  output.Bytes,
		Offset: output.Offset + inc.Length,
		Length: output.Length - inc.Length,
	}
}

// Add implements Outputs.
func (*ByteSequenceOutputsImpl) Add(prefix, output *util.BytesRef) *util.BytesRef {
	if prefix == byteSequenceNoOutput {
		return output
	}
	if output == byteSequenceNoOutput {
		return prefix
	}
	buf := make([]byte, prefix.Length+output.Length)
	copy(buf, prefix.Bytes[prefix.Offset:prefix.Offset+prefix.Length])
	copy(buf[prefix.Length:], output.Bytes[output.Offset:output.Offset+output.Length])
	return &util.BytesRef{Bytes: buf, Offset: 0, Length: len(buf)}
}

// Write implements Outputs. Encoded as VInt length + length raw bytes.
func (*ByteSequenceOutputsImpl) Write(prefix *util.BytesRef, out store.DataOutput) error {
	vlw, ok := out.(store.VariableLengthOutput)
	if !ok {
		return errNotVariableLengthOutput
	}
	if err := vlw.WriteVInt(int32(prefix.Length)); err != nil {
		return err
	}
	if prefix.Length == 0 {
		return nil
	}
	return out.WriteBytesN(prefix.Bytes[prefix.Offset:prefix.Offset+prefix.Length], prefix.Length)
}

// WriteFinalOutput implements Outputs (default behaviour).
func (b *ByteSequenceOutputsImpl) WriteFinalOutput(o *util.BytesRef, out store.DataOutput) error {
	return b.Write(o, out)
}

// Read implements Outputs.
func (*ByteSequenceOutputsImpl) Read(in store.DataInput) (*util.BytesRef, error) {
	vli, ok := in.(store.VariableLengthInput)
	if !ok {
		return nil, errNotVariableLengthInput
	}
	n, err := vli.ReadVInt()
	if err != nil {
		return nil, err
	}
	if n < 0 || n > maxOutputBytes {
		return nil, fmt.Errorf("ByteSequenceOutputs.Read: length %d exceeds maximum %d", n, maxOutputBytes)
	}
	if n == 0 {
		return byteSequenceNoOutput, nil
	}
	buf := make([]byte, n)
	if err := in.ReadBytes(buf); err != nil {
		return nil, err
	}
	return &util.BytesRef{Bytes: buf, Offset: 0, Length: int(n)}, nil
}

// SkipOutput implements Outputs.
func (*ByteSequenceOutputsImpl) SkipOutput(in store.DataInput) error {
	vli, ok := in.(store.VariableLengthInput)
	if !ok {
		return errNotVariableLengthInput
	}
	n, err := vli.ReadVInt()
	if err != nil {
		return err
	}
	if n == 0 {
		return nil
	}
	// Best-effort skip: drain n bytes from the DataInput. We use
	// ReadBytes into a discard buffer because the DataInput interface
	// does not expose a SkipBytes method.
	scratch := make([]byte, n)
	return in.ReadBytes(scratch)
}

// ReadFinalOutput implements Outputs.
func (b *ByteSequenceOutputsImpl) ReadFinalOutput(in store.DataInput) (*util.BytesRef, error) {
	return b.Read(in)
}

// SkipFinalOutput implements Outputs.
func (b *ByteSequenceOutputsImpl) SkipFinalOutput(in store.DataInput) error { return b.SkipOutput(in) }

// GetNoOutput implements Outputs.
func (*ByteSequenceOutputsImpl) GetNoOutput() *util.BytesRef { return byteSequenceNoOutput }

// OutputToString implements Outputs.
func (*ByteSequenceOutputsImpl) OutputToString(o *util.BytesRef) string { return o.String() }

// Merge implements Outputs. ByteSequenceOutputs.merge is not
// overridden in Lucene, so we return ErrUnsupportedMerge.
func (*ByteSequenceOutputsImpl) Merge(_, _ *util.BytesRef) (*util.BytesRef, error) {
	return nil, ErrUnsupportedMerge
}

// RAMBytesUsed implements Outputs. Approximate: header + byte array.
func (*ByteSequenceOutputsImpl) RAMBytesUsed(o *util.BytesRef) int64 {
	const headerBytes = 32 // BytesRef header + slice header overhead
	return int64(headerBytes + cap(o.Bytes))
}

// String returns "ByteSequenceOutputs", matching Lucene's toString.
func (*ByteSequenceOutputsImpl) String() string { return "ByteSequenceOutputs" }
