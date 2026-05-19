// Purpose: Unit tests for the scalar VectorUtilSupport methods (GOC-3423).
// Lucene reference: org.apache.lucene.internal.vectorization.DefaultVectorUtilSupport
//   has no dedicated upstream test peer (it is exercised indirectly through
//   VectorUtil's tests in TestVectorUtil). These tests pin the Go port's
//   contract directly against small, hand-computed expected values plus a
//   handful of structural invariants.

package vectorization

import (
	"math"
	"testing"
)

// almostEqualF32 returns true when |a - b| is below the provided tolerance.
// We compare in float64 to avoid the very rounding the production code aims
// to preserve.
func almostEqualF32(a, b, tol float32) bool {
	if math.IsNaN(float64(a)) || math.IsNaN(float64(b)) {
		return false
	}
	d := float64(a) - float64(b)
	if d < 0 {
		d = -d
	}
	return d <= float64(tol)
}

func TestVectorUtilSupport_DotProductFloats(t *testing.T) {
	t.Parallel()
	s := &VectorUtilSupport{}

	// Short (<= 32 elements) takes the tail-only path.
	a := []float32{1, 2, 3, 4}
	b := []float32{5, 6, 7, 8}
	if got, want := s.DotProductFloats(a, b), float32(70); got != want {
		t.Fatalf("short DotProductFloats = %v, want %v", got, want)
	}

	// Long (> 32 elements) takes the unrolled + tail combination.
	const n = 40
	la := make([]float32, n)
	lb := make([]float32, n)
	var want float64
	for i := 0; i < n; i++ {
		la[i] = float32(i + 1)
		lb[i] = float32((i + 1) * 2)
		want += float64(la[i]) * float64(lb[i])
	}
	if got := s.DotProductFloats(la, lb); !almostEqualF32(got, float32(want), 1e-3) {
		t.Fatalf("long DotProductFloats = %v, want %v", got, want)
	}
}

func TestVectorUtilSupport_CosineFloats(t *testing.T) {
	t.Parallel()
	s := &VectorUtilSupport{}

	a := []float32{1, 0, 0}
	b := []float32{1, 0, 0}
	if got := s.CosineFloats(a, b); !almostEqualF32(got, 1, 1e-6) {
		t.Fatalf("identical CosineFloats = %v, want 1", got)
	}

	a = []float32{1, 0}
	b = []float32{0, 1}
	if got := s.CosineFloats(a, b); !almostEqualF32(got, 0, 1e-6) {
		t.Fatalf("orthogonal CosineFloats = %v, want 0", got)
	}

	// Long path: cos(v, v) == 1.
	const n = 50
	la := make([]float32, n)
	for i := 0; i < n; i++ {
		la[i] = float32(i + 1)
	}
	if got := s.CosineFloats(la, la); !almostEqualF32(got, 1, 1e-4) {
		t.Fatalf("long CosineFloats(v,v) = %v, want 1", got)
	}
}

func TestVectorUtilSupport_SquareDistanceFloats(t *testing.T) {
	t.Parallel()
	s := &VectorUtilSupport{}

	a := []float32{1, 2, 3}
	b := []float32{4, 6, 8}
	// (3*3) + (4*4) + (5*5) = 9 + 16 + 25 = 50
	if got, want := s.SquareDistanceFloats(a, b), float32(50); got != want {
		t.Fatalf("SquareDistanceFloats = %v, want %v", got, want)
	}

	// Long-path equivalence with a tail.
	const n = 35
	la := make([]float32, n)
	lb := make([]float32, n)
	var want float64
	for i := 0; i < n; i++ {
		la[i] = float32(i)
		lb[i] = float32(i + 1)
		d := float64(la[i]) - float64(lb[i])
		want += d * d
	}
	if got := s.SquareDistanceFloats(la, lb); !almostEqualF32(got, float32(want), 1e-3) {
		t.Fatalf("long SquareDistanceFloats = %v, want %v", got, want)
	}
}

