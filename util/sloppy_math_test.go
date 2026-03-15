// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Source: lucene/core/src/test/org/apache/lucene/util/TestSloppyMath.java
// Purpose: Tests for SloppyMath approximations including haversine distance,
//          trigonometric functions (cos, sin, asin) with tolerance checking.

package util

import (
	"math"
	"testing"
)

// Accuracy constants matching the Java test
const (
	testCosDelta              = 1e-15
	testSinDelta              = 1e-12
	testAsinDelta             = 1e-7
	testHaversinDelta         = 38e-2
	testReasonableHaversinDelta = 1e-5
)

// TestCos_SpecialCases tests cos() special cases (NaN, infinity) and specific values
func TestCos_SpecialCases(t *testing.T) {
	// Test NaN and infinity return NaN
	if !math.IsNaN(Cos(math.NaN())) {
		t.Error("Expected Cos(NaN) to return NaN")
	}
	if !math.IsNaN(Cos(math.Inf(-1))) {
		t.Error("Expected Cos(-Inf) to return NaN")
	}
	if !math.IsNaN(Cos(math.Inf(1))) {
		t.Error("Expected Cos(+Inf) to return NaN")
	}

	// Test specific values against StrictMath.cos (using math.Cos)
	testCases := []struct {
		input    float64
		expected float64
	}{
		{1, math.Cos(1)},
		{0, math.Cos(0)},
		{math.Pi / 2, math.Cos(math.Pi / 2)},
		{-math.Pi / 2, math.Cos(-math.Pi / 2)},
		{math.Pi / 4, math.Cos(math.Pi / 4)},
		{-math.Pi / 4, math.Cos(-math.Pi / 4)},
		{math.Pi * 2 / 3, math.Cos(math.Pi * 2 / 3)},
		{-math.Pi * 2 / 3, math.Cos(-math.Pi * 2 / 3)},
		{math.Pi / 6, math.Cos(math.Pi / 6)},
		{-math.Pi / 6, math.Cos(-math.Pi / 6)},
	}

	for _, tc := range testCases {
		actual := Cos(tc.input)
		diff := math.Abs(tc.expected - actual)
		if diff > testCosDelta {
			t.Errorf("Cos(%v): expected %v, got %v (diff %v > %v)",
				tc.input, tc.expected, actual, diff, testCosDelta)
		}
	}
}

// TestCos_RandomValues tests cos() against random values
func TestCos_RandomValues(t *testing.T) {
	// Testing purely random values within the efficient range
	for i := 0; i < 10000; i++ {
		d := RandomFloat64() * SinCosMaxValueForIntModulo
		if RandomBool() {
			d = -d
		}

		expected := math.Cos(d)
		actual := Cos(d)
		diff := math.Abs(expected - actual)
		if diff > testCosDelta {
			t.Errorf("Cos(%v): expected %v, got %v (diff %v > %v)",
				d, expected, actual, diff, testCosDelta)
		}
	}
}

// TestSin_SpecialCases tests sin() special cases (NaN, infinity) and specific values
func TestSin_SpecialCases(t *testing.T) {
	// Test NaN and infinity return NaN
	if !math.IsNaN(Sin(math.NaN())) {
		t.Error("Expected Sin(NaN) to return NaN")
	}
	if !math.IsNaN(Sin(math.Inf(-1))) {
		t.Error("Expected Sin(-Inf) to return NaN")
	}
	if !math.IsNaN(Sin(math.Inf(1))) {
		t.Error("Expected Sin(+Inf) to return NaN")
	}

	// Test specific values against StrictMath.sin (using math.Sin)
	testCases := []struct {
		input    float64
		expected float64
	}{
		{1, math.Sin(1)},
		{0, math.Sin(0)},
		{math.Pi / 2, math.Sin(math.Pi / 2)},
		{-math.Pi / 2, math.Sin(-math.Pi / 2)},
		{math.Pi / 4, math.Sin(math.Pi / 4)},
		{-math.Pi / 4, math.Sin(-math.Pi / 4)},
		{math.Pi * 2 / 3, math.Sin(math.Pi * 2 / 3)},
		{-math.Pi * 2 / 3, math.Sin(-math.Pi * 2 / 3)},
		{math.Pi / 6, math.Sin(math.Pi / 6)},
		{-math.Pi / 6, math.Sin(-math.Pi / 6)},
	}

	for _, tc := range testCases {
		actual := Sin(tc.input)
		diff := math.Abs(tc.expected - actual)
		if diff > testSinDelta {
			t.Errorf("Sin(%v): expected %v, got %v (diff %v > %v)",
				tc.input, tc.expected, actual, diff, testSinDelta)
		}
	}
}

