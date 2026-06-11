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

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// MinimumMSEGrid holds the optimal starting intervals for each
// bit-width in (0, 8]. Row i (0-based) corresponds to bit-width i+1
// and contains the symmetric (lower, upper) pair that minimises the
// mean-squared error for a unit-variance uniform distribution.
//
// Mirrors org.apache.lucene.util.quantization.OptimizedScalarQuantizer.MINIMUM_MSE_GRID
// (Lucene 10.4.0). The values are deliberately frozen as float32 to
// match the Java reference's `static final float[][]` table; promoting
// them to float64 would shift the optimisation seed and break
// byte-for-byte parity at higher bit-widths.
var MinimumMSEGrid = [8][2]float32{
	{-0.798, 0.798},
	{-1.493, 1.493},
	{-2.051, 2.051},
	{-2.514, 2.514},
	{-2.916, 2.916},
	{-3.278, 3.278},
	{-3.611, 3.611},
	{-3.922, 3.922},
}

// Default tuning constants for [OptimizedScalarQuantizer]. Mirror the
// Java reference's `DEFAULT_LAMBDA` and `DEFAULT_ITERS` private
// constants verbatim.
const (
	// defaultLambda is the anisotropy parameter applied by
	// [NewOptimizedScalarQuantizer]. Smaller values favour reducing the
	// component of the quantisation error parallel to the embedding at
	// the cost of larger overall error.
	defaultLambda = 0.1
	// defaultIters caps the coordinate-descent loop inside
	// [OptimizedScalarQuantizer.optimizeIntervals].
	defaultIters = 5
)

// OptimizedScalarQuantizer is a scalar quantiser that adapts its
// quantisation interval to each input vector via coordinate descent.
// Port of org.apache.lucene.util.quantization.OptimizedScalarQuantizer
// (Lucene 10.4.0).
//
// Unlike [ScalarQuantizer], OptimizedScalarQuantizer derives its
// per-vector bounds from a centroid-subtracted view and refines them
// to minimise an anisotropic MSE loss. The result is a tighter
// dynamic range, particularly at low bit-widths.
//
// The zero value is not usable; build instances via
// [NewOptimizedScalarQuantizer] or [NewOptimizedScalarQuantizerWith].
// OptimizedScalarQuantizer is immutable after construction and safe
// for concurrent use; the per-call vector slices passed to
// [OptimizedScalarQuantizer.ScalarQuantize] and
// [OptimizedScalarQuantizer.MultiScalarQuantize] are mutated in
// place, matching the Java reference, so callers that need to retain
// the raw vector must copy upfront.
type OptimizedScalarQuantizer struct {
	similarityFunction index.VectorSimilarityFunction
	// lambda controls how much the optimisation tolerates errors
	// parallel to the input vector. Smaller values bias the loss
	// toward minimising parallel error at the cost of larger overall
	// error. Held as float32 to match the Java field, then promoted to
	// float64 inside the loss kernel for stable accumulation.
	lambda float32
	// iters caps the coordinate-descent inner loop.
	iters int
}

// QuantizationResult captures the bookkeeping emitted by every
// quantisation pass: the optimised interval, the similarity-dependent
// correction term, and the sum of the quantised components.
//
// Mirrors the Java reference's `record QuantizationResult` with the
// same field ordering and semantics. The fields are exported because
// callers downstream (codecs, scorers) consume them directly; the
// type is intentionally value-typed so it travels cheaply through
// slices.
type QuantizationResult struct {
	// LowerInterval is the lower bound of the optimised quantisation
	// interval, mirroring Java's `lowerInterval()` accessor.
	LowerInterval float32
	// UpperInterval is the upper bound of the optimised quantisation
	// interval, mirroring Java's `upperInterval()` accessor.
	UpperInterval float32
	// AdditionalCorrection carries the per-vector correction term that
	// scorers need to reconstruct the dequantised similarity. For
	// Euclidean similarity it equals the squared norm of the centred
	// vector; for the other similarity functions it equals the dot
	// product between the raw vector and the centroid. Mirrors Java's
	// `additionalCorrection()` accessor.
	AdditionalCorrection float32
	// QuantizedComponentSum is the sum of all quantised components
	// produced by the quantisation pass. Mirrors Java's
	// `quantizedComponentSum()` accessor.
	QuantizedComponentSum int32
}

