// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"encoding/binary"
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// writeBlocksLE persists the 64-bit blocks slice into the directory
// using little-endian byte order, matching ByteBuffersIndexInput's
// ReadLong endianness. Using IndexOutput.WriteLong would emit
// big-endian bytes that the reader would mis-decode.
func writeBlocksLE(t *testing.T, dir *store.ByteBuffersDirectory, name string, blocks []int64) {
	t.Helper()
	out, err := dir.CreateOutput(name, store.IOContext{Context: store.ContextWrite})
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	buf := make([]byte, 8)
	for _, b := range blocks {
		binary.LittleEndian.PutUint64(buf, uint64(b))
		if err := out.WriteBytes(buf); err != nil {
			t.Fatalf("WriteBytes: %v", err)
		}
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestDirectPacked64SingleBlockReader_RoundTripAgainstPacked64SingleBlock
// uses Packed64SingleBlock as the oracle: it writes the very same
// block layout the in-memory Packed64SingleBlock would, then asserts
// the on-disk reader returns identical values for every index across
// the bitsPerValue spectrum supported by FormatPackedSingleBlock.
func TestDirectPacked64SingleBlockReader_RoundTripAgainstPacked64SingleBlock(t *testing.T) {
	t.Parallel()
	const valueCount = 257 // forces a non-multiple-of-valuesPerBlock tail
	for _, bpv := range packed64SingleBlockSupported {
		bpv := bpv
		oracle := newPacked64SingleBlock(valueCount, bpv)
		r := rand.New(rand.NewSource(int64(bpv) * 1_000_003))
		mask := (uint64(1) << uint(bpv)) - 1
		want := make([]int64, valueCount)
		for i := 0; i < valueCount; i++ {
			v := int64(r.Uint64() & mask)
			oracle.Set(i, v)
			want[i] = v
		}

		dir := store.NewByteBuffersDirectory()
		name := "blocks.bin"
		writeBlocksLE(t, dir, name, oracle.blocks)

		in, err := dir.OpenInput(name, store.IOContext{Context: store.ContextRead})
		if err != nil {
			t.Fatalf("bpv=%d OpenInput: %v", bpv, err)
		}

		reader := NewDirectPacked64SingleBlockReader(bpv, valueCount, in)
		if got := reader.Size(); got != valueCount {
			t.Fatalf("bpv=%d Size: got %d want %d", bpv, got, valueCount)
		}
		if got := reader.RamBytesUsed(); got != 0 {
			t.Fatalf("bpv=%d RamBytesUsed: got %d want 0", bpv, got)
		}

		// Random-access probes, including the final index.
		for i := 0; i < valueCount; i++ {
			got := reader.Get(i)
			if got != want[i] {
				t.Fatalf("bpv=%d Get(%d): got %d want %d", bpv, i, got, want[i])
			}
			if oracleVal := oracle.Get(i); got != oracleVal {
				t.Fatalf("bpv=%d divergence from oracle at %d: reader=%d oracle=%d", bpv, i, got, oracleVal)
			}
		}

		// GetBulk falls back to the generic sequential path.
		bulk := make([]int64, valueCount)
		n := reader.GetBulk(0, bulk, 0, valueCount)
		if n != valueCount {
			t.Fatalf("bpv=%d GetBulk n: got %d want %d", bpv, n, valueCount)
		}
		for i, v := range bulk {
			if v != want[i] {
				t.Fatalf("bpv=%d GetBulk[%d]: got %d want %d", bpv, i, v, want[i])
			}
		}

		if err := in.Close(); err != nil {
			t.Fatalf("bpv=%d Close: %v", bpv, err)
		}
	}
}

// TestDirectPacked64SingleBlockReader_HonoursStartPointer verifies
// that the reader anchors its block offsets at the file pointer
// captured at construction time, not at zero.
func TestDirectPacked64SingleBlockReader_HonoursStartPointer(t *testing.T) {
	t.Parallel()
	const bpv = 8
	const valueCount = 16

	dir := store.NewByteBuffersDirectory()
	out, err := dir.CreateOutput("prefixed.bin", store.IOContext{Context: store.ContextWrite})
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	// Prefix bytes the reader must skip past.
	const prefix = 13
	if err := out.WriteBytes(make([]byte, prefix)); err != nil {
		t.Fatalf("WriteBytes prefix: %v", err)
	}
	oracle := newPacked64SingleBlock(valueCount, bpv)
	for i := 0; i < valueCount; i++ {
		oracle.Set(i, int64(i*7+1))
	}
	buf := make([]byte, 8)
	for _, b := range oracle.blocks {
		binary.LittleEndian.PutUint64(buf, uint64(b))
		if err := out.WriteBytes(buf); err != nil {
			t.Fatalf("WriteBytes block: %v", err)
		}
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	in, err := dir.OpenInput("prefixed.bin", store.IOContext{Context: store.ContextRead})
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	if err := in.SetPosition(prefix); err != nil {
		t.Fatalf("SetPosition: %v", err)
	}

	reader := NewDirectPacked64SingleBlockReader(bpv, valueCount, in)
	for i := 0; i < valueCount; i++ {
		if got, want := reader.Get(i), oracle.Get(i); got != want {
			t.Fatalf("Get(%d): got %d want %d", i, got, want)
		}
	}
	if err := in.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
