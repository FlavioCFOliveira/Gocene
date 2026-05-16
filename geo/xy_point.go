// Code in this file mirrors org.apache.lucene.geo.XYPoint from
// Apache Lucene 10.4.0.

package geo

// XYPoint represents a single cartesian point in a 2D plane. It is
// the Go port of org.apache.lucene.geo.XYPoint.
//
// XYPoint is immutable; the zero value is not valid. Use NewXYPoint
// to construct it.
type XYPoint struct {
	x float32
	y float32
}

// NewXYPoint constructs an XYPoint at the given (x, y) coordinates.
// Both coordinates are validated via XYCheckVal (finite float32).
func NewXYPoint(x, y float32) (XYPoint, error) {
	if _, err := XYCheckVal(x); err != nil {
		return XYPoint{}, err
	}
	if _, err := XYCheckVal(y); err != nil {
		return XYPoint{}, err
	}
	return XYPoint{x: x, y: y}, nil
}

// MustNewXYPoint is the panic-on-error variant.
func MustNewXYPoint(x, y float32) XYPoint {
	p, err := NewXYPoint(x, y)
	if err != nil {
		panic(err)
	}
	return p
}

// X returns the x coordinate.
func (p XYPoint) X() float32 { return p.x }

// Y returns the y coordinate.
func (p XYPoint) Y() float32 { return p.y }

// toComponent2D returns the Point2D component for the cartesian
// point. XY shapes route through point2D as well, since point2D
// uses (x, y) coordinates directly.
func (p XYPoint) toComponent2D() Component2D {
	return newPoint2D(float64(p.x), float64(p.y))
}

// xyGeometry is the sealed marker on XYGeometry.
func (XYPoint) xyGeometry() {}

// Equals reports whether two XYPoints have bit-identical
// coordinates. Follows Java's Float.compare semantics.
func (p XYPoint) Equals(o XYPoint) bool {
	return javaFloatCompare(p.x, o.x) == 0 && javaFloatCompare(p.y, o.y) == 0
}

// HashCode mirrors Java's XYPoint.hashCode():
//
//	result = Float.hashCode(x)
//	result = 31*result + Float.hashCode(y)
func (p XYPoint) HashCode() int32 {
	result := javaFloatHashCode(p.x)
	result = 31*result + javaFloatHashCode(p.y)
	return result
}

// String mirrors Java's XYPoint.toString() exactly:
// "XYPoint(x,y)".
func (p XYPoint) String() string {
	var b []byte
	b = append(b, "XYPoint("...)
	b = appendJavaFloat(b, p.x)
	b = append(b, ',')
	b = appendJavaFloat(b, p.y)
	b = append(b, ')')
	return string(b)
}
