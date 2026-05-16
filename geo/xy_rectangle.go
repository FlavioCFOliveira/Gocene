// Code in this file mirrors org.apache.lucene.geo.XYRectangle from
// Apache Lucene 10.4.0.

package geo

import (
	"errors"
	"fmt"
	"math"
)

// XYRectangle is an axis-aligned bounding box in cartesian (x, y)
// space. It is the Go port of org.apache.lucene.geo.XYRectangle.
//
// Immutable; the zero value is not valid. Use NewXYRectangle to
// construct it.
type XYRectangle struct {
	minX float32
	maxX float32
	minY float32
	maxY float32
}

// Sentinel errors mirroring the Java IllegalArgumentException
// messages.
var (
	// ErrInvalidXYRectangleBounds is returned when minX > maxX or
	// minY > maxY.
	ErrInvalidXYRectangleBounds = errors.New("geo: invalid XY rectangle bounds")
)

// NewXYRectangle constructs an XYRectangle. minX must be <= maxX
// and minY must be <= maxY; each corner is validated via
// XYCheckVal.
func NewXYRectangle(minX, maxX, minY, maxY float32) (XYRectangle, error) {
	if minX > maxX {
		return XYRectangle{}, fmt.Errorf("%w: minX must be lower than maxX, got %s > %s",
			ErrInvalidXYRectangleBounds, formatJavaFloat(minX), formatJavaFloat(maxX))
	}
	if minY > maxY {
		return XYRectangle{}, fmt.Errorf("%w: minY must be lower than maxY, got %s > %s",
			ErrInvalidXYRectangleBounds, formatJavaFloat(minY), formatJavaFloat(maxY))
	}
	if _, err := XYCheckVal(minX); err != nil {
		return XYRectangle{}, err
	}
	if _, err := XYCheckVal(maxX); err != nil {
		return XYRectangle{}, err
	}
	if _, err := XYCheckVal(minY); err != nil {
		return XYRectangle{}, err
	}
	if _, err := XYCheckVal(maxY); err != nil {
		return XYRectangle{}, err
	}
	return XYRectangle{minX: minX, maxX: maxX, minY: minY, maxY: maxY}, nil
}

// MustNewXYRectangle is the panic-on-error variant.
func MustNewXYRectangle(minX, maxX, minY, maxY float32) XYRectangle {
	r, err := NewXYRectangle(minX, maxX, minY, maxY)
	if err != nil {
		panic(err)
	}
	return r
}

// MinX / MaxX / MinY / MaxY accessors.
func (r XYRectangle) MinX() float32 { return r.minX }
func (r XYRectangle) MaxX() float32 { return r.maxX }
func (r XYRectangle) MinY() float32 { return r.minY }
func (r XYRectangle) MaxY() float32 { return r.maxY }

// toComponent2D returns the cartesian rectangle Component2D.
func (r XYRectangle) toComponent2D() Component2D {
	return newRectangle2D(float64(r.minX), float64(r.maxX), float64(r.minY), float64(r.maxY))
}

// xyGeometry is the sealed marker on XYGeometry.
func (XYRectangle) xyGeometry() {}

// Equals reports whether two XYRectangles have bit-identical bounds.
func (r XYRectangle) Equals(o XYRectangle) bool {
	return javaFloatCompare(r.minX, o.minX) == 0 &&
		javaFloatCompare(r.minY, o.minY) == 0 &&
		javaFloatCompare(r.maxX, o.maxX) == 0 &&
		javaFloatCompare(r.maxY, o.maxY) == 0
}

// HashCode mirrors Java's XYRectangle.hashCode().
func (r XYRectangle) HashCode() int32 {
	result := javaFloatHashCode(r.minX)
	result = 31*result + javaFloatHashCode(r.minY)
	result = 31*result + javaFloatHashCode(r.maxX)
	result = 31*result + javaFloatHashCode(r.maxY)
	return result
}

// String mirrors Java's XYRectangle.toString() verbatim.
func (r XYRectangle) String() string {
	var b []byte
	b = append(b, "XYRectangle(x="...)
	b = appendJavaFloat(b, r.minX)
	b = append(b, " TO "...)
	b = appendJavaFloat(b, r.maxX)
	b = append(b, " y="...)
	b = appendJavaFloat(b, r.minY)
	b = append(b, " TO "...)
	b = appendJavaFloat(b, r.maxY)
	b = append(b, ')')
	return string(b)
}

// FromXYPointDistance computes the axis-aligned bounding box of a
// cartesian circle of the given centre and radius. It is the Go port
// of XYRectangle.fromPointDistance.
func FromXYPointDistance(x, y, radius float32) (XYRectangle, error) {
	if _, err := XYCheckVal(x); err != nil {
		return XYRectangle{}, err
	}
	if _, err := XYCheckVal(y); err != nil {
		return XYRectangle{}, err
	}
	if radius < 0 {
		return XYRectangle{}, fmt.Errorf("geo: radius must be bigger than 0, got %s",
			formatJavaFloat(radius))
	}
	if math.IsNaN(float64(radius)) || math.IsInf(float64(radius), 0) {
		return XYRectangle{}, fmt.Errorf("geo: radius must be finite, got %s",
			formatJavaFloat(radius))
	}
	// LUCENE-9243: Math.nextUp(radius) so the bbox encloses the
	// disk even after the float-rounding error in the corner
	// computations.
	distanceBox := math.Nextafter32(radius, float32(math.Inf(1)))
	minX := maxFloat32(-math.MaxFloat32, x-distanceBox)
	maxX := minFloat32(math.MaxFloat32, x+distanceBox)
	minY := maxFloat32(-math.MaxFloat32, y-distanceBox)
	maxY := minFloat32(math.MaxFloat32, y+distanceBox)
	return NewXYRectangle(minX, maxX, minY, maxY)
}

func minFloat32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func maxFloat32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
