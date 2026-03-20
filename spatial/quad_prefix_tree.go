// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"fmt"
	"math"
)

// QuadPrefixTree implements a prefix tree using quad tree subdivision.
// Quad trees subdivide space into 4 quadrants recursively, providing
// a hierarchical spatial indexing system.
//
// Each level of the quad tree divides the current cell into 4 equal quadrants:
//   - 0: Southwest (SW) - lower left
//   - 1: Southeast (SE) - lower right
//   - 2: Northwest (NW) - upper left
//   - 3: Northeast (NE) - upper right
//
// This is the Go port of Lucene's QuadPrefixTree.
type QuadPrefixTree struct {
	*BaseSpatialPrefixTree
	maxLevels int
}

// NewQuadPrefixTree creates a new QuadPrefixTree with the specified maximum levels.
//
// Parameters:
//   - maxLevels: The maximum depth of the tree (1-30 recommended). Higher values provide
//     more precision but require more storage. Each level quadruples the number of cells.
//
// Returns an error if maxLevels is invalid.
func NewQuadPrefixTree(maxLevels int) (*QuadPrefixTree, error) {
	if maxLevels < 1 || maxLevels > 50 {
		return nil, fmt.Errorf("maxLevels must be between 1 and 50, got %d", maxLevels)
	}

	// World bounds: longitude [-180, 180], latitude [-90, 90]
	worldBounds := NewRectangle(-180, -90, 180, 90)

	base := &BaseSpatialPrefixTree{
		worldBounds: worldBounds,
		maxLevels:   maxLevels,
	}

	return &QuadPrefixTree{
		BaseSpatialPrefixTree: base,
		maxLevels:             maxLevels,
	}, nil
}

// NewQuadPrefixTreeWithDefaultLevels creates a QuadPrefixTree with default 16 levels.
func NewQuadPrefixTreeWithDefaultLevels() (*QuadPrefixTree, error) {
	return NewQuadPrefixTree(16)
}

// GetLevelForDistance returns the appropriate level for a given distance in degrees.
// The distance represents the desired precision - the returned level will have cell
// dimensions approximately equal to or smaller than the specified distance.
func (t *QuadPrefixTree) GetLevelForDistance(distance float64) int {
	if distance <= 0 {
		return t.maxLevels
	}

	// Quad tree approximate cell dimensions per level (in degrees at equator)
	// Each level divides dimensions by 2
	// Level 1: 360 degrees (world)
	// Level 2: 180 degrees
	// Level 3: 90 degrees
	// Level 4: 45 degrees
	// Level 5: 22.5 degrees
	// Level 6: 11.25 degrees
	// Level 7: 5.625 degrees
	// Level 8: 2.8125 degrees
	// Level 9: 1.40625 degrees
	// Level 10: 0.703 degrees
	// Level 11: 0.351 degrees
	// Level 12: 0.176 degrees
	// Level 13: 0.088 degrees
	// Level 14: 0.044 degrees
	// Level 15: 0.022 degrees
	// Level 16: 0.011 degrees

	level := 1
	dim := 360.0 // Starting dimension at level 1

	for level < t.maxLevels {
		dim /= 2
		if dim <= distance {
			return level + 1
		}
		level++
	}

	return t.maxLevels
}

