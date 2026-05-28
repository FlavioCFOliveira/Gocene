// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom

import (
	"errors"
	"math"
)

// Minimum resolution constants matching the Java originals.
const (
	// MinimumResolution is the absolute minimum value treated as non-zero.
	MinimumResolution = 1.0e-12
	// MinimumAngularResolution is Math.PI * MinimumResolution.
	MinimumAngularResolution = math.Pi * MinimumResolution
	// MinimumResolutionSquared is MinimumResolution * MinimumResolution.
	MinimumResolutionSquared = MinimumResolution * MinimumResolution
	// MinimumResolutionCubed is MinimumResolutionSquared * MinimumResolution.
	MinimumResolutionCubed = MinimumResolutionSquared * MinimumResolution

	// minimumGramSchmidtEnvelope is the convergence envelope used while
	// refining a perpendicular vector. It is a bit smaller than the minimum
	// resolution so the math does not subsequently fail in other places.
	//
	// Port of Vector.MINIMUM_GRAM_SCHMIDT_ENVELOPE.
	minimumGramSchmidtEnvelope = MinimumResolution * 0.5
)

// ErrDegenerateVector is returned when a perpendicular vector cannot be
// constructed because the two source vectors are parallel or degenerate.
//
// Port of the IllegalArgumentException thrown by the Vector(A,B) constructor.
var ErrDegenerateVector = errors.New("geom: degenerate/parallel vector constructed")

// Vector is a 3-D vector (or point) in Cartesian space.
//
// Port of org.apache.lucene.spatial3d.geom.Vector.
type Vector struct {
	X float64
	Y float64
	Z float64
}

// NewVector creates a vector from Cartesian coordinates.
func NewVector(x, y, z float64) *Vector { return &Vector{X: x, Y: y, Z: z} }

// NewVectorCrossProduct creates the cross product of A and B.
func NewVectorCrossProduct(a, b *Vector) *Vector {
	return &Vector{
		X: a.Y*b.Z - a.Z*b.Y,
		Y: a.Z*b.X - a.X*b.Z,
		Z: a.X*b.Y - a.Y*b.X,
	}
}

// NewVectorPerpendicular constructs a normalised vector perpendicular to the
// two non-zero vectors A=(ax,ay,az) and B=(bx,by,bz). It returns
// ErrDegenerateVector if the vectors are parallel.
//
// The result is refined with the Gram-Schmidt process so that the dot product
// between the normal and each of the two source vectors falls below the
// convergence envelope, mirroring Lucene's numerical-precision handling.
//
// Port of org.apache.lucene.spatial3d.geom.Vector(double,double,double,double,double,double).
func NewVectorPerpendicular(ax, ay, az, bx, by, bz float64) (*Vector, error) {
	// Compute the naive perpendicular.
	thisX := ay*bz - az*by
	thisY := az*bx - ax*bz
	thisZ := ax*by - ay*bx

	magnitude := Magnitude(thisX, thisY, thisZ)
	if magnitude == 0.0 {
		return nil, ErrDegenerateVector
	}
	inverseMagnitude := 1.0 / magnitude

	normalizeX := thisX * inverseMagnitude
	normalizeY := thisY * inverseMagnitude
	normalizeZ := thisZ * inverseMagnitude
	// For a plane to work, the dot product between the normal vector and the
	// points needs to be less than the minimum resolution. This is sometimes
	// not true for points that are very close, so we converge with Gram-Schmidt.
	for i := 0; ; i++ {
		currentDotProdA := ax*normalizeX + ay*normalizeY + az*normalizeZ
		currentDotProdB := bx*normalizeX + by*normalizeY + bz*normalizeZ
		if math.Abs(currentDotProdA) < minimumGramSchmidtEnvelope &&
			math.Abs(currentDotProdB) < minimumGramSchmidtEnvelope {
			break
		}
		// Converge on the one that has the largest dot product.
		var currentVectorX, currentVectorY, currentVectorZ, currentDotProd float64
		if math.Abs(currentDotProdA) > math.Abs(currentDotProdB) {
			currentVectorX, currentVectorY, currentVectorZ = ax, ay, az
			currentDotProd = currentDotProdA
		} else {
			currentVectorX, currentVectorY, currentVectorZ = bx, by, bz
			currentDotProd = currentDotProdB
		}
		// Adjust.
		normalizeX -= currentDotProd * currentVectorX
		normalizeY -= currentDotProd * currentVectorY
		normalizeZ -= currentDotProd * currentVectorZ
		// Normalize.
		correctedMagnitude := Magnitude(normalizeX, normalizeY, normalizeZ)
		inverseCorrectedMagnitude := 1.0 / correctedMagnitude
		normalizeX *= inverseCorrectedMagnitude
		normalizeY *= inverseCorrectedMagnitude
		normalizeZ *= inverseCorrectedMagnitude
		// Safety valve; the method normally converges quickly.
		if i > 10 {
			return nil, ErrDegenerateVector
		}
	}
	return &Vector{X: normalizeX, Y: normalizeY, Z: normalizeZ}, nil
}

