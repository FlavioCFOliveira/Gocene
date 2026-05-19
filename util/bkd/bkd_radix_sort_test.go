// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package bkd

import (
	"bytes"
	"math/rand"
	"testing"
)

// Port of org.apache.lucene.util.bkd.TestBKDRadixSort from Lucene
// 10.4.0 (commit 9983b7c). The tests exercise
// BKDRadixSelector.HeapRadixSort across every split dimension, after
// loading a HeapPointWriter with values produced by a deterministic
// per-subtest RNG. verifySort asserts that the sort is total over
// (split dim, data dims, docID) — matching the Java assertion chain.
//
// Helpers newTestEnv and mustConfig live in bkd_radix_selector_test.go
// (same package) and are reused here.

func TestBKDRadixSort_Random(t *testing.T) {
	rng := rand.New(rand.NewSource(0xBCD00))
	cfg := randomRadixSortConfig(t, rng)
	numPoints := nextIntInRange(rng, 1, DefaultMaxPointsInLeafNode)
	points := NewHeapPointWriter(cfg, numPoints)
	value := make([]byte, cfg.PackedBytesLength())
	for i := 0; i < numPoints; i++ {
		rng.Read(value)
		if err := points.Append(value, i); err != nil {
			t.Fatalf("Append[%d]: %v", i, err)
		}
	}
	verifyRadixSort(t, rng, cfg, points, 0, numPoints)
}

func TestBKDRadixSort_RandomAllEquals(t *testing.T) {
	rng := rand.New(rand.NewSource(0xBCD01))
	cfg := randomRadixSortConfig(t, rng)
	numPoints := nextIntInRange(rng, 1, DefaultMaxPointsInLeafNode)
	points := NewHeapPointWriter(cfg, numPoints)
	value := make([]byte, cfg.PackedBytesLength())
	rng.Read(value)
	for i := 0; i < numPoints; i++ {
		if err := points.Append(value, rng.Intn(numPoints)); err != nil {
			t.Fatalf("Append[%d]: %v", i, err)
		}
	}
	verifyRadixSort(t, rng, cfg, points, 0, numPoints)
}

func TestBKDRadixSort_RandomLastByteTwoValues(t *testing.T) {
	rng := rand.New(rand.NewSource(0xBCD02))
	cfg := randomRadixSortConfig(t, rng)
	numPoints := nextIntInRange(rng, 1, DefaultMaxPointsInLeafNode)
	points := NewHeapPointWriter(cfg, numPoints)
	value := make([]byte, cfg.PackedBytesLength())
	rng.Read(value)
	for i := 0; i < numPoints; i++ {
		docID := 2
		if rng.Intn(2) == 0 {
			docID = 1
		}
		if err := points.Append(value, docID); err != nil {
			t.Fatalf("Append[%d]: %v", i, err)
		}
	}
	verifyRadixSort(t, rng, cfg, points, 0, numPoints)
}

func TestBKDRadixSort_RandomFewDifferentValues(t *testing.T) {
	rng := rand.New(rand.NewSource(0xBCD03))
	cfg := randomRadixSortConfig(t, rng)
	numPoints := nextIntInRange(rng, 1, DefaultMaxPointsInLeafNode)
	points := NewHeapPointWriter(cfg, numPoints)
	numberValues := rng.Intn(8) + 2
	different := make([][]byte, numberValues)
	for i := range different {
		different[i] = make([]byte, cfg.PackedBytesLength())
		rng.Read(different[i])
	}
	for i := 0; i < numPoints; i++ {
		if err := points.Append(different[rng.Intn(numberValues)], i); err != nil {
			t.Fatalf("Append[%d]: %v", i, err)
		}
	}
	verifyRadixSort(t, rng, cfg, points, 0, numPoints)
}

func TestBKDRadixSort_RandomDataDimDifferent(t *testing.T) {
	rng := rand.New(rand.NewSource(0xBCD04))
	cfg := randomRadixSortConfig(t, rng)
	numPoints := nextIntInRange(rng, 1, DefaultMaxPointsInLeafNode)
	points := NewHeapPointWriter(cfg, numPoints)
	value := make([]byte, cfg.PackedBytesLength())
	rng.Read(value)
	totalDataDims := cfg.NumDims() - cfg.NumIndexDims()
	dataLen := totalDataDims * cfg.BytesPerDim()
	dataValues := make([]byte, dataLen)
	for i := 0; i < numPoints; i++ {
		rng.Read(dataValues)
		copy(value[cfg.PackedIndexBytesLength():cfg.PackedIndexBytesLength()+dataLen], dataValues)
		if err := points.Append(value, rng.Intn(numPoints)); err != nil {
			t.Fatalf("Append[%d]: %v", i, err)
		}
	}
	verifyRadixSort(t, rng, cfg, points, 0, numPoints)
}