// NewOptimizedScalarQuantizer constructs an OptimizedScalarQuantizer
// using the Java reference's default lambda (0.1) and iteration cap
// (5). Mirrors the Java single-argument constructor.
func NewOptimizedScalarQuantizer(similarityFunction index.VectorSimilarityFunction) *OptimizedScalarQuantizer {
	return NewOptimizedScalarQuantizerWith(similarityFunction, defaultLambda, defaultIters)
}

// NewOptimizedScalarQuantizerWith constructs an OptimizedScalarQuantizer
// with an explicit lambda and iteration cap. Mirrors the Java
// three-argument constructor.
//
// lambda controls anisotropy (smaller values bias toward minimising
// parallel error); iters caps the coordinate-descent loop. The Java
// reference accepts both values without validation; we mirror that
// permissive behaviour so the constructor stays infallible.
func NewOptimizedScalarQuantizerWith(
	similarityFunction index.VectorSimilarityFunction,
	lambda float32,
	iters int,
) *OptimizedScalarQuantizer {
	return &OptimizedScalarQuantizer{
		similarityFunction: similarityFunction,
		lambda:             lambda,
		iters:              iters,
	}
}

// MultiScalarQuantize quantises vector at every requested bit-width
// in a single pass over the data. Each entry of destinations receives
// the quantisation for the matching entry of bits; the returned slice
// carries one [QuantizationResult] per bit-width.
//
// vector is centred against centroid in place (matching the Java
// reference's mutation contract). Each row of destinations must have
// the same length as vector and the same length as bits. Cosine
// similarity additionally requires both vector and centroid to be
// unit-length; the check is enforced even when assertions would be
// disabled in Java because production correctness depends on it.
//
// Returns an error if any input is malformed: nil arguments,
// length mismatches, a non-unit input under Cosine, or a bit-width
// outside (0, 8].
func (q *OptimizedScalarQuantizer) MultiScalarQuantize(
	vector []float32,
	destinations [][]byte,
	bits []byte,
	centroid []float32,
) ([]QuantizationResult, error) {
	if vector == nil {
		return nil, fmt.Errorf("quantization: MultiScalarQuantize: vector is nil")
	}
	if centroid == nil {
		return nil, fmt.Errorf("quantization: MultiScalarQuantize: centroid is nil")
	}
	if len(bits) != len(destinations) {
		return nil, fmt.Errorf("quantization: MultiScalarQuantize: bits/destinations length mismatch (%d vs %d)",
			len(bits), len(destinations))
	}
	if len(vector) != len(centroid) {
		return nil, fmt.Errorf("quantization: MultiScalarQuantize: vector/centroid length mismatch (%d vs %d)",
			len(vector), len(centroid))
	}
	for i, dest := range destinations {
		if len(dest) < len(vector) {
			return nil, fmt.Errorf("quantization: MultiScalarQuantize: destinations[%d] too small (%d < %d)",
				i, len(dest), len(vector))
		}
	}
	if q.similarityFunction == index.VectorSimilarityFunctionCosine {
		if !util.IsUnitVector(vector) {
			return nil, fmt.Errorf("quantization: MultiScalarQuantize: vector must be unit-length under COSINE")
		}
		if !util.IsUnitVector(centroid) {
			return nil, fmt.Errorf("quantization: MultiScalarQuantize: centroid must be unit-length under COSINE")
		}
	}

	var intervalScratch [2]float32
	vecMean := 0.0
	vecVar := 0.0
	var norm2 float32
	var centroidDot float32
	min := float32(math.MaxFloat32)
	max := -float32(math.MaxFloat32)

	// First pass: centre the vector, accumulate norm2, centroidDot,
	// running mean and variance (Welford). The Java reference mutates
	// vector[i] in place; we mirror that exactly.
	euclidean := q.similarityFunction == index.VectorSimilarityFunctionEuclidean
	for i := 0; i < len(vector); i++ {
		if !euclidean {
			centroidDot += vector[i] * centroid[i]
		}
		vector[i] = vector[i] - centroid[i]
		if vector[i] < min {
			min = vector[i]
		}
		if vector[i] > max {
			max = vector[i]
		}
		norm2 += vector[i] * vector[i]
		delta := float64(vector[i]) - vecMean
		vecMean += delta / float64(i+1)
		vecVar += delta * (float64(vector[i]) - vecMean)
	}
		if len(vector) == 0 {
		return nil, fmt.Errorf("ScalarQuantize: empty vector")
	}
	vecVar /= float64(len(vector))
	vecStd := math.Sqrt(vecVar)

	results := make([]QuantizationResult, len(bits))
	for i := 0; i < len(bits); i++ {
		b := bits[i]
		if b == 0 || b > 8 {
			return nil, fmt.Errorf("quantization: MultiScalarQuantize: bits[%d] must satisfy 0 < bits <= 8, got %d", i, b)
		}
		points := 1 << b
		// Seed the interval from the bit-width-specific MSE grid then
		// clamp into the observed [min, max] range.
		intervalScratch[0] = float32(clampF64(
			float64(MinimumMSEGrid[b-1][0])*vecStd+vecMean,
			float64(min), float64(max)))
		intervalScratch[1] = float32(clampF64(
			float64(MinimumMSEGrid[b-1][1])*vecStd+vecMean,
			float64(min), float64(max)))
		q.optimizeIntervals(&intervalScratch, vector, norm2, points)
		nSteps := float32(int(1<<b) - 1)
		a := intervalScratch[0]
		bb := intervalScratch[1]
		step := (bb - a) / nSteps
		var sumQuery int32
		dest := destinations[i]
		for h := 0; h < len(vector); h++ {
			xi := clampF32(vector[h], a, bb)
			var assignment int32
			if step != 0 {
				assignment = int32(roundHalfUpF32((xi - a) / step))
			}
			sumQuery += assignment
			dest[h] = byte(assignment)
		}
		var additional float32
		if euclidean {
			additional = norm2
		} else {
			additional = centroidDot
		}
		results[i] = QuantizationResult{
			LowerInterval:         intervalScratch[0],
			UpperInterval:         intervalScratch[1],
			AdditionalCorrection:  additional,
			QuantizedComponentSum: sumQuery,
		}
	}
	return results, nil
}

