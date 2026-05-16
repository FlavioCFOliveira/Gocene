// Code in this file mirrors org.apache.lucene.geo.LatLonGeometry from
// Apache Lucene 10.4.0.

package geo

import (
	"errors"
	"fmt"
)

// LatLonGeometry is the base contract for geographic (lat/lon)
// geometries. It is the Go port of
// org.apache.lucene.geo.LatLonGeometry.
//
// Concrete implementations declared in this package are Point,
// Rectangle, Line, Polygon, and Circle (added by later tasks in the
// Sprint 10 plan).
//
// LatLonGeometry composes the sealed Geometry interface; external
// packages cannot satisfy it because toComponent2D is unexported.
type LatLonGeometry interface {
	Geometry

	// latLonGeometry is a marker method that distinguishes this
	// interface from XYGeometry at the type level. It exists purely
	// to prevent an XYGeometry from being assigned to a
	// LatLonGeometry variable (and vice versa), mirroring Java's
	// abstract-class hierarchy.
	latLonGeometry()
}

// ErrNilGeometries is returned by Create / CreateLatLonGeometry when
// the input slice is nil. It corresponds to the Java
// IllegalArgumentException with message "geometries must not be null".
var ErrNilGeometries = errors.New("geo: geometries must not be null")

// ErrEmptyGeometries is returned by Create / CreateLatLonGeometry when
// the input slice is empty. It corresponds to the Java
// IllegalArgumentException with message "geometries must not be empty".
var ErrEmptyGeometries = errors.New("geo: geometries must not be empty")

// nilGeometryError formats the per-index "geometries[i] must not be
// null" message used by Lucene when a slot in the array is null.
func nilGeometryError(i int) error {
	return fmt.Errorf("geo: geometries[%d] must not be null", i)
}

// CreateLatLonGeometry builds a Component2D from one or more
// LatLonGeometry inputs.
//
// It is the Go port of
// org.apache.lucene.geo.LatLonGeometry#create(LatLonGeometry...).
//
// Behavior mirrors the Java original:
//
//   - A nil slice returns ErrNilGeometries (Java throws
//     IllegalArgumentException("geometries must not be null")).
//   - An empty slice returns ErrEmptyGeometries (Java throws
//     IllegalArgumentException("geometries must not be empty")).
//   - A nil element at index i returns an error formatted with the
//     index (Java throws IllegalArgumentException("geometries[i] must
//     not be null")).
//   - A single non-nil geometry returns its own Component2D.
//   - Multiple geometries return a composite Component2D whose
//     spatial relations are the set-union of the individual
//     components.
//
// In Java the equivalent of the multi-geometry path is
// ComponentTree.create(components), which builds an interval tree for
// O(log n) lookups. ComponentTree is package-private and not part of
// the public byte-for-byte contract (Component2D is not serialised);
// in Go we use a linear composite at the moment, which preserves the
// observable union semantics while keeping the geo package free of
// the ComponentTree internals (those land with task #277).
func CreateLatLonGeometry(geometries ...LatLonGeometry) (Component2D, error) {
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
