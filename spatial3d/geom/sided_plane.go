// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom

import "math"

// SidedPlane is a Plane combined with a sign value indicating which evaluation
// results are on the "inside" of the plane.
//
// Port of org.apache.lucene.spatial3d.geom.SidedPlane.
type SidedPlane struct {
	Plane
	signum float64 // Math.signum(evaluate(insidePoint)); +1 or -1
}

// signum returns the sign of v (-1, 0, or +1), matching java.lang.Math.signum
// for finite, non-NaN inputs (the only inputs used by this package).
func signum(v float64) float64 {
	switch {
	case v > 0:
		return 1.0
	case v < 0:
		return -1.0
	default:
		return v // preserves +0/-0; treated as "on plane"
	}
}

// NewSidedPlaneFromPlane creates a SidedPlane from an existing Plane, recording
// the sign of evaluate(insidePoint). Returns nil if the inside point lies on
// the plane (sidedness is then undeterminable).
//
// Port of SidedPlane(Vector,Vector,Vector)-style sign capture for a ready plane.
func NewSidedPlaneFromPlane(insidePoint *Vector, p *Plane) *SidedPlane {
	s := signum(p.Evaluate(insidePoint))
	if s == 0.0 {
		return nil
	}
	return &SidedPlane{Plane: *p, signum: s}
}

// NewSidedPlane creates a sided plane from (A,B,C,D) and the inside point.
//
// Port of SidedPlane(Vector,double,double,double,double) sign capture.
func NewSidedPlane(insidePoint *Vector, a, b, c, d float64) *SidedPlane {
	return NewSidedPlaneFromPlane(insidePoint, NewPlane(a, b, c, d))
}

// NewSidedPlaneVectorD creates a sided plane from a normal vector, D offset, and
// the inside point.
//
// Port of SidedPlane(Vector,Vector,double).
func NewSidedPlaneVectorD(insidePoint, v *Vector, d float64) *SidedPlane {
	return NewSidedPlaneFromPlane(insidePoint, NewPlaneFromVectorD(v, d))
}

// NewSidedPlaneFromPointAndUnit constructs a sided plane from a scalar inside
// point (px, py, pz), a unit normal vector v, and a D offset.  This mirrors
// Java's SidedPlane(double insideX, double insideY, double insideZ, Vector n, double D).
func NewSidedPlaneFromPointAndUnit(px, py, pz float64, v *Vector, d float64) *SidedPlane {
	return NewSidedPlaneVectorD(&Vector{X: px, Y: py, Z: pz}, v, d)
}

// NewSidedPlaneHorizontal creates a sided horizontal plane at sin(latitude) on
// the planet model, with the inside point determining the sign.
//
// Port of SidedPlane(Vector,PlanetModel,double).
func NewSidedPlaneHorizontal(insidePoint *Vector, pm *PlanetModel, sinLat float64) *SidedPlane {
	return NewSidedPlaneFromPlane(insidePoint, NewPlaneHorizontal(pm, sinLat))
}

// NewSidedPlaneVertical creates a sided vertical plane through (x,y) with the
// inside point determining the sign.
//
// Port of SidedPlane(Vector,double,double).
func NewSidedPlaneVertical(insidePoint *Vector, x, y float64) *SidedPlane {
	return NewSidedPlaneFromPlane(insidePoint, NewPlaneVertical(x, y))
}

// NewSidedPlaneThreeVectors creates a plane through A, B, and the origin, with
// insidePoint determining the inside sign. Returns nil when A and B are
// parallel or insidePoint is on the plane.
//
// Port of SidedPlane(Vector,Vector,Vector).
func NewSidedPlaneThreeVectors(insidePoint, a, b *Vector) *SidedPlane {
	p, err := NewPlaneFromTwoVectors(a, b)
	if err != nil {
		return nil
	}
	return NewSidedPlaneFromPlane(insidePoint, p)
}