// ScalarQuantize quantises vector at a single bit-width and writes
// the result into destination. Returns the per-vector
// [QuantizationResult] describing the chosen interval and the scoring
// corrections.
//
// vector is centred against centroid in place (matching the Java
// reference's mutation contract). destination must be at least as
// long as vector. Cosine similarity additionally requires both
// vector and centroid to be unit-length. Returns an error if any
// input is malformed: nil arguments, length mismatches, a non-unit
// input under Cosine, or a bit-width outside (0, 8].
func (q *OptimizedScalarQuantizer) ScalarQuantize(
	vector []float32,
	destination []byte,
	bits byte,
	centroid []float32,
) (QuantizationResult, error) {
	var zero QuantizationResult
	if vector == nil {
		return zero, fmt.Errorf("quantization: ScalarQuantize: vector is nil")
	}
	if centroid == nil {
		return zero, fmt.Errorf("quantization: ScalarQuantize: centroid is nil")
	}
	if len(vector) != len(centroid) {
		return zero, fmt.Errorf("quantization: ScalarQuantize: vector/centroid length mismatch (%d vs %d)",
			len(vector), len(centroid))
	}
	if len(vector) > len(destination) {
		return zero, fmt.Errorf("quantization: ScalarQuantize: destination too small (%d < %d)",
			len(destination), len(vector))
	}
	if bits == 0 || bits > 8 {
		return zero, fmt.Errorf("quantization: ScalarQuantize: bits must satisfy 0 < bits <= 8, got %d", bits)
	}
	if q.similarityFunction == index.VectorSimilarityFunctionCosine {
		if !util.IsUnitVector(vector) {
			return zero, fmt.Errorf("quantization: ScalarQuantize: vector must be unit-length under COSINE")
		}
		if !util.IsUnitVector(centroid) {
			return zero, fmt.Errorf("quantization: ScalarQuantize: centroid must be unit-length under COSINE")
		}
	}

	points := 1 << bits
	vecMean := 0.0
	vecVar := 0.0
	var norm2 float32
	var centroidDot float32
	min := float32(math.MaxFloat32)
	max := -float32(math.MaxFloat32)

	euclidean := q.similarityFunction == index.VectorSimilarityFunctionEuclidean
	for i := 0; i < len(vector); i++ {
		if !euclidean {
			centroidDot += vector[i] * centroid[i]
		}
		vector[i] = vector[i] - centroid[i]
		if vector[i] < min {
			min = vector[i]
		}
		if vector[i] > max {
			max = vector[i]
		}
		norm2 += vector[i] * vector[i]
		delta := float64(vector[i]) - vecMean
		vecMean += delta / float64(i+1)
		vecVar += delta * (float64(vector[i]) - vecMean)
	}
		if len(vector) == 0 {
		return QuantizationResult{}, fmt.Errorf("ScalarQuantize: empty vector")
	}
	vecVar /= float64(len(vector))
	vecStd := math.Sqrt(vecVar)

	var intervalScratch [2]float32
	intervalScratch[0] = float32(clampF64(
		float64(MinimumMSEGrid[bits-1][0])*vecStd+vecMean,
		float64(min), float64(max)))
	intervalScratch[1] = float32(clampF64(
		float64(MinimumMSEGrid[bits-1][1])*vecStd+vecMean,
		float64(min), float64(max)))
	q.optimizeIntervals(&intervalScratch, vector, norm2, points)

	nSteps := float32(int(1<<bits) - 1)
	a := intervalScratch[0]
	bb := intervalScratch[1]
	step := (bb - a) / nSteps
	var sumQuery int32
	for h := 0; h < len(vector); h++ {
		xi := clampF32(vector[h], a, bb)
		var assignment int32
		if step != 0 {
			assignment = int32(roundHalfUpF32((xi - a) / step))
		}
		sumQuery += assignment
		destination[h] = byte(assignment)
	}
	var additional float32
	if euclidean {
		additional = norm2
	} else {
		additional = centroidDot
	}
	return QuantizationResult{
		LowerInterval:         intervalScratch[0],
		UpperInterval:         intervalScratch[1],
		AdditionalCorrection:  additional,
		QuantizedComponentSum: sumQuery,
	}, nil
}

