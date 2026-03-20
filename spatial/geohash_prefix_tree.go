// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"fmt"
	"math"
)

// GeohashPrefixTree implements a prefix tree using Geohash encoding.
// Geohash is a hierarchical spatial indexing system that subdivides the world
// into grid cells using base-32 encoding. Each level adds precision by
// subdividing cells into 32 sub-cells.
//
// The geohash encoding uses 32 characters (0-9, b-z excluding a,i,l,o) to encode
// latitude and longitude coordinates. Each character added to the geohash
// represents approximately 5 bits of precision (2.5 bits for latitude, 2.5 bits for longitude).
//
// This is the Go port of Lucene's GeohashPrefixTree.
type GeohashPrefixTree struct {
	*BaseSpatialPrefixTree
	maxLevels int
}

// geohashBase32 is the character set used for geohash encoding.
// Note: Geohash excludes 'a', 'i', 'l', 'o' to avoid confusion.
const geohashBase32 = "0123456789bcdefghjkmnpqrstuvwxyz"

// geohashBits represents the bit patterns for each base32 character.
// Used for fast lookup during encoding/decoding.
var geohashBitPatterns = map[byte]uint8{
	'0': 0, '1': 1, '2': 2, '3': 3, '4': 4, '5': 5, '6': 6, '7': 7,
	'8': 8, '9': 9, 'b': 10, 'c': 11, 'd': 12, 'e': 13, 'f': 14, 'g': 15,
	'h': 16, 'j': 17, 'k': 18, 'm': 19, 'n': 20, 'p': 21, 'q': 22, 'r': 23,
	's': 24, 't': 25, 'u': 26, 'v': 27, 'w': 28, 'x': 29, 'y': 30, 'z': 31,
}

// NewGeohashPrefixTree creates a new GeohashPrefixTree with the specified maximum levels.
//
// Parameters:
//   - maxLevels: The maximum depth of the tree (1-12 recommended). Higher values provide
//     more precision but require more storage. Each level adds approximately 5 bits of precision.
//
// Returns an error if maxLevels is invalid.
func NewGeohashPrefixTree(maxLevels int) (*GeohashPrefixTree, error) {
	if maxLevels < 1 || maxLevels > 24 {
		return nil, fmt.Errorf("maxLevels must be between 1 and 24, got %d", maxLevels)
	}

	// World bounds: longitude [-180, 180], latitude [-90, 90]
	worldBounds := NewRectangle(-180, -90, 180, 90)

	base := &BaseSpatialPrefixTree{
		worldBounds: worldBounds,
		maxLevels:   maxLevels,
	}

	return &GeohashPrefixTree{
		BaseSpatialPrefixTree: base,
		maxLevels:             maxLevels,
	}, nil
}

// NewGeohashPrefixTreeWithDefaultLevels creates a GeohashPrefixTree with default 12 levels.
func NewGeohashPrefixTreeWithDefaultLevels() (*GeohashPrefixTree, error) {
	return NewGeohashPrefixTree(12)
}

// GetLevelForDistance returns the appropriate level for a given distance in degrees.
// The distance represents the desired precision - the returned level will have cell
// dimensions approximately equal to or smaller than the specified distance.
func (t *GeohashPrefixTree) GetLevelForDistance(distance float64) int {
	if distance <= 0 {
		return t.maxLevels
	}

	// Geohash approximate cell dimensions per level (in degrees at equator)
	// Level 1: ~22.5 degrees
	// Level 2: ~5.625 degrees
	// Level 3: ~1.406 degrees
	// Level 4: ~0.351 degrees
	// Level 5: ~0.0879 degrees
	// Level 6: ~0.02197 degrees
	// Level 7: ~0.00549 degrees
	// Level 8: ~0.00137 degrees
	// Level 9: ~0.00034 degrees
	// Level 10: ~0.000085 degrees
	// Level 11: ~0.000021 degrees
	// Level 12: ~0.000005 degrees
	levelDimensions := []float64{
		22.5, 5.625, 1.406, 0.351, 0.0879, 0.02197,
		0.00549, 0.00137, 0.00034, 0.000085, 0.000021, 0.000005,
	}

	// Find the appropriate level
	for level, dim := range levelDimensions {
		if dim <= distance {
			return level + 1
		}
	}

	return t.maxLevels
}