// NewVectorPerpendicularFromVectors is the two-vector form of
// NewVectorPerpendicular.
//
// Port of org.apache.lucene.spatial3d.geom.Vector(Vector,Vector).
func NewVectorPerpendicularFromVectors(a, b *Vector) (*Vector, error) {
	return NewVectorPerpendicular(a.X, a.Y, a.Z, b.X, b.Y, b.Z)
}

// CrossProductEvaluateIsZero evaluates the (Gram-Schmidt refined) perpendicular
// of A and B against point, returning true when the dot product resolves to
// "zero" (within the minimum resolution) or the vectors are parallel.
//
// Port of org.apache.lucene.spatial3d.geom.Vector.crossProductEvaluateIsZero.
func CrossProductEvaluateIsZero(a, b, point *Vector) bool {
	perp, err := NewVectorPerpendicularFromVectors(a, b)
	if err != nil {
		// Magnitude zero => parallel => treated as coplanar.
		return true
	}
	return math.Abs(perp.X*point.X+perp.Y*point.Y+perp.Z*point.Z) < MinimumResolution
}

// computeDesiredEllipsoidMagnitude returns the magnitude that projects the unit
// vector (x,y,z) onto the given planet's ellipsoid surface.
//
// Port of org.apache.lucene.spatial3d.geom.Vector.computeDesiredEllipsoidMagnitude.
func computeDesiredEllipsoidMagnitude(pm *PlanetModel, x, y, z float64) float64 {
	return 1.0 / math.Sqrt(x*x*pm.InverseXYScalingSquared+
		y*y*pm.InverseXYScalingSquared+
		z*z*pm.InverseZScalingSquared)
}

// computeDesiredEllipsoidMagnitudeZ returns the ellipsoid magnitude for a unit
// vector specified only by its z value.
//
// Port of org.apache.lucene.spatial3d.geom.Vector.computeDesiredEllipsoidMagnitude(PlanetModel,double).
func computeDesiredEllipsoidMagnitudeZ(pm *PlanetModel, z float64) float64 {
	return 1.0 / math.Sqrt((1.0-z*z)*pm.InverseXYScalingSquared+
		z*z*pm.InverseZScalingSquared)
}

// Magnitude returns sqrt(x²+y²+z²) for the given components.
func Magnitude(x, y, z float64) float64 {
	return math.Sqrt(x*x + y*y + z*z)
}

// Normalize returns the unit vector in the same direction.
func (v *Vector) Normalize() *Vector {
	m := v.Magnitude()
	if m < MinimumResolution {
		return &Vector{}
	}
	return &Vector{X: v.X / m, Y: v.Y / m, Z: v.Z / m}
}

// Magnitude returns the Euclidean length of the vector.
func (v *Vector) Magnitude() float64 {
	return Magnitude(v.X, v.Y, v.Z)
}

// DotProduct returns the dot product with another vector.
func (v *Vector) DotProduct(other *Vector) float64 {
	return v.X*other.X + v.Y*other.Y + v.Z*other.Z
}

// DotProductXYZ returns the dot product with (x,y,z).
func (v *Vector) DotProductXYZ(x, y, z float64) float64 {
	return v.X*x + v.Y*y + v.Z*z
}

// IsWithin reports whether point (x,y,z) satisfies all membership bounds.
func (v *Vector) IsWithin(bounds []Membership, moreBounds ...Membership) bool {
	for _, b := range bounds {
		if !b.IsWithin(v.X, v.Y, v.Z) {
			return false
		}
	}
	for _, b := range moreBounds {
		if !b.IsWithin(v.X, v.Y, v.Z) {
			return false
		}
	}
	return true
}

// Translate returns a new vector translated by the given offsets.
//
// Matches Lucene's Vector.translate, which subtracts the offsets.
func (v *Vector) Translate(xOffset, yOffset, zOffset float64) *Vector {
	return &Vector{X: v.X - xOffset, Y: v.Y - yOffset, Z: v.Z - zOffset}
}

