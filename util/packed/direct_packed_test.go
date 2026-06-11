// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestDirectPacked is the Gocene port of Apache Lucene 10.4.0's
// TestDirectPacked (org.apache.lucene.util.packed). The Java original
// exercises DirectWriter/DirectReader round-trips through a Directory;
// here the same coverage is achieved against store.ByteArrayDataOutput
// + byteSliceRandomAccess (defined in direct_reader_test.go) because
// Gocene does not yet expose RandomAccessSlice on its Directory API.
//
// Lucene's DirectReader.getMergeInstance variant is not (yet) exposed
// by Gocene; the merge branch therefore reduces to the same code path
// as the non-merge branch (documented per-subtest below) while still
// preserving the bitsPerValue spectrum.

// TestDirectPackedSimple is the Go port of TestDirectPacked#testSimple.
// It writes a handful of small values at bpv=2 and verifies positional
// reads via DirectReader.
func TestDirectPackedSimple(t *testing.T) {
	t.Parallel()
	bitsPerValue := DirectWriterBitsRequired(2)
	out := store.NewByteArrayDataOutput(8)
	w, err := GetDirectWriter(out, 5, bitsPerValue)
	if err != nil {
		t.Fatalf("GetDirectWriter: %v", err)
	}
	for _, v := range []int64{1, 0, 2, 1, 2} {
		if err := w.Add(v); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	in := &byteSliceRandomAccess{data: out.GetBytes()}
	r, err := GetDirectReader(in, bitsPerValue)
	if err != nil {
		t.Fatalf("GetDirectReader: %v", err)
	}
	want := []int64{1, 0, 2, 1, 2}
	for i, v := range want {
		got, err := r.Get(int64(i))
		if err != nil {
			t.Fatalf("[%d]: unexpected error: %v", i, err)
		}
		if got != v {
			t.Fatalf("[%d]: got %d want %d", i, got, v)
		}
	}
}

// TestDirectPackedNotEnoughValues is the Go port of
// TestDirectPacked#testNotEnoughValues. Lucene throws
// IllegalStateException with "Wrong number of values added..."; Gocene
// returns an equivalent error from Finish.
func TestDirectPackedNotEnoughValues(t *testing.T) {
	t.Parallel()
	bitsPerValue := DirectWriterBitsRequired(2)
	out := store.NewByteArrayDataOutput(8)
	w, err := GetDirectWriter(out, 5, bitsPerValue)
	if err != nil {
		t.Fatalf("GetDirectWriter: %v", err)
	}
	for _, v := range []int64{1, 0, 2, 1} {
		if err := w.Add(v); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}
	err = w.Finish()
	if err == nil {
		t.Fatal("expected Finish to fail when fewer values were added")
	}
	if !strings.Contains(err.Error(), "wrong number of values added") {
		t.Fatalf("expected 'wrong number of values added' in error, got: %v", err)
	}
}

// TestDirectPackedRandom is the Go port of TestDirectPacked#testRandom.
func TestDirectPackedRandom(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(1))
	for bpv := 1; bpv <= 64; bpv++ {
		doTestDirectPackedBpv(t, r, bpv, 0, false)
	}
}

// TestDirectPackedRandomWithOffset is the Go port of
// TestDirectPacked#testRandomWithOffset.
func TestDirectPackedRandomWithOffset(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(2))
	offset := int64(r.Intn(100) + 1)
	for bpv := 1; bpv <= 64; bpv++ {
		doTestDirectPackedBpv(t, r, bpv, offset, false)
	}
}

// TestDirectPackedRandomMerge is the Go port of
// TestDirectPacked#testRandomMerge.
//
// Gocene does not (yet) expose DirectReader.GetMergeInstance, so the
// merge=true branch resolves to the same reader path; the test name is
// kept aligned with the upstream suite to make a future port of the
// merge variant a drop-in replacement.
func TestDirectPackedRandomMerge(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(3))
	for bpv := 1; bpv <= 64; bpv++ {
		doTestDirectPackedBpv(t, r, bpv, 0, true)
	}
}

