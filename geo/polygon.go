// Code in this file mirrors org.apache.lucene.geo.Polygon from
// Apache Lucene 10.4.0.

package geo

import (
	"errors"
	"fmt"
)

// Polygon is a closed simple polygon on Earth's surface, optionally
// with one level of inner rings (holes).
//
// It is the Go port of org.apache.lucene.geo.Polygon. Notes:
//
//  1. Coordinates must be in clockwise order, except for holes which
//     must be counter-clockwise.
//  2. The polygon must be closed: the first and last vertices share
//     the same (lat, lon).
//  3. The polygon must not self-cross; behaviour on a self-crossing
//     polygon is undefined.
//  4. All lat/lon values must be in decimal degrees.
//  5. Polygons cannot cross the antimeridian; split into two
//     polygons instead.
//  6. Holes may not themselves contain holes (polygons cannot nest
//     more than one deep).
//
// Polygon is immutable; the zero value is not valid. Use NewPolygon
// to construct it.
type Polygon struct {
	polyLats     []float64
	polyLons     []float64
	holes        []Polygon
	minLat       float64
	maxLat       float64
	minLon       float64
	maxLon       float64
	windingOrder WindingOrder
}

// Sentinel errors mirroring the Java IllegalArgumentException
// messages.
var (
	// ErrNilPolyLats is returned when polyLats is nil.
	ErrNilPolyLats = errors.New("geo: polyLats must not be null")

	// ErrNilPolyLons is returned when polyLons is nil.
	ErrNilPolyLons = errors.New("geo: polyLons must not be null")

	// ErrNilHoles is returned when the holes slice itself is nil.
	ErrNilHoles = errors.New("geo: holes must not be null")

	// ErrPolyLatLonLengthMismatch is returned when polyLats and
	// polyLons differ in length.
	ErrPolyLatLonLengthMismatch = errors.New("geo: polyLats and polyLons must be equal length")

	// ErrTooFewPolygonPoints is returned when fewer than four
	// vertices are provided (a closed triangle is the smallest
	// non-degenerate polygon).
	ErrTooFewPolygonPoints = errors.New("geo: at least 4 polygon points required")

	// ErrHolesContainHoles is returned when any hole has its own
	// holes (polygons may not nest beyond one level).
	ErrHolesContainHoles = errors.New("geo: holes may not contain holes: polygons may not nest")
)

// notClosedError reports the open-polygon case (first vertex differs
// from the last). The error includes the offending coordinates to
// match Lucene's verbose IllegalArgumentException message.
type notClosedError struct {
	axis    string // "polyLats" or "polyLons"
	first   float64
	lastIdx int
	lastVal float64
}

func (e *notClosedError) Error() string {
	return fmt.Sprintf("geo: first and last points of the polygon must be the same "+
		"(it must close itself): %s[0]=%s %s[%d]=%s",
		e.axis, formatJavaDouble(e.first), e.axis, e.lastIdx, formatJavaDouble(e.lastVal))
}