// verifyRadixSort mirrors verifySort from Lucene's TestBKDRadixSort: it
// runs HeapRadixSort once per split dimension and asserts the resulting
// order respects (split dim bytes, data dim bytes, docID).
func verifyRadixSort(t *testing.T, rng *rand.Rand, cfg BKDConfig, points *HeapPointWriter, start, end int) {
	t.Helper()
	env := newTestEnv(t, cfg, 1000)
	for splitDim := 0; splitDim < cfg.NumDims(); splitDim++ {
		env.selector.HeapRadixSort(points, start, end, splitDim, randomCommonPrefix(rng, cfg, points, start, end, splitDim))
		previous := make([]byte, cfg.PackedBytesLength())
		previousDocID := -1
		dimOffset := splitDim * cfg.BytesPerDim()
		for j := start; j < end; j++ {
			pv := points.GetPackedValueSlice(j)
			br := pv.PackedValue()
			cmp := bytes.Compare(
				br.Bytes[br.Offset+dimOffset:br.Offset+dimOffset+cfg.BytesPerDim()],
				previous[dimOffset:dimOffset+cfg.BytesPerDim()],
			)
			if cmp < 0 {
				t.Fatalf("split dim %d, index %d: split-dim bytes out of order", splitDim, j)
			}
			if cmp == 0 {
				dataOffset := cfg.NumIndexDims() * cfg.BytesPerDim()
				cmp = bytes.Compare(
					br.Bytes[br.Offset+dataOffset:br.Offset+cfg.PackedBytesLength()],
					previous[dataOffset:cfg.PackedBytesLength()],
				)
				if cmp < 0 {
					t.Fatalf("split dim %d, index %d: data-dim bytes out of order", splitDim, j)
				}
			}
			if cmp == 0 && pv.DocID() < previousDocID {
				t.Fatalf("split dim %d, index %d: docID %d < previous %d", splitDim, j, pv.DocID(), previousDocID)
			}
			copy(previous, br.Bytes[br.Offset:br.Offset+cfg.PackedBytesLength()])
			previousDocID = pv.DocID()
		}
	}
}

// randomCommonPrefix returns a common-prefix length less than or equal
// to the actual common prefix of the split dimension across [start,end).
func randomCommonPrefix(rng *rand.Rand, cfg BKDConfig, points *HeapPointWriter, start, end, sortDim int) int {
	commonPrefixLength := cfg.BytesPerDim()
	first := points.GetPackedValueSlice(start).PackedValue()
	offset := sortDim * cfg.BytesPerDim()
	firstValue := make([]byte, cfg.BytesPerDim())
	copy(firstValue, first.Bytes[first.Offset+offset:first.Offset+offset+cfg.BytesPerDim()])
	for i := start + 1; i < end; i++ {
		br := points.GetPackedValueSlice(i).PackedValue()
		diff := mismatchAt(br.Bytes[br.Offset+offset:br.Offset+offset+cfg.BytesPerDim()], firstValue)
		if diff != -1 && commonPrefixLength > diff {
			if diff == 0 {
				return 0
			}
			commonPrefixLength = diff
		}
	}
	if rng.Intn(2) == 0 {
		return commonPrefixLength
	}
	return rng.Intn(commonPrefixLength)
}

// mismatchAt returns the first index at which a and b differ, or -1 if
// equal. Matches java.util.Arrays.mismatch semantics for equal-length
// slices, which is the only shape required here.
func mismatchAt(a, b []byte) int {
	n := len(a)
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return -1
}

// randomRadixSortConfig mirrors getRandomConfig from TestBKDRadixSort:
// numIndexDims in [1, MaxIndexDims], numDims in [numIndexDims, MaxDims],
// bytesPerDim in [2, 30], maxPointsInLeafNode in [50, 2000].
func randomRadixSortConfig(t *testing.T, rng *rand.Rand) BKDConfig {
	t.Helper()
	numIndexDims := nextIntInRange(rng, 1, MaxIndexDims)
	numDims := nextIntInRange(rng, numIndexDims, MaxDims)
	bytesPerDim := nextIntInRange(rng, 2, 30)
	maxPointsInLeaf := nextIntInRange(rng, 50, 2000)
	return mustConfig(t, numDims, numIndexDims, bytesPerDim, maxPointsInLeaf)
}

// nextIntInRange returns a uniformly distributed integer in [lo, hi],
// matching TestUtil.nextInt(random, lo, hi) from Lucene's test harness.
func nextIntInRange(rng *rand.Rand, lo, hi int) int {
	if hi < lo {
		hi = lo
	}
	return lo + rng.Intn(hi-lo+1)
}
