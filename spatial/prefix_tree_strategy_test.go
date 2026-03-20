// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"testing"
)

// MockSpatialPrefixTree is a mock implementation of SpatialPrefixTree for testing.
type MockSpatialPrefixTree struct {
	BaseSpatialPrefixTree
}

// NewMockSpatialPrefixTree creates a new mock prefix tree.
func NewMockSpatialPrefixTree() *MockSpatialPrefixTree {
	return &MockSpatialPrefixTree{
		BaseSpatialPrefixTree: BaseSpatialPrefixTree{
			worldBounds: NewRectangle(-180, -90, 180, 90),
			maxLevels:   16,
		},
	}
}

// GetLevelForDistance returns the appropriate level for a given distance.
func (t *MockSpatialPrefixTree) GetLevelForDistance(distance float64) int {
	// Simple implementation: smaller distance = higher level
	if distance < 1 {
		return 10
	}
	if distance < 10 {
		return 8
	}
	if distance < 100 {
		return 6
	}
	return 4
}

// GetCellsForShape returns mock cells for a shape.
func (t *MockSpatialPrefixTree) GetCellsForShape(shape Shape, level int) ([]Cell, error) {
	bbox := shape.GetBoundingBox()
	center := bbox.GetCenter()

	// Create a simple mock cell
	cell := &MockCell{
		BaseCell: BaseCell{
			token:  "mock_cell",
			level:  level,
			bbox:   bbox,
			isLeaf: level >= t.maxLevels,
		},
		center: center,
	}

	return []Cell{cell}, nil
}

// GetCell returns a mock cell for the given token.
func (t *MockSpatialPrefixTree) GetCell(token string) (Cell, error) {
	return &MockCell{
		BaseCell: BaseCell{
			token:  token,
			level:  len(token),
			bbox:   t.worldBounds,
			isLeaf: false,
		},
		center: t.worldBounds.GetCenter(),
	}, nil
}

// MockCell is a mock implementation of Cell for testing.
type MockCell struct {
	BaseCell
	center Point
}

// GetShape returns the cell's shape (center point).
func (c *MockCell) GetShape() Shape {
	return c.center
}

func TestNewPrefixTreeStrategy(t *testing.T) {
	ctx := NewSpatialContext()
	prefixTree := NewMockSpatialPrefixTree()

	tests := []struct {
		name        string
		fieldName   string
		prefixTree  SpatialPrefixTree
		detailLevel int
		ctx         *SpatialContext
		wantErr     bool
		errContains string
	}{
		{
			name:        "valid strategy",
			fieldName:   "location",
			prefixTree:  prefixTree,
			detailLevel: 8,
			ctx:         ctx,
			wantErr:     false,
		},
		{
			name:        "empty field name",
			fieldName:   "",
			prefixTree:  prefixTree,
			detailLevel: 8,
			ctx:         ctx,
			wantErr:     true,
			errContains: "field name cannot be empty",
		},
		{
			name:        "nil context",
			fieldName:   "location",
			prefixTree:  prefixTree,
			detailLevel: 8,
			ctx:         nil,
			wantErr:     true,
			errContains: "spatial context cannot be nil",
		},
		{
			name:        "nil prefix tree",
			fieldName:   "location",
			prefixTree:  nil,
			detailLevel: 8,
			ctx:         ctx,
			wantErr:     true,
			errContains: "prefix tree cannot be nil",
		},
		{
			name:        "detail level too low",
			fieldName:   "location",
			prefixTree:  prefixTree,
			detailLevel: 0,
			ctx:         ctx,
			wantErr:     true,
			errContains: "detail level must be between",
		},
		{
			name:        "detail level too high",
			fieldName:   "location",
			prefixTree:  prefixTree,
			detailLevel: 20,
			ctx:         ctx,
			wantErr:     true,
			errContains: "detail level must be between",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy, err := NewPrefixTreeStrategy(tt.fieldName, tt.prefixTree, tt.detailLevel, tt.ctx)
			if tt.wantErr {
				if err == nil {
					t.Errorf("NewPrefixTreeStrategy() expected error but got none")
					return
				}
				if tt.errContains != "" && !containsSubstring(err.Error(), tt.errContains) {
					t.Errorf("NewPrefixTreeStrategy() error = %v, should contain %v", err, tt.errContains)
				}
				return
			}
			if err != nil {
				t.Errorf("NewPrefixTreeStrategy() unexpected error = %v", err)
				return
			}
			if strategy == nil {
				t.Error("NewPrefixTreeStrategy() returned nil strategy")
				return
			}

			// Verify field name
			if strategy.GetFieldName() != tt.fieldName {
				t.Errorf("GetFieldName() = %v, want %v", strategy.GetFieldName(), tt.fieldName)
			}
			if strategy.GetDetailLevel() != tt.detailLevel {
				t.Errorf("GetDetailLevel() = %v, want %v", strategy.GetDetailLevel(), tt.detailLevel)
			}
			if strategy.GetPrefixGridFieldName() != tt.fieldName+"_grid" {
				t.Errorf("GetPrefixGridFieldName() = %v, want %v", strategy.GetPrefixGridFieldName(), tt.fieldName+"_grid")
			}
		})
	}
}

