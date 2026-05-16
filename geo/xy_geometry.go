// Code in this file mirrors org.apache.lucene.geo.XYGeometry from
// Apache Lucene 10.4.0.

package geo

// XYGeometry is the base contract for cartesian (XY) geometries. It
// is the Go port of org.apache.lucene.geo.XYGeometry.
//
// Concrete implementations declared in this package are XYPoint,
// XYRectangle, XYLine, XYPolygon, and XYCircle (added by later tasks
// in the Sprint 10 plan).
//
// XYGeometry composes the sealed Geometry interface; external
// packages cannot satisfy it because toComponent2D is unexported.
// The marker method xyGeometry distinguishes it from LatLonGeometry
// at compile time so that the two coordinate spaces cannot be mixed
// silently.
type XYGeometry interface {
	Geometry

	// xyGeometry is a marker method that distinguishes this
	// interface from LatLonGeometry at the type level.
	xyGeometry()
}

// CreateXYGeometry builds a Component2D from one or more XYGeometry
// inputs.
//
// It is the Go port of
// org.apache.lucene.geo.XYGeometry#create(XYGeometry...).
//
// Behavior mirrors the Java original and the LatLonGeometry sibling
// factory: nil slice -> ErrNilGeometries, empty slice ->
// ErrEmptyGeometries, nil element at index i -> formatted error
// referencing i, single non-nil -> its own Component2D, multiple ->
// composite Component2D with union semantics. The multi-geometry path
// is implemented by the same in-package multiComponent2D used by
// CreateLatLonGeometry, replacing Lucene's package-private
// ComponentTree until task #277 lands the full port.
func CreateXYGeometry(geometries ...XYGeometry) (Component2D, error) {
	if geometries == nil {
		return nil, ErrNilGeometries
	}
	if len(geometries) == 0 {
		return nil, ErrEmptyGeometries
	}
	if len(geometries) == 1 {
		if geometries[0] == nil {
			return nil, nilGeometryError(0)
		}
		return geometries[0].toComponent2D(), nil
	}
	components := make([]Component2D, len(geometries))
	for i, g := range geometries {
		if g == nil {
			return nil, nilGeometryError(i)
		}
		components[i] = g.toComponent2D()
	}
	return newMultiComponent2D(components), nil
}
