// Package geo provides geometric primitives and spatial-relationship
// operations ported from org.apache.lucene.geo (Apache Lucene 10.4.0).
//
// The package mirrors the Java type hierarchy:
//
//	Geometry (abstract base)
//	  - LatLonGeometry (lat/lon geographic shapes)
//	      Point, Rectangle, Line, Polygon, Circle
//	  - XYGeometry (Cartesian shapes)
//	      XYPoint, XYRectangle, XYLine, XYPolygon, XYCircle
//
// Each concrete geometry can be converted to a Component2D, the internal
// spatial-relationship engine.
package geo

// Geometry is the base contract for all geometric primitives in the geo
// package. It is the Go port of org.apache.lucene.geo.Geometry.
//
// In Lucene the equivalent type is an abstract class with a single
// protected abstract method, toComponent2D(). In Go we model it as an
// unexported-method interface (a "sealed" interface) so that the set of
// implementations is restricted to types declared inside this package,
// just as Java enforces via the package-private constructor and abstract
// shape of the superclass.
//
// Callers should not implement Geometry from outside this package; doing
// so will fail to compile because toComponent2D is unexported.
type Geometry interface {
	// toComponent2D returns a Component2D representation of the geometry,
	// used internally by the spatial relationship operations.
	//
	// It mirrors the Java protected abstract method
	// org.apache.lucene.geo.Geometry#toComponent2D.
	toComponent2D() Component2D
}
