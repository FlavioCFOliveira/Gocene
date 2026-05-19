// Purpose: Scalar VectorUtilSupport implementation mirror.
// Lucene reference: org.apache.lucene.internal.vectorization.DefaultVectorUtilSupport
//   (lucene/core/src/java/org/apache/lucene/internal/vectorization/DefaultVectorUtilSupport.java).
//
// The Java original is a package-private final class implementing the
// VectorUtilSupport interface. Gocene exposes VectorUtilSupport as a struct
// (see [[vectorization]]) rather than an interface to keep the SIMD-stub
// surface flat; the scalar fallbacks below are attached as methods on that
// struct. The arithmetic follows the upstream scalar branch verbatim,
// including the >32-element unrolled paths, so behaviour is byte-equivalent
// to Lucene's non-FMA path.
//
// Deviations from upstream:
//   - No FMA fast path. Java's DefaultVectorUtilSupport gates on
//     Constants.HAS_FAST_SCALAR_FMA and delegates to Math.fma when true.
//     Go's stdlib does not expose a fused-multiply-add intrinsic for float32,
//     and the upstream non-FMA branch (a*b + c) is the documented fallback,
//     so we always take it. This matches what the Java implementation does on
//     hosts without fast FMA support.
//   - byte-as-signed semantics. Java treats byte as signed by default and
//     uses Byte.toUnsignedInt for the unsigned paths. Go's byte is unsigned,
//     so signed-byte arithmetic is recovered via int8 conversion in the
//     signed dot-product / cosine / squareDistance helpers.

package vectorization

import (
	"encoding/binary"
	"math"
	"math/bits"
)

// fma mirrors the scalar fma helper in the Java original. Lucene falls back to
// a*b + c when Constants.HAS_FAST_SCALAR_FMA is false; Go has no equivalent
// intrinsic for float32, so we always take that branch.
func fma(a, b, c float32) float32 {
	return a*b + c
}

// DotProductFloats returns the dot product of two equal-length float32
// vectors. Mirrors DefaultVectorUtilSupport.dotProduct(float[], float[]).
//
// The implementation uses a 4-way unrolled accumulator for inputs longer than
// 32 elements, matching the upstream layout so the rounding sequence is
// preserved.
func (s *VectorUtilSupport) DotProductFloats(a, b []float32) float32 {
	var res float32
	i := 0
	if len(a) > 32 {
		var acc1, acc2, acc3, acc4 float32
		upperBound := len(a) &^ (4 - 1)
		for ; i < upperBound; i += 4 {
			acc1 = fma(a[i], b[i], acc1)
			acc2 = fma(a[i+1], b[i+1], acc2)
			acc3 = fma(a[i+2], b[i+2], acc3)
			acc4 = fma(a[i+3], b[i+3], acc4)
		}
		res += acc1 + acc2 + acc3 + acc4
	}
	for ; i < len(a); i++ {
		res = fma(a[i], b[i], res)
	}
	return res
}

// CosineFloats returns the cosine similarity of two equal-length float32
// vectors. Mirrors DefaultVectorUtilSupport.cosine(float[], float[]) with the
// 2-way unrolled path for inputs longer than 32 elements.
func (s *VectorUtilSupport) CosineFloats(a, b []float32) float32 {
	var sum, norm1, norm2 float32
	i := 0
	if len(a) > 32 {
		var sum1, sum2 float32
		var norm11, norm12 float32
		var norm21, norm22 float32

		upperBound := len(a) &^ (2 - 1)
		for ; i < upperBound; i += 2 {
			sum1 = fma(a[i], b[i], sum1)
			norm11 = fma(a[i], a[i], norm11)
			norm21 = fma(b[i], b[i], norm21)

			sum2 = fma(a[i+1], b[i+1], sum2)
			norm12 = fma(a[i+1], a[i+1], norm12)
			norm22 = fma(b[i+1], b[i+1], norm22)
		}
		sum += sum1 + sum2
		norm1 += norm11 + norm12
		norm2 += norm21 + norm22
	}
	for ; i < len(a); i++ {
		sum = fma(a[i], b[i], sum)
		norm1 = fma(a[i], a[i], norm1)
		norm2 = fma(b[i], b[i], norm2)
	}
	return float32(float64(sum) / math.Sqrt(float64(norm1)*float64(norm2)))
}

