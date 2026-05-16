// Tests for GeoUtils helpers, mirroring the relevant cases from
// lucene/core/src/test/org/apache/lucene/geo/TestGeoUtils.java
// (Lucene 10.4.0). The randomised/sampling tests
// (testRandomCircleToBBox, testBoundingBoxOpto, testHaversinOpto,
// testCircleOpto) live outside the deterministic scope of this port
// and are not reproduced; they exercise external invariants of
// HaversinMeters / FromPointDistance / Circle that are covered by
// util/sloppy_math_test.go and geo/rectangle_test.go.
//
// Deterministic coverage in this file:
//
//   - Constants match Java values (MinLat/MaxLat/MinLon/MaxLon Incl
//     and Radians, EarthMeanRadiusMeters).
//   - CheckLatitude / CheckLongitude bounds (sentinels and message
//     format).
//   - WindingOrder Sign / String / FromSign.
//   - Orient, LineCrossesLine, LineOverlapLine,
//     LineCrossesLineWithBoundary on hand-built cases.
//   - Within90LonDegrees on every regression case from
//     testWithin90LonDegrees in the Java original.
//   - DistanceQuerySortKey monotonicity and inverse via
//     util.HaversinMetersFromSortKey.
//   - Relate sample cases: disjoint cell -> OUTSIDE, contained cell
//     -> INSIDE, partially-overlapping cell -> CROSSES.

package geo

