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

package quantization

import (
	"fmt"
	"math"
	"math/rand"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ScalarQuantizationSampleSize is the maximum number of float vectors
// inspected when computing quantiles. Larger corpora are sampled via
// reservoir sampling. Mirrors Lucene's SCALAR_QUANTIZATION_SAMPLE_SIZE.
const ScalarQuantizationSampleSize = 25_000

// scratchSize is the number of vectors that the quantile-gathering
// scratch buffer can hold before [getUpperAndLowerQuantile] is invoked.
// Mirrors Lucene's package-private SCRATCH_SIZE. Kept lowercase to
// match the Java visibility; the value is also referenced by the test
// peer in this package.
const scratchSize = 20

// scalarQuantizerRandomSeed matches the fixed seed used by the Java
// reference's static Random(42), so reservoir-sampling decisions stay
// deterministic across runs. The byte-for-byte sampling order does not
// match Lucene (Java Random and math/rand produce different sequences),
// but determinism within Gocene is preserved.
const scalarQuantizerRandomSeed int64 = 42

// scalarQuantizerRandom is the package-level deterministic RNG used by
// reservoir sampling, mirroring Java's `private static final Random random
// = new Random(42)`. The Java reference threads a shared, fixed-seed
// generator across calls; we do the same so successive calls observe
// the evolving state instead of restarting from seed 42 every time.
//
// math/rand.Rand is not safe for concurrent use; neither is the Java
// Random under Lucene's usage (ScalarQuantizer.fromVectors is not
// invoked concurrently in the reference). If concurrent quantiser
// construction becomes a requirement, a per-call rand.Rand seeded from
// a counter is the cheapest fix.
var scalarQuantizerRandom = rand.New(rand.NewSource(scalarQuantizerRandomSeed))

// ScalarQuantizer scalar-quantizes float vectors into int8-range byte
// values. Port of org.apache.lucene.util.quantization.ScalarQuantizer
// (Lucene 10.4.0).
//
// Quantization maps the range [minQuantile, maxQuantile] onto
// [0, 2^bits - 1] so that
//
//	byte  = (float - minQuantile) * (2^bits - 1) / (maxQuantile - minQuantile)
//	float = byte * (maxQuantile - minQuantile) / (2^bits - 1) + minQuantile
//
// alpha and scale cache the two scale factors used by the hot path.
//
// ScalarQuantizer is immutable; the zero value is not usable. Construct
// instances via [NewScalarQuantizer] or the static factories
// [FromVectors] / [FromVectorsAutoInterval].
type ScalarQuantizer struct {
	alpha       float32
	scale       float32
	bits        byte
	minQuantile float32
	maxQuantile float32
}

// NewScalarQuantizer constructs a quantiser for the supplied lower and
// upper quantile bounds and a target bit-width in (0, 8]. Returns an
// error if any quantile is NaN or infinite (matching the Java
// IllegalStateException) or if bits is outside the valid range
// (matching the Java assertion).
//
// The Java reference uses assertions for the (maxQuantile >= minQuantile)
// and (0 < bits <= 8) invariants; we surface them as explicit errors so
// production builds do not silently produce an invalid quantiser.
func NewScalarQuantizer(minQuantile, maxQuantile float32, bits byte) (*ScalarQuantizer, error) {
	if isNaNOrInfFloat32(minQuantile) || isNaNOrInfFloat32(maxQuantile) {
		return nil, fmt.Errorf("quantization: scalar quantizer does not support infinite or NaN values")
	}
	if maxQuantile < minQuantile {
		return nil, fmt.Errorf("quantization: maxQuantile (%v) must be >= minQuantile (%v)",
			maxQuantile, minQuantile)
	}
	if bits == 0 || bits > 8 {
		return nil, fmt.Errorf("quantization: bits must satisfy 0 < bits <= 8, got %d", bits)
	}
	divisor := float32((uint32(1) << bits) - 1)
	q := &ScalarQuantizer{
		bits:        bits,
		minQuantile: minQuantile,
		maxQuantile: maxQuantile,
	}
	// Avoid division by zero when min == max: the resulting quantiser
	// maps every float onto byte 0 (alpha stays at 0). The Java code
	// produces +Inf/NaN here under -ea; we keep the more defensive
	// behaviour because the bypass arises legitimately for empty
	// corpora (fromVectors returns ScalarQuantizer(0, 0, bits)).
	if span := maxQuantile - minQuantile; span != 0 {
		q.scale = divisor / span
		q.alpha = span / divisor
	}
	return q, nil
}

// Quantize quantises src into dest using the configured bounds and
// returns the corrective offset that callers add when reconstructing
// the dequantised inner product. For Euclidean similarity the offset
// is always zero; for the other functions the offset cancels the
// per-vector contribution of minQuantile when scoring against another
// quantised vector.
//
// dest must have the same length as src; the function panics on
// mismatch, mirroring Lucene's assertion. When similarityFunction is
// Cosine the source must be unit-length, matching Lucene's
// VectorUtil.isUnitVector(src) assertion; the check is enforced in
// builds where util.IsUnitVector returns false to keep production code
// safe even when assertions are disabled.
func (q *ScalarQuantizer) Quantize(src []float32, dest []byte, similarityFunction index.VectorSimilarityFunction) float32 {
	if len(src) != len(dest) {
		panic(fmt.Sprintf("quantization: src/dest length mismatch: %d!=%d", len(src), len(dest)))
	}
	if similarityFunction == index.VectorSimilarityFunctionCosine && !util.IsUnitVector(src) {
		panic("quantization: Quantize with COSINE requires a unit-length source vector")
	}
	correction := minMaxScalarQuantize(src, dest, q.scale, q.alpha, q.minQuantile, q.maxQuantile)
	if similarityFunction == index.VectorSimilarityFunctionEuclidean {
		return 0
	}
	return correction
}

// RecalculateCorrectiveOffset replays the score correction for the
// supplied quantisedVector using a new (this) quantiser, given the
// quantiser that produced quantisedVector originally. Mirrors Java's
// recalculateCorrectiveOffset; returns 0 for Euclidean similarity.
func (q *ScalarQuantizer) RecalculateCorrectiveOffset(
	quantizedVector []byte,
	oldQuantizer *ScalarQuantizer,
	similarityFunction index.VectorSimilarityFunction,
) float32 {
	if similarityFunction == index.VectorSimilarityFunctionEuclidean {
		return 0
	}
	return recalculateOffset(
		quantizedVector,
		oldQuantizer.alpha,
		oldQuantizer.minQuantile,
		q.scale,
		q.alpha,
		q.minQuantile,
		q.maxQuantile,
	)
}

// DeQuantize reconstructs a float vector from the supplied byte
// vector. dest must have the same length as src; the function panics
// on mismatch, mirroring Lucene's assertion. The byte values are
// interpreted as unsigned (Byte.toUnsignedInt in Java).
func (q *ScalarQuantizer) DeQuantize(src []byte, dest []float32) {
	if len(src) != len(dest) {
		panic(fmt.Sprintf("quantization: src/dest length mismatch: %d!=%d", len(src), len(dest)))
	}
	for i := range src {
		dest[i] = q.alpha*float32(src[i]) + q.minQuantile
	}
}

// GetLowerQuantile returns the lower quantile bound used at
// construction time (Java: getLowerQuantile).
func (q *ScalarQuantizer) GetLowerQuantile() float32 { return q.minQuantile }

// GetUpperQuantile returns the upper quantile bound used at
// construction time (Java: getUpperQuantile).
func (q *ScalarQuantizer) GetUpperQuantile() float32 { return q.maxQuantile }

// GetConstantMultiplier returns alpha squared, the multiplier shared
// by all quantised inner products derived from this quantiser (Java:
// getConstantMultiplier).
func (q *ScalarQuantizer) GetConstantMultiplier() float32 { return q.alpha * q.alpha }

// GetBits returns the configured bit-width (Java: getBits).
func (q *ScalarQuantizer) GetBits() byte { return q.bits }

// String returns the canonical text representation matching Java's
// ScalarQuantizer.toString.
func (q *ScalarQuantizer) String() string {
	return fmt.Sprintf("ScalarQuantizer{minQuantile=%v, maxQuantile=%v, bits=%d}",
		q.minQuantile, q.maxQuantile, q.bits)
}

// FromVectors derives a ScalarQuantizer by sampling the supplied
// FloatVectorValues. When totalVectorCount fits inside the default
// sample budget (ScalarQuantizationSampleSize) every live vector is
// inspected; otherwise reservoir sampling reduces the workload. Mirrors
// Java's ScalarQuantizer.fromVectors(values, confidenceInterval,
// totalVectorCount, bits).
func FromVectors(
	floatVectorValues FloatVectorValues,
	confidenceInterval float32,
	totalVectorCount int,
	bits byte,
) (*ScalarQuantizer, error) {
	return fromVectorsWithSampleSize(
		floatVectorValues,
		confidenceInterval,
		totalVectorCount,
		bits,
		ScalarQuantizationSampleSize,
	)
}

// fromVectorsWithSampleSize is the package-internal variant of
// FromVectors that exposes the sample-budget parameter. Used by the
// test peer to exercise the reservoir-sampling branch without
// allocating tens of thousands of synthetic vectors.
func fromVectorsWithSampleSize(
	floatVectorValues FloatVectorValues,
	confidenceInterval float32,
	totalVectorCount int,
	bits byte,
	quantizationSampleSize int,
) (*ScalarQuantizer, error) {
	if !(confidenceInterval >= 0.9 && confidenceInterval <= 1.0) {
		return nil, fmt.Errorf("quantization: confidenceInterval must lie in [0.9, 1.0], got %v", confidenceInterval)
	}
	if quantizationSampleSize <= scratchSize {
		return nil, fmt.Errorf("quantization: quantizationSampleSize must exceed scratchSize (%d), got %d",
			scratchSize, quantizationSampleSize)
	}
	if totalVectorCount == 0 {
		return NewScalarQuantizer(0, 0, bits)
	}
	iterator := floatVectorValues.Iterator()
	// confidenceInterval == 1 takes the exact min/max path; we use a
	// tight float32 comparison so the bitwise-identical value coming
	// from the Java caller still trips this fast path.
	if confidenceInterval == 1.0 {
		min := float32(math.Inf(+1))
		max := float32(math.Inf(-1))
		for {
			doc, err := iterator.NextDoc()
			if err != nil {
				return nil, fmt.Errorf("quantization: NextDoc: %w", err)
			}
			if doc == util.NO_MORE_DOCS {
				break
			}
			vec, err := floatVectorValues.VectorValue(iterator.Index())
			if err != nil {
				return nil, fmt.Errorf("quantization: VectorValue: %w", err)
			}
			for _, v := range vec {
				if v < min {
					min = v
				}
				if v > max {
					max = v
				}
			}
		}
		return NewScalarQuantizer(min, max, bits)
	}

	scratchLen := scratchSize
	if totalVectorCount < scratchLen {
		scratchLen = totalVectorCount
	}
	scratch := make([]float32, floatVectorValues.Dimension()*scratchLen)
	count := 0
	var upperSum, lowerSum [1]float64
	confidenceIntervals := [1]float32{confidenceInterval}

	if totalVectorCount <= quantizationSampleSize {
		// Dense path: walk every live vector, fill the scratch in
		// scratchLen-sized batches and update the running sums.
		i := 0
		for {
			doc, err := iterator.NextDoc()
			if err != nil {
				return nil, fmt.Errorf("quantization: NextDoc: %w", err)
			}
			if doc == util.NO_MORE_DOCS {
				break
			}
			vec, err := floatVectorValues.VectorValue(iterator.Index())
			if err != nil {
				return nil, fmt.Errorf("quantization: VectorValue: %w", err)
			}
			copy(scratch[i*len(vec):(i+1)*len(vec)], vec)
			i++
			if i == scratchLen {
				if err := extractQuantiles(confidenceIntervals[:], scratch, upperSum[:], lowerSum[:]); err != nil {
					return nil, err
				}
				i = 0
				count++
			}
		}
		// Java intentionally drops the trailing scratch tail; we follow
		// suit so very small corpora keep their extreme-confidence
		// protection.
		if count == 0 {
			return NewScalarQuantizer(0, 0, bits)
		}
		return NewScalarQuantizer(float32(lowerSum[0])/float32(count), float32(upperSum[0])/float32(count), bits)
	}

	// Reservoir-sampled path: pick quantizationSampleSize ordinals at
	// random and walk to them in order.
	vectorsToTake := reservoirSampleIndices(totalVectorCount, quantizationSampleSize)
	index := 0
	idx := 0
	for _, target := range vectorsToTake {
		for index <= target {
			// MergedVectorValues does not support advance(docId), so we
			// linearly drive the iterator forward, exactly like Java.
			if _, err := iterator.NextDoc(); err != nil {
				return nil, fmt.Errorf("quantization: NextDoc: %w", err)
			}
			index++
		}
		vec, err := floatVectorValues.VectorValue(iterator.Index())
		if err != nil {
			return nil, fmt.Errorf("quantization: VectorValue: %w", err)
		}
		copy(scratch[idx*len(vec):(idx+1)*len(vec)], vec)
		idx++
		if idx == scratchSize {
			if err := extractQuantiles(confidenceIntervals[:], scratch, upperSum[:], lowerSum[:]); err != nil {
				return nil, err
			}
			count++
			idx = 0
		}
	}
	if count == 0 {
		return NewScalarQuantizer(0, 0, bits)
	}
	return NewScalarQuantizer(float32(lowerSum[0])/float32(count), float32(upperSum[0])/float32(count), bits)
}

// FromVectorsAutoInterval derives a ScalarQuantizer by performing a
// grid search over candidate confidence intervals and picking the pair
// that maximises score-error correlation. Cosine similarity is
// rejected (Java assertion); the caller is expected to convert Cosine
// to DOT_PRODUCT plus unit normalisation upfront. Mirrors Java's
// ScalarQuantizer.fromVectorsAutoInterval.
func FromVectorsAutoInterval(
	floatVectorValues FloatVectorValues,
	function index.VectorSimilarityFunction,
	totalVectorCount int,
	bits byte,
) (*ScalarQuantizer, error) {
	if function == index.VectorSimilarityFunctionCosine {
		return nil, fmt.Errorf("quantization: FromVectorsAutoInterval does not support COSINE; normalise to DOT_PRODUCT upstream")
	}
	if totalVectorCount == 0 {
		return NewScalarQuantizer(0, 0, bits)
	}

	sampleSize := totalVectorCount
	if sampleSize > 1000 {
		sampleSize = 1000
	}
	scratchLen := scratchSize
	if totalVectorCount < scratchLen {
		scratchLen = totalVectorCount
	}
	scratch := make([]float32, floatVectorValues.Dimension()*scratchLen)
	count := 0
	var upperSum, lowerSum [2]float64
	sampledDocs := make([][]float32, 0, sampleSize)
	dim := floatVectorValues.Dimension()
	// Confidence-interval pair matches Java exactly.
	dimFactor := float32(dim) / 10
	if dimFactor > 32 {
		dimFactor = 32
	}
	confidenceIntervals := [2]float32{
		1 - dimFactor/float32(dim+1),
		1 - 1/float32(dim+1),
	}
	iterator := floatVectorValues.Iterator()

	if totalVectorCount <= sampleSize {
		i := 0
		for {
			doc, err := iterator.NextDoc()
			if err != nil {
				return nil, fmt.Errorf("quantization: NextDoc: %w", err)
			}
			if doc == util.NO_MORE_DOCS {
				break
			}
			vec, err := floatVectorValues.VectorValue(iterator.Index())
			if err != nil {
				return nil, fmt.Errorf("quantization: VectorValue: %w", err)
			}
			gatherSample(vec, scratch, &sampledDocs, i)
			i++
			if i == scratchLen {
				if err := extractQuantiles(confidenceIntervals[:], scratch, upperSum[:], lowerSum[:]); err != nil {
					return nil, err
				}
				i = 0
				count++
			}
		}
	} else {
		vectorsToTake := reservoirSampleIndices(totalVectorCount, 1000)
		index := 0
		idx := 0
		for _, target := range vectorsToTake {
			for index <= target {
				if _, err := iterator.NextDoc(); err != nil {
					return nil, fmt.Errorf("quantization: NextDoc: %w", err)
				}
				index++
			}
			vec, err := floatVectorValues.VectorValue(iterator.Index())
			if err != nil {
				return nil, fmt.Errorf("quantization: VectorValue: %w", err)
			}
			gatherSample(vec, scratch, &sampledDocs, idx)
			idx++
			if idx == scratchSize {
				if err := extractQuantiles(confidenceIntervals[:], scratch, upperSum[:], lowerSum[:]); err != nil {
					return nil, err
				}
				count++
				idx = 0
			}
		}
	}

	if count == 0 {
		return NewScalarQuantizer(0, 0, bits)
	}

	// Java naming preserved verbatim: al/au/bl/bu carry the four corner
	// candidates that bracket the grid search.
	cf := float32(count)
	al := float32(lowerSum[1]) / cf
	bu := float32(upperSum[1]) / cf
	au := float32(lowerSum[0]) / cf
	bl := float32(upperSum[0]) / cf
	if isNaNOrInfFloat32(al) || isNaNOrInfFloat32(au) || isNaNOrInfFloat32(bl) || isNaNOrInfFloat32(bu) {
		return nil, fmt.Errorf("quantization: quantile calculation resulted in NaN or infinite values")
	}
	var lowerCandidates [16]float32
	var upperCandidates [16]float32
	idx := 0
	for i := float32(0); i < 32; i += 2 {
		lowerCandidates[idx] = al + i*(au-al)/32
		upperCandidates[idx] = bl + i*(bu-bl)/32
		idx++
	}
	neighbours := findNearestNeighbours(sampledDocs, function)
	bestLower, bestUpper := candidateGridSearch(neighbours, sampledDocs, lowerCandidates[:], upperCandidates[:], function, bits)
	return NewScalarQuantizer(bestLower, bestUpper, bits)
}

// reservoirSampleIndices implements the same reservoir-sampling
// scheme as the Java reference: bootstrap the result with the first
// sampleSize ordinals, then replace each subsequent ordinal with
// probability sampleSize/(i+1).
//
// The deterministic seed (42) is preserved; the resulting indices may
// differ from Java because java.util.Random and math/rand do not share
// the same algorithm. Determinism inside Gocene is preserved across
// runs, which is all the test peer asserts.
func reservoirSampleIndices(numFloatVecs, sampleSize int) []int {
	vectorsToTake := make([]int, sampleSize)
	for i := 0; i < sampleSize; i++ {
		vectorsToTake[i] = i
	}
	for i := sampleSize; i < numFloatVecs; i++ {
		j := scalarQuantizerRandom.Intn(i + 1)
		if j < sampleSize {
			vectorsToTake[j] = i
		}
	}
	sort.Ints(vectorsToTake)
	return vectorsToTake
}

// extractQuantiles updates upperSum/lowerSum with the per-interval
// quantile contributions of the scratch buffer. Mirrors Java's
// private extractQuantiles helper.
func extractQuantiles(confidenceIntervals []float32, scratch []float32, upperSum, lowerSum []float64) error {
	if len(confidenceIntervals) != len(upperSum) || len(confidenceIntervals) != len(lowerSum) {
		return fmt.Errorf("quantization: extractQuantiles length mismatch")
	}
	for i, ci := range confidenceIntervals {
		lower, upper := getUpperAndLowerQuantile(scratch, ci)
		upperSum[i] += float64(upper)
		lowerSum[i] += float64(lower)
	}
	return nil
}

// gatherSample mirrors Java's gatherSample: copy the source vector
// into a fresh slice for downstream nearest-neighbour scoring and also
// stash it into the rolling scratch buffer.
func gatherSample(vec []float32, scratch []float32, sampled *[][]float32, i int) {
	cp := make([]float32, len(vec))
	copy(cp, vec)
	*sampled = append(*sampled, cp)
	copy(scratch[i*len(vec):(i+1)*len(vec)], vec)
}

// getUpperAndLowerQuantile returns the lower and upper quantile values
// of arr at the supplied confidenceInterval. Mirrors Java's
// ScalarQuantizer.getUpperAndLowerQuantile; the returned tuple matches
// Java's `new float[]{min, max}` ordering.
//
// arr is reordered in place by the underlying IntroSelector. Callers
// that retain arr after the call should clone it upfront.
func getUpperAndLowerQuantile(arr []float32, confidenceInterval float32) (float32, float32) {
	if len(arr) == 0 {
		panic("quantization: getUpperAndLowerQuantile: empty input")
	}
	if len(arr) <= 2 {
		// Trivial: sort and return the extremes.
		sortFloat32Slice(arr)
		return arr[0], arr[len(arr)-1]
	}
	// Java casts (int)(N * (1 - q) / 2 + 0.5f) — preserve the exact
	// rounding rule so the same value of confidenceInterval picks the
	// same selector index as the reference.
	selectorIndex := int(float32(len(arr))*(1-confidenceInterval)/2 + 0.5)
	if selectorIndex > 0 {
		sel := newFloatIntroSelector(arr)
		sel.Select(0, len(arr), len(arr)-selectorIndex)
		sel.Select(0, len(arr)-selectorIndex, selectorIndex)
	}
	min := float32(math.Inf(+1))
	max := float32(math.Inf(-1))
	for i := selectorIndex; i < len(arr)-selectorIndex; i++ {
		if arr[i] < min {
			min = arr[i]
		}
		if arr[i] > max {
			max = arr[i]
		}
	}
	return min, max
}

// sortFloat32Slice sorts a []float32 ascending using NaN-tolerant
// comparison; used by the small-input path of getUpperAndLowerQuantile.
func sortFloat32Slice(arr []float32) {
	sort.Slice(arr, func(i, j int) bool { return arr[i] < arr[j] })
}

// floatIntroSelector is the IntroSelector impl that mirrors Java's
// FloatSelector inner class: it selects on a []float32 in place.
type floatIntroSelector struct {
	util.Selector
	arr   []float32
	pivot float32
}

// newFloatIntroSelector wires a floatIntroSelector through
// util.NewIntroSelector so it inherits the introspective quickselect
// loop.
func newFloatIntroSelector(arr []float32) *util.IntroSelector {
	fs := &floatIntroSelector{arr: arr}
	return util.NewIntroSelector(fs)
}

func (fs *floatIntroSelector) Swap(i, j int) {
	fs.arr[i], fs.arr[j] = fs.arr[j], fs.arr[i]
}

// Select is unused for IntroSelectorInterface (the embedding selector
// drives the loop) but required by SelectorInterface; mirroring Java's
// abstract Selector.select(int from, int to, int k).
func (fs *floatIntroSelector) Select(from, to, k int) {
	util.NewIntroSelector(fs).Select(from, to, k)
}

func (fs *floatIntroSelector) Compare(i, j int) int {
	return compareFloat32(fs.arr[i], fs.arr[j])
}

func (fs *floatIntroSelector) SetPivot(i int)         { fs.pivot = fs.arr[i] }
func (fs *floatIntroSelector) ComparePivot(j int) int { return compareFloat32(fs.pivot, fs.arr[j]) }

// compareFloat32 returns a tri-valued comparison compatible with the
// IntroSelector contract: -1 for a<b, +1 for a>b, 0 for equal. NaNs
// sort high (any NaN compares greater than any non-NaN), which matches
// the IEEE-flavoured behaviour Lucene relies on for its quantile
// selection over a sample buffer.
//
// We do not distinguish -0 from +0 (Java's Float.compare treats them
// as distinct under -ea); the quantile bounds are unaffected by that
// distinction.
func compareFloat32(a, b float32) int {
	if a < b {
		return -1
	}
	if a > b {
		return +1
	}
	switch {
	case math.IsNaN(float64(a)) && math.IsNaN(float64(b)):
		return 0
	case math.IsNaN(float64(a)):
		return +1
	case math.IsNaN(float64(b)):
		return -1
	default:
		return 0
	}
}

// minMaxScalarQuantize is the inlined Go counterpart of Lucene's
// VectorUtil.minMaxScalarQuantize. It maps each src component onto
// [0, divisor] using (src - minQ) * scale, rounds with the standard
// banker-free round-half-up scheme, clamps to the valid byte range,
// and reports the dot-product correction sum_i (alpha * b_i +
// minQ) * minQ.
//
// Kept inline to avoid leaking a quantization-specific helper into
// util/. If VectorUtil.minMaxScalarQuantize is ported later, this
// function can become a thin shim.
func minMaxScalarQuantize(src []float32, dest []byte, scale, alpha, minQ, maxQ float32) float32 {
	if len(src) != len(dest) {
		panic(fmt.Sprintf("quantization: minMaxScalarQuantize: length mismatch %d!=%d", len(src), len(dest)))
	}
	var correctiveOffset float32
	for i, v := range src {
		// Clamp to the configured quantile range first so out-of-band
		// values cannot blow up the round result.
		x := v
		if x < minQ {
			x = minQ
		} else if x > maxQ {
			x = maxQ
		}
		scaled := (x - minQ) * scale
		// Java's `(int)(scaled + 0.5f)` performs truncation toward zero
		// after a half-up bias; since scaled >= 0 by construction, this
		// is equivalent to round-half-up. We replicate that without
		// math.Round to dodge banker's rounding semantics.
		q := int32(scaled + 0.5)
		if q < 0 {
			q = 0
		} else if q > 255 {
			q = 255
		}
		dest[i] = byte(q)
		// Reconstructed contribution to the dot-product correction.
		correctiveOffset += (alpha*float32(byte(q)) + minQ) * minQ
	}
	return correctiveOffset
}

// recalculateOffset is the inlined Go counterpart of Lucene's
// VectorUtil.recalculateOffset. It walks an already-quantised byte
// vector under the old quantiser (oldAlpha, oldMinQ) and computes the
// correction term that the new (scale, alpha, minQ, maxQ) quantiser
// would have produced. Used by RecalculateCorrectiveOffset.
func recalculateOffset(quantized []byte, oldAlpha, oldMinQ, scale, alpha, minQ, maxQ float32) float32 {
	var correctiveOffset float32
	for _, q := range quantized {
		// Dequantise under the old quantiser, then re-quantise under
		// the new one and accumulate the same correction term the
		// quantize path emits.
		x := oldAlpha*float32(q) + oldMinQ
		if x < minQ {
			x = minQ
		} else if x > maxQ {
			x = maxQ
		}
		scaled := (x - minQ) * scale
		nq := int32(scaled + 0.5)
		if nq < 0 {
			nq = 0
		} else if nq > 255 {
			nq = 255
		}
		correctiveOffset += (alpha*float32(byte(nq)) + minQ) * minQ
	}
	return correctiveOffset
}

// findNearestNeighbours computes the top-10 nearest neighbours of
// every supplied vector under the chosen similarity function. Mirrors
// Java's findNearestNeighbours private helper.
//
// The Java reference uses Lucene's HitQueue (a min-heap of ScoreDoc
// instances) to maintain a bounded top-K; we use a smaller bounded
// slice with manual minimum tracking because k is fixed at 10. The
// algorithmic invariant (every pair scored exactly once, score-doc.doc
// records the *other* ordinal) is preserved.
func findNearestNeighbours(vectors [][]float32, function index.VectorSimilarityFunction) []scoreDocsAndScoreVariance {
	type heap struct {
		// docs is the bounded top-K bucket, kept compact: index 0..n-1
		// hold valid (doc, score) pairs.
		docs []scoreDoc
		// minIdx marks the position of the smallest score in docs (only
		// meaningful when len(docs)==cap(docs)).
		minIdx int
	}
	const topK = 10
	heaps := make([]heap, len(vectors))
	for i := range heaps {
		heaps[i].docs = make([]scoreDoc, 0, topK)
	}
	insert := func(h *heap, sd scoreDoc) {
		if len(h.docs) < cap(h.docs) {
			h.docs = append(h.docs, sd)
			if len(h.docs) == cap(h.docs) {
				// Recompute the running minimum once the bucket is full.
				h.minIdx = 0
				for k := 1; k < len(h.docs); k++ {
					if h.docs[k].score < h.docs[h.minIdx].score {
						h.minIdx = k
					}
				}
			}
			return
		}
		if sd.score <= h.docs[h.minIdx].score {
			return
		}
		h.docs[h.minIdx] = sd
		// Rescan for the new minimum; k=10 so this stays O(1) in
		// constant factors.
		h.minIdx = 0
		for k := 1; k < len(h.docs); k++ {
			if h.docs[k].score < h.docs[h.minIdx].score {
				h.minIdx = k
			}
		}
	}
	for i := 0; i < len(vectors); i++ {
		for j := i + 1; j < len(vectors); j++ {
			score := computeSimilarity(function, vectors[i], vectors[j])
			insert(&heaps[i], scoreDoc{doc: j, score: score})
			insert(&heaps[j], scoreDoc{doc: i, score: score})
		}
	}
	result := make([]scoreDocsAndScoreVariance, len(vectors))
	var mv onlineMeanAndVar
	for i := range vectors {
		docs := heaps[i].docs
		// Sort ascending by score so the consumer sees the same order
		// (low -> high) that HitQueue.pop() would yield.
		sort.Slice(docs, func(a, b int) bool { return docs[a].score < docs[b].score })
		mv.reset()
		for _, sd := range docs {
			mv.add(float64(sd.score))
		}
		// Copy out of the heap so future insertions cannot mutate the
		// recorded slice.
		out := make([]scoreDoc, len(docs))
		copy(out, docs)
		result[i] = scoreDocsAndScoreVariance{scoreDocs: out, scoreVariance: mv.varF32()}
	}
	return result
}

// candidateGridSearch performs the two-phase quantile grid search
// matching Java's candidateGridSearch: a coarse sweep over the corners
// of a 4x4 quadrant followed by a refinement over the best quadrant.
// Returns the (lower, upper) pair with the highest score-error
// correlation.
func candidateGridSearch(
	neighbours []scoreDocsAndScoreVariance,
	vectors [][]float32,
	lowerCandidates, upperCandidates []float32,
	function index.VectorSimilarityFunction,
	bits byte,
) (float32, float32) {
	maxCorr := math.Inf(-1)
	var bestLower, bestUpper float32
	bestQuadrantLower := 0
	bestQuadrantUpper := 0
	corr := newScoreErrorCorrelator(function, neighbours, vectors, bits)
	for i := 0; i < len(lowerCandidates); i += 4 {
		lower := lowerCandidates[i]
		if isNaNOrInfFloat32(lower) {
			continue
		}
		for j := 0; j < len(upperCandidates); j += 4 {
			upper := upperCandidates[j]
			if isNaNOrInfFloat32(upper) {
				continue
			}
			if upper <= lower {
				continue
			}
			mean := corr.scoreErrorCorrelation(lower, upper)
			if mean > maxCorr {
				maxCorr = mean
				bestLower = lower
				bestUpper = upper
				bestQuadrantLower = i
				bestQuadrantUpper = j
			}
		}
	}
	for i := bestQuadrantLower + 1; i < bestQuadrantLower+4; i++ {
		for j := bestQuadrantUpper + 1; j < bestQuadrantUpper+4; j++ {
			lower := lowerCandidates[i]
			upper := upperCandidates[j]
			if isNaNOrInfFloat32(lower) || isNaNOrInfFloat32(upper) {
				continue
			}
			if upper <= lower {
				continue
			}
			mean := corr.scoreErrorCorrelation(lower, upper)
			if mean > maxCorr {
				maxCorr = mean
				bestLower = lower
				bestUpper = upper
			}
		}
	}
	return bestLower, bestUpper
}

// scoreDoc mirrors Lucene's ScoreDoc but keeps only the fields the
// grid search actually consults. The full ScoreDoc port lives in the
// search package; using it here would pull in the search-side
// transitive deps unnecessarily.
type scoreDoc struct {
	doc   int
	score float32
}

// scoreDocsAndScoreVariance pairs a top-K scoring bucket with the
// variance of its scores. Mirrors Java's private record of the same
// name.
type scoreDocsAndScoreVariance struct {
	scoreDocs     []scoreDoc
	scoreVariance float32
}

// onlineMeanAndVar is the Welford-style streaming mean/variance
// accumulator that the Java reference inlines into its private inner
// class.
type onlineMeanAndVar struct {
	mean float64
	v    float64
	n    int
}

func (m *onlineMeanAndVar) reset() { m.mean, m.v, m.n = 0, 0, 0 }
func (m *onlineMeanAndVar) add(x float64) {
	m.n++
	delta := x - m.mean
	m.mean += delta / float64(m.n)
	m.v += delta * (x - m.mean)
}
func (m *onlineMeanAndVar) varF32() float32 {
	if m.n < 2 {
		return float32(math.NaN())
	}
	return float32(m.v / float64(m.n-1))
}

// scoreErrorCorrelator implements the Java inner class of the same
// name. It evaluates the score-error correlation under a trial
// (lower, upper) quantile pair by quantising each sample vector,
// scoring it against its top-K neighbours, and tracking how the
// variance of the quantised-vs-float score gap relates to the variance
// of the floating-point scores.
type scoreErrorCorrelator struct {
	function   index.VectorSimilarityFunction
	neighbours []scoreDocsAndScoreVariance
	vectors    [][]float32
	query      []byte
	vector     []byte
	bits       byte
	corr       onlineMeanAndVar
	errors     onlineMeanAndVar
}

func newScoreErrorCorrelator(
	function index.VectorSimilarityFunction,
	neighbours []scoreDocsAndScoreVariance,
	vectors [][]float32,
	bits byte,
) *scoreErrorCorrelator {
	return &scoreErrorCorrelator{
		function:   function,
		neighbours: neighbours,
		vectors:    vectors,
		query:      make([]byte, len(vectors[0])),
		vector:     make([]byte, len(vectors[0])),
		bits:       bits,
	}
}

func (c *scoreErrorCorrelator) scoreErrorCorrelation(lowerQuantile, upperQuantile float32) float64 {
	c.corr.reset()
	q, err := NewScalarQuantizer(lowerQuantile, upperQuantile, c.bits)
	if err != nil {
		// Should not happen: candidateGridSearch filters out NaN/Inf
		// inputs upstream and upper > lower is already enforced.
		return 0
	}
	sim, err := FromVectorSimilarity(c.function, q.GetConstantMultiplier(), c.bits)
	if err != nil {
		return 0
	}
	for i := range c.neighbours {
		queryCorrection := q.Quantize(c.vectors[i], c.query, c.function)
		bucket := c.neighbours[i]
		c.errors.reset()
		for _, sd := range bucket.scoreDocs {
			vectorCorrection := q.Quantize(c.vectors[sd.doc], c.vector, c.function)
			qScore := sim.Score(c.query, queryCorrection, c.vector, vectorCorrection)
			c.errors.add(float64(qScore - sd.score))
		}
		c.corr.add(float64(1 - c.errors.varF32()/bucket.scoreVariance))
	}
	if math.IsNaN(c.corr.mean) {
		return 0
	}
	return c.corr.mean
}

// computeSimilarity is the local counterpart of codecs.ComputeSimilarity
// used during the auto-interval grid search. Pulling the codecs helper
// in would introduce an import cycle, so we duplicate the four-case
// dispatch here. The numeric output matches codecs.ComputeSimilarity
// for the same inputs.
func computeSimilarity(simFunc index.VectorSimilarityFunction, v1, v2 []float32) float32 {
	switch simFunc {
	case index.VectorSimilarityFunctionEuclidean:
		return util.NormalizeDistanceToUnitInterval(util.SquareDistance(v1, v2))
	case index.VectorSimilarityFunctionDotProduct:
		return util.NormalizeToUnitInterval(util.DotProduct(v1, v2))
	case index.VectorSimilarityFunctionCosine:
		return util.NormalizeToUnitInterval(util.Cosine(v1, v2))
	case index.VectorSimilarityFunctionMaximumInnerProduct:
		return util.ScaleMaxInnerProductScore(util.DotProduct(v1, v2))
	default:
		return 0
	}
}

// isNaNOrInfFloat32 reports whether v is NaN or +/-Inf, with the
// minimum allocation footprint (float32 -> float64 conversion is free
// on AMD64/ARM64 and stays inlinable).
func isNaNOrInfFloat32(v float32) bool {
	f := float64(v)
	return math.IsNaN(f) || math.IsInf(f, 0)
}
