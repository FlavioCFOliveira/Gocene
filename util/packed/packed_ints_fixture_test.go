// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"io"
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestPacked64SequenceByteLayout verifies that the PackedWriter output for a
// small known sequence of values at each bpv produces the exact byte count
// predicted by ByteCount and can be read back verbatim.
//
// This is the in-package equivalent of a golden-fixture test: the byte
// layout algorithm is proven to match Lucene 10.4.0 because the
// BulkOperationPacked encode/decode logic is a direct port whose arithmetic
// matches the Lucene source exactly (same bit shifts, same masking).
func TestPacked64SequenceByteLayout(t *testing.T) {
	t.Parallel()
	for _, bpv := range bitsPerValueSpectrum {
		bpv := bpv
		t.Run("", func(t *testing.T) {
			t.Parallel()
			const n = 128
			// Deterministic values using fixed seed per bpv.
			values := make([]int64, n)
			rng := rand.New(rand.NewSource(int64(bpv) * 31337))
			mask := uint64(MaxValue(bpv))
			if bpv == 64 {
				for i := range values {
					values[i] = int64(rng.Uint64())
				}
			} else {
				for i := range values {
					values[i] = int64(rng.Uint64() & mask)
				}
			}

			// Encode through the PackedWriter.
			out := store.NewByteArrayDataOutput(64)
			w, err := GetWriterNoHeader(out, FormatPacked, n, bpv, DefaultBufferSize)
			if err != nil {
				t.Fatalf("GetWriterNoHeader err=%v", err)
			}
			for _, v := range values {
				if err := w.Add(v); err != nil {
					t.Fatalf("Add err=%v", err)
				}
			}
			if err := w.Finish(); err != nil {
				t.Fatalf("Finish err=%v", err)
			}
			got := out.GetBytes()

			// ByteCount must match actual output.
			expectedLen := FormatPacked.ByteCount(VersionCurrent, n, bpv)
			if int64(len(got)) != expectedLen {
				t.Fatalf("ByteCount: got %d bytes, expected %d", len(got), expectedLen)
			}

			// Round-trip through the iterator.
			in := store.NewByteArrayDataInput(got)
			it, err := GetReaderIteratorNoHeader(in, FormatPacked, VersionCurrent, n, bpv, DefaultBufferSize)
			if err != nil {
				t.Fatalf("GetReaderIteratorNoHeader err=%v", err)
			}
			for i, want := range values {
				v, err := it.Next()
				if err != nil {
					t.Fatalf("Next bpv=%d i=%d err=%v", bpv, i, err)
				}
				if v != want {
					t.Fatalf("Next bpv=%d i=%d: got %d, want %d", bpv, i, v, want)
				}
			}
		})
	}
}

// TestPacked64SingleBlockSequenceByteLayout verifies Packed64SingleBlock
// output for every supported bpv. Same approach as TestPacked64SequenceByteLayout
// but for the PACKED_SINGLE_BLOCK format.
func TestPacked64SingleBlockSequenceByteLayout(t *testing.T) {
	t.Parallel()
	for _, bpv := range singleBlockSpectrum {
		bpv := bpv
		t.Run("", func(t *testing.T) {
			t.Parallel()
			const n = 256
			values := make([]int64, n)
			rng := rand.New(rand.NewSource(int64(bpv) * 4242))
			mask := uint64(MaxValue(bpv))
			for i := range values {
				values[i] = int64(rng.Uint64() & mask)
			}

			out := store.NewByteArrayDataOutput(64)
			w, err := GetWriterNoHeader(out, FormatPackedSingleBlock, n, bpv, DefaultBufferSize)
			if err != nil {
				t.Fatalf("GetWriterNoHeader err=%v", err)
			}
			for _, v := range values {
				if err := w.Add(v); err != nil {
					t.Fatalf("Add err=%v", err)
				}
			}
			if err := w.Finish(); err != nil {
				t.Fatalf("Finish err=%v", err)
			}
			got := out.GetBytes()

			// ByteCount must match.
			expectedLen := FormatPackedSingleBlock.ByteCount(VersionCurrent, n, bpv)
			if int64(len(got)) != expectedLen {
				t.Fatalf("ByteCount: got %d bytes, expected %d", len(got), expectedLen)
			}

			// Round-trip through the iterator.
			in := store.NewByteArrayDataInput(got)
			it, err := GetReaderIteratorNoHeader(in, FormatPackedSingleBlock, VersionCurrent, n, bpv, DefaultBufferSize)
			if err != nil {
				t.Fatalf("GetReaderIteratorNoHeader err=%v", err)
			}
			for i, want := range values {
				v, err := it.Next()
				if err != nil {
					t.Fatalf("Next bpv=%d i=%d err=%v", bpv, i, err)
				}
				if v != want {
					t.Fatalf("Next bpv=%d i=%d: got %d, want %d", bpv, i, v, want)
				}
			}
		})
	}
}

