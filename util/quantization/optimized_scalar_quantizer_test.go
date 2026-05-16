// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package quantization

import (
	"math"
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// allBits mirrors Java's TestOptimizedScalarQuantizer.ALL_BITS.
var allBits = []byte{1, 2, 3, 4, 5, 6, 7, 8}

// newOSQRand returns a per-test seeded RNG. The exact draws need not
// match Java's java.util.Random; the assertions in the Java reference
// hold for any uniform [0, 1) source.
func newOSQRand() *rand.Rand { return rand.New(rand.NewSource(42)) }

// assertValidQuantizedRange mirrors the Java helper of the same name:
// quantised bytes must be in [0, 2^bits). For bits == 8 we follow the
// Java reference, which only asserts the upper bound (since Java
// bytes are signed and the assertion `b >= 0` would otherwise reject
// the high half of an 8-bit unsigned domain).
func assertValidQuantizedRange(t *testing.T, quantized []byte, bits byte) {
	t.Helper()
	upper := 1 << bits
	for i, b := range quantized {
		if bits < 8 {
			// Go bytes are unsigned: the Java check `b >= 0` is
			// trivially true. We keep the check to match the Java
			// peer's intent: an 8-bit-or-less assignment must fit.
			if int(b) >= upper {
				t.Fatalf("bits=%d idx=%d: quantised value %d >= 1<<%d", bits, i, b, bits)
			}
		} else {
			if int(b) >= upper {
				t.Fatalf("bits=%d idx=%d: quantised value %d >= 1<<%d", bits, i, b, bits)
			}
		}
	}
}

// assertValidResults mirrors the Java helper: every result must have
// finite intervals, lower <= upper, finite correction, and a
// non-negative quantised component sum.
func assertValidResults(t *testing.T, results ...QuantizationResult) {
	t.Helper()
	for i, r := range results {
		if !isFiniteF32(r.LowerInterval) {
			t.Fatalf("result[%d]: lower interval not finite: %v", i, r.LowerInterval)
		}
		if !isFiniteF32(r.UpperInterval) {
			t.Fatalf("result[%d]: upper interval not finite: %v", i, r.UpperInterval)
		}
		if r.LowerInterval > r.UpperInterval {
			t.Fatalf("result[%d]: lower (%v) > upper (%v)", i, r.LowerInterval, r.UpperInterval)
		}
		if !isFiniteF32(r.AdditionalCorrection) {
			t.Fatalf("result[%d]: additional correction not finite: %v", i, r.AdditionalCorrection)
		}
		if r.QuantizedComponentSum < 0 {
			t.Fatalf("result[%d]: quantised component sum negative: %d", i, r.QuantizedComponentSum)
		}
	}
}

func isFiniteF32(v float32) bool {
	f := float64(v)
	return !math.IsNaN(f) && !math.IsInf(f, 0)
}

// TestOptimizedScalarQuantizer_QuantizationQuality mirrors Java's
// TestOptimizedScalarQuantizer.testQuantizationQuality: for a small
// corpus of dims=16 vectors and every supported bit-width, the
// dequantised vector must stay within 1/(1<<bits) MAE of the source.
func TestOptimizedScalarQuantizer_QuantizationQuality(t *testing.T) {
	const (
		dims       = 16
		numVectors = 32
	)
	r := newOSQRand()
	vectors := make([][]float32, numVectors)
	centroid := make([]float32, dims)
	for i := 0; i < numVectors; i++ {
		vectors[i] = make([]float32, dims)
		for j := 0; j < dims; j++ {
			vectors[i][j] = r.Float32()
			centroid[j] += vectors[i][j]
		}
	}
	for j := 0; j < dims; j++ {
		centroid[j] /= float32(numVectors)
	}

	// Similarity is irrelevant for this test, per the Java comment.
	osq := NewOptimizedScalarQuantizer(index.VectorSimilarityFunctionDotProduct)
	scratch := make([]float32, dims)
	for _, bit := range allBits {
		eps := 1.0 / float32(uint32(1)<<bit)
		destination := make([]byte, dims)
		for i := 0; i < numVectors; i++ {
			copy(scratch, vectors[i])
			result, err := osq.ScalarQuantize(scratch, destination, bit, centroid)
			if err != nil {
				t.Fatalf("bits=%d idx=%d: ScalarQuantize: %v", bit, i, err)
			}
			assertValidResults(t, result)
			assertValidQuantizedRange(t, destination, bit)

			dequantized := make([]float32, dims)
			if _, err := DeQuantize(
				destination,
				dequantized,
				bit,
				result.LowerInterval,
				result.UpperInterval,
				centroid,
			); err != nil {
				t.Fatalf("bits=%d idx=%d: DeQuantize: %v", bit, i, err)
			}
			var mae float32
			for k := 0; k < dims; k++ {
				mae += absF32(dequantized[k] - vectors[i][k])
			}
			mae /= float32(dims)
			if mae > eps {
				t.Fatalf("bits=%d idx=%d: mae=%v > eps=%v", bit, i, mae, eps)
			}
		}
	}
}

// TestOptimizedScalarQuantizer_AbusiveEdgeCases mirrors Java's
// testAbusiveEdgeCases: large zero arrays (skipping Cosine because
// the zero vector violates the unit-length precondition) and
// single-element vectors under every similarity function.
func TestOptimizedScalarQuantizer_AbusiveEdgeCases(t *testing.T) {
	r := newOSQRand()
	similarities := []index.VectorSimilarityFunction{
		index.VectorSimilarityFunctionEuclidean,
		index.VectorSimilarityFunctionDotProduct,
		index.VectorSimilarityFunctionCosine,
		index.VectorSimilarityFunctionMaximumInnerProduct,
	}

	for _, sim := range similarities {
		if sim == index.VectorSimilarityFunctionCosine {
			continue
		}
		vector := make([]float32, 4096)
		centroid := make([]float32, 4096)
		osq := NewOptimizedScalarQuantizer(sim)
		destinations := make([][]byte, len(MinimumMSEGrid))
		for i := range destinations {
			destinations[i] = make([]byte, 4096)
		}
		results, err := osq.MultiScalarQuantize(vector, destinations, allBits, centroid)
		if err != nil {
			t.Fatalf("sim=%v: MultiScalarQuantize: %v", sim, err)
		}
		if len(results) != len(MinimumMSEGrid) {
			t.Fatalf("sim=%v: got %d results, want %d", sim, len(results), len(MinimumMSEGrid))
		}
		assertValidResults(t, results...)
		// Each destination must remain all-zero because the input
		// was the zero vector and the (centroid-subtracted) vector
		// is also zero.
		zero := make([]byte, 4096)
		for i, dest := range destinations {
			for j := range dest {
				if dest[j] != zero[j] {
					t.Fatalf("sim=%v bits=%d idx=%d: expected zero quantisation, got %d",
						sim, allBits[i], j, dest[j])
				}
			}
		}
		destination := make([]byte, 4096)
		for _, bit := range allBits {
			// Refresh the zero vector before each call because the
			// previous call's in-place subtraction left zeros, but a
			// fresh slice keeps the test self-documenting.
			for k := range vector {
				vector[k] = 0
			}
			result, err := osq.ScalarQuantize(vector, destination, bit, centroid)
			if err != nil {
				t.Fatalf("sim=%v bits=%d: ScalarQuantize: %v", sim, bit, err)
			}
			assertValidResults(t, result)
			for j := range destination {
				if destination[j] != 0 {
					t.Fatalf("sim=%v bits=%d idx=%d: expected zero quantisation, got %d",
						sim, bit, j, destination[j])
				}
			}
		}
	}

	// Single-value vectors across every similarity function.
	for _, sim := range similarities {
		vector := []float32{r.Float32()}
		centroid := []float32{r.Float32()}
		if sim == index.VectorSimilarityFunctionCosine {
			util.L2Normalize(vector)
			util.L2Normalize(centroid)
		}
		osq := NewOptimizedScalarQuantizer(sim)
		destinations := make([][]byte, len(MinimumMSEGrid))
		for i := range destinations {
			destinations[i] = make([]byte, 1)
		}
		results, err := osq.MultiScalarQuantize(vector, destinations, allBits, centroid)
		if err != nil {
			t.Fatalf("sim=%v: MultiScalarQuantize (single-value): %v", sim, err)
		}
		if len(results) != len(MinimumMSEGrid) {
			t.Fatalf("sim=%v: got %d results, want %d", sim, len(results), len(MinimumMSEGrid))
		}
		assertValidResults(t, results...)
		for i, dest := range destinations {
			assertValidQuantizedRange(t, dest, allBits[i])
		}
		for _, bit := range allBits {
			vec := []float32{r.Float32()}
			cen := []float32{r.Float32()}
			if sim == index.VectorSimilarityFunctionCosine {
				util.L2Normalize(vec)
				util.L2Normalize(cen)
			}
			destination := make([]byte, 1)
			result, err := osq.ScalarQuantize(vec, destination, bit, cen)
			if err != nil {
				t.Fatalf("sim=%v bits=%d: ScalarQuantize (single-value): %v", sim, bit, err)
			}
			assertValidResults(t, result)
			assertValidQuantizedRange(t, destination, bit)
		}
	}
}

// TestOptimizedScalarQuantizer_MathematicalConsistency mirrors Java's
// testMathematicalConsistency: every similarity function produces a
// valid result with valid quantised ranges, both via
// MultiScalarQuantize and the single-bit ScalarQuantize entry point.
func TestOptimizedScalarQuantizer_MathematicalConsistency(t *testing.T) {
	r := newOSQRand()
	dims := r.Intn(4096) + 1
	vector := make([]float32, dims)
	for i := range vector {
		vector[i] = r.Float32()
	}
	centroid := make([]float32, dims)
	for i := range centroid {
		centroid[i] = r.Float32()
	}
	copyVec := make([]float32, dims)
	cosineCentroid := make([]float32, dims)
	copy(cosineCentroid, centroid)

	similarities := []index.VectorSimilarityFunction{
		index.VectorSimilarityFunctionEuclidean,
		index.VectorSimilarityFunctionDotProduct,
		index.VectorSimilarityFunctionCosine,
		index.VectorSimilarityFunctionMaximumInnerProduct,
	}
	for _, sim := range similarities {
		copy(copyVec, vector)
		cen := centroid
		if sim == index.VectorSimilarityFunctionCosine {
			util.L2Normalize(copyVec)
			copy(cosineCentroid, centroid)
			util.L2Normalize(cosineCentroid)
			cen = cosineCentroid
		}
		osq := NewOptimizedScalarQuantizer(sim)
		destinations := make([][]byte, len(MinimumMSEGrid))
		for i := range destinations {
			destinations[i] = make([]byte, dims)
		}
		results, err := osq.MultiScalarQuantize(copyVec, destinations, allBits, cen)
		if err != nil {
			t.Fatalf("sim=%v: MultiScalarQuantize: %v", sim, err)
		}
		if len(results) != len(MinimumMSEGrid) {
			t.Fatalf("sim=%v: got %d results, want %d", sim, len(results), len(MinimumMSEGrid))
		}
		assertValidResults(t, results...)
		for i, dest := range destinations {
			assertValidQuantizedRange(t, dest, allBits[i])
		}
		for _, bit := range allBits {
			destination := make([]byte, dims)
			copy(copyVec, vector)
			cen := centroid
			if sim == index.VectorSimilarityFunctionCosine {
				util.L2Normalize(copyVec)
				copy(cosineCentroid, centroid)
				util.L2Normalize(cosineCentroid)
				cen = cosineCentroid
			}
			result, err := osq.ScalarQuantize(copyVec, destination, bit, cen)
			if err != nil {
				t.Fatalf("sim=%v bits=%d: ScalarQuantize: %v", sim, bit, err)
			}
			assertValidResults(t, result)
			assertValidQuantizedRange(t, destination, bit)
		}
	}
}

// TestOptimizedScalarQuantizer_UnpackBinary mirrors Java's
// testUnpackBinary: PackAsBinary followed by UnpackBinary round-trips
// every 0/1 byte. The Java helper uses ScalarEncoding to size the
// scratch buffer; we replicate the same formula inline to avoid a
// codecs import cycle (single-bit query nibble packs 8 dimensions
// into 1 byte, so the packed length is ceil(dims / 8)).
func TestOptimizedScalarQuantizer_UnpackBinary(t *testing.T) {
	r := newOSQRand()
	dim := r.Intn(4096) + 1
	scratch := make([]byte, dim)
	for i := range scratch {
		if r.Intn(2) == 1 {
			scratch[i] = 1
		}
	}
	packed := make([]byte, (len(scratch)+7)/8)
	unpacked := make([]byte, len(scratch))
	if err := PackAsBinary(scratch, packed); err != nil {
		t.Fatalf("PackAsBinary: %v", err)
	}
	UnpackBinary(packed, unpacked)
	for i := range scratch {
		if scratch[i] != unpacked[i] {
			t.Fatalf("idx=%d: scratch=%d unpacked=%d", i, scratch[i], unpacked[i])
		}
	}
}

// TestOptimizedScalarQuantizer_PackTransposeDibit mirrors Java's
// testPackTransposeDibit: TransposeDibit followed by UntransposeDibit
// round-trips every 0..3 dibit. The Java helper uses
// ScalarEncoding.DIBIT_QUERY_NIBBLE for sizing; the packed length is
// 2 * ceil(dims / 8) (two stripes of one bit-plane each).
func TestOptimizedScalarQuantizer_PackTransposeDibit(t *testing.T) {
	r := newOSQRand()
	dim := r.Intn(4096) + 1
	scratch := make([]byte, dim)
	for i := range scratch {
		scratch[i] = byte(r.Intn(4))
	}
	stripe := (len(scratch) + 7) / 8
	packed := make([]byte, 2*stripe)
	unpacked := make([]byte, len(scratch))
	if err := TransposeDibit(scratch, packed); err != nil {
		t.Fatalf("TransposeDibit: %v", err)
	}
	UntransposeDibit(packed, unpacked)
	for i := range scratch {
		if scratch[i] != unpacked[i] {
			t.Fatalf("idx=%d: scratch=%d unpacked=%d", i, scratch[i], unpacked[i])
		}
	}
}

// TestOptimizedScalarQuantizer_TransposeHalfByte verifies the
// half-byte transpose primitive. The Java test peer does not exercise
// it directly (it is consumed inside the lucene104 codec), so we add
// a Go-level round-trip across all 4 bit-planes to keep the helper
// covered.
func TestOptimizedScalarQuantizer_TransposeHalfByte(t *testing.T) {
	r := newOSQRand()
	dim := r.Intn(512) + 1
	input := make([]byte, dim)
	for i := range input {
		input[i] = byte(r.Intn(16))
	}
	stripe := (len(input) + 7) / 8
	packed := make([]byte, 4*stripe)
	if err := TransposeHalfByte(input, packed); err != nil {
		t.Fatalf("TransposeHalfByte: %v", err)
	}
	// Reverse: untranspose by walking the four bit-planes back into
	// the per-dimension nibbles.
	got := make([]byte, len(input))
	for i := 0; i < len(input); i++ {
		bitIdx := uint(7 - (i % 8))
		byteIdx := i / 8
		lo := (packed[byteIdx] >> bitIdx) & 1
		lm := (packed[byteIdx+stripe] >> bitIdx) & 1
		um := (packed[byteIdx+2*stripe] >> bitIdx) & 1
		hi := (packed[byteIdx+3*stripe] >> bitIdx) & 1
		got[i] = lo | (lm << 1) | (um << 2) | (hi << 3)
	}
	for i := range input {
		if input[i] != got[i] {
			t.Fatalf("idx=%d: input=%d got=%d", i, input[i], got[i])
		}
	}
}

// TestOptimizedScalarQuantizer_Discretize checks the Java static
// helper of the same name across the documented contract.
func TestOptimizedScalarQuantizer_Discretize(t *testing.T) {
	cases := []struct {
		value, bucket, want int
	}{
		{0, 8, 0},
		{1, 8, 8},
		{7, 8, 8},
		{8, 8, 8},
		{9, 8, 16},
		{15, 8, 16},
		{16, 8, 16},
		{17, 8, 24},
		// Bucket=4 covers half-byte packing.
		{0, 4, 0},
		{1, 4, 4},
		{4, 4, 4},
		{5, 4, 8},
	}
	for _, c := range cases {
		if got := Discretize(c.value, c.bucket); got != c.want {
			t.Fatalf("Discretize(%d,%d) = %d, want %d", c.value, c.bucket, got, c.want)
		}
	}
}

// TestOptimizedScalarQuantizer_DeQuantizeErrors covers the input
// validation surface that the Java reference handles via assertions.
func TestOptimizedScalarQuantizer_DeQuantizeErrors(t *testing.T) {
	cases := []struct {
		name        string
		quantized   []byte
		dequantized []float32
		centroid    []float32
		bits        byte
	}{
		{"nil quantized", nil, make([]float32, 1), make([]float32, 1), 4},
		{"nil dequantized", []byte{0}, nil, make([]float32, 1), 4},
		{"nil centroid", []byte{0}, make([]float32, 1), nil, 4},
		{"dequantized too small", []byte{0, 1}, make([]float32, 1), make([]float32, 2), 4},
		{"centroid too small", []byte{0, 1}, make([]float32, 2), make([]float32, 1), 4},
		{"bits=0", []byte{0}, make([]float32, 1), make([]float32, 1), 0},
		{"bits=9", []byte{0}, make([]float32, 1), make([]float32, 1), 9},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := DeQuantize(c.quantized, c.dequantized, c.bits, 0, 1, c.centroid); err == nil {
				t.Fatalf("expected error, got nil")
			}
		})
	}
}