func TestPrefixTreeStrategy_CreateIndexableFields(t *testing.T) {
	ctx := NewSpatialContext()
	prefixTree := NewMockSpatialPrefixTree()
	strategy, _ := NewPrefixTreeStrategy("location", prefixTree, 8, ctx)

	tests := []struct {
		name       string
		shape      Shape
		wantErr    bool
		fieldCount int
	}{
		{
			name:       "valid point",
			shape:      NewPoint(10.0, 20.0),
			wantErr:    false,
			fieldCount: 1,
		},
		{
			name:       "valid rectangle",
			shape:      NewRectangle(10.0, 20.0, 30.0, 40.0),
			wantErr:    false,
			fieldCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields, err := strategy.CreateIndexableFields(tt.shape)
			if tt.wantErr {
				if err == nil {
					t.Errorf("CreateIndexableFields() expected error but got none")
					return
				}
				return
			}
			if err != nil {
				t.Errorf("CreateIndexableFields() unexpected error = %v", err)
				return
			}
			if len(fields) != tt.fieldCount {
				t.Errorf("CreateIndexableFields() returned %d fields, want %d", len(fields), tt.fieldCount)
			}
		})
	}
}

func TestPrefixTreeStrategy_MakeQuery(t *testing.T) {
	ctx := NewSpatialContext()
	prefixTree := NewMockSpatialPrefixTree()
	strategy, _ := NewPrefixTreeStrategy("location", prefixTree, 8, ctx)

	tests := []struct {
		name        string
		operation   SpatialOperation
		shape       Shape
		wantErr     bool
		errContains string
	}{
		{
			name:      "intersects with rectangle",
			operation: SpatialOperationIntersects,
			shape:     NewRectangle(10.0, 20.0, 30.0, 40.0),
			wantErr:   false,
		},
		{
			name:      "isWithin with rectangle",
			operation: SpatialOperationIsWithin,
			shape:     NewRectangle(10.0, 20.0, 30.0, 40.0),
			wantErr:   false,
		},
		{
			name:      "contains with rectangle",
			operation: SpatialOperationContains,
			shape:     NewRectangle(10.0, 20.0, 30.0, 40.0),
			wantErr:   false,
		},
		{
			name:        "unsupported operation",
			operation:   SpatialOperationEquals,
			shape:       NewRectangle(10.0, 20.0, 30.0, 40.0),
			wantErr:     true,
			errContains: "not supported by PrefixTreeStrategy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, err := strategy.MakeQuery(tt.operation, tt.shape)
			if tt.wantErr {
				if err == nil {
					t.Errorf("MakeQuery() expected error but got none")
					return
				}
				if tt.errContains != "" && !containsSubstring(err.Error(), tt.errContains) {
					t.Errorf("MakeQuery() error = %v, should contain %v", err, tt.errContains)
				}
				return
			}
			if err != nil {
				t.Errorf("MakeQuery() unexpected error = %v", err)
				return
			}
			if query == nil {
				t.Error("MakeQuery() returned nil query")
			}
		})
	}
}

