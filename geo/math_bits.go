// Helpers that mirror small slices of java.lang.Math and
// java.lang.Double on top of Go's math package, kept in one place so
// the surrounding port code stays import-light.

package geo

import (
	"math"
	"strconv"
)

// strconvFormatFloat32 wraps strconv.FormatFloat with the
// single-precision bit count so the shortest round-trip rendering
// matches Java's Float.toString convention.
func strconvFormatFloat32(v float32) string {
	return strconv.FormatFloat(float64(v), 'g', -1, 32)
}

// mathFloat64bits is the Java equivalent of
// Double.doubleToRawLongBits(double) cast to uint64.
func mathFloat64bits(v float64) uint64 { return math.Float64bits(v) }

// javaFloatCompare mirrors Java's Float.compare on the IEEE-754 bit
// pattern of two float32 values. Returns -1, 0, or +1.
func javaFloatCompare(a, b float32) int {
	ab := transformFloatToInt(a)
	bb := transformFloatToInt(b)
	switch {
	case ab < bb:
		return -1
	case ab > bb:
		return 1
	default:
		return 0
	}
}

// transformFloatToInt applies the same bit-twiddle Java's
// Float.compare uses to fold the IEEE-754 representation into a
// total order.
func transformFloatToInt(v float32) int32 {
	bits := int32(math.Float32bits(v))
	if bits < 0 {
		bits ^= 0x7fff_ffff
	}
	return bits
}

// javaFloatHashCode mirrors Java's Float.hashCode(float):
//
//	int bits = Float.floatToIntBits(value);
//	return bits;
//
// Like Double.hashCode it canonicalises all NaN bit patterns to a
// single value via floatToIntBits; XYCheckVal rejects NaN so we
// don't need to canonicalise here.
func javaFloatHashCode(v float32) int32 {
	return int32(math.Float32bits(v))
}

// appendJavaFloat formats a float32 using Java Float.toString
// semantics and appends it to dst.
func appendJavaFloat(dst []byte, v float32) []byte {
	return append(dst, formatJavaFloat(v)...)
}

// formatJavaFloat renders a float32 like Java's Float.toString:
// integral values carry the trailing ".0", finite values use the
// shortest round-trip representation.
func formatJavaFloat(v float32) string {
	d := float64(v)
	if math.IsNaN(d) {
		return "NaN"
	}
	if math.IsInf(d, 1) {
		return "Infinity"
	}
	if math.IsInf(d, -1) {
		return "-Infinity"
	}
	// Format via Go's strconv at single-precision so trailing
	// digits match Float.toString rather than Double.toString.
	s := strconvFormatFloat32(v)
	if !containsDecimalOrExponent(s) {
		s += ".0"
	}
	return s
}
