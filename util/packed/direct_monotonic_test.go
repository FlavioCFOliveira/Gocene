// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"encoding/binary"
	"math"
	"math/rand"
	"sort"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// trackingByteOutput is a tiny DataOutputAt that records every written
// byte and reports its own running position; used to feed both the
// meta and data streams when round-tripping DirectMonotonicWriter.
type trackingByteOutput struct {
	out *store.ByteArrayDataOutput
}

func newTrackingByteOutput(initialCapacity int) *trackingByteOutput {
	return &trackingByteOutput{out: store.NewByteArrayDataOutput(initialCapacity)}
}

func (t *trackingByteOutput) WriteByte(b byte) error    { return t.out.WriteByte(b) }
func (t *trackingByteOutput) WriteBytes(b []byte) error { return t.out.WriteBytes(b) }
func (t *trackingByteOutput) WriteBytesN(b []byte, n int) error {
	return t.out.WriteBytesN(b, n)
}
func (t *trackingByteOutput) WriteShort(i int16) error   { return t.out.WriteShort(i) }
func (t *trackingByteOutput) WriteInt(i int32) error     { return t.out.WriteInt(i) }
func (t *trackingByteOutput) WriteLong(i int64) error    { return t.out.WriteLong(i) }
func (t *trackingByteOutput) WriteString(s string) error { return t.out.WriteString(s) }
func (t *trackingByteOutput) GetFilePointer() int64      { return int64(len(t.out.GetBytes())) }

func (t *trackingByteOutput) Bytes() []byte { return t.out.GetBytes() }

// TestDirectMonotonicRoundTrip writes a known monotonic sequence and
// reads it back through DirectMonotonicReader using the meta produced
// by the writer.
func TestDirectMonotonicRoundTrip(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		blockShift int
		values     []int64
	}{
		{"perfectly linear", 6, linearSequence(0, 3, 256)},
		{"slightly noisy linear", 6, noisySequence(100, 5, 0.7, 256, 42)},
		{"all-zero", 5, make([]int64, 64)},
		{"single block", 4, linearSequence(7, 11, 12)},
		{"large slope", 8, linearSequence(0, 1_000_000, 600)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reader, _, _ := writeAndOpenDirectMonotonic(t, tc.values, tc.blockShift)
			for i, want := range tc.values {
				got, err := reader.Get(int64(i))
			if err != nil {
				t.Errorf("[%d]: unexpected error: %v", i, err)
			}
			if got != want {
					t.Errorf("[%d]: got %d want %d", i, got, want)
				}
			}
		})
	}
}

// TestDirectMonotonicBinarySearch verifies the binary search helper on
// a known sorted sequence with no duplicates.
func TestDirectMonotonicBinarySearch(t *testing.T) {
	t.Parallel()
	values := linearSequence(0, 7, 257) // 0, 7, 14, ..., 1792
	const blockShift = 6
	reader, _, _ := writeAndOpenDirectMonotonic(t, values, blockShift)

	// Hit
	idx, err := reader.BinarySearch(0, int64(len(values)), 70) // values[10]
	if err != nil {
		t.Fatal(err)
	}
	if idx != 10 {
		t.Errorf("BinarySearch(70) = %d, want 10", idx)
	}
	// Miss — value 71 falls between values[10]=70 and values[11]=77.
	idx, err = reader.BinarySearch(0, int64(len(values)), 71)
	if err != nil {
		t.Fatal(err)
	}
	if idx >= 0 {
		t.Errorf("BinarySearch(71) = %d, want negative insertion point", idx)
	}
}

// TestDirectMonotonicRejectsBlockShift exercises the bounds-checking
// path so we don't silently accept invalid configurations.
func TestDirectMonotonicRejectsBlockShift(t *testing.T) {
	t.Parallel()
	meta := newTrackingByteOutput(0)
	data := newTrackingByteOutput(0)
	if _, err := NewDirectMonotonicWriter(meta, data, 10, DirectMonotonicMinBlockShift-1); err == nil {
		t.Error("expected error for blockShift below MIN, got nil")
	}
	if _, err := NewDirectMonotonicWriter(meta, data, 10, DirectMonotonicMaxBlockShift+1); err == nil {
		t.Error("expected error for blockShift above MAX, got nil")
	}
	if _, err := NewDirectMonotonicWriter(meta, data, -1, 6); err == nil {
		t.Error("expected error for negative numValues, got nil")
	}
}

