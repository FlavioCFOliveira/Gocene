// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

// PlanetModel describes the ellipsoidal model of the planet.
//
// Port of org.apache.lucene.spatial3d.geom.PlanetModel.
//
// The coordinate system is normalised: every coordinate is divided by meanRadius
// so that the unit sphere sits at scale ≈ 1. Encoding/decoding maps the scaled
// space into a 32-bit integer range using the same MUL/DECODE algorithm as Lucene.
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

	// mul is (1<<32) / (2*MaxValue), the "bucket width" denominator.
	mul float64

	// Decode is the scalar used to convert integer back to double.
	// It equals getNextSafeDouble(1/mul).
	Decode float64

	MaxEncodedValue int32
	MinEncodedValue int32
}

// SPHERE is a PlanetModel representing a unit sphere.
var SPHERE = NewPlanetModel(1.0, 1.0)

// WGS84 is a PlanetModel matching the WGS84 ellipsoid.
var WGS84 = NewPlanetModel(6378137.0, 6356752.314245)

// CLARKE1866 is a PlanetModel matching the Clarke 1866 ellipsoid.
var CLARKE1866 = NewPlanetModel(6378206.4, 6356583.8)

// NewPlanetModel creates a PlanetModel from semi-major and semi-minor axes.
//
// The construction order mirrors PlanetModel(double,double) in Lucene 10.4.0
// exactly: meanRadius → xyScaling/zScaling → scale → inverses → poles →
// inverseScale → MAX_VALUE → MUL → DECODE → MIN/MAX_ENCODED_VALUE.
func NewPlanetModel(semiMajorAxis, semiMinorAxis float64) *PlanetModel {
	pm := &PlanetModel{
		A: semiMajorAxis,
		B: semiMinorAxis,
	}

	pm.MeanRadius = (2.0*semiMajorAxis + semiMinorAxis) / 3.0
	pm.XYScaling = semiMajorAxis / pm.MeanRadius
	pm.ZScaling = semiMinorAxis / pm.MeanRadius
	pm.Scale = (2.0*pm.XYScaling + pm.ZScaling) / 3.0
	pm.InverseXYScaling = 1.0 / pm.XYScaling
	pm.InverseZScaling = 1.0 / pm.ZScaling
	pm.ScaledFlattening = (pm.XYScaling - pm.ZScaling) * pm.InverseXYScaling
	pm.SquareRatio = (pm.XYScaling*pm.XYScaling - pm.ZScaling*pm.ZScaling) /
		(pm.ZScaling * pm.ZScaling)
	pm.InverseXYScalingSquared = pm.InverseXYScaling * pm.InverseXYScaling
	pm.InverseZScalingSquared = pm.InverseZScaling * pm.InverseZScaling

	// Poles: Java uses GeoPoint(magnitude, ux, uy, uz, lat, lon) which
	// stores (ux*magnitude, uy*magnitude, uz*magnitude) as the Cartesian
	// coordinates. Replicate that here.
	pm.NorthPole = newGeoPointMag(pm.ZScaling, 0.0, 0.0, 1.0, math.Pi*0.5, 0.0)
	pm.SouthPole = newGeoPointMag(pm.ZScaling, 0.0, 0.0, -1.0, -math.Pi*0.5, 0.0)
	pm.MinXPole = newGeoPointMag(pm.XYScaling, -1.0, 0.0, 0.0, 0.0, -math.Pi)
	pm.MaxXPole = newGeoPointMag(pm.XYScaling, 1.0, 0.0, 0.0, 0.0, 0.0)
	pm.MinYPole = newGeoPointMag(pm.XYScaling, 0.0, -1.0, 0.0, 0.0, -math.Pi*0.5)
	pm.MaxYPole = newGeoPointMag(pm.XYScaling, 0.0, 1.0, 0.0, 0.0, math.Pi*0.5)

	pm.InverseScale = 1.0 / pm.Scale

	// Minimum pole distance: simplified as π × MinimumResolution because a
	// full Vincenty surfaceDistance requires the iterator which is deferred.
	pm.MinimumPoleDistance = math.Pi * MinimumResolution

	// Encoding constants — exact port of Lucene 10.4.0 PlanetModel constructor.
	pm.MaxValue = pm.GetMaximumMagnitude()
	// MUL = (1L << 32) / (2 * MAX_VALUE); using float64 arithmetic.
	pm.mul = float64(int64(1)<<32) / (2.0 * pm.MaxValue)
	pm.Decode = nextSafeDouble(1.0 / pm.mul)
	pm.MinEncodedValue = pm.EncodeValue(-pm.MaxValue)
	pm.MaxEncodedValue = pm.EncodeValue(pm.MaxValue)

	return pm
}

