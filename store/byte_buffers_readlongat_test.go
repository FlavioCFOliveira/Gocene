// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"encoding/binary"
	"testing"
)

// TestByteBuffersIndexInput_ReadLongAt_LittleEndian is the regression test for
// rmp #4735. The RandomAccessInput contract (and Lucene 10.4.0's
// ByteBuffersDataInput) is little-endian, and ByteBuffersIndexInput.ReadLong is
// little-endian — but ReadLongAt was implemented as big-endian, so random-access
// readers (doc-values jump table, lucene103 trie, packed DirectReader) decoded
// garbage from a ByteBuffersDirectory-backed input. ReadLongAt must read
// little-endian and agree with the sequential ReadLong.
func TestByteBuffersIndexInput_ReadLongAt_LittleEndian(t *testing.T) {
	dir := NewByteBuffersDirectory()
	defer dir.Close()

	const want = int64(0x0102030405060708)
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, uint64(want))

	out, err := dir.CreateOutput("longat", IOContext{})
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	if err := out.WriteBytes(buf); err != nil {
		t.Fatalf("WriteBytes: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	in, err := dir.OpenInput("longat", IOContext{})
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	defer in.Close()

	bbi, ok := in.(*ByteBuffersIndexInput)
	if !ok {
		t.Fatalf("OpenInput returned %T, want *ByteBuffersIndexInput", in)
	}

	got, err := bbi.ReadLongAt(0)
	if err != nil {
		t.Fatalf("ReadLongAt(0): %v", err)
	}
	if got != want {
		t.Errorf("ReadLongAt(0) = %#016x, want %#016x (must be little-endian)", got, want)
	}

	// ReadLongAt must agree with the sequential little-endian ReadLong.
	seq, err := in.ReadLong()
	if err != nil {
		t.Fatalf("ReadLong: %v", err)
	}
	if seq != want {
		t.Errorf("ReadLong = %#016x, want %#016x", seq, want)
	}
	if seq != got {
		t.Errorf("ReadLong (%#016x) and ReadLongAt (%#016x) disagree on endianness", seq, got)
	}
}