// TestDirectMonotonicValidation mirrors Lucene's testValidation: it
// asserts that the three rejection messages are emitted, including
// the case where blockShift is too low for numValues.
//
// Port of TestDirectMonotonic.testValidation (Lucene 10.4.0).
func TestDirectMonotonicValidation(t *testing.T) {
	t.Parallel()
	meta := newTrackingByteOutput(0)
	data := newTrackingByteOutput(0)

	_, err := NewDirectMonotonicWriter(meta, data, -1, 10)
	if err == nil || !strings.Contains(err.Error(), "numValues can't be negative") {
		t.Errorf("expected negative-numValues error, got %v", err)
	}

	_, err = NewDirectMonotonicWriter(meta, data, 10, 1)
	if err == nil || !strings.Contains(err.Error(), "blockShift must be in") {
		t.Errorf("expected blockShift-range error, got %v", err)
	}

	// Same rejection Lucene asserts on blockShift=5, numValues=1<<40.
	_, err = NewDirectMonotonicWriter(meta, data, int64(1)<<40, 5)
	if err == nil || !strings.Contains(err.Error(), "blockShift is too low") {
		t.Errorf("expected too-low blockShift error, got %v", err)
	}
}

// TestDirectMonotonicEmpty round-trips an empty sequence and asserts
// that the reader can be constructed without error.
//
// Port of TestDirectMonotonic.testEmpty (Lucene 10.4.0).
func TestDirectMonotonicEmpty(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(0xE36117))
	blockShift := DirectMonotonicMinBlockShift +
		r.Intn(DirectMonotonicMaxBlockShift-DirectMonotonicMinBlockShift+1)

	meta := newTrackingByteOutput(0)
	data := newTrackingByteOutput(0)
	w, err := NewDirectMonotonicWriter(meta, data, 0, blockShift)
	if err != nil {
		t.Fatalf("NewDirectMonotonicWriter: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}

	metaIn := store.NewByteArrayDataInput(meta.Bytes())
	parsedMeta, err := LoadDirectMonotonicMeta(metaIn, 0, blockShift)
	if err != nil {
		t.Fatalf("LoadDirectMonotonicMeta: %v", err)
	}
	if _, err := NewDirectMonotonicReader(parsedMeta, &byteSliceRandomAccess{data: data.Bytes()}); err != nil {
		t.Fatalf("NewDirectMonotonicReader: %v", err)
	}
}

// TestDirectMonotonicSimple round-trips Lucene's `{1, 2, 5, 7, 8, 100}`
// fixture with blockShift=2, the canonical small smoke test.
//
// Port of TestDirectMonotonic.testSimple (Lucene 10.4.0).
func TestDirectMonotonicSimple(t *testing.T) {
	t.Parallel()
	values := []int64{1, 2, 5, 7, 8, 100}
	const blockShift = 2
	reader, _, _ := writeAndOpenDirectMonotonic(t, values, blockShift)
	for i, want := range values {
		got, err := reader.Get(int64(i))
			if err != nil {
				t.Errorf("[%d]: unexpected error: %v", i, err)
			}
			if got != want {
			t.Errorf("[%d]: got %d want %d", i, got, want)
		}
	}
}

// TestDirectMonotonicConstantSlope round-trips a strictly-linear
// sequence min + inc*i; with avgInc absorbing every value, the encoded
// data stream stays empty (every block has bpv=0).
//
// Port of TestDirectMonotonic.testConstantSlope (Lucene 10.4.0).
func TestDirectMonotonicConstantSlope(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(0xC051A))
	blockShift := DirectMonotonicMinBlockShift +
		r.Intn(DirectMonotonicMaxBlockShift-DirectMonotonicMinBlockShift+1)
	numValues := 1 + r.Intn(1<<14)
	min := r.Int63() - (1 << 62) // sign-spread, not just non-negative
	inc := int64(r.Intn(1 << (r.Intn(20) + 1)))

	values := make([]int64, numValues)
	for i := range values {
		values[i] = min + inc*int64(i)
	}

	reader, dataBytes, _ := writeAndOpenDirectMonotonic(t, values, blockShift)
	for i, want := range values {
		got, err := reader.Get(int64(i))
			if err != nil {
				t.Errorf("[%d]: unexpected error: %v", i, err)
			}
			if got != want {
			t.Fatalf("[%d]: got %d want %d (inc=%d, blockShift=%d)", i, got, want, inc, blockShift)
		}
	}
	if len(dataBytes) != 0 {
		t.Errorf("constant slope should not require encoded deltas; data has %d bytes", len(dataBytes))
	}
}

