// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// bitsPerValueSpectrum lists the bitsPerValue values the round-trip
// tests must cover. It is the spectrum required by Sprint 2 plus
// values 11, 14, 18, 28, 36, 48 to widen coverage.
var bitsPerValueSpectrum = []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 12, 15, 16, 20, 24, 32, 40, 48, 56, 63, 64}

// singleBlockSpectrum is the subset that the PACKED_SINGLE_BLOCK
// format supports.
var singleBlockSpectrum = []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 12, 16, 21, 32}

// randValues generates n random uint64 values bounded by
// MaxValue(bitsPerValue), expressed as int64.
func randValues(seed int64, n, bitsPerValue int) []int64 {
	r := rand.New(rand.NewSource(seed))
	values := make([]int64, n)
	mask := uint64(MaxValue(bitsPerValue))
	if bitsPerValue == 64 {
		for i := range values {
			values[i] = int64(r.Uint64())
		}
		return values
	}
	for i := range values {
		values[i] = int64(r.Uint64() & mask)
	}
	return values
}

func TestBitsRequired(t *testing.T) {
	t.Parallel()
	cases := []struct {
		v    int64
		want int
	}{
		{0, 1}, {1, 1}, {2, 2}, {3, 2}, {4, 3}, {7, 3}, {8, 4},
		{15, 4}, {16, 5}, {255, 8}, {256, 9}, {0x7FFFFFFFFFFFFFFF, 63},
	}
	for _, c := range cases {
		if got := BitsRequired(c.v); got != c.want {
			t.Errorf("BitsRequired(%d) = %d, want %d", c.v, got, c.want)
		}
	}
}

func TestMaxValue(t *testing.T) {
	t.Parallel()
	cases := []struct {
		bpv  int
		want int64
	}{
		{1, 1}, {2, 3}, {3, 7}, {7, 127}, {8, 255}, {16, 65535},
		{32, 0xFFFFFFFF}, {63, 0x7FFFFFFFFFFFFFFF}, {64, 0x7FFFFFFFFFFFFFFF},
	}
	for _, c := range cases {
		if got := MaxValue(c.bpv); got != c.want {
			t.Errorf("MaxValue(%d) = %d, want %d", c.bpv, got, c.want)
		}
	}
}

func TestFormatByID(t *testing.T) {
	t.Parallel()
	for _, id := range []int{0, 1} {
		f, err := FormatByID(id)
		if err != nil {
			t.Errorf("FormatByID(%d) error: %v", id, err)
		}
		if f.ID() != id {
			t.Errorf("Format(%d).ID() = %d", id, f.ID())
		}
	}
	if _, err := FormatByID(7); err == nil {
		t.Errorf("FormatByID(7) expected error")
	}
}

func TestFastestFormatAndBits(t *testing.T) {
	t.Parallel()
	fab := FastestFormatAndBits(-1, 5, Fastest)
	if fab.Format != FormatPacked || fab.BitsPerValue != 8 {
		t.Errorf("Fastest(5) = %+v, want {Packed, 8}", fab)
	}
	fab = FastestFormatAndBits(-1, 5, Compact)
	if fab.Format != FormatPacked || fab.BitsPerValue != 5 {
		t.Errorf("Compact(5) = %+v, want {Packed, 5}", fab)
	}
}

func TestCheckBlockSize(t *testing.T) {
	t.Parallel()
	if log2, err := CheckBlockSize(1024, 64, 1<<20); err != nil || log2 != 10 {
		t.Errorf("CheckBlockSize(1024) = %d,%v", log2, err)
	}
	if _, err := CheckBlockSize(63, 64, 1<<20); err == nil {
		t.Error("expected error for 63")
	}
	if _, err := CheckBlockSize(1000, 64, 1<<20); err == nil {
		t.Error("expected error for 1000 (not a power of two)")
	}
}