// TestDirectPackedRandomMergeWithOffset is the Go port of
// TestDirectPacked#testRandomMergeWithOffset.
func TestDirectPackedRandomMergeWithOffset(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(4))
	offset := int64(r.Intn(100) + 1)
	for bpv := 1; bpv <= 64; bpv++ {
		doTestDirectPackedBpv(t, r, bpv, offset, true)
	}
}

// doTestDirectPackedBpv is the Go counterpart of
// TestDirectPacked#doTestBpv. The merge flag is accepted for parity
// with the Lucene source; see TestDirectPackedRandomMerge for the
// rationale.
func doTestDirectPackedBpv(t *testing.T, r *rand.Rand, bpv int, offset int64, merge bool) {
	t.Helper()
	const numIters = 10
	for i := 0; i < numIters; i++ {
		original := randomLongsForBpv(r, bpv)
		bitsRequired := bpv
		if bpv != 64 {
			bitsRequired = DirectWriterBitsRequired(int64(1) << uint(bpv-1))
		}
		out := store.NewByteArrayDataOutput(64)
		for j := int64(0); j < offset; j++ {
			if err := out.WriteByte(byte(r.Intn(256))); err != nil {
				t.Fatalf("bpv=%d iter=%d: prefix write: %v", bpv, i, err)
			}
		}
		w, err := GetDirectWriter(out, int64(len(original)), bitsRequired)
		if err != nil {
			t.Fatalf("bpv=%d iter=%d: GetDirectWriter: %v", bpv, i, err)
		}
		for _, v := range original {
			if err := w.Add(v); err != nil {
				t.Fatalf("bpv=%d iter=%d: Add: %v", bpv, i, err)
			}
		}
		if err := w.Finish(); err != nil {
			t.Fatalf("bpv=%d iter=%d: Finish: %v", bpv, i, err)
		}
		bytes := out.GetBytes()
		wantBytes, err := DirectWriterBytesRequired(int64(len(original)), bitsRequired)
		if err != nil {
			t.Fatalf("bpv=%d iter=%d: DirectWriterBytesRequired: %v", bpv, i, err)
		}
		if int64(len(bytes))-offset != wantBytes {
			t.Fatalf("bpv=%d iter=%d: payload bytes=%d, want %d", bpv, i, int64(len(bytes))-offset, wantBytes)
		}
		in := &byteSliceRandomAccess{data: bytes}
		// merge is accepted only for parity with the upstream test;
		// the underlying reader is identical until GetMergeInstance
		// is ported.
		_ = merge
		reader, err := GetDirectReaderAt(in, bitsRequired, offset)
		if err != nil {
			t.Fatalf("bpv=%d iter=%d: GetDirectReaderAt: %v", bpv, i, err)
		}
		for j, want := range original {
			got, err := reader.Get(int64(j))
			if err != nil {
				t.Fatalf("bpv=%d iter=%d [%d]: unexpected error: %v", bpv, i, j, err)
			}
			if got != want {
				t.Fatalf("bpv=%d iter=%d [%d]: got %d want %d", bpv, i, j, got, want)
			}
		}
	}
}

// randomLongsForBpv mirrors TestDirectPacked#randomLongs.
func randomLongsForBpv(r *rand.Rand, bpv int) []int64 {
	amount := r.Intn(5000)
	out := make([]int64, amount)
	max := MaxValue(bpv)
	for i := range out {
		out[i] = randomLongBetween(r, 0, max)
	}
	return out
}

// randomLongBetween returns a value in the inclusive range [min,max].
// It is the local equivalent of RandomNumbers.randomLongBetween used
// by the Lucene source.
func randomLongBetween(r *rand.Rand, min, max int64) int64 {
	if min > max {
		panic("randomLongBetween: min > max")
	}
	rangeSize := uint64(max) - uint64(min) + 1
	if rangeSize == 0 { // max-min+1 overflowed (bpv=64 full range)
		return int64(r.Uint64())
	}
	return min + int64(r.Uint64()%rangeSize)
}