// TestOptimizedScalarQuantizer_ScalarQuantizeErrors covers the
// validation surface for ScalarQuantize.
func TestOptimizedScalarQuantizer_ScalarQuantizeErrors(t *testing.T) {
	osq := NewOptimizedScalarQuantizer(index.VectorSimilarityFunctionDotProduct)
	cases := []struct {
		name     string
		vec      []float32
		dest     []byte
		bits     byte
		centroid []float32
	}{
		{"nil vector", nil, make([]byte, 1), 4, make([]float32, 1)},
		{"nil centroid", make([]float32, 1), make([]byte, 1), 4, nil},
		{"length mismatch", make([]float32, 2), make([]byte, 2), 4, make([]float32, 3)},
		{"destination too small", make([]float32, 4), make([]byte, 2), 4, make([]float32, 4)},
		{"bits=0", make([]float32, 1), make([]byte, 1), 0, make([]float32, 1)},
		{"bits=9", make([]float32, 1), make([]byte, 1), 9, make([]float32, 1)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := osq.ScalarQuantize(c.vec, c.dest, c.bits, c.centroid); err == nil {
				t.Fatalf("expected error, got nil")
			}
		})
	}
}

// TestOptimizedScalarQuantizer_MultiScalarQuantizeErrors covers the
// validation surface for MultiScalarQuantize.
func TestOptimizedScalarQuantizer_MultiScalarQuantizeErrors(t *testing.T) {
	osq := NewOptimizedScalarQuantizer(index.VectorSimilarityFunctionEuclidean)
	bits := []byte{4}
	dest := [][]byte{make([]byte, 4)}
	vec := make([]float32, 4)
	cen := make([]float32, 4)

	if _, err := osq.MultiScalarQuantize(nil, dest, bits, cen); err == nil {
		t.Fatalf("expected error for nil vector")
	}
	if _, err := osq.MultiScalarQuantize(vec, dest, bits, nil); err == nil {
		t.Fatalf("expected error for nil centroid")
	}
	if _, err := osq.MultiScalarQuantize(vec, dest, []byte{4, 5}, cen); err == nil {
		t.Fatalf("expected error for bits/destinations length mismatch")
	}
	if _, err := osq.MultiScalarQuantize(vec, [][]byte{make([]byte, 2)}, bits, cen); err == nil {
		t.Fatalf("expected error for destination too small")
	}
	if _, err := osq.MultiScalarQuantize(vec, dest, []byte{9}, cen); err == nil {
		t.Fatalf("expected error for bits out of range")
	}
	if _, err := osq.MultiScalarQuantize(make([]float32, 3), dest, bits, cen); err == nil {
		t.Fatalf("expected error for vector/centroid length mismatch")
	}
}

