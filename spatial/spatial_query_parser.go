// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// SpatialQueryParser parses spatial query strings into spatial Query objects.
// This parser extends the standard query syntax with spatial operations.
//
// Supported query formats:
//   - "Intersects(POINT(-1 1))" - Intersection with point
//   - "IsWithin(CIRCLE(-1 1 d=10))" - Within circle
//   - "Contains(ENVELOPE(-1, 1, 1, -1))" - Contains envelope
//   - "Disjoint(BBOX(-10, 10, 10, -10))" - Disjoint from bbox
//   - "geo_distance(field:location point:POINT(-1 1) distance:10km)" - Distance query
//   - "geo_box(field:location minX:-10 maxX:10 minY:-10 maxY:10)" - Bounding box query
//
// The parser supports multiple spatial strategies:
//   - PointVector: Uses DoublePoint fields for precise spatial queries
//   - BBox: Uses bounding box fields for envelope queries
//   - PrefixTree: Uses geohash/quad prefix tree for grid-based queries
type SpatialQueryParser struct {
	ctx             *SpatialContext
	argsParser      *SpatialArgsParser
	defaultField    string
	defaultStrategy SpatialStrategy
}

// NewSpatialQueryParser creates a new spatial query parser.
//
// Parameters:
//   - ctx: The spatial context for shape operations
//   - defaultField: The default field name for spatial queries
//   - defaultStrategy: The default spatial strategy to use
func NewSpatialQueryParser(ctx *SpatialContext, defaultField string, defaultStrategy SpatialStrategy) *SpatialQueryParser {
	return &SpatialQueryParser{
		ctx:             ctx,
		argsParser:      NewSpatialArgsParser(ctx),
		defaultField:    defaultField,
		defaultStrategy: defaultStrategy,
	}
}

// NewSpatialQueryParserWithContext creates a new parser with just a spatial context.
// Uses default values for field and strategy.
func NewSpatialQueryParserWithContext(ctx *SpatialContext) *SpatialQueryParser {
	return &SpatialQueryParser{
		ctx:        ctx,
		argsParser: NewSpatialArgsParser(ctx),
	}
}

// Parse parses a spatial query string into a Query.
//
// The query string can be in various formats:
//   - Standard spatial operation format: "Intersects(POINT(-1 1))"
//   - Extended format with field specification: "Intersects(location:POINT(-1 1))"
//   - Function-style format: "geo_distance(field:location point:POINT(-1 1) distance:10)"
func (p *SpatialQueryParser) Parse(queryString string) (search.Query, error) {
	if queryString == "" {
		return nil, fmt.Errorf("empty query string")
	}

	queryString = strings.TrimSpace(queryString)

	// Check for function-style queries first
	if strings.HasPrefix(queryString, "geo_") {
		return p.parseFunctionQuery(queryString)
	}

	// Try to parse as standard spatial operation
	return p.parseSpatialOperation(queryString)
}

// parseSpatialOperation parses a standard spatial operation like "Intersects(POINT(-1 1))".
func (p *SpatialQueryParser) parseSpatialOperation(queryString string) (search.Query, error) {
	// Parse using SpatialArgsParser
	args, err := p.argsParser.Parse(queryString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse spatial operation: %w", err)
	}

	// Create appropriate query based on operation and strategy
	return p.createQueryFromArgs(args)
}

// parseFunctionQuery parses function-style queries like "geo_distance(...)" or "geo_box(...)".
func (p *SpatialQueryParser) parseFunctionQuery(queryString string) (search.Query, error) {
	// Find opening parenthesis
	idx := strings.Index(queryString, "(")
	if idx == -1 {
		return nil, fmt.Errorf("invalid function query format: missing '('")
	}

	if !strings.HasSuffix(queryString, ")") {
		return nil, fmt.Errorf("invalid function query format: missing ')'")
	}

	functionName := strings.ToLower(strings.TrimSpace(queryString[:idx]))
	paramsStr := queryString[idx+1 : len(queryString)-1]

	switch functionName {
	case "geo_distance":
		return p.parseGeoDistanceQuery(paramsStr)
	case "geo_box", "geo_bbox":
		return p.parseGeoBoxQuery(paramsStr)
	case "geo_intersects":
		return p.parseGeoIntersectsQuery(paramsStr)
	case "geo_within":
		return p.parseGeoWithinQuery(paramsStr)
	default:
		return nil, fmt.Errorf("unknown spatial function: %s", functionName)
	}
}

