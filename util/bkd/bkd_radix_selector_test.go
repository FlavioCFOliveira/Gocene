// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package bkd

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// testEnv bundles a directory and BKDRadixSelector for table-driven
// tests; callers close the directory in a deferred call.
type testEnv struct {
	dir      *store.ByteBuffersDirectory
	selector *BKDRadixSelector
	config   BKDConfig
}

func newTestEnv(t *testing.T, cfg BKDConfig, maxPointsSortInHeap int) *testEnv {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	sel, err := NewBKDRadixSelector(cfg, maxPointsSortInHeap, dir, "test")
	if err != nil {
		t.Fatalf("NewBKDRadixSelector: %v", err)
	}
	t.Cleanup(func() { _ = dir.Close() })
	return &testEnv{dir: dir, selector: sel, config: cfg}
}

// mustConfig builds a BKDConfig or aborts the test.
func mustConfig(t *testing.T, numDims, numIndexDims, bytesPerDim, maxPointsInLeaf int) BKDConfig {
	t.Helper()
	cfg, err := NewBKDConfig(numDims, numIndexDims, bytesPerDim, maxPointsInLeaf)
	if err != nil {
		t.Fatalf("NewBKDConfig: %v", err)
	}
	return cfg
}

// fillHeapWriter appends the supplied (packed, docID) pairs to a fresh
// HeapPointWriter sized exactly to len(points).
func fillHeapWriter(t *testing.T, cfg BKDConfig, points []selectorPoint) *HeapPointWriter {
	t.Helper()
	w := NewHeapPointWriter(cfg, len(points))
	for _, p := range points {
		if err := w.Append(p.packed, p.docID); err != nil {
			t.Fatalf("HeapPointWriter.Append: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("HeapPointWriter.Close: %v", err)
	}
	return w
}

// fillOfflineWriter appends the supplied points to a fresh
// OfflinePointWriter backed by `dir`.
func fillOfflineWriter(t *testing.T, cfg BKDConfig, dir store.Directory, points []selectorPoint) *OfflinePointWriter {
	t.Helper()
	w, err := NewOfflinePointWriter(cfg, dir, "test", "src", int64(len(points)))
	if err != nil {
		t.Fatalf("NewOfflinePointWriter: %v", err)
	}
	for _, p := range points {
		if err := w.Append(p.packed, p.docID); err != nil {
			t.Fatalf("OfflinePointWriter.Append: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("OfflinePointWriter.Close: %v", err)
	}
	return w
}

// selectorPoint is a tiny fixture record matching the (packed, docID)
// constructor pair of PointWriter.
type selectorPoint struct {
	packed []byte
	docID  int
}

// readAllPoints materialises every (packed, docID) pair in `slice` by
// iterating the writer's reader. The packed slices are copied so the
// caller can hold them past the next reader advance.
func readAllPoints(t *testing.T, cfg BKDConfig, slice PathSlice) []selectorPoint {
	t.Helper()
	rd, err := slice.Writer.GetReader(slice.Start, slice.Count)
	if err != nil {
		t.Fatalf("GetReader: %v", err)
	}
	defer rd.Close()

	var out []selectorPoint
	for {
		hasNext, err := rd.Next()
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if !hasNext {
			break
		}
		pv := rd.PointValue()
		bytesCopy := append([]byte(nil), pv.PackedValue().Bytes[pv.PackedValue().Offset:pv.PackedValue().Offset+pv.PackedValue().Length]...)
		out = append(out, selectorPoint{packed: bytesCopy, docID: pv.DocID()})
	}
	return out
}

// dimSlice returns the bytesPerDim window of `packed` corresponding to
// `dim`. The returned slice aliases `packed`.
func dimSlice(cfg BKDConfig, packed []byte, dim int) []byte {
	off := dim * cfg.BytesPerDim()
	return packed[off : off+cfg.BytesPerDim()]
}

// dataDimsSlice returns the data-only-dim block of `packed`. The
// returned slice aliases `packed`.
func dataDimsSlice(cfg BKDConfig, packed []byte) []byte {
	return packed[cfg.PackedIndexBytesLength():cfg.PackedBytesLength()]
}

// verifySelect mirrors the Java TestBKDRadixSelector.verify helper:
// it checks slice counts, the max-vs-min invariant on the split dim,
// data-dim and docID tie-breaks, and that the returned partitionPoint
// equals the min of the right slice's dim. It does NOT destroy the
// slices (the caller owns lifetime).
func verifySelect(
	t *testing.T,
	cfg BKDConfig,
	leftSlice, rightSlice PathSlice,
	partitionPoint []byte,
	splitDim int,
	expectedLeft, expectedRight int64,
) {
	t.Helper()
	if leftSlice.Count != expectedLeft {
		t.Fatalf("left count: got %d want %d", leftSlice.Count, expectedLeft)
	}
	if rightSlice.Count != expectedRight {
		t.Fatalf("right count: got %d want %d", rightSlice.Count, expectedRight)
	}
	left := readAllPoints(t, cfg, leftSlice)
	right := readAllPoints(t, cfg, rightSlice)

	// Determine max(left) and min(right) on the split dim.
	maxDim := make([]byte, cfg.BytesPerDim())
	if len(left) > 0 {
		copy(maxDim, dimSlice(cfg, left[0].packed, splitDim))
		for _, p := range left[1:] {
			if bytes.Compare(dimSlice(cfg, p.packed, splitDim), maxDim) > 0 {
				copy(maxDim, dimSlice(cfg, p.packed, splitDim))
			}
		}
	}
	minDim := make([]byte, cfg.BytesPerDim())
	for i := range minDim {
		minDim[i] = 0xFF
	}
	if len(right) > 0 {
		copy(minDim, dimSlice(cfg, right[0].packed, splitDim))
		for _, p := range right[1:] {
			if bytes.Compare(dimSlice(cfg, p.packed, splitDim), minDim) < 0 {
				copy(minDim, dimSlice(cfg, p.packed, splitDim))
			}
		}
	}

	if len(left) > 0 && len(right) > 0 {
		cmp := bytes.Compare(maxDim, minDim)
		if cmp > 0 {
			t.Fatalf("max(left)=%x must be <= min(right)=%x", maxDim, minDim)
		}
		if cmp == 0 {
			// Tie on split dim -> check data-dim ordering when the
			// configuration has data-only dims.
			dataLen := (cfg.NumDims() - cfg.NumIndexDims()) * cfg.BytesPerDim()
			if dataLen > 0 {
				maxDataDim := maxDataDimOnTie(cfg, left, splitDim, maxDim)
				minDataDim := minDataDimOnTie(cfg, right, splitDim, minDim)
				dcmp := bytes.Compare(maxDataDim, minDataDim)
				if dcmp > 0 {
					t.Fatalf("max(left.data)=%x must be <= min(right.data)=%x", maxDataDim, minDataDim)
				}
				if dcmp == 0 {
					maxDoc := maxDocIDOnTie(cfg, left, splitDim, partitionPoint, maxDataDim)
					minDoc := minDocIDOnTie(cfg, right, splitDim, partitionPoint, minDataDim)
					if minDoc < maxDoc {
						t.Fatalf("on full tie: min docID of right (%d) must be >= max docID of left (%d)", minDoc, maxDoc)
					}
				}
			} else {
				// No data-dims: tie-break must have been on docID.
				maxDoc := maxDocIDOnTie(cfg, left, splitDim, partitionPoint, nil)
				minDoc := minDocIDOnTie(cfg, right, splitDim, partitionPoint, nil)
				if minDoc < maxDoc {
					t.Fatalf("on full tie: min docID of right (%d) must be >= max docID of left (%d)", minDoc, maxDoc)
				}
			}
		}
	}

	if len(right) > 0 {
		if !bytes.Equal(partitionPoint, minDim) {
			t.Fatalf("partitionPoint mismatch: got %x, want min(right)=%x", partitionPoint, minDim)
		}
	}
}

func maxDataDimOnTie(cfg BKDConfig, points []selectorPoint, splitDim int, dimVal []byte) []byte {
	dataLen := (cfg.NumDims() - cfg.NumIndexDims()) * cfg.BytesPerDim()
	maxData := make([]byte, dataLen)
	first := true
	for _, p := range points {
		if !bytes.Equal(dimSlice(cfg, p.packed, splitDim), dimVal) {
			continue
		}
		dd := dataDimsSlice(cfg, p.packed)
		if first || bytes.Compare(dd, maxData) > 0 {
			copy(maxData, dd)
			first = false
		}
	}
	return maxData
}

func minDataDimOnTie(cfg BKDConfig, points []selectorPoint, splitDim int, dimVal []byte) []byte {
	dataLen := (cfg.NumDims() - cfg.NumIndexDims()) * cfg.BytesPerDim()
	minData := make([]byte, dataLen)
	for i := range minData {
		minData[i] = 0xFF
	}
	for _, p := range points {
		if !bytes.Equal(dimSlice(cfg, p.packed, splitDim), dimVal) {
			continue
		}
		dd := dataDimsSlice(cfg, p.packed)
		if bytes.Compare(dd, minData) < 0 {
			copy(minData, dd)
		}
	}
	return minData
}

func maxDocIDOnTie(cfg BKDConfig, points []selectorPoint, splitDim int, dimVal, dataDim []byte) int {
	doc := -1 // -1 stands in for "no point matched the tie criteria" so callers compare cleanly
	for _, p := range points {
		if !bytes.Equal(dimSlice(cfg, p.packed, splitDim), dimVal) {
			continue
		}
		if dataDim != nil && !bytes.Equal(dataDimsSlice(cfg, p.packed), dataDim) {
			continue
		}
		if p.docID > doc {
			doc = p.docID
		}
	}
	return doc
}

func minDocIDOnTie(cfg BKDConfig, points []selectorPoint, splitDim int, dimVal, dataDim []byte) int {
	doc := -1
	for _, p := range points {
		if !bytes.Equal(dimSlice(cfg, p.packed, splitDim), dimVal) {
			continue
		}
		if dataDim != nil && !bytes.Equal(dataDimsSlice(cfg, p.packed), dataDim) {
			continue
		}
		if doc == -1 || p.docID < doc {
			doc = p.docID
		}
	}
	return doc
}

// intToSortableBytes is the Go equivalent of NumericUtils.intToSortableBytes
// from Lucene: writes a big-endian 4-byte representation of value with the
// sign bit flipped so unsigned byte comparison matches int ordering.
func intToSortableBytes(value int32, out []byte) {
	binary.BigEndian.PutUint32(out, uint32(value)^0x80000000)
}

// TestNewBKDRadixSelector_Errors covers the constructor input validation.
func TestNewBKDRadixSelector_Errors(t *testing.T) {
	cfg := mustConfig(t, 1, 1, 4, DefaultMaxPointsInLeafNode)
	if _, err := NewBKDRadixSelector(cfg, 100, nil, "test"); err == nil {
		t.Fatalf("expected error for nil tempDir")
	}
	if _, err := NewBKDRadixSelector(cfg, -1, store.NewByteBuffersDirectory(), "test"); err == nil {
		t.Fatalf("expected error for negative maxPointsSortInHeap")
	}
}

// TestBKDRadixSelector_Select_CheckArgs verifies the partitionPoint
// range invariants.
func TestBKDRadixSelector_Select_CheckArgs(t *testing.T) {
	cfg := mustConfig(t, 1, 1, 4, DefaultMaxPointsInLeafNode)
	env := newTestEnv(t, cfg, 100)
	hw := NewHeapPointWriter(cfg, 4)
	_ = hw.Close()
	slice := PathSlice{Writer: hw, Start: 0, Count: 4}
	slices := make([]PathSlice, 2)
	if _, err := env.selector.Select(slice, slices, 0, 4, -1, 0, 0); err == nil {
		t.Fatalf("expected error for partitionPoint < from")
	}
	if _, err := env.selector.Select(slice, slices, 0, 4, 4, 0, 0); err == nil {
		t.Fatalf("expected error for partitionPoint >= to")
	}
	if _, err := env.selector.Select(slice, slices[:1], 0, 4, 1, 0, 0); err == nil {
		t.Fatalf("expected error for partitionSlices length < 2")
	}
}

// TestBKDRadixSelector_BasicHeap is the Go counterpart of Lucene's
// testBasic: heap writer, four distinct points, middle=2, verifying
// the partition byte and slice counts.
func TestBKDRadixSelector_BasicHeap(t *testing.T) {
	cfg := mustConfig(t, 1, 1, 4, DefaultMaxPointsInLeafNode)
	points := make([]selectorPoint, 4)
	for i := 0; i < 4; i++ {
		buf := make([]byte, cfg.PackedBytesLength())
		intToSortableBytes(int32(i+1), buf)
		points[i] = selectorPoint{packed: buf, docID: i}
	}
	env := newTestEnv(t, cfg, 1000)
	hw := fillHeapWriter(t, cfg, points)
	slice := PathSlice{Writer: hw, Start: 0, Count: 4}

	slices := make([]PathSlice, 2)
	pp, err := env.selector.Select(slice, slices, 0, 4, 2, 0, 0)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	verifySelect(t, cfg, slices[0], slices[1], pp, 0, 2, 2)
	// In the heap path, both slices point at the same backing writer.
	if slices[0].Writer != hw || slices[1].Writer != hw {
		t.Fatalf("heap path should return slices over the same writer")
	}
}

// TestBKDRadixSelector_Heap_Boundaries exercises k=0 and k=count-1 on
// the heap path. These cases stress the radix selector tail logic.
func TestBKDRadixSelector_Heap_Boundaries(t *testing.T) {
	cfg := mustConfig(t, 1, 1, 4, DefaultMaxPointsInLeafNode)
	const n = 16
	points := make([]selectorPoint, n)
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < n; i++ {
		buf := make([]byte, cfg.PackedBytesLength())
		rng.Read(buf)
		points[i] = selectorPoint{packed: buf, docID: i}
	}
	env := newTestEnv(t, cfg, 1000)

	tests := []struct {
		name           string
		partitionPoint int64
		expectedLeft   int64
		expectedRight  int64
	}{
		{"k=1 (count-1 left)", 1, 1, n - 1},
		{"k=count-1", n - 1, n - 1, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hw := fillHeapWriter(t, cfg, points)
			slices := make([]PathSlice, 2)
			pp, err := env.selector.Select(
				PathSlice{Writer: hw, Start: 0, Count: n},
				slices, 0, n, tc.partitionPoint, 0, 0,
			)
			if err != nil {
				t.Fatalf("Select: %v", err)
			}
			verifySelect(t, cfg, slices[0], slices[1], pp, 0, tc.expectedLeft, tc.expectedRight)
		})
	}
}

// TestBKDRadixSelector_Offline_SmallRange exercises the offline path
// with very few points; the maxPointsSortInHeap budget is intentionally
// kept below the point count so the offline pipeline is invoked.
func TestBKDRadixSelector_Offline_SmallRange(t *testing.T) {
	cfg := mustConfig(t, 1, 1, 4, DefaultMaxPointsInLeafNode)
	const n = 8
	points := make([]selectorPoint, n)
	rng := rand.New(rand.NewSource(2))
	for i := 0; i < n; i++ {
		buf := make([]byte, cfg.PackedBytesLength())
		rng.Read(buf)
		points[i] = selectorPoint{packed: buf, docID: i}
	}
	env := newTestEnv(t, cfg, 0) // force offline
	ow := fillOfflineWriter(t, cfg, env.dir, points)

	slices := make([]PathSlice, 2)
	pp, err := env.selector.Select(
		PathSlice{Writer: ow, Start: 0, Count: n},
		slices, 0, n, n/2, 0, 0,
	)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	verifySelect(t, cfg, slices[0], slices[1], pp, 0, n/2, n/2)
	if slices[0].Writer == ow || slices[1].Writer == ow {
		t.Fatalf("offline path should allocate fresh writers")
	}
}

// TestBKDRadixSelector_Offline_AllEqual exercises the "common prefix
// equals bytesSorted" branch, where every point is byte-for-byte
// identical and the partition reduces to a count-based split using
// docIDs as tie-breakers.
func TestBKDRadixSelector_Offline_AllEqual(t *testing.T) {
	cfg := mustConfig(t, 1, 1, 4, DefaultMaxPointsInLeafNode)
	const n = 16
	common := make([]byte, cfg.PackedBytesLength())
	rng := rand.New(rand.NewSource(3))
	rng.Read(common)
	points := make([]selectorPoint, n)
	for i := 0; i < n; i++ {
		// Distinct docIDs to exercise the docID tie-breaker.
		points[i] = selectorPoint{packed: append([]byte(nil), common...), docID: i}
	}

	env := newTestEnv(t, cfg, 0)
	ow := fillOfflineWriter(t, cfg, env.dir, points)

	slices := make([]PathSlice, 2)
	pp, err := env.selector.Select(
		PathSlice{Writer: ow, Start: 0, Count: n},
		slices, 0, n, n/2, 0, 0,
	)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	verifySelect(t, cfg, slices[0], slices[1], pp, 0, n/2, n/2)
}

// TestBKDRadixSelector_Offline_TwoValuesByDocID covers the case where
// two distinct values appear repeatedly but the split must straddle
// equal values, so the docID tie-break is exercised via the offline
// path. Mirrors testRandomLastByteTwoValues from Lucene.
func TestBKDRadixSelector_Offline_TwoValuesByDocID(t *testing.T) {
	cfg := mustConfig(t, 1, 1, 4, DefaultMaxPointsInLeafNode)
	const n = 32
	a := make([]byte, cfg.PackedBytesLength())
	b := make([]byte, cfg.PackedBytesLength())
	rng := rand.New(rand.NewSource(4))
	rng.Read(a)
	copy(b, a)
	b[len(b)-1] ^= 0x01 // only the last byte differs
	points := make([]selectorPoint, n)
	for i := 0; i < n; i++ {
		if rng.Intn(2) == 0 {
			points[i] = selectorPoint{packed: append([]byte(nil), a...), docID: 1}
		} else {
			points[i] = selectorPoint{packed: append([]byte(nil), b...), docID: 2}
		}
	}

	env := newTestEnv(t, cfg, 0)
	ow := fillOfflineWriter(t, cfg, env.dir, points)

	pp, err := env.selector.Select(
		PathSlice{Writer: ow, Start: 0, Count: n},
		make([]PathSlice, 2), 0, n, n/2, 0, 0,
	)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	// Reload via the writers returned from Select: we cannot verify
	// against the destroyed source. Run a second select on a fresh
	// instance so the lifecycle is testable.
	ow2 := fillOfflineWriter(t, cfg, env.dir, points)
	slices := make([]PathSlice, 2)
	pp, err = env.selector.Select(
		PathSlice{Writer: ow2, Start: 0, Count: n},
		slices, 0, n, n/2, 0, 0,
	)
	if err != nil {
		t.Fatalf("Select (2nd): %v", err)
	}
	verifySelect(t, cfg, slices[0], slices[1], pp, 0, n/2, n/2)
}

// TestBKDRadixSelector_Offline_MultiDim exercises a multi-dimensional
// configuration where the split dimension can vary. We sweep every
// indexed dimension and check the invariants for each.
func TestBKDRadixSelector_Offline_MultiDim(t *testing.T) {
	cfg := mustConfig(t, 4, 2, 3, DefaultMaxPointsInLeafNode)
	const n = 64
	rng := rand.New(rand.NewSource(5))
	srcPoints := make([]selectorPoint, n)
	for i := 0; i < n; i++ {
		buf := make([]byte, cfg.PackedBytesLength())
		rng.Read(buf)
		srcPoints[i] = selectorPoint{packed: buf, docID: i}
	}

	for splitDim := 0; splitDim < cfg.NumIndexDims(); splitDim++ {
		t.Run(fmt.Sprintf("splitDim=%d", splitDim), func(t *testing.T) {
			env := newTestEnv(t, cfg, 0)
			ow := fillOfflineWriter(t, cfg, env.dir, srcPoints)
			slices := make([]PathSlice, 2)
			pp, err := env.selector.Select(
				PathSlice{Writer: ow, Start: 0, Count: n},
				slices, 0, n, n/2, splitDim, 0,
			)
			if err != nil {
				t.Fatalf("Select: %v", err)
			}
			verifySelect(t, cfg, slices[0], slices[1], pp, splitDim, n/2, n/2)
		})
	}
}

// TestBKDRadixSelector_Offline_MediumRandom is the medium-size random
// fuzz test counterpart of Lucene's testRandomBinaryMedium (scaled
// down). Each iteration picks a fresh random config, generates random
// points, and verifies the partition invariants across both heap and
// offline pipelines.
func TestBKDRadixSelector_Offline_MediumRandom(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping medium random test in -short mode")
	}
	rng := rand.New(rand.NewSource(6))
	const iterations = 8
	for iter := 0; iter < iterations; iter++ {
		t.Run(fmt.Sprintf("iter=%d", iter), func(t *testing.T) {
			cfg := randomConfig(t, rng)
			n := 200 + rng.Intn(800)
			points := randomPoints(rng, cfg, n)
			splitDim := rng.Intn(cfg.NumIndexDims())
			partitionPoint := 1 + rng.Intn(n-2)
			maxOnHeap := rng.Intn(2 * n)

			env := newTestEnv(t, cfg, maxOnHeap)

			// Heap path
			heap := fillHeapWriter(t, cfg, points)
			heapSlices := make([]PathSlice, 2)
			pp, err := env.selector.Select(
				PathSlice{Writer: heap, Start: 0, Count: int64(n)},
				heapSlices, 0, int64(n), int64(partitionPoint), splitDim, 0,
			)
			if err != nil {
				t.Fatalf("Select (heap): %v", err)
			}
			verifySelect(t, cfg, heapSlices[0], heapSlices[1], pp, splitDim, int64(partitionPoint), int64(n-partitionPoint))

			// Offline path on the same points
			offline := fillOfflineWriter(t, cfg, env.dir, points)
			offSlices := make([]PathSlice, 2)
			pp, err = env.selector.Select(
				PathSlice{Writer: offline, Start: 0, Count: int64(n)},
				offSlices, 0, int64(n), int64(partitionPoint), splitDim, 0,
			)
			if err != nil {
				t.Fatalf("Select (offline): %v", err)
			}
			verifySelect(t, cfg, offSlices[0], offSlices[1], pp, splitDim, int64(partitionPoint), int64(n-partitionPoint))
		})
	}
}

// TestBKDRadixSelector_Heap_DataDimsTieBreak constructs a configuration
// with data-only dimensions and forces every point onto the same
// index-dim value, so the tie must be broken on the data dims (and
// docID).
func TestBKDRadixSelector_Heap_DataDimsTieBreak(t *testing.T) {
	cfg := mustConfig(t, 3, 1, 4, DefaultMaxPointsInLeafNode) // 1 index dim, 2 data dims
	const n = 32
	rng := rand.New(rand.NewSource(7))
	// Shared index-dim bytes; data dims vary.
	indexBytes := make([]byte, cfg.BytesPerDim())
	rng.Read(indexBytes)
	dataLen := (cfg.NumDims() - cfg.NumIndexDims()) * cfg.BytesPerDim()

	points := make([]selectorPoint, n)
	for i := 0; i < n; i++ {
		buf := make([]byte, cfg.PackedBytesLength())
		copy(buf[:cfg.BytesPerDim()], indexBytes)
		data := make([]byte, dataLen)
		rng.Read(data)
		copy(buf[cfg.PackedIndexBytesLength():], data)
		points[i] = selectorPoint{packed: buf, docID: i}
	}

	env := newTestEnv(t, cfg, 1000)
	heap := fillHeapWriter(t, cfg, points)
	slices := make([]PathSlice, 2)
	pp, err := env.selector.Select(
		PathSlice{Writer: heap, Start: 0, Count: n},
		slices, 0, n, n/2, 0, 0,
	)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	verifySelect(t, cfg, slices[0], slices[1], pp, 0, n/2, n/2)
}

// TestBKDRadixSelector_Offline_DimCommonPrefixHint feeds Select a
// non-zero dimCommonPrefix hint to ensure the offline pipeline honours
// the caller-supplied prefix length.
func TestBKDRadixSelector_Offline_DimCommonPrefixHint(t *testing.T) {
	cfg := mustConfig(t, 1, 1, 4, DefaultMaxPointsInLeafNode)
	const n = 32
	rng := rand.New(rand.NewSource(8))
	shared := []byte{0xAB, 0xCD}
	points := make([]selectorPoint, n)
	for i := 0; i < n; i++ {
		buf := make([]byte, cfg.PackedBytesLength())
		copy(buf, shared) // first 2 bytes shared
		rng.Read(buf[len(shared):])
		points[i] = selectorPoint{packed: buf, docID: i}
	}

	env := newTestEnv(t, cfg, 0)
	ow := fillOfflineWriter(t, cfg, env.dir, points)
	slices := make([]PathSlice, 2)
	pp, err := env.selector.Select(
		PathSlice{Writer: ow, Start: 0, Count: n},
		slices, 0, n, n/2, 0, len(shared),
	)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	verifySelect(t, cfg, slices[0], slices[1], pp, 0, n/2, n/2)
	// And the partition byte must start with the shared prefix.
	for i, b := range shared {
		if pp[i] != b {
			t.Fatalf("partitionPoint[%d]=0x%02x, want shared prefix byte 0x%02x", i, pp[i], b)
		}
	}
}

// TestBKDRadixSelector_OfflineSubrange exercises a non-zero start
// offset: Select must operate on [from, to) inside a larger writer.
func TestBKDRadixSelector_OfflineSubrange(t *testing.T) {
	cfg := mustConfig(t, 2, 2, 4, DefaultMaxPointsInLeafNode)
	const n = 32
	rng := rand.New(rand.NewSource(9))
	points := randomPoints(rng, cfg, n)

	env := newTestEnv(t, cfg, 0)
	ow := fillOfflineWriter(t, cfg, env.dir, points)
	var start, end int64 = 4, 28
	slices := make([]PathSlice, 2)
	pp, err := env.selector.Select(
		PathSlice{Writer: ow, Start: start, Count: end - start},
		slices, start, end, (start+end)/2, 0, 0,
	)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	verifySelect(t, cfg, slices[0], slices[1], pp, 0, (end-start)/2, (end-start)/2)
}

// TestBKDRadixSelector_PathSlice ensures the PathSlice helper preserves
// field values.
func TestBKDRadixSelector_PathSlice(t *testing.T) {
	cfg := mustConfig(t, 1, 1, 4, DefaultMaxPointsInLeafNode)
	hw := NewHeapPointWriter(cfg, 1)
	slice := NewPathSlice(hw, 5, 10)
	if slice.Writer != hw || slice.Start != 5 || slice.Count != 10 {
		t.Fatalf("PathSlice fields not preserved: %+v", slice)
	}
}

// TestToIntExact exercises the overflow guard.
func TestToIntExact(t *testing.T) {
	if _, err := toIntExact(int64(1)<<40, "x"); err == nil {
		t.Fatalf("expected overflow error")
	}
	if v, err := toIntExact(42, "x"); err != nil || v != 42 {
		t.Fatalf("expected (42, nil), got (%d, %v)", v, err)
	}
}

// TestMismatchRange covers the parity helper.
func TestMismatchRange(t *testing.T) {
	if got := mismatchRange([]byte{1, 2, 3}, []byte{1, 2, 3}); got != -1 {
		t.Fatalf("equal slices: got %d want -1", got)
	}
	if got := mismatchRange([]byte{1, 2, 3}, []byte{1, 9, 3}); got != 1 {
		t.Fatalf("first diff at 1: got %d", got)
	}
	if got := mismatchRange([]byte{1, 2}, []byte{1, 2, 3}); got != 2 {
		t.Fatalf("shorter prefix: got %d want 2", got)
	}
}

// TestBKDRadixSelector_HeapRadixSort verifies the in-place leaf
// ordering helper used by callers that need a fully sorted heap.
func TestBKDRadixSelector_HeapRadixSort(t *testing.T) {
	cfg := mustConfig(t, 1, 1, 4, DefaultMaxPointsInLeafNode)
	const n = 64
	rng := rand.New(rand.NewSource(10))
	points := randomPoints(rng, cfg, n)
	heap := fillHeapWriter(t, cfg, points)

	sel, err := NewBKDRadixSelector(cfg, 1000, store.NewByteBuffersDirectory(), "test")
	if err != nil {
		t.Fatalf("NewBKDRadixSelector: %v", err)
	}
	sel.HeapRadixSort(heap, 0, n, 0, 0)

	// After sorting, the dim=0 bytes must be non-decreasing.
	prev := make([]byte, cfg.BytesPerDim())
	for i := 0; i < n; i++ {
		pv := heap.GetPackedValueSlice(i)
		packed := pv.PackedValue()
		cur := packed.Bytes[packed.Offset : packed.Offset+cfg.BytesPerDim()]
		if i > 0 && bytes.Compare(prev, cur) > 0 {
			t.Fatalf("HeapRadixSort: out of order at index %d: prev=%x cur=%x", i, prev, cur)
		}
		copy(prev, cur)
	}
}

// randomConfig returns a fresh BKDConfig with small parameters
// suitable for fast tests. Mirrors Lucene's getRandomConfig but bounded
// tighter for the Go suite.
func randomConfig(t *testing.T, rng *rand.Rand) BKDConfig {
	t.Helper()
	numIndexDims := 1 + rng.Intn(4)       // 1..4
	numDims := numIndexDims + rng.Intn(3) // up to 2 extra data-only dims
	bytesPerDim := 2 + rng.Intn(5)        // 2..6
	maxPointsInLeaf := 50 + rng.Intn(500) // 50..549
	cfg, err := NewBKDConfig(numDims, numIndexDims, bytesPerDim, maxPointsInLeaf)
	if err != nil {
		t.Fatalf("randomConfig: %v", err)
	}
	return cfg
}

// randomPoints generates `n` random points compatible with `cfg`,
// docIDs assigned by sequence index.
func randomPoints(rng *rand.Rand, cfg BKDConfig, n int) []selectorPoint {
	out := make([]selectorPoint, n)
	for i := 0; i < n; i++ {
		buf := make([]byte, cfg.PackedBytesLength())
		rng.Read(buf)
		out[i] = selectorPoint{packed: buf, docID: i}
	}
	return out
}

// TestBKDRadixSelector_ErrorPropagation feeds Select a PointWriter
// implementation that is neither heap nor offline; the selector must
// reject it. Re-uses the package-wide stubPointWriter from
// point_reader_writer_test.go.
func TestBKDRadixSelector_ErrorPropagation(t *testing.T) {
	cfg := mustConfig(t, 1, 1, 4, DefaultMaxPointsInLeafNode)
	env := newTestEnv(t, cfg, 100)
	slice := PathSlice{Writer: &stubPointWriter{}, Start: 0, Count: 4}
	if _, err := env.selector.Select(slice, make([]PathSlice, 2), 0, 4, 2, 0, 0); err == nil {
		t.Fatalf("expected error for unknown writer type")
	}
}

// TestBKDRadixSelector_Offline_FewDifferentValues mirrors Lucene's
// testRandomFewDifferentValues: a small alphabet of distinct values is
// drawn repeatedly so the histogram concentrates mass into a handful
// of buckets, exercising the recursive delta partition path.
func TestBKDRadixSelector_Offline_FewDifferentValues(t *testing.T) {
	cfg := mustConfig(t, 2, 2, 3, DefaultMaxPointsInLeafNode)
	const n = 256
	rng := rand.New(rand.NewSource(11))
	alphabet := 4 + rng.Intn(4) // 4..7 distinct packed values
	values := make([][]byte, alphabet)
	for i := range values {
		values[i] = make([]byte, cfg.PackedBytesLength())
		rng.Read(values[i])
	}
	points := make([]selectorPoint, n)
	for i := 0; i < n; i++ {
		v := values[rng.Intn(alphabet)]
		points[i] = selectorPoint{packed: append([]byte(nil), v...), docID: i}
	}

	env := newTestEnv(t, cfg, 0)
	ow := fillOfflineWriter(t, cfg, env.dir, points)
	slices := make([]PathSlice, 2)
	pp, err := env.selector.Select(
		PathSlice{Writer: ow, Start: 0, Count: n},
		slices, 0, n, n/2, 0, 0,
	)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	verifySelect(t, cfg, slices[0], slices[1], pp, 0, n/2, n/2)
}

// TestBKDRadixSelector_Offline_AllDocIDsEqual exercises a degenerate
// configuration where every point shares the same docID. Tie-breaking
// then has nothing left to break on and the partition reduces to
// counting bytes from the partitionBucket trail.
func TestBKDRadixSelector_Offline_AllDocIDsEqual(t *testing.T) {
	cfg := mustConfig(t, 1, 1, 4, DefaultMaxPointsInLeafNode)
	const n = 64
	rng := rand.New(rand.NewSource(12))
	points := make([]selectorPoint, n)
	for i := 0; i < n; i++ {
		buf := make([]byte, cfg.PackedBytesLength())
		rng.Read(buf)
		points[i] = selectorPoint{packed: buf, docID: 0}
	}
	env := newTestEnv(t, cfg, 0)
	ow := fillOfflineWriter(t, cfg, env.dir, points)
	slices := make([]PathSlice, 2)
	pp, err := env.selector.Select(
		PathSlice{Writer: ow, Start: 0, Count: n},
		slices, 0, n, n/2, 0, 0,
	)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	verifySelect(t, cfg, slices[0], slices[1], pp, 0, n/2, n/2)
}

// TestBKDRadixSelector_Heap_DataDimDiffOnly creates points with
// identical indexed dims but different data dims, exercising the
// data-dim tie-breaker in the heap pipeline. Mirrors a slice of
// Lucene's testRandomDataDimDiffValues.
func TestBKDRadixSelector_Heap_DataDimDiffOnly(t *testing.T) {
	cfg := mustConfig(t, 3, 1, 3, DefaultMaxPointsInLeafNode)
	const n = 48
	rng := rand.New(rand.NewSource(13))
	idx := make([]byte, cfg.BytesPerDim())
	rng.Read(idx)
	dataLen := (cfg.NumDims() - cfg.NumIndexDims()) * cfg.BytesPerDim()
	points := make([]selectorPoint, n)
	for i := 0; i < n; i++ {
		buf := make([]byte, cfg.PackedBytesLength())
		copy(buf, idx)
		data := make([]byte, dataLen)
		rng.Read(data)
		copy(buf[cfg.PackedIndexBytesLength():], data)
		points[i] = selectorPoint{packed: buf, docID: i}
	}
	env := newTestEnv(t, cfg, 1000)
	heap := fillHeapWriter(t, cfg, points)
	slices := make([]PathSlice, 2)
	pp, err := env.selector.Select(
		PathSlice{Writer: heap, Start: 0, Count: n},
		slices, 0, n, n/2, 0, 0,
	)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	verifySelect(t, cfg, slices[0], slices[1], pp, 0, n/2, n/2)
}
