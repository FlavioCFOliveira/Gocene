// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom

import "math"

// Plane is a 3-D plane defined by its normal vector (A,B,C) and offset D,
// satisfying the equation Ax + By + Cz + D = 0.
//
// Port of org.apache.lucene.spatial3d.geom.Plane.
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
//
// Port of Plane(double,double,double,double).
func NewPlane(a, b, c, d float64) *Plane {
	return &Plane{Vector: Vector{X: a, Y: b, Z: c}, D: d}
}

// NewPlaneFromVectorD creates a plane from a normal vector and offset D.
//
// Port of Plane(Vector,double).
func NewPlaneFromVectorD(v *Vector, d float64) *Plane {
	return &Plane{Vector: *v, D: d}
}

// NewPlaneFromTwoVectors creates a plane through two points and the origin
// (D=0). The normal is the normalised perpendicular of a and b. It returns
// ErrDegenerateVector if a and b are parallel.
//
// Port of Plane(Vector,Vector).
func NewPlaneFromTwoVectors(a, b *Vector) (*Plane, error) {
	v, err := NewVectorPerpendicularFromVectors(a, b)
	if err != nil {
		return nil, err
	}
	return &Plane{Vector: *v, D: 0}, nil
}

// NewPlaneVectorPoint creates a plane through vector A, the point (bx,by,bz),
// and the origin (D=0). The normal is the normalised perpendicular.
//
// Port of Plane(Vector,double,double,double).
func NewPlaneVectorPoint(a *Vector, bx, by, bz float64) (*Plane, error) {
	v, err := NewVectorPerpendicular(a.X, a.Y, a.Z, bx, by, bz)
	if err != nil {
		return nil, err
	}
	return &Plane{Vector: *v, D: 0}, nil
}

// NewPlaneHorizontal creates a horizontal plane at the given sin(latitude) on
// the planet model.
//
// Port of Plane(PlanetModel,double).
func NewPlaneHorizontal(pm *PlanetModel, sinLat float64) *Plane {
	return &Plane{
		Vector: Vector{X: 0, Y: 0, Z: 1},
		D:      -sinLat * computeDesiredEllipsoidMagnitudeZ(pm, sinLat),
	}
}

// NewPlaneVertical creates a vertical plane through (x,y) and the origin.
//
// Port of Plane(double,double): normal is (y,-x,0), D=0.
func NewPlaneVertical(x, y float64) *Plane {
	return &Plane{Vector: Vector{X: y, Y: -x, Z: 0}, D: 0}
}

// NewLatitudePlane creates a horizontal plane at the given sin(latitude).
//
// Deprecated: retained for backward compatibility. Use NewPlaneHorizontal,
// which applies the ellipsoid magnitude exactly as Lucene does.
func NewLatitudePlane(pm *PlanetModel, sinLat float64) *Plane {
	return NewPlaneHorizontal(pm, sinLat)
}

// NewLongitudePlane creates a vertical plane through the given lon.
//
// Deprecated: retained for backward compatibility. Use NewPlaneVertical, which
// matches Lucene's normal orientation.
func NewLongitudePlane(x, y float64) *Plane {
	return NewPlaneVertical(x, y)
}

// ConstructNormalizedZPlaneXY constructs a normalised plane through the x-y
// point (x,y,0) and the Z axis. Returns nil when (x,y) is at the origin.
//
// Port of Plane.constructNormalizedZPlane(double,double).
func ConstructNormalizedZPlaneXY(x, y float64) *Plane {
	if math.Abs(x) < MinimumResolution && math.Abs(y) < MinimumResolution {
		return nil
	}
	denom := 1.0 / math.Sqrt(x*x+y*y)
	return NewPlane(y*denom, -x*denom, 0.0, 0.0)
}

// ConstructNormalizedZPlane picks the best of planePoints (greatest x-y
// distance) and constructs a normalised plane through it and the Z axis.
//
// Port of Plane.constructNormalizedZPlane(Vector...).
func ConstructNormalizedZPlane(planePoints ...*Vector) *Plane {
	bestDistance := 0.0
	var bestPoint *Vector
	for _, point := range planePoints {
		pointDist := point.X*point.X + point.Y*point.Y
		if pointDist > bestDistance {
			bestDistance = pointDist
			bestPoint = point
		}
	}
	if bestPoint == nil {
		return nil
	}
	return ConstructNormalizedZPlaneXY(bestPoint.X, bestPoint.Y)
}

