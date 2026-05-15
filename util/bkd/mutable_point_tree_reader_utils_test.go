// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package bkd

import (
	"bytes"
	"math/rand/v2"
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// testPoint is the Go equivalent of TestMutablePointTreeReaderUtils.Point
// from the Lucene reference test. The packed value is stored in a
// BytesRef whose offset is intentionally non-zero so the helpers cannot
// silently ignore it.
type testPoint struct {
	packedValue *util.BytesRef
	doc         int
}

// newTestPoint constructs a testPoint that mirrors the Java
// constructor: it prepends one random byte before the packed value and
// sets Offset=1 so the BytesRef's Offset is exercised by the adapter.
func newTestPoint(packed []byte, doc int, rng *rand.Rand) *testPoint {
	bytes := make([]byte, len(packed)+1)
	bytes[0] = byte(rng.IntN(256))
	copy(bytes[1:], packed)
	return &testPoint{
		packedValue: &util.BytesRef{
			Bytes:  bytes,
			Offset: 1,
			Length: len(packed),
		},
		doc: doc,
	}
}

// dummyPointsReader is the Go equivalent of
// TestMutablePointTreeReaderUtils.DummyPointsReader. It satisfies the
// [MutablePointTree] surface against a backing []*testPoint slice and
// implements the Save/Restore scratch contract used by the stable MSB
// radix sorter.
type dummyPointsReader struct {
	points []*testPoint
	temp   []*testPoint
}

func newDummyPointsReader(points []*testPoint) *dummyPointsReader {
	cloned := make([]*testPoint, len(points))
	copy(cloned, points)
	return &dummyPointsReader{points: cloned}
}

func (r *dummyPointsReader) GetValue(i int, dst *util.BytesRef) {
	pv := r.points[i].packedValue
	dst.Bytes = pv.Bytes
	dst.Offset = pv.Offset
	dst.Length = pv.Length
}

func (r *dummyPointsReader) GetByteAt(i, k int) byte {
	pv := r.points[i].packedValue
	return pv.Bytes[pv.Offset+k]
}

func (r *dummyPointsReader) GetDocID(i int) int { return r.points[i].doc }

func (r *dummyPointsReader) Swap(i, j int) {
	r.points[i], r.points[j] = r.points[j], r.points[i]
}

func (r *dummyPointsReader) Save(i, j int) {
	if r.temp == nil {
		r.temp = make([]*testPoint, len(r.points))
	}
	r.temp[j] = r.points[i]
}

func (r *dummyPointsReader) Restore(i, j int) {
	if r.temp == nil {
		return
	}
	copy(r.points[i:j], r.temp[i:j])
}

// ---------------------------------------------------------------------
// Sort tests
// ---------------------------------------------------------------------

func TestSortMutablePointTree(t *testing.T) {
	t.Parallel()
	for iter := 0; iter < 10; iter++ {
		doTestSortMutablePointTree(t, false)
	}
}

func TestSortMutablePointTree_IncrementalDocID(t *testing.T) {
	t.Parallel()
	for iter := 0; iter < 10; iter++ {
		doTestSortMutablePointTree(t, true)
	}
}

func doTestSortMutablePointTree(t *testing.T, isDocIDIncremental bool) {
	t.Helper()
	rng := rand.New(rand.NewPCG(uint64(rand.Int64()), uint64(rand.Int64())))

	bytesPerDim := nextIntInclusive(rng, 1, 16)
	// Keep maxDoc bounded so the test runs in reasonable time.
	maxDoc := nextIntInclusive(rng, 1, 1<<rng.IntN(20)+1)
	config, err := NewBKDConfig(1, 1, bytesPerDim, DefaultMaxPointsInLeafNode)
	if err != nil {
		t.Fatalf("NewBKDConfig: %v", err)
	}
	points := createRandomPoints(rng, config, maxDoc, make([]int, 1), isDocIDIncremental)
	reader := newDummyPointsReader(points)

	SortMutablePointTree(config, maxDoc, reader, 0, len(points))

	// Reference ordering: stable sort by packed value, then doc ID.
	expected := make([]*testPoint, len(points))
	copy(expected, points)
	sort.SliceStable(expected, func(i, j int) bool {
		cmp := bytes.Compare(
			expected[i].packedValue.Bytes[expected[i].packedValue.Offset:expected[i].packedValue.Offset+expected[i].packedValue.Length],
			expected[j].packedValue.Bytes[expected[j].packedValue.Offset:expected[j].packedValue.Offset+expected[j].packedValue.Length],
		)
		if cmp != 0 {
			return cmp < 0
		}
		return expected[i].doc < expected[j].doc
	})

	if len(expected) != len(reader.points) {
		t.Fatalf("length mismatch: expected %d got %d", len(expected), len(reader.points))
	}

	var prev *testPoint
	for i := range expected {
		if !bytesRefEqualValues(expected[i].packedValue, reader.points[i].packedValue) {
			t.Fatalf("packedValue mismatch at %d", i)
		}
		// Same pointer expected: SortMutablePointTree never copies the
		// BytesRef payload; it only reorders the underlying slot pointers.
		if expected[i].packedValue != reader.points[i].packedValue {
			t.Fatalf("packedValue identity diverged at %d", i)
		}
		if prev != nil &&
			bytesRefEqualValues(reader.points[i].packedValue, prev.packedValue) &&
			reader.points[i].doc < prev.doc {
			t.Fatalf("doc IDs out of order at %d: prev=%d cur=%d", i, prev.doc, reader.points[i].doc)
		}
		prev = reader.points[i]
	}
}

// ---------------------------------------------------------------------
// SortByDim tests
// ---------------------------------------------------------------------

func TestSortMutablePointTreeByDim(t *testing.T) {
	t.Parallel()
	for iter := 0; iter < 5; iter++ {
		doTestSortMutablePointTreeByDim(t)
	}
}

func doTestSortMutablePointTreeByDim(t *testing.T) {
	t.Helper()
	rng := rand.New(rand.NewPCG(uint64(rand.Int64()), uint64(rand.Int64())))

	config := createRandomConfig(rng)
	maxDoc := nextIntInclusive(rng, 1, 1<<rng.IntN(20)+1)
	commonPrefixLengths := make([]int, config.NumDims())
	points := createRandomPoints(rng, config, maxDoc, commonPrefixLengths, false)
	reader := newDummyPointsReader(points)
	sortedDim := rng.IntN(config.NumIndexDims())

	SortMutablePointTreeByDim(
		config, sortedDim, commonPrefixLengths,
		reader, 0, len(points),
		&util.BytesRef{}, &util.BytesRef{},
	)

	bytesPerDim := config.BytesPerDim()
	indexBytes := config.PackedIndexBytesLength()
	packedBytes := config.PackedBytesLength()
	dataDimsLength := (config.NumDims() - config.NumIndexDims()) * bytesPerDim
	offset := sortedDim * bytesPerDim

	for i := 1; i < len(points); i++ {
		prev := reader.points[i-1].packedValue
		cur := reader.points[i].packedValue
		cmp := bytes.Compare(
			prev.Bytes[prev.Offset+offset:prev.Offset+offset+bytesPerDim],
			cur.Bytes[cur.Offset+offset:cur.Offset+offset+bytesPerDim],
		)
		if cmp == 0 {
			cmp = bytes.Compare(
				prev.Bytes[prev.Offset+indexBytes:prev.Offset+indexBytes+dataDimsLength],
				cur.Bytes[cur.Offset+indexBytes:cur.Offset+indexBytes+dataDimsLength],
			)
			if cmp == 0 {
				cmp = reader.points[i-1].doc - reader.points[i].doc
			}
		}
		if cmp > 0 {
			t.Fatalf("order violation at %d (sortedDim=%d, packedBytes=%d)", i, sortedDim, packedBytes)
		}
	}
}

// ---------------------------------------------------------------------
// Partition tests
// ---------------------------------------------------------------------

func TestPartitionMutablePointTree(t *testing.T) {
	t.Parallel()
	for iter := 0; iter < 5; iter++ {
		doTestPartitionMutablePointTree(t)
	}
}

func doTestPartitionMutablePointTree(t *testing.T) {
	t.Helper()
	rng := rand.New(rand.NewPCG(uint64(rand.Int64()), uint64(rand.Int64())))

	config := createRandomConfig(rng)
	commonPrefixLengths := make([]int, config.NumDims())
	maxDoc := nextIntInclusive(rng, 1, 1<<rng.IntN(20)+1)
	points := createRandomPoints(rng, config, maxDoc, commonPrefixLengths, false)
	splitDim := rng.IntN(config.NumIndexDims())
	reader := newDummyPointsReader(points)
	pivot := rng.IntN(len(points))

	PartitionMutablePointTree(
		config, maxDoc, splitDim, commonPrefixLengths[splitDim],
		reader, 0, len(points), pivot,
		&util.BytesRef{}, &util.BytesRef{},
	)

	bytesPerDim := config.BytesPerDim()
	offset := splitDim * bytesPerDim
	indexBytes := config.PackedIndexBytesLength()
	dataDimsLength := (config.NumDims() - config.NumIndexDims()) * bytesPerDim
	pivotValue := reader.points[pivot].packedValue
	pivotDoc := reader.points[pivot].doc

	for i, p := range reader.points {
		v := p.packedValue
		cmp := bytes.Compare(
			v.Bytes[v.Offset+offset:v.Offset+offset+bytesPerDim],
			pivotValue.Bytes[pivotValue.Offset+offset:pivotValue.Offset+offset+bytesPerDim],
		)
		if cmp == 0 {
			cmp = bytes.Compare(
				v.Bytes[v.Offset+indexBytes:v.Offset+indexBytes+dataDimsLength],
				pivotValue.Bytes[pivotValue.Offset+indexBytes:pivotValue.Offset+indexBytes+dataDimsLength],
			)
			if cmp == 0 {
				cmp = p.doc - pivotDoc
			}
		}
		switch {
		case i < pivot:
			if cmp > 0 {
				t.Fatalf("left side violation at %d (pivot=%d, splitDim=%d): cmp=%d", i, pivot, splitDim, cmp)
			}
		case i > pivot:
			if cmp < 0 {
				t.Fatalf("right side violation at %d (pivot=%d, splitDim=%d): cmp=%d", i, pivot, splitDim, cmp)
			}
		default:
			if cmp != 0 {
				t.Fatalf("pivot position broken: cmp=%d", cmp)
			}
		}
	}
}

// ---------------------------------------------------------------------
// Edge-case targeted tests
// ---------------------------------------------------------------------

// TestSortMutablePointTree_MaxDocOne verifies the boundary where
// maxDoc-1 == 0 (so bitsRequired returns 1). The radix key includes one
// trailing byte for the doc id; with all doc ids zero, the entire range
// must sort purely by packed value.
func TestSortMutablePointTree_MaxDocOne(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewPCG(0xC0FFEE, 0xBADF00D))

	config, err := NewBKDConfig(1, 1, 4, DefaultMaxPointsInLeafNode)
	if err != nil {
		t.Fatalf("NewBKDConfig: %v", err)
	}
	points := make([]*testPoint, 32)
	for i := range points {
		v := make([]byte, 4)
		for j := range v {
			v[j] = byte(rng.IntN(256))
		}
		points[i] = newTestPoint(v, 0, rng)
	}
	reader := newDummyPointsReader(points)

	SortMutablePointTree(config, 1, reader, 0, len(points))

	for i := 1; i < len(reader.points); i++ {
		prev := reader.points[i-1].packedValue
		cur := reader.points[i].packedValue
		if bytes.Compare(prev.ValidBytes(), cur.ValidBytes()) > 0 {
			t.Fatalf("maxDoc=1 sort produced out-of-order pair at %d", i)
		}
	}
}

