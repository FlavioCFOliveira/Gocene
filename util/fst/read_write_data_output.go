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
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ReadWriteDataOutput is the Go port of the package-private
// org.apache.lucene.util.fst.ReadWriteDataOutput. It is an in-memory
// DataOutput that, once frozen, also implements FSTReader so the FST
// can be traversed without an intermediate serialise/deserialise step.
//
// Implementation note: Lucene's reference is backed by
// ByteBuffersDataOutput (multi-page). The Go port uses a single
// contiguous byte slice that grows via util.Oversize; this matches
// the "single buffer" fast-path branch in Lucene's getReverseBytesReader
// and yields the same byte stream. Multi-page support is intentionally
// deferred: Go can address slices well beyond the practical FST sizes
// that motivated paging in the JVM, and reusing the existing
// ReverseBytesReader keeps the rest of the FST package unchanged.
type ReadWriteDataOutput struct {
	bytes  []byte
	pos    int
	frozen bool
}

// NewReadWriteDataOutput allocates a ReadWriteDataOutput. blockBits is
// kept for API parity with Lucene's constructor; it seeds the initial
// capacity (1 << blockBits) but does not otherwise influence layout
// because the Go port keeps the bytes in a single contiguous slice.
func NewReadWriteDataOutput(blockBits int) *ReadWriteDataOutput {
	if blockBits < 1 || blockBits > 30 {
		panic(fmt.Sprintf("ReadWriteDataOutput: blockBits must be 1..30 (got %d)", blockBits))
	}
	return &ReadWriteDataOutput{bytes: make([]byte, 0, 1<<blockBits)}
}

// WriteByte implements store.DataOutput.
func (rw *ReadWriteDataOutput) WriteByte(b byte) error {
	if rw.frozen {
		return errFrozen
	}
	rw.ensureCapacity(1)
	rw.bytes = rw.bytes[:rw.pos+1]
	rw.bytes[rw.pos] = b
	rw.pos++
	return nil
}

// WriteBytes implements store.DataOutput.
func (rw *ReadWriteDataOutput) WriteBytes(b []byte) error { return rw.WriteBytesN(b, len(b)) }

// WriteBytesN implements store.DataOutput.
func (rw *ReadWriteDataOutput) WriteBytesN(b []byte, n int) error {
	if rw.frozen {
		return errFrozen
	}
	if n < 0 || n > len(b) {
		return fmt.Errorf("ReadWriteDataOutput.WriteBytesN: invalid n=%d (len=%d)", n, len(b))
	}
	if n == 0 {
		return nil
	}
	rw.ensureCapacity(n)
	rw.bytes = rw.bytes[:rw.pos+n]
	copy(rw.bytes[rw.pos:], b[:n])
	rw.pos += n
	return nil
}

// WriteShort emits a little-endian int16, matching the canonical
// post-versionLittleEndian layout used elsewhere in the package.
func (rw *ReadWriteDataOutput) WriteShort(v int16) error {
	if err := rw.WriteByte(byte(v)); err != nil {
		return err
	}
	return rw.WriteByte(byte(v >> 8))
}

// WriteInt emits a little-endian int32.
func (rw *ReadWriteDataOutput) WriteInt(v int32) error {
	for i := 0; i < 4; i++ {
		if err := rw.WriteByte(byte(v >> (8 * i))); err != nil {
			return err
		}
	}
	return nil
}

// WriteLong emits a little-endian int64.
func (rw *ReadWriteDataOutput) WriteLong(v int64) error {
	for i := 0; i < 8; i++ {
		if err := rw.WriteByte(byte(v >> (8 * i))); err != nil {
			return err
		}
	}
	return nil
}

// WriteString implements store.DataOutput.
func (rw *ReadWriteDataOutput) WriteString(s string) error { return store.WriteString(rw, s) }

// WriteVInt implements store.VariableLengthOutput.
func (rw *ReadWriteDataOutput) WriteVInt(v int32) error {
	for v&^int32(0x7F) != 0 {
		if err := rw.WriteByte(byte((v & 0x7F) | 0x80)); err != nil {
			return err
		}
		v = int32(uint32(v) >> 7)
	}
	return rw.WriteByte(byte(v))
}

// WriteVLong implements store.VariableLengthOutput.
func (rw *ReadWriteDataOutput) WriteVLong(v int64) error {
	for v&^int64(0x7F) != 0 {
		if err := rw.WriteByte(byte((v & 0x7F) | 0x80)); err != nil {
			return err
		}
		v = int64(uint64(v) >> 7)
	}
	return rw.WriteByte(byte(v))
}

// Freeze commits the buffer for read-only access. After Freeze is
// called, further writes return errFrozen. Mirrors
// ReadWriteDataOutput.freeze.
func (rw *ReadWriteDataOutput) Freeze() { rw.frozen = true }

// IsFrozen reports whether Freeze has been called.
func (rw *ReadWriteDataOutput) IsFrozen() bool { return rw.frozen }

// Position returns the current write position. Useful for tests that
// inspect the running byte count.
func (rw *ReadWriteDataOutput) Position() int { return rw.pos }

// Bytes returns the populated prefix of the backing slice. The slice
// aliases the internal storage; callers must not mutate it.
func (rw *ReadWriteDataOutput) Bytes() []byte { return rw.bytes[:rw.pos] }

// GetReverseBytesReader implements FSTReader. The reader walks the
// frozen byte slice in reverse, positioned at the last written byte.
func (rw *ReadWriteDataOutput) GetReverseBytesReader() BytesReader {
	if !rw.frozen {
		panic("ReadWriteDataOutput.GetReverseBytesReader: must call Freeze first")
	}
	return NewReverseBytesReader(rw.bytes[:rw.pos])
}

// WriteTo implements FSTReader by copying the populated bytes to out.
func (rw *ReadWriteDataOutput) WriteTo(out store.DataOutput) error {
	return out.WriteBytesN(rw.bytes, rw.pos)
}

// RAMBytesUsed implements FSTReader (and Accountable).
func (rw *ReadWriteDataOutput) RAMBytesUsed() int64 {
	const base = 48
	return int64(base + cap(rw.bytes))
}

// ensureCapacity grows the backing slice so that at least add more
// bytes can be written. The capacity grows exponentially via
// util.Oversize.
func (rw *ReadWriteDataOutput) ensureCapacity(add int) {
	required := rw.pos + add
	if required <= cap(rw.bytes) {
		return
	}
	newCap := util.Oversize(required, 1)
	if newCap < required {
		newCap = required
	}
	next := make([]byte, len(rw.bytes), newCap)
	copy(next, rw.bytes)
	rw.bytes = next
}

// errFrozen is returned from any write call after Freeze.
var errFrozen = errors.New("fst: ReadWriteDataOutput is frozen; no further writes allowed")

// Compile-time interface checks.
var (
	_ store.DataOutput           = (*ReadWriteDataOutput)(nil)
	_ store.VariableLengthOutput = (*ReadWriteDataOutput)(nil)
	_ FSTReader                  = (*ReadWriteDataOutput)(nil)
)