// ConstructPerpendicularCenterPlaneOnePoint finds a plane perpendicular to the
// given plane that goes through both the origin and the point M (which must be
// on the plane). Returns ErrDegenerateVector when no such plane can be found.
//
// Port of Plane.constructPerpendicularCenterPlaneOnePoint.
func ConstructPerpendicularCenterPlaneOnePoint(plane *Plane, m *Vector) (*Plane, error) {
	a0, b0, c0 := plane.X, plane.Y, plane.Z

	a1Denom := c0*m.Y - b0*m.Z
	b1Denom := c0*m.X - a0*m.Z
	c1Denom := b0*m.X - a0*m.Y

	var a1, b1, c1 float64
	switch {
	case math.Abs(a1Denom) >= math.Abs(b1Denom) && math.Abs(a1Denom) >= math.Abs(c1Denom):
		a1 = 1.0
		if math.Abs(m.Y) >= math.Abs(m.Z) {
			c1 = (b0*m.X - a0*m.Y) / a1Denom
			b1 = (-m.X - c1*m.Z) / m.Y
		} else {
			b1 = (a0*m.Z - c0*m.X) / a1Denom
			c1 = (-m.X - b1*m.Y) / m.Z
		}
	case math.Abs(b1Denom) >= math.Abs(a1Denom) && math.Abs(b1Denom) >= math.Abs(c1Denom):
		b1 = 1.0
		if math.Abs(m.X) >= math.Abs(m.Z) {
			c1 = (a0*m.Y - b0*m.X) / b1Denom
			a1 = (-m.Y - c1*m.Z) / m.X
		} else {
			a1 = (b0*m.Z - c0*m.Y) / b1Denom
			c1 = (-m.Y - a1*m.X) / m.Z
		}
	case math.Abs(c1Denom) >= math.Abs(a1Denom) && math.Abs(c1Denom) >= math.Abs(b1Denom):
		c1 = 1.0
		if math.Abs(m.X) >= math.Abs(m.Y) {
			b1 = (a0*m.Z - c0*m.X) / c1Denom
			a1 = (-m.Z - b1*m.Y) / m.X
		} else {
			a1 = (c0*m.Y - b0*m.Z) / c1Denom
			b1 = (-m.Z - a1*m.X) / m.Y
		}
	default:
		return nil, ErrDegenerateVector
	}

	normFactor := 1.0 / math.Sqrt(a1*a1+b1*b1+c1*c1)
	vx, vy, vz := a1*normFactor, b1*normFactor, c1*normFactor
	return NewPlane(vx, vy, vz, -(vx*m.X + vy*m.Y + vz*m.Z)), nil
}

// Evaluate computes Ax+By+Cz+D for the given vector.
//
// Port of Plane.evaluate(Vector).
func (p *Plane) Evaluate(v *Vector) float64 {
	return p.X*v.X + p.Y*v.Y + p.Z*v.Z + p.D
}

// EvaluateXYZ computes Ax+By+Cz+D.
//
// Port of Plane.evaluate(double,double,double).
func (p *Plane) EvaluateXYZ(x, y, z float64) float64 {
	return p.X*x + p.Y*y + p.Z*z + p.D
}

// EvaluateIsZero reports whether Ax+By+Cz+D ≈ 0 for the given vector.
//
// Port of Plane.evaluateIsZero(Vector).
func (p *Plane) EvaluateIsZero(v *Vector) bool {
	return math.Abs(p.Evaluate(v)) < MinimumResolution
}

// EvaluateIsZeroXYZ reports whether Ax+By+Cz+D ≈ 0 for (x,y,z).
//
// Port of Plane.evaluateIsZero(double,double,double).
func (p *Plane) EvaluateIsZeroXYZ(x, y, z float64) bool {
	return math.Abs(p.EvaluateXYZ(x, y, z)) < MinimumResolution
}

// Normalize returns a Plane with a unit normal, or nil if indeterminate.
//
// Port of Plane.normalize().
func (p *Plane) Normalize() *Plane {
	m := p.Vector.Magnitude()
	if m < MinimumResolution {
		return nil
	}
	return &Plane{
		Vector: Vector{X: p.X / m, Y: p.Y / m, Z: p.Z / m},
		D:      p.D / m,
	}
}

