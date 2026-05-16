// Code in this file mirrors org.apache.lucene.geo.Circle from Apache
// Lucene 10.4.0.

package geo

import (
	"errors"
	"fmt"
	"math"
)

// Circle is a geographic circle on Earth's surface, defined by a
// (lat, lon) centre and a radius in metres.
//
// It is the Go port of org.apache.lucene.geo.Circle. The circle is
// evaluated on the spherical Earth model; for very large circles or
// circles near the poles, polygons may give better results.
// Dateline-crossing is handled by the Component2D representation.
//
// Circle is immutable; the zero value is not valid. Use NewCircle to
// construct it.
type Circle struct {
	lat          float64
	lon          float64
	radiusMeters float64
}

// ErrInvalidRadius is the sentinel returned by NewCircle when the
// radius is non-finite or negative. It mirrors the Java
// IllegalArgumentException message format
// `radiusMeters: 'X' is invalid`, where X is the offending value
// rendered in Java Double.toString form.
var ErrInvalidRadius = errors.New("geo: invalid radius")

// NewCircle constructs a Circle. It validates the centre coordinates
// against the inclusive lat/lon bounds and the radius against the
// finite, non-negative range Lucene enforces. NaN, +Inf, -Inf and
// negative radii are all rejected.
func NewCircle(lat, lon, radiusMeters float64) (Circle, error) {
	if err := CheckLatitude(lat); err != nil {
		return Circle{}, err
	}
	if err := CheckLongitude(lon); err != nil {
		return Circle{}, err
	}
	if !isFinite(radiusMeters) || radiusMeters < 0 {
		return Circle{}, fmt.Errorf("%w radiusMeters: '%s' is invalid",
			ErrInvalidRadius, formatJavaDouble(radiusMeters))
	}
	return Circle{lat: lat, lon: lon, radiusMeters: radiusMeters}, nil
}

// MustNewCircle is the panic-on-error variant of NewCircle.
func MustNewCircle(lat, lon, radiusMeters float64) Circle {
	c, err := NewCircle(lat, lon, radiusMeters)
	if err != nil {
		panic(err)
	}
	return c
}

// isFinite reports whether v is a finite number (not NaN and not
// +/-Inf). Equivalent to Java's Double.isFinite.
func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

// Lat returns the latitude of the centre in decimal degrees.
func (c Circle) Lat() float64 { return c.lat }

// Lon returns the longitude of the centre in decimal degrees.
func (c Circle) Lon() float64 { return c.lon }

// Radius returns the circle's radius in metres.
func (c Circle) Radius() float64 { return c.radiusMeters }

// toComponent2D returns the Component2D for the circle. The
// implementation uses haversin distance to test point inclusion,
// matching Lucene's HaversinDistance strategy.
func (c Circle) toComponent2D() Component2D {
	return newCircle2DFromCircle(c)
}

// latLonGeometry is the sealed marker on LatLonGeometry.
func (Circle) latLonGeometry() {}

// Equals reports whether two Circles have bit-identical centre and
// radius. Follows Java's Double.compare semantics.
func (c Circle) Equals(o Circle) bool {
	return javaDoubleCompare(c.lat, o.lat) == 0 &&
		javaDoubleCompare(c.lon, o.lon) == 0 &&
		javaDoubleCompare(c.radiusMeters, o.radiusMeters) == 0
}

// HashCode mirrors Java's Circle.hashCode():
//
//	result = hash(lat)
//	result = 31*result + hash(lon)
//	result = 31*result + hash(radius)
func (c Circle) HashCode() int32 {
	result := javaDoubleHashCode(c.lat)
	result = 31*result + javaDoubleHashCode(c.lon)
	result = 31*result + javaDoubleHashCode(c.radiusMeters)
	return result
}

// String mirrors Java's Circle.toString():
//
//	"Circle([lat,lon] radius = R meters)"
func (c Circle) String() string {
	var b []byte
	b = append(b, "Circle("...)
	b = append(b, '[')
	b = appendJavaDouble(b, c.lat)
	b = append(b, ',')
	b = appendJavaDouble(b, c.lon)
	b = append(b, ']')
	b = append(b, " radius = "...)
	b = appendJavaDouble(b, c.radiusMeters)
	b = append(b, " meters)"...)
	return string(b)
}
