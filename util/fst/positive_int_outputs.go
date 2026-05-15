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

import "github.com/FlavioCFOliveira/Gocene/store"

// positiveIntNoOutput is the singleton "no output" int64 value. The
// FST contract requires that getNoOutput always returns the same
// object; in Java the Long autoboxing returns the cached Long(0L),
// and Lucene compares it with ==. Here we use the literal int64(0)
// and document that comparison must be by value, not by pointer.
//
// In practice none of the operations on PositiveIntOutputs depend on
// reference equality of the sentinel — Lucene's ByteSequenceOutputs
// uses pointer identity but PositiveIntOutputs operates on Long
// which is a value type with semantic equality, so we mirror that.
const positiveIntNoOutput int64 = 0

// PositiveIntOutputsImpl is the Go port of
// org.apache.lucene.util.fst.PositiveIntOutputs. Outputs are
// non-negative int64 values; 0 is the sentinel "no output".
type PositiveIntOutputsImpl struct{}

var positiveIntOutputsSingleton = &PositiveIntOutputsImpl{}

// PositiveIntOutputs returns the singleton.
func PositiveIntOutputs() Outputs[int64] { return positiveIntOutputsSingleton }

// PositiveIntOutputsSingleton returns the concrete singleton.
func PositiveIntOutputsSingleton() *PositiveIntOutputsImpl { return positiveIntOutputsSingleton }

// Common implements Outputs.
func (*PositiveIntOutputsImpl) Common(o1, o2 int64) int64 {
	if o1 == positiveIntNoOutput || o2 == positiveIntNoOutput {
		return positiveIntNoOutput
	}
	if o1 < o2 {
		return o1
	}
	return o2
}

// Subtract implements Outputs.
func (*PositiveIntOutputsImpl) Subtract(output, inc int64) int64 {
	if inc == positiveIntNoOutput {
		return output
	}
	if output == inc {
		return positiveIntNoOutput
	}
	return output - inc
}

// Add implements Outputs.
func (*PositiveIntOutputsImpl) Add(prefix, output int64) int64 {
	if prefix == positiveIntNoOutput {
		return output
	}
	if output == positiveIntNoOutput {
		return prefix
	}
	return prefix + output
}

// Write implements Outputs.
func (*PositiveIntOutputsImpl) Write(output int64, out store.DataOutput) error {
	vlw, ok := out.(store.VariableLengthOutput)
	if !ok {
		return errNotVariableLengthOutput
	}
	return vlw.WriteVLong(output)
}

// WriteFinalOutput implements Outputs.
func (p *PositiveIntOutputsImpl) WriteFinalOutput(output int64, out store.DataOutput) error {
	return p.Write(output, out)
}

// Read implements Outputs.
func (*PositiveIntOutputsImpl) Read(in store.DataInput) (int64, error) {
	vli, ok := in.(store.VariableLengthInput)
	if !ok {
		return 0, errNotVariableLengthInput
	}
	v, err := vli.ReadVLong()
	if err != nil {
		return 0, err
	}
	if v == 0 {
		return positiveIntNoOutput, nil
	}
	return v, nil
}

// SkipOutput implements Outputs.
func (p *PositiveIntOutputsImpl) SkipOutput(in store.DataInput) error {
	_, err := p.Read(in)
	return err
}

// ReadFinalOutput implements Outputs.
func (p *PositiveIntOutputsImpl) ReadFinalOutput(in store.DataInput) (int64, error) {
	return p.Read(in)
}

// SkipFinalOutput implements Outputs.
func (p *PositiveIntOutputsImpl) SkipFinalOutput(in store.DataInput) error { return p.SkipOutput(in) }

// GetNoOutput implements Outputs.
func (*PositiveIntOutputsImpl) GetNoOutput() int64 { return positiveIntNoOutput }

// OutputToString implements Outputs.
func (*PositiveIntOutputsImpl) OutputToString(output int64) string {
	// Matches Long.toString in Java.
	if output == 0 {
		return "0"
	}
	neg := output < 0
	var buf [20]byte
	i := len(buf)
	u := uint64(output)
	if neg {
		u = uint64(-output)
	}
	for u > 0 {
		i--
		buf[i] = byte('0' + u%10)
		u /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// Merge implements Outputs. PositiveIntOutputs.merge is not
// overridden.
func (*PositiveIntOutputsImpl) Merge(_, _ int64) (int64, error) { return 0, ErrUnsupportedMerge }

// RAMBytesUsed implements Outputs. Long is 24 bytes in the JVM; we
// approximate with 8 (raw int64) since the Go value is unboxed.
func (*PositiveIntOutputsImpl) RAMBytesUsed(_ int64) int64 { return 8 }

// String returns "PositiveIntOutputs".
func (*PositiveIntOutputsImpl) String() string { return "PositiveIntOutputs" }
