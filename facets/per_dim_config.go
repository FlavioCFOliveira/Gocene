// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import (
	"fmt"
	"sort"
	"strings"
)

// PerDimConfig provides per-dimension configuration for facets.
// This allows fine-grained control over how each dimension is indexed and queried.
//
// This is the Go port of Lucene's per-dimension configuration support,
// allowing different settings for each facet dimension.
type PerDimConfig struct {
	// dim is the dimension name
	dim string

	// indexFieldName is the field name used for indexing this dimension
	indexFieldName string

	// multiValued indicates if this dimension can have multiple values per document
	multiValued bool

	// hierarchical indicates if this dimension supports hierarchical paths
	hierarchical bool

	// requireDimCount indicates if the count for this dimension is required
	requireDimCount bool

	// drillDownTerms is the number of drill-down terms to generate
	drillDownTerms int

	// indexFieldNamePrefix is the prefix for the index field name
	indexFieldNamePrefix string

	// customProperties holds custom properties for this dimension
	customProperties map[string]string
}

// NewPerDimConfig creates a new PerDimConfig for the given dimension.
//
// Parameters:
//   - dim: the dimension name
//
// Returns:
//   - a new PerDimConfig instance with default values
func NewPerDimConfig(dim string) *PerDimConfig {
	return &PerDimConfig{
		dim:                  dim,
		indexFieldName:       dim,
		multiValued:          false,
		hierarchical:         false,
		requireDimCount:      false,
		drillDownTerms:       1,
		indexFieldNamePrefix: "",
		customProperties:     make(map[string]string),
	}
}

// GetDim returns the dimension name.
func (pdc *PerDimConfig) GetDim() string {
	return pdc.dim
}

// SetIndexFieldName sets the index field name for this dimension.
func (pdc *PerDimConfig) SetIndexFieldName(name string) *PerDimConfig {
	pdc.indexFieldName = name
	return pdc
}

// GetIndexFieldName returns the index field name for this dimension.
func (pdc *PerDimConfig) GetIndexFieldName() string {
	if pdc.indexFieldNamePrefix != "" {
		return pdc.indexFieldNamePrefix + pdc.indexFieldName
	}
	return pdc.indexFieldName
}

// SetMultiValued sets whether this dimension allows multiple values per document.
func (pdc *PerDimConfig) SetMultiValued(multiValued bool) *PerDimConfig {
	pdc.multiValued = multiValued
	return pdc
}

// IsMultiValued returns true if this dimension allows multiple values per document.
func (pdc *PerDimConfig) IsMultiValued() bool {
	return pdc.multiValued
}

// SetHierarchical sets whether this dimension supports hierarchical paths.
func (pdc *PerDimConfig) SetHierarchical(hierarchical bool) *PerDimConfig {
	pdc.hierarchical = hierarchical
	return pdc
}

// IsHierarchical returns true if this dimension supports hierarchical paths.
func (pdc *PerDimConfig) IsHierarchical() bool {
	return pdc.hierarchical
}

// SetRequireDimCount sets whether the count for this dimension is required.
func (pdc *PerDimConfig) SetRequireDimCount(require bool) *PerDimConfig {
	pdc.requireDimCount = require
	return pdc
}

// IsRequireDimCount returns true if the count for this dimension is required.
func (pdc *PerDimConfig) IsRequireDimCount() bool {
	return pdc.requireDimCount
}

// SetDrillDownTerms sets the number of drill-down terms to generate.
func (pdc *PerDimConfig) SetDrillDownTerms(terms int) *PerDimConfig {
	if terms > 0 {
		pdc.drillDownTerms = terms
	}
	return pdc
}

// GetDrillDownTerms returns the number of drill-down terms to generate.
func (pdc *PerDimConfig) GetDrillDownTerms() int {
	return pdc.drillDownTerms
}

// SetIndexFieldNamePrefix sets the prefix for the index field name.
func (pdc *PerDimConfig) SetIndexFieldNamePrefix(prefix string) *PerDimConfig {
	pdc.indexFieldNamePrefix = prefix
	return pdc
}

// GetIndexFieldNamePrefix returns the prefix for the index field name.
func (pdc *PerDimConfig) GetIndexFieldNamePrefix() string {
	return pdc.indexFieldNamePrefix
}

// SetCustomProperty sets a custom property for this dimension.
func (pdc *PerDimConfig) SetCustomProperty(key, value string) *PerDimConfig {
	pdc.customProperties[key] = value
	return pdc
}