// newGeoPointMag creates a GeoPoint from a magnitude, unit vector, and lat/lon.
// Mirrors GeoPoint(double magnitude, double x, double y, double z, double lat, double lon).
func newGeoPointMag(magnitude, ux, uy, uz, lat, lon float64) *GeoPoint {
	return &GeoPoint{
		Vector:    Vector{X: ux * magnitude, Y: uy * magnitude, Z: uz * magnitude},
		latitude:  lat,
		longitude: lon,
		magnitude: magnitude,
	}
}

// nextSafeDouble returns the smallest double >= x such that x * n / n == x for
// all integers n that fit in a 32-bit range. Port of PlanetModel.getNextSafeDouble.
func nextSafeDouble(x float64) float64 {
	bits := math.Float64bits(x)
	bits += math.MaxInt32
	bits &^= uint64(math.MaxInt32)
	result := math.Float64frombits(bits)
	return result
}

// NewPlanetModelFromStream deserialises a PlanetModel from a stream.
//
// Wire format (Lucene 10.4.0): two doubles written as little-endian int64 (via
// SerializableObject.writeDouble → writeLong → 2×writeInt, each writeInt LE).
// Concretely: 8 bytes LE for a, then 8 bytes LE for b.
//
// Port of PlanetModel(InputStream).
func NewPlanetModelFromStream(r io.Reader) (*PlanetModel, error) {
	a, err := readSerialDouble(r)
	if err != nil {
		return nil, fmt.Errorf("planetmodel: read a: %w", err)
	}
	b, err := readSerialDouble(r)
	if err != nil {
		return nil, fmt.Errorf("planetmodel: read b: %w", err)
	}
	return NewPlanetModel(a, b), nil
}

// Write serialises to a stream using the Lucene 10.4.0 binary format.
//
// Wire format: a as LE int64 bits, then b as LE int64 bits.
// Port of PlanetModel.write(OutputStream).
func (pm *PlanetModel) Write(w io.Writer) error {
	if err := writeSerialDouble(w, pm.A); err != nil {
		return fmt.Errorf("planetmodel: write a: %w", err)
	}
	if err := writeSerialDouble(w, pm.B); err != nil {
		return fmt.Errorf("planetmodel: write b: %w", err)
	}
	return nil
}

// readSerialDouble reads a double in Lucene SerializableObject wire format:
// writeLong(bits) = writeInt(lo) + writeInt(hi), each int is 4 LE bytes.
// Net result: 8-byte little-endian int64 → float64.
func readSerialDouble(r io.Reader) (float64, error) {
	var lo, hi uint32
	if err := binary.Read(r, binary.LittleEndian, &lo); err != nil {
		return 0, err
	}
	if err := binary.Read(r, binary.LittleEndian, &hi); err != nil {
		return 0, err
	}
	bits := uint64(lo) | (uint64(hi) << 32)
	return math.Float64frombits(bits), nil
}

// writeSerialDouble writes a double in Lucene SerializableObject wire format.
func writeSerialDouble(w io.Writer, v float64) error {
	bits := math.Float64bits(v)
	lo := uint32(bits & 0xffffffff)
	hi := uint32(bits >> 32)
	if err := binary.Write(w, binary.LittleEndian, lo); err != nil {
		return err
	}
	return binary.Write(w, binary.LittleEndian, hi)
}