// parseGeoDistanceQuery parses a geo_distance query.
// Format: "geo_distance(field:location point:POINT(-1 1) distance:10km)"
func (p *SpatialQueryParser) parseGeoDistanceQuery(paramsStr string) (search.Query, error) {
	params := p.parseNamedParameters(paramsStr)

	// Get field name
	field := p.defaultField
	if f, ok := params["field"]; ok {
		field = f
	}
	if field == "" {
		return nil, fmt.Errorf("field is required for geo_distance query")
	}

	// Parse point
	pointStr, ok := params["point"]
	if !ok {
		return nil, fmt.Errorf("point is required for geo_distance query")
	}
	center, err := p.parsePointFromString(pointStr)
	if err != nil {
		return nil, fmt.Errorf("invalid point: %w", err)
	}

	// Parse distance
	distanceStr, ok := params["distance"]
	if !ok {
		return nil, fmt.Errorf("distance is required for geo_distance query")
	}
	distance, err := p.parseDistance(distanceStr)
	if err != nil {
		return nil, fmt.Errorf("invalid distance: %w", err)
	}

	// Create distance query
	return NewDistanceQuery(
		field,
		center,
		distance,
		nil, // prefix tree - will be set by caller
		9,   // detail level
		p.ctx.Calculator,
	), nil
}

// parseGeoBoxQuery parses a geo_box/bbox query.
// Format: "geo_box(field:location minX:-10 maxX:10 minY:-10 maxY:10)"
func (p *SpatialQueryParser) parseGeoBoxQuery(paramsStr string) (search.Query, error) {
	params := p.parseNamedParameters(paramsStr)

	// Get field name
	field := p.defaultField
	if f, ok := params["field"]; ok {
		field = f
	}
	if field == "" {
		return nil, fmt.Errorf("field is required for geo_box query")
	}

	// Parse bounds
	minX, err := p.parseFloatParam(params, "minx", "minX", "min_lon", "minLon")
	if err != nil {
		return nil, err
	}
	maxX, err := p.parseFloatParam(params, "maxx", "maxX", "max_lon", "maxLon")
	if err != nil {
		return nil, err
	}
	minY, err := p.parseFloatParam(params, "miny", "minY", "min_lat", "minLat")
	if err != nil {
		return nil, err
	}
	maxY, err := p.parseFloatParam(params, "maxy", "maxY", "max_lat", "maxLat")
	if err != nil {
		return nil, err
	}

	// Create rectangle shape
	rect := NewRectangle(minX, minY, maxX, maxY)

	// Create spatial args for intersects operation
	args := NewSpatialArgs(SpatialOperationIntersects, rect)
	args.SetDistErrPct(0.025)

	return p.createQueryFromArgs(args)
}

// parseGeoIntersectsQuery parses a geo_intersects query.
// Format: "geo_intersects(field:location shape:POINT(-1 1))"
func (p *SpatialQueryParser) parseGeoIntersectsQuery(paramsStr string) (search.Query, error) {
	params := p.parseNamedParameters(paramsStr)

	// Parse shape
	shapeStr, ok := params["shape"]
	if !ok {
		return nil, fmt.Errorf("shape is required for geo_intersects query")
	}

	shape, err := p.argsParser.parseShape(shapeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid shape: %w", err)
	}

	// Create spatial args
	args := NewSpatialArgs(SpatialOperationIntersects, shape)

	return p.createQueryFromArgs(args)
}

