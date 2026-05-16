// Code in this file mirrors org.apache.lucene.geo.Point from Apache
// Lucene 10.4.0.

package geo

import "strconv"

// Point represents a single geographic point on Earth's surface,
// expressed in decimal-degree latitude and longitude.
//
// It is the Go port of org.apache.lucene.geo.Point. Notes from the
// Java javadoc:
//
//  1. latitude/longitude values must be in decimal degrees.
//  2. For more advanced GeoSpatial indexing and query operations see
//     the spatial-extras module.
//
// Point is immutable; the zero value is not valid. Use NewPoint to
// construct it with input validation.
type Point struct {
	lat float64
	lon float64
}

// NewPoint constructs a Point at the given latitude and longitude.
// Both coordinates are validated against the Lucene inclusive bounds
// (CheckLatitude / CheckLongitude). On invalid input it returns an
// error whose message matches the Java IllegalArgumentException text.
func NewPoint(lat, lon float64) (Point, error) {
	if err := CheckLatitude(lat); err != nil {
		return Point{}, err
	}
	if err := CheckLongitude(lon); err != nil {
		return Point{}, err
	}
	return Point{lat: lat, lon: lon}, nil
}

// MustNewPoint is the panic-on-error variant of NewPoint, intended
// for tests and package-init contexts where the inputs are known to
// be valid.
func MustNewPoint(lat, lon float64) Point {
	p, err := NewPoint(lat, lon)
	if err != nil {
		panic(err)
	}
	return p
}

// Lat returns the latitude of the point in decimal degrees.
func (p Point) Lat() float64 { return p.lat }

// Lon returns the longitude of the point in decimal degrees.
func (p Point) Lon() float64 { return p.lon }

// toComponent2D returns the Component2D representation of the point.
// It satisfies the sealed Geometry interface contract.
func (p Point) toComponent2D() Component2D { return newPoint2D(p.lon, p.lat) }

// latLonGeometry is the sealed marker on LatLonGeometry; declared as
// a method on Point so that *Point satisfies LatLonGeometry.
func (Point) latLonGeometry() {}

// Equals reports whether two Points have bit-identical latitude and
// longitude. Equality follows Java's Double.compare semantics: NaNs
// would be equal to each other and +0.0 differs from -0.0. Since
// NewPoint rejects NaN, in practice this collapses to ordinary
// floating-point equality outside the +0.0 / -0.0 edge case.
func (p Point) Equals(o Point) bool {
	return javaDoubleCompare(p.lat, o.lat) == 0 && javaDoubleCompare(p.lon, o.lon) == 0
}

// HashCode mirrors Java's Point.hashCode(): a 31-mix of
// Double.hashCode(lat) and Double.hashCode(lon). It returns a Go
// int32-shaped value to match Java's `int` hashCode.
func (p Point) HashCode() int32 {
	result := javaDoubleHashCode(p.lat)
	result = 31*result + javaDoubleHashCode(p.lon)
	return result
}

// String mirrors Java's Point.toString() exactly: "Point(lon,lat)".
// Note Lucene emits longitude FIRST in this format despite the
// constructor taking (lat, lon); the order is preserved verbatim so
// log lines and diagnostic output match the Java original.
func (p Point) String() string {
	var b []byte
	b = append(b, "Point("...)
	b = appendJavaDouble(b, p.lon)
	b = append(b, ',')
	b = appendJavaDouble(b, p.lat)
	b = append(b, ')')
	return string(b)
}

// appendJavaDouble appends v in Java Double.toString format to dst.
// Wraps formatJavaDouble for fast-path slice growth.
func appendJavaDouble(dst []byte, v float64) []byte {
	return append(dst, formatJavaDouble(v)...)
}

// javaDoubleCompare mirrors Java's Double.compare semantics on bit
// patterns: it imposes a total order including NaN handling, by
// comparing the IEEE-754 long bit patterns after a flip of negative
// values to make them sort below positives. Returns -1, 0, or +1.
func javaDoubleCompare(a, b float64) int {
	ab := transformDoubleToLong(a)
	bb := transformDoubleToLong(b)
	switch {
	case ab < bb:
		return -1
	case ab > bb:
		return 1
	default:
		return 0
	}
}

// transformDoubleToLong applies the same bit-twiddle Java uses to
// fold the IEEE-754 representation into a total order. Borrowed from
// java.lang.Double.compare(double, double).
func transformDoubleToLong(v float64) int64 {
	bits := int64(0)
	// math.Float64bits returns the IEEE bit pattern as uint64.
	bits = int64(float64bitsConst(v))
	if bits < 0 {
		// Map negative-sign bit patterns so smaller magnitudes come
		// out smaller in unsigned comparison space.
		bits ^= 0x7fff_ffff_ffff_ffff
	}
	return bits
}

// float64bitsConst is a tiny shim over math.Float64bits kept here so
// the implementation reads top-to-bottom without an external import
// cluttering the file. It is unexported and inlinable.
func float64bitsConst(v float64) uint64 { return mathFloat64bits(v) }

// javaDoubleHashCode mirrors Java's Double.hashCode(double):
//
//	long bits = Double.doubleToLongBits(value);
//	return (int)(bits ^ (bits >>> 32));
//
// We use Float64bits which already follows the Java
// doubleToRawLongBits semantics for finite values; the only
// difference is that doubleToLongBits collapses all NaN bit patterns
// to a single canonical NaN value. Since NewPoint rejects NaN we do
// not need to canonicalise here.
func javaDoubleHashCode(v float64) int32 {
	bits := mathFloat64bits(v)
	return int32(int64(bits) ^ (int64(bits) >> 32))
}

// ParsePoint parses a Lucene Point.toString() representation. This is
// not part of the Java original but is convenient for round-trip
// testing. Kept package-private (lowercase first letter) to avoid
// extending the public API beyond what Lucene exposes.
func parsePointString(s string) (Point, error) {
	if len(s) < len("Point(,)") || s[:len("Point(")] != "Point(" || s[len(s)-1] != ')' {
		return Point{}, &strconv.NumError{Func: "ParsePoint", Num: s, Err: strconv.ErrSyntax}
	}
	inner := s[len("Point(") : len(s)-1]
	comma := -1
	for i := 0; i < len(inner); i++ {
		if inner[i] == ',' {
			comma = i
			break
		}
	}
	if comma < 0 {
		return Point{}, &strconv.NumError{Func: "ParsePoint", Num: s, Err: strconv.ErrSyntax}
	}
	lon, err := strconv.ParseFloat(inner[:comma], 64)
	if err != nil {
		return Point{}, err
	}
	lat, err := strconv.ParseFloat(inner[comma+1:], 64)
	if err != nil {
		return Point{}, err
	}
	return NewPoint(lat, lon)
}
