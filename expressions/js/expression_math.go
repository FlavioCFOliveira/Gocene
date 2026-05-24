// Package js implements org.apache.lucene.expressions.js: the bundled
// JavaScript-syntax expression compiler.
package js

import "math"

// ExpressionMath exposes the bundled mathematical helpers available inside
// every compiled expression. Mirrors org.apache.lucene.expressions.js.ExpressionMath.

// Pow returns base raised to the supplied exponent.
func Pow(base, exp float64) float64 { return math.Pow(base, exp) }

// Log10 returns the base-10 logarithm.
func Log10(x float64) float64 { return math.Log10(x) }

// Log2 returns the base-2 logarithm.
func Log2(x float64) float64 { return math.Log2(x) }

// Sqrt returns the square root.
func Sqrt(x float64) float64 { return math.Sqrt(x) }

// Exp returns e raised to x.
func Exp(x float64) float64 { return math.Exp(x) }

// Sin / Cos / Tan are the trigonometric helpers.
func Sin(x float64) float64 { return math.Sin(x) }
func Cos(x float64) float64 { return math.Cos(x) }
func Tan(x float64) float64 { return math.Tan(x) }

// Abs returns |x|.
func Abs(x float64) float64 { return math.Abs(x) }

// Ceil / Floor / Round mirror their JavaScript equivalents.
func Ceil(x float64) float64  { return math.Ceil(x) }
func Floor(x float64) float64 { return math.Floor(x) }
func Round(x float64) float64 { return math.Round(x) }

// Max / Min are the two-argument variants used inside expressions.
func Max(a, b float64) float64 { return math.Max(a, b) }
func Min(a, b float64) float64 { return math.Min(a, b) }

// Acos, Asin, Atan, Atan2 are inverse trigonometric helpers.
func Acos(x float64) float64          { return math.Acos(x) }
func Asin(x float64) float64          { return math.Asin(x) }
func Atan(x float64) float64          { return math.Atan(x) }
func Atan2(y, x float64) float64      { return math.Atan2(y, x) }

// Acosh, Asinh, Atanh are inverse hyperbolic helpers.
func Acosh(x float64) float64 { return math.Acosh(x) }
func Asinh(x float64) float64 { return math.Asinh(x) }
func Atanh(x float64) float64 { return math.Atanh(x) }

// Cosh, Sinh, Tanh are hyperbolic helpers.
func Cosh(x float64) float64 { return math.Cosh(x) }
func Sinh(x float64) float64 { return math.Sinh(x) }
func Tanh(x float64) float64 { return math.Tanh(x) }

// Ln returns the natural logarithm.
func Ln(x float64) float64 { return math.Log(x) }

// Logn returns the base-n logarithm: log(x) / log(base).
func Logn(base, x float64) float64 { return math.Log(x) / math.Log(base) }

// Haversin computes the haversine of the central angle between two points
// on a sphere from their (lat1, lon1, lat2, lon2) in degrees. Returns the
// great-circle distance in kilometres (Earth radius = 6371.0 km), matching
// the Lucene ExpressionMath.haversin(lat1,lon1,lat2,lon2) contract.
func Haversin(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusKm = 6371.0
	dLat := (lat2 - lat1) * math.Pi / 180.0
	dLon := (lon2 - lon1) * math.Pi / 180.0
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180.0)*math.Cos(lat2*math.Pi/180.0)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * earthRadiusKm * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}
