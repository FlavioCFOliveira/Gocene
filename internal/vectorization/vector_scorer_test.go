// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

package vectorization

// Port of org.apache.lucene.internal.vectorization.TestVectorScorer
// (Lucene 10.4.0, lucene/core/src/test/org/apache/lucene/internal/
// vectorization/TestVectorScorer.java).
//
// Java semantics versus Gocene reality
// ------------------------------------
// The Java test exists to prove that the JDK-21 Panama/MemorySegment
// scorer (Lucene99MemorySegmentFlatVectorsScorer) produces scores that
// are numerically equivalent to those returned by the portable
// DefaultFlatVectorScorer fallback. Its @BeforeClass guard
//
//	assumeTrue(MEMSEG_SCORER.getClass() != DEFAULT_SCORER.getClass())
//
// skips the test entirely when the two scorers collapse into one
// (i.e. when no SIMD path is available on the running JVM).
//
// In Gocene the SIMD-vs-fallback split does not exist. The Go port
// folds the role of Lucene99MemorySegmentFlatVectorsScorer directly
// into codecs/hnsw.DefaultFlatVectorScorer (see
// internal/vectorization/lucene99_memory_segment_flat_vectors_scorer.go
// and codecs/hnsw/default_flat_vector_scorer.go). There is exactly one
// FlatVectorsScorer implementation; the Java assumeTrue guard would
// always skip the test, which would defeat the purpose of porting it.
//
// We therefore re-interpret the test as a self-consistency parity
// check across the three observable API surfaces of the single
// portable scorer:
//
//   1. RandomVectorScorerSupplier.Scorer() + SetScoringOrdinal(idx0) +
//      Score(idx1)               -- ordinal-against-ordinal
//   2. GetRandomVectorScorer(target).Score(idx1)
//      where target is the raw vector at idx0       -- float target
//   3. GetRandomVectorScorerByte(target).Score(idx1)
//      where target is the raw vector at idx0       -- byte target
//
// All three paths must yield the same float32 score, for every
// similarity function (COSINE, EUCLIDEAN, DOT_PRODUCT,
// MAXIMUM_INNER_PRODUCT) and for both encodings (BYTE, FLOAT32).
//
// Mapping from Java @Test methods to Go subtests
// ----------------------------------------------
//   testSimpleScorer               -> TestVectorScorer_SimpleByte
//   testSimpleScorerSmallChunkSize -> n/a (no chunked mmap dispatch)
//   testSimpleScorerMedChunkSize   -> n/a (no chunked mmap dispatch)
//   testRandomScorer (and Min/Max) -> TestVectorScorer_RandomByte
//   testRandomSmallChunkSize       -> n/a (no chunked mmap dispatch)
//   testRandomSlice / SliceSmall   -> n/a (slicing is a store-layer
//                                     concern, exercised separately by
//                                     store/mmap_directory_test.go;
//                                     scorer parity does not depend on
//                                     it in Gocene)
//   testCopiesAcrossThreads        -> TestVectorScorer_CopiesAcrossGoroutines
//   testLarge (@Monster)           -> n/a (gigabytes of disk, off by
//                                     default in Lucene too)
//   testWithFloatValues            -> TestVectorScorer_Float
//
// All chunk-size / slice variants collapse because Gocene's scorer
// operates on []byte / []float32 slices directly, with no JDK
// Foreign-Memory indirection to validate. The store-layer mmap
// chunking is already covered by store/multi_mmap_test.go.

import (
	"fmt"
	"math"
	"math/rand/v2"
	"strings"
	"sync"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs/hnsw"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
	hnswutil "github.com/FlavioCFOliveira/Gocene/util/hnsw"
)

// vectorScorerDelta mirrors the Java test's DELTA constant (1e-5).
const vectorScorerDelta = 1e-5

// vectorScorerLoopCount mirrors Java's TIMES = 100. Each parity
// iteration revalidates the three API surfaces against a fresh
// random ordinal pair.
const vectorScorerLoopCount = 100

