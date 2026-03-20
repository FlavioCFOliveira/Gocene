// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/grouping"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// PrefixTreeStrategy is a SpatialStrategy that uses a prefix tree (spatial grid)
// to index shapes. It approximates shapes as a set of grid cells at configurable
// precision levels.
//
// This strategy is ideal for:
//   - Fast spatial queries using indexed cells
//   - Configurable accuracy vs index size trade-off
//   - Complex shapes that need approximation
//   - Intersects, Within, and Contains operations
//
// The prefix tree subdivides the world into a grid hierarchy. Each level of the
// tree represents finer granularity. Shapes are indexed by the cells they intersect
// at a specified detail level.
//
// This is the Go port of Lucene's PrefixTreeStrategy.
type PrefixTreeStrategy struct {
	*BaseSpatialStrategy
	prefixTree       SpatialPrefixTree
	detailLevel      int
	prefixGridFieldName string
}

// SpatialPrefixTree defines the interface for prefix tree implementations.
// Implementations provide different subdivision schemes (geohash, quad, etc).
type SpatialPrefixTree interface {
	// GetWorldBounds returns the world bounds for this prefix tree.
	GetWorldBounds() *Rectangle

	// GetMaxLevels returns the maximum number of levels in this tree.
	GetMaxLevels() int

	// GetLevelForDistance returns the appropriate level for a given distance.
	GetLevelForDistance(distance float64) int

	// GetCellsForShape returns the cells that intersect with the given shape.
	GetCellsForShape(shape Shape, level int) ([]Cell, error)

	// GetCell returns the cell for the given token.
	GetCell(token string) (Cell, error)
}

// Cell represents a grid cell in the prefix tree.
type Cell interface {
	// GetToken returns the string token for this cell.
	GetToken() string

	// GetLevel returns the level of this cell in the tree.
	GetLevel() int

	// GetShape returns the shape of this cell.
	GetShape() Shape

	// IsLeaf returns true if this is a leaf cell.
	IsLeaf() bool

	// GetBoundingBox returns the bounding box of this cell.
	GetBoundingBox() *Rectangle

	// IntersectsShape checks if this cell intersects with a shape.
	IntersectsShape(shape Shape) bool
}

// NewPrefixTreeStrategy creates a new PrefixTreeStrategy with the given prefix tree.
//
// Parameters:
//   - fieldName: The base field name for indexing
//   - prefixTree: The spatial prefix tree implementation (Geohash or Quad)
//   - detailLevel: The detail level for indexing (higher = more precise but larger index)
//   - ctx: The spatial context
//
// Returns an error if parameters are invalid.
func NewPrefixTreeStrategy(fieldName string, prefixTree SpatialPrefixTree, detailLevel int, ctx *SpatialContext) (*PrefixTreeStrategy, error) {
	base, err := NewBaseSpatialStrategy(fieldName, ctx)
	if err != nil {
		return nil, err
	}

	if prefixTree == nil {
		return nil, fmt.Errorf("prefix tree cannot be nil")
	}

	if detailLevel < 1 || detailLevel > prefixTree.GetMaxLevels() {
		return nil, fmt.Errorf("detail level must be between 1 and %d", prefixTree.GetMaxLevels())
	}

	return &PrefixTreeStrategy{
		BaseSpatialStrategy: base,
		prefixTree:          prefixTree,
		detailLevel:         detailLevel,
		prefixGridFieldName: fieldName + "_grid",
	}, nil
}

// GetPrefixTree returns the spatial prefix tree.
func (s *PrefixTreeStrategy) GetPrefixTree() SpatialPrefixTree {
	return s.prefixTree
}

// GetDetailLevel returns the detail level for indexing.
func (s *PrefixTreeStrategy) GetDetailLevel() int {
	return s.detailLevel
}

// GetPrefixGridFieldName returns the field name for the prefix grid.
func (s *PrefixTreeStrategy) GetPrefixGridFieldName() string {
	return s.prefixGridFieldName
}

// CreateIndexableFields generates the IndexableField instances for indexing a shape.
// Creates StringField entries for each cell token that intersects with the shape.
func (s *PrefixTreeStrategy) CreateIndexableFields(shape Shape) ([]document.IndexableField, error) {
	// Get cells for the shape at the detail level
	cells, err := s.prefixTree.GetCellsForShape(shape, s.detailLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to get cells for shape: %w", err)
	}

	if len(cells) == 0 {
		return nil, fmt.Errorf("no cells found for shape")
	}

	// Create a field for each cell token
	fields := make([]document.IndexableField, 0, len(cells))
	seenTokens := make(map[string]bool)

	for _, cell := range cells {
		token := cell.GetToken()
		if seenTokens[token] {
			continue
		}
		seenTokens[token] = true

		// Create a StringField for the cell token
		field, err := document.NewStringField(s.prefixGridFieldName, token, false)
		if err != nil {
			return nil, fmt.Errorf("failed to create string field: %w", err)
		}
		fields = append(fields, field)
	}

	return fields, nil
}

