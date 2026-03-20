// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// QueryParserPlugin defines the interface for query parser plugins.
// This allows custom query types to be parsed by extending the query parser.
type QueryParserPlugin interface {
	// GetName returns the name of this plugin.
	GetName() string

	// Parse parses a query string segment and returns a Query if applicable.
	// Returns (nil, nil) if this plugin cannot handle the input.
	Parse(querySegment string) (search.Query, error)

	// CanParse returns true if this plugin can parse the given query segment.
	CanParse(querySegment string) bool
}

// SpatialQueryParserPlugin is a plugin that enables spatial query parsing
// within the standard query parser framework.
//
// This plugin integrates with Lucene's query parser to support spatial queries
// alongside traditional text queries. It recognizes spatial query patterns
// and delegates parsing to the SpatialQueryParser.
//
// Example usage:
//
//	plugin := NewSpatialQueryParserPlugin(ctx, "location", strategy)
//	parser := queryparser.NewQueryParserWithPlugins("defaultField", analyzer, []QueryParserPlugin{plugin})
//	query := parser.Parse("title:hotel AND geo_distance(field:location point:POINT(-1 1) distance:10km)")
type SpatialQueryParserPlugin struct {
	parser        *SpatialQueryParser
	fieldMappings map[string]string // Maps field prefixes to spatial fields
	config        *SpatialPluginConfig
}

// SpatialPluginConfig holds configuration for the spatial query parser plugin.
type SpatialPluginConfig struct {
	// DefaultField is the default spatial field name.
	DefaultField string

	// DefaultStrategy is the default spatial strategy.
	DefaultStrategy SpatialStrategy

	// AllowSpatialInDefaultField allows spatial queries without explicit field.
	AllowSpatialInDefaultField bool

	// SupportedOperations lists which spatial operations are enabled.
	// If empty, all operations are enabled.
	SupportedOperations []SpatialOperation

	// MaxDistanceErrorPct is the maximum allowed distance error percentage.
	MaxDistanceErrorPct float64

	// AutoDetectSpatialFields automatically detects spatial field names.
	AutoDetectSpatialFields bool

	// SpatialFieldPrefixes is a list of prefixes that indicate spatial fields.
	SpatialFieldPrefixes []string
}

// DefaultSpatialPluginConfig returns a default configuration.
func DefaultSpatialPluginConfig() *SpatialPluginConfig {
	return &SpatialPluginConfig{
		DefaultField:               "",
		AllowSpatialInDefaultField: true,
		SupportedOperations:        []SpatialOperation{}, // All operations
		MaxDistanceErrorPct:        0.025,
		AutoDetectSpatialFields:    true,
		SpatialFieldPrefixes:       []string{"geo_", "location", "spatial_"},
	}
}

// NewSpatialQueryParserPlugin creates a new spatial query parser plugin.
//
// Parameters:
//   - ctx: The spatial context for shape operations
//   - defaultField: The default field name for spatial queries
//   - defaultStrategy: The default spatial strategy to use
func NewSpatialQueryParserPlugin(ctx *SpatialContext, defaultField string, defaultStrategy SpatialStrategy) *SpatialQueryParserPlugin {
	return NewSpatialQueryParserPluginWithConfig(ctx, defaultField, defaultStrategy, DefaultSpatialPluginConfig())
}

// NewSpatialQueryParserPluginWithConfig creates a new plugin with custom configuration.
func NewSpatialQueryParserPluginWithConfig(ctx *SpatialContext, defaultField string, defaultStrategy SpatialStrategy, config *SpatialPluginConfig) *SpatialQueryParserPlugin {
	if config == nil {
		config = DefaultSpatialPluginConfig()
	}

	// Only set defaults if not already configured
	if config.DefaultField == "" {
		config.DefaultField = defaultField
	}
	if config.DefaultStrategy == nil {
		config.DefaultStrategy = defaultStrategy
	}

	return &SpatialQueryParserPlugin{
		parser:        NewSpatialQueryParser(ctx, defaultField, defaultStrategy),
		fieldMappings: make(map[string]string),
		config:        config,
	}
}

// GetName returns the name of this plugin.
func (p *SpatialQueryParserPlugin) GetName() string {
	return "spatial"
}