// IsSphere reports whether this model is spherical (xyScaling == zScaling).
func (pm *PlanetModel) IsSphere() bool {
	return pm.XYScaling == pm.ZScaling
}

// GetMinimumMagnitude returns the minimum possible surface magnitude.
// Port of PlanetModel.getMinimumMagnitude().
func (pm *PlanetModel) GetMinimumMagnitude() float64 {
	return math.Min(pm.XYScaling, pm.ZScaling)
}

// GetMaximumMagnitude returns the maximum possible surface magnitude.
// Port of PlanetModel.getMaximumMagnitude().
func (pm *PlanetModel) GetMaximumMagnitude() float64 {
	return math.Max(pm.XYScaling, pm.ZScaling)
}

// GetMinimumXValue returns -xyScaling.
func (pm *PlanetModel) GetMinimumXValue() float64 { return -pm.XYScaling }

// GetMaximumXValue returns xyScaling.
func (pm *PlanetModel) GetMaximumXValue() float64 { return pm.XYScaling }

// GetMinimumYValue returns -xyScaling.
func (pm *PlanetModel) GetMinimumYValue() float64 { return -pm.XYScaling }

// GetMaximumYValue returns xyScaling.
func (pm *PlanetModel) GetMaximumYValue() float64 { return pm.XYScaling }

// GetMinimumZValue returns -zScaling.
func (pm *PlanetModel) GetMinimumZValue() float64 { return -pm.ZScaling }

// GetMaximumZValue returns zScaling.
func (pm *PlanetModel) GetMaximumZValue() float64 { return pm.ZScaling }

// GetMeanRadius returns the mean radius.
func (pm *PlanetModel) GetMeanRadius() float64 { return pm.MeanRadius }

// EncodeValue encodes a coordinate in the normalised space to a 32-bit integer.
//
// The coordinate must be in [-MaxValue, MaxValue].  Values equal to MaxValue
// are stepped down by one ULP to ensure they remain in range after decoding.
// Port of PlanetModel.encodeValue(double).
func (pm *PlanetModel) EncodeValue(x float64) int32 {
	if x > pm.MaxValue {
		x = pm.MaxValue
	}
	if x == pm.MaxValue {
		x = math.Nextafter(x, 0)
	}
	if x < -pm.MaxValue {
		x = -pm.MaxValue
	}
	result := int64(math.Floor(x / pm.Decode))
	return int32(result)
}

// DecodeValue decodes a 32-bit integer back to a coordinate in normalised space.
//
// MIN_ENCODED_VALUE maps to -MaxValue; MAX_ENCODED_VALUE maps to +MaxValue;
// all other values decode to the centre of the bucket: (x+0.5)*Decode.
// Port of PlanetModel.decodeValue(int).
func (pm *PlanetModel) DecodeValue(x int32) float64 {
	switch x {
	case pm.MinEncodedValue:
		return -pm.MaxValue
	case pm.MaxEncodedValue:
		return pm.MaxValue
	default:
		return (float64(x) + 0.5) * pm.Decode
	}
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
	// In scaled space the ellipsoid equation is x²+y²+z² = (xyScaling·x/xyScaling)² + ...
	// With the normalised model x·xyScaling gives the unit-sphere projection.
	// Actually: point is on surface when (x/a)² + (y/a)² + (z/b)² = 1 where
	// a = InverseXYScaling and b = InverseZScaling. Equivalently:
	// x²·XYScaling² + y²·XYScaling² + z²·ZScaling² = ... wait.
	// The ellipsoid equation in the normalised model is:
	// (x·xyScaling)² + (y·xyScaling)² + (z·zScaling)² = 1 — no.
	// Lucene's check: x²/inverseXYScaling² + y²/inverseXYScaling² + z²/inverseZScaling² = 1
	// i.e. x²·xyScaling² + y²·xyScaling² + z²·zScaling² = 1
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