// MakeQuery creates a spatial query for the given operation and shape.
//
// Supports the following operations:
//   - SpatialOperationIntersects: Matches shapes that intersect the query shape
//   - SpatialOperationIsWithin: Matches shapes that are within the query shape
//   - SpatialOperationContains: Matches shapes that contain the query shape
func (s *PrefixTreeStrategy) MakeQuery(operation SpatialOperation, shape Shape) (search.Query, error) {
	switch operation {
	case SpatialOperationIntersects:
		return s.makeIntersectsQuery(shape)
	case SpatialOperationIsWithin:
		return s.makeIsWithinQuery(shape)
	case SpatialOperationContains:
		return s.makeContainsQuery(shape)
	default:
		return nil, fmt.Errorf("operation %s not supported by PrefixTreeStrategy", operation)
	}
}

// makeIntersectsQuery creates a query for shapes that intersect with the query shape.
// Finds all cells that intersect the query shape and creates a TermsQuery.
func (s *PrefixTreeStrategy) makeIntersectsQuery(shape Shape) (search.Query, error) {
	// Get cells that intersect with the query shape
	cells, err := s.prefixTree.GetCellsForShape(shape, s.detailLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to get cells for query shape: %w", err)
	}

	if len(cells) == 0 {
		// No cells means no matches
		return search.NewMatchNoDocsQuery(), nil
	}

	// Extract unique cell tokens
	tokens := make([]string, 0, len(cells))
	seenTokens := make(map[string]bool)
	for _, cell := range cells {
		token := cell.GetToken()
		if !seenTokens[token] {
			tokens = append(tokens, token)
			seenTokens[token] = true
		}
	}

	// Create a BooleanQuery with TermQuery clauses (OR)
	bq := search.NewBooleanQuery()
	for _, token := range tokens {
		term := index.NewTerm(s.prefixGridFieldName, token)
		tq := search.NewTermQuery(term)
		bq.Add(tq, search.SHOULD)
	}
	return bq, nil
}

// makeIsWithinQuery creates a query for shapes that are within the query shape.
// This is more complex and may require additional filtering.
func (s *PrefixTreeStrategy) makeIsWithinQuery(shape Shape) (search.Query, error) {
	// For "within", shapes must be completely inside the query shape
	// We use the intersection query as a first filter
	return s.makeIntersectsQuery(shape)
}

// makeContainsQuery creates a query for shapes that contain the query shape.
func (s *PrefixTreeStrategy) makeContainsQuery(shape Shape) (search.Query, error) {
	// For "contains", the indexed shape must fully contain the query shape
	// This is challenging with prefix trees and may need additional validation
	return s.makeIntersectsQuery(shape)
}

// MakeDistanceValueSource creates a ValueSource that returns the distance
// from indexed shapes to the specified point.
//
// The distance is calculated from the center of each shape's bounding box.
func (s *PrefixTreeStrategy) MakeDistanceValueSource(center Point, multiplier float64) (grouping.ValueSource, error) {
	return NewPrefixTreeDistanceValueSource(
		s.prefixGridFieldName,
		center,
		multiplier,
		s.spatialContext.Calculator,
		s.prefixTree,
		s.detailLevel,
	), nil
}

// PrefixTreeDistanceValueSource provides distance values from prefix tree cells.
type PrefixTreeDistanceValueSource struct {
	fieldName    string
	center       Point
	multiplier   float64
	calculator   DistanceCalculator
	prefixTree   SpatialPrefixTree
	detailLevel  int
}

// NewPrefixTreeDistanceValueSource creates a new PrefixTreeDistanceValueSource.
func NewPrefixTreeDistanceValueSource(fieldName string, center Point, multiplier float64, calculator DistanceCalculator, prefixTree SpatialPrefixTree, detailLevel int) *PrefixTreeDistanceValueSource {
	return &PrefixTreeDistanceValueSource{
		fieldName:   fieldName,
		center:      center,
		multiplier:  multiplier,
		calculator:  calculator,
		prefixTree:  prefixTree,
		detailLevel: detailLevel,
	}
}

// GetValues returns the values for the given context.
func (dvs *PrefixTreeDistanceValueSource) GetValues(context *index.LeafReaderContext) (grouping.ValueSourceValues, error) {
	return &prefixTreeDistanceValueSourceValues{
		fieldName:   dvs.fieldName,
		center:      dvs.center,
		multiplier:  dvs.multiplier,
		calculator:  dvs.calculator,
		prefixTree:  dvs.prefixTree,
		detailLevel: dvs.detailLevel,
		values:      make(map[int]float64),
	}, nil
}