// vectorScorerSimilarities is the {COSINE, EUCLIDEAN, DOT_PRODUCT,
// MAXIMUM_INNER_PRODUCT} sweep that every Java @Test loops over.
var vectorScorerSimilarities = []index.VectorSimilarityFunction{
	index.VectorSimilarityFunctionCosine,
	index.VectorSimilarityFunctionEuclidean,
	index.VectorSimilarityFunctionDotProduct,
	index.VectorSimilarityFunctionMaximumInnerProduct,
}

// scorerUnderTest is the canonical Gocene equivalent of both Java
// DEFAULT_SCORER and MEMSEG_SCORER. Because they collapse to the
// same type in Go (see file-level comment), there is a single
// instance; the parity assertions still hold structurally because
// we exercise three independent API surfaces of that same scorer.
var scorerUnderTest hnsw.FlatVectorsScorer = hnsw.DefaultFlatVectorScorerInstance

// fixedByteVectorValues is an in-memory ByteVectorValues used by
// the byte-encoded scorer paths. It mirrors the off-heap byte view
// the Java test builds via OffHeapByteVectorValues.DenseOffHeapVectorValues.
// We use an on-heap fixture because the Go scorer operates on []byte
// slices regardless of provenance.
type fixedByteVectorValues struct {
	dim     int
	vectors [][]byte
}

func (v *fixedByteVectorValues) Dimension() int                      { return v.dim }
func (v *fixedByteVectorValues) Size() int                           { return len(v.vectors) }
func (v *fixedByteVectorValues) OrdToDoc(ord int) int                { return ord }
func (v *fixedByteVectorValues) GetEncoding() index.VectorEncoding   { return index.VectorEncodingByte }
func (v *fixedByteVectorValues) GetAcceptOrds(b util.Bits) util.Bits { return b }
func (v *fixedByteVectorValues) Iterator() hnswutil.DocIndexIterator { return nil }
func (v *fixedByteVectorValues) VectorValue(ord int) ([]byte, error) { return v.vectors[ord], nil }
func (v *fixedByteVectorValues) CopyByte() (hnsw.ByteVectorValues, error) {
	cp := make([][]byte, len(v.vectors))
	for i, src := range v.vectors {
		cp[i] = append([]byte(nil), src...)
	}
	return &fixedByteVectorValues{dim: v.dim, vectors: cp}, nil
}

// fixedFloatVectorValues is the float32 counterpart of
// fixedByteVectorValues, used by TestVectorScorer_Float.
type fixedFloatVectorValues struct {
	dim     int
	vectors [][]float32
}

func (v *fixedFloatVectorValues) Dimension() int       { return v.dim }
func (v *fixedFloatVectorValues) Size() int            { return len(v.vectors) }
func (v *fixedFloatVectorValues) OrdToDoc(ord int) int { return ord }
func (v *fixedFloatVectorValues) GetEncoding() index.VectorEncoding {
	return index.VectorEncodingFloat32
}
func (v *fixedFloatVectorValues) GetAcceptOrds(b util.Bits) util.Bits { return b }
func (v *fixedFloatVectorValues) Iterator() hnswutil.DocIndexIterator { return nil }
func (v *fixedFloatVectorValues) VectorValue(ord int) ([]float32, error) {
	return v.vectors[ord], nil
}

func (v *fixedFloatVectorValues) CopyFloat() (hnsw.FloatVectorValues, error) {
	cp := make([][]float32, len(v.vectors))
	for i, src := range v.vectors {
		cp[i] = append([]float32(nil), src...)
	}
	return &fixedFloatVectorValues{dim: v.dim, vectors: cp}, nil
}

