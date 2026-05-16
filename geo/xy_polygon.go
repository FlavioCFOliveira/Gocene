// Code in this file mirrors org.apache.lucene.geo.XYPolygon from
// Apache Lucene 10.4.0.

package geo

import (
	"errors"
	"fmt"
)

// XYPolygon is a closed simple polygon in cartesian (x, y) space.
// It is the Go port of org.apache.lucene.geo.XYPolygon.
type XYPolygon struct {
	xs           []float32
	ys           []float32
	holes        []XYPolygon
	minX         float32
	maxX         float32
	minY         float32
	maxY         float32
	windingOrder WindingOrder
}

// Sentinel errors mirroring the Java IllegalArgumentException
// messages.
var (
	// ErrNilXYPolygonX is returned when the x slice is nil.
	ErrNilXYPolygonX = errors.New("geo: x must not be null")
	// ErrNilXYPolygonY is returned when the y slice is nil.
	ErrNilXYPolygonY = errors.New("geo: y must not be null")
	// ErrXYPolygonLengthMismatch is returned when x and y differ in
	// length.
	ErrXYPolygonLengthMismatch = errors.New("geo: x and y must be equal length")
	// ErrTooFewXYPolygonPoints is returned when fewer than four
	// vertices are supplied.
	ErrTooFewXYPolygonPoints = errors.New("geo: at least 4 polygon points required")
	// ErrXYHolesContainHoles is returned when any hole has its own
	// holes.
	ErrXYHolesContainHoles = errors.New("geo: holes may not contain holes: polygons may not nest")
)

// notClosedXYError reports the open-polygon case for XYPolygon.
type notClosedXYError struct {
	axis     string
	first    float32
	lastIdx  int
	lastVal  float32
}

func (e *notClosedXYError) Error() string {
	return fmt.Sprintf("geo: first and last points of the polygon must be the same "+
		"(it must close itself): %s[0]=%s %s[%d]=%s",
		e.axis, formatJavaFloat(e.first), e.axis, e.lastIdx, formatJavaFloat(e.lastVal))
}

// NewXYPolygon constructs an XYPolygon. All Java-original
// validations are reproduced as returned errors.
func NewXYPolygon(xs, ys []float32, holes ...XYPolygon) (XYPolygon, error) {
	if xs == nil {
		return XYPolygon{}, ErrNilXYPolygonX
	}
	if ys == nil {
		return XYPolygon{}, ErrNilXYPolygonY
	}
	if len(xs) != len(ys) {
		return XYPolygon{}, ErrXYPolygonLengthMismatch
	}
	if len(xs) < 4 {
		return XYPolygon{}, ErrTooFewXYPolygonPoints
	}
	if xs[0] != xs[len(xs)-1] {
		return XYPolygon{}, &notClosedXYError{
			axis: "x", first: xs[0],
			lastIdx: len(xs) - 1, lastVal: xs[len(xs)-1],
		}
	}
	if ys[0] != ys[len(ys)-1] {
		return XYPolygon{}, &notClosedXYError{
			axis: "y", first: ys[0],
			lastIdx: len(ys) - 1, lastVal: ys[len(ys)-1],
		}
	}
	for i := range holes {
		if holes[i].NumHoles() > 0 {
			return XYPolygon{}, ErrXYHolesContainHoles
		}
	}
	for i := 0; i < len(xs); i++ {
		if _, err := XYCheckVal(xs[i]); err != nil {
			return XYPolygon{}, err
		}
		if _, err := XYCheckVal(ys[i]); err != nil {
			return XYPolygon{}, err
		}
	}

	x2 := make([]float32, len(xs))
	y2 := make([]float32, len(ys))
	copy(x2, xs)
	copy(y2, ys)
	hl := make([]XYPolygon, len(holes))
	copy(hl, holes)

	minX, maxX := x2[0], x2[0]
	minY, maxY := y2[0], y2[0]
	windingSum := 0.0
	numPts := len(x2) - 1
	for i, j := 1, 0; i < numPts; j, i = i, i+1 {
		if x2[i] < minX {
			minX = x2[i]
		}
		if x2[i] > maxX {
			maxX = x2[i]
		}
		if y2[i] < minY {
			minY = y2[i]
		}
		if y2[i] > maxY {
			maxY = y2[i]
		}
		windingSum += (float64(x2[j])-float64(x2[numPts]))*(float64(y2[i])-float64(y2[numPts])) -
			(float64(y2[j])-float64(y2[numPts]))*(float64(x2[i])-float64(x2[numPts]))
	}
	wo := WindingClockwise
	if windingSum < 0 {
		wo = WindingCounterClockwise
	}
	return XYPolygon{
		xs: x2, ys: y2, holes: hl,
		minX: minX, maxX: maxX,
		minY: minY, maxY: maxY,
		windingOrder: wo,
	}, nil
}