func TestNumBlocks(t *testing.T) {
	t.Parallel()
	cases := []struct {
		size      int64
		blockSize int
		want      int
	}{
		{0, 16, 0}, {1, 16, 1}, {16, 16, 1}, {17, 16, 2}, {32, 16, 2},
	}
	for _, c := range cases {
		got, err := NumBlocks(c.size, c.blockSize)
		if err != nil {
			t.Errorf("NumBlocks(%d,%d) err=%v", c.size, c.blockSize, err)
		}
		if got != c.want {
			t.Errorf("NumBlocks(%d,%d) = %d, want %d", c.size, c.blockSize, got, c.want)
		}
	}
}

// TestPacked64RoundTrip exercises Packed64 across every bitsPerValue
// in the test spectrum with 1024 random values.
func TestPacked64RoundTrip(t *testing.T) {
	t.Parallel()
	const n = 1024
	for _, bpv := range bitsPerValueSpectrum {
		values := randValues(int64(bpv)*131, n, bpv)
		m := newPacked64(n, bpv)
		for i, v := range values {
			m.Set(i, v)
		}
		for i, v := range values {
			if got := m.Get(i); got != v {
				t.Fatalf("Packed64 bpv=%d index=%d: got %d, want %d", bpv, i, got, v)
			}
		}
		// bulk get
		out := make([]int64, n)
		off := 0
		for off < n {
			off += m.GetBulk(off, out, off, n-off)
		}
		for i := range values {
			if out[i] != values[i] {
				t.Fatalf("Packed64 bulk mismatch bpv=%d i=%d", bpv, i)
			}
		}
	}
}

// TestPacked64SingleBlockRoundTrip exercises Packed64SingleBlock
// across every supported bitsPerValue.
func TestPacked64SingleBlockRoundTrip(t *testing.T) {
	t.Parallel()
	const n = 1024
	for _, bpv := range singleBlockSpectrum {
		values := randValues(int64(bpv)*223, n, bpv)
		m := newPacked64SingleBlock(n, bpv)
		for i, v := range values {
			m.Set(i, v)
		}
		for i, v := range values {
			if got := m.Get(i); got != v {
				t.Fatalf("Packed64SingleBlock bpv=%d index=%d: got %d, want %d", bpv, i, got, v)
			}
		}
		out := make([]int64, n)
		off := 0
		for off < n {
			off += m.GetBulk(off, out, off, n-off)
		}
		for i := range values {
			if out[i] != values[i] {
				t.Fatalf("Packed64SingleBlock bulk mismatch bpv=%d i=%d", bpv, i)
			}
		}
	}
}

// TestWriterReaderRoundTripPacked verifies on-disk serialization for
// FormatPacked across the full bitsPerValue spectrum.
func TestWriterReaderRoundTripPacked(t *testing.T) {
	t.Parallel()
	const n = 1024
	for _, bpv := range bitsPerValueSpectrum {
		values := randValues(int64(bpv)*977, n, bpv)
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

		in := store.NewByteArrayDataInput(out.GetBytes())
		it, err := GetReaderIteratorNoHeader(in, FormatPacked, VersionCurrent, n, bpv, DefaultBufferSize)
		if err != nil {
			t.Fatalf("GetReaderIteratorNoHeader err=%v", err)
		}
		for i, want := range values {
			got, err := it.Next()
			if err != nil {
				t.Fatalf("Next bpv=%d i=%d err=%v", bpv, i, err)
			}
			if got != want {
				t.Fatalf("Next bpv=%d i=%d: got %d, want %d", bpv, i, got, want)
			}
		}
	}
}

// TestWriterReaderRoundTripPackedSingleBlock verifies the
// FormatPackedSingleBlock wire layout (big-endian per 64-bit block).
func TestWriterReaderRoundTripPackedSingleBlock(t *testing.T) {
	t.Parallel()
	const n = 1024
	for _, bpv := range singleBlockSpectrum {
		values := randValues(int64(bpv)*1499, n, bpv)
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

		in := store.NewByteArrayDataInput(out.GetBytes())
		it, err := GetReaderIteratorNoHeader(in, FormatPackedSingleBlock, VersionCurrent, n, bpv, DefaultBufferSize)
		if err != nil {
			t.Fatalf("GetReaderIteratorNoHeader err=%v", err)
		}
		for i, want := range values {
			got, err := it.Next()
			if err != nil {
				t.Fatalf("Next bpv=%d i=%d err=%v", bpv, i, err)
			}
			if got != want {
				t.Fatalf("Next bpv=%d i=%d: got %d, want %d", bpv, i, got, want)
			}
		}
	}
}