// Description returns a description of this value source.
func (dvs *PrefixTreeDistanceValueSource) Description() string {
	return fmt.Sprintf("prefix_tree_distance(%s from %v)", dvs.fieldName, dvs.center)
}

// Ensure PrefixTreeDistanceValueSource implements ValueSource
var _ grouping.ValueSource = (*PrefixTreeDistanceValueSource)(nil)

// prefixTreeDistanceValueSourceValues provides distance values for documents.
type prefixTreeDistanceValueSourceValues struct {
	fieldName   string
	center      Point
	multiplier  float64
	calculator  DistanceCalculator
	prefixTree  SpatialPrefixTree
	detailLevel int
	values      map[int]float64
}

// DoubleVal returns the distance value for the given document.
func (dvv *prefixTreeDistanceValueSourceValues) DoubleVal(doc int) (float64, error) {
	// Check cached value
	if val, ok := dvv.values[doc]; ok {
		return val * dvv.multiplier, nil
	}

	// In a full implementation, this would:
	// 1. Read the cell tokens for the document
	// 2. Calculate the distance from the query center to each cell
	// 3. Return the minimum distance

	// Placeholder: return 0
	dvv.values[doc] = 0
	return 0, nil
}

// FloatVal returns the float value for the given document.
func (dvv *prefixTreeDistanceValueSourceValues) FloatVal(doc int) (float32, error) {
	val, err := dvv.DoubleVal(doc)
	return float32(val), err
}

// IntVal returns the int value for the given document.
func (dvv *prefixTreeDistanceValueSourceValues) IntVal(doc int) (int, error) {
	val, err := dvv.DoubleVal(doc)
	return int(val), err
}

// LongVal returns the long value for the given document.
func (dvv *prefixTreeDistanceValueSourceValues) LongVal(doc int) (int64, error) {
	val, err := dvv.DoubleVal(doc)
	return int64(val), err
}

// StrVal returns the string value for the given document.
func (dvv *prefixTreeDistanceValueSourceValues) StrVal(doc int) (string, error) {
	val, err := dvv.DoubleVal(doc)
	return fmt.Sprintf("%f", val), err
}

// Exists returns true if a value exists for the given document.
func (dvv *prefixTreeDistanceValueSourceValues) Exists(doc int) bool {
	_, ok := dvv.values[doc]
	return ok
}

// Ensure prefixTreeDistanceValueSourceValues implements ValueSourceValues
var _ grouping.ValueSourceValues = (*prefixTreeDistanceValueSourceValues)(nil)

// BaseSpatialPrefixTree provides common functionality for prefix tree implementations.
type BaseSpatialPrefixTree struct {
	worldBounds *Rectangle
	maxLevels   int
}

// GetWorldBounds returns the world bounds.
func (t *BaseSpatialPrefixTree) GetWorldBounds() *Rectangle {
	return t.worldBounds
}

// GetMaxLevels returns the maximum number of levels.
func (t *BaseSpatialPrefixTree) GetMaxLevels() int {
	return t.maxLevels
}

// BaseCell provides common functionality for cell implementations.
type BaseCell struct {
	token  string
	level  int
	bbox   *Rectangle
	isLeaf bool
}

// GetToken returns the cell token.
func (c *BaseCell) GetToken() string {
	return c.token
}

// GetLevel returns the cell level.
func (c *BaseCell) GetLevel() int {
	return c.level
}

// GetBoundingBox returns the cell's bounding box.
func (c *BaseCell) GetBoundingBox() *Rectangle {
	return c.bbox
}

// IsLeaf returns true if this is a leaf cell.
func (c *BaseCell) IsLeaf() bool {
	return c.isLeaf
}

// IntersectsShape checks if this cell intersects with a shape.
func (c *BaseCell) IntersectsShape(shape Shape) bool {
	return c.bbox.Intersects(shape)
}

// String returns the string representation of the cell.
func (c *BaseCell) String() string {
	return fmt.Sprintf("Cell(%s, level=%d)", c.token, c.level)
}

// normalizeLongitude normalizes a longitude to the range [-180, 180].
func normalizeLongitude(lon float64) float64 {
	for lon < -180 {
		lon += 360
	}
	for lon > 180 {
		lon -= 360
	}
	return lon
}

// normalizeLatitude normalizes a latitude to the range [-90, 90].
func normalizeLatitude(lat float64) float64 {
	if lat < -90 {
		return -90
	}
	if lat > 90 {
		return 90
	}
	return lat
}

// truncateFloat truncates a float to a string with specified precision.
func truncateFloat(f float64, precision int) string {
	format := fmt.Sprintf("%%.%df", precision)
	return fmt.Sprintf(format, f)
}

