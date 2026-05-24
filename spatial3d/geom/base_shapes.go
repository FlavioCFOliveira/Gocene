// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom

// BasePlanetObject is the base for objects tied to a PlanetModel.
//
// Port of org.apache.lucene.spatial3d.geom.BasePlanetObject.
type BasePlanetObject struct {
	PlanetModelField *PlanetModel
}

// GetPlanetModel returns the associated PlanetModel.
func (b *BasePlanetObject) GetPlanetModel() *PlanetModel { return b.PlanetModelField }

// GeoBaseShape is the abstract base for all geo shapes.
//
// Port of org.apache.lucene.spatial3d.geom.GeoBaseShape.
type GeoBaseShape struct {
	BasePlanetObject
}

// GetBounds is a no-op stub — deferred to #2693.
func (s *GeoBaseShape) GetBounds(_ Bounds) {}

// GeoBaseMembershipShape is the base for shapes that implement Membership.
//
// Port of org.apache.lucene.spatial3d.geom.GeoBaseMembershipShape.
type GeoBaseMembershipShape struct {
	GeoBaseShape
}

// IsWithin returns false — deferred to #2693.
func (s *GeoBaseMembershipShape) IsWithin(_, _, _ float64) bool { return false }

// GetEdgePoints returns nil — deferred to #2693.
func (s *GeoBaseMembershipShape) GetEdgePoints() []*GeoPoint { return nil }

// Intersects returns false — deferred to #2693.
func (s *GeoBaseMembershipShape) Intersects(_ *Plane, _ []*GeoPoint, _ ...Membership) bool {
	return false
}

// GeoBaseAreaShape is the base for shapes that implement GeoAreaShape.
//
// Port of org.apache.lucene.spatial3d.geom.GeoBaseAreaShape.
type GeoBaseAreaShape struct {
	GeoBaseMembershipShape
}

// GetRelationship returns RelDisjoint — deferred to #2693.
func (s *GeoBaseAreaShape) GetRelationship(_ GeoShape) int { return RelDisjoint }

// GeoBaseBBox is the base for bounding-box shapes.
//
// Port of org.apache.lucene.spatial3d.geom.GeoBaseBBox.
type GeoBaseBBox struct {
	GeoBaseAreaShape
}

// Expand returns nil — deferred to #2693.
func (s *GeoBaseBBox) Expand(_ float64) GeoBBox { return nil }

// GeoBaseCircle is the base for circular shapes.
//
// Port of org.apache.lucene.spatial3d.geom.GeoBaseCircle.
type GeoBaseCircle struct {
	GeoBaseMembershipShape
	radius float64
}

// GetRadius returns the radius.
func (s *GeoBaseCircle) GetRadius() float64 { return s.radius }

// GeoBasePolygon is the base for polygon shapes.
//
// Port of org.apache.lucene.spatial3d.geom.GeoBasePolygon.
type GeoBasePolygon struct {
	GeoBaseMembershipShape
}

// GeoBasePath is the base for path shapes.
//
// Port of org.apache.lucene.spatial3d.geom.GeoBasePath.
type GeoBasePath struct {
	GeoBaseMembershipShape
	cutoffAngle float64
}

// GeoBaseDistanceShape is the base for shapes that compute distances.
//
// Port of org.apache.lucene.spatial3d.geom.GeoBaseDistanceShape.
type GeoBaseDistanceShape struct {
	GeoBaseMembershipShape
}

// ComputeDistance returns 0 — deferred to #2693.
func (s *GeoBaseDistanceShape) ComputeDistance(_ DistanceStyle, _, _, _ float64) float64 {
	return 0
}

// GetRadius returns 0 — deferred to #2693.
func (s *GeoBaseDistanceShape) GetRadius() float64 { return 0 }

// GeoBaseCompositeMembershipShape is the base for composite membership shapes.
//
// Port of org.apache.lucene.spatial3d.geom.GeoBaseCompositeMembershipShape.
type GeoBaseCompositeMembershipShape struct {
	GeoBaseMembershipShape
	shapes []GeoMembershipShape
}

// AddShape appends a sub-shape.
func (s *GeoBaseCompositeMembershipShape) AddShape(shape GeoMembershipShape) {
	s.shapes = append(s.shapes, shape)
}

// GeoBaseCompositeAreaShape is the base for composite area shapes.
//
// Port of org.apache.lucene.spatial3d.geom.GeoBaseCompositeAreaShape.
type GeoBaseCompositeAreaShape struct {
	GeoBaseAreaShape
	shapes []GeoAreaShape
}

// AddShape appends a sub-shape.
func (s *GeoBaseCompositeAreaShape) AddShape(shape GeoAreaShape) {
	s.shapes = append(s.shapes, shape)
}

// GeoBaseCompositeShape is the base for composite shapes (any kind).
//
// Port of org.apache.lucene.spatial3d.geom.GeoBaseCompositeShape.
type GeoBaseCompositeShape[T GeoMembershipShape] struct {
	GeoBaseMembershipShape
	shapes []T
}

// AddShape appends a sub-shape.
func (s *GeoBaseCompositeShape[T]) AddShape(shape T) {
	s.shapes = append(s.shapes, shape)
}