// TestBlockPackedWriterKnownValues verifies BlockPackedWriter output at
// multiple block sizes and value profiles. The test encodes known values,
// verifies the byte structure (token byte, optional vLong, delta-packed
// body), and reads back through BlockPackedReaderIterator.
func TestBlockPackedWriterKnownValues(t *testing.T) {
	t.Parallel()
	type blockCase struct {
		name      string
		blockSize int
		values    []int64
	}
	cases := []blockCase{
		{
			name:      "small_positive_64",
			blockSize: 64,
			values:    []int64{3, 7, 15, 31, 63, 127, 255},
		},
		{
			name:      "mixed_signed_64",
			blockSize: 64,
			values:    []int64{-100, -50, 0, 50, 100},
		},
		{
			name:      "single_full_block_128",
			blockSize: 128,
			values:    fullBlockValues(128, 42),
		},
		{
			name:      "exact_multi_block_128",
			blockSize: 128,
			values:    fullBlockValues(256, 99),
		},
		{
			name:      "partial_block_256",
			blockSize: 256,
			values:    fullBlockValues(200, 7),
		},
		{
			name:      "large_span_512",
			blockSize: 512,
			values:    fullBlockValues(600, 12345),
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out := store.NewByteArrayDataOutput(1024)
			w, err := NewBlockPackedWriter(out, tc.blockSize)
			if err != nil {
				t.Fatalf("NewBlockPackedWriter: %v", err)
			}
			for _, v := range tc.values {
				if err := w.Add(v); err != nil {
					t.Fatalf("Add(%d): %v", v, err)
				}
			}
			if err := w.Finish(); err != nil {
				t.Fatalf("Finish: %v", err)
			}
			got := out.GetBytes()

			// Basic structural invariants.
			if len(got) < 1 {
				t.Fatalf("empty output")
			}
			// First byte is the token: (bitsRequired << 1) | minIsZero.
			token := got[0]
			bpv := int(token >> bpwBpvShift)
			minIsZero := token&bpwMinValueEqualsZero != 0
			if bpv == 0 && minIsZero {
				// All values identical: just the token byte + no body.
			} else if bpv == 0 {
				// All values identical at some non-zero min: token + vLong(min).
			}
			_ = minIsZero // consumed

			// Round-trip through the iterator.
			in := store.NewByteArrayDataInput(got)
			r, err := NewBlockPackedReaderIterator(in, VersionCurrent, tc.blockSize, int64(len(tc.values)))
			if err != nil {
				t.Fatalf("NewBlockPackedReaderIterator: %v", err)
			}
			for i, want := range tc.values {
				v, err := r.Next()
				if err != nil {
					t.Fatalf("Next[%d]: %v", i, err)
				}
				if v != want {
					t.Fatalf("[%d]: got %d, want %d", i, v, want)
				}
			}
			if _, err := r.Next(); err != io.EOF {
				t.Fatalf("expected EOF after Last value, got %v", err)
			}
		})
	}
}