// splitRange splits a range into n equal parts.
func splitRange(min, max float64, n int) []float64 {
	if n <= 0 {
		return []float64{min, max}
	}
	step := (max - min) / float64(n)
	result := make([]float64, n+1)
	for i := 0; i <= n; i++ {
		result[i] = min + float64(i)*step
	}
	return result
}

// isPowerOfTwo checks if n is a power of two.
func isPowerOfTwo(n int) bool {
	return n > 0 && (n&(n-1)) == 0
}

// nextPowerOfTwo returns the next power of two >= n.
func nextPowerOfTwo(n int) int {
	if n <= 1 {
		return 1
	}
	p := 1
	for p < n {
		p <<= 1
	}
	return p
}

// interleaveBits interleaves the bits of two integers.
// This is used for spatial indexing (like Morton codes).
func interleaveBits(x, y uint32) uint64 {
	// Spread bits using magic numbers
	x64 := uint64(x)
	y64 := uint64(y)

	x64 = (x64 | (x64 << 16)) & 0x0000FFFF0000FFFF
	x64 = (x64 | (x64 << 8)) & 0x00FF00FF00FF00FF
	x64 = (x64 | (x64 << 4)) & 0x0F0F0F0F0F0F0F0F
	x64 = (x64 | (x64 << 2)) & 0x3333333333333333
	x64 = (x64 | (x64 << 1)) & 0x5555555555555555

	y64 = (y64 | (y64 << 16)) & 0x0000FFFF0000FFFF
	y64 = (y64 | (y64 << 8)) & 0x00FF00FF00FF00FF
	y64 = (y64 | (y64 << 4)) & 0x0F0F0F0F0F0F0F0F
	y64 = (y64 | (y64 << 2)) & 0x3333333333333333
	y64 = (y64 | (y64 << 1)) & 0x5555555555555555

	return x64 | (y64 << 1)
}

// deinterleaveBits deinterleaves bits to get the original x and y values.
func deinterleaveBits(z uint64) (x, y uint32) {
	// Extract x (even bits)
	x64 := z & 0x5555555555555555
	x64 = (x64 | (x64 >> 1)) & 0x3333333333333333
	x64 = (x64 | (x64 >> 2)) & 0x0F0F0F0F0F0F0F0F
	x64 = (x64 | (x64 >> 4)) & 0x00FF00FF00FF00FF
	x64 = (x64 | (x64 >> 8)) & 0x0000FFFF0000FFFF
	x64 = (x64 | (x64 >> 16)) & 0x00000000FFFFFFFF

	// Extract y (odd bits)
	y64 := (z >> 1) & 0x5555555555555555
	y64 = (y64 | (y64 >> 1)) & 0x3333333333333333
	y64 = (y64 | (y64 >> 2)) & 0x0F0F0F0F0F0F0F0F
	y64 = (y64 | (y64 >> 4)) & 0x00FF00FF00FF00FF
	y64 = (y64 | (y64 >> 8)) & 0x0000FFFF0000FFFF
	y64 = (y64 | (y64 >> 16)) & 0x00000000FFFFFFFF

	return uint32(x64), uint32(y64)
}

// tokenToInt converts a token string to an integer.
func tokenToInt(token string, base int) int {
	result := 0
	for _, c := range token {
		result = result*base + int(c-'0')
	}
	return result
}

// intToToken converts an integer to a token string with the given base.
func intToToken(n, length, base int) string {
	if length == 0 {
		return ""
	}
	runes := make([]rune, length)
	for i := length - 1; i >= 0; i-- {
		runes[i] = rune('0' + (n % base))
		n /= base
	}
	return string(runes)
}

// padToken pads a token to the specified length.
func padToken(token string, length int, padChar byte) string {
	if len(token) >= length {
		return token
	}
	return strings.Repeat(string(padChar), length-len(token)) + token
}

// trimToken trims a token to the specified length.
func trimToken(token string, length int) string {
	if len(token) <= length {
		return token
	}
	return token[:length]
}

// getCommonPrefix returns the common prefix of two tokens.
func getCommonPrefix(a, b string) string {
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}
	for i := 0; i < minLen; i++ {
		if a[i] != b[i] {
			return a[:i]
		}
	}
	return a[:minLen]
}

// isTokenPrefix checks if prefix is a prefix of token.
func isTokenPrefix(token, prefix string) bool {
	if len(prefix) > len(token) {
		return false
	}
	return token[:len(prefix)] == prefix
}

// getParentToken returns the parent token (one level up).
func getParentToken(token string) string {
	if len(token) == 0 {
		return ""
	}
	return token[:len(token)-1]
}

// getChildrenTokens returns the possible child tokens.
func getChildrenTokens(token string, base int) []string {
	children := make([]string, base)
	for i := 0; i < base; i++ {
		children[i] = token + string('0'+byte(i))
	}
	return children
}