// parseGeoWithinQuery parses a geo_within query.
// Format: "geo_within(field:location shape:ENVELOPE(-10, 10, 10, -10))"
func (p *SpatialQueryParser) parseGeoWithinQuery(paramsStr string) (search.Query, error) {
	params := p.parseNamedParameters(paramsStr)

	// Parse shape
	shapeStr, ok := params["shape"]
	if !ok {
		return nil, fmt.Errorf("shape is required for geo_within query")
	}

	shape, err := p.argsParser.parseShape(shapeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid shape: %w", err)
	}

	// Create spatial args
	args := NewSpatialArgs(SpatialOperationIsWithin, shape)

	return p.createQueryFromArgs(args)
}

// parseNamedParameters parses named parameters from a string.
// Format: "key1:value1 key2:value2" or "key1:value1, key2:value2"
func (p *SpatialQueryParser) parseNamedParameters(paramsStr string) map[string]string {
	params := make(map[string]string)

	// Split by comma or space
	var parts []string
	if strings.Contains(paramsStr, ",") {
		parts = strings.Split(paramsStr, ",")
	} else {
		parts = strings.Fields(paramsStr)
	}

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Find colon separator
		idx := strings.Index(part, ":")
		if idx == -1 {
			continue
		}

		key := strings.TrimSpace(part[:idx])
		value := strings.TrimSpace(part[idx+1:])

		// Handle quoted values
		if strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") {
			value = value[1 : len(value)-1]
		}
		if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
			value = value[1 : len(value)-1]
		}

		params[key] = value
	}

	return params
}

// parsePointFromString parses a point from various formats.
func (p *SpatialQueryParser) parsePointFromString(s string) (Point, error) {
	// Try to parse as POINT(...) format
	s = strings.TrimSpace(s)
	if strings.HasPrefix(strings.ToUpper(s), "POINT(") {
		return p.argsParser.parsePoint(s[6 : len(s)-1])
	}

	// Try to parse as "x y" format
	parts := strings.Fields(s)
	if len(parts) == 2 {
		var x, y float64
		if _, err := fmt.Sscanf(parts[0], "%f", &x); err == nil {
			if _, err := fmt.Sscanf(parts[1], "%f", &y); err == nil {
				return Point{X: x, Y: y}, nil
			}
		}
	}

	// Try to parse as "x,y" format
	parts = strings.Split(s, ",")
	if len(parts) == 2 {
		var x, y float64
		if _, err := fmt.Sscanf(strings.TrimSpace(parts[0]), "%f", &x); err == nil {
			if _, err := fmt.Sscanf(strings.TrimSpace(parts[1]), "%f", &y); err == nil {
				return Point{X: x, Y: y}, nil
			}
		}
	}

	return Point{}, fmt.Errorf("cannot parse point from: %s", s)
}

// parseDistance parses a distance string, supporting units.
func (p *SpatialQueryParser) parseDistance(s string) (float64, error) {
	s = strings.TrimSpace(strings.ToLower(s))

	// Remove units and convert
	multiplier := 1.0
	if strings.HasSuffix(s, "km") {
		multiplier = 1.0 // Already in km, need to convert to degrees approximately
		s = strings.TrimSuffix(s, "km")
		// Rough conversion: 1 degree ≈ 111 km
		multiplier = 1.0 / 111.0
	} else if strings.HasSuffix(s, "mi") || strings.HasSuffix(s, "miles") {
		s = strings.TrimSuffix(s, "mi")
		s = strings.TrimSuffix(s, "miles")
		// Rough conversion: 1 degree ≈ 69 miles
		multiplier = 1.0 / 69.0
	} else if strings.HasSuffix(s, "m") {
		multiplier = 1.0 / 111000.0 // meters to degrees
		s = strings.TrimSuffix(s, "m")
	} else if strings.HasSuffix(s, "deg") || strings.HasSuffix(s, "degrees") {
		s = strings.TrimSuffix(s, "deg")
		s = strings.TrimSuffix(s, "degrees")
		// Already in degrees
	}

	var distance float64
	if _, err := fmt.Sscanf(strings.TrimSpace(s), "%f", &distance); err != nil {
		return 0, fmt.Errorf("invalid distance value: %w", err)
	}

	return distance * multiplier, nil
}

