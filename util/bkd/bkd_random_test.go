// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package bkd

import (
	"math/rand"
	"os"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// This file ports the randomised BKD tests from Apache Lucene 10.4.0's
// TestBKD (lucene/core/src/test/.../bkd/TestBKD.java). Each test
// generates randomised point data with specific structural properties
// and validates tree correctness via the verify() helper.

// TestBKD_RandomIntsNDims ports testRandomIntsNDims: random N-dim int
// points with a random sub-range query verified against ground truth.
func TestBKD_RandomIntsNDims(t *testing.T) {
	rng := verifyRNG(t)
	numDocs := 100 + rng.Intn(901) // ~100-1000
	numDims := 1 + rng.Intn(5)     // [1, 5]
	numIndexDims := 1 + rng.Intn(numDims) // [1, numDims]

	docValues := make([][][]byte, numDocs)
	for docID := 0; docID < numDocs; docID++ {
		values := make([][]byte, numDims)
		for dim := 0; dim < numDims; dim++ {
			buf := make([]byte, 4)
			rng.Read(buf)
			values[dim] = buf
		}
		docValues[docID] = values
	}
	verify(t, rng, docValues, nil, numDims, numIndexDims, 4)
}

// TestBKD_BigIntNDims ports testBigIntNDims: random points with variable
// byte width (2..30 bytes per dim), which exercises the common-prefix
// compression paths in the BKD leaf format.
func TestBKD_BigIntNDims(t *testing.T) {
	rng := verifyRNG(t)
	numBytesPerDim := 2 + rng.Intn(29) // [2, 30]
	numDataDims := 1 + rng.Intn(MaxDims)
	numIndexDims := 1 + rng.Intn(numDataDims)
	if numIndexDims > MaxIndexDims {
		numIndexDims = MaxIndexDims
	}
	numDocs := 100 + rng.Intn(901) // ~100-1000

	docValues := make([][][]byte, numDocs)
	for docID := 0; docID < numDocs; docID++ {
		values := make([][]byte, numDataDims)
		for dim := 0; dim < numDataDims; dim++ {
			buf := make([]byte, numBytesPerDim)
			rng.Read(buf)
			values[dim] = buf
		}
		docValues[docID] = values
	}
	verify(t, rng, docValues, nil, numDataDims, numIndexDims, numBytesPerDim)
}

// TestBKD_RandomBinaryTiny ports testRandomBinaryTiny: doTestRandomBinary(10).
func TestBKD_RandomBinaryTiny(t *testing.T) {
	rng := verifyRNG(t)
	doTestRandomBinary(t, rng, 10)
}

// TestBKD_RandomBinaryMedium ports testRandomBinaryMedium: doTestRandomBinary(10000).
func TestBKD_RandomBinaryMedium(t *testing.T) {
	rng := verifyRNG(t)
	doTestRandomBinary(t, rng, 10000)
}

// TestBKD_RandomBinaryBig ports testRandomBinaryBig (@Nightly in Java):
// doTestRandomBinary(200000). Gated behind GOCENE_RUN_MONSTERS=1.
func TestBKD_RandomBinaryBig(t *testing.T) {
	if os.Getenv("GOCENE_RUN_MONSTERS") != "1" {
		t.Fatalf("deferred: monster test %s requires GOCENE_RUN_MONSTERS=1 (200k points)", t.Name())
	}
	rng := verifyRNG(t)
	doTestRandomBinary(t, rng, 200000)
}

func doTestRandomBinary(t *testing.T, rng *rand.Rand, count int) {
	t.Helper()
	numDocs := count + rng.Intn(count) // [count, count*2)
	numBytesPerDim := 2 + rng.Intn(29) // [2, 30]
	numDataDims := 1 + rng.Intn(MaxDims)
	numIndexDims := 1 + rng.Intn(numDataDims)
	if numIndexDims > MaxIndexDims {
		numIndexDims = MaxIndexDims
	}

	docValues := make([][][]byte, numDocs)
	for docID := 0; docID < numDocs; docID++ {
		values := make([][]byte, numDataDims)
		for dim := 0; dim < numDataDims; dim++ {
			buf := make([]byte, numBytesPerDim)
			rng.Read(buf)
			values[dim] = buf
		}
		docValues[docID] = values
	}
	verify(t, rng, docValues, nil, numDataDims, numIndexDims, numBytesPerDim)
}

// TestBKD_AllEqual ports testAllEqual: every doc has the same packed
// value across every dim; exercises the single-value leaf compression.
func TestBKD_AllEqual(t *testing.T) {
	rng := verifyRNG(t)
	numBytesPerDim := 2 + rng.Intn(29) // [2, 30]
	numDataDims := 1 + rng.Intn(MaxDims)
	numIndexDims := 1 + rng.Intn(numDataDims)
	if numIndexDims > MaxIndexDims {
		numIndexDims = MaxIndexDims
	}
	numDocs := 100 + rng.Intn(901) // ~100-1000

	// Generate a single set of dimension values.
	dimValues := make([][]byte, numDataDims)
	for dim := 0; dim < numDataDims; dim++ {
		buf := make([]byte, numBytesPerDim)
		rng.Read(buf)
		dimValues[dim] = buf
	}

	docValues := make([][][]byte, numDocs)
	for docID := 0; docID < numDocs; docID++ {
		values := make([][]byte, numDataDims)
		for dim := 0; dim < numDataDims; dim++ {
			values[dim] = dimValues[dim]
		}
		docValues[docID] = values
	}
	verify(t, rng, docValues, nil, numDataDims, numIndexDims, numBytesPerDim)
}

// TestBKD_IndexDimEqualDataDimDifferent ports testIndexDimEqualDataDimDifferent:
// index dims share a single value; data dims vary.
func TestBKD_IndexDimEqualDataDimDifferent(t *testing.T) {
	rng := verifyRNG(t)
	numBytesPerDim := 2 + rng.Intn(29) // [2, 30]
	numDataDims := 2 + rng.Intn(MaxDims-1) // [2, MaxDims]
	numIndexDims := 1 + rng.Intn(numDataDims-1) // [1, numDataDims-1]
	if numIndexDims > MaxIndexDims {
		numIndexDims = MaxIndexDims
	}
	numDocs := 100 + rng.Intn(901) // ~100-1000

	// Generate a fixed value for each index dim.
	indexDimValues := make([][]byte, numIndexDims)
	for dim := 0; dim < numIndexDims; dim++ {
		buf := make([]byte, numBytesPerDim)
		rng.Read(buf)
		indexDimValues[dim] = buf
	}

	docValues := make([][][]byte, numDocs)
	for docID := 0; docID < numDocs; docID++ {
		values := make([][]byte, numDataDims)
		for dim := 0; dim < numIndexDims; dim++ {
			values[dim] = indexDimValues[dim]
		}
		for dim := numIndexDims; dim < numDataDims; dim++ {
			buf := make([]byte, numBytesPerDim)
			rng.Read(buf)
			values[dim] = buf
		}
		docValues[docID] = values
	}
	verify(t, rng, docValues, nil, numDataDims, numIndexDims, numBytesPerDim)
}

// TestBKD_OneDimLowCard ports testOneDimLowCard: one dim takes one of
// two values, forcing many splits on that dim.
func TestBKD_OneDimLowCard(t *testing.T) {
	rng := verifyRNG(t)
	numBytesPerDim := 2 + rng.Intn(29) // [2, 30]
	numDataDims := 2 + rng.Intn(MaxDims-1) // [2, MaxDims]
	numIndexDims := 2 + rng.Intn(numDataDims-1) // [2, numDataDims-1]
	if numIndexDims > MaxIndexDims {
		numIndexDims = MaxIndexDims
	}
	numDocs := 1000 + rng.Intn(9001) // ~1000-10000

	theLowCardDim := rng.Intn(numDataDims)
	value1 := make([]byte, numBytesPerDim)
	rng.Read(value1)
	value2 := make([]byte, numBytesPerDim)
	copy(value2, value1)
	if value2[numBytesPerDim-1] == 0 || rng.Intn(2) == 0 {
		value2[numBytesPerDim-1]++
	} else {
		value2[numBytesPerDim-1]--
	}

	docValues := make([][][]byte, numDocs)
	for docID := 0; docID < numDocs; docID++ {
		values := make([][]byte, numDataDims)
		for dim := 0; dim < numDataDims; dim++ {
			if dim == theLowCardDim {
				if rng.Intn(2) == 0 {
					values[dim] = value1
				} else {
					values[dim] = value2
				}
			} else {
				buf := make([]byte, numBytesPerDim)
				rng.Read(buf)
				values[dim] = buf
			}
		}
		docValues[docID] = values
	}
	// Use small leaf size to force a lot of splitting.
	verifyWithConfig(t, rng, docValues, nil, numDataDims, numIndexDims, numBytesPerDim, 20+rng.Intn(31))
}

// TestBKD_OneDimTwoValues ports testOneDimTwoValues: one dim takes one
// of two values; should trigger run-length compression with run lengths
// greater than 255.
func TestBKD_OneDimTwoValues(t *testing.T) {
	rng := verifyRNG(t)
	numBytesPerDim := 2 + rng.Intn(29) // [2, 30]
	numDataDims := 1 + rng.Intn(MaxDims)
	numIndexDims := 1 + rng.Intn(numDataDims)
	if numIndexDims > MaxIndexDims {
		numIndexDims = MaxIndexDims
	}
	numDocs := 1000 + rng.Intn(9001) // ~1000-10000

	theDim := rng.Intn(numDataDims)
	value1 := make([]byte, numBytesPerDim)
	rng.Read(value1)
	value2 := make([]byte, numBytesPerDim)
	rng.Read(value2)

	docValues := make([][][]byte, numDocs)
	for docID := 0; docID < numDocs; docID++ {
		values := make([][]byte, numDataDims)
		for dim := 0; dim < numDataDims; dim++ {
			if dim == theDim {
				if rng.Intn(2) == 0 {
					values[dim] = value1
				} else {
					values[dim] = value2
				}
			} else {
				buf := make([]byte, numBytesPerDim)
				rng.Read(buf)
				values[dim] = buf
			}
		}
		docValues[docID] = values
	}
	verify(t, rng, docValues, nil, numDataDims, numIndexDims, numBytesPerDim)
}

// TestBKD_RandomFewDifferentValues ports testRandomFewDifferentValues:
// few cardinalities across many docs, exercising the low-cardinality
// leaf path.
func TestBKD_RandomFewDifferentValues(t *testing.T) {
	rng := verifyRNG(t)
	numBytesPerDim := 2 + rng.Intn(29) // [2, 30]
	numDataDims := 1 + rng.Intn(MaxDims)
	numIndexDims := 1 + rng.Intn(numDataDims)
	if numIndexDims > MaxIndexDims {
		numIndexDims = MaxIndexDims
	}
	numDocs := 1000 + rng.Intn(9001) // ~1000-10000

	// Generate 5 distinct value sets.
	distinctValues := make([][][]byte, 5)
	for i := 0; i < 5; i++ {
		vals := make([][]byte, numDataDims)
		for dim := 0; dim < numDataDims; dim++ {
			buf := make([]byte, numBytesPerDim)
			rng.Read(buf)
			vals[dim] = buf
		}
		distinctValues[i] = vals
	}

	docValues := make([][][]byte, numDocs)
	for docID := 0; docID < numDocs; docID++ {
		docValues[docID] = distinctValues[rng.Intn(5)]
	}
	verify(t, rng, docValues, nil, numDataDims, numIndexDims, numBytesPerDim)
}

// TestBKD_MultiValued ports testMultiValued: a single doc carries
// multiple packed values; checks the BKD writer/reader path against
// a multi-valued points scenario. Uses a docIDs array to map each
// point to its owning doc.
func TestBKD_MultiValued(t *testing.T) {
	rng := verifyRNG(t)
	numBytesPerDim := 2 + rng.Intn(9) // [2, 10]
	numDataDims := 1 + rng.Intn(MaxDims)
	numIndexDims := 1 + rng.Intn(numDataDims)
	if numIndexDims > MaxIndexDims {
		numIndexDims = MaxIndexDims
	}
	numDocs := 100 + rng.Intn(101) // ~100-200

	// Generate random per-doc multi-values.
	docValuesList := make([][][]byte, numDocs)        // docValuesList[doc] = list of per-dim arrays
	totalValues := 0
	for docID := 0; docID < numDocs; docID++ {
		numValues := 1 + rng.Intn(10) // [1, 10]
		docVals := make([][]byte, numValues)
		for val := 0; val < numValues; val++ {
			packed := make([]byte, numBytesPerDim*numDataDims)
			rng.Read(packed)
			docVals[val] = packed
		}
		docValuesList[docID] = docVals
		totalValues += numValues
	}

	// Flatten into the verify format: each per-doc point is a separate
	// entry in allValues, with a docIDs array mapping each entry to its doc.
	allValues := make([][][]byte, totalValues)
	docIDs := make([]int, totalValues)
	idx := 0
	for docID := 0; docID < numDocs; docID++ {
		for _, packed := range docValuesList[docID] {
			// Split the flat packed value into per-dim arrays.
			perDim := make([][]byte, numDataDims)
			for dim := 0; dim < numDataDims; dim++ {
				perDim[dim] = packed[dim*numBytesPerDim : (dim+1)*numBytesPerDim]
			}
			allValues[idx] = perDim
			docIDs[idx] = docID
			idx++
		}
	}

	verify(t, rng, allValues, docIDs, numDataDims, numIndexDims, numBytesPerDim)
}

// TestBKD_2DLongOrdsOffline ports test2DLongOrdsOffline: 2D, 8-byte
// dims, offline (disk-backed) writer path. Uses a small maxMB to force
// the writer to spill to disk.
func TestBKD_2DLongOrdsOffline(t *testing.T) {
	rng := verifyRNG(t)
	numDocs := 1000 + rng.Intn(9001) // ~1000-10000
	numDataDims := 2
	numIndexDims := 2
	numBytesPerDim := 8

	_, err := NewBKDConfig(numDataDims, numIndexDims, numBytesPerDim, 64)
	if err != nil {
		t.Fatalf("NewBKDConfig: %v", err)
	}

	docValues := make([][][]byte, numDocs)
	for docID := 0; docID < numDocs; docID++ {
		values := make([][]byte, numDataDims)
		for dim := 0; dim < numDataDims; dim++ {
			buf := make([]byte, numBytesPerDim)
			rng.Read(buf)
			values[dim] = buf
		}
		docValues[docID] = values
	}

	// Use a small maxMB to force offline path. With BytesPerDoc=20
	// (2 dims * 8 bytes + 4-byte docID), maxMB=0.01 gives
	// maxPointsSortInHeap = floor(0.01*1024*1024/20) = 524, which is
	// >= maxPointsInLeafNode (64) and < totalPointCount (~1000-10000),
	// so the writer will spill to disk.
	const maxMB = 0.01

	dir := store.NewByteBuffersDirectory()
	t.Cleanup(func() { _ = dir.Close() })

	verifyWithDir(t, rng, dir, docValues, nil, numDataDims, numIndexDims, numBytesPerDim, 64, maxMB)
}

// TestBKD_WastedLeadingBytes ports testWastedLeadingBytes: every doc
// has the same leading bytes on every dim, exercising the common-prefix
// compression of the leaf block format.
func TestBKD_WastedLeadingBytes(t *testing.T) {
	rng := verifyRNG(t)
	numBytesPerDim := 2 + rng.Intn(29) // [2, 30]
	numDataDims := 1 + rng.Intn(MaxDims)
	numIndexDims := 1 + rng.Intn(numDataDims)
	if numIndexDims > MaxIndexDims {
		numIndexDims = MaxIndexDims
	}
	numDocs := 100 + rng.Intn(901) // ~100-1000

	// Generate a common prefix for each dim.
	commonPrefix := make([][]byte, numDataDims)
	for dim := 0; dim < numDataDims; dim++ {
		buf := make([]byte, numBytesPerDim/2) // first half is common
		rng.Read(buf)
		commonPrefix[dim] = buf
	}

	docValues := make([][][]byte, numDocs)
	for docID := 0; docID < numDocs; docID++ {
		values := make([][]byte, numDataDims)
		for dim := 0; dim < numDataDims; dim++ {
			buf := make([]byte, numBytesPerDim)
			copy(buf, commonPrefix[dim])
			// Fill the remaining bytes randomly.
			rng.Read(buf[numBytesPerDim/2:])
			values[dim] = buf
		}
		docValues[docID] = values
	}
	verify(t, rng, docValues, nil, numDataDims, numIndexDims, numBytesPerDim)
}

// TestBKD_CheckDataDimOptimalOrder ports testCheckDataDimOptimalOrder:
// assertion that the writer reorders data dims to minimise leaf-block
// size when index dims < data dims.
func TestBKD_CheckDataDimOptimalOrder(t *testing.T) {
	rng := verifyRNG(t)
	numBytesPerDim := 2 + rng.Intn(9) // [2, 10]
	// 3 data dims, 2 index dims forces the data-dim reordering path.
	numDataDims := 3
	numIndexDims := 2
	numDocs := 100 + rng.Intn(901) // ~100-1000

	docValues := make([][][]byte, numDocs)
	for docID := 0; docID < numDocs; docID++ {
		values := make([][]byte, numDataDims)
		for dim := 0; dim < numDataDims; dim++ {
			buf := make([]byte, numBytesPerDim)
			rng.Read(buf)
			values[dim] = buf
		}
		docValues[docID] = values
	}
	verify(t, rng, docValues, nil, numDataDims, numIndexDims, numBytesPerDim)
}
