// Code in this file mirrors org.apache.lucene.geo.Rectangle from
// Apache Lucene 10.4.0.

package geo

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// Rectangle is an axis-aligned bounding box on Earth's surface,
// expressed in decimal-degree latitude and longitude.
//
// It is the Go port of org.apache.lucene.geo.Rectangle. A rectangle
// can cross the antimeridian (the international dateline), in which
// case MinLon > MaxLon; the predicate CrossesDateline reports this
// state. The latitude bounds always satisfy MinLat <= MaxLat.
//
// Rectangle is immutable; the zero value is not valid. Use
// NewRectangle / MustNewRectangle to construct it with input
// validation.
type Rectangle struct {
	minLat float64
	maxLat float64
	minLon float64
	maxLon float64
}

// NewRectangle constructs a Rectangle from the inclusive
// latitude/longitude bounds. Both coordinates are validated against
// the Lucene inclusive bounds via CheckLatitude / CheckLongitude. It
// returns an error if any bound is out of range or if minLat >
// maxLat. The minLon > maxLon case is allowed (signals a
// dateline-crossing rectangle, matching Lucene).
func NewRectangle(minLat, maxLat, minLon, maxLon float64) (Rectangle, error) {
	if err := CheckLatitude(minLat); err != nil {
		return Rectangle{}, err
	}
	if err := CheckLatitude(maxLat); err != nil {
		return Rectangle{}, err
	}
	if err := CheckLongitude(minLon); err != nil {
		return Rectangle{}, err
	}
	if err := CheckLongitude(maxLon); err != nil {
		return Rectangle{}, err
	}
	if maxLat < minLat {
		// Java enforces this via an `assert`. We mirror it as a
		// returned error to avoid the panic-vs-assertion mismatch
		// between Go and Java.
		return Rectangle{}, &invalidRectangleError{minLat: minLat, maxLat: maxLat}
	}
	return Rectangle{
		minLat: minLat,
		maxLat: maxLat,
		minLon: minLon,
		maxLon: maxLon,
	}, nil
}

// MustNewRectangle is the panic-on-error variant of NewRectangle for
// tests and package-init contexts.
func MustNewRectangle(minLat, maxLat, minLon, maxLon float64) Rectangle {
	r, err := NewRectangle(minLat, maxLat, minLon, maxLon)
	if err != nil {
		panic(err)
	}
	return r
}

// invalidRectangleError reports the maxLat < minLat case explicitly.
// It is unexported because callers can already detect the condition
// via errors.Is(err, ErrInvalidLatitude) for the bounds-check case;
// this error is reserved for the minLat/maxLat ordering violation.
type invalidRectangleError struct {
	minLat float64
	maxLat float64
}

func (e *invalidRectangleError) Error() string {
	return "geo: invalid rectangle: maxLat (" + formatJavaDouble(e.maxLat) +
		") < minLat (" + formatJavaDouble(e.minLat) + ")"
}

// MinLat returns the inclusive minimum latitude in decimal degrees.
func (r Rectangle) MinLat() float64 { return r.minLat }

// MaxLat returns the inclusive maximum latitude in decimal degrees.
func (r Rectangle) MaxLat() float64 { return r.maxLat }

// MinLon returns the inclusive minimum longitude in decimal degrees.
// MAY be greater than MaxLon for rectangles that cross the dateline.
func (r Rectangle) MinLon() float64 { return r.minLon }

// MaxLon returns the inclusive maximum longitude in decimal degrees.
func (r Rectangle) MaxLon() float64 { return r.maxLon }

// CrossesDateline reports whether the rectangle straddles the
// antimeridian (i.e. MaxLon < MinLon).
func (r Rectangle) CrossesDateline() bool { return r.maxLon < r.minLon }

// toComponent2D returns the Component2D for the rectangle. It
// satisfies the sealed Geometry interface.
//
// IMPORTANT: The Java original quantises the bounds via
// GeoEncodingUtils before constructing Rectangle2D (so that the
// resulting Component2D matches the cell layout used by
// LatLonPoint.newBoxQuery). The geo package will perform that
// quantisation as part of task #280 (GeoEncodingUtils). Until then,
// this method returns a Rectangle2D over the raw double bounds; the
// observable difference is at most 1 quantisation step (~1e-7°),
// which is below the precision of the inputs typically passed by
// users.
func (r Rectangle) toComponent2D() Component2D {
	if r.CrossesDateline() {
		return newMultiComponent2D([]Component2D{
			newRectangle2D(MinLonIncl, r.maxLon, r.minLat, r.maxLat),
			newRectangle2D(r.minLon, MaxLonIncl, r.minLat, r.maxLat),
		})
	}
	return newRectangle2D(r.minLon, r.maxLon, r.minLat, r.maxLat)
}

// latLonGeometry is the sealed marker on LatLonGeometry, satisfied
// here so Rectangle is a LatLonGeometry.
func (Rectangle) latLonGeometry() {}

// ContainsPoint reports whether the rectangle defined by
// (minLat, maxLat, minLon, maxLon) contains the (lat, lon) point.
//
// It is the static counterpart of org.apache.lucene.geo.Rectangle
// #containsPoint and is provided as a package-level helper for
// callers that do not have a Rectangle instance handy. The order of
// arguments matches the Java original.
func ContainsPoint(lat, lon, minLat, maxLat, minLon, maxLon float64) bool {
	return lat >= minLat && lat <= maxLat && lon >= minLon && lon <= maxLon
}