// GetCellsForShape returns the cells that intersect with the given shape at the specified level.
func (t *GeohashPrefixTree) GetCellsForShape(shape Shape, level int) ([]Cell, error) {
	if level < 1 || level > t.maxLevels {
		return nil, fmt.Errorf("level must be between 1 and %d", t.maxLevels)
	}

	bbox := shape.GetBoundingBox()
	if bbox == nil {
		return nil, fmt.Errorf("shape has no bounding box")
	}

	// Get center point of the bounding box
	centerX := (bbox.MinX + bbox.MaxX) / 2
	centerY := (bbox.MinY + bbox.MaxY) / 2

	// Encode the center point to get the primary cell
	token := t.Encode(centerX, centerY, level)

	// Create a geohash cell
	cell := &GeohashCell{
		BaseCell: &BaseCell{
			token:  token,
			level:  level,
			isLeaf: level >= t.maxLevels,
		},
		prefixTree: t,
	}

	// Calculate the actual bounds for this cell
	cell.bbox = t.DecodeToBBox(token)
	cell.shape = cell.bbox

	return []Cell{cell}, nil
}

// GetCell returns the cell for the given geohash token.
func (t *GeohashPrefixTree) GetCell(token string) (Cell, error) {
	if token == "" {
		return nil, fmt.Errorf("token cannot be empty")
	}

	// Validate token characters
	for i := 0; i < len(token); i++ {
		c := token[i]
		if _, ok := geohashBitPatterns[c]; !ok {
			return nil, fmt.Errorf("invalid character in geohash token: %c", c)
		}
	}

	level := len(token)
	if level > t.maxLevels {
		level = t.maxLevels
		token = token[:level]
	}

	cell := &GeohashCell{
		BaseCell: &BaseCell{
			token:  token,
			level:  level,
			isLeaf: level >= t.maxLevels,
		},
		prefixTree: t,
	}

	cell.bbox = t.DecodeToBBox(token)
	cell.shape = cell.bbox

	return cell, nil
}

// Encode encodes latitude and longitude into a geohash string of the specified length.
// Longitude (x) must be in range [-180, 180], Latitude (y) must be in range [-90, 90].
func (t *GeohashPrefixTree) Encode(lon, lat float64, length int) string {
	if length <= 0 {
		return ""
	}
	if length > t.maxLevels {
		length = t.maxLevels
	}

	// Normalize coordinates
	lon = normalizeLongitude(lon)
	lat = normalizeLatitude(lat)

	var result []byte
	lonMin, lonMax := -180.0, 180.0
	latMin, latMax := -90.0, 90.0

	var bits uint8
	bits = 0
	bitCount := 0
	isLon := true // Alternate between longitude and latitude bits

	for len(result) < length {
		if isLon {
			// Process longitude bit
			lonMid := (lonMin + lonMax) / 2
			if lon >= lonMid {
				bits = (bits << 1) | 1
				lonMin = lonMid
			} else {
				bits = bits << 1
				lonMax = lonMid
			}
		} else {
			// Process latitude bit
			latMid := (latMin + latMax) / 2
			if lat >= latMid {
				bits = (bits << 1) | 1
				latMin = latMid
			} else {
				bits = bits << 1
				latMax = latMid
			}
		}

		isLon = !isLon
		bitCount++

		// When we have 5 bits, convert to a base32 character
		if bitCount == 5 {
			result = append(result, geohashBase32[bits])
			bits = 0
			bitCount = 0
		}
	}

	return string(result)
}

// Decode decodes a geohash string into latitude and longitude coordinates.
// Returns the center point of the geohash cell.
func (t *GeohashPrefixTree) Decode(token string) (lon, lat float64) {
	bbox := t.DecodeToBBox(token)
	if bbox == nil {
		return 0, 0
	}
	return (bbox.MinX + bbox.MaxX) / 2, (bbox.MinY + bbox.MaxY) / 2
}

// DecodeToBBox decodes a geohash string into a bounding box rectangle.
func (t *GeohashPrefixTree) DecodeToBBox(token string) *Rectangle {
	if token == "" {
		return nil
	}

	lonMin, lonMax := -180.0, 180.0
	latMin, latMax := -90.0, 90.0
	isLon := true

	for i := 0; i < len(token); i++ {
		c := token[i]
		bits, ok := geohashBitPatterns[c]
		if !ok {
			return nil // Invalid character
		}

		// Process 5 bits
		for bit := 4; bit >= 0; bit-- {
			bitSet := (bits >> uint(bit)) & 1
			if isLon {
				lonMid := (lonMin + lonMax) / 2
				if bitSet == 1 {
					lonMin = lonMid
				} else {
					lonMax = lonMid
				}
			} else {
				latMid := (latMin + latMax) / 2
				if bitSet == 1 {
					latMin = latMid
				} else {
					latMax = latMid
				}
			}
			isLon = !isLon
		}
	}

	return NewRectangle(lonMin, latMin, lonMax, latMax)
}

