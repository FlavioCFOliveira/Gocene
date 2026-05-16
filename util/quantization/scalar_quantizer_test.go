// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package quantization

import (
	"errors"
	"math"
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// testSimpleFloatVectorValues is the Go counterpart of Java's
// TestScalarQuantizer.TestSimpleFloatVectorValues. It walks a fixed
// 2-D float array, optionally skipping a set of "deleted" rows, and
// yields (docId, ordinal) pairs through Iterator().
type testSimpleFloatVectorValues struct {
	floats         [][]float32
	deletedVectors map[int]struct{}
	ordToDoc       []int
	numLiveVectors int
}

func newTestSimpleFloatVectorValues(floats [][]float32, deleted map[int]struct{}) *testSimpleFloatVectorValues {
	live := len(floats) - len(deleted)
	ordToDoc := make([]int, live)
	if deleted == nil {
		for i := 0; i < live; i++ {
			ordToDoc[i] = i
		}
	} else {
		ord := 0
		for doc := 0; doc < len(floats); doc++ {
			if _, dead := deleted[doc]; !dead {
				ordToDoc[ord] = doc
				ord++
			}
		}
	}
	return &testSimpleFloatVectorValues{
		floats:         floats,
		deletedVectors: deleted,
		ordToDoc:       ordToDoc,
		numLiveVectors: live,
	}
}

func (t *testSimpleFloatVectorValues) Dimension() int { return len(t.floats[0]) }

func (t *testSimpleFloatVectorValues) VectorValue(ord int) ([]float32, error) {
	if ord < 0 || ord >= t.numLiveVectors {
		return nil, errors.New("ordinal out of range")
	}
	return t.floats[t.ordToDoc[ord]], nil
}

func (t *testSimpleFloatVectorValues) Iterator() DocIndexIterator {
	return &testSimpleIterator{values: t, ord: -1, doc: -1}
}

type testSimpleIterator struct {
	values *testSimpleFloatVectorValues
	ord    int
	doc    int
}

func (it *testSimpleIterator) NextDoc() (int, error) {
	for it.doc < len(it.values.floats)-1 {
		it.doc++
		if it.values.deletedVectors == nil {
			it.ord++
			return it.doc, nil
		}
		if _, dead := it.values.deletedVectors[it.doc]; !dead {
			it.ord++
			return it.doc, nil
		}
	}
	it.doc = util.NO_MORE_DOCS
	return util.NO_MORE_DOCS, nil
}

func (it *testSimpleIterator) Index() int { return it.ord }

// Deterministic per-test PRNG. We pin the seed to keep results stable
// across runs without depending on Lucene's java.util.Random.
func newTestRand() *rand.Rand { return rand.New(rand.NewSource(1)) }

func randomFloatArray(r *rand.Rand, dims int) []float32 {
	arr := make([]float32, dims)
	for j := 0; j < dims; j++ {
		// Java: random().nextFloat(-1, 1)
		arr[j] = r.Float32()*2 - 1
	}
	return arr
}

func randomFloats(r *rand.Rand, num, dims int) [][]float32 {
	out := make([][]float32, num)
	for i := 0; i < num; i++ {
		out[i] = randomFloatArray(r, dims)
	}
	return out
}

func shuffleFloatArray(r *rand.Rand, arr []float32) {
	for i := len(arr) - 1; i > 0; i-- {
		j := r.Intn(i + 1)
		arr[i], arr[j] = arr[j], arr[i]
	}
}

func fromFloats(floats [][]float32) FloatVectorValues {
	return newTestSimpleFloatVectorValues(floats, nil)
}

func fromFloatsWithRandomDeletions(r *rand.Rand, floats [][]float32, numDeleted int) *testSimpleFloatVectorValues {
	deleted := make(map[int]struct{}, numDeleted)
	for i := 0; i < numDeleted; i++ {
		deleted[r.Intn(len(floats))] = struct{}{}
	}
	return newTestSimpleFloatVectorValues(floats, deleted)
}

// TestTinyVectors mirrors Java's testTinyVectors: small synthetic
// corpora exercise FromVectors and FromVectorsAutoInterval across the
// supported similarity functions and bit-widths. We assert only that
// construction succeeds — the Java reference comment notes that
// quantisation quality is poor at this sample size.
func TestTinyVectors(t *testing.T) {
	r := newTestRand()
	for _, function := range []index.VectorSimilarityFunction{
		index.VectorSimilarityFunctionEuclidean,
		index.VectorSimilarityFunctionDotProduct,
		index.VectorSimilarityFunctionCosine,
		index.VectorSimilarityFunctionMaximumInnerProduct,
	} {
		dims := r.Intn(9) + 1
		numVecs := r.Intn(9) + 10
		floats := randomFloats(r, numVecs, dims)
		if function == index.VectorSimilarityFunctionCosine {
			for _, v := range floats {
				util.L2Normalize(v)
			}
		}
		for _, bits := range []byte{4, 7} {
			values := fromFloats(floats)
			actualFunction := function
			if actualFunction == index.VectorSimilarityFunctionCosine {
				actualFunction = index.VectorSimilarityFunctionDotProduct
			}
			useFromVectors := r.Intn(2) == 0
			var (
				sq  *ScalarQuantizer
				err error
			)
			if useFromVectors {
				sq, err = FromVectors(values, 0.9, numVecs, bits)
			} else {
				sq, err = FromVectorsAutoInterval(values, actualFunction, numVecs, bits)
			}
			if err != nil {
				t.Fatalf("function=%v bits=%d: unexpected err: %v", function, bits, err)
			}
			if sq == nil {
				t.Fatalf("function=%v bits=%d: got nil quantiser", function, bits)
			}
		}
	}
}

// TestNanAndInfValueFailure mirrors Java's testNanAndInfValueFailure:
// feeding a vector full of NaN/Inf must make both static factories
// return a non-nil error.
func TestNanAndInfValueFailure(t *testing.T) {
	r := newTestRand()
	for _, function := range []index.VectorSimilarityFunction{
		index.VectorSimilarityFunctionEuclidean,
		index.VectorSimilarityFunctionDotProduct,
		index.VectorSimilarityFunctionCosine,
		index.VectorSimilarityFunctionMaximumInnerProduct,
	} {
		dims := r.Intn(9) + 1
		numVecs := r.Intn(9) + 10
		floats := make([][]float32, numVecs)
		for i := 0; i < numVecs; i++ {
			floats[i] = make([]float32, dims)
			for j := 0; j < dims; j++ {
				if r.Intn(2) == 0 {
					floats[i][j] = float32(math.NaN())
				} else {
					floats[i][j] = float32(math.Inf(+1))
				}
			}
		}
		for _, bits := range []byte{4, 7} {
			values := fromFloats(floats)
			if _, err := FromVectors(values, 0.9, numVecs, bits); err == nil {
				t.Errorf("FromVectors(function=%v bits=%d): expected error, got nil", function, bits)
			}
			actualFunction := function
			if actualFunction == index.VectorSimilarityFunctionCosine {
				actualFunction = index.VectorSimilarityFunctionDotProduct
			}
			values2 := fromFloats(floats)
			if _, err := FromVectorsAutoInterval(values2, actualFunction, numVecs, bits); err == nil {
				t.Errorf("FromVectorsAutoInterval(function=%v bits=%d): expected error, got nil", function, bits)
			}
		}
	}
}

// TestQuantizeAndDeQuantize7Bit mirrors Java's
// testQuantizeAndDeQuantize7Bit: 128-dim float vectors are quantised
// to 7-bit, deQuantised, and re-quantised; the round-trip must stay
// within tolerance and the byte values must lie in [0, 127].
func TestQuantizeAndDeQuantize7Bit(t *testing.T) {
	r := newTestRand()
	const dims = 128
	const numVecs = 100
	simFunc := index.VectorSimilarityFunctionDotProduct
	floats := randomFloats(r, numVecs, dims)
	values := fromFloats(floats)
	sq, err := FromVectors(values, 1, numVecs, 7)
	if err != nil {
		t.Fatalf("FromVectors: %v", err)
	}
	dequantized := make([]float32, dims)
	quantized := make([]byte, dims)
	requantized := make([]byte, dims)
	var maxDimValue, minDimValue byte = 0, 127
	for i := 0; i < numVecs; i++ {
		sq.Quantize(floats[i], quantized, simFunc)
		sq.DeQuantize(quantized, dequantized)
		sq.Quantize(dequantized, requantized, simFunc)
		for j := 0; j < dims; j++ {
			if quantized[j] > maxDimValue {
				maxDimValue = quantized[j]
			}
			if quantized[j] < minDimValue {
				minDimValue = quantized[j]
			}
			if diff := math.Abs(float64(dequantized[j] - floats[i][j])); diff > 0.02 {
				t.Errorf("dequantized mismatch i=%d j=%d: |%v - %v| = %v > 0.02",
					i, j, dequantized[j], floats[i][j], diff)
			}
			if quantized[j] != requantized[j] {
				t.Errorf("requantize mismatch i=%d j=%d: %d != %d",
					i, j, quantized[j], requantized[j])
			}
		}
	}
	// 7-bit quantization must stay in [0, 127].
	if maxDimValue > 127 {
		t.Errorf("maxDimValue=%d exceeds 127", maxDimValue)
	}
}

// TestQuantizeAndDeQuantize8Bit mirrors Java's
// testQuantizeAndDeQuantize8Bit: same shape as the 7-bit test but
// without the 0..127 byte-range guard.
func TestQuantizeAndDeQuantize8Bit(t *testing.T) {
	r := newTestRand()
	const dims = 128
	const numVecs = 100
	simFunc := index.VectorSimilarityFunctionDotProduct
	floats := randomFloats(r, numVecs, dims)
	values := fromFloats(floats)
	sq, err := FromVectors(values, 1, numVecs, 8)
	if err != nil {
		t.Fatalf("FromVectors: %v", err)
	}
	dequantized := make([]float32, dims)
	quantized := make([]byte, dims)
	requantized := make([]byte, dims)
	for i := 0; i < numVecs; i++ {
		sq.Quantize(floats[i], quantized, simFunc)
		sq.DeQuantize(quantized, dequantized)
		sq.Quantize(dequantized, requantized, simFunc)
		for j := 0; j < dims; j++ {
			if diff := math.Abs(float64(dequantized[j] - floats[i][j])); diff > 0.02 {
				t.Errorf("dequantized mismatch i=%d j=%d: |%v - %v| = %v > 0.02",
					i, j, dequantized[j], floats[i][j], diff)
			}
			if quantized[j] != requantized[j] {
				t.Errorf("requantize mismatch i=%d j=%d: %d != %d",
					i, j, quantized[j], requantized[j])
			}
		}
	}
}

// TestQuantiles mirrors Java's testQuantiles: a synthetic [0, 1000)
// array yields predictable percentile bounds for the three standard
// confidence intervals (0.9, 0.95, 0.99).
func TestQuantiles(t *testing.T) {
	r := newTestRand()
	percs := make([]float32, 1000)
	for i := 0; i < 1000; i++ {
		percs[i] = float32(i)
	}
	shuffleFloatArray(r, percs)
	low, high := getUpperAndLowerQuantile(percs, 0.9)
	if math.Abs(float64(low-50)) > 1e-5 || math.Abs(float64(high-949)) > 1e-5 {
		t.Errorf("0.9 quantile: got (%v, %v), want (50, 949)", low, high)
	}
	shuffleFloatArray(r, percs)
	low, high = getUpperAndLowerQuantile(percs, 0.95)
	if math.Abs(float64(low-25)) > 1e-5 || math.Abs(float64(high-974)) > 1e-5 {
		t.Errorf("0.95 quantile: got (%v, %v), want (25, 974)", low, high)
	}
	shuffleFloatArray(r, percs)
	low, high = getUpperAndLowerQuantile(percs, 0.99)
	if math.Abs(float64(low-5)) > 1e-5 || math.Abs(float64(high-994)) > 1e-5 {
		t.Errorf("0.99 quantile: got (%v, %v), want (5, 994)", low, high)
	}
}

// TestEdgeCase mirrors Java's testEdgeCase: a five-element constant
// array must yield equal lower and upper bounds.
func TestEdgeCase(t *testing.T) {
	arr := []float32{1, 1, 1, 1, 1}
	low, high := getUpperAndLowerQuantile(arr, 0.9)
	if low != 1 || high != 1 {
		t.Errorf("constant-array quantile: got (%v, %v), want (1, 1)", low, high)
	}
}

// TestScalarWithSampling mirrors Java's testScalarWithSampling:
// FromVectors must not error when invoked with random-deletion-backed
// values and a non-default sample budget. The Java reference asserts
// only that the call does not throw; we follow suit.
func TestScalarWithSampling(t *testing.T) {
	r := newTestRand()
	numVecs := r.Intn(128) + 5
	const dims = 64
	floats := randomFloats(r, numVecs, dims)
	cases := []func(){
		func() {
			values := fromFloatsWithRandomDeletions(r, floats, r.Intn(numVecs-1)+1)
			sample := values.numLiveVectors - 1
			if sample < scratchSize+1 {
				sample = scratchSize + 1
			}
			if _, err := fromVectorsWithSampleSize(values, 0.99, values.numLiveVectors, 7, sample); err != nil {
				t.Fatalf("case#1: %v", err)
			}
		},
		func() {
			values := fromFloatsWithRandomDeletions(r, floats, r.Intn(numVecs-1)+1)
			sample := values.numLiveVectors - 1
			if sample < scratchSize+1 {
				sample = scratchSize + 1
			}
			if _, err := fromVectorsWithSampleSize(values, 0.99, values.numLiveVectors, 7, sample); err != nil {
				t.Fatalf("case#2: %v", err)
			}
		},
		func() {
			values := fromFloatsWithRandomDeletions(r, floats, r.Intn(numVecs-1)+1)
			sample := values.numLiveVectors - 1
			if sample < scratchSize+1 {
				sample = scratchSize + 1
			}
			if _, err := fromVectorsWithSampleSize(values, 0.99, values.numLiveVectors, 7, sample); err != nil {
				t.Fatalf("case#3: %v", err)
			}
		},
		func() {
			values := fromFloatsWithRandomDeletions(r, floats, r.Intn(numVecs-1)+1)
			sample := r.Intn(len(values.floats)-1) + 1
			if sample < scratchSize+1 {
				sample = scratchSize + 1
			}
			if _, err := fromVectorsWithSampleSize(values, 0.99, values.numLiveVectors, 7, sample); err != nil {
				t.Fatalf("case#4: %v", err)
			}
		},
	}
	for _, c := range cases {
		c()
	}
}

// TestFromVectorsAutoInterval4Bit mirrors Java's
// testFromVectorsAutoInterval4Bit: unit-normalised 128-dim vectors
// quantised to 4 bits should round-trip within a coarse tolerance and
// stay inside [0, 15].
func TestFromVectorsAutoInterval4Bit(t *testing.T) {
	r := newTestRand()
	const dims = 128
	const numVecs = 100
	simFunc := index.VectorSimilarityFunctionDotProduct
	floats := randomFloats(r, numVecs, dims)
	for _, v := range floats {
		util.L2Normalize(v)
	}
	values := fromFloats(floats)
	sq, err := FromVectorsAutoInterval(values, simFunc, numVecs, 4)
	if err != nil {
		t.Fatalf("FromVectorsAutoInterval: %v", err)
	}
	if sq == nil {
		t.Fatal("FromVectorsAutoInterval: nil quantiser")
	}
	dequantized := make([]float32, dims)
	quantized := make([]byte, dims)
	requantized := make([]byte, dims)
	var maxDimValue, minDimValue byte = 0, 127
	for i := 0; i < numVecs; i++ {
		sq.Quantize(floats[i], quantized, simFunc)
		sq.DeQuantize(quantized, dequantized)
		sq.Quantize(dequantized, requantized, simFunc)
		for j := 0; j < dims; j++ {
			if quantized[j] > maxDimValue {
				maxDimValue = quantized[j]
			}
			if quantized[j] < minDimValue {
				minDimValue = quantized[j]
			}
			if diff := math.Abs(float64(dequantized[j] - floats[i][j])); diff > 0.2 {
				t.Errorf("dequantized mismatch i=%d j=%d: |%v - %v| = %v > 0.2",
					i, j, dequantized[j], floats[i][j], diff)
			}
			if quantized[j] != requantized[j] {
				t.Errorf("requantize mismatch i=%d j=%d: %d != %d",
					i, j, quantized[j], requantized[j])
			}
		}
	}
	// 4-bit quantization must stay in [0, 15].
	if maxDimValue > 15 {
		t.Errorf("maxDimValue=%d exceeds 15", maxDimValue)
	}
	_ = minDimValue
}

// TestConstructorRejectsBadBits exercises the Go-specific defensive
// branches of NewScalarQuantizer that Java enforces with assertions.
func TestConstructorRejectsBadBits(t *testing.T) {
	for _, bits := range []byte{0, 9, 10, 255} {
		if _, err := NewScalarQuantizer(0, 1, bits); err == nil {
			t.Errorf("bits=%d: expected error", bits)
		}
	}
	if _, err := NewScalarQuantizer(1, 0, 7); err == nil {
		t.Errorf("max < min: expected error")
	}
	if _, err := NewScalarQuantizer(float32(math.NaN()), 1, 7); err == nil {
		t.Errorf("NaN min: expected error")
	}
	if _, err := NewScalarQuantizer(0, float32(math.Inf(1)), 7); err == nil {
		t.Errorf("Inf max: expected error")
	}
}

// TestConstructorAccessors verifies the getter surface of a
// constructed quantiser matches its inputs.
func TestConstructorAccessors(t *testing.T) {
	sq, err := NewScalarQuantizer(-0.5, 0.5, 7)
	if err != nil {
		t.Fatalf("NewScalarQuantizer: %v", err)
	}
	if got := sq.GetLowerQuantile(); got != -0.5 {
		t.Errorf("GetLowerQuantile: got %v, want -0.5", got)
	}
	if got := sq.GetUpperQuantile(); got != 0.5 {
		t.Errorf("GetUpperQuantile: got %v, want 0.5", got)
	}
	if got := sq.GetBits(); got != 7 {
		t.Errorf("GetBits: got %d, want 7", got)
	}
	// alpha = 1/127; getConstantMultiplier = alpha^2.
	wantAlpha := float32(1.0 / 127.0)
	if got := sq.GetConstantMultiplier(); math.Abs(float64(got-wantAlpha*wantAlpha)) > 1e-12 {
		t.Errorf("GetConstantMultiplier: got %v, want %v", got, wantAlpha*wantAlpha)
	}
}

// TestStringFormat anchors the canonical toString output so future
// refactors cannot silently change it (downstream code may log
// quantisers for debugging).
func TestStringFormat(t *testing.T) {
	sq, err := NewScalarQuantizer(-1, 1, 7)
	if err != nil {
		t.Fatalf("NewScalarQuantizer: %v", err)
	}
	want := "ScalarQuantizer{minQuantile=-1, maxQuantile=1, bits=7}"
	if got := sq.String(); got != want {
		t.Errorf("String: got %q, want %q", got, want)
	}
}

// TestRecalculateCorrectiveOffset checks the round-trip of
// RecalculateCorrectiveOffset against a manual replay through
// DeQuantize + Quantize. Java does not have a dedicated test for this
// method; we keep coverage explicit because the helper has no other
// caller in this package yet.
func TestRecalculateCorrectiveOffset(t *testing.T) {
	r := newTestRand()
	const dims = 16
	floats := randomFloats(r, 4, dims)
	values := fromFloats(floats)
	oldQ, err := FromVectors(values, 1, 4, 7)
	if err != nil {
		t.Fatalf("FromVectors (old): %v", err)
	}
	newQ, err := FromVectors(fromFloats(floats), 0.9, 4, 7)
	if err != nil {
		t.Fatalf("FromVectors (new): %v", err)
	}
	dequant := make([]float32, dims)
	quant := make([]byte, dims)
	for _, fn := range []index.VectorSimilarityFunction{
		index.VectorSimilarityFunctionEuclidean,
		index.VectorSimilarityFunctionDotProduct,
		index.VectorSimilarityFunctionMaximumInnerProduct,
	} {
		oldQ.Quantize(floats[0], quant, fn)
		// Expected correction: dequantise under oldQ, quantise under
		// newQ, return the Quantize correction.
		oldQ.DeQuantize(quant, dequant)
		want := newQ.Quantize(dequant, make([]byte, dims), fn)
		got := newQ.RecalculateCorrectiveOffset(quant, oldQ, fn)
		if math.Abs(float64(got-want)) > 1e-3 {
			t.Errorf("function=%v: RecalculateCorrectiveOffset=%v, want %v", fn, got, want)
		}
	}
}

// TestQuantizeLengthMismatchPanics ensures the assertion-style guards
// surface as panics, matching the Java assertion semantics (under -ea
// the JVM throws; in Go we panic deterministically).
func TestQuantizeLengthMismatchPanics(t *testing.T) {
	sq, err := NewScalarQuantizer(-1, 1, 7)
	if err != nil {
		t.Fatalf("NewScalarQuantizer: %v", err)
	}
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Quantize: expected panic on length mismatch")
		}
	}()
	sq.Quantize([]float32{1, 2}, make([]byte, 3), index.VectorSimilarityFunctionDotProduct)
}

