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
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// charSequenceNoOutput is the singleton "no output" CharsRef.
var charSequenceNoOutput = util.NewCharsRef()

// CharSequenceOutputsImpl is the Go port of
// org.apache.lucene.util.fst.CharSequenceOutputs: an Outputs whose
// values are sequences of characters represented by CharsRef.
//
// NOTE on encoding: Lucene serialises each char as a VInt over its
// 16-bit UTF-16 value. The Gocene CharsRef uses []rune (int32) as its
// backing array, so callers must keep code points within the 16-bit
// range (0..0xFFFF) to preserve the byte format. Values outside that
// range are still encoded as their full VInt and will round-trip
// faithfully within Gocene, but will NOT match Lucene-produced bytes
// for the same logical character.
type CharSequenceOutputsImpl struct{}

var charSequenceOutputsSingleton = &CharSequenceOutputsImpl{}

// CharSequenceOutputs returns the singleton. Equivalent to
// CharSequenceOutputs.getSingleton().
func CharSequenceOutputs() Outputs[*util.CharsRef] { return charSequenceOutputsSingleton }

// CharSequenceOutputsSingleton returns the concrete singleton.
func CharSequenceOutputsSingleton() *CharSequenceOutputsImpl { return charSequenceOutputsSingleton }

// mismatchPosRune is the rune-slice analogue of mismatchPos.
func mismatchPosRune(a []rune, ao, ae int, b []rune, bo, be int) int {
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
func (*CharSequenceOutputsImpl) Common(o1, o2 *util.CharsRef) *util.CharsRef {
	mp := mismatchPosRune(
		o1.Chars, o1.Offset, o1.Offset+o1.Length,
		o2.Chars, o2.Offset, o2.Offset+o2.Length,
	)
	switch {
	case mp == 0:
		return charSequenceNoOutput
	case mp == -1:
		return o1
	case mp == o1.Length:
		return o1
	case mp == o2.Length:
		return o2
	default:
		return &util.CharsRef{Chars: o1.Chars, Offset: o1.Offset, Length: mp}
	}
}

// Subtract implements Outputs.
func (*CharSequenceOutputsImpl) Subtract(output, inc *util.CharsRef) *util.CharsRef {
	if inc == charSequenceNoOutput {
		return output
	}
	if inc.Length == output.Length {
		return charSequenceNoOutput
	}
	return &util.CharsRef{
		Chars:  output.Chars,
		Offset: output.Offset + inc.Length,
		Length: output.Length - inc.Length,
	}
}

// Add implements Outputs.
func (*CharSequenceOutputsImpl) Add(prefix, output *util.CharsRef) *util.CharsRef {
	if prefix == charSequenceNoOutput {
		return output
	}
	if output == charSequenceNoOutput {
		return prefix
	}
	buf := make([]rune, prefix.Length+output.Length)
	copy(buf, prefix.Chars[prefix.Offset:prefix.Offset+prefix.Length])
	copy(buf[prefix.Length:], output.Chars[output.Offset:output.Offset+output.Length])
	return &util.CharsRef{Chars: buf, Offset: 0, Length: len(buf)}
}

// Write implements Outputs. Encodes as VInt(length) followed by
// length per-char VInts (matching Lucene's CharSequenceOutputs.write).
func (*CharSequenceOutputsImpl) Write(prefix *util.CharsRef, out store.DataOutput) error {
	vlw, ok := out.(store.VariableLengthOutput)
	if !ok {
		return errNotVariableLengthOutput
	}
	if err := vlw.WriteVInt(int32(prefix.Length)); err != nil {
		return err
	}
	for i := 0; i < prefix.Length; i++ {
		if err := vlw.WriteVInt(int32(prefix.Chars[prefix.Offset+i])); err != nil {
			return err
		}
	}
	return nil
}

// WriteFinalOutput implements Outputs (default behaviour).
func (c *CharSequenceOutputsImpl) WriteFinalOutput(o *util.CharsRef, out store.DataOutput) error {
	return c.Write(o, out)
}

// Read implements Outputs.
func (*CharSequenceOutputsImpl) Read(in store.DataInput) (*util.CharsRef, error) {
	vli, ok := in.(store.VariableLengthInput)
	if !ok {
		return nil, errNotVariableLengthInput
	}
	n, err := vli.ReadVInt()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return charSequenceNoOutput, nil
	}
	chars := make([]rune, n)
	for i := int32(0); i < n; i++ {
		v, err := vli.ReadVInt()
		if err != nil {
			return nil, err
		}
		chars[i] = rune(v)
	}
	return &util.CharsRef{Chars: chars, Offset: 0, Length: int(n)}, nil
}

// SkipOutput implements Outputs.
func (*CharSequenceOutputsImpl) SkipOutput(in store.DataInput) error {
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
func (c *CharSequenceOutputsImpl) ReadFinalOutput(in store.DataInput) (*util.CharsRef, error) {
	return c.Read(in)
}

// SkipFinalOutput implements Outputs.
func (c *CharSequenceOutputsImpl) SkipFinalOutput(in store.DataInput) error { return c.SkipOutput(in) }

// GetNoOutput implements Outputs.
func (*CharSequenceOutputsImpl) GetNoOutput() *util.CharsRef { return charSequenceNoOutput }

// OutputToString implements Outputs.
func (*CharSequenceOutputsImpl) OutputToString(o *util.CharsRef) string { return o.String() }

// Merge implements Outputs. CharSequenceOutputs.merge is not
// overridden in Lucene.
func (*CharSequenceOutputsImpl) Merge(_, _ *util.CharsRef) (*util.CharsRef, error) {
	return nil, ErrUnsupportedMerge
}

// RAMBytesUsed implements Outputs. Approximation.
func (*CharSequenceOutputsImpl) RAMBytesUsed(o *util.CharsRef) int64 {
	const headerBytes = 32
	return int64(headerBytes + cap(o.Chars)*4)
}

// String returns "CharSequenceOutputs".
func (*CharSequenceOutputsImpl) String() string { return "CharSequenceOutputs" }
