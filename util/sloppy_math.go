// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Some code derived from jodk: http://code.google.com/p/jodk/ (apache 2.0)
// Asin() derived from fdlibm: http://www.netlib.org/fdlibm/e_asin.c (public domain):
// =============================================================================
// Copyright (C) 1993 by Sun Microsystems, Inc. All rights reserved.
//
// Developed at SunSoft, a Sun Microsystems, Inc. business.
// Permission to use, copy, modify, and distribute this
// software is freely granted, provided that this notice
// is preserved.
// =============================================================================

package util

import (
	"math"
)

// SloppyMath provides math functions that trade off accuracy for speed.
// These approximations are useful for performance-sensitive operations
// where slight precision loss is acceptable.

// Earth's mean radius in meters (WGS84 equatorial radius)
// See: http://earth-info.nga.mil/GandG/publications/tr8350.2/wgs84fin.pdf
const toMeters = 6_371_008.7714

// Constants for cos/asin calculations
const (
	oneDivF2 = 1.0 / 2.0
	oneDivF3 = 1.0 / 6.0
	oneDivF4 = 1.0 / 24.0
)

// Constants for sin/cos table generation
const (
	pio2      = math.Pi / 2.0
	sinCosTabsSize = (1 << 11) + 1
)

// Pre-computed high-precision constants
var (
	// 1.57079632673412561417e+00 first 33 bits of pi/2
	pio2Hi = math.Float64frombits(0x3FF921FB54400000)
	// 6.07710050650619224932e-11 pi/2 - PIO2_HI
	pio2Lo = math.Float64frombits(0x3DD0B4611A626331)
	twoPiHi = 4 * pio2Hi
	twoPiLo = 4 * pio2Lo
	sinCosDeltaHi = twoPiHi / (sinCosTabsSize - 1)
	sinCosDeltaLo = twoPiLo / (sinCosTabsSize - 1)
	sinCosIndexer = 1.0 / (sinCosDeltaHi + sinCosDeltaLo)

	// SinCosMaxValueForIntModulo is the max abs value for fast modulo
	// This value must be < (Integer.MAX_VALUE / SIN_COS_INDEXER)
	SinCosMaxValueForIntModulo = ((math.MaxInt32 >> 9) / sinCosIndexer) * 0.99
)

// Lookup tables for sin/cos
var (
	sinTab = make([]float64, sinCosTabsSize)
	cosTab = make([]float64, sinCosTabsSize)
)

// Constants for asin table generation
const (
	asinTabsSize = (1 << 13) + 1
)

// Supposed to be >= sin(77.2deg), as fdlibm code is supposed to work with values > 0.975,
// but seems to work well enough as long as value >= sin(25deg).
var asinMaxValueForTabs = math.Sin(73.0 * math.Pi / 180.0)

var (
	asinDelta   = asinMaxValueForTabs / (asinTabsSize - 1)
	asinIndexer = 1.0 / asinDelta
)

// Lookup tables for asin
var (
	asinTab          = make([]float64, asinTabsSize)
	asinDer1DivF1Tab = make([]float64, asinTabsSize)
	asinDer2DivF2Tab = make([]float64, asinTabsSize)
	asinDer3DivF3Tab = make([]float64, asinTabsSize)
	asinDer4DivF4Tab = make([]float64, asinTabsSize)
)

// Constants for asin polynomial approximation (from fdlibm)
var (
	// 1.57079632679489655800e+00
	asinPio2Hi = math.Float64frombits(0x3FF921FB54442D18)
	// 6.12323399573676603587e-17
	asinPio2Lo = math.Float64frombits(0x3C91A62633145C07)
	//  1.66666666666666657415e-01
	asinPs0 = math.Float64frombits(0x3fc5555555555555)
	// -3.25565818622400915405e-01
	asinPs1 = math.Float64frombits(0xbfd4d61203eb6f7d)
	//  2.01212532134862925881e-01
	asinPs2 = math.Float64frombits(0x3fc9c1550e884455)
	// -4.00555345006794114027e-02
	asinPs3 = math.Float64frombits(0xbfa48228b5688f3b)
	//  7.91534994289814532176e-04
	asinPs4 = math.Float64frombits(0x3f49efe07501b288)
	//  3.47933107596021167570e-05
	asinPs5 = math.Float64frombits(0x3f023de10dfdf709)
	// -2.40339491173441421878e+00
	asinQs1 = math.Float64frombits(0xc0033a271c8a2d4b)
	//  2.02094576023350569471e+00
	asinQs2 = math.Float64frombits(0x40002ae59c598ac8)
	// -6.88283971605453293030e-01
	asinQs3 = math.Float64frombits(0xbfe6066c1b8d0159)
	//  7.70381505559019352791e-02
	asinQs4 = math.Float64frombits(0x3fb3b8c5b12e9282)
)