// NewSidedPlaneOnSide creates a plane through A, B, and the origin. When onSide
// is true, point p is on the inside; when false, p is on the outside (the sign
// is negated). Returns nil when A and B are parallel or p is on the plane.
//
// Port of SidedPlane(Vector,boolean,Vector,Vector).
func NewSidedPlaneOnSide(p *Vector, onSide bool, a, b *Vector) *SidedPlane {
	plane, err := NewPlaneFromTwoVectors(a, b)
	if err != nil {
		return nil
	}
	s := signum(plane.Evaluate(p))
	if !onSide {
		s = -s
	}
	if s == 0.0 {
		return nil
	}
	return &SidedPlane{Plane: *plane, signum: s}
}

// NewSidedPlaneReversed returns a SidedPlane identical to sp but with the
// inside sign reversed.
//
// Port of SidedPlane(SidedPlane).
func NewSidedPlaneReversed(sp *SidedPlane) *SidedPlane {
	return &SidedPlane{Plane: sp.Plane, signum: -sp.signum}
}

// ConstructNormalizedPerpendicularSidedPlane constructs a sided plane through
// point1 and point2 whose normal lies in the plane of normalVector, oriented so
// insidePoint is on the inside. Returns nil if it cannot be constructed.
//
// Port of SidedPlane.constructNormalizedPerpendicularSidedPlane.
func ConstructNormalizedPerpendicularSidedPlane(insidePoint, normalVector, point1, point2 *Vector) *SidedPlane {
	pointsVector := &Vector{
		X: point1.X - point2.X,
		Y: point1.Y - point2.Y,
		Z: point1.Z - point2.Z,
	}
	newNormalVector, err := NewVectorPerpendicularFromVectors(normalVector, pointsVector)
	if err != nil {
		return nil
	}
	d := -newNormalVector.DotProduct(point1)
	return NewSidedPlaneVectorD(insidePoint, newNormalVector, d)
}

// ConstructSidedPlaneFromOnePoint finds a plane perpendicular to the given
// plane that goes through both the origin and intersectionPoint, oriented so
// insidePoint is on the inside. Returns nil if it cannot be constructed.
//
// Port of SidedPlane.constructSidedPlaneFromOnePoint.
func ConstructSidedPlaneFromOnePoint(insidePoint *Vector, plane *Plane, intersectionPoint *Vector) *SidedPlane {
	newPlane, err := ConstructPerpendicularCenterPlaneOnePoint(plane, intersectionPoint)
	if err != nil {
		return nil
	}
	return NewSidedPlane(insidePoint, newPlane.X, newPlane.Y, newPlane.Z, newPlane.D)
}

// IsWithin reports whether (x,y,z) is on the inside of this sided plane.
//
// Port of SidedPlane.isWithin: points within the minimum resolution of the
// plane are always considered inside; otherwise the evaluation sign must match.
func (sp *SidedPlane) IsWithin(x, y, z float64) bool {
	evalResult := sp.EvaluateXYZ(x, y, z)
	if math.Abs(evalResult) < MinimumResolution {
		return true
	}
	return signum(evalResult) == sp.signum
}

// IsWithinVector is the Vector-typed convenience form of IsWithin.
func (sp *SidedPlane) IsWithinVector(v *Vector) bool {
	return sp.IsWithin(v.X, v.Y, v.Z)
}

// StrictlyWithin reports whether (x,y,z) is strictly inside (sign 0 or match).
//
// Port of SidedPlane.strictlyWithin.
func (sp *SidedPlane) StrictlyWithin(x, y, z float64) bool {
	s := signum(sp.EvaluateXYZ(x, y, z))
	return s == 0.0 || s == sp.signum
}

// Equals reports equality.
func (sp *SidedPlane) Equals(other *SidedPlane) bool {
	if other == nil {
		return false
	}
	return sp.Plane.Equals(&other.Plane) && sp.signum == other.signum
}

// String returns a debug representation.
func (sp *SidedPlane) String() string {
	return "[A=" + fmtFloat(sp.X) + ", B=" + fmtFloat(sp.Y) + ", C=" + fmtFloat(sp.Z) +
		", D=" + fmtFloat(sp.D) + ", side=" + fmtFloat(sp.signum) + "]"
}

var _ Membership = (*SidedPlane)(nil)
