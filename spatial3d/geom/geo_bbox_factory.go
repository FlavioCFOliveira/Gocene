// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom

import (
	"math"
)

// GeoBBoxFactory is a factory for GeoBBox.
//
// Port of org.apache.lucene.spatial3d.geom.GeoBBoxFactory.
type GeoBBoxFactory struct{}

// minWideExtent is the longitude span at or above which a rectangle is "wide".
//
// Port of GeoWideRectangle.MIN_WIDE_EXTENT.
const minWideExtent = math.Pi - MinimumAngularResolution

// MakeGeoBBox creates a geobbox of the right kind given the specified bounds.
//
// Port of org.apache.lucene.spatial3d.geom.GeoBBoxFactory.makeGeoBBox.
func MakeGeoBBox(pm *PlanetModel, topLat, bottomLat, leftLon, rightLon float64) (GeoBBox, error) {
	halfPI := math.Pi * 0.5
	if topLat > halfPI {
		topLat = halfPI
	}
	if bottomLat < -halfPI {
		bottomLat = -halfPI
	}
	if leftLon < -math.Pi {
		leftLon = -math.Pi
	}
	if rightLon > math.Pi {
		rightLon = math.Pi
	}

	// Full-longitude-band cases.
	if (longitudesEqual(leftLon, -math.Pi) && longitudesEqual(rightLon, math.Pi)) ||
		(longitudesEqual(rightLon, -math.Pi) && longitudesEqual(leftLon, math.Pi)) {
		if isNorthPole(topLat) && isSouthPole(bottomLat) {
			return NewGeoWorld(pm), nil
		}
		if latitudesEqual(topLat, bottomLat) {
			if isNorthPole(topLat) {
				return NewGeoDegeneratePoint(pm, NewGeoPointModel(pm, topLat, 0.0)), nil
			}
			if isSouthPole(bottomLat) {
				return NewGeoDegeneratePoint(pm, NewGeoPointModel(pm, bottomLat, 0.0)), nil
			}
			// Degenerate latitude circle at topLat.
			return NewGeoDegenerateLatitudeZone(pm, topLat), nil
		}
		if isNorthPole(topLat) {
			return NewGeoNorthLatitudeZone(pm, bottomLat), nil
		}
		if isSouthPole(bottomLat) {
			return NewGeoSouthLatitudeZone(pm, topLat), nil
		}
		return NewGeoLatitudeZone(pm, topLat, bottomLat), nil
	}

	extent := rightLon - leftLon
	if extent < 0.0 {
		extent += math.Pi * 2.0
	}

	// Longitude-slice cases (no latitude bounding).
	if isNorthPole(topLat) && isSouthPole(bottomLat) {
		if extent >= minWideExtent {
			return NewGeoWideLongitudeSlice(pm, leftLon, rightLon)
		}
		if longitudesEqual(leftLon, rightLon) {
			return NewGeoDegenerateLongitudeSlice(pm, leftLon)
		}
		return NewGeoLongitudeSlice(pm, leftLon, rightLon)
	}

	// Degenerate vertical line (lon band of zero width, non-degenerate latitudes).
	if longitudesEqual(leftLon, rightLon) {
		if latitudesEqual(topLat, bottomLat) {
			return NewGeoDegeneratePoint(pm, NewGeoPointModel(pm, topLat, leftLon)), nil
		}
		return NewGeoDegenerateVerticalLine(pm, topLat, bottomLat, leftLon)
	}

	// Wide rectangle variants (extent >= PI).
	if extent >= minWideExtent {
		if latitudesEqual(topLat, bottomLat) {
			if isNorthPole(topLat) {
				return NewGeoDegeneratePoint(pm, NewGeoPointModel(pm, topLat, 0.0)), nil
			}
			if isSouthPole(bottomLat) {
				return NewGeoDegeneratePoint(pm, NewGeoPointModel(pm, bottomLat, 0.0)), nil
			}
			return NewGeoWideDegenerateHorizontalLine(pm, topLat, leftLon, rightLon)
		}
		if isNorthPole(topLat) {
			return NewGeoWideNorthRectangle(pm, bottomLat, leftLon, rightLon)
		}
		if isSouthPole(bottomLat) {
			return NewGeoWideSouthRectangle(pm, topLat, leftLon, rightLon)
		}
		return NewGeoWideRectangle(pm, topLat, bottomLat, leftLon, rightLon)
	}

	// Narrow cases (extent < PI).
	if latitudesEqual(topLat, bottomLat) {
		if isNorthPole(topLat) {
			return NewGeoDegeneratePoint(pm, NewGeoPointModel(pm, topLat, 0.0)), nil
		}
		if isSouthPole(bottomLat) {
			return NewGeoDegeneratePoint(pm, NewGeoPointModel(pm, bottomLat, 0.0)), nil
		}
		return NewGeoDegenerateHorizontalLine(pm, topLat, leftLon, rightLon)
	}
	if isNorthPole(topLat) {
		return NewGeoNorthRectangle(pm, bottomLat, leftLon, rightLon)
	}
	if isSouthPole(bottomLat) {
		return NewGeoSouthRectangle(pm, topLat, leftLon, rightLon)
	}
	return NewGeoRectangle(pm, topLat, bottomLat, leftLon, rightLon)
}

// MakeGeoBBoxFromBounds creates a geobbox of the right kind given the specified LatLonBounds.
//
// Port of org.apache.lucene.spatial3d.geom.GeoBBoxFactory.makeGeoBBox(PlanetModel, LatLonBounds).
func MakeGeoBBoxFromBounds(pm *PlanetModel, bounds *LatLonBounds) (GeoBBox, error) {
	topLat := math.Pi * 0.5
	if !bounds.CheckNoTopLatitudeBound() && bounds.HasLat() {
		topLat = bounds.GetMaxLatitude()
	}
	bottomLat := -math.Pi * 0.5
	if !bounds.CheckNoBottomLatitudeBound() && bounds.HasLat() {
		bottomLat = bounds.GetMinLatitude()
	}
	leftLon := -math.Pi
	if !bounds.CheckNoLongitudeBound() && bounds.HasLon() {
		leftLon = bounds.GetMinLongitude()
	}
	rightLon := math.Pi
	if !bounds.CheckNoLongitudeBound() && bounds.HasLon() {
		rightLon = bounds.GetMaxLongitude()
	}
	return MakeGeoBBox(pm, topLat, bottomLat, leftLon, rightLon)
}

func longitudesEqual(a, b float64) bool { return math.Abs(a-b) < MinimumAngularResolution }

func latitudesEqual(a, b float64) bool {
	return math.Abs(a-b) < MinimumAngularResolution ||
		math.Abs(math.Sin(a)-math.Sin(b)) < MinimumResolution
}

func isNorthPole(lat float64) bool { return latitudesEqual(lat, math.Pi*0.5) }
func isSouthPole(lat float64) bool { return latitudesEqual(lat, -math.Pi*0.5) }