// GetCustomProperty returns a custom property for this dimension.
func (pdc *PerDimConfig) GetCustomProperty(key string) string {
	return pdc.customProperties[key]
}

// HasCustomProperty returns true if the custom property exists.
func (pdc *PerDimConfig) HasCustomProperty(key string) bool {
	_, exists := pdc.customProperties[key]
	return exists
}

// RemoveCustomProperty removes a custom property.
func (pdc *PerDimConfig) RemoveCustomProperty(key string) *PerDimConfig {
	delete(pdc.customProperties, key)
	return pdc
}

// GetCustomPropertyKeys returns all custom property keys.
func (pdc *PerDimConfig) GetCustomPropertyKeys() []string {
	keys := make([]string, 0, len(pdc.customProperties))
	for key := range pdc.customProperties {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// Clone creates a deep copy of this configuration.
func (pdc *PerDimConfig) Clone() *PerDimConfig {
	clone := NewPerDimConfig(pdc.dim)
	clone.indexFieldName = pdc.indexFieldName
	clone.multiValued = pdc.multiValued
	clone.hierarchical = pdc.hierarchical
	clone.requireDimCount = pdc.requireDimCount
	clone.drillDownTerms = pdc.drillDownTerms
	clone.indexFieldNamePrefix = pdc.indexFieldNamePrefix

	for key, value := range pdc.customProperties {
		clone.customProperties[key] = value
	}

	return clone
}

// Validate validates this configuration.
// Returns an error if the configuration is invalid.
func (pdc *PerDimConfig) Validate() error {
	if pdc.dim == "" {
		return fmt.Errorf("dimension name cannot be empty")
	}

	if pdc.indexFieldName == "" {
		return fmt.Errorf("index field name cannot be empty")
	}

	if pdc.drillDownTerms < 1 {
		return fmt.Errorf("drill down terms must be at least 1, got %d", pdc.drillDownTerms)
	}

	return nil
}

// Equals returns true if this configuration equals another.
func (pdc *PerDimConfig) Equals(other *PerDimConfig) bool {
	if other == nil {
		return false
	}

	if pdc.dim != other.dim {
		return false
	}

	if pdc.indexFieldName != other.indexFieldName {
		return false
	}

	if pdc.multiValued != other.multiValued {
		return false
	}

	if pdc.hierarchical != other.hierarchical {
		return false
	}

	if pdc.requireDimCount != other.requireDimCount {
		return false
	}

	if pdc.drillDownTerms != other.drillDownTerms {
		return false
	}

	if pdc.indexFieldNamePrefix != other.indexFieldNamePrefix {
		return false
	}

	if len(pdc.customProperties) != len(other.customProperties) {
		return false
	}

	for key, value := range pdc.customProperties {
		if other.customProperties[key] != value {
			return false
		}
	}

	return true
}

// String returns a string representation of this configuration.
func (pdc *PerDimConfig) String() string {
	var parts []string
	parts = append(parts, fmt.Sprintf("PerDimConfig{"))
	parts = append(parts, fmt.Sprintf("  dim: %q", pdc.dim))
	parts = append(parts, fmt.Sprintf("  indexFieldName: %q", pdc.GetIndexFieldName()))
	parts = append(parts, fmt.Sprintf("  multiValued: %v", pdc.multiValued))
	parts = append(parts, fmt.Sprintf("  hierarchical: %v", pdc.hierarchical))
	parts = append(parts, fmt.Sprintf("  requireDimCount: %v", pdc.requireDimCount))
	parts = append(parts, fmt.Sprintf("  drillDownTerms: %d", pdc.drillDownTerms))

	if pdc.indexFieldNamePrefix != "" {
		parts = append(parts, fmt.Sprintf("  indexFieldNamePrefix: %q", pdc.indexFieldNamePrefix))
	}

	if len(pdc.customProperties) > 0 {
		parts = append(parts, fmt.Sprintf("  customProperties: %d", len(pdc.customProperties)))
	}

	parts = append(parts, "}")

	return strings.Join(parts, "\n")
}

// PerDimConfigBuilder helps build PerDimConfig instances.
type PerDimConfigBuilder struct {
	config *PerDimConfig
}

// NewPerDimConfigBuilder creates a new builder for the given dimension.
func NewPerDimConfigBuilder(dim string) *PerDimConfigBuilder {
	return &PerDimConfigBuilder{
		config: NewPerDimConfig(dim),
	}
}

// SetIndexFieldName sets the index field name.
func (b *PerDimConfigBuilder) SetIndexFieldName(name string) *PerDimConfigBuilder {
	b.config.SetIndexFieldName(name)
	return b
}

// SetMultiValued sets whether this dimension allows multiple values.
func (b *PerDimConfigBuilder) SetMultiValued(multiValued bool) *PerDimConfigBuilder {
	b.config.SetMultiValued(multiValued)
	return b
}

// SetHierarchical sets whether this dimension supports hierarchical paths.
func (b *PerDimConfigBuilder) SetHierarchical(hierarchical bool) *PerDimConfigBuilder {
	b.config.SetHierarchical(hierarchical)
	return b
}

// SetRequireDimCount sets whether the count for this dimension is required.
func (b *PerDimConfigBuilder) SetRequireDimCount(require bool) *PerDimConfigBuilder {
	b.config.SetRequireDimCount(require)
	return b
}

// SetDrillDownTerms sets the number of drill-down terms.
func (b *PerDimConfigBuilder) SetDrillDownTerms(terms int) *PerDimConfigBuilder {
	b.config.SetDrillDownTerms(terms)
	return b
}

// SetIndexFieldNamePrefix sets the prefix for the index field name.
func (b *PerDimConfigBuilder) SetIndexFieldNamePrefix(prefix string) *PerDimConfigBuilder {
	b.config.SetIndexFieldNamePrefix(prefix)
	return b
}

// SetCustomProperty sets a custom property.
func (b *PerDimConfigBuilder) SetCustomProperty(key, value string) *PerDimConfigBuilder {
	b.config.SetCustomProperty(key, value)
	return b
}

// Build builds and returns the PerDimConfig.
func (b *PerDimConfigBuilder) Build() (*PerDimConfig, error) {
	if err := b.config.Validate(); err != nil {
		return nil, err
	}
	return b.config.Clone(), nil
}

// PerDimConfigRegistry manages per-dimension configurations for multiple dimensions.
type PerDimConfigRegistry struct {
	// configs maps dimension names to their configurations
	configs map[string]*PerDimConfig
}

// NewPerDimConfigRegistry creates a new registry.
func NewPerDimConfigRegistry() *PerDimConfigRegistry {
	return &PerDimConfigRegistry{
		configs: make(map[string]*PerDimConfig),
	}
}

// Register registers a per-dimension configuration.
func (r *PerDimConfigRegistry) Register(config *PerDimConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	r.configs[config.GetDim()] = config.Clone()
	return nil
}

// Get returns the configuration for the given dimension.
func (r *PerDimConfigRegistry) Get(dim string) *PerDimConfig {
	if config, exists := r.configs[dim]; exists {
		return config.Clone()
	}
	return nil
}

// Has returns true if a configuration exists for the given dimension.
func (r *PerDimConfigRegistry) Has(dim string) bool {
	_, exists := r.configs[dim]
	return exists
}

// Remove removes the configuration for the given dimension.
func (r *PerDimConfigRegistry) Remove(dim string) bool {
	if _, exists := r.configs[dim]; exists {
		delete(r.configs, dim)
		return true
	}
	return false
}

// GetAll returns all registered configurations.
func (r *PerDimConfigRegistry) GetAll() map[string]*PerDimConfig {
	result := make(map[string]*PerDimConfig)
	for dim, config := range r.configs {
		result[dim] = config.Clone()
	}
	return result
}

// GetDims returns all registered dimension names.
func (r *PerDimConfigRegistry) GetDims() []string {
	dims := make([]string, 0, len(r.configs))
	for dim := range r.configs {
		dims = append(dims, dim)
	}
	sort.Strings(dims)
	return dims
}

// Clear removes all configurations.
func (r *PerDimConfigRegistry) Clear() {
	r.configs = make(map[string]*PerDimConfig)
}

// Count returns the number of registered configurations.
func (r *PerDimConfigRegistry) Count() int {
	return len(r.configs)
}

// IsEmpty returns true if no configurations are registered.
func (r *PerDimConfigRegistry) IsEmpty() bool {
	return len(r.configs) == 0
}

// Merge merges another registry into this one.
// Configurations from the other registry take precedence.
func (r *PerDimConfigRegistry) Merge(other *PerDimConfigRegistry) error {
	if other == nil {
		return fmt.Errorf("cannot merge nil registry")
	}

	for dim, config := range other.configs {
		r.configs[dim] = config.Clone()
	}

	return nil
}

// Clone creates a deep copy of this registry.
func (r *PerDimConfigRegistry) Clone() *PerDimConfigRegistry {
	clone := NewPerDimConfigRegistry()
	for dim, config := range r.configs {
		clone.configs[dim] = config.Clone()
	}
	return clone
}