// init initializes the lookup tables
func init() {
	// Initialize sin and cos tables
	sinCosPiIndex := (sinCosTabsSize - 1) / 2
	sinCosPiMul2Index := 2 * sinCosPiIndex
	sinCosPiMul0_5Index := sinCosPiIndex / 2
	sinCosPiMul1_5Index := 3 * sinCosPiIndex / 2

	for i := 0; i < sinCosTabsSize; i++ {
		// angle: in [0, 2*PI]
		angle := float64(i)*sinCosDeltaHi + float64(i)*sinCosDeltaLo
		sinAngle := math.Sin(angle)
		cosAngle := math.Cos(angle)

		// For indexes corresponding to null cosine or sine, we make sure the value is zero
		// and not an epsilon. This allows for a much better accuracy for results close to zero.
		if i == sinCosPiIndex {
			sinAngle = 0.0
		} else if i == sinCosPiMul2Index {
			sinAngle = 0.0
		} else if i == sinCosPiMul0_5Index {
			cosAngle = 0.0
		} else if i == sinCosPiMul1_5Index {
			cosAngle = 0.0
		}

		sinTab[i] = sinAngle
		cosTab[i] = cosAngle
	}

	// Initialize asin tables
	for i := 0; i < asinTabsSize; i++ {
		// x: in [0, ASIN_MAX_VALUE_FOR_TABS]
		x := float64(i) * asinDelta
		asinTab[i] = math.Asin(x)

		oneMinusXSqInv := 1.0 / (1 - x*x)
		oneMinusXSqInv0_5 := math.Sqrt(oneMinusXSqInv)
		oneMinusXSqInv1_5 := oneMinusXSqInv0_5 * oneMinusXSqInv
		oneMinusXSqInv2_5 := oneMinusXSqInv1_5 * oneMinusXSqInv
		oneMinusXSqInv3_5 := oneMinusXSqInv2_5 * oneMinusXSqInv

		asinDer1DivF1Tab[i] = oneMinusXSqInv0_5
		asinDer2DivF2Tab[i] = (x * oneMinusXSqInv1_5) * oneDivF2
		asinDer3DivF3Tab[i] = ((1 + 2*x*x) * oneMinusXSqInv2_5) * oneDivF3
		asinDer4DivF4Tab[i] = ((5 + 2*x*(2+x*(5-2*x))) * oneMinusXSqInv3_5) * oneDivF4
	}
}

// HaversinMeters returns the Haversine distance in meters between two points specified
// in decimal degrees (latitude/longitude). This works correctly even if the dateline
// is between the two points.
//
// Error is at most 4E-1 (40cm) from the actual haversine distance, but is typically much
// smaller for reasonable distances: around 1E-5 (0.01mm) for distances less than 1000km.
//
// Parameters:
//   - lat1: Latitude of the first point
//   - lon1: Longitude of the first point
//   - lat2: Latitude of the second point
//   - lon2: Longitude of the second point
//
// Returns: distance in meters
func HaversinMeters(lat1, lon1, lat2, lon2 float64) float64 {
	return HaversinMetersFromSortKey(HaversinSortKey(lat1, lon1, lat2, lon2))
}

// HaversinMetersFromSortKey returns the Haversine distance in meters between two points
// given the previous result from HaversinSortKey.
//
// Parameters:
//   - sortKey: the sort key from HaversinSortKey
//
// Returns: distance in meters
func HaversinMetersFromSortKey(sortKey float64) float64 {
	return toMeters * 2 * Asin(math.Min(1, math.Sqrt(sortKey*0.5)))
}