// IsWithin reports whether (x,y,z) is on or above the plane.
//
// Note: Plane itself has no notion of sidedness; this convenience preserves the
// historical Gocene behaviour (eval >= -resolution). SidedPlane overrides it
// with proper sign handling.
func (p *Plane) IsWithin(x, y, z float64) bool {
	return p.EvaluateXYZ(x, y, z) >= -MinimumResolution
}

// IsNumericallyIdentical reports whether this plane and q describe the same
// surface, accounting for orientation and offset.
//
// Port of Plane.isNumericallyIdentical(Plane).
func (p *Plane) IsNumericallyIdentical(q *Plane) bool {
	cross1 := p.Y*q.Z - p.Z*q.Y
	cross2 := p.Z*q.X - p.X*q.Z
	cross3 := p.X*q.Y - p.Y*q.X
	if cross1*cross1+cross2*cross2+cross3*cross3 >= MinimumResolutionSquared {
		return false
	}
	denom := 1.0 / (q.X*q.X + q.Y*q.Y + q.Z*q.Z)
	return p.EvaluateIsZeroXYZ(-q.X*q.D*denom, -q.Y*q.D*denom, -q.Z*q.D*denom)
}

// FindIntersections returns the crossing points of this plane with q on the
// planet surface, restricted to the given bounds. It returns nil when the two
// planes are numerically identical.
//
// Port of Plane.findIntersections(PlanetModel,Plane,Membership...).
func (p *Plane) FindIntersections(pm *PlanetModel, q *Plane, bounds ...Membership) []*GeoPoint {
	if p.IsNumericallyIdentical(q) {
		return nil
	}
	return p.findIntersections(pm, q, bounds, nil)
}

// findIntersections is the bounded intersection core.
//
// Port of Plane.findIntersections(PlanetModel,Plane,Membership[],Membership[]).
func (p *Plane) findIntersections(pm *PlanetModel, q *Plane, bounds, moreBounds []Membership) []*GeoPoint {
	lineVectorX := p.Y*q.Z - p.Z*q.Y
	lineVectorY := p.Z*q.X - p.X*q.Z
	lineVectorZ := p.X*q.Y - p.Y*q.X
	if math.Abs(lineVectorX) < MinimumResolution &&
		math.Abs(lineVectorY) < MinimumResolution &&
		math.Abs(lineVectorZ) < MinimumResolution {
		// Parallel planes.
		return NoPoints
	}

	var x0, y0, z0 float64
	denomYZ := p.Y*q.Z - p.Z*q.Y
	denomXZ := p.X*q.Z - p.Z*q.X
	denomXY := p.X*q.Y - p.Y*q.X
	switch {
	case math.Abs(denomYZ) >= math.Abs(denomXZ) && math.Abs(denomYZ) >= math.Abs(denomXY):
		if math.Abs(denomYZ) < MinimumResolutionSquared {
			return NoPoints
		}
		denom := 1.0 / denomYZ
		x0 = 0.0
		y0 = (-p.D*q.Z - p.Z*-q.D) * denom
		z0 = (p.Y*-q.D + p.D*q.Y) * denom
	case math.Abs(denomXZ) >= math.Abs(denomXY) && math.Abs(denomXZ) >= math.Abs(denomYZ):
		if math.Abs(denomXZ) < MinimumResolutionSquared {
			return NoPoints
		}
		denom := 1.0 / denomXZ
		x0 = (-p.D*q.Z - p.Z*-q.D) * denom
		y0 = 0.0
		z0 = (p.X*-q.D + p.D*q.X) * denom
	default:
		if math.Abs(denomXY) < MinimumResolutionSquared {
			return NoPoints
		}
		denom := 1.0 / denomXY
		x0 = (-p.D*q.Y - p.Y*-q.D) * denom
		y0 = (p.X*-q.D + p.D*q.X) * denom
		z0 = 0.0
	}

	a := lineVectorX*lineVectorX*pm.InverseXYScalingSquared +
		lineVectorY*lineVectorY*pm.InverseXYScalingSquared +
		lineVectorZ*lineVectorZ*pm.InverseZScalingSquared
	b := 2.0 * (lineVectorX*x0*pm.InverseXYScalingSquared +
		lineVectorY*y0*pm.InverseXYScalingSquared +
		lineVectorZ*z0*pm.InverseZScalingSquared)
	c := x0*x0*pm.InverseXYScalingSquared +
		y0*y0*pm.InverseXYScalingSquared +
		z0*z0*pm.InverseZScalingSquared - 1.0

	bSquaredMinus := b*b - 4.0*a*c
	switch {
	case math.Abs(bSquaredMinus) < MinimumResolutionSquared:
		inverse2A := 1.0 / (2.0 * a)
		t := -b * inverse2A
		pointX := lineVectorX*t + x0
		pointY := lineVectorY*t + y0
		pointZ := lineVectorZ*t + z0
		if !meetsAllBoundsXYZ(pointX, pointY, pointZ, bounds) ||
			!meetsAllBoundsXYZ(pointX, pointY, pointZ, moreBounds) {
			return NoPoints
		}
		return []*GeoPoint{NewGeoPoint(pointX, pointY, pointZ)}
	case bSquaredMinus > 0.0:
		inverse2A := 1.0 / (2.0 * a)
		sqrtTerm := math.Sqrt(bSquaredMinus)
		t1 := (-b + sqrtTerm) * inverse2A
		t2 := (-b - sqrtTerm) * inverse2A
		point1X := lineVectorX*t1 + x0
		point1Y := lineVectorY*t1 + y0
		point1Z := lineVectorZ*t1 + z0
		point2X := lineVectorX*t2 + x0
		point2Y := lineVectorY*t2 + y0
		point2Z := lineVectorZ*t2 + z0
		point1Valid := meetsAllBoundsXYZ(point1X, point1Y, point1Z, bounds) &&
			meetsAllBoundsXYZ(point1X, point1Y, point1Z, moreBounds)
		point2Valid := meetsAllBoundsXYZ(point2X, point2Y, point2Z, bounds) &&
			meetsAllBoundsXYZ(point2X, point2Y, point2Z, moreBounds)
		switch {
		case point1Valid && point2Valid:
			return []*GeoPoint{
				NewGeoPoint(point1X, point1Y, point1Z),
				NewGeoPoint(point2X, point2Y, point2Z),
			}
		case point1Valid:
			return []*GeoPoint{NewGeoPoint(point1X, point1Y, point1Z)}
		case point2Valid:
			return []*GeoPoint{NewGeoPoint(point2X, point2Y, point2Z)}
		default:
			return NoPoints
		}
	default:
		return NoPoints
	}
}