// SquareDistanceFloats returns the squared Euclidean distance between two
// equal-length float32 vectors. Mirrors
// DefaultVectorUtilSupport.squareDistance(float[], float[]) with the 4-way
// unrolled path for inputs longer than 32 elements.
func (s *VectorUtilSupport) SquareDistanceFloats(a, b []float32) float32 {
	var res float32
	i := 0
	if len(a) > 32 {
		var acc1, acc2, acc3, acc4 float32

		upperBound := len(a) &^ (4 - 1)
		for ; i < upperBound; i += 4 {
			diff1 := a[i] - b[i]
			acc1 = fma(diff1, diff1, acc1)

			diff2 := a[i+1] - b[i+1]
			acc2 = fma(diff2, diff2, acc2)

			diff3 := a[i+2] - b[i+2]
			acc3 = fma(diff3, diff3, acc3)

			diff4 := a[i+3] - b[i+3]
			acc4 = fma(diff4, diff4, acc4)
		}
		res += acc1 + acc2 + acc3 + acc4
	}
	for ; i < len(a); i++ {
		diff := a[i] - b[i]
		res = fma(diff, diff, res)
	}
	return res
}

// DotProductBytes returns the dot product of two equal-length byte vectors
// treating the bytes as signed (Java byte semantics). Mirrors
// DefaultVectorUtilSupport.dotProduct(byte[], byte[]).
func (s *VectorUtilSupport) DotProductBytes(a, b []byte) int {
	total := 0
	for i := 0; i < len(a); i++ {
		total += int(int8(a[i])) * int(int8(b[i]))
	}
	return total
}

// Uint8DotProduct returns the dot product of two equal-length byte vectors
// treating the bytes as unsigned. Mirrors
// DefaultVectorUtilSupport.uint8DotProduct(byte[], byte[]).
func (s *VectorUtilSupport) Uint8DotProduct(a, b []byte) int {
	total := 0
	for i := 0; i < len(a); i++ {
		total += int(a[i]) * int(b[i])
	}
	return total
}

// Int4DotProduct mirrors DefaultVectorUtilSupport.int4DotProduct, which
// delegates to the signed-byte dot product.
func (s *VectorUtilSupport) Int4DotProduct(a, b []byte) int {
	return s.DotProductBytes(a, b)
}

// Int4DotProductSinglePacked computes the int4 dot product between an
// unpacked vector and a singly-packed vector. Mirrors
// DefaultVectorUtilSupport.int4DotProductSinglePacked.
func (s *VectorUtilSupport) Int4DotProductSinglePacked(unpacked, packed []byte) int {
	total := 0
	for i := 0; i < len(packed); i++ {
		packedByte := packed[i]
		unpacked1 := int8(unpacked[i])
		unpacked2 := int8(unpacked[i+len(packed)])
		total += int(packedByte&0x0F) * int(unpacked2)
		total += int(packedByte>>4) * int(unpacked1)
	}
	return total
}

// Int4DotProductBothPacked computes the int4 dot product between two
// doubly-packed vectors. Mirrors
// DefaultVectorUtilSupport.int4DotProductBothPacked.
func (s *VectorUtilSupport) Int4DotProductBothPacked(a, b []byte) int {
	total := 0
	for i := 0; i < len(a); i++ {
		aByte := a[i]
		bByte := b[i]
		total += int(aByte&0x0F) * int(bByte&0x0F)
		total += int(aByte>>4) * int(bByte>>4)
	}
	return total
}

// CosineBytes returns the cosine similarity of two equal-length signed-byte
// vectors. Mirrors DefaultVectorUtilSupport.cosine(byte[], byte[]). The
// upstream note about non-overflow when dim < 2^18 applies unchanged.
func (s *VectorUtilSupport) CosineBytes(a, b []byte) float32 {
	sum := 0
	norm1 := 0
	norm2 := 0
	for i := 0; i < len(a); i++ {
		elem1 := int(int8(a[i]))
		elem2 := int(int8(b[i]))
		sum += elem1 * elem2
		norm1 += elem1 * elem1
		norm2 += elem2 * elem2
	}
	return float32(float64(sum) / math.Sqrt(float64(norm1)*float64(norm2)))
}

// SquareDistanceBytes returns the squared Euclidean distance between two
// equal-length signed-byte vectors. Mirrors
// DefaultVectorUtilSupport.squareDistance(byte[], byte[]).
func (s *VectorUtilSupport) SquareDistanceBytes(a, b []byte) int {
	squareSum := 0
	for i := 0; i < len(a); i++ {
		diff := int(int8(a[i])) - int(int8(b[i]))
		squareSum += diff * diff
	}
	return squareSum
}