// Parse parses a query string segment and returns a Query if it's a spatial query.
// Returns (nil, nil) if the input is not a spatial query.
func (p *SpatialQueryParserPlugin) Parse(querySegment string) (search.Query, error) {
	if !p.CanParse(querySegment) {
		return nil, nil
	}

	// Check if this is a prefixed spatial query (e.g., "location:Intersects(...)")
	fieldName, spatialQuery := p.extractSpatialQuery(querySegment)
	if fieldName != "" {
		// Create a temporary parser with the specific field
		tempParser := p.parser
		if fieldName != p.config.DefaultField {
			// Clone parser with new field
			tempParser = NewSpatialQueryParser(p.parser.GetSpatialContext(), fieldName, p.config.DefaultStrategy)
		}
		return tempParser.Parse(spatialQuery)
	}

	// Parse as standard spatial query
	return p.parser.Parse(querySegment)
}

// CanParse returns true if this plugin can parse the given query segment.
func (p *SpatialQueryParserPlugin) CanParse(querySegment string) bool {
	querySegment = strings.TrimSpace(querySegment)
	if querySegment == "" {
		return false
	}

	// Check for function-style queries first (before field extraction)
	lowerQuery := strings.ToLower(querySegment)
	if strings.HasPrefix(lowerQuery, "geo_") {
		return true
	}

	// Check for field-prefixed spatial queries (e.g., "location:Intersects(...)")
	if idx := strings.Index(querySegment, ":"); idx > 0 {
		fieldName := querySegment[:idx]
		if p.isSpatialField(fieldName) {
			remainder := querySegment[idx+1:]
			return p.parser.IsSpatialQuery(remainder)
		}
	}

	// Check for direct spatial queries
	if p.parser.IsSpatialQuery(querySegment) {
		return true
	}

	return false
}

// extractSpatialQuery extracts the field name and spatial query from a prefixed query.
// Returns ("", "") if not a prefixed spatial query.
func (p *SpatialQueryParserPlugin) extractSpatialQuery(querySegment string) (string, string) {
	idx := strings.Index(querySegment, ":")
	if idx <= 0 {
		return "", ""
	}

	fieldName := strings.TrimSpace(querySegment[:idx])
	remainder := strings.TrimSpace(querySegment[idx+1:])

	if !p.isSpatialField(fieldName) {
		return "", ""
	}

	if !p.parser.IsSpatialQuery(remainder) {
		return "", ""
	}

	return fieldName, remainder
}

// isSpatialField checks if a field name is a spatial field.
func (p *SpatialQueryParserPlugin) isSpatialField(fieldName string) bool {
	// Check explicit mappings
	if _, ok := p.fieldMappings[fieldName]; ok {
		return true
	}

	// Check if matches default field
	if fieldName == p.config.DefaultField {
		return true
	}

	// Check prefixes
	if p.config.AutoDetectSpatialFields {
		for _, prefix := range p.config.SpatialFieldPrefixes {
			if strings.HasPrefix(strings.ToLower(fieldName), prefix) {
				return true
			}
		}
	}

	return false
}

// SetDefaultField sets the default spatial field.
func (p *SpatialQueryParserPlugin) SetDefaultField(field string) {
	p.config.DefaultField = field
	p.parser.SetDefaultField(field)
}

// GetDefaultField returns the default spatial field.
func (p *SpatialQueryParserPlugin) GetDefaultField() string {
	return p.config.DefaultField
}

// SetDefaultStrategy sets the default spatial strategy.
func (p *SpatialQueryParserPlugin) SetDefaultStrategy(strategy SpatialStrategy) {
	p.config.DefaultStrategy = strategy
	p.parser.SetDefaultStrategy(strategy)
}

// GetDefaultStrategy returns the default spatial strategy.
func (p *SpatialQueryParserPlugin) GetDefaultStrategy() SpatialStrategy {
	return p.config.DefaultStrategy
}

// AddFieldMapping adds a field name mapping.
// This allows custom field names to be recognized as spatial fields.
func (p *SpatialQueryParserPlugin) AddFieldMapping(fieldName, spatialField string) {
	p.fieldMappings[fieldName] = spatialField
}

// RemoveFieldMapping removes a field name mapping.
func (p *SpatialQueryParserPlugin) RemoveFieldMapping(fieldName string) {
	delete(p.fieldMappings, fieldName)
}

