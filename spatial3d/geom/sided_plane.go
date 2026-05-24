// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom

// SidedPlane is a Plane that additionally implements Membership by
// tracking which side of the plane "inside" means.
//
// Port of org.apache.lucene.spatial3d.geom.SidedPlane.
type SidedPlane struct {
	Plane
	signum float64 // +1 or -1: the sign of evaluate(insidePoint)
}

// NewSidedPlaneFromPlane creates a SidedPlane from an existing Plane and an
// inside point.
func NewSidedPlaneFromPlane(insidePoint *Vector, p *Plane) *SidedPlane {
	eval := p.Evaluate(insidePoint)
	s := 1.0
	if eval < 0 {
		s = -1.0
	}
	return &SidedPlane{Plane: *p, signum: s}
}

// NewSidedPlane creates a plane from (A,B,C,D) and records the sign from insidePoint.
func NewSidedPlane(insidePoint *Vector, a, b, c, d float64) *SidedPlane {
	p := NewPlane(a, b, c, d)
	return NewSidedPlaneFromPlane(insidePoint, p)
}

// NewSidedPlaneThreeVectors creates the plane containing a×b,
// oriented so insidePoint is on the positive side.
func NewSidedPlaneThreeVectors(insidePoint, a, b *Vector) *SidedPlane {
	p := NewPlaneFromTwoVectors(a, b)
	return NewSidedPlaneFromPlane(insidePoint, p)
}

// IsWithin reports whether (x,y,z) is on the inside of this sided plane.
func (sp *SidedPlane) IsWithin(x, y, z float64) bool {
	val := sp.EvaluateXYZ(x, y, z)
	if sp.signum > 0 {
		return val >= -MinimumResolution
	}
	return val <= MinimumResolution
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
	return "SidedPlane(" + sp.Plane.String() + ",signum=" + fmtFloat(sp.signum) + ")"
}

var _ Membership = (*SidedPlane)(nil)