func TestVectorUtilSupport_ByteSignedDotProduct(t *testing.T) {
	t.Parallel()
	s := &VectorUtilSupport{}

	// Mix of positive and negative bytes to lock in signed semantics.
	a := []byte{0x01, 0xFF, 0x7F} // 1, -1, 127
	b := []byte{0x02, 0xFE, 0x80} // 2, -2, -128
	// 1*2 + (-1)*(-2) + 127*(-128) = 2 + 2 - 16256 = -16252
	if got, want := s.DotProductBytes(a, b), -16252; got != want {
		t.Fatalf("DotProductBytes = %d, want %d", got, want)
	}
}

func TestVectorUtilSupport_Uint8DotProduct(t *testing.T) {
	t.Parallel()
	s := &VectorUtilSupport{}

	a := []byte{0xFF, 0x80}
	b := []byte{0x02, 0x03}
	// 255*2 + 128*3 = 510 + 384 = 894
	if got, want := s.Uint8DotProduct(a, b), 894; got != want {
		t.Fatalf("Uint8DotProduct = %d, want %d", got, want)
	}
}

func TestVectorUtilSupport_Int4DotProductDelegations(t *testing.T) {
	t.Parallel()
	s := &VectorUtilSupport{}

	a := []byte{0x0A, 0xFB}
	b := []byte{0x03, 0xFD}
	if s.Int4DotProduct(a, b) != s.DotProductBytes(a, b) {
		t.Fatal("Int4DotProduct must delegate to DotProductBytes")
	}
	if s.Int4SquareDistance(a, b) != s.SquareDistanceBytes(a, b) {
		t.Fatal("Int4SquareDistance must delegate to SquareDistanceBytes")
	}
}

func TestVectorUtilSupport_Int4PackedDotProducts(t *testing.T) {
	t.Parallel()
	s := &VectorUtilSupport{}

	// Singly packed: packed has the low nibble paired with the second half of
	// the unpacked vector and the high nibble paired with the first half.
	unpacked := []byte{0x01, 0x02, 0x03, 0x04}
	packed := []byte{0x21, 0x43}
	// i=0: low(1)*unpacked2(3) + high(2)*unpacked1(1) = 3 + 2 = 5
	// i=1: low(3)*unpacked2(4) + high(4)*unpacked1(2) = 12 + 8 = 20
	if got, want := s.Int4DotProductSinglePacked(unpacked, packed), 25; got != want {
		t.Fatalf("Int4DotProductSinglePacked = %d, want %d", got, want)
	}

	// Both packed: per byte sum of low*low + high*high.
	a := []byte{0x21, 0x43}
	b := []byte{0x12, 0x34}
	// byte0: 1*2 + 2*1 = 4
	// byte1: 3*4 + 4*3 = 24
	if got, want := s.Int4DotProductBothPacked(a, b), 28; got != want {
		t.Fatalf("Int4DotProductBothPacked = %d, want %d", got, want)
	}
}

func TestVectorUtilSupport_CosineBytes(t *testing.T) {
	t.Parallel()
	s := &VectorUtilSupport{}

	a := []byte{0x01, 0xFF} // 1, -1
	b := []byte{0xFF, 0x01} // -1, 1
	// sum = -1 + -1 = -2; norms = 2 each; cos = -2 / sqrt(4) = -1.
	if got := s.CosineBytes(a, b); !almostEqualF32(got, -1, 1e-6) {
		t.Fatalf("CosineBytes = %v, want -1", got)
	}
}

func TestVectorUtilSupport_SquareDistanceBytes(t *testing.T) {
	t.Parallel()
	s := &VectorUtilSupport{}

	a := []byte{0x01, 0x7F}
	b := []byte{0xFF, 0x80}
	// diff = 1-(-1)=2, 127-(-128)=255; squareSum = 4 + 65025 = 65029
	if got, want := s.SquareDistanceBytes(a, b), 65029; got != want {
		t.Fatalf("SquareDistanceBytes = %d, want %d", got, want)
	}
}