// TestOptimizedScalarQuantizer_CosineRejectsNonUnit ensures that
// Cosine similarity rejects non-unit inputs without panicking.
func TestOptimizedScalarQuantizer_CosineRejectsNonUnit(t *testing.T) {
	osq := NewOptimizedScalarQuantizer(index.VectorSimilarityFunctionCosine)
	vector := []float32{2, 0, 0}
	centroid := []float32{1, 0, 0}
	util.L2Normalize(centroid)
	dest := make([]byte, len(vector))
	if _, err := osq.ScalarQuantize(vector, dest, 4, centroid); err == nil {
		t.Fatalf("expected error for non-unit vector under COSINE")
	}
	util.L2Normalize(vector)
	nonUnitCentroid := []float32{2, 0, 0}
	if _, err := osq.ScalarQuantize(vector, dest, 4, nonUnitCentroid); err == nil {
		t.Fatalf("expected error for non-unit centroid under COSINE")
	}
}

// TestOptimizedScalarQuantizer_PackBinaryRejectsOutOfRange ensures
// that PackAsBinary rejects bytes outside {0, 1}.
func TestOptimizedScalarQuantizer_PackBinaryRejectsOutOfRange(t *testing.T) {
	vec := []byte{0, 1, 2}
	packed := make([]byte, 1)
	if err := PackAsBinary(vec, packed); err == nil {
		t.Fatalf("expected error for value=2")
	}
}

// TestOptimizedScalarQuantizer_TransposeHalfByteRejectsOutOfRange
// ensures TransposeHalfByte rejects values that overflow a nibble.
func TestOptimizedScalarQuantizer_TransposeHalfByteRejectsOutOfRange(t *testing.T) {
	in := []byte{0, 1, 16}
	out := make([]byte, 4)
	if err := TransposeHalfByte(in, out); err == nil {
		t.Fatalf("expected error for value=16")
	}
}