// Intersects reports whether this plane intersects q on the planet surface
// within the given bounds. notablePoints / moreNotablePoints are consulted only
// when the two planes are numerically identical.
//
// Port of Plane.intersects(PlanetModel,Plane,GeoPoint[],GeoPoint[],Membership[],Membership...).
func (p *Plane) Intersects(pm *PlanetModel, q *Plane, notablePoints, moreNotablePoints []*GeoPoint, bounds []Membership, moreBounds ...Membership) bool {
	if p.IsNumericallyIdentical(q) {
		for _, pt := range notablePoints {
			if meetsAllBoundsVector(pt, bounds, moreBounds) {
				return true
			}
		}
		for _, pt := range moreNotablePoints {
			if meetsAllBoundsVector(pt, bounds, moreBounds) {
				return true
			}
		}
		return false
	}

	lineVectorX := p.Y*q.Z - p.Z*q.Y
	lineVectorY := p.Z*q.X - p.X*q.Z
	lineVectorZ := p.X*q.Y - p.Y*q.X
	if math.Abs(lineVectorX) < MinimumResolution &&
		math.Abs(lineVectorY) < MinimumResolution &&
		math.Abs(lineVectorZ) < MinimumResolution {
		return false
	}

	var x0, y0, z0 float64
	denomYZ := p.Y*q.Z - p.Z*q.Y
	denomXZ := p.X*q.Z - p.Z*q.X
	denomXY := p.X*q.Y - p.Y*q.X
	switch {
	case math.Abs(denomYZ) >= math.Abs(denomXZ) && math.Abs(denomYZ) >= math.Abs(denomXY):
		if math.Abs(denomYZ) < MinimumResolutionSquared {
			return false
		}
		denom := 1.0 / denomYZ
		x0 = 0.0
		y0 = (-p.D*q.Z - p.Z*-q.D) * denom
		z0 = (p.Y*-q.D + p.D*q.Y) * denom
	case math.Abs(denomXZ) >= math.Abs(denomXY) && math.Abs(denomXZ) >= math.Abs(denomYZ):
		if math.Abs(denomXZ) < MinimumResolutionSquared {
			return false
		}
		denom := 1.0 / denomXZ
		x0 = (-p.D*q.Z - p.Z*-q.D) * denom
		y0 = 0.0
		z0 = (p.X*-q.D + p.D*q.X) * denom
	default:
		if math.Abs(denomXY) < MinimumResolutionSquared {
			return false
		}
		denom := 1.0 / denomXY
		x0 = (-p.D*q.Y - p.Y*-q.D) * denom
		y0 = (p.X*-q.D + p.D*q.X) * denom
		z0 = 0.0
	}

	a := lineVectorX*lineVectorX*pm.InverseXYScalingSquared +
		lineVectorY*lineVectorY*pm.InverseXYScalingSquared +
		lineVectorZ*lineVectorZ*pm.InverseZScalingSquared
	b := 2.0 * (lineVectorX*x0*pm.InverseXYScalingSquared +
		lineVectorY*y0*pm.InverseXYScalingSquared +
		lineVectorZ*z0*pm.InverseZScalingSquared)
	c := x0*x0*pm.InverseXYScalingSquared +
		y0*y0*pm.InverseXYScalingSquared +
		z0*z0*pm.InverseZScalingSquared - 1.0

	bSquaredMinus := b*b - 4.0*a*c
	switch {
	case math.Abs(bSquaredMinus) < MinimumResolutionSquared:
		inverse2A := 1.0 / (2.0 * a)
		t := -b * inverse2A
		pointX := lineVectorX*t + x0
		pointY := lineVectorY*t + y0
		pointZ := lineVectorZ*t + z0
		return meetsAllBoundsXYZ(pointX, pointY, pointZ, bounds) &&
			meetsAllBoundsXYZ(pointX, pointY, pointZ, moreBounds)
	case bSquaredMinus > 0.0:
		inverse2A := 1.0 / (2.0 * a)
		sqrtTerm := math.Sqrt(bSquaredMinus)
		t1 := (-b + sqrtTerm) * inverse2A
		t2 := (-b - sqrtTerm) * inverse2A
		point1X := lineVectorX*t1 + x0
		point1Y := lineVectorY*t1 + y0
		point1Z := lineVectorZ*t1 + z0
		if meetsAllBoundsXYZ(point1X, point1Y, point1Z, bounds) &&
			meetsAllBoundsXYZ(point1X, point1Y, point1Z, moreBounds) {
			return true
		}
		point2X := lineVectorX*t2 + x0
		point2Y := lineVectorY*t2 + y0
		point2Z := lineVectorZ*t2 + z0
		return meetsAllBoundsXYZ(point2X, point2Y, point2Z, bounds) &&
			meetsAllBoundsXYZ(point2X, point2Y, point2Z, moreBounds)
	default:
		return false
	}
}