// scoreViaSupplier exercises API surface (1): build a supplier,
// extract an UpdateableRandomVectorScorer, set the scoring ordinal,
// and score against the target ordinal. Mirrors the
// `getRandomVectorScorerSupplier(...).scorer()` block in the Java
// test.
func scoreViaSupplier(
	t *testing.T,
	values hnswutil.KnnVectorValues,
	sim index.VectorSimilarityFunction,
	scoringOrd, targetOrd int,
) float32 {
	t.Helper()
	supplier, err := scorerUnderTest.GetRandomVectorScorerSupplier(sim, values)
	if err != nil {
		t.Fatalf("GetRandomVectorScorerSupplier(%v): %v", sim, err)
	}
	scorer, err := supplier.Scorer()
	if err != nil {
		t.Fatalf("Supplier.Scorer(): %v", err)
	}
	if err := scorer.SetScoringOrdinal(scoringOrd); err != nil {
		t.Fatalf("SetScoringOrdinal(%d): %v", scoringOrd, err)
	}
	s, err := scorer.Score(targetOrd)
	if err != nil {
		t.Fatalf("Score(%d): %v", targetOrd, err)
	}
	return s
}

// scoreViaByteTarget exercises API surface (3): build a scorer bound
// to an explicit byte query. Mirrors
// `getRandomVectorScorer(sim, vv, byte[])` in the Java test.
func scoreViaByteTarget(
	t *testing.T,
	values hnswutil.KnnVectorValues,
	sim index.VectorSimilarityFunction,
	query []byte,
	targetOrd int,
) float32 {
	t.Helper()
	scorer, err := scorerUnderTest.GetRandomVectorScorerByte(sim, values, query)
	if err != nil {
		t.Fatalf("GetRandomVectorScorerByte(%v): %v", sim, err)
	}
	s, err := scorer.Score(targetOrd)
	if err != nil {
		t.Fatalf("Score(%d): %v", targetOrd, err)
	}
	return s
}

// scoreViaFloatTarget exercises API surface (2) for float32 fields:
// build a scorer bound to an explicit float32 query. Mirrors
// `getRandomVectorScorer(sim, vv, float[])` in the Java test.
func scoreViaFloatTarget(
	t *testing.T,
	values hnswutil.KnnVectorValues,
	sim index.VectorSimilarityFunction,
	query []float32,
	targetOrd int,
) float32 {
	t.Helper()
	scorer, err := scorerUnderTest.GetRandomVectorScorer(sim, values, query)
	if err != nil {
		t.Fatalf("GetRandomVectorScorer(%v): %v", sim, err)
	}
	s, err := scorer.Score(targetOrd)
	if err != nil {
		t.Fatalf("Score(%d): %v", targetOrd, err)
	}
	return s
}

// assertFloatNear is the Go equivalent of JUnit
// `assertEquals(expected, actual, DELTA)`.
func assertFloatNear(t *testing.T, label string, got, want float32) {
	t.Helper()
	d := float64(got - want)
	if math.Abs(d) > vectorScorerDelta {
		t.Fatalf("%s: got %g want %g (delta=%g)", label, got, want, d)
	}
}

// TestVectorScorer_SimpleByte is the Go port of
// TestVectorScorer#testSimpleScorer (and its chunk-size variants).
// Sweeps three dimensions {31, 32, 33} chosen by the Java test to
// straddle SIMD lane boundaries; in Gocene they retain interest as
// a small odd/even/odd sweep around the typical 32-byte stride
// boundary. The chunk-size variants collapse into the single sweep
// because no chunked-mmap dispatch lives in the Go scorer.
func TestVectorScorer_SimpleByte(t *testing.T) {
	t.Parallel()
	for _, dims := range []int{31, 32, 33} {
		dims := dims
		t.Run(fmt.Sprintf("dims=%d", dims), func(t *testing.T) {
			t.Parallel()
			vectors := [][]byte{make([]byte, dims), make([]byte, dims)}
			for i := 0; i < dims; i++ {
				vectors[0][i] = byte(i)
				vectors[1][i] = byte(dims - i)
			}
			vv := &fixedByteVectorValues{dim: dims, vectors: vectors}
			for _, sim := range vectorScorerSimilarities {
				sim := sim
				for _, ords := range [][2]int{{0, 1}, {1, 0}} {
					idx0, idx1 := ords[0], ords[1]
					expected := scoreViaSupplier(t, vv, sim, idx0, idx1)
					target := vectors[idx0]
					got := scoreViaByteTarget(t, vv, sim, target, idx1)
					assertFloatNear(t,
						fmt.Sprintf("sim=%v idx0=%d idx1=%d", sim, idx0, idx1),
						got, expected,
					)
				}
			}
		})
	}
}