// TestDirectMonotonicZeroValuesSmallBlockShift writes a long all-zero
// sequence; LoadDirectMonotonicMeta should return the shared
// singleZeroBlockMeta instance on every call.
//
// Port of TestDirectMonotonic.testZeroValuesSmallBlobShift (Lucene 10.4.0).
func TestDirectMonotonicZeroValuesSmallBlockShift(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(0x0))
	numValues := 8 + r.Intn(1<<14)
	maxShift := int(math.Round(math.Log2(float64(numValues)))) - 1
	if maxShift < DirectMonotonicMinBlockShift {
		maxShift = DirectMonotonicMinBlockShift
	}
	blockShift := DirectMonotonicMinBlockShift
	if maxShift > DirectMonotonicMinBlockShift {
		blockShift += r.Intn(maxShift - DirectMonotonicMinBlockShift + 1)
	}

	values := make([]int64, numValues)
	reader, dataBytes, metaBytes := writeAndOpenDirectMonotonic(t, values, blockShift)

	// All Gets must return zero.
	for i := range values {
		got, err := reader.Get(int64(i))
		if err != nil {
			t.Fatalf("[%d]: unexpected error: %v", i, err)
		}
		if got != 0 {
			t.Fatalf("[%d]: got %d want 0", i, got)
		}
	}
	// Data stream remains empty (no per-block deltas were written).
	if len(dataBytes) != 0 {
		t.Errorf("zero sequence should leave data stream empty; got %d bytes", len(dataBytes))
	}
	// Re-loading meta must return the shared singleZeroBlockMeta.
	first, err := LoadDirectMonotonicMeta(store.NewByteArrayDataInput(metaBytes), int64(numValues), blockShift)
	if err != nil {
		t.Fatalf("first reload: %v", err)
	}
	second, err := LoadDirectMonotonicMeta(store.NewByteArrayDataInput(metaBytes), int64(numValues), blockShift)
	if err != nil {
		t.Fatalf("second reload: %v", err)
	}
	if first != second {
		t.Errorf("expected singleZeroBlockMeta to be shared across loads (%p vs %p)", first, second)
	}
}

// TestDirectMonotonicRandom drives the writer/reader with several
// random monotonic sequences across a range of block shifts.
//
// Port of TestDirectMonotonic.testRandom (Lucene 10.4.0).
func TestDirectMonotonicRandom(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(0xC0FFEE0))
	const iters = 6
	for iter := 0; iter < iters; iter++ {
		blockShift := DirectMonotonicMinBlockShift +
			r.Intn(DirectMonotonicMaxBlockShift-DirectMonotonicMinBlockShift+1)
		const maxNumValues = 1 << 14
		var numValues int
		if r.Intn(2) == 0 {
			numValues = 1 + r.Intn(maxNumValues)
		} else {
			numBlocks := r.Intn(maxNumValues>>blockShift + 1)
			numValues = r.Intn(numBlocks+1) << blockShift
		}
		values := make([]int64, numValues)
		if numValues > 0 {
			values[0] = r.Int63() - (1 << 62)
			for i := 1; i < numValues; i++ {
				values[i] = values[i-1] + int64(r.Intn(1<<(r.Intn(20)+1)))
			}
		}
		reader, _, _ := writeAndOpenDirectMonotonic(t, values, blockShift)
		for i, want := range values {
			got, err := reader.Get(int64(i))
			if err != nil {
				t.Errorf("[%d]: unexpected error: %v", i, err)
			}
			if got != want {
				t.Fatalf("iter=%d blockShift=%d [%d]: got %d want %d", iter, blockShift, i, got, want)
			}
		}
	}
}

// TestDirectMonotonicMonotonicBinarySearch covers Lucene's fixed
// 9-element fixture for binarySearch.
//
// Port of TestDirectMonotonic.testMonotonicBinarySearch (Lucene 10.4.0).
func TestDirectMonotonicMonotonicBinarySearch(t *testing.T) {
	t.Parallel()
	doMonotonicBinarySearchAgainstArray(t,
		[]int64{4, 7, 8, 10, 19, 30, 55, 78, 100}, 2, rand.New(rand.NewSource(0x5EA12)))
}

// TestDirectMonotonicMonotonicBinarySearchRandom runs the same
// binary-search invariants across many random arrays.
//
// Port of TestDirectMonotonic.testMonotonicBinarySearchRandom (Lucene 10.4.0).
func TestDirectMonotonicMonotonicBinarySearchRandom(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(0xBA51C))
	const iters = 25
	for iter := 0; iter < iters; iter++ {
		arrayLength := r.Intn(1 << (r.Intn(10) + 1))
		array := make([]int64, arrayLength)
		base := r.Int63() - (1 << 62)
		bpv := 4 + r.Intn(58) // 4..61, matching Lucene's TestUtil.nextInt(4, 61)
		maxDelta := int64(1)
		if bpv < 63 {
			maxDelta = (int64(1) << bpv) - 1
		}
		for i := range array {
			array[i] = base + r.Int63n(maxDelta+1)
		}
		sort.Slice(array, func(i, j int) bool { return array[i] < array[j] })
		doMonotonicBinarySearchAgainstArray(t, array, 2+r.Intn(9), r)
	}
}

