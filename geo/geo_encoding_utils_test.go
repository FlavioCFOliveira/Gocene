// Tests for GeoEncodingUtils, mirroring TestGeoEncodingUtils.java
// (Lucene 10.4.0):
//
//   - testEncodeEdgeCases: MIN/MAX lat/lon Incl encode to Int32 min/max.
//   - testLatitudeQuantization: round-trip + neighbour quantisation.
//   - testLongitudeQuantization: same as above for longitude.
//
// The Java tests are randomised; the Go port reproduces them with a
// deterministic-seeded math/rand stream so failures are reproducible.

package geo

import (
	"math"
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

func TestGeoEncodingUtils_EncodeEdgeCases(t *testing.T) {
	t.Parallel()
	if got := EncodeLatitude(MinLatIncl); got != math.MinInt32 {
		t.Errorf("EncodeLatitude(MinLatIncl) = %d; want MinInt32", got)
	}
	if got := EncodeLatitudeCeil(MinLatIncl); got != math.MinInt32 {
		t.Errorf("EncodeLatitudeCeil(MinLatIncl) = %d; want MinInt32", got)
	}
	if got := EncodeLatitude(MaxLatIncl); got != math.MaxInt32 {
		t.Errorf("EncodeLatitude(MaxLatIncl) = %d; want MaxInt32", got)
	}
	if got := EncodeLatitudeCeil(MaxLatIncl); got != math.MaxInt32 {
		t.Errorf("EncodeLatitudeCeil(MaxLatIncl) = %d; want MaxInt32", got)
	}
	if got := EncodeLongitude(MinLonIncl); got != math.MinInt32 {
		t.Errorf("EncodeLongitude(MinLonIncl) = %d; want MinInt32", got)
	}
	if got := EncodeLongitudeCeil(MinLonIncl); got != math.MinInt32 {
		t.Errorf("EncodeLongitudeCeil(MinLonIncl) = %d; want MinInt32", got)
	}
	if got := EncodeLongitude(MaxLonIncl); got != math.MaxInt32 {
		t.Errorf("EncodeLongitude(MaxLonIncl) = %d; want MaxInt32", got)
	}
	if got := EncodeLongitudeCeil(MaxLonIncl); got != math.MaxInt32 {
		t.Errorf("EncodeLongitudeCeil(MaxLonIncl) = %d; want MaxInt32", got)
	}
}

func TestGeoEncodingUtils_LatitudeQuantization(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewSource(0xCAFE_F00D)) //nolint:gosec // deterministic test fixture
	latDecodeStep := 180.0 / float64(uint64(1)<<32)

	for i := 0; i < 1000; i++ {
		encoded := int32(rng.Int31())
		// rand.Int31 only generates non-negative values, so flip
		// half the time to span the full int32 range.
		if rng.Intn(2) == 0 {
			encoded = ^encoded
		}
		expected := MinLatIncl + float64(int64(encoded)-int64(math.MinInt32))*latDecodeStep
		decoded := DecodeLatitude(encoded)
		if decoded != expected {
			t.Fatalf("DecodeLatitude(%d) = %v; want %v", encoded, decoded, expected)
		}
		// round-trip
		if got := EncodeLatitude(decoded); got != encoded {
			t.Fatalf("round-trip: EncodeLatitude(DecodeLatitude(%d)) = %d", encoded, got)
		}
		if got := EncodeLatitudeCeil(decoded); got != encoded {
			t.Fatalf("ceil round-trip: EncodeLatitudeCeil(DecodeLatitude(%d)) = %d", encoded, got)
		}
		if encoded == math.MaxInt32 {
			continue
		}
		max := expected + latDecodeStep
		if DecodeLatitude(encoded+1) != max {
			t.Fatalf("DecodeLatitude(%d+1) = %v; want %v", encoded, DecodeLatitude(encoded+1), max)
		}
		// Boundary nudges.
		minEdge := math.Nextafter(expected, math.Inf(1))
		maxEdge := math.Nextafter(max, math.Inf(-1))
		if got := EncodeLatitude(minEdge); got != encoded {
			t.Fatalf("EncodeLatitude(minEdge) = %d; want %d", got, encoded)
		}
		if got := EncodeLatitudeCeil(minEdge); got != encoded+1 {
			t.Fatalf("EncodeLatitudeCeil(minEdge) = %d; want %d", got, encoded+1)
		}
		if got := EncodeLatitude(maxEdge); got != encoded {
			t.Fatalf("EncodeLatitude(maxEdge) = %d; want %d", got, encoded)
		}
		if got := EncodeLatitudeCeil(maxEdge); got != encoded+1 {
			t.Fatalf("EncodeLatitudeCeil(maxEdge) = %d; want %d", got, encoded+1)
		}
	}
}

