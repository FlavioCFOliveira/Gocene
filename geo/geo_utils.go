// Code in this file mirrors org.apache.lucene.geo.GeoUtils from
// Apache Lucene 10.4.0. The full GeoUtils port is owned by task #282;
// the constants and helpers below are the minimal surface needed by
// the concrete geometry types (Point, Rectangle, Line, Polygon, ...)
// to validate their inputs. They are intentionally kept signature- and
// behaviour-compatible with the Java original so that task #282 can
// extend rather than replace them.

package geo

import (
	"fmt"
	"math"
)

// Inclusive bounds on geographic coordinates in decimal degrees.
// Mirror the Java constants org.apache.lucene.geo.GeoUtils.MIN_LAT_INCL,
// MAX_LAT_INCL, MIN_LON_INCL, MAX_LON_INCL.
const (
	// MinLatIncl is the inclusive minimum latitude in decimal degrees.
	MinLatIncl = -90.0

	// MaxLatIncl is the inclusive maximum latitude in decimal degrees.
	MaxLatIncl = 90.0

	// MinLonIncl is the inclusive minimum longitude in decimal degrees.
	MinLonIncl = -180.0

	// MaxLonIncl is the inclusive maximum longitude in decimal degrees.
	MaxLonIncl = 180.0
)

// WindingOrder describes the orientation of a polygon's vertices.
// It is the Go port of org.apache.lucene.geo.GeoUtils.WindingOrder.
//
// The integer Sign() values match the Java enum: CW=-1, COLINEAR=0,
// CCW=+1.
type WindingOrder int

const (
	// WindingClockwise is the clockwise winding order. Used for the
	// shell of a polygon (Lucene convention).
	WindingClockwise WindingOrder = -1

	// WindingColinear means the polygon is degenerate (its signed
	// area is exactly zero).
	WindingColinear WindingOrder = 0

	// WindingCounterClockwise is the counter-clockwise winding
	// order. Used for holes in a polygon (Lucene convention).
	WindingCounterClockwise WindingOrder = 1
)

// Sign returns the integer sign of the winding order (-1, 0, or +1),
// matching the Java enum's sign() accessor.
func (w WindingOrder) Sign() int { return int(w) }

// String returns the symbolic name of the winding order, matching
// the Java enum constant names (CW, COLINEAR, CCW).
func (w WindingOrder) String() string {
	switch w {
	case WindingClockwise:
		return "CW"
	case WindingColinear:
		return "COLINEAR"
	case WindingCounterClockwise:
		return "CCW"
	default:
		return "UNKNOWN"
	}
}

// EarthMeanRadiusMeters is the WGS84 mean Earth radius in metres,
// matching org.apache.lucene.geo.GeoUtils.EARTH_MEAN_RADIUS_METERS
// and org.apache.lucene.util.SloppyMath's internal `toMeters`
// constant. Used by Rectangle.FromPointDistance, Circle, and the
// haversine helpers.
const EarthMeanRadiusMeters = 6_371_008.7714

// Radian counterparts of the inclusive degree bounds. Mirror the Java
// constants org.apache.lucene.geo.GeoUtils.MIN_LAT_RADIANS,
// MAX_LAT_RADIANS, MIN_LON_RADIANS, MAX_LON_RADIANS, each defined as
// Math.toRadians of the corresponding degree bound.
var (
	// MinLatRadians is MinLatIncl converted to radians.
	MinLatRadians = MinLatIncl * math.Pi / 180

	// MaxLatRadians is MaxLatIncl converted to radians.
	MaxLatRadians = MaxLatIncl * math.Pi / 180

	// MinLonRadians is MinLonIncl converted to radians.
	MinLonRadians = MinLonIncl * math.Pi / 180

	// MaxLonRadians is MaxLonIncl converted to radians.
	MaxLonRadians = MaxLonIncl * math.Pi / 180
)

// ErrInvalidLatitude / ErrInvalidLongitude provide a sentinel type
// for callers that want to distinguish coordinate-validation errors
// from other failure modes. The error returned by CheckLatitude /
// CheckLongitude wraps these sentinels.
var (
	ErrInvalidLatitude  = fmt.Errorf("invalid latitude")
	ErrInvalidLongitude = fmt.Errorf("invalid longitude")
)

// CheckLatitude validates that the latitude is a finite decimal-degree
// value in [MinLatIncl, MaxLatIncl]. NaN, +Inf, -Inf are rejected.
//
// The returned error message matches the Java original so that
// behavioural tests can compare against the Lucene exception text
// (TestPoint expects "invalid latitude 134.14; must be between -90.0
// and 90.0", for example).
func CheckLatitude(latitude float64) error {
	if math.IsNaN(latitude) || latitude < MinLatIncl || latitude > MaxLatIncl {
		return fmt.Errorf("%w %s; must be between %s and %s",
			ErrInvalidLatitude,
			formatJavaDouble(latitude),
			formatJavaDouble(MinLatIncl),
			formatJavaDouble(MaxLatIncl))
	}
	return nil
}

// CheckLongitude validates that the longitude is a finite
// decimal-degree value in [MinLonIncl, MaxLonIncl]. NaN, +Inf, -Inf
// are rejected. The returned error message matches the Java original.
func CheckLongitude(longitude float64) error {
	if math.IsNaN(longitude) || longitude < MinLonIncl || longitude > MaxLonIncl {
		return fmt.Errorf("%w %s; must be between %s and %s",
			ErrInvalidLongitude,
			formatJavaDouble(longitude),
			formatJavaDouble(MinLonIncl),
			formatJavaDouble(MaxLonIncl))
	}
	return nil
}

// formatJavaDouble renders a float64 using Java's Double.toString
// convention as closely as fmt allows. For finite values the Java
// representation always includes a decimal point ("90" -> "90.0",
// "1.5" -> "1.5"). The %g verb in Go elides the trailing ".0" so we
// post-process to add it back when the value is integral.
//
// This matches Lucene's exception text format, e.g.
//
//	"invalid latitude 134.14; must be between -90.0 and 90.0"
//
// without depending on a heavier formatting library.
func formatJavaDouble(v float64) string {
	if math.IsNaN(v) {
		return "NaN"
	}
	if math.IsInf(v, 1) {
		return "Infinity"
	}
	if math.IsInf(v, -1) {
		return "-Infinity"
	}
	// %g uses the shortest representation that round-trips; that is
	// the same algorithm Java uses for Double.toString in practice.
	s := fmt.Sprintf("%g", v)
	// Ensure integral values carry the trailing ".0" that Java
	// always emits.
	if !containsDecimalOrExponent(s) {
		s += ".0"
	}
	return s
}

// containsDecimalOrExponent reports whether the rendered double
// already includes a decimal point or scientific-notation exponent.
func containsDecimalOrExponent(s string) bool {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '.', 'e', 'E':
			return true
		}
	}
	return false
}