func TestVectorUtilSupport_PackedSquareDistances(t *testing.T) {
	t.Parallel()
	s := &VectorUtilSupport{}

	unpacked := []byte{0x01, 0x02, 0x03, 0x04}
	packed := []byte{0x21, 0x43}
	// i=0: diff1 = low(1)-unpacked2(3) = -2; diff2 = high(2)-unpacked1(1) = 1; 4+1
	// i=1: diff1 = low(3)-unpacked2(4) = -1; diff2 = high(4)-unpacked1(2) = 2; 1+4
	if got, want := s.Int4SquareDistanceSinglePacked(unpacked, packed), 10; got != want {
		t.Fatalf("Int4SquareDistanceSinglePacked = %d, want %d", got, want)
	}

	a := []byte{0x21, 0x43}
	b := []byte{0x12, 0x34}
	// byte0: diff1=1-2=-1, diff2=2-1=1 -> 2
	// byte1: diff1=3-4=-1, diff2=4-3=1 -> 2
	if got, want := s.Int4SquareDistanceBothPacked(a, b), 4; got != want {
		t.Fatalf("Int4SquareDistanceBothPacked = %d, want %d", got, want)
	}
}

func TestVectorUtilSupport_Uint8SquareDistance(t *testing.T) {
	t.Parallel()
	s := &VectorUtilSupport{}

	a := []byte{0xFF, 0x10}
	b := []byte{0x00, 0x20}
	// diff = 255, -16; squareSum = 65025 + 256 = 65281
	if got, want := s.Uint8SquareDistance(a, b), 65281; got != want {
		t.Fatalf("Uint8SquareDistance = %d, want %d", got, want)
	}
}

func TestVectorUtilSupport_FindNextGEQ(t *testing.T) {
	t.Parallel()
	s := &VectorUtilSupport{}

	buf := []int32{1, 3, 5, 7, 9}
	if got := s.FindNextGEQ(buf, 6, 0, len(buf)); got != 3 {
		t.Fatalf("FindNextGEQ(>=6) = %d, want 3", got)
	}
	if got := s.FindNextGEQ(buf, 10, 0, len(buf)); got != len(buf) {
		t.Fatalf("FindNextGEQ(>=10) = %d, want %d", got, len(buf))
	}
	if got := s.FindNextGEQ(buf, 0, 2, 4); got != 2 {
		t.Fatalf("FindNextGEQ(range) = %d, want 2", got)
	}
}

func TestVectorUtilSupport_Int4BitDotProduct(t *testing.T) {
	t.Parallel()
	s := &VectorUtilSupport{}

	// d.length == 1, so each q stripe is a single byte.
	q := []byte{0xFF, 0xFF, 0xFF, 0xFF}
	d := []byte{0xFF}
	// Each of the 4 stripes contributes popcount(0xFF & 0xFF) = 8.
	// ret = 8<<0 + 8<<1 + 8<<2 + 8<<3 = 8 * (1+2+4+8) = 120.
	if got, want := s.Int4BitDotProduct(q, d), int64(120); got != want {
		t.Fatalf("Int4BitDotProduct = %d, want %d", got, want)
	}

	// Stripe wider than 4 bytes to exercise the binary.LittleEndian path.
	const stripe = 5
	q2 := make([]byte, stripe*4)
	d2 := make([]byte, stripe)
	for i := range q2 {
		q2[i] = 0xFF
	}
	for i := range d2 {
		d2[i] = 0xFF
	}
	// Each stripe contributes popcount = stripe*8 = 40.
	// ret = 40 * (1+2+4+8) = 600.
	if got, want := s.Int4BitDotProduct(q2, d2), int64(600); got != want {
		t.Fatalf("Int4BitDotProduct(wide) = %d, want %d", got, want)
	}
}

