// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom

import (
	"fmt"
	"io"
	"math"
)

// GeoPoint is a point on the sphere surface. It extends Vector with
// lazy-computed latitude, longitude, and magnitude fields.
//
// Port of org.apache.lucene.spatial3d.geom.GeoPoint.
type GeoPoint struct {
	Vector
	magnitude float64
	latitude  float64
	longitude float64
}

// NewGeoPoint creates a point from normalised Cartesian coordinates.
func NewGeoPoint(x, y, z float64) *GeoPoint {
	return &GeoPoint{Vector: Vector{X: x, Y: y, Z: z}, magnitude: math.NaN()}
}

// NewGeoPointLatLon creates a point from lat/lon in radians on a planet model.
func NewGeoPointLatLon(pm *PlanetModel, lat, lon float64) *GeoPoint {
	sinLat, cosLat := math.Sin(lat), math.Cos(lat)
	sinLon, cosLon := math.Sin(lon), math.Cos(lon)
	x := pm.A * cosLat * cosLon
	y := pm.A * cosLat * sinLon
	z := pm.B * sinLat
	return &GeoPoint{
		Vector:    Vector{X: x, Y: y, Z: z},
		latitude:  lat,
		longitude: lon,
		magnitude: math.NaN(),
	}
}

// NewGeoPointFromStream deserialises from a stream.
//
// Wire format (Lucene 10.4.0 GeoPoint(InputStream)):
// 5 doubles in SerializableObject order: lat, lon, x, y, z.
// Each double is written as a little-endian int64 (writeLong = writeInt(lo) + writeInt(hi)).
//
// Port of GeoPoint(InputStream).
func NewGeoPointFromStream(_ *PlanetModel, r io.Reader) (*GeoPoint, error) {
	lat, err := readSerialDouble(r)
	if err != nil {
		return nil, fmt.Errorf("geopoint: read lat: %w", err)
	}
	lon, err := readSerialDouble(r)
	if err != nil {
		return nil, fmt.Errorf("geopoint: read lon: %w", err)
	}
	x, err := readSerialDouble(r)
	if err != nil {
		return nil, fmt.Errorf("geopoint: read x: %w", err)
	}
	y, err := readSerialDouble(r)
	if err != nil {
		return nil, fmt.Errorf("geopoint: read y: %w", err)
	}
	z, err := readSerialDouble(r)
	if err != nil {
		return nil, fmt.Errorf("geopoint: read z: %w", err)
	}
	return &GeoPoint{
		Vector:    Vector{X: x, Y: y, Z: z},
		latitude:  lat,
		longitude: lon,
		magnitude: math.NaN(),
	}, nil
}

// GetLatitude returns the latitude in radians, computing it lazily.
func (p *GeoPoint) GetLatitude() float64 {
	if !math.IsInf(p.latitude, 0) && !math.IsNaN(p.latitude) {
		return p.latitude
	}
	p.latitude = math.Asin(p.Z / p.getMagnitude())
	return p.latitude
}

// GetLongitude returns the longitude in radians, computing it lazily.
func (p *GeoPoint) GetLongitude() float64 {
	if !math.IsInf(p.longitude, 0) && !math.IsNaN(p.longitude) {
		return p.longitude
	}
	p.longitude = math.Atan2(p.Y, p.X)
	return p.longitude
}

// Magnitude returns the Euclidean magnitude of this point.
func (p *GeoPoint) Magnitude() float64 {
	return p.getMagnitude()
}

func (p *GeoPoint) getMagnitude() float64 {
	if math.IsNaN(p.magnitude) {
		p.magnitude = Magnitude(p.X, p.Y, p.Z)
	}
	return p.magnitude
}

// ArcDistance returns the arc distance (radians) from this point to v.
func (p *GeoPoint) ArcDistance(v *Vector) float64 {
	d := p.DotProduct(v)
	m1, m2 := p.getMagnitude(), v.Magnitude()
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
	return math.Acos(cos)
}

// IsIdentical reports whether p is within tolerance of other.
func (p *GeoPoint) IsIdentical(other *GeoPoint) bool {
	if other == nil {
		return false
	}
	return p.IsNumericallyIdentical(other.X, other.Y, other.Z)
}

// Write serialises the point to the stream.
//
// Wire format (Lucene 10.4.0 GeoPoint.write(OutputStream)):
// 5 doubles in order: lat, lon, x, y, z, each as a little-endian int64.
//
// Port of GeoPoint.write(OutputStream).
func (p *GeoPoint) Write(w io.Writer) error {
	for _, v := range [5]float64{p.GetLatitude(), p.GetLongitude(), p.X, p.Y, p.Z} {
		if err := writeSerialDouble(w, v); err != nil {
			return fmt.Errorf("geopoint: write: %w", err)
		}
	}
	return nil
}

// String returns a debug representation.
func (p *GeoPoint) String() string {
	return "GeoPoint(" + fmtFloat(p.X) + "," + fmtFloat(p.Y) + "," + fmtFloat(p.Z) + ")"
}