// TestVectorScorer_RandomByte is the Go port of
// TestVectorScorer#testRandomScorer (and the Max / Min /
// SmallChunkSize variants). Each variant differs only in how it
// fills the byte arrays — random, all 127, all -128. We retain all
// three suppliers and iterate `vectorScorerLoopCount` random ord
// pairs per (supplier, similarity) combination.
func TestVectorScorer_RandomByte(t *testing.T) {
	t.Parallel()
	suppliers := map[string]func(int) []byte{
		"random": byteArrayRandom,
		"max":    byteArrayFill(127),
		// Java uses Byte.MIN_VALUE == -128. The Go `byte` type is
		// unsigned, so we encode the same bit pattern as 0x80.
		"min": byteArrayFill(0x80),
	}
	for name, supplier := range suppliers {
		name, supplier := name, supplier
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rng := newDeterministicRNG(name)
			dims := randIntBetween(rng, 1, 4096)
			size := randIntBetween(rng, 2, 100)
			vectors := make([][]byte, size)
			for i := range vectors {
				vectors[i] = supplier(dims)
			}
			vv := &fixedByteVectorValues{dim: dims, vectors: vectors}
			for times := 0; times < vectorScorerLoopCount; times++ {
				for _, sim := range vectorScorerSimilarities {
					idx0 := randIntBetween(rng, 0, size-1)
					idx1 := randIntBetween(rng, 0, size-1)
					expected := scoreViaSupplier(t, vv, sim, idx0, idx1)
					got := scoreViaByteTarget(t, vv, sim, vectors[idx0], idx1)
					assertFloatNear(t,
						fmt.Sprintf("sim=%v idx0=%d idx1=%d", sim, idx0, idx1),
						got, expected,
					)
				}
			}
		})
	}
}

// TestVectorScorer_CopiesAcrossGoroutines is the Go port of
// TestVectorScorer#testCopiesAcrossThreads. The Java test proves the
// MEMSEG supplier's per-thread copies do not stomp each other; the
// Go test asserts the same invariant on
// RandomVectorScorerSupplier.Copy(), which the supplier docstring
// explicitly designates as the goroutine-safety boundary.
//
// Run with -race to surface any latent shared-state hazard in the
// supplier graph.
func TestVectorScorer_CopiesAcrossGoroutines(t *testing.T) {
	t.Parallel()
	const dims = 34 // > the Java chunk size of 32; retained for parity
	vec1 := make([]byte, dims)
	vec2 := make([]byte, dims)
	for i := 0; i < dims; i++ {
		vec1[i] = 1
		vec2[i] = 2
	}
	vectors := [][]byte{vec1, vec1, vec2, vec2}
	vv := &fixedByteVectorValues{dim: dims, vectors: vectors}

	for _, sim := range vectorScorerSimilarities {
		sim := sim
		t.Run(sim.String(), func(t *testing.T) {
			t.Parallel()
			baseSupplier, err := scorerUnderTest.GetRandomVectorScorerSupplier(sim, vv)
			if err != nil {
				t.Fatalf("base supplier: %v", err)
			}
			baseScorer, err := baseSupplier.Scorer()
			if err != nil {
				t.Fatalf("base scorer: %v", err)
			}
			if err := baseScorer.SetScoringOrdinal(0); err != nil {
				t.Fatalf("base SetScoringOrdinal(0): %v", err)
			}
			expected1, err := baseScorer.Score(1)
			if err != nil {
				t.Fatalf("base Score(1): %v", err)
			}
			if err := baseScorer.SetScoringOrdinal(2); err != nil {
				t.Fatalf("base SetScoringOrdinal(2): %v", err)
			}
			expected2, err := baseScorer.Score(3)
			if err != nil {
				t.Fatalf("base Score(3): %v", err)
			}

			parallelSupplier, err := scorerUnderTest.GetRandomVectorScorerSupplier(sim, vv)
			if err != nil {
				t.Fatalf("parallel supplier: %v", err)
			}

			var wg sync.WaitGroup
			errCh := make(chan error, 2)
			for _, c := range []struct {
				target, ord int
				want        float32
			}{
				{0, 1, expected1},
				{2, 3, expected2},
			} {
				c := c
				copySupplier, err := parallelSupplier.Copy()
				if err != nil {
					t.Fatalf("Supplier.Copy(): %v", err)
				}
				wg.Add(1)
				go func() {
					defer wg.Done()
					scorer, err := copySupplier.Scorer()
					if err != nil {
						errCh <- fmt.Errorf("copy.Scorer(): %w", err)
						return
					}
					if err := scorer.SetScoringOrdinal(c.target); err != nil {
						errCh <- fmt.Errorf("SetScoringOrdinal(%d): %w", c.target, err)
						return
					}
					for i := 0; i < 100; i++ {
						got, err := scorer.Score(c.ord)
						if err != nil {
							errCh <- fmt.Errorf("Score(%d): %w", c.ord, err)
							return
						}
						if math.Abs(float64(got-c.want)) > vectorScorerDelta {
							errCh <- fmt.Errorf("iter %d: got %g want %g", i, got, c.want)
							return
						}
					}
				}()
			}
			wg.Wait()
			close(errCh)
			for err := range errCh {
				t.Errorf("goroutine error: %v", err)
			}
		})
	}
}