// GetNeighbors returns the 8 neighboring geohash cells of the given token.
func (t *GeohashPrefixTree) GetNeighbors(token string) []string {
	if token == "" {
		return nil
	}

	bbox := t.DecodeToBBox(token)
	if bbox == nil {
		return nil
	}

	centerLon := (bbox.MinX + bbox.MaxX) / 2
	centerLat := (bbox.MinY + bbox.MaxY) / 2
	width := bbox.MaxX - bbox.MinX
	height := bbox.MaxY - bbox.MinY

	neighbors := make([]string, 0, 8)
	level := len(token)

	// Directions: N, NE, E, SE, S, SW, W, NW
	directions := []struct {
		dx, dy float64
	}{
		{0, height},       // N
		{width, height},   // NE
		{width, 0},        // E
		{width, -height},  // SE
		{0, -height},      // S
		{-width, -height}, // SW
		{-width, 0},       // W
		{-width, height},  // NW
	}

	for _, dir := range directions {
		newLon := centerLon + dir.dx
		newLat := centerLat + dir.dy

		// Normalize coordinates
		newLon = normalizeLongitude(newLon)
		newLat = normalizeLatitude(newLat)

		neighbor := t.Encode(newLon, newLat, level)
		if neighbor != "" && neighbor != token {
			neighbors = append(neighbors, neighbor)
		}
	}

	return neighbors
}

// GeohashCell implements the Cell interface for geohash cells.
type GeohashCell struct {
	*BaseCell
	prefixTree *GeohashPrefixTree
	shape      Shape
}

// GetShape returns the shape of this cell (its bounding box).
func (c *GeohashCell) GetShape() Shape {
	if c.shape == nil {
		c.shape = c.bbox
	}
	return c.shape
}

// GetNeighbors returns the neighboring cells of this cell.
func (c *GeohashCell) GetNeighbors() []Cell {
	neighborTokens := c.prefixTree.GetNeighbors(c.token)
	neighbors := make([]Cell, 0, len(neighborTokens))

	for _, token := range neighborTokens {
		cell, err := c.prefixTree.GetCell(token)
		if err == nil {
			neighbors = append(neighbors, cell)
		}
	}

	return neighbors
}

// GetParent returns the parent cell (one level up).
func (c *GeohashCell) GetParent() Cell {
	if c.level <= 1 {
		return nil
	}
	parentToken := c.token[:len(c.token)-1]
	cell, _ := c.prefixTree.GetCell(parentToken)
	return cell
}

// GetChildren returns the child cells (one level down).
func (c *GeohashCell) GetChildren() []Cell {
	if c.level >= c.prefixTree.maxLevels {
		return nil
	}

	children := make([]Cell, 0, 32)
	for i := 0; i < 32; i++ {
		childToken := c.token + string(geohashBase32[i])
		cell, err := c.prefixTree.GetCell(childToken)
		if err == nil {
			children = append(children, cell)
		}
	}

	return children
}

// Ensure GeohashPrefixTree implements SpatialPrefixTree
var _ SpatialPrefixTree = (*GeohashPrefixTree)(nil)

// Ensure GeohashCell implements Cell
var _ Cell = (*GeohashCell)(nil)

// CalculateGeohashPrecision returns the approximate precision in meters for a given geohash level.
// This is an approximation assuming the geohash is near the equator.
func CalculateGeohashPrecision(level int) float64 {
	// Approximate dimensions at equator for each geohash level (in meters)
	// These values are approximate and vary by latitude
	precisionByLevel := []float64{
		5000000, // Level 1: ~5000km
		1250000, // Level 2: ~1250km
		156000,  // Level 3: ~156km
		39000,   // Level 4: ~39km
		4900,    // Level 5: ~4.9km
		1220,    // Level 6: ~1.22km
		153,     // Level 7: ~153m
		38.2,    // Level 8: ~38m
		4.78,    // Level 9: ~4.78m
		1.19,    // Level 10: ~1.19m
		0.149,   // Level 11: ~0.149m
		0.037,   // Level 12: ~0.037m
	}

	if level < 1 {
		return precisionByLevel[0]
	}
	if level > len(precisionByLevel) {
		return precisionByLevel[len(precisionByLevel)-1] / math.Pow(2, float64(level-len(precisionByLevel)))
	}
	return precisionByLevel[level-1]
}

// GetGeohashBase32 returns the base32 character set used for geohash encoding.
func GetGeohashBase32() string {
	return geohashBase32
}