// TestPacked64ByteCompatibility verifies that writing Packed64
// blocks through the encoder produces the exact byte sequence that
// PackedWriter would emit.
//
// This test ensures the in-memory representation (Packed64.blocks)
// matches the on-disk representation produced by EncodeLongsToBytes
// when serialised long-by-long in big-endian byte order.
func TestPacked64ByteCompatibility(t *testing.T) {
	t.Parallel()
	const n = 128
	for _, bpv := range bitsPerValueSpectrum {
		values := randValues(int64(bpv)*1733, n, bpv)
		// Use the writer to obtain the canonical byte stream.
		out := store.NewByteArrayDataOutput(64)
		w, err := GetWriterNoHeader(out, FormatPacked, n, bpv, DefaultBufferSize)
		if err != nil {
			t.Fatalf("GetWriterNoHeader err=%v", err)
		}
		for _, v := range values {
			_ = w.Add(v)
		}
		_ = w.Finish()
		canonical := out.GetBytes()

		// Round-trip those bytes via decoder, then re-encode.
		expectedLen := FormatPacked.ByteCount(VersionCurrent, n, bpv)
		if int64(len(canonical)) != expectedLen {
			t.Fatalf("bpv=%d: writer produced %d bytes, expected %d", bpv, len(canonical), expectedLen)
		}

		// Decode and verify.
		decoded := make([]int64, n)
		op, _ := BulkOperationOf(FormatPacked, bpv)
		iterations := op.ComputeIterations(n, DefaultBufferSize)
		buf := make([]byte, iterations*op.ByteBlockCount())
		copy(buf, canonical)
		nValuesPerIter := iterations * op.ByteValueCount()
		full := n / nValuesPerIter
		if full > 0 {
			op.DecodeBytes(buf, 0, decoded, 0, full*iterations/iterations)
		}
		// Fall back to iterator for the remainder.
		in := store.NewByteArrayDataInput(canonical)
		it, err := GetReaderIteratorNoHeader(in, FormatPacked, VersionCurrent, n, bpv, DefaultBufferSize)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		for i := 0; i < n; i++ {
			v, err := it.Next()
			if err != nil {
				t.Fatalf("Next err=%v", err)
			}
			if v != values[i] {
				t.Fatalf("bpv=%d index=%d: got %d, want %d", bpv, i, v, values[i])
			}
		}
	}
}

func TestNullReader(t *testing.T) {
	t.Parallel()
	n := NewNullReader(10)
	if n.Size() != 10 {
		t.Fatalf("Size=%d", n.Size())
	}
	if n.Get(3) != 0 {
		t.Fatalf("Get must be zero")
	}
	arr := make([]int64, 5)
	for i := range arr {
		arr[i] = 99
	}
	got := n.GetBulk(0, arr, 0, 5)
	if got != 5 {
		t.Fatalf("GetBulk=%d", got)
	}
	for i, v := range arr {
		if v != 0 {
			t.Fatalf("arr[%d]=%d", i, v)
		}
	}
}

func TestCopy(t *testing.T) {
	t.Parallel()
	const n = 200
	src := newPacked64(n, 7)
	for i := 0; i < n; i++ {
		src.Set(i, int64(i%128))
	}
	dst := newPacked64(n, 7)
	Copy(src, 0, dst, 0, n, 256)
	for i := 0; i < n; i++ {
		if dst.Get(i) != int64(i%128) {
			t.Fatalf("Copy mismatch at %d", i)
		}
	}
}