// GetSampleIntersectionPoint returns one point on the intersection between this
// plane, q, and the world, or nil if there is none.
//
// Port of Plane.getSampleIntersectionPoint.
func (p *Plane) GetSampleIntersectionPoint(pm *PlanetModel, q *Plane) *GeoPoint {
	intersections := p.findIntersections(pm, q, nil, nil)
	if len(intersections) == 0 {
		return nil
	}
	return intersections[0]
}

// meetsAllBoundsXYZ reports whether (x,y,z) is within every bound.
//
// Port of Plane.meetsAllBounds(double,double,double,Membership[]).
func meetsAllBoundsXYZ(x, y, z float64, bounds []Membership) bool {
	for _, b := range bounds {
		if b != nil && !b.IsWithin(x, y, z) {
			return false
		}
	}
	return true
}

// meetsAllBoundsVector reports whether the point is within both bound sets.
//
// Port of Plane.meetsAllBounds(Vector,Membership[],Membership[]).
func meetsAllBoundsVector(p *GeoPoint, bounds, moreBounds []Membership) bool {
	return meetsAllBoundsXYZ(p.X, p.Y, p.Z, bounds) &&
		meetsAllBoundsXYZ(p.X, p.Y, p.Z, moreBounds)
}

// recordBoundsAddPoint mirrors the private static Plane.addPoint helper: it
// admits point to boundsInfo only when point satisfies every supplied
// Membership bound.
//
// Port of Plane.addPoint(Bounds,Membership[],GeoPoint).
func recordBoundsAddPoint(boundsInfo *XYZBounds, bounds []Membership, point *GeoPoint) {
	for _, b := range bounds {
		if b != nil && !b.IsWithin(point.X, point.Y, point.Z) {
			return
		}
	}
	boundsInfo.AddPoint(point)
}