func TestVectorUtilSupport_Int4DibitDotProduct(t *testing.T) {
	t.Parallel()
	s := &VectorUtilSupport{}

	// d.length == 2 -> 2 stripes of size 1; q must be length 4.
	q := []byte{0xFF, 0xFF, 0xFF, 0xFF}
	d := []byte{0xFF, 0xFF}
	// Each stripe sub-call returns 8<<0 + 8<<1 + 8<<2 + 8<<3 = 120.
	// Total = ret0 + (ret1 << 1) = 120 + 240 = 360.
	if got, want := s.Int4DibitDotProduct(q, d), int64(360); got != want {
		t.Fatalf("Int4DibitDotProduct = %d, want %d", got, want)
	}
}

func TestVectorUtilSupport_MinMaxScalarQuantize(t *testing.T) {
	t.Parallel()
	s := &VectorUtilSupport{}

	// Quantize a vector whose values straddle the [minQuantile, maxQuantile]
	// range. We pin the byte output and require the correction to be finite.
	vector := []float32{0, 0.5, 1.0}
	dest := make([]byte, len(vector))
	const (
		minQ     = float32(0)
		maxQ     = float32(1)
		scaleVal = float32(127) // 127 / (1 - 0)
		alphaVal = float32(1.0 / 127.0)
	)
	correction := s.MinMaxScalarQuantize(vector, dest, scaleVal, alphaVal, minQ, maxQ)

	wantDest := []byte{0, 64, 127} // round(127 * 0.5) = 64
	for i := range dest {
		if dest[i] != wantDest[i] {
			t.Fatalf("dest[%d] = %d, want %d", i, dest[i], wantDest[i])
		}
	}
	if math.IsNaN(float64(correction)) || math.IsInf(float64(correction), 0) {
		t.Fatalf("correction must be finite, got %v", correction)
	}

	// Recalculate must produce a finite value for the dest we just generated.
	offset := s.RecalculateScalarQuantizationOffset(
		dest, alphaVal, minQ, scaleVal, alphaVal, minQ, maxQ,
	)
	if math.IsNaN(float64(offset)) || math.IsInf(float64(offset), 0) {
		t.Fatalf("recalculated offset must be finite, got %v", offset)
	}
}

func TestVectorUtilSupport_FilterByScore(t *testing.T) {
	t.Parallel()
	s := &VectorUtilSupport{}

	docs := []int32{10, 20, 30, 40, 50}
	scores := []float64{0.1, 0.9, 0.2, 0.8, 0.5}
	newSize := s.FilterByScore(docs, scores, 0.5, len(docs))
	if newSize != 3 {
		t.Fatalf("newSize = %d, want 3", newSize)
	}
	wantDocs := []int32{20, 40, 50}
	wantScores := []float64{0.9, 0.8, 0.5}
	for i := 0; i < newSize; i++ {
		if docs[i] != wantDocs[i] {
			t.Fatalf("docs[%d] = %d, want %d", i, docs[i], wantDocs[i])
		}
		if scores[i] != wantScores[i] {
			t.Fatalf("scores[%d] = %v, want %v", i, scores[i], wantScores[i])
		}
	}

	// upTo less than len(docs) bounds the scan as upstream does.
	docs2 := []int32{10, 20, 30}
	scores2 := []float64{0.9, 0.1, 0.8}
	if got := s.FilterByScore(docs2, scores2, 0.5, 2); got != 1 {
		t.Fatalf("upTo-bounded FilterByScore = %d, want 1", got)
	}
}

func TestVectorUtilSupport_RoundHalfAwayFromZero(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   float32
		want int32
	}{
		{0.5, 1},
		{1.5, 2},
		{-0.5, 0}, // Java Math.round breaks ties toward +inf.
		{-1.4, -1},
		{2.49, 2},
		{2.5, 3},
	}
	for _, c := range cases {
		if got := roundHalfAwayFromZero(c.in); got != c.want {
			t.Fatalf("roundHalfAwayFromZero(%v) = %d, want %d", c.in, got, c.want)
		}
	}
}