// doMonotonicBinarySearchAgainstArray replays the invariants Lucene
// asserts in doTestMonotonicBinarySearchAgainstLongArray.
func doMonotonicBinarySearchAgainstArray(t *testing.T, array []int64, blockShift int, r *rand.Rand) {
	t.Helper()
	reader, _, _ := writeAndOpenDirectMonotonic(t, array, blockShift)

	if len(array) == 0 {
		idx, err := reader.BinarySearch(0, 0, 42)
		if err != nil {
			t.Fatal(err)
		}
		if idx != -1 {
			t.Errorf("empty: BinarySearch = %d, want -1", idx)
		}
		return
	}

	// Every present value is found and round-trips through the index.
	for i, v := range array {
		idx, err := reader.BinarySearch(0, int64(len(array)), v)
		if err != nil {
			t.Fatal(err)
		}
		if idx < 0 || idx >= int64(len(array)) {
			t.Fatalf("BinarySearch(%d) = %d, out of range", v, idx)
		}
		if array[idx] != v {
			t.Errorf("BinarySearch(%d) at [%d]: got %d want %d", i, idx, array[idx], v)
		}
	}
	// Below-range key.
	if array[0] != math.MinInt64 {
		idx, err := reader.BinarySearch(0, int64(len(array)), array[0]-1)
		if err != nil {
			t.Fatal(err)
		}
		if idx != -1 {
			t.Errorf("below-range: BinarySearch = %d, want -1", idx)
		}
	}
	// Above-range key.
	if array[len(array)-1] != math.MaxInt64 {
		idx, err := reader.BinarySearch(0, int64(len(array)), array[len(array)-1]+1)
		if err != nil {
			t.Fatal(err)
		}
		if want := int64(-1 - len(array)); idx != want {
			t.Errorf("above-range: BinarySearch = %d, want %d", idx, want)
		}
	}
	// Intermediate (non-present) keys must report a valid insertion point.
	for i := 0; i < len(array)-2; i++ {
		if array[i]+1 >= array[i+1] {
			continue
		}
		var intermediate int64
		if r.Intn(2) == 0 {
			intermediate = array[i] + 1
		} else {
			intermediate = array[i+1] - 1
		}
		idx, err := reader.BinarySearch(0, int64(len(array)), intermediate)
		if err != nil {
			t.Fatal(err)
		}
		if idx >= 0 {
			t.Errorf("intermediate key %d unexpectedly found at %d", intermediate, idx)
			continue
		}
		insertionPoint := -1 - idx
		if insertionPoint <= 0 || insertionPoint >= int64(len(array)) {
			t.Errorf("insertion point %d out of range for len=%d", insertionPoint, len(array))
			continue
		}
		if array[insertionPoint] <= intermediate {
			t.Errorf("array[insertionPoint=%d]=%d not > %d", insertionPoint, array[insertionPoint], intermediate)
		}
		if array[insertionPoint-1] >= intermediate {
			t.Errorf("array[insertionPoint-1=%d]=%d not < %d", insertionPoint-1, array[insertionPoint-1], intermediate)
		}
	}
}

// writeAndOpenDirectMonotonic flushes values via the writer and opens
// a reader over the resulting meta/data streams.
func writeAndOpenDirectMonotonic(t *testing.T, values []int64, blockShift int) (*DirectMonotonicReader, []byte, []byte) {
	t.Helper()
	meta := newTrackingByteOutput(64)
	data := newTrackingByteOutput(64)
	w, err := NewDirectMonotonicWriter(meta, data, int64(len(values)), blockShift)
	if err != nil {
		t.Fatalf("NewDirectMonotonicWriter: %v", err)
	}
	for _, v := range values {
		if err := w.Add(v); err != nil {
			t.Fatalf("Add(%d): %v", v, err)
		}
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	parsedMeta, err := LoadDirectMonotonicMeta(store.NewByteArrayDataInput(meta.Bytes()), int64(len(values)), blockShift)
	if err != nil {
		t.Fatalf("LoadDirectMonotonicMeta: %v", err)
	}
	reader, err := NewDirectMonotonicReader(parsedMeta, &byteSliceRandomAccess{data: data.Bytes()})
	if err != nil {
		t.Fatalf("NewDirectMonotonicReader: %v", err)
	}
	return reader, data.Bytes(), meta.Bytes()
}

func linearSequence(start, step int64, n int) []int64 {
	out := make([]int64, n)
	for i := range out {
		out[i] = start + step*int64(i)
	}
	return out
}

func noisySequence(start, step int64, jitter float64, n int, seed int64) []int64 {
	r := rand.New(rand.NewSource(seed))
	out := make([]int64, n)
	cur := start
	for i := range out {
		cur += step + int64(r.Float64()*jitter*float64(step))
		out[i] = cur
	}
	return out
}

// guard against unused imports if signatures move later
var _ = binary.LittleEndian