func TestPrefixTreeStrategy_MakeDistanceValueSource(t *testing.T) {
	ctx := NewSpatialContext()
	prefixTree := NewMockSpatialPrefixTree()
	strategy, _ := NewPrefixTreeStrategy("location", prefixTree, 8, ctx)
	center := NewPoint(0.0, 0.0)

	vs, err := strategy.MakeDistanceValueSource(center, 1.0)
	if err != nil {
		t.Errorf("MakeDistanceValueSource() unexpected error = %v", err)
		return
	}
	if vs == nil {
		t.Error("MakeDistanceValueSource() returned nil value source")
		return
	}

	// Check description
	desc := vs.Description()
	if desc == "" {
		t.Error("Description() returned empty string")
	}
}

func TestPrefixTreeDistanceValueSource(t *testing.T) {
	prefixTree := NewMockSpatialPrefixTree()
	vs := NewPrefixTreeDistanceValueSource(
		"location_grid",
		NewPoint(0.0, 0.0),
		1.0,
		&HaversineCalculator{},
		prefixTree,
		8,
	)

	if vs == nil {
		t.Error("NewPrefixTreeDistanceValueSource() returned nil")
		return
	}

	// Test description
	desc := vs.Description()
	if desc == "" {
		t.Error("Description() returned empty string")
	}

	// Test GetValues
	values, err := vs.GetValues(nil)
	if err != nil {
		t.Errorf("GetValues() unexpected error = %v", err)
		return
	}
	if values == nil {
		t.Error("GetValues() returned nil")
		return
	}

	// Test DoubleVal (placeholder implementation returns 0)
	val, err := values.DoubleVal(0)
	if err != nil {
		t.Errorf("DoubleVal() unexpected error = %v", err)
		return
	}
	if val != 0 {
		t.Errorf("DoubleVal() = %v, want 0 (placeholder)", val)
	}
}

func TestPrefixTreeDistanceValueSource_Types(t *testing.T) {
	prefixTree := NewMockSpatialPrefixTree()
	vs := NewPrefixTreeDistanceValueSource(
		"location_grid",
		NewPoint(0.0, 0.0),
		1.0,
		&HaversineCalculator{},
		prefixTree,
		8,
	)

	values, _ := vs.GetValues(nil)

	// Test FloatVal
	floatVal, err := values.FloatVal(0)
	if err != nil {
		t.Errorf("FloatVal() error = %v", err)
	}
	if floatVal != 0 {
		t.Errorf("FloatVal() = %v, want 0", floatVal)
	}

	// Test IntVal
	intVal, err := values.IntVal(0)
	if err != nil {
		t.Errorf("IntVal() error = %v", err)
	}
	if intVal != 0 {
		t.Errorf("IntVal() = %v, want 0", intVal)
	}

	// Test LongVal
	longVal, err := values.LongVal(0)
	if err != nil {
		t.Errorf("LongVal() error = %v", err)
	}
	if longVal != 0 {
		t.Errorf("LongVal() = %v, want 0", longVal)
	}

	// Test StrVal
	strVal, err := values.StrVal(0)
	if err != nil {
		t.Errorf("StrVal() error = %v", err)
	}
	if strVal != "0.000000" {
		t.Errorf("StrVal() = %v, want \"0.000000\"", strVal)
	}
}

func TestBaseSpatialPrefixTree(t *testing.T) {
	tree := &BaseSpatialPrefixTree{
		worldBounds: NewRectangle(-180, -90, 180, 90),
		maxLevels:   16,
	}

	if tree.GetWorldBounds() == nil {
		t.Error("GetWorldBounds() returned nil")
	}

	if tree.GetMaxLevels() != 16 {
		t.Errorf("GetMaxLevels() = %d, want 16", tree.GetMaxLevels())
	}
}

