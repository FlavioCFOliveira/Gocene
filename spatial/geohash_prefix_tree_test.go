// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"fmt"
	"math"
	"testing"
)

func TestNewGeohashPrefixTree(t *testing.T) {
	tests := []struct {
		name      string
		maxLevels int
		wantErr   bool
	}{
		{
			name:      "Valid 12 levels",
			maxLevels: 12,
			wantErr:   false,
		},
		{
			name:      "Valid 1 level",
			maxLevels: 1,
			wantErr:   false,
		},
		{
			name:      "Valid 24 levels",
			maxLevels: 24,
			wantErr:   false,
		},
		{
			name:      "Invalid 0 levels",
			maxLevels: 0,
			wantErr:   true,
		},
		{
			name:      "Invalid 25 levels",
			maxLevels: 25,
			wantErr:   true,
		},
		{
			name:      "Invalid negative levels",
			maxLevels: -1,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewGeohashPrefixTree(tt.maxLevels)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewGeohashPrefixTree() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got == nil {
				t.Error("NewGeohashPrefixTree() returned nil without error")
			}
		})
	}
}

func TestNewGeohashPrefixTreeWithDefaultLevels(t *testing.T) {
	tree, err := NewGeohashPrefixTreeWithDefaultLevels()
	if err != nil {
		t.Fatalf("NewGeohashPrefixTreeWithDefaultLevels() error = %v", err)
	}
	if tree == nil {
		t.Fatal("NewGeohashPrefixTreeWithDefaultLevels() returned nil")
	}
	if tree.maxLevels != 12 {
		t.Errorf("expected 12 levels, got %d", tree.maxLevels)
	}
}

func TestGeohashPrefixTree_GetWorldBounds(t *testing.T) {
	tree, err := NewGeohashPrefixTree(12)
	if err != nil {
		t.Fatalf("NewGeohashPrefixTree() error = %v", err)
	}

	bounds := tree.GetWorldBounds()
	if bounds == nil {
		t.Fatal("GetWorldBounds() returned nil")
	}

	// World bounds should be [-180, -90] to [180, 90]
	if bounds.MinX != -180 {
		t.Errorf("expected MinX = -180, got %f", bounds.MinX)
	}
	if bounds.MinY != -90 {
		t.Errorf("expected MinY = -90, got %f", bounds.MinY)
	}
	if bounds.MaxX != 180 {
		t.Errorf("expected MaxX = 180, got %f", bounds.MaxX)
	}
	if bounds.MaxY != 90 {
		t.Errorf("expected MaxY = 90, got %f", bounds.MaxY)
	}
}

func TestGeohashPrefixTree_GetMaxLevels(t *testing.T) {
	tree, err := NewGeohashPrefixTree(12)
	if err != nil {
		t.Fatalf("NewGeohashPrefixTree() error = %v", err)
	}

	if tree.GetMaxLevels() != 12 {
		t.Errorf("expected maxLevels = 12, got %d", tree.GetMaxLevels())
	}
}

func TestGeohashPrefixTree_Encode(t *testing.T) {
	tree, err := NewGeohashPrefixTree(12)
	if err != nil {
		t.Fatalf("NewGeohashPrefixTree() error = %v", err)
	}

	tests := []struct {
		name     string
		lon      float64
		lat      float64
		length   int
		expected int // expected length
	}{
		{
			name:     "Encode London (0, 51)",
			lon:      0,
			lat:      51,
			length:   12,
			expected: 12,
		},
		{
			name:     "Encode New York (-74, 40)",
			lon:      -74,
			lat:      40,
			length:   12,
			expected: 12,
		},
		{
			name:     "Encode Tokyo (139, 35)",
			lon:      139,
			lat:      35,
			length:   12,
			expected: 12,
		},
		{
			name:     "Encode Sydney (151, -33)",
			lon:      151,
			lat:      -33,
			length:   12,
			expected: 12,
		},
		{
			name:     "Encode with short length",
			lon:      0,
			lat:      0,
			length:   5,
			expected: 5,
		},
		{
			name:     "Encode with length 0",
			lon:      0,
			lat:      0,
			length:   0,
			expected: 0,
		},
		{
			name:     "Encode negative coordinates",
			lon:      -122.4194,
			lat:      37.7749,
			length:   9,
			expected: 9,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tree.Encode(tt.lon, tt.lat, tt.length)
			if len(got) != tt.expected {
				t.Errorf("Encode() length = %d, expected %d", len(got), tt.expected)
			}

			// Verify all characters are valid geohash characters
			for i := 0; i < len(got); i++ {
				c := got[i]
				found := false
				for j := 0; j < len(geohashBase32); j++ {
					if geohashBase32[j] == c {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Encode() produced invalid character: %c", c)
				}
			}
		})
	}
}