// DeQuantize reconstructs a centroid-shifted float vector from its
// quantised bytes. The implementation is the inverse of the linear
// mapping applied by [OptimizedScalarQuantizer.ScalarQuantize].
//
// quantized's bytes are interpreted as unsigned (matching Java's
// `quantized[h] & 0xFF`). dequantized must be at least as long as
// quantized; the function returns dequantized for call chaining,
// mirroring the Java signature.
//
// Returns an error if the arguments are malformed: nil slices,
// dequantized too short, length mismatch with centroid, or a
// bit-width outside (0, 8].
func DeQuantize(
	quantized []byte,
	dequantized []float32,
	bits byte,
	lowerInterval, upperInterval float32,
	centroid []float32,
) ([]float32, error) {
	if quantized == nil {
		return nil, fmt.Errorf("quantization: DeQuantize: quantized is nil")
	}
	if dequantized == nil {
		return nil, fmt.Errorf("quantization: DeQuantize: dequantized is nil")
	}
	if centroid == nil {
		return nil, fmt.Errorf("quantization: DeQuantize: centroid is nil")
	}
	if len(quantized) > len(dequantized) {
		return nil, fmt.Errorf("quantization: DeQuantize: dequantized too small (%d < %d)",
			len(dequantized), len(quantized))
	}
	if len(centroid) < len(quantized) {
		return nil, fmt.Errorf("quantization: DeQuantize: centroid too small (%d < %d)",
			len(centroid), len(quantized))
	}
	if bits == 0 || bits > 8 {
		return nil, fmt.Errorf("quantization: DeQuantize: bits must satisfy 0 < bits <= 8, got %d", bits)
	}
	nSteps := (1 << bits) - 1
	step := (float64(upperInterval) - float64(lowerInterval)) / float64(nSteps)
	for h := 0; h < len(quantized); h++ {
		xi := float64(quantized[h])*step + float64(lowerInterval)
		dequantized[h] = float32(xi + float64(centroid[h]))
	}
	return dequantized, nil
}