// Contains is the instance method counterpart of ContainsPoint. It
// honours dateline-crossing rectangles (where the longitude range is
// the union of [minLon, MaxLonIncl] and [MinLonIncl, maxLon]).
func (r Rectangle) Contains(lat, lon float64) bool {
	if lat < r.minLat || lat > r.maxLat {
		return false
	}
	if r.CrossesDateline() {
		return lon >= r.minLon || lon <= r.maxLon
	}
	return lon >= r.minLon && lon <= r.maxLon
}

// Equals reports whether two Rectangles have bit-identical bounds.
func (r Rectangle) Equals(o Rectangle) bool {
	return javaDoubleCompare(r.minLat, o.minLat) == 0 &&
		javaDoubleCompare(r.minLon, o.minLon) == 0 &&
		javaDoubleCompare(r.maxLat, o.maxLat) == 0 &&
		javaDoubleCompare(r.maxLon, o.maxLon) == 0
}

// HashCode mirrors Java's Rectangle.hashCode() exactly:
//
//	result := hash(minLat)
//	result = 31*result + hash(minLon)
//	result = 31*result + hash(maxLat)
//	result = 31*result + hash(maxLon)
func (r Rectangle) HashCode() int32 {
	result := javaDoubleHashCode(r.minLat)
	result = 31*result + javaDoubleHashCode(r.minLon)
	result = 31*result + javaDoubleHashCode(r.maxLat)
	result = 31*result + javaDoubleHashCode(r.maxLon)
	return result
}

// String mirrors Java's Rectangle.toString() verbatim, including the
// optional " [crosses dateline!]" suffix.
func (r Rectangle) String() string {
	var b []byte
	b = append(b, "Rectangle(lat="...)
	b = appendJavaDouble(b, r.minLat)
	b = append(b, " TO "...)
	b = appendJavaDouble(b, r.maxLat)
	b = append(b, " lon="...)
	b = appendJavaDouble(b, r.minLon)
	b = append(b, " TO "...)
	b = appendJavaDouble(b, r.maxLon)
	if r.CrossesDateline() {
		b = append(b, " [crosses dateline!]"...)
	}
	b = append(b, ')')
	return string(b)
}

// FromPointDistance computes the WGS-84 bounding box of a spherical
// cap of radius radiusMeters centred at (centerLat, centerLon). It is
// the Go port of Rectangle.fromPointDistance.
//
// The implementation uses util.Sin / util.Cos / util.Asin, which are
// the Lucene SloppyMath approximations and therefore match the Java
// reference bit-for-bit (the SloppyMath tables are deterministic). It
// returns the centre's polar bounding box when the disk overlaps a
// pole, matching the Java behaviour.
func FromPointDistance(centerLat, centerLon, radiusMeters float64) (Rectangle, error) {
	if err := CheckLatitude(centerLat); err != nil {
		return Rectangle{}, err
	}
	if err := CheckLongitude(centerLon); err != nil {
		return Rectangle{}, err
	}
	radLat := centerLat * math.Pi / 180
	radLon := centerLon * math.Pi / 180
	// LUCENE-7143: add a tiny epsilon to the radius before
	// converting to radians, to compensate for the worst-case error
	// of the SloppyMath approximations downstream.
	radDistance := (radiusMeters + 7e-2) / EarthMeanRadiusMeters
	minLatR := radLat - radDistance
	maxLatR := radLat + radDistance

	var minLonR, maxLonR float64
	if minLatR > MinLatRadians && maxLatR < MaxLatRadians {
		deltaLon := util.Asin(util.Sin(radDistance) / util.Cos(radLat))
		minLonR = radLon - deltaLon
		if minLonR < MinLonRadians {
			minLonR += 2 * math.Pi
		}
		maxLonR = radLon + deltaLon
		if maxLonR > MaxLonRadians {
			maxLonR -= 2 * math.Pi
		}
	} else {
		// Disk overlaps a pole — bbox becomes a full lon ring,
		// clamped on latitude.
		minLatR = math.Max(minLatR, MinLatRadians)
		maxLatR = math.Min(maxLatR, MaxLatRadians)
		minLonR = MinLonRadians
		maxLonR = MaxLonRadians
	}

	return NewRectangle(
		minLatR*180/math.Pi,
		maxLatR*180/math.Pi,
		minLonR*180/math.Pi,
		maxLonR*180/math.Pi,
	)
}

// AxisLatError is the maximum error of AxisLat, expressed in
// degrees. It mirrors Rectangle.AXISLAT_ERROR.
var AxisLatError = (0.1 / EarthMeanRadiusMeters) * 180 / math.Pi

// AxisLat computes the latitude at which a sphere of radius
// radiusMeters centred at centerLat intersects the meridians of its
// own bounding box.
//
// Returned value is within AxisLatError of the true value. Mirrors
// Rectangle.axisLat.
func AxisLat(centerLat, radiusMeters float64) float64 {
	const PIO2 = math.Pi / 2
	l1 := centerLat * math.Pi / 180
	r := (radiusMeters + 7e-2) / EarthMeanRadiusMeters

	if math.Abs(l1)+r >= MaxLatRadians {
		if centerLat >= 0 {
			return MaxLatIncl
		}
		return MinLatIncl
	}

	if centerLat >= 0 {
		l1 = PIO2 - l1
	} else {
		l1 = l1 + PIO2
	}

	l2 := math.Acos(math.Cos(l1) / math.Cos(r))

	if centerLat >= 0 {
		l2 = PIO2 - l2
	} else {
		l2 = l2 - PIO2
	}

	return l2 * 180 / math.Pi
}
