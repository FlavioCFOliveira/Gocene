// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom

import (
	"io"
	"math"
)

// PlanetModel describes the ellipsoidal model of the planet.
//
// Port of org.apache.lucene.spatial3d.geom.PlanetModel.
type PlanetModel struct {
	// A is the semi-major axis (equatorial radius).
	A float64
	// B is the semi-minor axis (polar radius).
	B float64

	XYScaling               float64
	ZScaling                float64
	InverseXYScaling        float64
	InverseZScaling         float64
	InverseXYScalingSquared float64
	InverseZScalingSquared  float64
	ScaledFlattening        float64
	SquareRatio             float64
	MeanRadius              float64
	Scale                   float64
	InverseScale            float64

	// Well-known pole and axis-intercept points.
	NorthPole *GeoPoint
	SouthPole *GeoPoint
	MinXPole  *GeoPoint
	MaxXPole  *GeoPoint
	MinYPole  *GeoPoint
	MaxYPole  *GeoPoint

	MinimumPoleDistance float64
	MaxValue            float64
	Decode              float64
	MaxEncodedValue     int
	MinEncodedValue     int
}

// SPHERE is a PlanetModel representing a unit sphere.
var SPHERE = NewPlanetModel(1.0, 1.0)

// WGS84 is a PlanetModel matching the WGS84 ellipsoid.
var WGS84 = NewPlanetModel(6378137.0, 6356752.314245)

// CLARKE1866 is a PlanetModel matching the Clarke 1866 ellipsoid.
var CLARKE1866 = NewPlanetModel(6378206.4, 6356583.8)

// NewPlanetModel creates a PlanetModel from semi-major and semi-minor axes.
func NewPlanetModel(semiMajorAxis, semiMinorAxis float64) *PlanetModel {
	pm := &PlanetModel{
		A: semiMajorAxis,
		B: semiMinorAxis,
	}
	pm.XYScaling = 1.0 / semiMajorAxis
	pm.ZScaling = 1.0 / semiMinorAxis
	pm.InverseXYScaling = semiMajorAxis
	pm.InverseZScaling = semiMinorAxis
	pm.InverseXYScalingSquared = semiMajorAxis * semiMajorAxis
	pm.InverseZScalingSquared = semiMinorAxis * semiMinorAxis
	if semiMajorAxis != semiMinorAxis {
		pm.ScaledFlattening = (semiMajorAxis - semiMinorAxis) / semiMajorAxis
	}
	pm.SquareRatio = (semiMajorAxis*semiMajorAxis - semiMinorAxis*semiMinorAxis) /
		(semiMinorAxis * semiMinorAxis)
	pm.MeanRadius = (2*semiMajorAxis + semiMinorAxis) / 3.0
	pm.Scale = (semiMajorAxis + semiMinorAxis) / 2.0
	if pm.Scale != 0 {
		pm.InverseScale = 1.0 / pm.Scale
	}

	// Pre-compute well-known pole points.
	pm.NorthPole = NewGeoPoint(0, 0, semiMinorAxis)
	pm.SouthPole = NewGeoPoint(0, 0, -semiMinorAxis)
	pm.MinXPole = NewGeoPoint(-semiMajorAxis, 0, 0)
	pm.MaxXPole = NewGeoPoint(semiMajorAxis, 0, 0)
	pm.MinYPole = NewGeoPoint(0, -semiMajorAxis, 0)
	pm.MaxYPole = NewGeoPoint(0, semiMajorAxis, 0)

	pm.MinimumPoleDistance = math.Pi * MinimumResolution
	pm.MaxValue = semiMajorAxis * (1 + MinimumResolution)

	// Encoding constants (simplified; full computation deferred to #2693).
	pm.MaxEncodedValue = math.MaxInt32 / 2
	pm.MinEncodedValue = -pm.MaxEncodedValue
	if pm.MaxValue > 0 {
		pm.Decode = pm.MaxValue / float64(pm.MaxEncodedValue)
	}

	return pm
}

// NewPlanetModelFromStream deserialises from a stream. Deferred to #2693.
func NewPlanetModelFromStream(_ io.Reader) (*PlanetModel, error) { return nil, nil }

// Write serialises to a stream. Deferred to #2693.
func (pm *PlanetModel) Write(_ io.Writer) error { return nil }

// IsSphere reports whether this model is spherical (a == b).
func (pm *PlanetModel) IsSphere() bool {
	return math.Abs(pm.A-pm.B) < MinimumResolution
}

// GetMinimumMagnitude returns the minimum possible surface magnitude.
func (pm *PlanetModel) GetMinimumMagnitude() float64 { return pm.B }

// GetMaximumMagnitude returns the maximum possible surface magnitude.
func (pm *PlanetModel) GetMaximumMagnitude() float64 { return pm.A }

// GetMinimumXValue returns -(A).
func (pm *PlanetModel) GetMinimumXValue() float64 { return -pm.A }

// GetMaximumXValue returns A.
func (pm *PlanetModel) GetMaximumXValue() float64 { return pm.A }

// GetMinimumYValue returns -(A).
func (pm *PlanetModel) GetMinimumYValue() float64 { return -pm.A }

// GetMaximumYValue returns A.
func (pm *PlanetModel) GetMaximumYValue() float64 { return pm.A }

// GetMinimumZValue returns -(B).
func (pm *PlanetModel) GetMinimumZValue() float64 { return -pm.B }

// GetMaximumZValue returns B.
func (pm *PlanetModel) GetMaximumZValue() float64 { return pm.B }

// GetMeanRadius returns the mean radius.
func (pm *PlanetModel) GetMeanRadius() float64 { return pm.MeanRadius }

// EncodeValue encodes x into an integer doc value. Deferred to #2693.
func (pm *PlanetModel) EncodeValue(x float64) int {
	if pm.Decode == 0 {
		return 0
	}
	return int(x / pm.Decode)
}

// DecodeValue decodes an integer doc value back to a float64. Deferred to #2693.
func (pm *PlanetModel) DecodeValue(x int) float64 {
	return float64(x) * pm.Decode
}

// PointOnSurface reports whether vector v lies on the ellipsoid surface.
func (pm *PlanetModel) PointOnSurface(v *Vector) bool {
	return pm.PointOnSurfaceXYZ(v.X, v.Y, v.Z)
}

// PointOnSurfaceXYZ reports whether (x,y,z) lies on the ellipsoid surface.
func (pm *PlanetModel) PointOnSurfaceXYZ(x, y, z float64) bool {
	scaledX := x * pm.XYScaling
	scaledY := y * pm.XYScaling
	scaledZ := z * pm.ZScaling
	val := scaledX*scaledX + scaledY*scaledY + scaledZ*scaledZ
	return math.Abs(val-1.0) < MinimumResolution
}

// Equals reports whether two PlanetModels have the same axes.
func (pm *PlanetModel) Equals(other *PlanetModel) bool {
	if other == nil {
		return false
	}
	return pm.A == other.A && pm.B == other.B
}

// HashCode returns a simple hash.
func (pm *PlanetModel) HashCode() int {
	return floatBits(pm.A)*31 + floatBits(pm.B)
}

// String returns a debug representation.
func (pm *PlanetModel) String() string {
	if pm.IsSphere() {
		return "PlanetModel(sphere,a=" + fmtFloat(pm.A) + ")"
	}
	return "PlanetModel(a=" + fmtFloat(pm.A) + ",b=" + fmtFloat(pm.B) + ")"
}