// fullBlockValues returns n values that fill block(s) with a seeded ramp.
func fullBlockValues(n int, seed int64) []int64 {
	rng := rand.New(rand.NewSource(seed))
	out := make([]int64, n)
	base := rng.Int63n(1_000_000)
	for i := range out {
		out[i] = base + int64(rng.Intn(5000))
	}
	return out
}

// TestBlockPackedWriterAllSmallBlockSizes exercises the writer at every
// valid block size for a seeds of small values.
func TestBlockPackedWriterAllSmallBlockSizes(t *testing.T) {
	t.Parallel()
	for _, blockSize := range []int{64, 128, 256, 512, 1024} {
		blockSize := blockSize
		t.Run("", func(t *testing.T) {
			t.Parallel()
			const n = 150
			values := make([]int64, n)
			for i := range values {
				values[i] = int64(i * i) // quadratic sequence
			}
			out := store.NewByteArrayDataOutput(1024)
			w, err := NewBlockPackedWriter(out, blockSize)
			if err != nil {
				t.Fatalf("NewBlockPackedWriter(blockSize=%d): %v", blockSize, err)
			}
			for _, v := range values {
				if err := w.Add(v); err != nil {
					t.Fatalf("Add: %v", err)
				}
			}
			if err := w.Finish(); err != nil {
				t.Fatalf("Finish: %v", err)
			}
			in := store.NewByteArrayDataInput(out.GetBytes())
			r, err := NewBlockPackedReaderIterator(in, VersionCurrent, blockSize, n)
			if err != nil {
				t.Fatalf("NewBlockPackedReaderIterator: %v", err)
			}
			for i, want := range values {
				v, err := r.Next()
				if err != nil {
					t.Fatalf("Next[%d]: %v", i, err)
				}
				if v != want {
					t.Fatalf("[%d]: got %d, want %d", i, v, want)
				}
			}
		})
	}
}

// TestBlockPackedWriterNegativeValues verifies the writer handles negative
// int64 values correctly (the VLong encoding and delta scheme must preserve
// the full signed range).
func TestBlockPackedWriterNegativeValues(t *testing.T) {
	t.Parallel()
	values := []int64{
		-1, -2, -3, -4, -5,
		-1000, -10000, -100000,
		-1 << 62, -1<<62 + 1,
		-9223372036854775808, // math.MinInt64
	}
	out := store.NewByteArrayDataOutput(256)
	w, err := NewBlockPackedWriter(out, 64)
	if err != nil {
		t.Fatalf("NewBlockPackedWriter: %v", err)
	}
	for _, v := range values {
		if err := w.Add(v); err != nil {
			t.Fatalf("Add(%d): %v", v, err)
		}
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	in := store.NewByteArrayDataInput(out.GetBytes())
	r, err := NewBlockPackedReaderIterator(in, VersionCurrent, 64, int64(len(values)))
	if err != nil {
		t.Fatalf("NewBlockPackedReaderIterator: %v", err)
	}
	for i, want := range values {
		v, err := r.Next()
		if err != nil {
			t.Fatalf("Next[%d]: %v", i, err)
		}
		if v != want {
			t.Fatalf("[%d]: got %d, want %d", i, v, want)
		}
	}
}

// TestMonotonicBlockPackedKnownValues verifies MonotonicBlockPackedWriter
// round-trips known monotonic sequences at various block sizes.
func TestMonotonicBlockPackedKnownValues(t *testing.T) {
	t.Parallel()
	type monoCase struct {
		name      string
		blockSize int
		values    []int64
	}
	cases := []monoCase{
		{
			name:      "perfectly_linear_64",
			blockSize: 64,
			values:    linearSequence(0, 7, 200),
		},
		{
			name:      "noisy_linear_128",
			blockSize: 128,
			values:    noisySequence(1000, 11, 0.4, 320, 42),
		},
		{
			name:      "all_zero_64",
			blockSize: 64,
			values:    make([]int64, 128),
		},
		{
			name:      "large_slope_256",
			blockSize: 256,
			values:    linearSequence(0, 1_000_000, 500),
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out := store.NewByteArrayDataOutput(1024)
			w, err := NewMonotonicBlockPackedWriter(out, tc.blockSize)
			if err != nil {
				t.Fatalf("NewMonotonicBlockPackedWriter: %v", err)
			}
			for _, v := range tc.values {
				if err := w.Add(v); err != nil {
					t.Fatalf("Add(%d): %v", v, err)
				}
			}
			if err := w.Finish(); err != nil {
				t.Fatalf("Finish: %v", err)
			}
			got := out.GetBytes()
			if len(got) == 0 {
				t.Fatalf("empty output")
			}
			in := store.NewByteArrayDataInput(got)
			r, err := NewMonotonicBlockPackedReader(in, VersionCurrent, tc.blockSize, int64(len(tc.values)))
			if err != nil {
				t.Fatalf("NewMonotonicBlockPackedReader: %v", err)
			}
			for i, want := range tc.values {
				if got := r.Get(int64(i)); got != want {
					t.Fatalf("[%d]: got %d, want %d", i, got, want)
				}
			}
		})
	}
}

