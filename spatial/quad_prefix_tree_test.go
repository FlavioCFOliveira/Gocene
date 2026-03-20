// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"fmt"
	"math"
	"testing"
)

func TestNewQuadPrefixTree(t *testing.T) {
	tests := []struct {
		name      string
		maxLevels int
		wantErr   bool
	}{
		{
			name:      "Valid 16 levels",
			maxLevels: 16,
			wantErr:   false,
		},
		{
			name:      "Valid 1 level",
			maxLevels: 1,
			wantErr:   false,
		},
		{
			name:      "Valid 50 levels",
			maxLevels: 50,
			wantErr:   false,
		},
		{
			name:      "Invalid 0 levels",
			maxLevels: 0,
			wantErr:   true,
		},
		{
			name:      "Invalid 51 levels",
			maxLevels: 51,
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
			got, err := NewQuadPrefixTree(tt.maxLevels)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewQuadPrefixTree() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got == nil {
				t.Error("NewQuadPrefixTree() returned nil without error")
			}
		})
	}
}

func TestNewQuadPrefixTreeWithDefaultLevels(t *testing.T) {
	tree, err := NewQuadPrefixTreeWithDefaultLevels()
	if err != nil {
		t.Fatalf("NewQuadPrefixTreeWithDefaultLevels() error = %v", err)
	}
	if tree == nil {
		t.Fatal("NewQuadPrefixTreeWithDefaultLevels() returned nil")
	}
	if tree.maxLevels != 16 {
		t.Errorf("expected 16 levels, got %d", tree.maxLevels)
	}
}

func TestQuadPrefixTree_GetWorldBounds(t *testing.T) {
	tree, err := NewQuadPrefixTree(16)
	if err != nil {
		t.Fatalf("NewQuadPrefixTree() error = %v", err)
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

func TestQuadPrefixTree_GetMaxLevels(t *testing.T) {
	tree, err := NewQuadPrefixTree(16)
	if err != nil {
		t.Fatalf("NewQuadPrefixTree() error = %v", err)
	}

	if tree.GetMaxLevels() != 16 {
		t.Errorf("expected maxLevels = 16, got %d", tree.GetMaxLevels())
	}
}

func TestQuadPrefixTree_Encode(t *testing.T) {
	tree, err := NewQuadPrefixTree(16)
	if err != nil {
		t.Fatalf("NewQuadPrefixTree() error = %v", err)
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
			length:   8,
			expected: 8,
		},
		{
			name:     "Encode New York (-74, 40)",
			lon:      -74,
			lat:      40,
			length:   8,
			expected: 8,
		},
		{
			name:     "Encode Tokyo (139, 35)",
			lon:      139,
			lat:      35,
			length:   8,
			expected: 8,
		},
		{
			name:     "Encode Sydney (151, -33)",
			lon:      151,
			lat:      -33,
			length:   8,
			expected: 8,
		},
		{
			name:     "Encode with short length",
			lon:      0,
			lat:      0,
			length:   3,
			expected: 3,
		},
		{
			name:     "Encode with length 0",
			lon:      0,
			lat:      0,
			length:   0,
			expected: 0,
		},
		{
			name:     "Encode at origin",
			lon:      0,
			lat:      0,
			length:   4,
			expected: 4,
		},
		{
			name:     "Encode at north pole",
			lon:      0,
			lat:      90,
			length:   4,
			expected: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tree.Encode(tt.lon, tt.lat, tt.length)
			if len(got) != tt.expected {
				t.Errorf("Encode() length = %d, expected %d", len(got), tt.expected)
			}

			// Verify all characters are valid quad characters (0-3)
			for i := 0; i < len(got); i++ {
				c := got[i]
				if c < '0' || c > '3' {
					t.Errorf("Encode() produced invalid character: %c", c)
				}
			}
		})
	}
}