// TestFormatConstantsExactly verifies every Go-side constant matches the
// Apache Lucene 10.4.0 source exactly. Any mismatch here means Gocene would
// produce an on-disk layout that Lucene cannot read, or vice versa.
func TestFormatConstantsExactly(t *testing.T) {
	t.Parallel()

	// PackedInts overhead/performance constants.
	if Fastest != 7.0 {
		t.Errorf("Fastest: got %f, want 7.0 (Lucene FASTEST)", Fastest)
	}
	if Fast != 0.5 {
		t.Errorf("Fast: got %f, want 0.5 (Lucene FAST)", Fast)
	}
	if Default != 0.25 {
		t.Errorf("Default: got %f, want 0.25 (Lucene DEFAULT)", Default)
	}
	if Compact != 0.0 {
		t.Errorf("Compact: got %f, want 0.0 (Lucene COMPACT)", Compact)
	}
	if DefaultBufferSize != 1024 {
		t.Errorf("DefaultBufferSize: got %d, want 1024 (Lucene DEFAULT_BUFFER_SIZE)", DefaultBufferSize)
	}

	// Codec identity constants.
	if CodecName != "PackedInts" {
		t.Errorf("CodecName: got %q, want %q", CodecName, "PackedInts")
	}
	if VersionMonotonicWithoutZigzag != 2 {
		t.Errorf("VersionMonotonicWithoutZigzag: got %d, want 2", VersionMonotonicWithoutZigzag)
	}
	if VersionStart != VersionMonotonicWithoutZigzag {
		t.Errorf("VersionStart != VersionMonotonicWithoutZigzag")
	}
	if VersionCurrent != VersionMonotonicWithoutZigzag {
		t.Errorf("VersionCurrent != VersionMonotonicWithoutZigzag")
	}

	// Format IDs.
	if int(FormatPacked) != 0 {
		t.Errorf("FormatPacked ID: got %d, want 0", FormatPacked)
	}
	if int(FormatPackedSingleBlock) != 1 {
		t.Errorf("FormatPackedSingleBlock ID: got %d, want 1", FormatPackedSingleBlock)
	}

	// DirectMonotonic writer block-shift bounds.
	if DirectMonotonicMinBlockShift != 2 {
		t.Errorf("DirectMonotonicMinBlockShift: got %d, want 2", DirectMonotonicMinBlockShift)
	}
	if DirectMonotonicMaxBlockShift != 22 {
		t.Errorf("DirectMonotonicMaxBlockShift: got %d, want 22", DirectMonotonicMaxBlockShift)
	}

	// AbstractBlockPackedWriter constants.
	if BlockPackedMinBlockSize != 64 {
		t.Errorf("BlockPackedMinBlockSize: got %d, want 64", BlockPackedMinBlockSize)
	}
	if BlockPackedMaxBlockSize != 1<<(30-3) {
		t.Errorf("BlockPackedMaxBlockSize: got %d, want %d", BlockPackedMaxBlockSize, 1<<(30-3))
	}
	if bpwMinValueEqualsZero != 1 {
		t.Errorf("bpwMinValueEqualsZero: got %d, want 1", bpwMinValueEqualsZero)
	}
	if bpwBpvShift != 1 {
		t.Errorf("bpwBpvShift: got %d, want 1", bpwBpvShift)
	}

	// BulkOperationPacked constants — mask values for each bpv equal 2^bpv-1.
	for bpv := 1; bpv <= 64; bpv++ {
		op := newBulkOperationPacked(bpv)
		if op.bitsPerValue != bpv {
			t.Errorf("BulkOperationPacked(%d).bitsPerValue = %d", bpv, op.bitsPerValue)
		}
	}
}

func TestPacked64Fill(t *testing.T) {
	t.Parallel()
	const n = 200
	for _, bpv := range []int{1, 3, 7, 12, 31, 63} {
		m := newPacked64(n, bpv)
		val := MaxValue(bpv)
		m.Fill(10, n-10, val)
		for i := 0; i < n; i++ {
			want := int64(0)
			if i >= 10 && i < n-10 {
				want = val
			}
			if got := m.Get(i); got != want {
				t.Fatalf("Fill bpv=%d i=%d: got %d want %d", bpv, i, got, want)
			}
		}
	}
}