func TestGeohashPrefixTree_Decode(t *testing.T) {
	tree, err := NewGeohashPrefixTree(12)
	if err != nil {
		t.Fatalf("NewGeohashPrefixTree() error = %v", err)
	}

	tests := []struct {
		name      string
		token     string
		wantLon   float64
		wantLat   float64
		tolerance float64 // tolerance for floating point comparison
	}{
		{
			name:      "Decode London area",
			token:     "gcpvj",
			wantLon:   0,
			wantLat:   51.5,
			tolerance: 1.0,
		},
		{
			name:      "Decode New York area",
			token:     "dr5r",
			wantLon:   -74,
			wantLat:   40,
			tolerance: 1.0,
		},
		{
			name:      "Decode empty token",
			token:     "",
			wantLon:   0,
			wantLat:   0,
			tolerance: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLon, gotLat := tree.Decode(tt.token)

			if tt.token == "" {
				if gotLon != 0 || gotLat != 0 {
					t.Errorf("Decode() empty token should return 0,0, got %f,%f", gotLon, gotLat)
				}
				return
			}

			if math.Abs(gotLon-tt.wantLon) > tt.tolerance {
				t.Errorf("Decode() longitude = %f, want %f (tolerance %f)", gotLon, tt.wantLon, tt.tolerance)
			}
			if math.Abs(gotLat-tt.wantLat) > tt.tolerance {
				t.Errorf("Decode() latitude = %f, want %f (tolerance %f)", gotLat, tt.wantLat, tt.tolerance)
			}
		})
	}
}

func TestGeohashPrefixTree_DecodeToBBox(t *testing.T) {
	tree, err := NewGeohashPrefixTree(12)
	if err != nil {
		t.Fatalf("NewGeohashPrefixTree() error = %v", err)
	}

	tests := []struct {
		name      string
		token     string
		wantNil   bool
		checkDims bool // whether to check dimensions
	}{
		{
			name:      "Valid geohash",
			token:     "gcpvj",
			wantNil:   false,
			checkDims: true,
		},
		{
			name:    "Empty token",
			token:   "",
			wantNil: true,
		},
		{
			name:    "Invalid character",
			token:   "aaaa", // 'a' is not a valid geohash character
			wantNil: true,
		},
		{
			name:    "Invalid character 'i'",
			token:   "iiii", // 'i' is not a valid geohash character
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bbox := tree.DecodeToBBox(tt.token)

			if tt.wantNil {
				if bbox != nil {
					t.Errorf("DecodeToBBox() should return nil for %s", tt.name)
				}
				return
			}

			if bbox == nil {
				t.Fatal("DecodeToBBox() returned nil for valid token")
			}

			if tt.checkDims {
				// Check that bbox has valid dimensions
				if bbox.MinX >= bbox.MaxX {
					t.Errorf("invalid bbox: MinX %f >= MaxX %f", bbox.MinX, bbox.MaxX)
				}
				if bbox.MinY >= bbox.MaxY {
					t.Errorf("invalid bbox: MinY %f >= MaxY %f", bbox.MinY, bbox.MaxY)
				}
			}
		})
	}
}

func TestGeohashPrefixTree_EncodeDecodeRoundtrip(t *testing.T) {
	tree, err := NewGeohashPrefixTree(12)
	if err != nil {
		t.Fatalf("NewGeohashPrefixTree() error = %v", err)
	}

	tests := []struct {
		name   string
		lon    float64
		lat    float64
		length int
		delta  float64 // acceptable delta
	}{
		{
			name:   "Roundtrip London",
			lon:    -0.1257,
			lat:    51.5085,
			length: 9,
			delta:  0.01,
		},
		{
			name:   "Roundtrip New York",
			lon:    -74.0060,
			lat:    40.7128,
			length: 9,
			delta:  0.01,
		},
		{
			name:   "Roundtrip Tokyo",
			lon:    139.6917,
			lat:    35.6895,
			length: 9,
			delta:  0.01,
		},
		{
			name:   "Roundtrip Sydney",
			lon:    151.2093,
			lat:    -33.8688,
			length: 9,
			delta:  0.01,
		},
		{
			name:   "Roundtrip Equator/Prime Meridian",
			lon:    0,
			lat:    0,
			length: 12,
			delta:  0.001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode
			token := tree.Encode(tt.lon, tt.lat, tt.length)
			if token == "" {
				t.Fatal("Encode() returned empty string")
			}

			// Decode
			gotLon, gotLat := tree.Decode(token)

			// Check that decoded coordinates are within the original cell
			bbox := tree.DecodeToBBox(token)
			if bbox == nil {
				t.Fatal("DecodeToBBox() returned nil")
			}

			// The original point should be within the decoded bbox
			if tt.lon < bbox.MinX || tt.lon > bbox.MaxX {
				t.Errorf("original lon %f not in bbox [%f, %f]", tt.lon, bbox.MinX, bbox.MaxX)
			}
			if tt.lat < bbox.MinY || tt.lat > bbox.MaxY {
				t.Errorf("original lat %f not in bbox [%f, %f]", tt.lat, bbox.MinY, bbox.MaxY)
			}

			// Check approximate match
			if math.Abs(gotLon-tt.lon) > tt.delta {
				t.Errorf("longitude: got %f, want %f (delta %f)", gotLon, tt.lon, tt.delta)
			}
			if math.Abs(gotLat-tt.lat) > tt.delta {
				t.Errorf("latitude: got %f, want %f (delta %f)", gotLat, tt.lat, tt.delta)
			}
		})
	}
}

