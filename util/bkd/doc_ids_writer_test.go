// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package bkd

import (
	"fmt"
	"math/rand/v2"
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// Mirrors lucene/core/src/test/org/apache/lucene/util/bkd/TestDocIdsWriter.java
// from Apache Lucene 10.4.0. Each Java test peer is ported one-for-one
// with the same structure: random docIDs are generated and the
// encode/decode round-trip is verified for byte length AND value
// content under both BKDVersionMetaFile (legacy scalar BPV24) and
// BKDVersionCurrent (vectorised BPV24+BPV21).

// versions enumerates the BKD versions exercised by the test peer.
var versions = []int{BKDVersionMetaFile, BKDVersionCurrent}

// captureVisitor is a minimal DocIDVisitor that appends each docID
// it is shown to an in-memory slice. Mirrors the inline anonymous
// visitor used by the Java test peer.
type captureVisitor struct {
	docIDs []int32
}

func (v *captureVisitor) Visit(docID int) error {
	v.docIDs = append(v.docIDs, int32(docID))
	return nil
}

// roundTrip writes ints through a fresh DocIdsWriter, reads them back
// twice (once via ReadInts, once via ReadIntsVisitor), and verifies
// both reads produced exactly the original sequence at exactly the
// same byte offset as the writer left the cursor.
func roundTrip(t *testing.T, ints []int32, version int) {
	t.Helper()

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	name := "tmp"
	docIdsWriter := NewDocIdsWriter(len(ints), version)

	var fileLen int64
	{
		out, err := dir.CreateOutput(name, store.IOContextWrite)
		if err != nil {
			t.Fatalf("CreateOutput: %v", err)
		}
		if err := docIdsWriter.WriteDocIds(ints, 0, len(ints), out); err != nil {
			t.Fatalf("WriteDocIds: %v", err)
		}
		fileLen = out.GetFilePointer()
		if err := out.Close(); err != nil {
			t.Fatalf("Close output: %v", err)
		}
	}

	// First read: ReadInts.
	{
		in, err := dir.OpenInput(name, store.IOContextReadOnce)
		if err != nil {
			t.Fatalf("OpenInput: %v", err)
		}
		read := make([]int32, len(ints))
		if err := docIdsWriter.ReadInts(in, len(ints), read); err != nil {
			t.Fatalf("ReadInts: %v", err)
		}
		if got, want := in.GetFilePointer(), fileLen; got != want {
			t.Fatalf("ReadInts file pointer: got %d want %d", got, want)
		}
		if !int32SlicesEqual(ints, read) {
			t.Fatalf("ReadInts round-trip mismatch:\n  want=%v\n  got =%v", ints, read)
		}
		if err := in.Close(); err != nil {
			t.Fatalf("Close input: %v", err)
		}
	}

	// Second read: ReadIntsVisitor.
	{
		in, err := dir.OpenInput(name, store.IOContextReadOnce)
		if err != nil {
			t.Fatalf("OpenInput: %v", err)
		}
		visitor := &captureVisitor{docIDs: make([]int32, 0, len(ints))}
		buffer := make([]int32, len(ints))
		if err := docIdsWriter.ReadIntsVisitor(in, len(ints), visitor, buffer); err != nil {
			t.Fatalf("ReadIntsVisitor: %v", err)
		}
		if got, want := in.GetFilePointer(), fileLen; got != want {
			t.Fatalf("ReadIntsVisitor file pointer: got %d want %d", got, want)
		}
		if !int32SlicesEqual(ints, visitor.docIDs) {
			t.Fatalf("ReadIntsVisitor round-trip mismatch:\n  want=%v\n  got =%v", ints, visitor.docIDs)
		}
		if err := in.Close(); err != nil {
			t.Fatalf("Close input: %v", err)
		}
	}

	if err := dir.DeleteFile(name); err != nil {
		t.Fatalf("DeleteFile: %v", err)
	}
}

func int32SlicesEqual(a, b []int32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestDocIdsWriter_Random mirrors TestDocIdsWriter.testRandom: random
// docIDs with random bit-width, sometimes at the default leaf-block
// size (which tickles the JIT-specialised decoders in the source).
func TestDocIdsWriter_Random(t *testing.T) {
	r := rand.New(rand.NewPCG(101, 202))

	const numIters = 100
	for iter := 0; iter < numIters; iter++ {
		var count int
		if r.IntN(2) == 0 {
			count = 1 + r.IntN(5000)
		} else {
			count = DefaultMaxPointsInLeafNode
		}
		bpv := randIntInRange(r, 1, 31)
		docIDs := make([]int32, count)
		mask := int32((uint32(1) << uint(bpv)) - 1)
		for i := range docIDs {
			docIDs[i] = int32(r.Uint32()) & mask
		}
		version := versions[r.IntN(len(versions))]
		t.Run(fmt.Sprintf("iter%d/count=%d/bpv=%d/v=%d", iter, count, bpv, version), func(t *testing.T) {
			roundTrip(t, docIDs, version)
		})
	}
}

// TestDocIdsWriter_Sorted mirrors TestDocIdsWriter.testSorted.
func TestDocIdsWriter_Sorted(t *testing.T) {
	r := rand.New(rand.NewPCG(303, 404))

	const numIters = 100
	for iter := 0; iter < numIters; iter++ {
		count := 1 + r.IntN(5000)
		bpv := randIntInRange(r, 1, 31)
		docIDs := make([]int32, count)
		mask := int32((uint32(1) << uint(bpv)) - 1)
		for i := range docIDs {
			docIDs[i] = int32(r.Uint32()) & mask
		}
		sort.Slice(docIDs, func(i, j int) bool { return docIDs[i] < docIDs[j] })
		version := versions[r.IntN(len(versions))]
		t.Run(fmt.Sprintf("iter%d/count=%d/bpv=%d/v=%d", iter, count, bpv, version), func(t *testing.T) {
			roundTrip(t, docIDs, version)
		})
	}
}

// TestDocIdsWriter_Cluster mirrors TestDocIdsWriter.testCluster: low-
// cardinality docIDs scattered around a base offset. Exercises the
// DELTA_BPV_16 encoding heavily.
func TestDocIdsWriter_Cluster(t *testing.T) {
	r := rand.New(rand.NewPCG(505, 606))

	const numIters = 100
	for iter := 0; iter < numIters; iter++ {
		var count int
		if r.IntN(2) == 0 {
			count = 1 + r.IntN(5000)
		} else {
			count = DefaultMaxPointsInLeafNode
		}
		minBase := r.IntN(1000)
		bpv := randIntInRange(r, 1, 16)
		docIDs := make([]int32, count)
		mask := int32((uint32(1) << uint(bpv)) - 1)
		for i := range docIDs {
			docIDs[i] = int32(minBase) + (int32(r.Uint32()) & mask)
		}
		version := versions[r.IntN(len(versions))]
		t.Run(fmt.Sprintf("iter%d/count=%d/min=%d/bpv=%d/v=%d", iter, count, minBase, bpv, version), func(t *testing.T) {
			roundTrip(t, docIDs, version)
		})
	}
}

// TestDocIdsWriter_BitSet mirrors TestDocIdsWriter.testBitSet: unique
// sorted ids in a small range, designed to trigger the BITSET_IDS
// encoding.
func TestDocIdsWriter_BitSet(t *testing.T) {
	r := rand.New(rand.NewPCG(707, 808))

	const numIters = 100
	for iter := 0; iter < numIters; iter++ {
		size := 1 + r.IntN(5000)
		small := r.IntN(1000)
		set := make(map[int32]struct{}, size)
		for len(set) < size {
			set[int32(small)+int32(r.IntN(size*16))] = struct{}{}
		}
		docIDs := make([]int32, 0, size)
		for id := range set {
			docIDs = append(docIDs, id)
		}
		sort.Slice(docIDs, func(i, j int) bool { return docIDs[i] < docIDs[j] })
		t.Run(fmt.Sprintf("iter%d/size=%d/small=%d", iter, size, small), func(t *testing.T) {
			// BitSet encoding is version-independent in DocIdsWriter; we
			// still exercise both to be sure no version-conditional
			// branch silently flips.
			roundTrip(t, docIDs, versions[r.IntN(len(versions))])
		})
	}
}

// TestDocIdsWriter_ContinuousIds mirrors
// TestDocIdsWriter.testContinuousIds: sequential ids starting from a
// random offset, triggering the CONTINUOUS_IDS encoding.
func TestDocIdsWriter_ContinuousIds(t *testing.T) {
	r := rand.New(rand.NewPCG(909, 1010))

	const numIters = 100
	for iter := 0; iter < numIters; iter++ {
		size := 1 + r.IntN(5000)
		start := r.IntN(1000000)
		docIDs := make([]int32, size)
		for i := range docIDs {
			docIDs[i] = int32(start + i)
		}
		t.Run(fmt.Sprintf("iter%d/size=%d/start=%d", iter, size, start), func(t *testing.T) {
			roundTrip(t, docIDs, versions[r.IntN(len(versions))])
		})
	}
}

// TestDocIdsWriter_DefaultMaxLeafBlockSize_AllEncodings forces all
// encodings to run at the default-leaf-block size that the Lucene
// readers specialise for. This guarantees coverage of the
// `count == BKDConfig.DEFAULT_MAX_POINTS_IN_LEAF_NODE` branch in
// readDelta16/readInts21/readInts24.
func TestDocIdsWriter_DefaultMaxLeafBlockSize_AllEncodings(t *testing.T) {
	const count = DefaultMaxPointsInLeafNode

	type encoding struct {
		name    string
		gen     func() []int32
		version int
	}
	encs := []encoding{
		{
			name: "continuous",
			gen: func() []int32 {
				ids := make([]int32, count)
				for i := range ids {
					ids[i] = int32(1000 + i)
				}
				return ids
			},
			version: BKDVersionCurrent,
		},
		{
			name: "bitset",
			gen: func() []int32 {
				// Sorted, unique, max - min + 1 between count and 16*count.
				ids := make([]int32, 0, count)
				cur := int32(100)
				for len(ids) < count {
					ids = append(ids, cur)
					cur += 2
				}
				return ids
			},
			version: BKDVersionCurrent,
		},
		{
			name: "delta16",
			gen: func() []int32 {
				// Random in [10000, 10000+0xFFFF]: dense enough that
				// strictlySorted is unlikely and min2max <= 0xFFFF.
				r := rand.New(rand.NewPCG(1, 2))
				ids := make([]int32, count)
				for i := range ids {
					ids[i] = 10000 + int32(r.IntN(0xFFFF+1))
				}
				return ids
			},
			version: BKDVersionCurrent,
		},
		{
			name: "bpv21-current",
			gen: func() []int32 {
				// max <= 0x1FFFFF (2,097,151); spread to defeat continuous/bitset.
				r := rand.New(rand.NewPCG(3, 4))
				ids := make([]int32, count)
				for i := range ids {
					ids[i] = int32(r.IntN(0x1FFFFF + 1))
				}
				return ids
			},
			version: BKDVersionCurrent,
		},
		{
			name: "bpv24-current",
			gen: func() []int32 {
				// max <= 0xFFFFFF, max > 0x1FFFFF.
				r := rand.New(rand.NewPCG(5, 6))
				ids := make([]int32, count)
				for i := range ids {
					ids[i] = 0x200000 + int32(r.IntN(0xFFFFFF-0x200000))
				}
				return ids
			},
			version: BKDVersionCurrent,
		},
		{
			name: "bpv24-legacy",
			gen: func() []int32 {
				// Same distribution but legacy version forces scalar layout.
				r := rand.New(rand.NewPCG(7, 8))
				ids := make([]int32, count)
				for i := range ids {
					ids[i] = 0x200000 + int32(r.IntN(0xFFFFFF-0x200000))
				}
				return ids
			},
			version: BKDVersionMetaFile,
		},
		{
			name: "bpv32",
			gen: func() []int32 {
				// max > 0xFFFFFF — falls into BPV_32 fallback.
				r := rand.New(rand.NewPCG(9, 10))
				ids := make([]int32, count)
				for i := range ids {
					// Keep positive; mix high and low ids.
					ids[i] = 0x1000000 + int32(r.IntN(0x7EFFFFFF))
				}
				return ids
			},
			version: BKDVersionCurrent,
		},
	}

	for _, e := range encs {
		t.Run(e.name, func(t *testing.T) {
			roundTrip(t, e.gen(), e.version)
		})
	}
}

// TestDocIdsWriter_LegacyDeltaVIntDecode verifies that the
// LEGACY_DELTA_VINT (=0) marker is still decoded correctly when
// present in old segments. The writer never emits this layout, so we
// build a synthetic file by hand and decode it.
func TestDocIdsWriter_LegacyDeltaVIntDecode(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	name := "legacy"
	docIDs := []int32{5, 12, 33, 100, 250, 2000}

	out, err := dir.CreateOutput(name, store.IOContextWrite)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	if err := out.WriteByte(docIDsLegacyDelta); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	// Write deltas as VInts: first doc is the absolute id, then
	// successive deltas. Mirrors readLegacyDeltaVInts which seeds
	// doc = 0 and accumulates.
	var prev int32
	for _, id := range docIDs {
		if err := store.WriteVInt(out, id-prev); err != nil {
			t.Fatalf("write delta: %v", err)
		}
		prev = id
	}
	fileLen := out.GetFilePointer()
	if err := out.Close(); err != nil {
		t.Fatalf("Close output: %v", err)
	}

	in, err := dir.OpenInput(name, store.IOContextReadOnce)
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	defer in.Close()

	w := NewDocIdsWriter(len(docIDs), BKDVersionMetaFile)
	read := make([]int32, len(docIDs))
	if err := w.ReadInts(in, len(docIDs), read); err != nil {
		t.Fatalf("ReadInts: %v", err)
	}
	if got := in.GetFilePointer(); got != fileLen {
		t.Fatalf("file pointer: got %d want %d", got, fileLen)
	}
	if !int32SlicesEqual(docIDs, read) {
		t.Fatalf("legacy decode mismatch:\n  want=%v\n  got =%v", docIDs, read)
	}
}

// TestDocIdsWriter_UnsupportedMarkerErrors verifies that an unknown
// marker byte at the head of a leaf block surfaces an explicit error
// rather than panicking or returning garbage docIDs.
func TestDocIdsWriter_UnsupportedMarkerErrors(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	name := "garbage"
	out, err := dir.CreateOutput(name, store.IOContextWrite)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	// Write a marker that is not a valid BPV.
	if err := out.WriteByte(0x07); err != nil {
		t.Fatalf("WriteByte: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	in, err := dir.OpenInput(name, store.IOContextReadOnce)
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	defer in.Close()
	w := NewDocIdsWriter(8, BKDVersionCurrent)
	read := make([]int32, 8)
	err = w.ReadInts(in, 8, read)
	if err == nil {
		t.Fatalf("expected error for unsupported marker; got nil")
	}
}

// TestDocIdsWriter_ByteFormat_ContinuousIds asserts the exact byte
// layout for the CONTINUOUS_IDS encoding: marker byte (-2 == 0xFE),
// then a VInt holding the start docID.
func TestDocIdsWriter_ByteFormat_ContinuousIds(t *testing.T) {
	docIDs := []int32{100, 101, 102, 103, 104}
	w := NewDocIdsWriter(len(docIDs), BKDVersionCurrent)
	buf := store.NewByteBuffersDataOutput()
	if err := w.WriteDocIds(docIDs, 0, len(docIDs), buf); err != nil {
		t.Fatalf("WriteDocIds: %v", err)
	}
	got := buf.ToArrayCopy()
	want := []byte{
		docIDsContinuous,
		100, // VInt 100 = 0x64 (single byte)
	}
	if !bytesEqual(got, want) {
		t.Fatalf("byte mismatch:\n  want=%x\n  got =%x", want, got)
	}
}

// TestDocIdsWriter_ByteFormat_BPV32 asserts the exact byte layout for
// BPV_32: marker byte (32), then count little-endian int32s.
func TestDocIdsWriter_ByteFormat_BPV32(t *testing.T) {
	// Force BPV_32: max > 0xFFFFFF.
	docIDs := []int32{0x01000000, 0x02000000, 0x03000000}
	w := NewDocIdsWriter(len(docIDs), BKDVersionCurrent)
	buf := store.NewByteBuffersDataOutput()
	if err := w.WriteDocIds(docIDs, 0, len(docIDs), buf); err != nil {
		t.Fatalf("WriteDocIds: %v", err)
	}
	got := buf.ToArrayCopy()
	want := []byte{
		docIDsBPV32,
		0x00, 0x00, 0x00, 0x01, // 0x01000000 LE
		0x00, 0x00, 0x00, 0x02, // 0x02000000 LE
		0x00, 0x00, 0x00, 0x03, // 0x03000000 LE
	}
	if !bytesEqual(got, want) {
		t.Fatalf("byte mismatch:\n  want=%x\n  got =%x", want, got)
	}
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
