// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"math/rand/v2"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// This file is a faithful Go port of
// org.apache.lucene.codecs.lucene90.compressing.TestStoredFieldsInt
// (Apache Lucene 10.4.0). The Java reference round-trips a randomised int[]
// through StoredFieldsInts.writeInts / readInts via a Directory's
// IndexOutput / IndexInput pair, optionally appending an 8-byte garbage tail
// to assert that the writer reports its precise length back via
// IndexOutput.getFilePointer(), and that the reader stops at exactly that
// offset.
//
// The Go test mirrors that contract one-for-one:
//   - Same two top-level cases (Random, AllEquals);
//   - Same intermediate helper (testStoredFieldsIntRoundTrip) modelled after
//     the Java `test(Directory, int[])`;
//   - Same per-iteration shape: random length in [1, 5000], random bpv in
//     [1, 31], values drawn uniformly from [0, (1<<bpv)-1];
//   - Garbage-long suffix is emitted on a 50/50 coin flip;
//   - File-pointer equality is asserted before and after the reader runs.
//
// Determinism note: Lucene's LuceneTestCase uses a per-run random seed wired
// through atLeast(int) to scale iteration counts (typically NIGHTLY-aware).
// The Go port pins a PCG seed per test so the run is reproducible and CI is
// not dependent on -race / wall clock; iteration counts use a fixed
// equivalent of atLeast(100).
//
// Directory choice: the Java reference uses `newDirectory()` from
// LuceneTestCase which yields a random Directory implementation per run. The
// Gocene `ByteBuffersDirectory` currently has a known endianness asymmetry
// (BE writes, LE reads) on its multi-byte primitives, so it cannot honour a
// round-trip contract through IndexOutput/IndexInput. We pin
// `SimpleFSDirectory` here — it shares the BE-on-both-sides invariant of
// Lucene's own MMapDirectory/NIOFSDirectory — backed by a t.TempDir.
const testStoredFieldsIntsAtLeast = 100

// testStoredFieldsIntRoundTrip encodes `ints` to a fresh "tmp" file in `dir`
// via StoredFieldsInts.WriteInts (Gocene: WriteStoredFieldsInts), optionally
// writes an 8-byte garbage suffix, then reads the values back into an
// offset-shifted long[] (Gocene: []int64) and asserts equality with the
// source plus that the input cursor stopped at the same file pointer the
// writer reported. Mirrors the private `test(Directory, int[])` in
// TestStoredFieldsInt.java line by line.
func testStoredFieldsIntRoundTrip(t *testing.T, dir store.Directory, rng *rand.Rand, ints []int32) {
	t.Helper()

	out, err := dir.CreateOutput("tmp", store.IOContextDefault)
	if err != nil {
		t.Fatalf("create output: %v", err)
	}
	if err := WriteStoredFieldsInts(ints, 0, len(ints), out); err != nil {
		_ = out.Close()
		t.Fatalf("WriteStoredFieldsInts: %v", err)
	}
	wantLen := out.GetFilePointer()
	// 50/50 garbage tail — mirrors `if (random().nextBoolean()) out.writeLong(0);`
	// in the Java reference. The point is to prove the reader does not run
	// past `wantLen`.
	if rng.IntN(2) == 0 {
		if err := out.WriteLong(0); err != nil {
			_ = out.Close()
			t.Fatalf("write garbage long: %v", err)
		}
	}
	if err := out.Close(); err != nil {
		t.Fatalf("close output: %v", err)
	}

	in, err := dir.OpenInput("tmp", store.IOContextReadOnce)
	if err != nil {
		t.Fatalf("open input: %v", err)
	}
	offset := rng.IntN(5)
	read := make([]int64, len(ints)+offset)
	if err := ReadStoredFieldsInts(in, len(ints), read, offset); err != nil {
		_ = in.Close()
		t.Fatalf("ReadStoredFieldsInts: %v", err)
	}
	for i, v := range ints {
		got := int32(read[offset+i])
		if got != v {
			_ = in.Close()
			t.Fatalf("value mismatch at %d: got %d want %d", i, got, v)
		}
	}
	if gotLen := in.GetFilePointer(); gotLen != wantLen {
		_ = in.Close()
		t.Fatalf("file pointer mismatch after read: got %d want %d", gotLen, wantLen)
	}
	if err := in.Close(); err != nil {
		t.Fatalf("close input: %v", err)
	}
	if err := dir.DeleteFile("tmp"); err != nil {
		t.Fatalf("delete tmp: %v", err)
	}
}

// TestStoredFieldsInt_Random is the Go port of
// TestStoredFieldsInt.testRandom: across many iterations, draw a random
// length in [1, 5000] and a random bpv in [1, 31], fill the array with
// uniformly random values in [0, (1<<bpv)-1], and round-trip through a
// Directory. The randomisation is enough to cover all three packing widths
// (8/16/32 bpv) plus the all-equal short circuit.
func TestStoredFieldsInt_Random(t *testing.T) {
	t.Parallel()

	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	t.Cleanup(func() { _ = dir.Close() })

	rng := rand.New(rand.NewPCG(0xC0DE_F00D, 0xDEAD_BEEF))
	for iter := 0; iter < testStoredFieldsIntsAtLeast; iter++ {
		length := rng.IntN(5000) + 1
		bpv := rng.IntN(31) + 1 // [1, 31]
		bound := uint32(1) << bpv
		values := make([]int32, length)
		for i := range values {
			// Java's TestUtil.nextInt(random(), 0, (1 << bpv) - 1) is inclusive on
			// the upper bound; rng.UintN(bound) yields [0, bound). When bpv==31
			// the bound is exactly 1<<31 which fits in a uint32 — using uint64
			// for the cast avoids any signed-arithmetic ambiguity.
			values[i] = int32(rng.Uint32N(bound))
		}
		testStoredFieldsIntRoundTrip(t, dir, rng, values)
	}
}

// TestStoredFieldsInt_AllEquals is the Go port of
// TestStoredFieldsInt.testAllEquals: pick a single random value in
// [0, (1<<bpv)-1], broadcast it across a random-length array, and round-trip.
// This exercises the all-equal short-circuit branch in the writer and the
// VInt-replicate branch in the reader.
func TestStoredFieldsInt_AllEquals(t *testing.T) {
	t.Parallel()

	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	t.Cleanup(func() { _ = dir.Close() })

	rng := rand.New(rand.NewPCG(0xCAFE_BABE, 0x1234_5678))
	docIDs := make([]int32, rng.IntN(5000)+1)
	bpv := rng.IntN(31) + 1
	bound := uint32(1) << bpv
	fill := int32(rng.Uint32N(bound))
	for i := range docIDs {
		docIDs[i] = fill
	}
	testStoredFieldsIntRoundTrip(t, dir, rng, docIDs)
}