func TestGeohashPrefixTree_GetCell(t *testing.T) {
	tree, err := NewGeohashPrefixTree(12)
	if err != nil {
		t.Fatalf("NewGeohashPrefixTree() error = %v", err)
	}

	tests := []struct {
		name    string
		token   string
		wantErr bool
	}{
		{
			name:    "Valid cell",
			token:   "gcpvj",
			wantErr: false,
		},
		{
			name:    "Empty token",
			token:   "",
			wantErr: true,
		},
		{
			name:    "Invalid character 'a'",
			token:   "aaaa",
			wantErr: true,
		},
		{
			name:    "Invalid character 'i'",
			token:   "iiii",
			wantErr: true,
		},
		{
			name:    "Invalid character 'l'",
			token:   "llll",
			wantErr: true,
		},
		{
			name:    "Invalid character 'o'",
			token:   "oooo",
			wantErr: true,
		},
		{
			name:    "Long token (truncated)",
			token:   "gcpvjgcpvjgcpvj", // 15 chars, should be truncated to 12
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cell, err := tree.GetCell(tt.token)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetCell() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && cell == nil {
				t.Error("GetCell() returned nil without error")
			}
		})
	}
}

func TestGeohashPrefixTree_GetCellsForShape(t *testing.T) {
	tree, err := NewGeohashPrefixTree(12)
	if err != nil {
		t.Fatalf("NewGeohashPrefixTree() error = %v", err)
	}

	tests := []struct {
		name     string
		shape    Shape
		level    int
		wantErr  bool
		minCells int
	}{
		{
			name:     "Point at origin",
			shape:    NewPoint(0, 0),
			level:    5,
			wantErr:  false,
			minCells: 1,
		},
		{
			name:     "Point in London",
			shape:    NewPoint(-0.1, 51.5),
			level:    8,
			wantErr:  false,
			minCells: 1,
		},
		{
			name:     "Small rectangle",
			shape:    NewRectangle(-0.1, 51.4, 0.1, 51.6),
			level:    6,
			wantErr:  false,
			minCells: 1,
		},
		{
			name:    "Invalid level (too high)",
			shape:   NewPoint(0, 0),
			level:   15,
			wantErr: true,
		},
		{
			name:    "Invalid level (zero)",
			shape:   NewPoint(0, 0),
			level:   0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cells, err := tree.GetCellsForShape(tt.shape, tt.level)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetCellsForShape() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(cells) < tt.minCells {
					t.Errorf("GetCellsForShape() returned %d cells, expected at least %d", len(cells), tt.minCells)
				}

				// Verify all cells are valid
				for _, cell := range cells {
					if cell == nil {
						t.Error("GetCellsForShape() returned nil cell")
						continue
					}
					if cell.GetToken() == "" {
						t.Error("GetCellsForShape() returned cell with empty token")
					}
					if cell.GetLevel() != tt.level {
						t.Errorf("GetCellsForShape() returned cell with level %d, expected %d", cell.GetLevel(), tt.level)
					}
				}
			}
		})
	}
}