// TestSin_RandomValues tests sin() against random values
func TestSin_RandomValues(t *testing.T) {
	// Testing purely random values within the efficient range
	for i := 0; i < 10000; i++ {
		d := RandomFloat64() * SinCosMaxValueForIntModulo
		if RandomBool() {
			d = -d
		}

		expected := math.Sin(d)
		actual := Sin(d)
		diff := math.Abs(expected - actual)
		if diff > testSinDelta {
			t.Errorf("Sin(%v): expected %v, got %v (diff %v > %v)",
				d, expected, actual, diff, testSinDelta)
		}
	}
}

// TestAsin_SpecialCases tests asin() special cases and specific values
func TestAsin_SpecialCases(t *testing.T) {
	// Test NaN returns NaN
	if !math.IsNaN(Asin(math.NaN())) {
		t.Error("Expected Asin(NaN) to return NaN")
	}

	// Test values outside [-1, 1] return NaN
	if !math.IsNaN(Asin(2)) {
		t.Error("Expected Asin(2) to return NaN")
	}
	if !math.IsNaN(Asin(-2)) {
		t.Error("Expected Asin(-2) to return NaN")
	}

	// Test specific values
	testCases := []struct {
		input    float64
		expected float64
	}{
		{-1, -math.Pi / 2},
		{-0.8660254, -math.Pi / 3},
		{-0.7071068, -math.Pi / 4},
		{-0.5, -math.Pi / 6},
		{0, 0},
		{0.5, math.Pi / 6},
		{0.7071068, math.Pi / 4},
		{0.8660254, math.Pi / 3},
		{1, math.Pi / 2},
	}

	for _, tc := range testCases {
		actual := Asin(tc.input)
		diff := math.Abs(tc.expected - actual)
		if diff > testAsinDelta {
			t.Errorf("Asin(%v): expected %v, got %v (diff %v > %v)",
				tc.input, tc.expected, actual, diff, testAsinDelta)
		}
	}
}

// TestAsin_RandomValues tests asin() against random values in [-1, 1]
func TestAsin_RandomValues(t *testing.T) {
	for i := 0; i < 10000; i++ {
		d := RandomFloat64()
		if RandomBool() {
			d = -d
		}

		expected := math.Asin(d)
		actual := Asin(d)

		// Check value is within expected range
		if actual < -math.Pi/2 {
			t.Errorf("Asin(%v) = %v, expected >= -Pi/2", d, actual)
		}
		if actual > math.Pi/2 {
			t.Errorf("Asin(%v) = %v, expected <= Pi/2", d, actual)
		}

		diff := math.Abs(expected - actual)
		if diff > testAsinDelta {
			t.Errorf("Asin(%v): expected %v, got %v (diff %v > %v)",
				d, expected, actual, diff, testAsinDelta)
		}
	}
}

// TestHaversin_NaNInputs tests that haversin returns NaN for NaN inputs
func TestHaversin_NaNInputs(t *testing.T) {
	if !math.IsNaN(HaversinMeters(1, 1, 1, math.NaN())) {
		t.Error("Expected HaversinMeters with NaN lon2 to return NaN")
	}
	if !math.IsNaN(HaversinMeters(1, 1, math.NaN(), 1)) {
		t.Error("Expected HaversinMeters with NaN lat2 to return NaN")
	}
	if !math.IsNaN(HaversinMeters(1, math.NaN(), 1, 1)) {
		t.Error("Expected HaversinMeters with NaN lon1 to return NaN")
	}
	if !math.IsNaN(HaversinMeters(math.NaN(), 1, 1, 1)) {
		t.Error("Expected HaversinMeters with NaN lat1 to return NaN")
	}
}

// TestHaversin_ZeroDistance tests that identical points have zero distance
func TestHaversin_ZeroDistance(t *testing.T) {
	testCases := []struct {
		lat1, lon1, lat2, lon2 float64
	}{
		{0, 0, 0, 0},
		{0, -180, 0, -180},
		{0, -180, 0, 180},
		{0, 180, 0, 180},
		{90, 0, 90, 0},
		{90, -180, 90, -180},
		{90, -180, 90, 180},
		{90, 180, 90, 180},
	}

	for _, tc := range testCases {
		actual := HaversinMeters(tc.lat1, tc.lon1, tc.lat2, tc.lon2)
		if actual != 0 {
			t.Errorf("HaversinMeters(%v, %v, %v, %v): expected 0, got %v",
				tc.lat1, tc.lon1, tc.lat2, tc.lon2, actual)
		}
	}
}