func TestQuadPrefixTree_EncodeQuadrants(t *testing.T) {
	tree, err := NewQuadPrefixTree(16)
	if err != nil {
		t.Fatalf("NewQuadPrefixTree() error = %v", err)
	}

	// Test encoding at level 1
	// SW: lon < 0, lat < 0
	// SE: lon >= 0, lat < 0
	// NW: lon < 0, lat >= 0
	// NE: lon >= 0, lat >= 0

	tests := []struct {
		lon      float64
		lat      float64
		expected string
	}{
		{-10, -10, "0"}, // SW
		{10, -10, "1"},  // SE
		{-10, 10, "2"},  // NW
		{10, 10, "3"},   // NE
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("(%.0f, %.0f)", tt.lon, tt.lat), func(t *testing.T) {
			got := tree.Encode(tt.lon, tt.lat, 1)
			if got != tt.expected {
				t.Errorf("Encode(%.0f, %.0f, 1) = %s, expected %s", tt.lon, tt.lat, got, tt.expected)
			}
		})
	}
}

func TestQuadPrefixTree_Decode(t *testing.T) {
	tree, err := NewQuadPrefixTree(16)
	if err != nil {
		t.Fatalf("NewQuadPrefixTree() error = %v", err)
	}

	tests := []struct {
		name      string
		token     string
		wantLon   float64
		wantLat   float64
		tolerance float64
	}{
		{
			name:      "Decode single digit 0",
			token:     "0",
			wantLon:   -90,
			wantLat:   -45,
			tolerance: 5,
		},
		{
			name:      "Decode single digit 1",
			token:     "1",
			wantLon:   90,
			wantLat:   -45,
			tolerance: 5,
		},
		{
			name:      "Decode single digit 2",
			token:     "2",
			wantLon:   -90,
			wantLat:   45,
			tolerance: 5,
		},
		{
			name:      "Decode single digit 3",
			token:     "3",
			wantLon:   90,
			wantLat:   45,
			tolerance: 5,
		},
		{
			name:    "Empty token",
			token:   "",
			wantLon: 0,
			wantLat: 0,
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

func TestQuadPrefixTree_DecodeToBBox(t *testing.T) {
	tree, err := NewQuadPrefixTree(16)
	if err != nil {
		t.Fatalf("NewQuadPrefixTree() error = %v", err)
	}

	tests := []struct {
		name      string
		token     string
		wantNil   bool
		checkDims bool
	}{
		{
			name:      "Valid token",
			token:     "0123",
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
			token:   "4",
			wantNil: true,
		},
		{
			name:    "Invalid character 'a'",
			token:   "a",
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

func TestQuadPrefixTree_EncodeDecodeRoundtrip(t *testing.T) {
	tree, err := NewQuadPrefixTree(16)
	if err != nil {
		t.Fatalf("NewQuadPrefixTree() error = %v", err)
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
			length: 12,
			delta:  0.1,
		},
		{
			name:   "Roundtrip New York",
			lon:    -74.0060,
			lat:    40.7128,
			length: 12,
			delta:  0.1,
		},
		{
			name:   "Roundtrip Tokyo",
			lon:    139.6917,
			lat:    35.6895,
			length: 12,
			delta:  0.1,
		},
		{
			name:   "Roundtrip Sydney",
			lon:    151.2093,
			lat:    -33.8688,
			length: 12,
			delta:  0.1,
		},
		{
			name:   "Roundtrip Equator/Prime Meridian",
			lon:    0,
			lat:    0,
			length: 16,
			delta:  0.01,
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

func TestQuadPrefixTree_GetCell(t *testing.T) {
	tree, err := NewQuadPrefixTree(16)
	if err != nil {
		t.Fatalf("NewQuadPrefixTree() error = %v", err)
	}

	tests := []struct {
		name    string
		token   string
		wantErr bool
	}{
		{
			name:    "Valid cell",
			token:   "0123",
			wantErr: false,
		},
		{
			name:    "Empty token",
			token:   "",
			wantErr: true,
		},
		{
			name:    "Invalid character '4'",
			token:   "4",
			wantErr: true,
		},
		{
			name:    "Invalid character '9'",
			token:   "9",
			wantErr: true,
		},
		{
			name:    "Invalid character 'a'",
			token:   "a",
			wantErr: true,
		},
		{
			name:    "Long token (truncated)",
			token:   "0123012301230123", // 16 chars
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

func TestQuadPrefixTree_GetCellsForShape(t *testing.T) {
	tree, err := NewQuadPrefixTree(16)
	if err != nil {
		t.Fatalf("NewQuadPrefixTree() error = %v", err)
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
			level:   20,
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

func TestQuadPrefixTree_GetLevelForDistance(t *testing.T) {
	tree, err := NewQuadPrefixTree(16)
	if err != nil {
		t.Fatalf("NewQuadPrefixTree() error = %v", err)
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

func TestQuadPrefixTree_GetNeighbors(t *testing.T) {
	tree, err := NewQuadPrefixTree(16)
	if err != nil {
		t.Fatalf("NewQuadPrefixTree() error = %v", err)
	}

	tests := []struct {
		name         string
		token        string
		wantNil      bool
		minNeighbors int
	}{
		{
			name:         "Normal cell",
			token:        "0123",
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
			token:        "0",
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

func TestQuadCell_GetNeighbors(t *testing.T) {
	tree, err := NewQuadPrefixTree(16)
	if err != nil {
		t.Fatalf("NewQuadPrefixTree() error = %v", err)
	}

	cell, err := tree.GetCell("0123")
	if err != nil {
		t.Fatalf("GetCell() error = %v", err)
	}

	quadCell, ok := cell.(*QuadCell)
	if !ok {
		t.Fatal("Cell is not a QuadCell")
	}

	neighbors := quadCell.GetNeighbors()
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

func TestQuadCell_GetParent(t *testing.T) {
	tree, err := NewQuadPrefixTree(16)
	if err != nil {
		t.Fatalf("NewQuadPrefixTree() error = %v", err)
	}

	tests := []struct {
		name       string
		token      string
		wantNil    bool
		wantParent string
	}{
		{
			name:       "Cell with parent",
			token:      "0123",
			wantNil:    false,
			wantParent: "012",
		},
		{
			name:    "Single char cell (no parent)",
			token:   "0",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cell, err := tree.GetCell(tt.token)
			if err != nil {
				t.Fatalf("GetCell() error = %v", err)
			}

			quadCell, ok := cell.(*QuadCell)
			if !ok {
				t.Fatal("Cell is not a QuadCell")
			}

			parent := quadCell.GetParent()

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

func TestQuadCell_GetChildren(t *testing.T) {
	tree, err := NewQuadPrefixTree(16)
	if err != nil {
		t.Fatalf("NewQuadPrefixTree() error = %v", err)
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
			token:       "012",
			maxLevels:   16,
			wantNil:     false,
			numChildren: 4, // Quad tree has 4 children per cell
		},
		{
			name:      "Max level cell (no children)",
			token:     "0123012301230123", // 16 chars
			maxLevels: 16,
			wantNil:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cell, err := tree.GetCell(tt.token)
			if err != nil {
				t.Fatalf("GetCell() error = %v", err)
			}

			quadCell, ok := cell.(*QuadCell)
			if !ok {
				t.Fatal("Cell is not a QuadCell")
			}

			children := quadCell.GetChildren()

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

func TestQuadCell_IntersectsShape(t *testing.T) {
	tree, err := NewQuadPrefixTree(16)
	if err != nil {
		t.Fatalf("NewQuadPrefixTree() error = %v", err)
	}

	// Get a cell that contains London area
	cell, err := tree.GetCell("3112")
	if err != nil {
		t.Fatalf("GetCell() error = %v", err)
	}

	// Get the cell's bbox
	bbox := cell.GetBoundingBox()
	centerLon := (bbox.MinX + bbox.MaxX) / 2
	centerLat := (bbox.MinY + bbox.MaxY) / 2

	// Test intersection with various shapes
	tests := []struct {
		name     string
		shape    Shape
		expected bool
	}{
		{
			name:     "Point inside cell",
			shape:    NewPoint(centerLon, centerLat),
			expected: true,
		},
		{
			name:     "Point far from cell",
			shape:    NewPoint(-100, 0),
			expected: false,
		},
		{
			name:     "Rectangle overlapping cell",
			shape:    NewRectangle(bbox.MinX-1, bbox.MinY-1, bbox.MaxX+1, bbox.MaxY+1),
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

func TestCalculateQuadPrecision(t *testing.T) {
	tests := []struct {
		level    int
		maxValue float64 // maximum expected precision
		minValue float64 // minimum expected precision
	}{
		{level: 1, maxValue: 50000000, minValue: 30000000},
		{level: 5, maxValue: 3000000, minValue: 2000000},
		{level: 10, maxValue: 100000, minValue: 70000},
		{level: 16, maxValue: 2000, minValue: 1000},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("Level_%d", tt.level), func(t *testing.T) {
			precision := CalculateQuadPrecision(tt.level)

			if precision < tt.minValue {
				t.Errorf("CalculateQuadPrecision(%d) = %f, expected >= %f", tt.level, precision, tt.minValue)
			}
			if precision > tt.maxValue {
				t.Errorf("CalculateQuadPrecision(%d) = %f, expected <= %f", tt.level, precision, tt.maxValue)
			}
		})
	}
}

func TestGetQuadrantName(t *testing.T) {
	tests := []struct {
		quadrant byte
		expected string
	}{
		{'0', "SW"},
		{'1', "SE"},
		{'2', "NW"},
		{'3', "NE"},
		{'4', "Unknown"},
		{'a', "Unknown"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("Quadrant_%c", tt.quadrant), func(t *testing.T) {
			got := GetQuadrantName(tt.quadrant)
			if got != tt.expected {
				t.Errorf("GetQuadrantName(%c) = %s, expected %s", tt.quadrant, got, tt.expected)
			}
		})
	}
}

func TestQuadPrefixTree_InterfaceCompliance(t *testing.T) {
	tree, err := NewQuadPrefixTree(16)
	if err != nil {
		t.Fatalf("NewQuadPrefixTree() error = %v", err)
	}

	// Verify that QuadPrefixTree implements SpatialPrefixTree
	var _ SpatialPrefixTree = tree

	// Verify interface methods work
	_ = tree.GetWorldBounds()
	_ = tree.GetMaxLevels()
	_ = tree.GetLevelForDistance(1.0)

	_, err = tree.GetCellsForShape(NewPoint(0, 0), 5)
	if err != nil {
		t.Errorf("GetCellsForShape() error = %v", err)
	}

	_, err = tree.GetCell("0123")
	if err != nil {
		t.Errorf("GetCell() error = %v", err)
	}
}

func TestQuadTreeSubdivision(t *testing.T) {
	tree, err := NewQuadPrefixTree(16)
	if err != nil {
		t.Fatalf("NewQuadPrefixTree() error = %v", err)
	}

	// Test subdivision at each level
	for level := 1; level <= 5; level++ {
		t.Run(fmt.Sprintf("Level_%d", level), func(t *testing.T) {
			// Encode a point at this level
			token := tree.Encode(0, 0, level)
			if len(token) != level {
				t.Errorf("Encode(0, 0, %d) returned token of length %d", level, len(token))
			}

			// Decode to get bbox
			bbox := tree.DecodeToBBox(token)
			if bbox == nil {
				t.Fatalf("DecodeToBBox(%s) returned nil", token)
			}

			// Calculate expected dimensions at this level
			// Level 1: 180 degrees (half of world)
			// Level 2: 90 degrees
			// Level n: 360 / 2^n
			expectedWidth := 360.0 / math.Pow(2, float64(level))
			expectedHeight := 180.0 / math.Pow(2, float64(level))

			actualWidth := bbox.MaxX - bbox.MinX
			actualHeight := bbox.MaxY - bbox.MinY

			// Allow small floating point tolerance
			tolerance := 0.0001
			if math.Abs(actualWidth-expectedWidth) > tolerance {
				t.Errorf("Level %d: expected width %f, got %f", level, expectedWidth, actualWidth)
			}
			if math.Abs(actualHeight-expectedHeight) > tolerance {
				t.Errorf("Level %d: expected height %f, got %f", level, expectedHeight, actualHeight)
			}
		})
	}
}