// Int4SquareDistance mirrors DefaultVectorUtilSupport.int4SquareDistance,
// which delegates to the signed-byte square-distance helper.
func (s *VectorUtilSupport) Int4SquareDistance(a, b []byte) int {
	return s.SquareDistanceBytes(a, b)
}

// Int4SquareDistanceSinglePacked computes the int4 squared distance between
// an unpacked vector and a singly-packed vector. Mirrors
// DefaultVectorUtilSupport.int4SquareDistanceSinglePacked.
func (s *VectorUtilSupport) Int4SquareDistanceSinglePacked(unpacked, packed []byte) int {
	total := 0
	for i := 0; i < len(packed); i++ {
		packedByte := packed[i]
		unpacked1 := int8(unpacked[i])
		unpacked2 := int8(unpacked[i+len(packed)])

		diff1 := int(packedByte&0x0F) - int(unpacked2)
		diff2 := int(packedByte>>4) - int(unpacked1)

		total += diff1*diff1 + diff2*diff2
	}
	return total
}

// Int4SquareDistanceBothPacked computes the int4 squared distance between two
// doubly-packed vectors. Mirrors
// DefaultVectorUtilSupport.int4SquareDistanceBothPacked.
func (s *VectorUtilSupport) Int4SquareDistanceBothPacked(a, b []byte) int {
	total := 0
	for i := 0; i < len(a); i++ {
		aByte := a[i]
		bByte := b[i]

		diff1 := int(aByte&0x0F) - int(bByte&0x0F)
		diff2 := int(aByte>>4) - int(bByte>>4)

		total += diff1*diff1 + diff2*diff2
	}
	return total
}

// Uint8SquareDistance returns the squared Euclidean distance between two
// equal-length unsigned-byte vectors. Mirrors
// DefaultVectorUtilSupport.uint8SquareDistance.
func (s *VectorUtilSupport) Uint8SquareDistance(a, b []byte) int {
	squareSum := 0
	for i := 0; i < len(a); i++ {
		diff := int(a[i]) - int(b[i])
		squareSum += diff * diff
	}
	return squareSum
}

// FindNextGEQ returns the index of the first element in buffer[from:to] that
// is greater than or equal to target, or to when none exists. Mirrors
// DefaultVectorUtilSupport.findNextGEQ.
func (s *VectorUtilSupport) FindNextGEQ(buffer []int32, target int32, from, to int) int {
	for i := from; i < to; i++ {
		if buffer[i] >= target {
			return i
		}
	}
	return to
}

// Int4BitDotProduct mirrors DefaultVectorUtilSupport.int4BitDotProduct: it
// delegates to the package-level int4BitDotProductImpl helper.
func (s *VectorUtilSupport) Int4BitDotProduct(int4Quantized, binaryQuantized []byte) int64 {
	return int4BitDotProductImpl(int4Quantized, binaryQuantized)
}

// int4BitDotProductImpl mirrors the public Java helper. q.length must equal
// d.length * 4; the caller is responsible for that invariant, matching the
// Java assert.
func int4BitDotProductImpl(q, d []byte) int64 {
	return int4BitDotProductStripe(q, d, 0, len(d))
}

// Int4DibitDotProduct mirrors DefaultVectorUtilSupport.int4DibitDotProduct.
func (s *VectorUtilSupport) Int4DibitDotProduct(int4Quantized, dibitQuantized []byte) int64 {
	return int4DibitDotProductImpl(int4Quantized, dibitQuantized)
}

// int4DibitDotProductImpl mirrors the public Java helper. q.length must equal
// d.length * 2; the dibit vector has two stripes (lower bits first, then
// upper bits), so scoring is two passes of the int4-bit dot product with the
// upper-stripe result shifted left by one.
func int4DibitDotProductImpl(q, d []byte) int64 {
	stripeSize := len(d) / 2
	ret0 := int4BitDotProductStripe(q, d, 0, stripeSize)
	ret1 := int4BitDotProductStripe(q, d, stripeSize, stripeSize)
	return ret0 + (ret1 << 1)
}

// int4BitDotProductStripe mirrors the private Java helper
// int4BitDotProductImpl(byte[], byte[], int, int). It computes the int4-bit
// dot product against a single stripe of the document vector.
func int4BitDotProductStripe(q, d []byte, dOffset, stripeSize int) int64 {
	var ret int64
	for i := 0; i < 4; i++ {
		r := 0
		var subRet int64
		upperBound := stripeSize &^ 3
		for ; r < upperBound; r += 4 {
			qv := binary.LittleEndian.Uint32(q[i*stripeSize+r:])
			dv := binary.LittleEndian.Uint32(d[dOffset+r:])
			subRet += int64(bits.OnesCount32(qv & dv))
		}
		for ; r < stripeSize; r++ {
			subRet += int64(bits.OnesCount8(q[i*stripeSize+r] & d[dOffset+r]))
		}
		ret += subRet << i
	}
	return ret
}