func TestGeohashPrefixTree_GetLevelForDistance(t *testing.T) {
	tree, err := NewGeohashPrefixTree(12)
	if err != nil {
		t.Fatalf("NewGeohashPrefixTree() error = %v", err)
	}

	tests := []struct {
		name     string
		distance float64
		wantMax  bool // whether we expect maxLevels
	}{
		{
			name:     "Very large distance",
			distance: 100,
			wantMax:  false,
		},
		{
			name:     "Medium distance",
			distance: 1.0,
			wantMax:  false,
		},
		{
			name:     "Small distance",
			distance: 0.01,
			wantMax:  false,
		},
		{
			name:     "Very small distance",
			distance: 0.00001,
			wantMax:  true,
		},
		{
			name:     "Zero distance",
			distance: 0,
			wantMax:  true,
		},
		{
			name:     "Negative distance",
			distance: -1,
			wantMax:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level := tree.GetLevelForDistance(tt.distance)

			if tt.wantMax {
				if level != tree.maxLevels {
					t.Errorf("GetLevelForDistance(%f) = %d, expected maxLevels %d", tt.distance, level, tree.maxLevels)
				}
			}

			// Level should always be within bounds
			if level < 1 {
				t.Errorf("GetLevelForDistance(%f) returned level %d, expected >= 1", tt.distance, level)
			}
			if level > tree.maxLevels {
				t.Errorf("GetLevelForDistance(%f) returned level %d, expected <= %d", tt.distance, level, tree.maxLevels)
			}
		})
	}
}

func TestGeohashPrefixTree_GetNeighbors(t *testing.T) {
	tree, err := NewGeohashPrefixTree(12)
	if err != nil {
		t.Fatalf("NewGeohashPrefixTree() error = %v", err)
	}

	tests := []struct {
		name         string
		token        string
		wantNil      bool
		minNeighbors int
	}{
		{
			name:         "Normal cell",
			token:        "gcpvj",
			wantNil:      false,
			minNeighbors: 1,
		},
		{
			name:    "Empty token",
			token:   "",
			wantNil: true,
		},
		{
			name:         "Single character",
			token:        "g",
			wantNil:      false,
			minNeighbors: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			neighbors := tree.GetNeighbors(tt.token)

			if tt.wantNil {
				if neighbors != nil {
					t.Errorf("GetNeighbors(%s) should return nil", tt.token)
				}
				return
			}

			if neighbors == nil {
				t.Fatal("GetNeighbors() returned nil")
			}

			if len(neighbors) < tt.minNeighbors {
				t.Errorf("GetNeighbors() returned %d neighbors, expected at least %d", len(neighbors), tt.minNeighbors)
			}

			// Verify no neighbor is the same as the original
			for _, neighbor := range neighbors {
				if neighbor == tt.token {
					t.Errorf("GetNeighbors() returned same token %s", neighbor)
				}
			}
		})
	}
}

func TestGeohashCell_GetNeighbors(t *testing.T) {
	tree, err := NewGeohashPrefixTree(12)
	if err != nil {
		t.Fatalf("NewGeohashPrefixTree() error = %v", err)
	}

	cell, err := tree.GetCell("gcpvj")
	if err != nil {
		t.Fatalf("GetCell() error = %v", err)
	}

	geohashCell, ok := cell.(*GeohashCell)
	if !ok {
		t.Fatal("Cell is not a GeohashCell")
	}

	neighbors := geohashCell.GetNeighbors()
	if len(neighbors) == 0 {
		t.Error("GetNeighbors() returned empty slice")
	}

	// Each neighbor should be a valid cell
	for _, neighbor := range neighbors {
		if neighbor == nil {
			t.Error("GetNeighbors() returned nil cell")
		}
	}
}

func TestGeohashCell_GetParent(t *testing.T) {
	tree, err := NewGeohashPrefixTree(12)
	if err != nil {
		t.Fatalf("NewGeohashPrefixTree() error = %v", err)
	}

	tests := []struct {
		name       string
		token      string
		wantNil    bool
		wantParent string
	}{
		{
			name:       "Cell with parent",
			token:      "gcpvj",
			wantNil:    false,
			wantParent: "gcpv",
		},
		{
			name:    "Single char cell (no parent)",
			token:   "g",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cell, err := tree.GetCell(tt.token)
			if err != nil {
				t.Fatalf("GetCell() error = %v", err)
			}

			geohashCell, ok := cell.(*GeohashCell)
			if !ok {
				t.Fatal("Cell is not a GeohashCell")
			}

			parent := geohashCell.GetParent()

			if tt.wantNil {
				if parent != nil {
					t.Errorf("GetParent() should return nil for token %s", tt.token)
				}
				return
			}

			if parent == nil {
				t.Fatal("GetParent() returned nil")
			}

			if parent.GetToken() != tt.wantParent {
				t.Errorf("GetParent() token = %s, want %s", parent.GetToken(), tt.wantParent)
			}
		})
	}
}