func TestBaseCell(t *testing.T) {
	cell := &BaseCell{
		token:  "test_token",
		level:  5,
		bbox:   NewRectangle(0, 0, 10, 10),
		isLeaf: false,
	}

	if cell.GetToken() != "test_token" {
		t.Errorf("GetToken() = %v, want \"test_token\"", cell.GetToken())
	}

	if cell.GetLevel() != 5 {
		t.Errorf("GetLevel() = %d, want 5", cell.GetLevel())
	}

	if cell.GetBoundingBox() == nil {
		t.Error("GetBoundingBox() returned nil")
	}

	if cell.IsLeaf() {
		t.Error("IsLeaf() should be false")
	}

	// Test String
	str := cell.String()
	if str == "" {
		t.Error("String() returned empty string")
	}
}

func TestNormalizeLongitude(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{0, 0},
		{180, 180},
		{-180, -180},
		{360, 0},
		{-360, 0},
		{540, 180},
		{-540, -180},
		{200, -160},
		{-200, 160},
	}

	for _, tt := range tests {
		result := normalizeLongitude(tt.input)
		if result != tt.expected {
			t.Errorf("normalizeLongitude(%f) = %f, want %f", tt.input, result, tt.expected)
		}
	}
}

func TestNormalizeLatitude(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{0, 0},
		{90, 90},
		{-90, -90},
		{100, 90},
		{-100, -90},
		{45, 45},
		{-45, -45},
	}

	for _, tt := range tests {
		result := normalizeLatitude(tt.input)
		if result != tt.expected {
			t.Errorf("normalizeLatitude(%f) = %f, want %f", tt.input, result, tt.expected)
		}
	}
}

func TestTruncateFloat(t *testing.T) {
	result := truncateFloat(3.14159, 2)
	if result != "3.14" {
		t.Errorf("truncateFloat(3.14159, 2) = %v, want \"3.14\"", result)
	}
}

func TestSplitRange(t *testing.T) {
	result := splitRange(0, 100, 4)
	if len(result) != 5 {
		t.Errorf("splitRange(0, 100, 4) returned %d points, want 5", len(result))
	}

	expected := []float64{0, 25, 50, 75, 100}
	for i, v := range result {
		if v != expected[i] {
			t.Errorf("splitRange()[%d] = %f, want %f", i, v, expected[i])
		}
	}
}