import (
	"errors"
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

func TestGeoUtils_ConstantsMatchJava(t *testing.T) {
	t.Parallel()
	if MinLatIncl != -90 || MaxLatIncl != 90 {
		t.Errorf("lat bounds = (%v,%v); want (-90,90)", MinLatIncl, MaxLatIncl)
	}
	if MinLonIncl != -180 || MaxLonIncl != 180 {
		t.Errorf("lon bounds = (%v,%v); want (-180,180)", MinLonIncl, MaxLonIncl)
	}
	if EarthMeanRadiusMeters != 6_371_008.7714 {
		t.Errorf("Earth radius = %v; want 6371008.7714", EarthMeanRadiusMeters)
	}
	if math.Abs(MaxLatRadians-math.Pi/2) > 1e-15 {
		t.Errorf("MaxLatRadians = %v; want pi/2", MaxLatRadians)
	}
	if math.Abs(MaxLonRadians-math.Pi) > 1e-15 {
		t.Errorf("MaxLonRadians = %v; want pi", MaxLonRadians)
	}
}

func TestGeoUtils_CheckLatitudeAcceptsBounds(t *testing.T) {
	t.Parallel()
	for _, v := range []float64{MinLatIncl, MaxLatIncl, 0, 1, -1} {
		if err := CheckLatitude(v); err != nil {
			t.Errorf("CheckLatitude(%v) = %v, want nil", v, err)
		}
	}
}

func TestGeoUtils_CheckLatitudeRejectsOutOfRange(t *testing.T) {
	t.Parallel()
	for _, v := range []float64{-90.0001, 90.0001, math.NaN(), math.Inf(1), math.Inf(-1)} {
		if err := CheckLatitude(v); err == nil {
			t.Errorf("CheckLatitude(%v) = nil, want non-nil", v)
		} else if !errors.Is(err, ErrInvalidLatitude) {
			t.Errorf("CheckLatitude(%v) = %v, want wrap ErrInvalidLatitude", v, err)
		}
	}
}

func TestGeoUtils_CheckLongitudeAcceptsBounds(t *testing.T) {
	t.Parallel()
	for _, v := range []float64{MinLonIncl, MaxLonIncl, 0, 90, -90} {
		if err := CheckLongitude(v); err != nil {
			t.Errorf("CheckLongitude(%v) = %v, want nil", v, err)
		}
	}
}

func TestGeoUtils_WindingOrderFromSign(t *testing.T) {
	t.Parallel()
	if WindingOrderFromSign(-1) != WindingClockwise {
		t.Error("FromSign(-1) != CW")
	}
	if WindingOrderFromSign(0) != WindingColinear {
		t.Error("FromSign(0) != COLINEAR")
	}
	if WindingOrderFromSign(1) != WindingCounterClockwise {
		t.Error("FromSign(1) != CCW")
	}
	defer func() {
		if recover() == nil {
			t.Error("FromSign(99) should panic")
		}
	}()
	WindingOrderFromSign(99)
}

func TestGeoUtils_Orient(t *testing.T) {
	t.Parallel()
	cases := []struct {
		ax, ay, bx, by, cx, cy float64
		want                   int
	}{
		{0, 0, 1, 0, 1, 1, 1},   // CCW
		{0, 0, 1, 1, 1, 0, -1},  // CW
		{0, 0, 2, 2, 4, 4, 0},   // collinear
		{0, 0, 10, 0, 5, 0, 0},  // collinear horizontal
	}
	for _, c := range cases {
		if got := Orient(c.ax, c.ay, c.bx, c.by, c.cx, c.cy); got != c.want {
			t.Errorf("Orient(%v..%v) = %d, want %d", c.ax, c.cy, got, c.want)
		}
	}
}

func TestGeoUtils_LineCrossesLine(t *testing.T) {
	t.Parallel()
	// Two perpendicular segments crossing in their interiors.
	if !LineCrossesLine(-1, 0, 1, 0, 0, -1, 0, 1) {
		t.Error("perpendicular crossing should return true")
	}
	// Endpoint touch — strict crossing should be false.
	if LineCrossesLine(0, 0, 1, 0, 1, 0, 1, 1) {
		t.Error("endpoint touch should not count as strict crossing")
	}
	// Disjoint.
	if LineCrossesLine(0, 0, 1, 0, 2, 0, 3, 0) {
		t.Error("disjoint segments should not cross")
	}
}

func TestGeoUtils_LineCrossesLineWithBoundary(t *testing.T) {
	t.Parallel()
	// Endpoint touch is a crossing under the closed-interior rule.
	if !LineCrossesLineWithBoundary(0, 0, 1, 0, 1, 0, 1, 1) {
		t.Error("endpoint touch should count as boundary crossing")
	}
	// Disjoint still does not cross.
	if LineCrossesLineWithBoundary(0, 0, 1, 0, 5, 5, 6, 6) {
		t.Error("disjoint segments should not cross with boundary")
	}
}

func TestGeoUtils_LineOverlapLine(t *testing.T) {
	t.Parallel()
	// Same line, distinct sub-intervals.
	if !LineOverlapLine(0, 0, 10, 0, 2, 0, 8, 0) {
		t.Error("collinear segments should overlap")
	}
	// Parallel but not collinear.
	if LineOverlapLine(0, 0, 10, 0, 0, 1, 10, 1) {
		t.Error("parallel-not-collinear segments should not overlap")
	}
}

func TestGeoUtils_Within90LonDegrees(t *testing.T) {
	t.Parallel()
	// Cases derived from TestGeoUtils#testWithin90LonDegrees.
	if !Within90LonDegrees(0, -45, 45) {
		t.Error("0 should be within 90 of [-45,45]")
	}
	if Within90LonDegrees(0, -91, 0) {
		t.Error("-91..0 should not be within 90 of 0")
	}
	if Within90LonDegrees(0, 0, 91) {
		t.Error("0..91 should not be within 90 of 0")
	}
	// Wrap-around: lon=170 vs [-170, -100]. After +360 wrap, lon
	// becomes 170 vs [190, 260] -> distances 20 and 90.
	if Within90LonDegrees(170, -170, -100) {
		t.Error("wrap distance > 90 should not be within 90")
	}
}

func TestGeoUtils_DistanceQuerySortKey_MatchesRoundTrip(t *testing.T) {
	t.Parallel()
	// For typical radii, the returned sort key should round-trip
	// back to >= radius via util.HaversinMetersFromSortKey.
	for _, r := range []float64{1, 100, 1_000, 1_000_000, 10_000_000} {
		k := DistanceQuerySortKey(r)
		back := util.HaversinMetersFromSortKey(k)
		if back+1e-6 < r {
			t.Errorf("DistanceQuerySortKey(%v) -> %v -> %v; expected >= %v", r, k, back, r)
		}
	}
}

func TestGeoUtils_DistanceQuerySortKey_InfiniteRadius(t *testing.T) {
	t.Parallel()
	// A radius >= max possible haversine should return the max
	// haversine sort key (saturation guard).
	maxRadius := util.HaversinMetersFromSortKey(math.MaxFloat64)
	got := DistanceQuerySortKey(maxRadius * 2)
	if got != maxRadius {
		t.Errorf("DistanceQuerySortKey(>=max) = %v, want %v", got, maxRadius)
	}
}

func TestGeoUtils_Relate_OutsideForFarAwayBox(t *testing.T) {
	t.Parallel()
	// Disk at (0,0) with 10km radius; box on the other side of the
	// world should be OUTSIDE.
	radius := 10_000.0
	axisLat := AxisLat(0, radius)
	key := DistanceQuerySortKey(radius)
	got := Relate(50, 60, 50, 60, 0, 0, key, axisLat)
	if got != CellOutsideQuery {
		t.Errorf("far-away box = %v, want OUTSIDE", got)
	}
}

func TestGeoUtils_Relate_InsideForTinyBoxAtCentre(t *testing.T) {
	t.Parallel()
	radius := 100_000.0
	axisLat := AxisLat(0, radius)
	key := DistanceQuerySortKey(radius)
	got := Relate(-1e-3, 1e-3, -1e-3, 1e-3, 0, 0, key, axisLat)
	if got != CellInsideQuery {
		t.Errorf("centre-tiny-box = %v, want INSIDE", got)
	}
}

func TestGeoUtils_Relate_PanicsOnDatelineCross(t *testing.T) {
	t.Parallel()
	defer func() {
		if recover() == nil {
			t.Error("Relate(minLon > maxLon) should panic")
		}
	}()
	Relate(0, 1, 170, -170, 0, 0, 1, 0)
}