// MinMaxScalarQuantize quantizes vector into dest using the provided
// quantization parameters and returns the corrective offset. Mirrors
// DefaultVectorUtilSupport.minMaxScalarQuantize.
func (s *VectorUtilSupport) MinMaxScalarQuantize(
	vector []float32, dest []byte, scale, alpha, minQuantile, maxQuantile float32,
) float32 {
	q := newScalarQuantizer(alpha, scale, minQuantile, maxQuantile)
	return q.quantize(vector, dest, 0)
}

// RecalculateScalarQuantizationOffset recomputes the corrective offset for an
// already-quantized vector under new quantization parameters. Mirrors
// DefaultVectorUtilSupport.recalculateScalarQuantizationOffset.
func (s *VectorUtilSupport) RecalculateScalarQuantizationOffset(
	vector []byte,
	oldAlpha, oldMinQuantile,
	scale, alpha, minQuantile, maxQuantile float32,
) float32 {
	q := newScalarQuantizer(alpha, scale, minQuantile, maxQuantile)
	return q.recalculateOffset(vector, 0, oldAlpha, oldMinQuantile)
}

// scalarQuantizer mirrors the package-private inner class
// DefaultVectorUtilSupport.ScalarQuantizer. Fields are kept private; instances
// are constructed via [[newScalarQuantizer]].
type scalarQuantizer struct {
	alpha       float32
	scale       float32
	minQuantile float32
	maxQuantile float32
}

func newScalarQuantizer(alpha, scale, minQuantile, maxQuantile float32) scalarQuantizer {
	return scalarQuantizer{
		alpha:       alpha,
		scale:       scale,
		minQuantile: minQuantile,
		maxQuantile: maxQuantile,
	}
}

func (q scalarQuantizer) quantize(vector []float32, dest []byte, start int) float32 {
	var correction float32
	for i := start; i < len(vector); i++ {
		correction += q.quantizeFloat(vector[i], dest, i)
	}
	return correction
}

func (q scalarQuantizer) recalculateOffset(vector []byte, start int, oldAlpha, oldMinQuantile float32) float32 {
	var correction float32
	for i := start; i < len(vector); i++ {
		// Undo the old quantization (Java reads vector[i] as unsigned).
		v := (oldAlpha * float32(vector[i])) + oldMinQuantile
		correction += q.quantizeFloat(v, nil, 0)
	}
	return correction
}

// quantizeFloat mirrors the private Java helper. dest may be nil, in which
// case only the corrective term is returned; matching upstream's null check.
func (q scalarQuantizer) quantizeFloat(v float32, dest []byte, destIndex int) float32 {
	// Clamp v into [minQuantile, maxQuantile] before scaling.
	dx := v - q.minQuantile
	clamped := v
	if clamped < q.minQuantile {
		clamped = q.minQuantile
	}
	if clamped > q.maxQuantile {
		clamped = q.maxQuantile
	}
	dxc := clamped - q.minQuantile
	// scale = 127 / (maxQuantile - minQuantile); roundedDxs is the quantized
	// byte value before the cast.
	roundedDxs := int32(roundHalfAwayFromZero(q.scale * dxc))
	dxq := float32(roundedDxs) * q.alpha
	if dest != nil {
		dest[destIndex] = byte(int8(roundedDxs))
	}
	return q.minQuantile*(v-q.minQuantile/2.0) + (dx-dxq)*dxq
}

// roundHalfAwayFromZero mirrors java.lang.Math.round(float), which rounds to
// the nearest integer with ties broken toward positive infinity.
func roundHalfAwayFromZero(v float32) int32 {
	return int32(math.Floor(float64(v) + 0.5))
}

// FilterByScore mirrors DefaultVectorUtilSupport.filterByScore: it compacts
// docBuffer/scoreBuffer in place keeping only entries with
// score >= minScoreInclusive, and returns the new size.
func (s *VectorUtilSupport) FilterByScore(
	docBuffer []int32, scoreBuffer []float64, minScoreInclusive float64, upTo int,
) int {
	newSize := 0
	for i := 0; i < upTo; i++ {
		doc := docBuffer[i]
		score := scoreBuffer[i]
		docBuffer[newSize] = doc
		scoreBuffer[newSize] = score
		if score >= minScoreInclusive {
			newSize++
		}
	}
	return newSize
}