// TestVectorScorer_Float is the Go port of
// TestVectorScorer#testWithFloatValues. It exercises the FLOAT32
// path with a single 1-dimensional vector {1.0f}, verifies the
// supplier's toString contains "float" (Java parity), and confirms
// that supplying a byte target against a FLOAT32 view is rejected.
func TestVectorScorer_Float(t *testing.T) {
	t.Parallel()
	vectors := [][]float32{{1.0}}
	vv := &fixedFloatVectorValues{dim: 1, vectors: vectors}

	for times := 0; times < vectorScorerLoopCount; times++ {
		for _, sim := range vectorScorerSimilarities {
			supplier, err := scorerUnderTest.GetRandomVectorScorerSupplier(sim, vv)
			if err != nil {
				t.Fatalf("GetRandomVectorScorerSupplier(%v): %v", sim, err)
			}
			// supplier.String() is the only introspectable surface
			// here; the per-Scorer() inner scorer is unexported and
			// has no String() method (intentional — see
			// codecs/hnsw/default_flat_vector_scorer_impl.go). The
			// Java test asserts both supplier.toString() and
			// scorer.toString() carry the word "float"; in Gocene we
			// preserve only the supplier-level assertion.
			if s, ok := supplier.(fmt.Stringer); ok {
				if !strings.Contains(strings.ToLower(s.String()), "float") {
					t.Fatalf("supplier.String() = %q, want substring %q", s.String(), "float")
				}
			} else {
				t.Fatalf("supplier %T does not implement fmt.Stringer", supplier)
			}

			scorer, err := supplier.Scorer()
			if err != nil {
				t.Fatalf("Supplier.Scorer(): %v", err)
			}
			if err := scorer.SetScoringOrdinal(0); err != nil {
				t.Fatalf("SetScoringOrdinal(0): %v", err)
			}
			expectedFromSupplier, err := scorer.Score(0)
			if err != nil {
				t.Fatalf("supplier.Score(0): %v", err)
			}

			expectedFromFloatTarget := scoreViaFloatTarget(t, vv, sim, []float32{1.0}, 0)
			assertFloatNear(t,
				fmt.Sprintf("float-target parity sim=%v", sim),
				expectedFromFloatTarget, expectedFromSupplier,
			)

			// Byte target against a FLOAT32 view must be rejected.
			// The Java reference relies on expectThrows; Gocene
			// returns an error from the constructor.
			if _, err := scorerUnderTest.GetRandomVectorScorerByte(
				sim, vv, []byte{1},
			); err == nil {
				t.Fatalf("GetRandomVectorScorerByte against FLOAT32 view: expected error, got nil")
			}
		}
	}
}