// TestDeQuantizeLengthMismatchPanics is the dequantise counterpart.
func TestDeQuantizeLengthMismatchPanics(t *testing.T) {
	sq, err := NewScalarQuantizer(-1, 1, 7)
	if err != nil {
		t.Fatalf("NewScalarQuantizer: %v", err)
	}
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("DeQuantize: expected panic on length mismatch")
		}
	}()
	sq.DeQuantize([]byte{1, 2}, make([]float32, 3))
}

// TestFromVectorsAutoIntervalRejectsCosine verifies the explicit
// Cosine guard documented on FromVectorsAutoInterval. The Java
// reference uses a plain assert; in Go we return an error.
func TestFromVectorsAutoIntervalRejectsCosine(t *testing.T) {
	if _, err := FromVectorsAutoInterval(fromFloats([][]float32{{1, 0}}),
		index.VectorSimilarityFunctionCosine, 1, 7); err == nil {
		t.Errorf("expected error for COSINE")
	}
}

// TestFromVectorsConfidenceIntervalGuard covers the [0.9, 1.0] range
// check that Java enforces with an assertion.
func TestFromVectorsConfidenceIntervalGuard(t *testing.T) {
	for _, ci := range []float32{-1, 0, 0.5, 0.89, 1.01, float32(math.NaN())} {
		if _, err := FromVectors(fromFloats([][]float32{{1, 0}}), ci, 1, 7); err == nil {
			t.Errorf("ci=%v: expected error", ci)
		}
	}
}

// TestFromVectorsEmptyCorpus covers the totalVectorCount == 0 branch
// that returns a placeholder quantiser.
func TestFromVectorsEmptyCorpus(t *testing.T) {
	sq, err := FromVectors(fromFloats([][]float32{{1, 0}}), 1, 0, 7)
	if err != nil {
		t.Fatalf("FromVectors: %v", err)
	}
	if sq.GetLowerQuantile() != 0 || sq.GetUpperQuantile() != 0 {
		t.Errorf("empty corpus: got (%v, %v), want (0, 0)", sq.GetLowerQuantile(), sq.GetUpperQuantile())
	}
}
