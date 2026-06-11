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

var intSequenceNoOutput = util.NewIntsRefEmpty()

// IntSequenceOutputsImpl is the Go port of
// org.apache.lucene.util.fst.IntSequenceOutputs. Outputs are
// sequences of ints serialised as VInt(length) plus length-many VInts
// (one per element). Values must fit in int32 to match Lucene's
// `int[]` element type and 32-bit VInt encoding.
type IntSequenceOutputsImpl struct{}

var intSequenceOutputsSingleton = &IntSequenceOutputsImpl{}

// IntSequenceOutputs returns the singleton.
func IntSequenceOutputs() Outputs[*util.IntsRef] { return intSequenceOutputsSingleton }

// IntSequenceOutputsSingleton returns the concrete singleton.
func IntSequenceOutputsSingleton() *IntSequenceOutputsImpl { return intSequenceOutputsSingleton }

// mismatchPosInt is the int-slice analogue of mismatchPos.
func mismatchPosInt(a []int, ao, ae int, b []int, bo, be int) int {
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
func (*IntSequenceOutputsImpl) Common(o1, o2 *util.IntsRef) *util.IntsRef {
	mp := mismatchPosInt(
		o1.Ints, o1.Offset, o1.Offset+o1.Length,
		o2.Ints, o2.Offset, o2.Offset+o2.Length,
	)
	switch {
	case mp == 0:
		return intSequenceNoOutput
	case mp == -1:
		return o1
	case mp == o1.Length:
		return o1
	case mp == o2.Length:
		return o2
	default:
		return &util.IntsRef{Ints: o1.Ints, Offset: o1.Offset, Length: mp}
	}
}

// Subtract implements Outputs.
func (*IntSequenceOutputsImpl) Subtract(output, inc *util.IntsRef) *util.IntsRef {
	if inc == intSequenceNoOutput {
		return output
	}
	if inc.Length == output.Length {
		return intSequenceNoOutput
	}
	return &util.IntsRef{
		Ints:   output.Ints,
		Offset: output.Offset + inc.Length,
		Length: output.Length - inc.Length,
	}
}

// Add implements Outputs.
func (*IntSequenceOutputsImpl) Add(prefix, output *util.IntsRef) *util.IntsRef {
	if prefix == intSequenceNoOutput {
		return output
	}
	if output == intSequenceNoOutput {
		return prefix
	}
	buf := make([]int, prefix.Length+output.Length)
	copy(buf, prefix.Ints[prefix.Offset:prefix.Offset+prefix.Length])
	copy(buf[prefix.Length:], output.Ints[output.Offset:output.Offset+output.Length])
	return &util.IntsRef{Ints: buf, Offset: 0, Length: len(buf)}
}

// Write implements Outputs.
func (*IntSequenceOutputsImpl) Write(prefix *util.IntsRef, out store.DataOutput) error {
	vlw, ok := out.(store.VariableLengthOutput)
	if !ok {
		return errNotVariableLengthOutput
	}
	if err := vlw.WriteVInt(int32(prefix.Length)); err != nil {
		return err
	}
	for i := 0; i < prefix.Length; i++ {
		if err := vlw.WriteVInt(int32(prefix.Ints[prefix.Offset+i])); err != nil {
			return err
		}
	}
	return nil
}

// WriteFinalOutput implements Outputs.
func (i *IntSequenceOutputsImpl) WriteFinalOutput(o *util.IntsRef, out store.DataOutput) error {
	return i.Write(o, out)
}

// Read implements Outputs.
func (*IntSequenceOutputsImpl) Read(in store.DataInput) (*util.IntsRef, error) {
	vli, ok := in.(store.VariableLengthInput)
	if !ok {
		return nil, errNotVariableLengthInput
	}
	n, err := vli.ReadVInt()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return intSequenceNoOutput, nil
	}
	if n < 0 || n > maxOutputBytes {
		return nil, fmt.Errorf("IntSequenceOutputs.Read: length %d exceeds maximum %d", n, maxOutputBytes)
	}
	ints := make([]int, n)
	for i := int32(0); i < n; i++ {
		v, err := vli.ReadVInt()
		if err != nil {
			return nil, err
		}
		ints[i] = int(v)
	}
	return &util.IntsRef{Ints: ints, Offset: 0, Length: int(n)}, nil
}

// SkipOutput implements Outputs.
func (*IntSequenceOutputsImpl) SkipOutput(in store.DataInput) error {
	vli, ok := in.(store.VariableLengthInput)
	if !ok {
		return errNotVariableLengthInput
	}
	n, err := vli.ReadVInt()
	if err != nil {
		return err
	}
	for i := int32(0); i < n; i++ {
		if _, err := vli.ReadVInt(); err != nil {
			return err
		}
	}
	return nil
}

// ReadFinalOutput implements Outputs.
func (i *IntSequenceOutputsImpl) ReadFinalOutput(in store.DataInput) (*util.IntsRef, error) {
	return i.Read(in)
}

// SkipFinalOutput implements Outputs.
func (i *IntSequenceOutputsImpl) SkipFinalOutput(in store.DataInput) error { return i.SkipOutput(in) }

// GetNoOutput implements Outputs.
func (*IntSequenceOutputsImpl) GetNoOutput() *util.IntsRef { return intSequenceNoOutput }

// OutputToString implements Outputs.
func (*IntSequenceOutputsImpl) OutputToString(o *util.IntsRef) string { return o.HexString() }

// Merge implements Outputs by concatenating the two int sequences.
// This mirrors Lucene's IntSequenceOutputs which concatenates.
func (*IntSequenceOutputsImpl) Merge(a, b *util.IntsRef) (*util.IntsRef, error) {
	if a == intSequenceNoOutput || a.Length == 0 {
		return b, nil
	}
	if b == intSequenceNoOutput || b.Length == 0 {
		return a, nil
	}
	buf := make([]int, a.Length+b.Length)
	copy(buf, a.Ints[a.Offset:a.Offset+a.Length])
	copy(buf[a.Length:], b.Ints[b.Offset:b.Offset+b.Length])
	return &util.IntsRef{Ints: buf, Offset: 0, Length: len(buf)}, nil
}

// RAMBytesUsed implements Outputs.
func (*IntSequenceOutputsImpl) RAMBytesUsed(o *util.IntsRef) int64 {
	const headerBytes = 32
	const intBytes = 8 // int is machine-word on 64-bit platforms
	return int64(headerBytes + cap(o.Ints)*intBytes)
}

// String returns "IntSequenceOutputs".
func (*IntSequenceOutputsImpl) String() string { return "IntSequenceOutputs" }