// TestHaversin_HalfCircle tests half a circle on the equator
func TestHaversin_HalfCircle(t *testing.T) {
	// Test half a circle on the equator, using WGS84 mean earth radius in meters
	earthRadiusMs := 6_371_008.7714
	halfCircle := earthRadiusMs * math.Pi

	actual := HaversinMeters(0, 0, 0, 180)
	if math.Abs(halfCircle-actual) > 0 {
		t.Errorf("HaversinMeters(0, 0, 0, 180): expected %v, got %v", halfCircle, actual)
	}
}

// TestHaversin_KnownDistances tests haversin against known distances
func TestHaversin_KnownDistances(t *testing.T) {
	// Test with random offsets (simulating the Java test)
	randomLat1 := 40.7143528 + float64(RandomIntN(10)-5)*360
	randomLon1 := -74.0059731 + float64(RandomIntN(10)-5)*360
	randomLat2 := 40.65 + float64(RandomIntN(10)-5)*360
	randomLon2 := -73.95 + float64(RandomIntN(10)-5)*360

	actual := HaversinMeters(randomLat1, randomLon1, randomLat2, randomLon2)
	expected := 8_572.1137
	if math.Abs(expected-actual) > 0.01 {
		t.Errorf("HaversinMeters with random offsets: expected %v, got %v", expected, actual)
	}

	// From Solr and ES tests (with their respective epsilons)
	testCases := []struct {
		lat1, lon1, lat2, lon2 float64
		expected               float64
		tolerance              float64
	}{
		{40.7143528, -74.0059731, 40.7143528, -74.0059731, 0, 0},
		{40.7143528, -74.0059731, 40.759011, -73.9844722, 5_285.89, 0.01},
		{40.7143528, -74.0059731, 40.718266, -74.007819, 462.10, 0.01},
		{40.7143528, -74.0059731, 40.7051157, -74.0088305, 1_054.98, 0.01},
		{40.7143528, -74.0059731, 40.7247222, -74, 1_258.12, 0.01},
		{40.7143528, -74.0059731, 40.731033, -73.9962255, 2_028.52, 0.01},
		{40.7143528, -74.0059731, 40.65, -73.95, 8_572.11, 0.01},
	}

	for _, tc := range testCases {
		actual := HaversinMeters(tc.lat1, tc.lon1, tc.lat2, tc.lon2)
		diff := math.Abs(tc.expected - actual)
		if diff > tc.tolerance {
			t.Errorf("HaversinMeters(%v, %v, %v, %v): expected %v, got %v (diff %v > %v)",
				tc.lat1, tc.lon1, tc.lat2, tc.lon2, tc.expected, actual, diff, tc.tolerance)
		}
	}
}

// TestHaversinSortKey tests that haversinSortKey sorts the same way as real haversin
func TestHaversinSortKey(t *testing.T) {
	for i := 0; i < 10000; i++ {
		centerLat := nextLatitude()
		centerLon := nextLongitude()

		lat1 := nextLatitude()
		lon1 := nextLongitude()

		lat2 := nextLatitude()
		lon2 := nextLongitude()

		dist1 := HaversinMeters(centerLat, centerLon, lat1, lon1)
		dist2 := HaversinMeters(centerLat, centerLon, lat2, lon2)
		expected := sloppyMathSignum(dist1 - dist2)

		sortKey1 := HaversinSortKey(centerLat, centerLon, lat1, lon1)
		sortKey2 := HaversinSortKey(centerLat, centerLon, lat2, lon2)
		actual := sloppyMathSignum(sortKey1 - sortKey2)

		if expected != actual {
			t.Errorf("Sort key comparison mismatch: expected sign %v, got %v", expected, actual)
		}

		// Verify that converting sort key back to meters gives the same result
		distFromSortKey1 := HaversinMetersFromSortKey(sortKey1)
		distFromSortKey2 := HaversinMetersFromSortKey(sortKey2)

		if dist1 != distFromSortKey1 {
			t.Errorf("HaversinMetersFromSortKey(%v): expected %v, got %v", sortKey1, dist1, distFromSortKey1)
		}
		if dist2 != distFromSortKey2 {
			t.Errorf("HaversinMetersFromSortKey(%v): expected %v, got %v", sortKey2, dist2, distFromSortKey2)
		}
	}
}

