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

// growableInitialSize is the package-private INITIAL_SIZE constant in
// Lucene's GrowableByteArrayDataOutput.
const growableInitialSize = 1 << 8

// GrowableByteArrayDataOutput is the Go port of the package-private
// org.apache.lucene.util.fst.GrowableByteArrayDataOutput. It holds a
// single contiguous byte slice that only grows, used as a scratch
// buffer while serialising a single FST node. Writes happen in the
// natural left-to-right direction; the FSTCompiler reverses the
// buffer in place before flushing it to the underlying DataOutput.
//
// Implements store.DataOutput plus store.VariableLengthOutput so that
// Outputs implementations (which require VariableLengthOutput) can
// write into it directly.
type GrowableByteArrayDataOutput struct {
	bytes     []byte
	nextWrite int
}

// NewGrowableByteArrayDataOutput returns a fresh buffer with the
// Lucene-standard 256-byte initial capacity.
func NewGrowableByteArrayDataOutput() *GrowableByteArrayDataOutput {
	return &GrowableByteArrayDataOutput{bytes: make([]byte, growableInitialSize)}
}

// WriteByte implements store.DataOutput.
func (g *GrowableByteArrayDataOutput) WriteByte(b byte) error {
	g.ensureCapacity(1)
	g.bytes[g.nextWrite] = b
	g.nextWrite++
	return nil
}

// WriteBytes implements store.DataOutput.
func (g *GrowableByteArrayDataOutput) WriteBytes(b []byte) error {
	return g.WriteBytesN(b, len(b))
}

// WriteBytesN implements store.DataOutput.
func (g *GrowableByteArrayDataOutput) WriteBytesN(b []byte, n int) error {
	if n < 0 || n > len(b) {
		return fmt.Errorf("GrowableByteArrayDataOutput.WriteBytesN: invalid n=%d (len=%d)", n, len(b))
	}
	if n == 0 {
		return nil
	}
	g.ensureCapacity(n)
	copy(g.bytes[g.nextWrite:], b[:n])
	g.nextWrite += n
	return nil
}

// WriteShort writes a 16-bit value. Matches the little-endian encoding
// used by store.ByteArrayDataOutput so that callers that round-trip
// through GrowableByteArrayDataOutput see the same byte layout.
func (g *GrowableByteArrayDataOutput) WriteShort(v int16) error {
	if err := g.WriteByte(byte(v)); err != nil {
		return err
	}
	return g.WriteByte(byte(v >> 8))
}

// WriteInt writes a 32-bit little-endian value, matching the rest of
// the package.
func (g *GrowableByteArrayDataOutput) WriteInt(v int32) error {
	if err := g.WriteByte(byte(v)); err != nil {
		return err
	}
	if err := g.WriteByte(byte(v >> 8)); err != nil {
		return err
	}
	if err := g.WriteByte(byte(v >> 16)); err != nil {
		return err
	}
	return g.WriteByte(byte(v >> 24))
}

// WriteLong writes a 64-bit little-endian value.
func (g *GrowableByteArrayDataOutput) WriteLong(v int64) error {
	for i := 0; i < 8; i++ {
		if err := g.WriteByte(byte(v >> (8 * i))); err != nil {
			return err
		}
	}
	return nil
}

// WriteString implements store.DataOutput by emitting a VInt length
// followed by the UTF-8 bytes.
func (g *GrowableByteArrayDataOutput) WriteString(s string) error {
	return store.WriteString(g, s)
}

// WriteVInt implements store.VariableLengthOutput.
func (g *GrowableByteArrayDataOutput) WriteVInt(v int32) error {
	for v&^int32(0x7F) != 0 {
		if err := g.WriteByte(byte((v & 0x7F) | 0x80)); err != nil {
			return err
		}
		v = int32(uint32(v) >> 7)
	}
	return g.WriteByte(byte(v))
}

// WriteVLong implements store.VariableLengthOutput.
func (g *GrowableByteArrayDataOutput) WriteVLong(v int64) error {
	for v&^int64(0x7F) != 0 {
		if err := g.WriteByte(byte((v & 0x7F) | 0x80)); err != nil {
			return err
		}
		v = int64(uint64(v) >> 7)
	}
	return g.WriteByte(byte(v))
}

// GetPosition returns the next-write offset.
func (g *GrowableByteArrayDataOutput) GetPosition() int { return g.nextWrite }

// SetPosition seeks the write head; the buffer is grown if the new
// position is beyond the current capacity. Matches the Java method of
// the same name.
func (g *GrowableByteArrayDataOutput) SetPosition(newLen int) {
	if newLen < 0 {
		panic(fmt.Sprintf("GrowableByteArrayDataOutput.SetPosition: negative newLen %d", newLen))
	}
	if newLen > g.nextWrite {
		g.ensureCapacityTo(newLen)
	}
	g.nextWrite = newLen
}

// GetBytes returns the backing slice. The slice is mutable; callers
// should not retain it across further writes because growth may
// replace the storage.
func (g *GrowableByteArrayDataOutput) GetBytes() []byte { return g.bytes }

// WriteTo writes the populated prefix [0, nextWrite) to out.
func (g *GrowableByteArrayDataOutput) WriteTo(out store.DataOutput) error {
	return out.WriteBytesN(g.bytes, g.nextWrite)
}

// WriteToBytes copies bytes from this buffer into dest, mirroring the
// 4-arg writeTo helper used by FSTCompiler when rearranging arcs.
func (g *GrowableByteArrayDataOutput) WriteToBytes(srcOffset int, dest []byte, destOffset, length int) {
	if srcOffset+length > g.nextWrite {
		panic(fmt.Sprintf(
			"GrowableByteArrayDataOutput.WriteToBytes: srcOffset(%d)+length(%d) > nextWrite(%d)",
			srcOffset, length, g.nextWrite,
		))
	}
	copy(dest[destOffset:destOffset+length], g.bytes[srcOffset:srcOffset+length])
}

// RAMBytesUsed approximates the heap footprint, matching
// GrowableByteArrayDataOutput.ramBytesUsed.
func (g *GrowableByteArrayDataOutput) RAMBytesUsed() int64 {
	const base = 32 // shallow size approximation
	return int64(base + cap(g.bytes))
}

// ensureCapacity makes sure at least capacityToWrite more bytes can be
// appended.
func (g *GrowableByteArrayDataOutput) ensureCapacity(capacityToWrite int) {
	if capacityToWrite <= 0 {
		return
	}
	g.ensureCapacityTo(g.nextWrite + capacityToWrite)
}

// ensureCapacityTo grows the backing slice so that its length is at
// least minLen, using the standard Lucene over-allocation factor.
func (g *GrowableByteArrayDataOutput) ensureCapacityTo(minLen int) {
	if minLen <= len(g.bytes) {
		return
	}
	newCap := util.Oversize(minLen, 1)
	if newCap < minLen {
		newCap = minLen
	}
	next := make([]byte, newCap)
	copy(next, g.bytes[:g.nextWrite])
	g.bytes = next
}

// Compile-time interface checks.
var (
	_ store.DataOutput           = (*GrowableByteArrayDataOutput)(nil)
	_ store.VariableLengthOutput = (*GrowableByteArrayDataOutput)(nil)
)
