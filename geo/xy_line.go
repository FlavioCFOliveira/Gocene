// Code in this file mirrors org.apache.lucene.geo.XYLine from
// Apache Lucene 10.4.0.

package geo

import "errors"

// XYLine is a poly-line in cartesian (x, y) space. It is the Go
// port of org.apache.lucene.geo.XYLine. Immutable; the zero value
// is not valid.
type XYLine struct {
	xs   []float32
	ys   []float32
	minX float32
	maxX float32
	minY float32
	maxY float32
}

// Sentinel errors mirroring the Java IllegalArgumentException
// messages.
var (
	// ErrNilXYLineX is returned when the x slice is nil.
	ErrNilXYLineX = errors.New("geo: x must not be null")
	// ErrNilXYLineY is returned when the y slice is nil.
	ErrNilXYLineY = errors.New("geo: y must not be null")
	// ErrXYLineLengthMismatch is returned when x and y differ in
	// length.
	ErrXYLineLengthMismatch = errors.New("geo: x and y must be equal length")
	// ErrTooFewXYLinePoints is returned when fewer than two vertices
	// are supplied.
	ErrTooFewXYLinePoints = errors.New("geo: at least 2 line points required")
)

// NewXYLine constructs an XYLine from parallel x and y slices. Both
// must be non-nil, equal length, and contain at least two points;
// every point is validated via XYCheckVal. Slices are
// defensively copied.
func NewXYLine(xs, ys []float32) (XYLine, error) {
	if xs == nil {
		return XYLine{}, ErrNilXYLineX
	}
	if ys == nil {
		return XYLine{}, ErrNilXYLineY
	}
	if len(xs) != len(ys) {
		return XYLine{}, ErrXYLineLengthMismatch
	}
	if len(xs) < 2 {
		return XYLine{}, ErrTooFewXYLinePoints
	}

	minX, minY := xs[0], ys[0]
	maxX, maxY := xs[0], ys[0]
	for i := 0; i < len(xs); i++ {
		if _, err := XYCheckVal(xs[i]); err != nil {
			return XYLine{}, err
		}
		if _, err := XYCheckVal(ys[i]); err != nil {
			return XYLine{}, err
		}
		if xs[i] < minX {
			minX = xs[i]
		}
		if xs[i] > maxX {
			maxX = xs[i]
		}
		if ys[i] < minY {
			minY = ys[i]
		}
		if ys[i] > maxY {
			maxY = ys[i]
		}
	}
	x2 := make([]float32, len(xs))
	y2 := make([]float32, len(ys))
	copy(x2, xs)
	copy(y2, ys)
	return XYLine{xs: x2, ys: y2, minX: minX, maxX: maxX, minY: minY, maxY: maxY}, nil
}

// MustNewXYLine is the panic-on-error variant.
func MustNewXYLine(xs, ys []float32) XYLine {
	l, err := NewXYLine(xs, ys)
	if err != nil {
		panic(err)
	}
	return l
}

// NumPoints returns the vertex count.
func (l XYLine) NumPoints() int { return len(l.xs) }

// X returns the x coordinate of the vertex at index v.
func (l XYLine) X(v int) float32 { return l.xs[v] }

// Y returns the y coordinate of the vertex at index v.
func (l XYLine) Y(v int) float32 { return l.ys[v] }

// Xs returns a defensive copy of the x slice.
func (l XYLine) Xs() []float32 {
	out := make([]float32, len(l.xs))
	copy(out, l.xs)
	return out
}

// Ys returns a defensive copy of the y slice.
func (l XYLine) Ys() []float32 {
	out := make([]float32, len(l.ys))
	copy(out, l.ys)
	return out
}

// MinX / MaxX / MinY / MaxY accessors.
func (l XYLine) MinX() float32 { return l.minX }
func (l XYLine) MaxX() float32 { return l.maxX }
func (l XYLine) MinY() float32 { return l.minY }
func (l XYLine) MaxY() float32 { return l.maxY }

// toComponent2D returns the line Component2D over the float64
// widened coordinates.
func (l XYLine) toComponent2D() Component2D {
	xs := make([]float64, len(l.xs))
	ys := make([]float64, len(l.ys))
	for i := range l.xs {
		xs[i] = float64(l.xs[i])
		ys[i] = float64(l.ys[i])
	}
	return newLine2DFromXY(xs, ys)
}

// xyGeometry is the sealed marker on XYGeometry.
func (XYLine) xyGeometry() {}

// Equals reports whether two XYLines have identical vertex arrays
// (Float.compare bit semantics).
func (l XYLine) Equals(o XYLine) bool {
	if len(l.xs) != len(o.xs) || len(l.ys) != len(o.ys) {
		return false
	}
	for i := range l.xs {
		if javaFloatCompare(l.xs[i], o.xs[i]) != 0 {
			return false
		}
	}
	for i := range l.ys {
		if javaFloatCompare(l.ys[i], o.ys[i]) != 0 {
			return false
		}
	}
	return true
}

// HashCode mirrors Java's XYLine.hashCode().
func (l XYLine) HashCode() int32 {
	return 31*javaFloat32ArrayHashCode(l.xs) + javaFloat32ArrayHashCode(l.ys)
}

// javaFloat32ArrayHashCode is the float32 counterpart of
// java.util.Arrays.hashCode(float[]).
func javaFloat32ArrayHashCode(a []float32) int32 {
	result := int32(1)
	for _, v := range a {
		result = 31*result + javaFloatHashCode(v)
	}
	return result
}

// String mirrors Java's XYLine.toString() exactly. Note that
// Java emits "[x, y]" per vertex (not the geographic "[lon, lat]"
// inversion).
func (l XYLine) String() string {
	b := make([]byte, 0, 16+len(l.xs)*16)
	b = append(b, "XYLine("...)
	for i := range l.xs {
		b = append(b, '[')
		b = appendJavaFloat(b, l.xs[i])
		b = append(b, ", "...)
		b = appendJavaFloat(b, l.ys[i])
		b = append(b, ']')
	}
	b = append(b, ')')
	return string(b)
}
