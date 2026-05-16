// Code in this file mirrors org.apache.lucene.geo.XYCircle from
// Apache Lucene 10.4.0.

package geo

import (
	"errors"
	"fmt"
	"math"
)

// XYCircle is a circle in cartesian (x, y) space, defined by a
// centre and a radius. It is the Go port of
// org.apache.lucene.geo.XYCircle.
type XYCircle struct {
	x      float32
	y      float32
	radius float32
}

// ErrInvalidXYRadius is wrapped by NewXYCircle when the radius is
// non-positive or non-finite.
var ErrInvalidXYRadius = errors.New("geo: invalid XY radius")

// NewXYCircle constructs an XYCircle. The radius must be strictly
// positive and finite (NaN / +/-Inf are rejected); the centre is
// validated via XYCheckVal.
func NewXYCircle(x, y, radius float32) (XYCircle, error) {
	if radius <= 0 {
		return XYCircle{}, fmt.Errorf("%w: radius must be bigger than 0, got %s",
			ErrInvalidXYRadius, formatJavaFloat(radius))
	}
	if math.IsNaN(float64(radius)) || math.IsInf(float64(radius), 0) {
		return XYCircle{}, fmt.Errorf("%w: radius must be finite, got %s",
			ErrInvalidXYRadius, formatJavaFloat(radius))
	}
	if _, err := XYCheckVal(x); err != nil {
		return XYCircle{}, err
	}
	if _, err := XYCheckVal(y); err != nil {
		return XYCircle{}, err
	}
	return XYCircle{x: x, y: y, radius: radius}, nil
}

// MustNewXYCircle is the panic-on-error variant.
func MustNewXYCircle(x, y, radius float32) XYCircle {
	c, err := NewXYCircle(x, y, radius)
	if err != nil {
		panic(err)
	}
	return c
}

// X / Y / Radius accessors.
func (c XYCircle) X() float32      { return c.x }
func (c XYCircle) Y() float32      { return c.y }
func (c XYCircle) Radius() float32 { return c.radius }

// toComponent2D builds the Component2D for the cartesian circle via
// the existing circle2D infrastructure, wiring up a
// cartesianCalculator.
func (c XYCircle) toComponent2D() Component2D {
	calc := newCartesianCalculator(float64(c.x), float64(c.y), float64(c.radius))
	return newCircle2DFromCalculator(calc)
}

// xyGeometry is the sealed marker on XYGeometry.
func (XYCircle) xyGeometry() {}

// Equals reports whether two XYCircles have bit-identical centre and
// radius.
func (c XYCircle) Equals(o XYCircle) bool {
	return javaFloatCompare(c.x, o.x) == 0 &&
		javaFloatCompare(c.y, o.y) == 0 &&
		javaFloatCompare(c.radius, o.radius) == 0
}

// HashCode mirrors Java's XYCircle.hashCode().
func (c XYCircle) HashCode() int32 {
	result := javaFloatHashCode(c.x)
	result = 31*result + javaFloatHashCode(c.y)
	result = 31*result + javaFloatHashCode(c.radius)
	return result
}

// String mirrors Java's XYCircle.toString() verbatim.
func (c XYCircle) String() string {
	var b []byte
	b = append(b, "XYCircle("...)
	b = append(b, '[')
	b = appendJavaFloat(b, c.x)
	b = append(b, ',')
	b = appendJavaFloat(b, c.y)
	b = append(b, ']')
	b = append(b, " radius = "...)
	b = appendJavaFloat(b, c.radius)
	b = append(b, ')')
	return string(b)
}