// TestHaversinFromSortKey tests haversinMeters from sort key with zero
func TestHaversinFromSortKey(t *testing.T) {
	actual := HaversinMetersFromSortKey(0)
	if actual != 0 {
		t.Errorf("HaversinMetersFromSortKey(0): expected 0, got %v", actual)
	}
}

// TestHaversin_AgainstSlowVersion tests haversin against a slow but accurate implementation
func TestHaversin_AgainstSlowVersion(t *testing.T) {
	for i := 0; i < 100000; i++ {
		lat1 := nextLatitude()
		lon1 := nextLongitude()
		lat2 := nextLatitude()
		lon2 := nextLongitude()

		expected := slowHaversin(lat1, lon1, lat2, lon2)
		actual := HaversinMeters(lat1, lon1, lat2, lon2)

		diff := math.Abs(expected - actual)
		if diff > testHaversinDelta {
			t.Errorf("HaversinMeters(%v, %v, %v, %v): expected %v, got %v (diff %v > %v)",
				lat1, lon1, lat2, lon2, expected, actual, diff, testHaversinDelta)
		}
	}
}

// TestHaversin_AcrossWholeWorldSteps tests across the whole world with step increments
func TestHaversin_AcrossWholeWorldSteps(t *testing.T) {
	for lat1 := -90; lat1 <= 90; lat1 += 10 {
		for lon1 := -180; lon1 <= 180; lon1 += 10 {
			for lat2 := -90; lat2 <= 90; lat2 += 10 {
				for lon2 := -180; lon2 <= 180; lon2 += 10 {
					expected := slowHaversin(float64(lat1), float64(lon1), float64(lat2), float64(lon2))
					actual := HaversinMeters(float64(lat1), float64(lon1), float64(lat2), float64(lon2))

					diff := math.Abs(expected - actual)
					if diff > testHaversinDelta {
						t.Errorf("HaversinMeters(%v, %v, %v, %v): expected %v, got %v (diff %v > %v)",
							lat1, lon1, lat2, lon2, expected, actual, diff, testHaversinDelta)
					}
				}
			}
		}
	}
}

// TestHaversin_AgainstSlowVersionReasonable tests haversin for reasonable distances (< 1000km)
func TestHaversin_AgainstSlowVersionReasonable(t *testing.T) {
	for i := 0; i < 100000; i++ {
		lat1 := nextLatitude()
		lon1 := nextLongitude()
		lat2 := nextLatitude()
		lon2 := nextLongitude()

		expected := HaversinMeters(lat1, lon1, lat2, lon2)
		if expected < 1_000_000 {
			actual := slowHaversin(lat1, lon1, lat2, lon2)
			diff := math.Abs(expected - actual)
			if diff > testReasonableHaversinDelta {
				t.Errorf("HaversinMeters(%v, %v, %v, %v): expected %v, got %v (diff %v > %v)",
					lat1, lon1, lat2, lon2, expected, actual, diff, testReasonableHaversinDelta)
			}
		}
	}
}

// slowHaversin is a simple implementation of the wikipedia formula for testing
func slowHaversin(lat1, lon1, lat2, lon2 float64) float64 {
	h1 := (1 - math.Cos((lat2-lat1)*math.Pi/180.0)) / 2
	h2 := (1 - math.Cos((lon2-lon1)*math.Pi/180.0)) / 2
	h := h1 + math.Cos(lat1*math.Pi/180.0)*math.Cos(lat2*math.Pi/180.0)*h2
	return 2 * 6371008.7714 * math.Asin(math.Min(1, math.Sqrt(h)))
}

// nextLatitude returns a random latitude between -90 and 90
func nextLatitude() float64 {
	return RandomFloat64()*180 - 90
}

// nextLongitude returns a random longitude between -180 and 180
func nextLongitude() float64 {
	return RandomFloat64()*360 - 180
}

// sloppyMathSignum returns the sign of a number: -1, 0, or 1
func sloppyMathSignum(x float64) int {
	if x < 0 {
		return -1
	}
	if x > 0 {
		return 1
	}
	return 0
}

// RandomFloat64 returns a random float64 in [0, 1)
func RandomFloat64() float64 {
	return GetRandom().Float64()
}