// GetFieldMapping returns the spatial field for a given field name.
func (p *SpatialQueryParserPlugin) GetFieldMapping(fieldName string) (string, bool) {
	val, ok := p.fieldMappings[fieldName]
	return val, ok
}

// SetConfig updates the plugin configuration.
func (p *SpatialQueryParserPlugin) SetConfig(config *SpatialPluginConfig) {
	p.config = config
	if config.DefaultField != "" {
		p.parser.SetDefaultField(config.DefaultField)
	}
	if config.DefaultStrategy != nil {
		p.parser.SetDefaultStrategy(config.DefaultStrategy)
	}
}

// GetConfig returns the current plugin configuration.
func (p *SpatialQueryParserPlugin) GetConfig() *SpatialPluginConfig {
	return p.config
}

// GetSpatialQueryParser returns the underlying spatial query parser.
func (p *SpatialQueryParserPlugin) GetSpatialQueryParser() *SpatialQueryParser {
	return p.parser
}

// IsOperationSupported checks if a spatial operation is supported by this plugin.
func (p *SpatialQueryParserPlugin) IsOperationSupported(op SpatialOperation) bool {
	if len(p.config.SupportedOperations) == 0 {
		return true // All operations supported by default
	}

	for _, supportedOp := range p.config.SupportedOperations {
		if supportedOp == op {
			return true
		}
	}
	return false
}

// SupportedOperationNames returns the names of supported operations.
func (p *SpatialQueryParserPlugin) SupportedOperationNames() []string {
	if len(p.config.SupportedOperations) == 0 {
		// Return all operation names
		return []string{
			"Intersects", "IsWithin", "Within", "Contains",
			"Disjoint", "Equals", "Overlaps",
			"BboxIntersects", "BboxWithin",
		}
	}

	names := make([]string, len(p.config.SupportedOperations))
	for i, op := range p.config.SupportedOperations {
		names[i] = op.String()
	}
	return names
}

// String returns a string representation of this plugin.
func (p *SpatialQueryParserPlugin) String() string {
	return fmt.Sprintf("SpatialQueryParserPlugin{field=%s, strategy=%T}",
		p.config.DefaultField, p.config.DefaultStrategy)
}

// Ensure SpatialQueryParserPlugin implements QueryParserPlugin
var _ QueryParserPlugin = (*SpatialQueryParserPlugin)(nil)

// SpatialPluginRegistry manages multiple spatial query parser plugins.
// This is useful when dealing with multiple spatial fields and strategies.
type SpatialPluginRegistry struct {
	plugins map[string]*SpatialQueryParserPlugin
}

// NewSpatialPluginRegistry creates a new plugin registry.
func NewSpatialPluginRegistry() *SpatialPluginRegistry {
	return &SpatialPluginRegistry{
		plugins: make(map[string]*SpatialQueryParserPlugin),
	}
}

// Register registers a spatial query parser plugin.
func (r *SpatialPluginRegistry) Register(plugin *SpatialQueryParserPlugin) {
	r.plugins[plugin.GetDefaultField()] = plugin
}

// Unregister removes a plugin for a given field.
func (r *SpatialPluginRegistry) Unregister(fieldName string) {
	delete(r.plugins, fieldName)
}

// Get retrieves a plugin for a given field.
func (r *SpatialPluginRegistry) Get(fieldName string) (*SpatialQueryParserPlugin, bool) {
	plugin, ok := r.plugins[fieldName]
	return plugin, ok
}

// GetAll returns all registered plugins.
func (r *SpatialPluginRegistry) GetAll() []*SpatialQueryParserPlugin {
	result := make([]*SpatialQueryParserPlugin, 0, len(r.plugins))
	for _, plugin := range r.plugins {
		result = append(result, plugin)
	}
	return result
}

// FindPluginForQuery finds the appropriate plugin for a query segment.
func (r *SpatialPluginRegistry) FindPluginForQuery(querySegment string) (*SpatialQueryParserPlugin, bool) {
	for _, plugin := range r.plugins {
		if plugin.CanParse(querySegment) {
			return plugin, true
		}
	}
	return nil, false
}

// ParseSpatialOperation parses a string into a SpatialOperation.
func ParseSpatialOperation(s string) (SpatialOperation, error) {
	op, ok := GetSpatialOperationFromString(s)
	if !ok {
		return SpatialOperationIntersects, fmt.Errorf("unknown spatial operation: %s", s)
	}
	return op, nil
}