// HaversinSortKey returns a sort key for distance. This is less expensive to compute than
// HaversinMeters, but it always compares the same. This can be converted into an actual
// distance with HaversinMetersFromSortKey, which effectively does the second half of the computation.
//
// Parameters:
//   - lat1: Latitude of the first point
//   - lon1: Longitude of the first point
//   - lat2: Latitude of the second point
//   - lon2: Longitude of the second point
//
// Returns: a sort key that can be compared with other sort keys
func HaversinSortKey(lat1, lon1, lat2, lon2 float64) float64 {
	x1 := lat1 * math.Pi / 180.0
	x2 := lat2 * math.Pi / 180.0
	h1 := 1 - Cos(x1-x2)
	h2 := 1 - Cos((lon1-lon2)*math.Pi/180.0)
	h := h1 + Cos(x1)*Cos(x2)*h2
	// Clobber crazy precision so subsequent rounding does not create ties
	return math.Float64frombits(math.Float64bits(h) & 0xFFFFFFFFFFFFFFF8)
}

// Cos returns the trigonometric cosine of an angle.
//
// Error is around 1E-15.
//
// Special cases:
//   - If the argument is NaN or an infinity, then the result is NaN.
//
// Parameters:
//   - a: an angle, in radians
//
// Returns: the cosine of the argument
func Cos(a float64) float64 {
	if a < 0.0 {
		a = -a
	}
	if a > SinCosMaxValueForIntModulo {
		return math.Cos(a)
	}

	// index: possibly outside tables range
	index := int(a*sinCosIndexer + 0.5)
	delta := (a - float64(index)*sinCosDeltaHi) - float64(index)*sinCosDeltaLo

	// Making sure index is within tables range
	// Last value of each table is the same as first, so we ignore it (tabs size minus one) for modulo
	index &= (sinCosTabsSize - 2) // index % (SIN_COS_TABS_SIZE-1)

	indexCos := cosTab[index]
	indexSin := sinTab[index]

	return indexCos + delta*(-indexSin+delta*(-indexCos*oneDivF2+delta*(indexSin*oneDivF3+delta*indexCos*oneDivF4)))
}

// Asin returns the arc sine of a value.
//
// The returned angle is in the range -pi/2 through pi/2. Error is around 1E-7.
//
// Special cases:
//   - If the argument is NaN or its absolute value is greater than 1, then the result is NaN.
//
// Parameters:
//   - a: the value whose arc sine is to be returned
//
// Returns: arc sine of the argument
func Asin(a float64) float64 {
	negateResult := false
	if a < 0.0 {
		a = -a
		negateResult = true
	}

	if a <= asinMaxValueForTabs {
		index := int(a*asinIndexer + 0.5)
		delta := a - float64(index)*asinDelta

		result := asinTab[index] +
			delta*(asinDer1DivF1Tab[index]+
				delta*(asinDer2DivF2Tab[index]+
					delta*(asinDer3DivF3Tab[index]+delta*asinDer4DivF4Tab[index])))

		if negateResult {
			return -result
		}
		return result
	}

	// value > ASIN_MAX_VALUE_FOR_TABS, or value is NaN
	// This part is derived from fdlibm
	if a < 1.0 {
		t := (1.0 - a) * 0.5
		p := t * (asinPs0 + t*(asinPs1+t*(asinPs2+t*(asinPs3+t*(asinPs4+t*asinPs5)))))
		q := 1.0 + t*(asinQs1+t*(asinQs2+t*(asinQs3+t*asinQs4)))
		s := math.Sqrt(t)
		z := s + s*(p/q)
		result := asinPio2Hi - ((z + z) - asinPio2Lo)

		if negateResult {
			return -result
		}
		return result
	}

	// value >= 1.0, or value is NaN
	if a == 1.0 {
		if negateResult {
			return -math.Pi / 2
		}
		return math.Pi / 2
	}
	return math.NaN()
}

// Sin returns the trigonometric sine of an angle converted as a cos operation.
//
// Error is around 1E-12.
//
// Note that this is not quite right... e.g. sin(0) != 0
//
// Special cases:
//   - If the argument is NaN or an infinity, then the result is NaN.
//
// Parameters:
//   - a: an angle, in radians
//
// Returns: the sine of the argument
func Sin(a float64) float64 {
	return Cos(a - pio2)
}