// TestSortMutablePointTree_SingleElement verifies the trivial
// boundary: a one-element range is already sorted and must not panic.
func TestSortMutablePointTree_SingleElement(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewPCG(1, 2))
	config, err := NewBKDConfig(1, 1, 2, DefaultMaxPointsInLeafNode)
	if err != nil {
		t.Fatalf("NewBKDConfig: %v", err)
	}
	points := []*testPoint{newTestPoint([]byte{0xAA, 0xBB}, 0, rng)}
	reader := newDummyPointsReader(points)
	SortMutablePointTree(config, 1, reader, 0, 1)
	if reader.points[0].doc != 0 {
		t.Fatalf("single-element sort altered the doc id")
	}
}

// TestPartitionMutablePointTree_PivotAtBounds covers the two extreme
// pivot positions (0 and n-1) to make sure the half-empty side does
// not produce false assertions.
func TestPartitionMutablePointTree_PivotAtBounds(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewPCG(7, 13))

	config, err := NewBKDConfig(2, 2, 4, DefaultMaxPointsInLeafNode)
	if err != nil {
		t.Fatalf("NewBKDConfig: %v", err)
	}
	for _, pivot := range []int{0, 99} {
		points := make([]*testPoint, 100)
		for i := range points {
			v := make([]byte, config.PackedBytesLength())
			for j := range v {
				v[j] = byte(rng.IntN(256))
			}
			points[i] = newTestPoint(v, rng.IntN(1000), rng)
		}
		reader := newDummyPointsReader(points)
		PartitionMutablePointTree(
			config, 1000, 0, 0,
			reader, 0, len(points), pivot,
			&util.BytesRef{}, &util.BytesRef{},
		)
		bytesPerDim := config.BytesPerDim()
		pv := reader.points[pivot].packedValue
		for i := 0; i < pivot; i++ {
			v := reader.points[i].packedValue
			if bytes.Compare(
				v.Bytes[v.Offset:v.Offset+bytesPerDim],
				pv.Bytes[pv.Offset:pv.Offset+bytesPerDim],
			) > 0 {
				t.Fatalf("pivot=%d: left side > pivot at %d on the split dim", pivot, i)
			}
		}
		for i := pivot + 1; i < len(reader.points); i++ {
			v := reader.points[i].packedValue
			if bytes.Compare(
				v.Bytes[v.Offset:v.Offset+bytesPerDim],
				pv.Bytes[pv.Offset:pv.Offset+bytesPerDim],
			) < 0 {
				t.Fatalf("pivot=%d: right side < pivot at %d on the split dim", pivot, i)
			}
		}
	}
}