// parseFloatParam extracts a float parameter using multiple possible key names.
func (p *SpatialQueryParser) parseFloatParam(params map[string]string, keys ...string) (float64, error) {
	for _, key := range keys {
		if val, ok := params[key]; ok {
			var result float64
			if _, err := fmt.Sscanf(val, "%f", &result); err != nil {
				return 0, fmt.Errorf("invalid value for %s: %w", key, err)
			}
			return result, nil
		}
	}
	return 0, fmt.Errorf("missing required parameter (tried: %v)", keys)
}

// createQueryFromArgs creates a Query from SpatialArgs.
func (p *SpatialQueryParser) createQueryFromArgs(args *SpatialArgs) (search.Query, error) {
	if p.defaultStrategy == nil {
		return nil, fmt.Errorf("no default strategy set for spatial query parser")
	}

	// Use the strategy to create the query
	return p.defaultStrategy.MakeQuery(args.Operation, args.Shape)
}

// SetDefaultField sets the default field for queries.
func (p *SpatialQueryParser) SetDefaultField(field string) {
	p.defaultField = field
}

// GetDefaultField returns the default field for queries.
func (p *SpatialQueryParser) GetDefaultField() string {
	return p.defaultField
}

// SetDefaultStrategy sets the default spatial strategy.
func (p *SpatialQueryParser) SetDefaultStrategy(strategy SpatialStrategy) {
	p.defaultStrategy = strategy
}

// GetDefaultStrategy returns the default spatial strategy.
func (p *SpatialQueryParser) GetDefaultStrategy() SpatialStrategy {
	return p.defaultStrategy
}

// GetSpatialContext returns the spatial context used by this parser.
func (p *SpatialQueryParser) GetSpatialContext() *SpatialContext {
	return p.ctx
}

// IsSpatialQuery checks if a query string appears to be a spatial query.
func (p *SpatialQueryParser) IsSpatialQuery(queryString string) bool {
	queryString = strings.TrimSpace(strings.ToLower(queryString))

	// Check for spatial operations
	operations := []string{
		"intersects(",
		"iswithin(",
		"within(",
		"contains(",
		"disjoint(",
		"equals(",
		"overlaps(",
		"bboxintersects(",
		"bboxwithin(",
	}

	for _, op := range operations {
		if strings.HasPrefix(queryString, op) {
			return true
		}
	}

	// Check for function-style queries
	if strings.HasPrefix(queryString, "geo_") {
		return true
	}

	return false
}

// SpatialQueryParserFactory creates SpatialQueryParser instances.
type SpatialQueryParserFactory struct {
	ctx             *SpatialContext
	defaultField    string
	defaultStrategy SpatialStrategy
}

// NewSpatialQueryParserFactory creates a new factory.
func NewSpatialQueryParserFactory(ctx *SpatialContext, defaultField string, defaultStrategy SpatialStrategy) *SpatialQueryParserFactory {
	return &SpatialQueryParserFactory{
		ctx:             ctx,
		defaultField:    defaultField,
		defaultStrategy: defaultStrategy,
	}
}

// CreateParser creates a new SpatialQueryParser.
func (f *SpatialQueryParserFactory) CreateParser() *SpatialQueryParser {
	return NewSpatialQueryParser(f.ctx, f.defaultField, f.defaultStrategy)
}

// CreateParserWithField creates a new parser with a specific field.
func (f *SpatialQueryParserFactory) CreateParserWithField(field string) *SpatialQueryParser {
	return NewSpatialQueryParser(f.ctx, field, f.defaultStrategy)
}

// CreateParserWithStrategy creates a new parser with a specific strategy.
func (f *SpatialQueryParserFactory) CreateParserWithStrategy(strategy SpatialStrategy) *SpatialQueryParser {
	return NewSpatialQueryParser(f.ctx, f.defaultField, strategy)
}

// SetDefaultField sets the default field for parsers created by this factory.
func (f *SpatialQueryParserFactory) SetDefaultField(field string) {
	f.defaultField = field
}

// SetDefaultStrategy sets the default strategy for parsers created by this factory.
func (f *SpatialQueryParserFactory) SetDefaultStrategy(strategy SpatialStrategy) {
	f.defaultStrategy = strategy
}