// IsParallel reports whether this vector is parallel to other (cross-product
// magnitude squared below the squared minimum resolution).
//
// Port of org.apache.lucene.spatial3d.geom.Vector.isParallel(Vector).
func (v *Vector) IsParallel(other *Vector) bool {
	return v.IsParallelXYZ(other.X, other.Y, other.Z)
}

// RotateXY rotates by angle in the XY plane.
func (v *Vector) RotateXY(angle float64) *Vector {
	return v.RotateXYSinCos(math.Sin(angle), math.Cos(angle))
}

// RotateXYSinCos rotates in the XY plane using precomputed sin/cos.
func (v *Vector) RotateXYSinCos(sinAngle, cosAngle float64) *Vector {
	return &Vector{
		X: v.X*cosAngle - v.Y*sinAngle,
		Y: v.X*sinAngle + v.Y*cosAngle,
		Z: v.Z,
	}
}

// RotateXZ rotates by angle in the XZ plane.
func (v *Vector) RotateXZ(angle float64) *Vector {
	return v.RotateXZSinCos(math.Sin(angle), math.Cos(angle))
}

// RotateXZSinCos rotates in the XZ plane using precomputed sin/cos.
func (v *Vector) RotateXZSinCos(sinAngle, cosAngle float64) *Vector {
	return &Vector{
		X: v.X*cosAngle - v.Z*sinAngle,
		Y: v.Y,
		Z: v.X*sinAngle + v.Z*cosAngle,
	}
}

// RotateZY rotates by angle in the ZY plane.
func (v *Vector) RotateZY(angle float64) *Vector {
	return v.RotateZYSinCos(math.Sin(angle), math.Cos(angle))
}

// RotateZYSinCos rotates in the ZY plane using precomputed sin/cos.
func (v *Vector) RotateZYSinCos(sinAngle, cosAngle float64) *Vector {
	return &Vector{
		X: v.X,
		Y: v.Y*cosAngle - v.Z*sinAngle,
		Z: v.Y*sinAngle + v.Z*cosAngle,
	}
}

// LinearDistanceSquared returns the squared Euclidean distance to v.
func (v *Vector) LinearDistanceSquared(other *Vector) float64 {
	dx, dy, dz := v.X-other.X, v.Y-other.Y, v.Z-other.Z
	return dx*dx + dy*dy + dz*dz
}

// LinearDistance returns the Euclidean distance to v.
func (v *Vector) LinearDistance(other *Vector) float64 {
	return math.Sqrt(v.LinearDistanceSquared(other))
}

// NormalDistanceSquared returns the squared normal distance.
func (v *Vector) NormalDistanceSquared(other *Vector) float64 {
	d := v.DotProduct(other)
	m1, m2 := v.Magnitude(), other.Magnitude()
	if m1 == 0 || m2 == 0 {
		return 0
	}
	cos := d / (m1 * m2)
	if cos > 1 {
		cos = 1
	}
	if cos < -1 {
		cos = -1
	}
	sin := math.Sqrt(1 - cos*cos)
	return sin * sin
}

// NormalDistance returns the normal distance (0 to 1).
func (v *Vector) NormalDistance(other *Vector) float64 {
	return math.Sqrt(v.NormalDistanceSquared(other))
}

// IsNumericallyIdentical reports whether (x,y,z) is within tolerance.
func (v *Vector) IsNumericallyIdentical(x, y, z float64) bool {
	dx, dy, dz := v.X-x, v.Y-y, v.Z-z
	return dx*dx+dy*dy+dz*dz < MinimumResolutionSquared
}

// IsParallelXYZ reports whether the vector is parallel to (x,y,z).
func (v *Vector) IsParallelXYZ(x, y, z float64) bool {
	// cross product magnitude² < eps²
	cx := v.Y*z - v.Z*y
	cy := v.Z*x - v.X*z
	cz := v.X*y - v.Y*x
	return cx*cx+cy*cy+cz*cz < MinimumResolutionSquared
}

// String returns a debug representation.
func (v *Vector) String() string {
	return "[" + fmtFloat(v.X) + "," + fmtFloat(v.Y) + "," + fmtFloat(v.Z) + "]"
}

// Equals reports value equality.
func (v *Vector) Equals(other *Vector) bool {
	if other == nil {
		return false
	}
	return v.X == other.X && v.Y == other.Y && v.Z == other.Z
}

// HashCode returns a simple hash.
func (v *Vector) HashCode() int {
	h := 31
	h = h*31 + floatBits(v.X)
	h = h*31 + floatBits(v.Y)
	h = h*31 + floatBits(v.Z)
	return h
}

func floatBits(f float64) int {
	bits := math.Float64bits(f)
	return int(bits ^ (bits >> 32))
}