// MustNewXYPolygon is the panic-on-error variant.
func MustNewXYPolygon(xs, ys []float32, holes ...XYPolygon) XYPolygon {
	p, err := NewXYPolygon(xs, ys, holes...)
	if err != nil {
		panic(err)
	}
	return p
}

// NumPoints returns the vertex count.
func (p XYPolygon) NumPoints() int { return len(p.xs) }

// PolyX returns the x coordinate of the vertex at the given index.
func (p XYPolygon) PolyX(vertex int) float32 { return p.xs[vertex] }

// PolyY returns the y coordinate of the vertex at the given index.
func (p XYPolygon) PolyY(vertex int) float32 { return p.ys[vertex] }

// PolyXs / PolyYs return defensive copies of the vertex arrays.
func (p XYPolygon) PolyXs() []float32 {
	out := make([]float32, len(p.xs))
	copy(out, p.xs)
	return out
}

func (p XYPolygon) PolyYs() []float32 {
	out := make([]float32, len(p.ys))
	copy(out, p.ys)
	return out
}

// Holes returns a defensive copy of the polygon's holes.
func (p XYPolygon) Holes() []XYPolygon {
	out := make([]XYPolygon, len(p.holes))
	copy(out, p.holes)
	return out
}

// Hole returns the hole at the given index.
func (p XYPolygon) Hole(i int) XYPolygon { return p.holes[i] }

// NumHoles returns the number of holes.
func (p XYPolygon) NumHoles() int { return len(p.holes) }

// WindingOrder returns the winding order of the polygon shell.
func (p XYPolygon) WindingOrder() WindingOrder { return p.windingOrder }

// MinX / MaxX / MinY / MaxY accessors.
func (p XYPolygon) MinX() float32 { return p.minX }
func (p XYPolygon) MaxX() float32 { return p.maxX }
func (p XYPolygon) MinY() float32 { return p.minY }
func (p XYPolygon) MaxY() float32 { return p.maxY }

// toComponent2D returns the Polygon2D Component2D widened to
// float64.
func (p XYPolygon) toComponent2D() Component2D {
	xs := make([]float64, len(p.xs))
	ys := make([]float64, len(p.ys))
	for i := range p.xs {
		xs[i] = float64(p.xs[i])
		ys[i] = float64(p.ys[i])
	}
	var holeComponents []*polygon2D
	if len(p.holes) > 0 {
		holeComponents = make([]*polygon2D, len(p.holes))
		for i := range p.holes {
			h := p.holes[i].toComponent2D().(*polygon2D)
			holeComponents[i] = h
		}
	}
	return newPolygon2DFromXY(xs, ys, holeComponents)
}

// xyGeometry is the sealed marker on XYGeometry.
func (XYPolygon) xyGeometry() {}

// Equals reports whether two polygons are structurally identical.
func (p XYPolygon) Equals(o XYPolygon) bool {
	if !equalsFloat32Slice(p.xs, o.xs) || !equalsFloat32Slice(p.ys, o.ys) {
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

func equalsFloat32Slice(a, b []float32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if javaFloatCompare(a[i], b[i]) != 0 {
			return false
		}
	}
	return true
}

// HashCode mirrors Java's XYPolygon.hashCode().
func (p XYPolygon) HashCode() int32 {
	result := int32(1)
	holesHash := int32(1)
	for i := range p.holes {
		holesHash = 31*holesHash + p.holes[i].HashCode()
	}
	result = 31*result + holesHash
	result = 31*result + javaFloat32ArrayHashCode(p.xs)
	result = 31*result + javaFloat32ArrayHashCode(p.ys)
	return result
}

// String mirrors Java's XYPolygon.toString().
func (p XYPolygon) String() string {
	b := make([]byte, 0, 16+len(p.xs)*16)
	b = append(b, "XYPolygon"...)
	for i := range p.xs {
		b = append(b, '[')
		b = appendJavaFloat(b, p.xs[i])
		b = append(b, ", "...)
		b = appendJavaFloat(b, p.ys[i])
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

// XYVerticesToGeoJSON renders the [[x, y], ...] coordinate array for
// a pair of parallel float32 slices, mirroring Java's static helper
// of the same name.
func XYVerticesToGeoJSON(xs, ys []float32) string {
	b := make([]byte, 0, 2+len(xs)*16)
	b = append(b, '[')
	for i := range xs {
		b = append(b, '[')
		b = appendJavaFloat(b, xs[i])
		b = append(b, ", "...)
		b = appendJavaFloat(b, ys[i])
		b = append(b, ']')
		if i != len(xs)-1 {
			b = append(b, ", "...)
		}
	}
	b = append(b, ']')
	return string(b)
}