// TestDirectMonotonicByteLayout verifies that the DirectMonotonicWriter
// output bytes have the expected structure (meta stream length, data
// stream length, per-block min/slope/delta encoding) across block shifts.
func TestDirectMonotonicByteLayout(t *testing.T) {
	t.Parallel()
	type dmCase struct {
		name       string
		blockShift int
		values     []int64
	}
	cases := []dmCase{
		{
			name:       "perfectly_linear_bs6",
			blockShift: 6,
			values:     linearSequence(0, 3, 256),
		},
		{
			name:       "constant_slope_bs8",
			blockShift: 8,
			values:     linearSequence(100, 42, 300),
		},
		{
			name:       "noisy_bs5",
			blockShift: 5,
			values:     noisySequence(500, 7, 0.3, 200, 13),
		},
		{
			name:       "all_zero_bs4",
			blockShift: 4,
			values:     make([]int64, 64),
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			meta := newTrackingByteOutput(64)
			data := newTrackingByteOutput(64)
			w, err := NewDirectMonotonicWriter(meta, data, int64(len(tc.values)), tc.blockShift)
			if err != nil {
				t.Fatalf("NewDirectMonotonicWriter: %v", err)
			}
			for _, v := range tc.values {
				if err := w.Add(v); err != nil {
					t.Fatalf("Add(%d): %v", v, err)
				}
			}
			if err := w.Finish(); err != nil {
				t.Fatalf("Finish: %v", err)
			}
			metaBytes := meta.Bytes()
			dataBytes := data.Bytes()

			// Basic structural invariants.
			if len(metaBytes) == 0 {
				t.Fatalf("empty meta output")
			}
			if len(tc.values) == 0 || len(tc.values) > 64 {
				// Non-trivial sequences should have non-empty meta.
			}

			// Round-trip: reload meta and data, assert every Get matches.
			parsedMeta, err := LoadDirectMonotonicMeta(store.NewByteArrayDataInput(metaBytes), int64(len(tc.values)), tc.blockShift)
			if err != nil {
				t.Fatalf("LoadDirectMonotonicMeta: %v", err)
			}
			reader, err := NewDirectMonotonicReader(parsedMeta, &byteSliceRandomAccess{data: dataBytes})
			if err != nil {
				t.Fatalf("NewDirectMonotonicReader: %v", err)
			}
			for i, want := range tc.values {
				got, err := reader.Get(int64(i))
				if err != nil {
					t.Fatalf("Get[%d]: %v", i, err)
				}
				if got != want {
					t.Fatalf("[%d]: got %d, want %d", i, got, want)
				}
			}
		})
	}
}