// loss evaluates the anisotropic MSE loss used by
// [OptimizedScalarQuantizer.optimizeIntervals]. Mirrors the Java
// private loss helper. The math is intentionally kept in float64 to
// preserve the reference's accumulation behaviour.
func (q *OptimizedScalarQuantizer) loss(vector []float32, interval [2]float32, points int, norm2 float32) float64 {
	a := float64(interval[0])
	b := float64(interval[1])
	step := (b - a) / (float64(points) - 1.0)
	stepInv := 1.0 / step
	var xe, e float64
	for _, xiF := range vector {
		xi := float64(xiF)
		// Java writes `step * Math.round((clamp(xi,a,b)-a) * stepInv)`;
		// the multiplication associativity must match exactly because
		// the order of operations changes the rounded float64 result.
		//
		// Java's Math.round(double) returns 0 for NaN (the result is
		// `(long) Math.floor(x + 0.5d)`, and `(long) NaN` == 0). When
		// step == 0 the multiplication (clamp - a) * stepInv evaluates
		// to `0 * Inf` = NaN; routing NaN through Java's round() yields
		// 0, keeping the loss finite. We must reproduce that exactly.
		var rounded float64
		raw := (clampF64(xi, a, b) - a) * stepInv
		if math.IsNaN(raw) {
			rounded = 0
		} else {
			rounded = math.Round(raw)
		}
		xiq := a + step*rounded
		xe += xi * (xi - xiq)
		e += (xi - xiq) * (xi - xiq)
	}
	lambda := float64(q.lambda)
	return (1.0-lambda)*xe*xe/float64(norm2) + lambda*e
}

// optimizeIntervals performs the coordinate-descent inner loop that
// tightens the quantisation interval. Mirrors the Java private
// optimizeIntervals helper byte-for-byte; the loss is not guaranteed
// to decrease monotonically, so we bail out as soon as it goes up.
func (q *OptimizedScalarQuantizer) optimizeIntervals(initInterval *[2]float32, vector []float32, norm2 float32, points int) {
	initialLoss := q.loss(vector, *initInterval, points, norm2)
	scale := (1.0 - q.lambda) / norm2
	if math.IsNaN(float64(scale)) || math.IsInf(float64(scale), 0) {
		return
	}
	for it := 0; it < q.iters; it++ {
		a := initInterval[0]
		b := initInterval[1]
		stepInv := float32(points-1) / (b - a)
		var daa, dab, dbb, dax, dbx float64
		for _, xiF := range vector {
			k := roundHalfUpF32((clampF32(xiF, a, b) - a) * stepInv)
			s := float64(k) / float64(points-1)
			daa += (1.0 - s) * (1.0 - s)
			dab += (1.0 - s) * s
			dbb += s * s
			dax += float64(xiF) * (1.0 - s)
			dbx += float64(xiF) * s
		}
		lambda := float64(q.lambda)
		m0 := float64(scale)*dax*dax + lambda*daa
		m1 := float64(scale)*dax*dbx + lambda*dab
		m2 := float64(scale)*dbx*dbx + lambda*dbb
		det := m0*m2 - m1*m1
		if det == 0 {
			return
		}
		aOpt := float32((m2*dax - m1*dbx) / det)
		bOpt := float32((m0*dbx - m1*dax) / det)
		// Early exit if the candidate interval is indistinguishable
		// from the current one.
		if absF32(initInterval[0]-aOpt) < 1e-8 && absF32(initInterval[1]-bOpt) < 1e-8 {
			return
		}
		newLoss := q.loss(vector, [2]float32{aOpt, bOpt}, points, norm2)
		// Loss went up: exit without updating.
		if newLoss > initialLoss {
			return
		}
		initInterval[0] = aOpt
		initInterval[1] = bOpt
		initialLoss = newLoss
	}
}

// Discretize rounds value up to the next multiple of bucket. Mirrors
// the Java static helper of the same name.
func Discretize(value, bucket int) int {
	return ((value + (bucket - 1)) / bucket) * bucket
}