// TestVectorScorer_FloatTargetAgainstByteValuesRejected covers the
// converse type-mismatch: a float target against a BYTE-encoded view
// must be rejected. The Java test exercises this via the same
// expectThrows block (line 396-399 in the reference), interleaved in
// testWithFloatValues; we hoist it into a dedicated test for
// readability.
func TestVectorScorer_FloatTargetAgainstByteValuesRejected(t *testing.T) {
	t.Parallel()
	vv := &fixedByteVectorValues{dim: 1, vectors: [][]byte{{1}}}
	if _, err := scorerUnderTest.GetRandomVectorScorer(
		index.VectorSimilarityFunctionEuclidean, vv, []float32{1.0},
	); err == nil {
		t.Fatalf("GetRandomVectorScorer against BYTE view: expected error, got nil")
	}
}

// TestVectorScorer_DimensionMismatchRejected mirrors the Java
// FlatVectorsScorer#checkDimensions contract: query length must
// equal field length for both target overloads.
func TestVectorScorer_DimensionMismatchRejected(t *testing.T) {
	t.Parallel()
	byteView := &fixedByteVectorValues{dim: 4, vectors: [][]byte{{1, 2, 3, 4}}}
	if _, err := scorerUnderTest.GetRandomVectorScorerByte(
		index.VectorSimilarityFunctionEuclidean, byteView, []byte{1, 2, 3},
	); err == nil {
		t.Fatal("byte target with mismatched dimension: expected error, got nil")
	}
	floatView := &fixedFloatVectorValues{dim: 4, vectors: [][]float32{{1, 2, 3, 4}}}
	if _, err := scorerUnderTest.GetRandomVectorScorer(
		index.VectorSimilarityFunctionEuclidean, floatView, []float32{1, 2, 3},
	); err == nil {
		t.Fatal("float target with mismatched dimension: expected error, got nil")
	}
}

// newDeterministicRNG returns a math/rand/v2 source seeded by a
// stable hash of name. Determinism matters because the Java test
// uses LuceneTestCase's seeded randomness; Gocene mirrors that by
// keying the seed off the subtest label so failures are
// reproducible from the test name alone.
func newDeterministicRNG(name string) *rand.Rand {
	var seed [32]byte
	for i, b := range []byte(name) {
		seed[i%32] ^= b
	}
	// Mix in a fixed nonce so adjacent labels produce well-separated
	// streams; the value itself is arbitrary (an ASCII signature of
	// the porting task).
	for i, b := range []byte("GOC-4278") {
		seed[(16+i)%32] ^= b
	}
	return rand.New(rand.NewChaCha8(seed))
}

// randIntBetween mirrors RandomNumbers.randomIntBetween(minIncl,
// maxIncl). It returns a uniform integer in [min, max] inclusive on
// both ends.
func randIntBetween(r *rand.Rand, minIncl, maxIncl int) int {
	if maxIncl < minIncl {
		panic(fmt.Sprintf("randIntBetween: max %d < min %d", maxIncl, minIncl))
	}
	span := maxIncl - minIncl + 1
	return minIncl + r.IntN(span)
}

// byteArrayRandom is the Go port of TestVectorScorer's
// BYTE_ARRAY_RANDOM_FUNC. We seed it lazily per call so the produced
// array distribution matches Java's RandomXoroshiro128PlusPlus only
// in expectation, not bit-for-bit. The parity check that matters is
// arithmetic equality across the three API surfaces, not raw bit
// equality with the JVM.
func byteArrayRandom(size int) []byte {
	out := make([]byte, size)
	r := rand.New(rand.NewChaCha8([32]byte{
		'G', 'O', 'C', '-', '4', '2', '7', '8',
		byte(size), byte(size >> 8), byte(size >> 16), byte(size >> 24),
	}))
	for i := range out {
		out[i] = byte(r.Uint32())
	}
	return out
}

// byteArrayFill returns a generator that emits arrays filled with
// value. Mirrors BYTE_ARRAY_MAX_FUNC / BYTE_ARRAY_MIN_FUNC.
func byteArrayFill(value byte) func(int) []byte {
	return func(size int) []byte {
		out := make([]byte, size)
		for i := range out {
			out[i] = value
		}
		return out
	}
}