// GetCellsForShape returns the cells that intersect with the given shape at the specified level.
func (t *QuadPrefixTree) GetCellsForShape(shape Shape, level int) ([]Cell, error) {
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

	// Create a quad cell
	cell := &QuadCell{
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

// GetCell returns the cell for the given quad token.
func (t *QuadPrefixTree) GetCell(token string) (Cell, error) {
	if token == "" {
		return nil, fmt.Errorf("token cannot be empty")
	}

	// Validate token characters (must be 0, 1, 2, or 3)
	for i := 0; i < len(token); i++ {
		c := token[i]
		if c < '0' || c > '3' {
			return nil, fmt.Errorf("invalid character in quad token: %c (must be 0-3)", c)
		}
	}

	level := len(token)
	if level > t.maxLevels {
		level = t.maxLevels
		token = token[:level]
	}

	cell := &QuadCell{
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

// Encode encodes latitude and longitude into a quad tree token of the specified length.
// Longitude (x) must be in range [-180, 180], Latitude (y) must be in range [-90, 90].
//
// The encoding follows the quad tree subdivision:
//   - Bit 0 (LSB): x bit (longitude)
//   - Bit 1: y bit (latitude)
//   - Values: 0=SW, 1=SE, 2=NW, 3=NE
func (t *QuadPrefixTree) Encode(lon, lat float64, length int) string {
	if length <= 0 {
		return ""
	}
	if length > t.maxLevels {
		length = t.maxLevels
	}

	// Normalize coordinates
	lon = normalizeLongitude(lon)
	lat = normalizeLatitude(lat)

	// Start with world bounds
	lonMin, lonMax := -180.0, 180.0
	latMin, latMax := -90.0, 90.0

	// Build the token
	result := make([]byte, length)

	for i := 0; i < length; i++ {
		// Calculate midpoints
		lonMid := (lonMin + lonMax) / 2
		latMid := (latMin + latMax) / 2

		// Determine which quadrant
		var quadrant byte
		if lat >= latMid {
			// North half
			if lon >= lonMid {
				quadrant = '3' // NE
				lonMin = lonMid
				latMin = latMid
			} else {
				quadrant = '2' // NW
				lonMax = lonMid
				latMin = latMid
			}
		} else {
			// South half
			if lon >= lonMid {
				quadrant = '1' // SE
				lonMin = lonMid
				latMax = latMid
			} else {
				quadrant = '0' // SW
				lonMax = lonMid
				latMax = latMid
			}
		}

		result[i] = quadrant
	}

	return string(result)
}

// Decode decodes a quad token into latitude and longitude coordinates.
// Returns the center point of the quad cell.
func (t *QuadPrefixTree) Decode(token string) (lon, lat float64) {
	bbox := t.DecodeToBBox(token)
	if bbox == nil {
		return 0, 0
	}
	return (bbox.MinX + bbox.MaxX) / 2, (bbox.MinY + bbox.MaxY) / 2
}

// DecodeToBBox decodes a quad token into a bounding box rectangle.
func (t *QuadPrefixTree) DecodeToBBox(token string) *Rectangle {
	if token == "" {
		return nil
	}

	lonMin, lonMax := -180.0, 180.0
	latMin, latMax := -90.0, 90.0

	for i := 0; i < len(token); i++ {
		c := token[i]
		if c < '0' || c > '3' {
			return nil // Invalid character
		}

		lonMid := (lonMin + lonMax) / 2
		latMid := (latMin + latMax) / 2

		switch c {
		case '0': // SW
			lonMax = lonMid
			latMax = latMid
		case '1': // SE
			lonMin = lonMid
			latMax = latMid
		case '2': // NW
			lonMax = lonMid
			latMin = latMid
		case '3': // NE
			lonMin = lonMid
			latMin = latMid
		}
	}

	return NewRectangle(lonMin, latMin, lonMax, latMax)
}

// GetNeighbors returns the 8 neighboring quad cells of the given token.
func (t *QuadPrefixTree) GetNeighbors(token string) []string {
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

// QuadCell implements the Cell interface for quad tree cells.
type QuadCell struct {
	*BaseCell
	prefixTree *QuadPrefixTree
	shape      Shape
}

// GetShape returns the shape of this cell (its bounding box).
func (c *QuadCell) GetShape() Shape {
	if c.shape == nil {
		c.shape = c.bbox
	}
	return c.shape
}

// GetNeighbors returns the neighboring cells of this cell.
func (c *QuadCell) GetNeighbors() []Cell {
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
func (c *QuadCell) GetParent() Cell {
	if c.level <= 1 {
		return nil
	}
	parentToken := c.token[:len(c.token)-1]
	cell, _ := c.prefixTree.GetCell(parentToken)
	return cell
}

// GetChildren returns the child cells (one level down).
func (c *QuadCell) GetChildren() []Cell {
	if c.level >= c.prefixTree.maxLevels {
		return nil
	}

	children := make([]Cell, 0, 4)
	for i := 0; i < 4; i++ {
		childToken := c.token + string('0'+byte(i))
		cell, err := c.prefixTree.GetCell(childToken)
		if err == nil {
			children = append(children, cell)
		}
	}

	return children
}

// Ensure QuadPrefixTree implements SpatialPrefixTree
var _ SpatialPrefixTree = (*QuadPrefixTree)(nil)

// Ensure QuadCell implements Cell
var _ Cell = (*QuadCell)(nil)

// CalculateQuadPrecision returns the approximate precision in meters for a given quad tree level.
// This is an approximation assuming the cell is near the equator.
func CalculateQuadPrecision(level int) float64 {
	// World width at equator in meters (approximately)
	const worldWidthMeters = 40075000 // Earth's circumference at equator

	// Each level halves the dimensions
	// Level 1: full world
	// Level 2: 1/2 of world
	// Level n: 1/(2^(n-1)) of world
	if level < 1 {
		return worldWidthMeters
	}

	cellsPerDimension := math.Pow(2, float64(level-1))
	cellWidth := worldWidthMeters / cellsPerDimension

	return cellWidth
}

// GetQuadrantName returns the human-readable name for a quadrant.
func GetQuadrantName(quadrant byte) string {
	switch quadrant {
	case '0':
		return "SW"
	case '1':
		return "SE"
	case '2':
		return "NW"
	case '3':
		return "NE"
	default:
		return "Unknown"
	}
}