// ---------------------------------------------------------------------
// Test helpers (parallels TestMutablePointTreeReaderUtils helpers).
// ---------------------------------------------------------------------

// createRandomConfig mirrors the Java createRandomConfig helper: random
// numDims/numIndexDims within the package bounds and a random bytesPerDim.
func createRandomConfig(rng *rand.Rand) BKDConfig {
	numIndexDims := nextIntInclusive(rng, 1, MaxIndexDims)
	numDims := nextIntInclusive(rng, numIndexDims, MaxDims)
	bytesPerDim := nextIntInclusive(rng, 1, 16)
	maxPointsInLeafNode := nextIntInclusive(rng, 50, 2000)
	config, err := NewBKDConfig(numDims, numIndexDims, bytesPerDim, maxPointsInLeafNode)
	if err != nil {
		panic(err)
	}
	return config
}

// createRandomPoints mirrors the Java createRandomPoints helper. It
// builds two shapes: a "primary" mode where all dimensions are random
// and a common prefix is injected per dim, and a "secondary" mode (with
// 1/10 probability) where index dims are shared and only data dims
// vary. The commonPrefixLengths slice is filled with the per-dim prefix
// lengths used to build the data; callers feed it to sortByDim /
// partition.
func createRandomPoints(rng *rand.Rand, config BKDConfig, maxDoc int, commonPrefixLengths []int, isDocIDIncremental bool) []*testPoint {
	if len(commonPrefixLengths) != config.NumDims() {
		panic("commonPrefixLengths length must equal config.NumDims()")
	}
	numPoints := nextIntInclusive(rng, 1, 1000)
	points := make([]*testPoint, numPoints)

	primaryShape := rng.IntN(10) != 0
	if primaryShape {
		for i := 0; i < numPoints; i++ {
			value := make([]byte, config.PackedBytesLength())
			for j := range value {
				value[j] = byte(rng.IntN(256))
			}
			doc := pickDoc(rng, i, maxDoc, isDocIDIncremental)
			points[i] = newTestPoint(value, doc, rng)
		}
		for i := 0; i < config.NumDims(); i++ {
			commonPrefixLengths[i] = nextIntInclusive(rng, 0, config.BytesPerDim())
		}
		firstValue := points[0].packedValue
		for i := 1; i < numPoints; i++ {
			for dim := 0; dim < config.NumDims(); dim++ {
				offset := dim * config.BytesPerDim()
				pv := points[i].packedValue
				copy(
					pv.Bytes[pv.Offset+offset:pv.Offset+offset+commonPrefixLengths[dim]],
					firstValue.Bytes[firstValue.Offset+offset:firstValue.Offset+offset+commonPrefixLengths[dim]],
				)
			}
		}
		return points
	}

	// Secondary shape: index dims are equal, data dims differ.
	numDataDims := config.NumDims() - config.NumIndexDims()
	indexDims := make([]byte, config.PackedIndexBytesLength())
	for j := range indexDims {
		indexDims[j] = byte(rng.IntN(256))
	}
	dataDims := make([]byte, numDataDims*config.BytesPerDim())
	for i := 0; i < numPoints; i++ {
		value := make([]byte, config.PackedBytesLength())
		copy(value, indexDims)
		for j := range dataDims {
			dataDims[j] = byte(rng.IntN(256))
		}
		copy(value[config.PackedIndexBytesLength():], dataDims)
		doc := pickDoc(rng, i, maxDoc, isDocIDIncremental)
		points[i] = newTestPoint(value, doc, rng)
	}
	for i := 0; i < config.NumIndexDims(); i++ {
		commonPrefixLengths[i] = config.BytesPerDim()
	}
	for i := config.NumIndexDims(); i < config.NumDims(); i++ {
		commonPrefixLengths[i] = nextIntInclusive(rng, 0, config.BytesPerDim())
	}
	firstValue := points[0].packedValue
	for i := 1; i < numPoints; i++ {
		for dim := config.NumIndexDims(); dim < config.NumDims(); dim++ {
			offset := dim * config.BytesPerDim()
			pv := points[i].packedValue
			copy(
				pv.Bytes[pv.Offset+offset:pv.Offset+offset+commonPrefixLengths[dim]],
				firstValue.Bytes[firstValue.Offset+offset:firstValue.Offset+offset+commonPrefixLengths[dim]],
			)
		}
	}
	return points
}

func pickDoc(rng *rand.Rand, i, maxDoc int, isDocIDIncremental bool) int {
	if isDocIDIncremental {
		if i < maxDoc-1 {
			return i
		}
		return maxDoc - 1
	}
	return rng.IntN(maxDoc)
}

func nextIntInclusive(rng *rand.Rand, lo, hi int) int {
	if hi < lo {
		panic("nextIntInclusive: hi < lo")
	}
	return lo + rng.IntN(hi-lo+1)
}

func bytesRefEqualValues(a, b *util.BytesRef) bool {
	return bytes.Equal(
		a.Bytes[a.Offset:a.Offset+a.Length],
		b.Bytes[b.Offset:b.Offset+b.Length],
	)
}
