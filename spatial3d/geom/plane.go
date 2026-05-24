// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom

import "math"

// Plane is a 3-D plane defined by its normal vector (A,B,C) and offset D,
// satisfying the equation Ax + By + Cz + D = 0.
//
// Port of org.apache.lucene.spatial3d.geom.Plane.
//
// Deviation: findIntersections, arcDistance, interpolate and related
// geometric helpers deferred to backlog #2693.
type Plane struct {
	Vector         // normal (A,B,C) stored in X,Y,Z
	D      float64 // offset term
}

// Well-known planes.
var (
	// NormalYPlane is the Y=0 plane.
	NormalYPlane = &Plane{Vector: Vector{X: 0, Y: 1, Z: 0}, D: 0}
	// NormalXPlane is the X=0 plane.
	NormalXPlane = &Plane{Vector: Vector{X: 1, Y: 0, Z: 0}, D: 0}
	// NormalZPlane is the Z=0 plane.
	NormalZPlane = &Plane{Vector: Vector{X: 0, Y: 0, Z: 1}, D: 0}
	// NoPoints is the empty GeoPoint slice.
	NoPoints = []*GeoPoint{}
	// NoBounds is the empty Membership slice.
	NoBounds = []Membership{}
)

// NewPlane creates a plane from Ax+By+Cz+D=0 coefficients.
func NewPlane(a, b, c, d float64) *Plane {
	return &Plane{Vector: Vector{X: a, Y: b, Z: c}, D: d}
}

// NewPlaneFromVectorD creates a plane from a normal vector and offset D.
func NewPlaneFromVectorD(v *Vector, d float64) *Plane {
	return &Plane{Vector: *v, D: d}
}

// NewPlaneFromTwoVectors creates the plane containing vectors a and b
// (cross product of a and b as normal, D=0).
func NewPlaneFromTwoVectors(a, b *Vector) *Plane {
	return &Plane{Vector: *NewVectorCrossProduct(a, b), D: 0}
}

// NewLatitudePlane creates a horizontal plane at the given sin(latitude).
func NewLatitudePlane(_ *PlanetModel, sinLat float64) *Plane {
	return &Plane{Vector: Vector{X: 0, Y: 0, Z: 1}, D: -sinLat}
}

// NewLongitudePlane creates a vertical plane through the given lon.
func NewLongitudePlane(x, y float64) *Plane {
	return &Plane{Vector: Vector{X: -y, Y: x, Z: 0}, D: 0}
}

// Evaluate computes Ax+By+Cz+D for the given vector.
func (p *Plane) Evaluate(v *Vector) float64 {
	return p.X*v.X + p.Y*v.Y + p.Z*v.Z + p.D
}

// EvaluateXYZ computes Ax+By+Cz+D.
func (p *Plane) EvaluateXYZ(x, y, z float64) float64 {
	return p.X*x + p.Y*y + p.Z*z + p.D
}

// EvaluateIsZero reports whether Ax+By+Cz+D ≈ 0 for the given vector.
func (p *Plane) EvaluateIsZero(v *Vector) bool {
	return math.Abs(p.Evaluate(v)) < MinimumResolution
}

// EvaluateIsZeroXYZ reports whether Ax+By+Cz+D ≈ 0 for (x,y,z).
func (p *Plane) EvaluateIsZeroXYZ(x, y, z float64) bool {
	return math.Abs(p.EvaluateXYZ(x, y, z)) < MinimumResolution
}

// Normalize returns a Plane with a unit normal.
func (p *Plane) Normalize() *Plane {
	m := p.Vector.Magnitude()
	if m < MinimumResolution {
		return p
	}
	return &Plane{
		Vector: Vector{X: p.X / m, Y: p.Y / m, Z: p.Z / m},
		D:      p.D / m,
	}
}

// IsWithin reports whether (x,y,z) is on or above the plane (D+eval >= 0).
func (p *Plane) IsWithin(x, y, z float64) bool {
	return p.EvaluateXYZ(x, y, z) >= -MinimumResolution
}

// FindIntersections returns intersection points of this plane with the
// planet model surface. Deferred to #2693; returns an empty slice.
func (p *Plane) FindIntersections(_ *PlanetModel, _ *Plane, _ ...Membership) []*GeoPoint {
	return NoPoints
}

// Equals reports equality.
func (p *Plane) Equals(other *Plane) bool {
	if other == nil {
		return false
	}
	return p.X == other.X && p.Y == other.Y && p.Z == other.Z && p.D == other.D
}

// HashCode returns a simple hash.
func (p *Plane) HashCode() int {
	h := p.Vector.HashCode()
	h = h*31 + floatBits(p.D)
	return h
}

// String returns a debug representation.
func (p *Plane) String() string {
	return "Plane(" + fmtFloat(p.X) + "x+" + fmtFloat(p.Y) + "y+" +
		fmtFloat(p.Z) + "z+" + fmtFloat(p.D) + "=0)"
}