func TestIsPowerOfTwo(t *testing.T) {
	tests := []struct {
		input    int
		expected bool
	}{
		{1, true},
		{2, true},
		{4, true},
		{8, true},
		{16, true},
		{3, false},
		{5, false},
		{0, false},
		{-1, false},
	}

	for _, tt := range tests {
		result := isPowerOfTwo(tt.input)
		if result != tt.expected {
			t.Errorf("isPowerOfTwo(%d) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestNextPowerOfTwo(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{0, 1},
		{1, 1},
		{2, 2},
		{3, 4},
		{5, 8},
		{15, 16},
		{17, 32},
	}

	for _, tt := range tests {
		result := nextPowerOfTwo(tt.input)
		if result != tt.expected {
			t.Errorf("nextPowerOfTwo(%d) = %d, want %d", tt.input, result, tt.expected)
		}
	}
}

func TestInterleaveBits(t *testing.T) {
	// Test with known values
	result := interleaveBits(0, 0)
	if result != 0 {
		t.Errorf("interleaveBits(0, 0) = %d, want 0", result)
	}

	result = interleaveBits(1, 0)
	if result != 1 {
		t.Errorf("interleaveBits(1, 0) = %d, want 1", result)
	}

	result = interleaveBits(0, 1)
	if result != 2 {
		t.Errorf("interleaveBits(0, 1) = %d, want 2", result)
	}
}

func TestDeinterleaveBits(t *testing.T) {
	// Test round-trip
	x, y := deinterleaveBits(0)
	if x != 0 || y != 0 {
		t.Errorf("deinterleaveBits(0) = (%d, %d), want (0, 0)", x, y)
	}

	x, y = deinterleaveBits(1)
	if x != 1 || y != 0 {
		t.Errorf("deinterleaveBits(1) = (%d, %d), want (1, 0)", x, y)
	}

	x, y = deinterleaveBits(2)
	if x != 0 || y != 1 {
		t.Errorf("deinterleaveBits(2) = (%d, %d), want (0, 1)", x, y)
	}
}

func TestTokenToInt(t *testing.T) {
	result := tokenToInt("123", 10)
	if result != 123 {
		t.Errorf("tokenToInt(\"123\", 10) = %d, want 123", result)
	}

	result = tokenToInt("10", 2)
	if result != 2 {
		t.Errorf("tokenToInt(\"10\", 2) = %d, want 2", result)
	}
}

func TestIntToToken(t *testing.T) {
	result := intToToken(123, 3, 10)
	if result != "123" {
		t.Errorf("intToToken(123, 3, 10) = %v, want \"123\"", result)
	}

	result = intToToken(5, 4, 2)
	if result != "0101" {
		t.Errorf("intToToken(5, 4, 2) = %v, want \"0101\"", result)
	}
}

func TestPadToken(t *testing.T) {
	result := padToken("abc", 5, '0')
	if result != "00abc" {
		t.Errorf("padToken(\"abc\", 5, '0') = %v, want \"00abc\"", result)
	}

	result = padToken("abcdef", 3, '0')
	if result != "abcdef" {
		t.Errorf("padToken(\"abcdef\", 3, '0') = %v, want \"abcdef\"", result)
	}
}

func TestTrimToken(t *testing.T) {
	result := trimToken("abcdef", 3)
	if result != "abc" {
		t.Errorf("trimToken(\"abcdef\", 3) = %v, want \"abc\"", result)
	}

	result = trimToken("ab", 5)
	if result != "ab" {
		t.Errorf("trimToken(\"ab\", 5) = %v, want \"ab\"", result)
	}
}

func TestGetCommonPrefix(t *testing.T) {
	result := getCommonPrefix("abcdef", "abcxyz")
	if result != "abc" {
		t.Errorf("getCommonPrefix(\"abcdef\", \"abcxyz\") = %v, want \"abc\"", result)
	}

	result = getCommonPrefix("abc", "abc")
	if result != "abc" {
		t.Errorf("getCommonPrefix(\"abc\", \"abc\") = %v, want \"abc\"", result)
	}

	result = getCommonPrefix("abc", "xyz")
	if result != "" {
		t.Errorf("getCommonPrefix(\"abc\", \"xyz\") = %v, want \"\"", result)
	}
}

func TestIsTokenPrefix(t *testing.T) {
	if !isTokenPrefix("abcdef", "abc") {
		t.Error("isTokenPrefix(\"abcdef\", \"abc\") should be true")
	}

	if isTokenPrefix("abc", "abcdef") {
		t.Error("isTokenPrefix(\"abc\", \"abcdef\") should be false")
	}

	if !isTokenPrefix("abc", "abc") {
		t.Error("isTokenPrefix(\"abc\", \"abc\") should be true")
	}
}

func TestGetParentToken(t *testing.T) {
	result := getParentToken("abcdef")
	if result != "abcde" {
		t.Errorf("getParentToken(\"abcdef\") = %v, want \"abcde\"", result)
	}

	result = getParentToken("")
	if result != "" {
		t.Errorf("getParentToken(\"\") = %v, want \"\"", result)
	}
}

func TestGetChildrenTokens(t *testing.T) {
	result := getChildrenTokens("abc", 4)
	if len(result) != 4 {
		t.Errorf("getChildrenTokens(\"abc\", 4) returned %d children, want 4", len(result))
	}

	expected := []string{"abc0", "abc1", "abc2", "abc3"}
	for i, child := range result {
		if child != expected[i] {
			t.Errorf("getChildrenTokens()[%d] = %v, want %v", i, child, expected[i])
		}
	}
}