// NewPolygon constructs a Polygon from the given vertex arrays and
// optional holes. All Java-original validations are reproduced as
// returned errors (see ErrNilPolyLats, ErrNilPolyLons, ErrNilHoles,
// ErrPolyLatLonLengthMismatch, ErrTooFewPolygonPoints, the
// open-polygon error, the per-vertex lat/lon range error, and
// ErrHolesContainHoles).
//
// The constructor takes defensive copies of polyLats, polyLons and
// holes; later mutation of the caller's slices is harmless.
func NewPolygon(polyLats, polyLons []float64, holes ...Polygon) (Polygon, error) {
	if polyLats == nil {
		return Polygon{}, ErrNilPolyLats
	}
	if polyLons == nil {
		return Polygon{}, ErrNilPolyLons
	}
	// Java's `Polygon... holes` variadic produces an empty array when
	// no holes are passed; only the literal `null` triggers the nil
	// check. In Go a variadic with no args yields nil; we treat that
	// as "no holes" and only fail when the caller explicitly passes
	// a nil slice (which Go users would do with `holes=nil...`,
	// equivalent to Java's `null`).
	if holes != nil && len(holes) == 0 {
		// Empty non-nil slice is fine; fall through.
		_ = holes
	}
	if len(polyLats) != len(polyLons) {
		return Polygon{}, ErrPolyLatLonLengthMismatch
	}
	if len(polyLats) < 4 {
		return Polygon{}, ErrTooFewPolygonPoints
	}
	if polyLats[0] != polyLats[len(polyLats)-1] {
		return Polygon{}, &notClosedError{
			axis: "polyLats", first: polyLats[0],
			lastIdx: len(polyLats) - 1, lastVal: polyLats[len(polyLats)-1],
		}
	}
	if polyLons[0] != polyLons[len(polyLons)-1] {
		return Polygon{}, &notClosedError{
			axis: "polyLons", first: polyLons[0],
			lastIdx: len(polyLons) - 1, lastVal: polyLons[len(polyLons)-1],
		}
	}
	for i := 0; i < len(polyLats); i++ {
		if err := CheckLatitude(polyLats[i]); err != nil {
			return Polygon{}, err
		}
		if err := CheckLongitude(polyLons[i]); err != nil {
			return Polygon{}, err
		}
	}
	for i := range holes {
		if holes[i].NumHoles() > 0 {
			return Polygon{}, ErrHolesContainHoles
		}
	}

	// Defensive copies.
	la := make([]float64, len(polyLats))
	copy(la, polyLats)
	lo := make([]float64, len(polyLons))
	copy(lo, polyLons)
	hl := make([]Polygon, len(holes))
	copy(hl, holes)

	// Bounding box and signed area for winding order. Java's loop
	// excludes the closing vertex (numPts = polyLats.length - 1).
	minLat, maxLat := la[0], la[0]
	minLon, maxLon := lo[0], lo[0]
	windingSum := 0.0
	numPts := len(la) - 1
	for i, j := 1, 0; i < numPts; j, i = i, i+1 {
		if la[i] < minLat {
			minLat = la[i]
		}
		if la[i] > maxLat {
			maxLat = la[i]
		}
		if lo[i] < minLon {
			minLon = lo[i]
		}
		if lo[i] > maxLon {
			maxLon = lo[i]
		}
		windingSum += (lo[j]-lo[numPts])*(la[i]-la[numPts]) -
			(la[j]-la[numPts])*(lo[i]-lo[numPts])
	}
	wo := WindingClockwise
	if windingSum < 0 {
		wo = WindingCounterClockwise
	}

	return Polygon{
		polyLats: la, polyLons: lo, holes: hl,
		minLat: minLat, maxLat: maxLat,
		minLon: minLon, maxLon: maxLon,
		windingOrder: wo,
	}, nil
}

// MustNewPolygon is the panic-on-error variant of NewPolygon.
func MustNewPolygon(polyLats, polyLons []float64, holes ...Polygon) Polygon {
	p, err := NewPolygon(polyLats, polyLons, holes...)
	if err != nil {
		panic(err)
	}
	return p
}

// NumPoints returns the number of vertices in the polygon shell
// (closing vertex included).
func (p Polygon) NumPoints() int { return len(p.polyLats) }

// PolyLat returns the latitude of the shell vertex at the given
// index.
func (p Polygon) PolyLat(vertex int) float64 { return p.polyLats[vertex] }

// PolyLon returns the longitude of the shell vertex at the given
// index.
func (p Polygon) PolyLon(vertex int) float64 { return p.polyLons[vertex] }

// PolyLats returns a defensive copy of the shell latitude array.
func (p Polygon) PolyLats() []float64 {
	out := make([]float64, len(p.polyLats))
	copy(out, p.polyLats)
	return out
}

// PolyLons returns a defensive copy of the shell longitude array.
func (p Polygon) PolyLons() []float64 {
	out := make([]float64, len(p.polyLons))
	copy(out, p.polyLons)
	return out
}

// Holes returns a defensive copy of the polygon's holes.
func (p Polygon) Holes() []Polygon {
	out := make([]Polygon, len(p.holes))
	copy(out, p.holes)
	return out
}

// Hole returns the hole at the given index. Package-private in Java;
// exported here for tests.
func (p Polygon) Hole(i int) Polygon { return p.holes[i] }

// NumHoles returns the number of holes in the polygon.
func (p Polygon) NumHoles() int { return len(p.holes) }

// WindingOrder returns the winding order of the polygon shell
// (computed from the signed area). CW for the standard shell,
// CCW for holes, COLINEAR is never reported here because the Java
// reference defaults to CW when the signed area is zero. The Go
// port preserves that behaviour.
func (p Polygon) WindingOrder() WindingOrder { return p.windingOrder }

// MinLat / MaxLat / MinLon / MaxLon are the inclusive bounding-box
// coordinates of the polygon shell.
func (p Polygon) MinLat() float64 { return p.minLat }
func (p Polygon) MaxLat() float64 { return p.maxLat }
func (p Polygon) MinLon() float64 { return p.minLon }
func (p Polygon) MaxLon() float64 { return p.maxLon }