func TestGeohashCell_GetChildren(t *testing.T) {
	tree, err := NewGeohashPrefixTree(12)
	if err != nil {
		t.Fatalf("NewGeohashPrefixTree() error = %v", err)
	}

	tests := []struct {
		name        string
		token       string
		maxLevels   int
		wantNil     bool
		numChildren int
	}{
		{
			name:        "Cell with children",
			token:       "gcpv",
			maxLevels:   12,
			wantNil:     false,
			numChildren: 32, // Geohash has 32 children per cell
		},
		{
			name:      "Max level cell (no children)",
			token:     "gcpvjgcpvjgcp",
			maxLevels: 12,
			wantNil:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cell, err := tree.GetCell(tt.token)
			if err != nil {
				t.Fatalf("GetCell() error = %v", err)
			}

			geohashCell, ok := cell.(*GeohashCell)
			if !ok {
				t.Fatal("Cell is not a GeohashCell")
			}

			children := geohashCell.GetChildren()

			if tt.wantNil {
				if children != nil {
					t.Errorf("GetChildren() should return nil for max level cell")
				}
				return
			}

			if children == nil {
				t.Fatal("GetChildren() returned nil")
			}

			if len(children) != tt.numChildren {
				t.Errorf("GetChildren() returned %d children, expected %d", len(children), tt.numChildren)
			}
		})
	}
}

func TestGeohashCell_IntersectsShape(t *testing.T) {
	tree, err := NewGeohashPrefixTree(12)
	if err != nil {
		t.Fatalf("NewGeohashPrefixTree() error = %v", err)
	}

	cell, err := tree.GetCell("gcpvj")
	if err != nil {
		t.Fatalf("GetCell() error = %v", err)
	}

	// Test intersection with various shapes
	tests := []struct {
		name     string
		shape    Shape
		expected bool
	}{
		{
			name:     "Point inside cell",
			shape:    NewPoint(-0.1, 51.52), // Point within gcpvj cell area (London)
			expected: true,
		},
		{
			name:     "Point far from cell",
			shape:    NewPoint(-100, 0),
			expected: false,
		},
		{
			name:     "Rectangle overlapping cell",
			shape:    NewRectangle(-1, 51, 1, 52),
			expected: true,
		},
		{
			name:     "Rectangle far from cell",
			shape:    NewRectangle(-100, -50, -90, -40),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cell.IntersectsShape(tt.shape)
			if result != tt.expected {
				t.Errorf("IntersectsShape() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestCalculateGeohashPrecision(t *testing.T) {
	tests := []struct {
		level    int
		minValue float64 // minimum expected precision
		maxValue float64 // maximum expected precision
	}{
		{level: 1, minValue: 1000000, maxValue: 10000000},
		{level: 5, minValue: 1000, maxValue: 10000},
		{level: 9, minValue: 1, maxValue: 10},
		{level: 12, minValue: 0.01, maxValue: 1},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("Level_%d", tt.level), func(t *testing.T) {
			precision := CalculateGeohashPrecision(tt.level)

			if precision < tt.minValue {
				t.Errorf("CalculateGeohashPrecision(%d) = %f, expected >= %f", tt.level, precision, tt.minValue)
			}
			if precision > tt.maxValue {
				t.Errorf("CalculateGeohashPrecision(%d) = %f, expected <= %f", tt.level, precision, tt.maxValue)
			}
		})
	}
}

func TestGetGeohashBase32(t *testing.T) {
	base32 := GetGeohashBase32()
	if len(base32) != 32 {
		t.Errorf("GetGeohashBase32() length = %d, expected 32", len(base32))
	}

	// Verify no excluded characters
	excludedChars := []byte{'a', 'i', 'l', 'o'}
	for _, excluded := range excludedChars {
		for i := 0; i < len(base32); i++ {
			if base32[i] == excluded {
				t.Errorf("GetGeohashBase32() contains excluded character %c", excluded)
			}
		}
	}
}

func TestGeohashPrefixTree_InterfaceCompliance(t *testing.T) {
	tree, err := NewGeohashPrefixTree(12)
	if err != nil {
		t.Fatalf("NewGeohashPrefixTree() error = %v", err)
	}

	// Verify that GeohashPrefixTree implements SpatialPrefixTree
	var _ SpatialPrefixTree = tree

	// Verify interface methods work
	_ = tree.GetWorldBounds()
	_ = tree.GetMaxLevels()
	_ = tree.GetLevelForDistance(1.0)

	_, err = tree.GetCellsForShape(NewPoint(0, 0), 5)
	if err != nil {
		t.Errorf("GetCellsForShape() error = %v", err)
	}

	_, err = tree.GetCell("gcpvj")
	if err != nil {
		t.Errorf("GetCell() error = %v", err)
	}
}
