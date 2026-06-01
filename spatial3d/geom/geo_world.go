// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom

import "fmt"

// NewGeoWorld constructs a GeoWorld (the whole sphere) for the given planet model.
//
// Port of org.apache.lucene.spatial3d.geom.GeoWorld(PlanetModel).
func NewGeoWorld(pm *PlanetModel) *GeoWorld {
	w := &GeoWorld{GeoBaseBBox: makeBBox(pm)}
	// originPoint mirrors Java: new GeoPoint(pm.xyScaling, 1.0, 0.0, 0.0)
	// i.e., the point at (xyScaling, 0, 0).
	w.originPoint = &GeoPoint{Vector: Vector{X: pm.XYScaling, Y: 0.0, Z: 0.0}}
	return w
}

// GetRadius returns PI — the entire sphere.
//
// Port of GeoWorld.getRadius.
func (w *GeoWorld) GetRadius() float64 { return 3.141592653589793 /* math.Pi */ }

// GetCenter returns a totally arbitrary point on the world.
//
// Port of GeoWorld.getCenter.
func (w *GeoWorld) GetCenter() *GeoPoint {
	if w.originPoint == nil {
		// Lazily initialise if constructed from the stub in shapes.go.
		w.originPoint = &GeoPoint{Vector: Vector{X: w.PlanetModelField.XYScaling}}
	}
	return w.originPoint
}

// GetEdgePoints returns an empty set — the world has no edge points.
//
// Port of GeoWorld.getEdgePoints.
func (w *GeoWorld) GetEdgePoints() []*GeoPoint { return []*GeoPoint{} }

// Expand returns the world itself — it cannot be expanded.
//
// Port of GeoWorld.expand.
func (w *GeoWorld) Expand(_ float64) GeoBBox { return w }

// Intersects returns false — the world encloses all shapes so no plane
// can cross its boundary.
//
// Port of GeoWorld.intersects(Plane,...).
func (w *GeoWorld) Intersects(_ *Plane, _ []*GeoPoint, _ ...Membership) bool { return false }

// GetBounds calls geoBaseGetBounds only — no additional planes needed.
//
// Port of GeoWorld.getBounds.
func (w *GeoWorld) GetBounds(bounds Bounds) {
	geoBaseGetBounds(w, w.PlanetModelField, bounds)
}

// String returns a debug representation.
func (w *GeoWorld) String() string {
	return fmt.Sprintf("GeoWorld: {planetmodel=%v}", w.PlanetModelField)
}

var (
	_ GeoBBox  = (*GeoWorld)(nil)
	_ GeoShape = (*GeoWorld)(nil)
)
