// Code in this file mirrors org.apache.lucene.geo.GeoUtils from
// Apache Lucene 10.4.0. The package-level helpers cover coordinate
// validation, Earth-model constants, orientation predicates, the
// haversin distance-query sort-key search, and the bbox/circle
// Relate decomposition used by the distance-query BKD walker.

package geo

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/util"
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

// WindingOrderFromSign maps an integer sign (-1, 0, +1) to the
// corresponding WindingOrder constant. It panics on an invalid
// sign, matching Java's WindingOrder.fromSign which throws
// IllegalArgumentException.
func WindingOrderFromSign(sign int) WindingOrder {
	switch sign {
	case -1:
		return WindingClockwise
	case 0:
		return WindingColinear
	case 1:
		return WindingCounterClockwise
	default:
		panic(fmt.Sprintf("geo: invalid WindingOrder sign: %d", sign))
	}
}

// Orient returns the orientation of the triple (a, b, c):
//
//	+1 if counter-clockwise,
//	-1 if clockwise,
//	 0 if collinear.
//
// It is the Go port of org.apache.lucene.geo.GeoUtils#orient. The
// implementation uses the simple double-precision cross product;
// the floating-point-robust variant from Shewchuk's "Orient2D" is
// noted in Lucene's source as a future improvement and is not in
// scope for this port.
func Orient(ax, ay, bx, by, cx, cy float64) int {
	v1 := (bx - ax) * (cy - ay)
	v2 := (cx - ax) * (by - ay)
	switch {
	case v1 > v2:
		return 1
	case v1 < v2:
		return -1
	default:
		return 0
	}
}

// LineCrossesLine reports whether the two open segments (a1, b1)
// and (a2, b2) intersect strictly in their interiors (endpoint
// touches are not counted). Mirrors Java's
// GeoUtils#lineCrossesLine.
func LineCrossesLine(a1x, a1y, b1x, b1y, a2x, a2y, b2x, b2y float64) bool {
	return Orient(a2x, a2y, b2x, b2y, a1x, a1y)*Orient(a2x, a2y, b2x, b2y, b1x, b1y) < 0 &&
		Orient(a1x, a1y, b1x, b1y, a2x, a2y)*Orient(a1x, a1y, b1x, b1y, b2x, b2y) < 0
}

// LineOverlapLine reports whether the two segments are collinear
// (every endpoint of either segment lies on the line through the
// other). Mirrors Java's GeoUtils#lineOverlapLine.
func LineOverlapLine(a1x, a1y, b1x, b1y, a2x, a2y, b2x, b2y float64) bool {
	return Orient(a2x, a2y, b2x, b2y, a1x, a1y) == 0 &&
		Orient(a2x, a2y, b2x, b2y, b1x, b1y) == 0 &&
		Orient(a1x, a1y, b1x, b1y, a2x, a2y) == 0 &&
		Orient(a1x, a1y, b1x, b1y, b2x, b2y) == 0
}

// LineCrossesLineWithBoundary is the closed-interior version of
// LineCrossesLine: a touch at the boundary (an endpoint of one
// segment lying on the other) counts as a crossing. Mirrors Java's
// GeoUtils#lineCrossesLineWithBoundary.
func LineCrossesLineWithBoundary(a1x, a1y, b1x, b1y, a2x, a2y, b2x, b2y float64) bool {
	return Orient(a2x, a2y, b2x, b2y, a1x, a1y)*Orient(a2x, a2y, b2x, b2y, b1x, b1y) <= 0 &&
		Orient(a1x, a1y, b1x, b1y, a2x, a2y)*Orient(a1x, a1y, b1x, b1y, b2x, b2y) <= 0
}

// Within90LonDegrees reports whether every longitude in
// [minLon, maxLon] is within 90 degrees of lon. Mirrors Java's
// package-private GeoUtils#within90LonDegrees and is exported here
// for reuse by Relate.
func Within90LonDegrees(lon, minLon, maxLon float64) bool {
	switch {
	case maxLon <= lon-180:
		lon -= 360
	case minLon >= lon+180:
		lon += 360
	}
	return maxLon-lon < 90 && lon-minLon < 90
}

// DistanceQuerySortKey performs a binary search over the IEEE-754
// bit-pattern space of non-negative doubles to find the smallest
// sort key whose haversine distance is >= radius. It is the Go port
// of GeoUtils#distanceQuerySortKey and is used by the BKD distance
// query to compare cell sort keys without recomputing the haversine
// for every point.
func DistanceQuerySortKey(radius float64) float64 {
	// Effectively infinite radius: bail with the maximum haversine
	// sort key, matching the Java guard.
	maxHav := util.HaversinMetersFromSortKey(math.MaxFloat64)
	if radius >= maxHav {
		return maxHav
	}

	lo := uint64(0)
	hi := math.Float64bits(math.MaxFloat64)
	for lo <= hi {
		mid := (lo + hi) >> 1
		sortKey := math.Float64frombits(mid)
		midRadius := util.HaversinMetersFromSortKey(sortKey)
		if midRadius == radius {
			return sortKey
		}
		if midRadius > radius {
			if mid == 0 {
				break
			}
			hi = mid - 1
		} else {
			lo = mid + 1
		}
	}
	return math.Float64frombits(lo)
}

// Relate computes the relation between a non-dateline-crossing
// bounding box and a haversine distance query whose centre is
// (lat, lon) and whose sort-key threshold is distanceSortKey. The
// axisLat parameter is the latitude of the disk's tangent meridian
// (as returned by AxisLat) and is used to fast-reject boxes that
// the disk cannot enclose.
//
// Panics with the same message Java emits when minLon > maxLon, as
// the function is undefined for dateline-crossing boxes.
//
// Mirrors org.apache.lucene.geo.GeoUtils#relate.
func Relate(minLat, maxLat, minLon, maxLon, lat, lon, distanceSortKey, axisLat float64) Relation {
	if minLon > maxLon {
		panic("geo: Box crosses the dateline")
	}
	if (lon < minLon || lon > maxLon) &&
		(axisLat+AxisLatError < minLat || axisLat-AxisLatError > maxLat) {
		// Disk does not straddle the meridian axis, so all four
		// corners must be checked.
		if util.HaversinSortKey(lat, lon, minLat, minLon) > distanceSortKey &&
			util.HaversinSortKey(lat, lon, minLat, maxLon) > distanceSortKey &&
			util.HaversinSortKey(lat, lon, maxLat, minLon) > distanceSortKey &&
			util.HaversinSortKey(lat, lon, maxLat, maxLon) > distanceSortKey {
			return CellOutsideQuery
		}
	}
	if Within90LonDegrees(lon, minLon, maxLon) &&
		util.HaversinSortKey(lat, lon, minLat, minLon) <= distanceSortKey &&
		util.HaversinSortKey(lat, lon, minLat, maxLon) <= distanceSortKey &&
		util.HaversinSortKey(lat, lon, maxLat, minLon) <= distanceSortKey &&
		util.HaversinSortKey(lat, lon, maxLat, maxLon) <= distanceSortKey {
		return CellInsideQuery
	}
	return CellCrossesQuery
}