// RecordBounds accumulates (x,y,z) bounds information for this plane intersected
// with the planet surface, updating boundsInfo with the extrema points found
// within the supplied Membership bounds. It is the single-plane variant used by
// XYZBounds.AddPlane; the intersection-plane variant (two planes) is deferred to
// rmp #4773.
//
// Port of org.apache.lucene.spatial3d.geom.Plane.recordBounds(PlanetModel,
// XYZBounds, Membership...). The Lagrange-multiplier extrema math and the
// Z-arc vertical-plane intersection follow the reference exactly; the
// extensive derivation comments are omitted here (see the Lucene source).
func (p *Plane) RecordBounds(pm *PlanetModel, boundsInfo *XYZBounds, bounds ...Membership) {
	a := p.X
	b := p.Y
	c := p.Z

	// Do Z. Symmetrical: intersect a vertical plane chosen by the x-y
	// orientation with this plane and the ellipsoid.
	if !boundsInfo.isSmallestMinZ(pm) || !boundsInfo.isLargestMaxZ(pm) {
		if math.Abs(a) >= MinimumResolution || math.Abs(b) >= MinimumResolution {
			normalizedZPlane := ConstructNormalizedZPlaneXY(a, b)
			points := p.findIntersections(pm, normalizedZPlane, bounds, NoBounds)
			for _, point := range points {
				recordBoundsAddPoint(boundsInfo, bounds, point)
			}
		} else {
			// a==b==0: any plane including the Z axis suffices.
			points := p.findIntersections(pm, NormalYPlane, NoBounds, NoBounds)
			if len(points) == 0 {
				points = p.findIntersections(pm, NormalXPlane, NoBounds, NoBounds)
			}
			if len(points) == 0 {
				boundsInfo.AddZValue(NewGeoPoint(0.0, 0.0, -p.Z))
			} else {
				boundsInfo.AddZValue(points[0])
			}
		}
	}

	// Common subexpressions.
	k := 1.0 /
		((p.X*p.X+p.Y*p.Y)*pm.XYScaling*pm.XYScaling +
			p.Z*p.Z*pm.ZScaling*pm.ZScaling)
	abSquared := pm.XYScaling * pm.XYScaling
	cSquared := pm.ZScaling * pm.ZScaling
	aSquared := a * a
	bSquared := b * b
	cValSquared := c * c

	r := 2.0 * p.D * k
	rSquared := r * r

	// Do X via Lagrange multipliers.
	if !boundsInfo.isSmallestMinX(pm) || !boundsInfo.isLargestMaxX(pm) {
		q := a * abSquared * k
		qSquared := q * q

		quadA := aSquared*abSquared*rSquared +
			bSquared*abSquared*rSquared +
			cValSquared*cSquared*rSquared -
			4.0
		quadB := -2.0*a*abSquared*r +
			2.0*aSquared*abSquared*r*q +
			2.0*bSquared*abSquared*r*q +
			2.0*cValSquared*cSquared*r*q
		quadC := abSquared -
			2.0*a*abSquared*q +
			aSquared*abSquared*qSquared +
			bSquared*abSquared*qSquared +
			cValSquared*cSquared*qSquared

		if math.Abs(quadA) >= MinimumResolutionSquared {
			sqrtTerm := quadB*quadB - 4.0*quadA*quadC
			switch {
			case math.Abs(sqrtTerm) < MinimumResolutionSquared:
				m := -quadB / (2.0 * quadA)
				if math.Abs(m) >= MinimumResolution {
					l := r*m + q
					denom0 := 0.5 / m
					thePoint := NewGeoPoint(
						(1.0-l*a)*abSquared*denom0,
						-l*b*abSquared*denom0,
						-l*c*cSquared*denom0)
					recordBoundsAddPoint(boundsInfo, bounds, thePoint)
				} else {
					boundsInfo.addXValueRaw(-p.D / a)
				}
			case sqrtTerm > 0.0:
				sqrtResult := math.Sqrt(sqrtTerm)
				commonDenom := 0.5 / quadA
				m1 := (-quadB + sqrtResult) * commonDenom
				m2 := (-quadB - sqrtResult) * commonDenom
				if math.Abs(m1) >= MinimumResolution || math.Abs(m2) >= MinimumResolution {
					l1 := r*m1 + q
					l2 := r*m2 + q
					denom1 := 0.5 / m1
					denom2 := 0.5 / m2
					recordBoundsAddPoint(boundsInfo, bounds, NewGeoPoint(
						(1.0-l1*a)*abSquared*denom1,
						-l1*b*abSquared*denom1,
						-l1*c*cSquared*denom1))
					recordBoundsAddPoint(boundsInfo, bounds, NewGeoPoint(
						(1.0-l2*a)*abSquared*denom2,
						-l2*b*abSquared*denom2,
						-l2*c*cSquared*denom2))
				} else {
					boundsInfo.addXValueRaw(-p.D / a)
				}
			}
		} else if math.Abs(quadB) > MinimumResolutionSquared {
			m := -quadC / quadB
			l := r*m + q
			denom0 := 0.5 / m
			recordBoundsAddPoint(boundsInfo, bounds, NewGeoPoint(
				(1.0-l*a)*abSquared*denom0,
				-l*b*abSquared*denom0,
				-l*c*cSquared*denom0))
		}
	}

	// Do Y via Lagrange multipliers.
	if !boundsInfo.isSmallestMinY(pm) || !boundsInfo.isLargestMaxY(pm) {
		q := b * abSquared * k
		qSquared := q * q

		quadA := aSquared*abSquared*rSquared +
			bSquared*abSquared*rSquared +
			cValSquared*cSquared*rSquared -
			4.0
		quadB := 2.0*aSquared*abSquared*r*q -
			2.0*b*abSquared*r +
			2.0*bSquared*abSquared*r*q +
			2.0*cValSquared*cSquared*r*q
		quadC := aSquared*abSquared*qSquared +
			abSquared -
			2.0*b*abSquared*q +
			bSquared*abSquared*qSquared +
			cValSquared*cSquared*qSquared

		if math.Abs(quadA) >= MinimumResolutionSquared {
			sqrtTerm := quadB*quadB - 4.0*quadA*quadC
			switch {
			case math.Abs(sqrtTerm) < MinimumResolutionSquared:
				m := -quadB / (2.0 * quadA)
				if math.Abs(m) >= MinimumResolution {
					l := r*m + q
					denom0 := 0.5 / m
					recordBoundsAddPoint(boundsInfo, bounds, NewGeoPoint(
						-l*a*abSquared*denom0,
						(1.0-l*b)*abSquared*denom0,
						-l*c*cSquared*denom0))
				} else {
					boundsInfo.addYValueRaw(-p.D / b)
				}
			case sqrtTerm > 0.0:
				sqrtResult := math.Sqrt(sqrtTerm)
				commonDenom := 0.5 / quadA
				m1 := (-quadB + sqrtResult) * commonDenom
				m2 := (-quadB - sqrtResult) * commonDenom
				if math.Abs(m1) >= MinimumResolution || math.Abs(m2) >= MinimumResolution {
					l1 := r*m1 + q
					l2 := r*m2 + q
					denom1 := 0.5 / m1
					denom2 := 0.5 / m2
					recordBoundsAddPoint(boundsInfo, bounds, NewGeoPoint(
						-l1*a*abSquared*denom1,
						(1.0-l1*b)*abSquared*denom1,
						-l1*c*cSquared*denom1))
					recordBoundsAddPoint(boundsInfo, bounds, NewGeoPoint(
						-l2*a*abSquared*denom2,
						(1.0-l2*b)*abSquared*denom2,
						-l2*c*cSquared*denom2))
				} else {
					boundsInfo.addYValueRaw(-p.D / b)
				}
			}
		} else if math.Abs(quadB) > MinimumResolutionSquared {
			m := -quadC / quadB
			l := r*m + q
			denom0 := 0.5 / m
			recordBoundsAddPoint(boundsInfo, bounds, NewGeoPoint(
				-l*a*abSquared*denom0,
				(1.0-l*b)*abSquared*denom0,
				-l*c*cSquared*denom0))
		}
	}
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