func TestGeoEncodingUtils_LongitudeQuantization(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewSource(0xBEEF_DEAD)) //nolint:gosec // deterministic test fixture
	lonDecodeStep := 360.0 / float64(uint64(1)<<32)

	for i := 0; i < 1000; i++ {
		encoded := int32(rng.Int31())
		if rng.Intn(2) == 0 {
			encoded = ^encoded
		}
		expected := MinLonIncl + float64(int64(encoded)-int64(math.MinInt32))*lonDecodeStep
		if got := DecodeLongitude(encoded); got != expected {
			t.Fatalf("DecodeLongitude(%d) = %v; want %v", encoded, got, expected)
		}
		if got := EncodeLongitude(expected); got != encoded {
			t.Fatalf("round-trip EncodeLongitude = %d; want %d", got, encoded)
		}
		if got := EncodeLongitudeCeil(expected); got != encoded {
			t.Fatalf("round-trip EncodeLongitudeCeil = %d; want %d", got, encoded)
		}
	}
}

func TestGeoEncodingUtils_DecodeBytesMatchesIntDecode(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewSource(0xABCD)) //nolint:gosec
	buf := make([]byte, 4)
	for i := 0; i < 200; i++ {
		v := int32(rng.Int31())
		if rng.Intn(2) == 0 {
			v = ^v
		}
		util.IntToSortableBytes(v, buf, 0)
		if got, want := DecodeLatitudeBytes(buf, 0), DecodeLatitude(v); got != want {
			t.Errorf("DecodeLatitudeBytes(%d) = %v; want %v", v, got, want)
		}
		if got, want := DecodeLongitudeBytes(buf, 0), DecodeLongitude(v); got != want {
			t.Errorf("DecodeLongitudeBytes(%d) = %v; want %v", v, got, want)
		}
	}
}

func TestGeoEncodingUtils_MinMaxLonEncodedConstants(t *testing.T) {
	t.Parallel()
	if MinLonEncoded != math.MinInt32 {
		t.Errorf("MinLonEncoded = %d; want MinInt32", MinLonEncoded)
	}
	if MaxLonEncoded != math.MaxInt32 {
		t.Errorf("MaxLonEncoded = %d; want MaxInt32", MaxLonEncoded)
	}
}

func TestGeoEncodingUtils_CreateDistancePredicate_InsideAndOutsidePoints(t *testing.T) {
	t.Parallel()
	// 10km radius around (0, 0).
	pred := CreateDistancePredicate(0, 0, 10_000)
	// Centre point should always pass.
	if !pred.Test(EncodeLatitude(0), EncodeLongitude(0)) {
		t.Error("centre point should be inside disk")
	}
	// 1000 km north should not.
	farLat := EncodeLatitude(10) // ~1110 km north
	farLon := EncodeLongitude(0)
	if pred.Test(farLat, farLon) {
		t.Error("point 10 degrees north should be outside 10km disk")
	}
}

func TestGeoEncodingUtils_CreateComponentPredicate_InsideAndOutsidePoints(t *testing.T) {
	t.Parallel()
	rect := MustNewRectangle(-1, 1, -1, 1)
	pred := CreateComponentPredicate(rect.toComponent2D())
	if !pred.Test(EncodeLatitude(0), EncodeLongitude(0)) {
		t.Error("centre point should be inside box")
	}
	if pred.Test(EncodeLatitude(5), EncodeLongitude(5)) {
		t.Error("(5,5) should be outside (-1..1, -1..1)")
	}
}
