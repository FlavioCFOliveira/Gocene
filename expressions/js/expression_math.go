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
	// Clamp a to [0, 1] to prevent NaN for antipodal/near-antipodal points
	// where IEEE-754 rounding can push a slightly above 1.0.
	// Matches Lucene's SloppyMath.haversinMeters() which uses min(1.0, ...).
	if a > 1 {
		a = 1
	}
	return 2 * earthRadiusKm * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

// --- JavaScript integer-overflow semantics ---
//
// Port of org.apache.lucene.expressions.js.ExpressionMath which wraps
// java.lang.Math with JavaScript's ToInt32/ToUint32 overflow behaviour.
// In Lucene, the ANTLR bytecode compiler applies ToInt32 after every
// arithmetic operation so that integer expressions match JavaScript's
// 32-bit signed integer overflow semantics.

const (
	// MaxInt32 is 2^31 - 1, the maximum signed 32-bit integer.
	MaxInt32 = 1<<31 - 1
	// MinInt32 is -2^31, the minimum signed 32-bit integer.
	MinInt32 = -1 << 31
	// MaxUint32 is 2^32 - 1.
	MaxUint32 = 1<<32 - 1
	// MaxInt64 is the maximum signed 64-bit integer, used by Lucene to
	// represent "positive infinity" for integer division by zero.
	MaxInt64 = 1<<63 - 1
)

// ToInt32 converts x to a signed 32-bit integer following ECMAScript's
// ToInt32 abstract operation: the value is truncated toward zero,
// then the lower 32 bits are interpreted as a signed two's-complement
// integer.
//
// Mirrors org.apache.lucene.expressions.js.ExpressionMath.toInt32.
func ToInt32(x float64) int32 {
	return int32(int64(x) & 0xFFFFFFFF)
}

// ToUint32 converts x to an unsigned 32-bit integer following ECMAScript's
// ToUint32 abstract operation. The result is in [0, 2^32-1].
func ToUint32(x float64) uint32 {
	return uint32(int64(x) & 0xFFFFFFFF)
}

// ToInt32Float64 converts x through ToInt32 and returns the result as float64,
// matching the JavaScript convention where integer operations produce
// floating-point values with 32-bit overflow applied.
func ToInt32Float64(x float64) float64 {
	return float64(ToInt32(x))
}

// ToUint32Float64 converts x through ToUint32 and returns the result as float64.
func ToUint32Float64(x float64) float64 {
	return float64(ToUint32(x))
}

// IntDiv performs integer division following Lucene's ANTLR bytecode
// compiler semantics: both operands are first converted to int64 (truncated
// toward zero), division by zero returns MaxInt64, and the result is
// converted back to float64.
//
// Mirrors org.apache.lucene.expressions.js.ExpressionMath.divide.
func IntDiv(a, b float64) float64 {
	x := int64(a)
	y := int64(b)
	if y == 0 {
		if x == 0 {
			return 0 // 0/0 = 0 per Lucene convention
		}
		// Return MaxInt64 with sign matching the numerator.
		if x > 0 {
			return float64(MaxInt64)
		}
		return float64(-MaxInt64)
	}
	return float64(x / y)
}

// IntMod performs integer modulus following JavaScript's % operator with
// 32-bit truncation.
func IntMod(a, b float64) float64 {
	return ToInt32Float64(math.Mod(a, b))
}