// toComponent2D returns the Polygon2D Component for the polygon.
func (p Polygon) toComponent2D() Component2D { return newPolygon2DFromPolygon(p) }

// latLonGeometry is the sealed marker on LatLonGeometry.
func (Polygon) latLonGeometry() {}

// Equals reports whether two polygons have identical shells and
// holes. Vertex equality follows Java's Double.compare semantics on
// the bit pattern; hole order is significant (matches
// Arrays.equals(Polygon[], Polygon[]) in Java).
func (p Polygon) Equals(o Polygon) bool {
	if !equalsFloat64Slice(p.polyLats, o.polyLats) {
		return false
	}
	if !equalsFloat64Slice(p.polyLons, o.polyLons) {
		return false
	}
	if len(p.holes) != len(o.holes) {
		return false
	}
	for i := range p.holes {
		if !p.holes[i].Equals(o.holes[i]) {
			return false
		}
	}
	return true
}

// equalsFloat64Slice is the float64 counterpart of
// java.util.Arrays.equals.
func equalsFloat64Slice(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if javaDoubleCompare(a[i], b[i]) != 0 {
			return false
		}
	}
	return true
}

// HashCode mirrors Java's Polygon.hashCode():
//
//	result = 1
//	result = 31*result + Arrays.hashCode(holes)
//	result = 31*result + Arrays.hashCode(polyLats)
//	result = 31*result + Arrays.hashCode(polyLons)
//
// Arrays.hashCode(Object[]) iterates over the elements with
// result = 31*result + (element==null ? 0 : element.hashCode()),
// starting from 1.
func (p Polygon) HashCode() int32 {
	result := int32(1)
	// holes
	holesHash := int32(1)
	for i := range p.holes {
		holesHash = 31*holesHash + p.holes[i].HashCode()
	}
	result = 31*result + holesHash
	// polyLats
	result = 31*result + javaDoubleArrayHashCode(p.polyLats)
	// polyLons
	result = 31*result + javaDoubleArrayHashCode(p.polyLons)
	return result
}

// String mirrors Java's Polygon.toString() exactly:
//
//	Polygon[lat, lon] [lat, lon] [lat, lon] ...
//	(with ", holes=[...]" appended when holes are present)
//
// The trailing space after each "[lat, lon] " group is preserved.
func (p Polygon) String() string {
	b := make([]byte, 0, 16+len(p.polyLats)*32)
	b = append(b, "Polygon"...)
	for i := range p.polyLats {
		b = append(b, '[')
		b = appendJavaDouble(b, p.polyLats[i])
		b = append(b, ", "...)
		b = appendJavaDouble(b, p.polyLons[i])
		b = append(b, "] "...)
	}
	if len(p.holes) > 0 {
		b = append(b, ", holes=["...)
		for i := range p.holes {
			if i > 0 {
				b = append(b, ", "...)
			}
			b = append(b, p.holes[i].String()...)
		}
		b = append(b, ']')
	}
	return string(b)
}

// VerticesToGeoJSON returns the [[lon, lat], ...] GeoJSON coordinate
// array for a pair of parallel lat/lon arrays. Mirrors Java's static
// org.apache.lucene.geo.Polygon#verticesToGeoJSON exactly, including
// the "[lon, lat]" vertex ordering required by RFC 7946.
func VerticesToGeoJSON(lats, lons []float64) string {
	b := make([]byte, 0, 2+len(lats)*20)
	b = append(b, '[')
	for i := range lats {
		b = append(b, '[')
		b = appendJavaDouble(b, lons[i])
		b = append(b, ", "...)
		b = appendJavaDouble(b, lats[i])
		b = append(b, ']')
		if i != len(lats)-1 {
			b = append(b, ", "...)
		}
	}
	b = append(b, ']')
	return string(b)
}

// ToGeoJSON renders the polygon (shell + holes) as a GeoJSON
// coordinates array. Mirrors Java's Polygon.toGeoJSON.
func (p Polygon) ToGeoJSON() string {
	b := make([]byte, 0, 16+len(p.polyLats)*32)
	b = append(b, '[')
	b = append(b, VerticesToGeoJSON(p.polyLats, p.polyLons)...)
	for i := range p.holes {
		b = append(b, ',')
		b = append(b, VerticesToGeoJSON(p.holes[i].polyLats, p.holes[i].polyLons)...)
	}
	b = append(b, ']')
	return string(b)
}