// TransposeHalfByte rearranges a half-byte (4-bit) quantised query
// vector into four bit-planes packed back-to-back, enabling fast
// bitwise comparison against indexed binary vectors.
//
// q is expected to hold 4-bit values (0..15) in the low nibble of
// each byte. The first quarter of quantQueryByte receives the
// low-order bits of every dimension, the second the next bit, and so
// on. Mirrors Java's `transposeHalfByte`.
//
// Returns an error if any q[i] is outside [0, 15], rather than
// silently corrupting the output.
func TransposeHalfByte(q []byte, quantQueryByte []byte) error {
	if len(quantQueryByte) < len(q)*3 {
		panic(fmt.Sprintf("TransposeHalfByte: quantQueryByte too small (%d < %d)", len(quantQueryByte), len(q)*3))
	}

	for _, v := range q {
		if v > 15 {
			return fmt.Errorf("quantization: TransposeHalfByte: input value %d exceeds 15", v)
		}
	}
	quarter := len(quantQueryByte) / 4
	for i := 0; i < len(q); {
		var lowerByte, lowerMiddleByte, upperMiddleByte, upperByte int
		for j := 7; j >= 0 && i < len(q); j-- {
			lowerByte |= int(q[i]&1) << j
			lowerMiddleByte |= int((q[i]>>1)&1) << j
			upperMiddleByte |= int((q[i]>>2)&1) << j
			upperByte |= int((q[i]>>3)&1) << j
			i++
		}
		index := ((i + 7) / 8) - 1
		quantQueryByte[index] = byte(lowerByte)
		quantQueryByte[index+quarter] = byte(lowerMiddleByte)
		quantQueryByte[index+2*quarter] = byte(upperMiddleByte)
		quantQueryByte[index+3*quarter] = byte(upperByte)
	}
	return nil
}

// PackAsBinary packs the binary vector (each byte 0 or 1) into the
// supplied byte array, MSB first. Mirrors Java's `packAsBinary`.
//
// Returns an error if any element of vector is outside {0, 1}.
func PackAsBinary(vector, packed []byte) error {
	for _, v := range vector {
		if v != 0 && v != 1 {
			return fmt.Errorf("quantization: PackAsBinary: input value %d not in {0, 1}", v)
		}
	}
	for i := 0; i < len(vector); {
		var result byte
		for j := 7; j >= 0 && i < len(vector); j-- {
			result |= byte((vector[i] & 1) << j)
			i++
		}
		idx := ((i + 7) / 8) - 1
		if idx >= len(packed) {
			return fmt.Errorf("quantization: PackAsBinary: packed too small (need at least %d, got %d)",
				idx+1, len(packed))
		}
		packed[idx] = result
	}
	return nil
}

// UnpackBinary unpacks a binary-packed byte array back into a vector
// of 0/1 bytes, MSB first. Mirrors Java's `unpackBinary`.
//
// The function stops once vector is filled; any extra bits in packed
// are ignored, matching the Java semantics.
func UnpackBinary(packed, vector []byte) {
	vectorIndex := 0
	for packedIndex := 0; packedIndex < len(packed) && vectorIndex < len(vector); packedIndex++ {
		packedByte := packed[packedIndex]
		for j := 7; j >= 0 && vectorIndex < len(vector); j-- {
			vector[vectorIndex] = (packedByte >> j) & 1
			vectorIndex++
		}
	}
}

// TransposeDibit rearranges a 2-bit (dibit) quantised vector into two
// bit-planes packed back-to-back, the dibit analogue of
// [TransposeHalfByte]. Mirrors Java's `transposeDibit`.
//
// The Java reference assumes the caller has sized `packed` correctly;
// out-of-bounds writes would corrupt memory. We return an error
// instead.
func TransposeDibit(vector, packed []byte) error {
	half := len(packed) / 2
	limit := len(vector) - 7
	i := 0
	index := 0
	// Fast path: SIMD-friendly 8-at-a-time stripe, identical to Java.
	for ; i < limit; i, index = i+8, index+1 {
		lowerByte := (int(vector[i])&1)<<7 |
			(int(vector[i+1])&1)<<6 |
			(int(vector[i+2])&1)<<5 |
			(int(vector[i+3])&1)<<4 |
			(int(vector[i+4])&1)<<3 |
			(int(vector[i+5])&1)<<2 |
			(int(vector[i+6])&1)<<1 |
			(int(vector[i+7]) & 1)
		upperByte := ((int(vector[i])>>1)&1)<<7 |
			((int(vector[i+1])>>1)&1)<<6 |
			((int(vector[i+2])>>1)&1)<<5 |
			((int(vector[i+3])>>1)&1)<<4 |
			((int(vector[i+4])>>1)&1)<<3 |
			((int(vector[i+5])>>1)&1)<<2 |
			((int(vector[i+6])>>1)&1)<<1 |
			((int(vector[i+7]) >> 1) & 1)
		if index >= half {
			return fmt.Errorf("quantization: TransposeDibit: packed too small (need at least %d, got %d)",
				2*(index+1), len(packed))
		}
		packed[index] = byte(lowerByte)
		packed[index+half] = byte(upperByte)
	}
	if i == len(vector) {
		return nil
	}
	// Tail path: 1..7 leftover dibits packed MSB-first.
	var lowerByte, upperByte int
	for j := 7; i < len(vector); j, i = j-1, i+1 {
		if vector[i] > 3 {
			return fmt.Errorf("quantization: TransposeDibit: input value %d exceeds 3", vector[i])
		}
		lowerByte |= (int(vector[i]) & 1) << j
		upperByte |= ((int(vector[i]) >> 1) & 1) << j
	}
	if index >= half {
		return fmt.Errorf("quantization: TransposeDibit: packed too small (need at least %d, got %d)",
			2*(index+1), len(packed))
	}
	packed[index] = byte(lowerByte)
	packed[index+half] = byte(upperByte)
	return nil
}

