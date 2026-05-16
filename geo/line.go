// Code in this file mirrors org.apache.lucene.geo.Line from Apache
// Lucene 10.4.0.

package geo

import (
	"errors"
	"strconv"
)

// Line is a poly-line on Earth's surface, defined as a sequence of
// (lat, lon) vertices joined by great-circle (or, for Component2D
// purposes, straight) segments.
//
// It is the Go port of org.apache.lucene.geo.Line. A Line is
// immutable and self-validating: the constructor rejects mismatched
// or under-length input slices, NaN/out-of-range coordinates, and
// nil arrays.
type Line struct {
	lats   []float64
	lons   []float64
	minLat float64
	maxLat float64
	minLon float64
	maxLon float64
}

// Sentinel errors mirroring the Java IllegalArgumentException
// messages.
var (
	// ErrNilLats is returned by NewLine when the latitude slice is
	// nil.
	ErrNilLats = errors.New("geo: lats must not be null")

	// ErrNilLons is returned by NewLine when the longitude slice is
	// nil.
	ErrNilLons = errors.New("geo: lons must not be null")

	// ErrLatLonLengthMismatch is returned by NewLine when the
	// latitude and longitude slices have different lengths.
	ErrLatLonLengthMismatch = errors.New("geo: lats and lons must be equal length")

	// ErrTooFewLinePoints is returned by NewLine when fewer than two
	// vertices are supplied.
	ErrTooFewLinePoints = errors.New("geo: at least 2 line points required")
)

// NewLine constructs a Line from parallel latitude and longitude
// slices. Both must be non-nil, of equal length, and contain at
// least two points; every point is validated against the inclusive
// lat/lon bounds. Internal copies are made of both slices so the
// returned Line is immune to caller-side mutation.
func NewLine(lats, lons []float64) (Line, error) {
	if lats == nil {
		return Line{}, ErrNilLats
	}
	if lons == nil {
		return Line{}, ErrNilLons
	}
	if len(lats) != len(lons) {
		return Line{}, ErrLatLonLengthMismatch
	}
	if len(lats) < 2 {
		return Line{}, ErrTooFewLinePoints
	}

	minLat, maxLat := lats[0], lats[0]
	minLon, maxLon := lons[0], lons[0]
	for i := 0; i < len(lats); i++ {
		if err := CheckLatitude(lats[i]); err != nil {
			return Line{}, err
		}
		if err := CheckLongitude(lons[i]); err != nil {
			return Line{}, err
		}
		if lats[i] < minLat {
			minLat = lats[i]
		}
		if lats[i] > maxLat {
			maxLat = lats[i]
		}
		if lons[i] < minLon {
			minLon = lons[i]
		}
		if lons[i] > maxLon {
			maxLon = lons[i]
		}
	}

	// Defensive copies — callers must not be able to mutate the
	// line's internal state.
	la := make([]float64, len(lats))
	copy(la, lats)
	lo := make([]float64, len(lons))
	copy(lo, lons)
	return Line{
		lats:   la,
		lons:   lo,
		minLat: minLat,
		maxLat: maxLat,
		minLon: minLon,
		maxLon: maxLon,
	}, nil
}

// MustNewLine is the panic-on-error variant of NewLine for tests
// and package-init contexts.
func MustNewLine(lats, lons []float64) Line {
	l, err := NewLine(lats, lons)
	if err != nil {
		panic(err)
	}
	return l
}

// NumPoints returns the number of vertices on the line.
func (l Line) NumPoints() int { return len(l.lats) }

// Lat returns the latitude of the vertex at the given index.
// Panics on out-of-bounds, matching the Java unchecked behaviour
// (Java throws ArrayIndexOutOfBoundsException).
func (l Line) Lat(vertex int) float64 { return l.lats[vertex] }

// Lon returns the longitude of the vertex at the given index.
func (l Line) Lon(vertex int) float64 { return l.lons[vertex] }

// Lats returns a copy of the latitude slice. The Java method
// returns a clone of the internal array; we mirror that to
// prevent external mutation.
func (l Line) Lats() []float64 {
	out := make([]float64, len(l.lats))
	copy(out, l.lats)
	return out
}

// Lons returns a copy of the longitude slice.
func (l Line) Lons() []float64 {
	out := make([]float64, len(l.lons))
	copy(out, l.lons)
	return out
}

// MinLat returns the inclusive minimum latitude of the line's
// bounding box.
func (l Line) MinLat() float64 { return l.minLat }

// MaxLat returns the inclusive maximum latitude of the line's
// bounding box.
func (l Line) MaxLat() float64 { return l.maxLat }

// MinLon returns the inclusive minimum longitude of the line's
// bounding box.
func (l Line) MinLon() float64 { return l.minLon }

// MaxLon returns the inclusive maximum longitude of the line's
// bounding box.
func (l Line) MaxLon() float64 { return l.maxLon }

// toComponent2D returns the Line2D Component for the line.
func (l Line) toComponent2D() Component2D {
	return newLine2DFromLine(l)
}

// latLonGeometry is the sealed marker on LatLonGeometry.
func (Line) latLonGeometry() {}

// Equals reports whether two Lines have identical vertex arrays
// (lat by lat, lon by lon). Vertex equality follows Java's
// Double.compare semantics on the bit pattern.
func (l Line) Equals(o Line) bool {
	if len(l.lats) != len(o.lats) || len(l.lons) != len(o.lons) {
		return false
	}
	for i := range l.lats {
		if javaDoubleCompare(l.lats[i], o.lats[i]) != 0 {
			return false
		}
	}
	for i := range l.lons {
		if javaDoubleCompare(l.lons[i], o.lons[i]) != 0 {
			return false
		}
	}
	return true
}

// HashCode mirrors Java's Line.hashCode():
//
//	result = Arrays.hashCode(lats)
//	result = 31 * result + Arrays.hashCode(lons)
//
// Arrays.hashCode(double[]) is defined as iterating with
// result = 31*result + Double.hashCode(element), starting from 1.
func (l Line) HashCode() int32 {
	return 31*javaDoubleArrayHashCode(l.lats) + javaDoubleArrayHashCode(l.lons)
}

// javaDoubleArrayHashCode mirrors java.util.Arrays.hashCode for
// double[]. The seed value 1 is the Java convention.
func javaDoubleArrayHashCode(a []float64) int32 {
	result := int32(1)
	for _, v := range a {
		result = 31*result + javaDoubleHashCode(v)
	}
	return result
}

// String mirrors Java's Line.toString() verbatim, including the
// [lon, lat] vertex ordering used in the Java original.
func (l Line) String() string {
	b := make([]byte, 0, 16+len(l.lats)*32)
	b = append(b, "Line("...)
	for i := range l.lats {
		b = append(b, '[')
		b = appendJavaDouble(b, l.lons[i])
		b = append(b, ", "...)
		b = appendJavaDouble(b, l.lats[i])
		b = append(b, ']')
	}
	b = append(b, ')')
	return string(b)
}

// numPointsString returns the digit-count of n; tiny helper kept
// local to avoid pulling in strconv for the String fast path.
//
//nolint:unused // future scaffolding for an optimised String builder.
func numPointsString(n int) int { return len(strconv.Itoa(n)) }