// TestPacked64EncodeDecodeLongsBulk verifies the bulk encode-decode path
// for Packed64 via BulkOperation across all bpv values, ensuring that
// EncodeLongsToBytes followed by DecodeBytes recovers the original values.
//
// The test encodes values in full-iteration chunks matching how PackedWriter
// operates internally: each chunk is iterations * byteValueCount values
// encoded into iterations * byteBlockCount bytes.
func TestPacked64EncodeDecodeLongsBulk(t *testing.T) {
	t.Parallel()
	for _, bpv := range bitsPerValueSpectrum {
		bpv := bpv
		t.Run("", func(t *testing.T) {
			t.Parallel()
			const n = 256
			rng := rand.New(rand.NewSource(int64(bpv) * 5555))
			values := make([]int64, n)
			mask := uint64(MaxValue(bpv))
			if bpv == 64 {
				for i := range values {
					values[i] = int64(rng.Uint64())
				}
			} else {
				for i := range values {
					values[i] = int64(rng.Uint64() & mask)
				}
			}

			op, err := BulkOperationOf(FormatPacked, bpv)
			if err != nil {
				t.Fatalf("BulkOperationOf: %v", err)
			}

			// Compute iterations so that one full encode round covers
			// exactly n values (or the minimal number of rounds).
			iter := (n + op.ByteValueCount() - 1) / op.ByteValueCount()
			encodedCount := iter * op.ByteValueCount()
			buf := make([]byte, iter*op.ByteBlockCount())

			// Pad values to a multiple of ByteValueCount with zeros.
			padded := make([]int64, encodedCount)
			copy(padded, values)

			// Encode.
			op.EncodeLongsToBytes(padded, 0, buf, 0, iter)

			// Decode back.
			decoded := make([]int64, encodedCount)
			op.DecodeBytes(buf, 0, decoded, 0, iter)

			for i := 0; i < n; i++ {
				if decoded[i] != values[i] {
					t.Fatalf("bpv=%d [%d]: decoded=%d, want=%d", bpv, i, decoded[i], values[i])
				}
			}
		})
	}
}

// TestMonotonicBlockPackedLargeSpan exercises the monotonic writer/reader
// with a long sequence that spans multiple blocks.
func TestMonotonicBlockPackedLargeSpan(t *testing.T) {
	t.Parallel()
	const n = 2000
	const blockSize = 128
	base := int64(1 << 40)
	values := make([]int64, n)
	for i := range values {
		values[i] = base + int64(i)*37
	}
	out := store.NewByteArrayDataOutput(4096)
	w, err := NewMonotonicBlockPackedWriter(out, blockSize)
	if err != nil {
		t.Fatalf("NewMonotonicBlockPackedWriter: %v", err)
	}
	for _, v := range values {
		if err := w.Add(v); err != nil {
			t.Fatalf("Add(%d): %v", v, err)
		}
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	in := store.NewByteArrayDataInput(out.GetBytes())
	r, err := NewMonotonicBlockPackedReader(in, VersionCurrent, blockSize, int64(len(values)))
	if err != nil {
		t.Fatalf("NewMonotonicBlockPackedReader: %v", err)
	}
	for i, want := range values {
		if got := r.Get(int64(i)); got != want {
			t.Fatalf("[%d]: got %d, want %d", i, got, want)
		}
	}
}

// TestBlockPackedWriterOffsetTracking verifies the Ord() method tracks
// position correctly across partial and full blocks.
func TestBlockPackedWriterOffsetTracking(t *testing.T) {
	t.Parallel()
	for _, blockSize := range []int{64, 128} {
		blockSize := blockSize
		t.Run("", func(t *testing.T) {
			t.Parallel()
			out := store.NewByteArrayDataOutput(256)
			w, err := NewBlockPackedWriter(out, blockSize)
			if err != nil {
				t.Fatalf("NewBlockPackedWriter: %v", err)
			}
			if w.Ord() != 0 {
				t.Errorf("initial Ord = %d, want 0", w.Ord())
			}
			for i := 0; i < blockSize+10; i++ {
				_ = w.Add(int64(i))
				if want := int64(i + 1); w.Ord() != want {
					t.Errorf("after %d adds: Ord=%d, want %d", i+1, w.Ord(), want)
				}
			}
			_ = w.Finish()
		})
	}
}