// UntransposeDibit reverses [TransposeDibit]: unpack the two
// bit-planes of `packed` into one byte per dimension of `vector`,
// each carrying a value in {0, 1, 2, 3}. Mirrors Java's
// `untransposeDibit`.
func UntransposeDibit(packed, vector []byte) {
	if len(packed) == 0 {
		return // nothing to untranspose
	}

	stripeSize := len(packed) / 2
	limit := len(vector) - 7
	i := 0
	index := 0
	for ; i < limit; i, index = i+8, index+1 {
		lowerByte := packed[index]
		upperByte := packed[index+stripeSize]
		vector[i] = ((lowerByte >> 7) & 1) | (((upperByte >> 7) & 1) << 1)
		vector[i+1] = ((lowerByte >> 6) & 1) | (((upperByte >> 6) & 1) << 1)
		vector[i+2] = ((lowerByte >> 5) & 1) | (((upperByte >> 5) & 1) << 1)
		vector[i+3] = ((lowerByte >> 4) & 1) | (((upperByte >> 4) & 1) << 1)
		vector[i+4] = ((lowerByte >> 3) & 1) | (((upperByte >> 3) & 1) << 1)
		vector[i+5] = ((lowerByte >> 2) & 1) | (((upperByte >> 2) & 1) << 1)
		vector[i+6] = ((lowerByte >> 1) & 1) | (((upperByte >> 1) & 1) << 1)
		vector[i+7] = (lowerByte & 1) | ((upperByte & 1) << 1)
	}
	if i < len(vector) {
		lowerByte := packed[index]
		upperByte := packed[index+stripeSize]
		for j := 7; i < len(vector); j, i = j-1, i+1 {
			vector[i] = ((lowerByte >> j) & 1) | (((upperByte >> j) & 1) << 1)
		}
	}
}

// clampF32 mirrors Java's static clamp(double,double,double) but on
// float32. Returns a if x < a, b if x > b, else x. NaN propagates
// through the comparisons identically to Java.
func clampF32(x, a, b float32) float32 {
	if x < a {
		x = a
	}
	if x > b {
		x = b
	}
	return x
}

// clampF64 mirrors Java's static clamp(double,double,double).
func clampF64(x, a, b float64) float64 {
	if x < a {
		x = a
	}
	if x > b {
		x = b
	}
	return x
}

// roundHalfUpF32 implements Java's `Math.round((float) x)`: round to
// nearest, ties up.
//
// Java's contract differs from `math.Floor(x+0.5)` in two corner
// cases we must match exactly:
//
//   - NaN -> 0 (Java casts the float through `(long)` after adding
//     0.5, and `(long) NaN` is defined as 0). The optimisation kernel
//     hits this path whenever the interval collapses (a == b) and
//     stepInv becomes +Inf, multiplied by zero, yielding NaN.
//   - +Inf -> max long (Long.MAX_VALUE in Java); -Inf -> min long.
//     Neither value can be represented in float32 without overflow; we
//     mirror Java's saturation by returning ±MaxFloat32. The current
//     callers cap the result back into a small range, so this only
//     affects accumulator state.
func roundHalfUpF32(x float32) float32 {
	f := float64(x)
	if math.IsNaN(f) {
		return 0
	}
	if math.IsInf(f, +1) {
		return math.MaxFloat32
	}
	if math.IsInf(f, -1) {
		return -math.MaxFloat32
	}
	// math.Floor(x + 0.5) reproduces Java's Math.round contract for
	// the well-behaved float range.
	return float32(math.Floor(f + 0.5))
}

// absF32 returns the absolute value of x without going through float64.
func absF32(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}
