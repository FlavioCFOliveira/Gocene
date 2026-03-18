package facets

import (
	"fmt"
	"sort"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
)

// FacetsConfig manages the configuration for faceted fields.
// It determines how facet fields are indexed and what type of faceting
// is supported for each field.
type FacetsConfig struct {
	// dimConfigs maps dimension names to their configuration
	dimConfigs map[string]*DimConfig

	// indexFieldName is the default index field name for facets
	indexFieldName string

	// drillDownFieldName is the field name used for drill-down queries
	drillDownFieldName string

	// validateFields indicates whether to validate facet fields during build
	validateFields bool

	// autoDetectHierarchical indicates whether to auto-detect hierarchical facets
	autoDetectHierarchical bool

	// defaultMultiValued is the default value for multi-valued fields
	defaultMultiValued bool

	// defaultHierarchical is the default value for hierarchical fields
	defaultHierarchical bool

	// defaultRequireDimCount is the default value for require dim count
	defaultRequireDimCount bool
}

// DimConfig holds the configuration for a single facet dimension.
type DimConfig struct {
	// Dim is the dimension/facet field name
	Dim string

	// IndexFieldName is the actual field name used for indexing
	IndexFieldName string

	// MultiValued indicates if this facet field can have multiple values per document
	MultiValued bool

	// RequireDimCount indicates if the count for this dimension is required
	RequireDimCount bool

	// Hierarchical indicates if this is a hierarchical facet
	Hierarchical bool
}

// NewFacetsConfig creates a new empty FacetsConfig.
func NewFacetsConfig() *FacetsConfig {
	return &FacetsConfig{
		dimConfigs:             make(map[string]*DimConfig),
		indexFieldName:         "$facets",
		drillDownFieldName:     "$facets.drilldown",
		validateFields:         true,
		autoDetectHierarchical: true,
		defaultMultiValued:     false,
		defaultHierarchical:    false,
		defaultRequireDimCount: false,
	}
}

// SetMultiValued configures whether the specified dimension allows multiple values per document.
// By default, facets are single-valued.
func (fc *FacetsConfig) SetMultiValued(dim string, multiValued bool) {
	config := fc.getOrCreateConfig(dim)
	config.MultiValued = multiValued
}

// SetRequireDimCount configures whether the count for the specified dimension is required.
// When true, the total count for the dimension is computed even when drilling down.
func (fc *FacetsConfig) SetRequireDimCount(dim string, require bool) {
	config := fc.getOrCreateConfig(dim)
	config.RequireDimCount = require
}

// SetHierarchical configures whether the specified dimension is hierarchical.
// Hierarchical facets support paths like "/electronics/phones".
func (fc *FacetsConfig) SetHierarchical(dim string, hierarchical bool) {
	config := fc.getOrCreateConfig(dim)
	config.Hierarchical = hierarchical
}

// SetIndexFieldName sets a custom index field name for the dimension.
// By default, the dimension name is used as the index field name.
func (fc *FacetsConfig) SetIndexFieldName(dim string, indexFieldName string) {
	config := fc.getOrCreateConfig(dim)
	config.IndexFieldName = indexFieldName
}

// GetDimConfig returns the configuration for the specified dimension.
// Returns nil if no configuration exists for the dimension.
func (fc *FacetsConfig) GetDimConfig(dim string) *DimConfig {
	return fc.dimConfigs[dim]
}

// GetIndexFieldName returns the index field name for the given dimension.
// If no custom name is set, returns the dimension name.
func (fc *FacetsConfig) GetIndexFieldName(dim string) string {
	config := fc.dimConfigs[dim]
	if config != nil && config.IndexFieldName != "" {
		return config.IndexFieldName
	}
	return dim
}

// IsMultiValued returns true if the dimension is configured as multi-valued.
func (fc *FacetsConfig) IsMultiValued(dim string) bool {
	config := fc.dimConfigs[dim]
	if config != nil {
		return config.MultiValued
	}
	return false
}

// IsHierarchical returns true if the dimension is configured as hierarchical.
func (fc *FacetsConfig) IsHierarchical(dim string) bool {
	config := fc.dimConfigs[dim]
	if config != nil {
		return config.Hierarchical
	}
	return false
}

// IsRequireDimCount returns true if the dimension requires dimension count.
func (fc *FacetsConfig) IsRequireDimCount(dim string) bool {
	config := fc.dimConfigs[dim]
	if config != nil {
		return config.RequireDimCount
	}
	return false
}

// getOrCreateConfig returns the existing config for dim or creates a new one.
func (fc *FacetsConfig) getOrCreateConfig(dim string) *DimConfig {
	config, exists := fc.dimConfigs[dim]
	if !exists {
		config = &DimConfig{
			Dim:            dim,
			IndexFieldName: dim,
		}
		fc.dimConfigs[dim] = config
	}
	return config
}

// Build builds the facet fields for the document using this configuration.
// This should be called before indexing to set up the proper facet indexing.
func (fc *FacetsConfig) Build(doc *document.Document) error {
	// This is a placeholder for the build process
	// The actual implementation will process FacetField instances in the document
	// and configure them according to the dimension settings
	return nil
}

// GetDims returns all configured dimension names.
func (fc *FacetsConfig) GetDims() []string {
	dims := make([]string, 0, len(fc.dimConfigs))
	for dim := range fc.dimConfigs {
		dims = append(dims, dim)
	}
	sort.Strings(dims)
	return dims
}

// SetDefaultIndexFieldName sets the default index field name for facets.
func (fc *FacetsConfig) SetDefaultIndexFieldName(name string) {
	fc.indexFieldName = name
}

// GetDefaultIndexFieldName returns the default index field name for facets.
func (fc *FacetsConfig) GetDefaultIndexFieldName() string {
	return fc.indexFieldName
}

// SetDrillDownFieldName sets the field name used for drill-down queries.
func (fc *FacetsConfig) SetDrillDownFieldName(name string) {
	fc.drillDownFieldName = name
}

// GetDrillDownFieldName returns the field name used for drill-down queries.
func (fc *FacetsConfig) GetDrillDownFieldName() string {
	return fc.drillDownFieldName
}

// SetValidateFields sets whether to validate facet fields during build.
func (fc *FacetsConfig) SetValidateFields(validate bool) {
	fc.validateFields = validate
}

// IsValidateFields returns true if facet fields should be validated during build.
func (fc *FacetsConfig) IsValidateFields() bool {
	return fc.validateFields
}

// SetAutoDetectHierarchical sets whether to auto-detect hierarchical facets.
func (fc *FacetsConfig) SetAutoDetectHierarchical(autoDetect bool) {
	fc.autoDetectHierarchical = autoDetect
}

// IsAutoDetectHierarchical returns true if hierarchical facets should be auto-detected.
func (fc *FacetsConfig) IsAutoDetectHierarchical() bool {
	return fc.autoDetectHierarchical
}

// SetDefaultMultiValued sets the default value for multi-valued fields.
func (fc *FacetsConfig) SetDefaultMultiValued(multiValued bool) {
	fc.defaultMultiValued = multiValued
}

// IsDefaultMultiValued returns the default value for multi-valued fields.
func (fc *FacetsConfig) IsDefaultMultiValued() bool {
	return fc.defaultMultiValued
}

// SetDefaultHierarchical sets the default value for hierarchical fields.
func (fc *FacetsConfig) SetDefaultHierarchical(hierarchical bool) {
	fc.defaultHierarchical = hierarchical
}

// IsDefaultHierarchical returns the default value for hierarchical fields.
func (fc *FacetsConfig) IsDefaultHierarchical() bool {
	return fc.defaultHierarchical
}

// SetDefaultRequireDimCount sets the default value for require dim count.
func (fc *FacetsConfig) SetDefaultRequireDimCount(require bool) {
	fc.defaultRequireDimCount = require
}

// IsDefaultRequireDimCount returns the default value for require dim count.
func (fc *FacetsConfig) IsDefaultRequireDimCount() bool {
	return fc.defaultRequireDimCount
}

// Validate validates the configuration.
// Returns an error if the configuration is invalid.
func (fc *FacetsConfig) Validate() error {
	// Check for duplicate index field names
	indexFieldNames := make(map[string]string) // maps index field name to dim
	for dim, config := range fc.dimConfigs {
		if config.IndexFieldName == "" {
			continue
		}
		if existingDim, exists := indexFieldNames[config.IndexFieldName]; exists {
			return fmt.Errorf("dimensions %q and %q share the same index field name %q",
				existingDim, dim, config.IndexFieldName)
		}
		indexFieldNames[config.IndexFieldName] = dim
	}

	// Validate each dimension configuration
	for dim, config := range fc.dimConfigs {
		if err := fc.validateDimConfig(dim, config); err != nil {
			return fmt.Errorf("invalid configuration for dimension %q: %w", dim, err)
		}
	}

	return nil
}

// validateDimConfig validates a single dimension configuration.
func (fc *FacetsConfig) validateDimConfig(dim string, config *DimConfig) error {
	if config == nil {
		return fmt.Errorf("configuration is nil")
	}

	if config.Dim != dim {
		return fmt.Errorf("dimension name mismatch: expected %q, got %q", dim, config.Dim)
	}

	// Validate hierarchical configuration
	if config.Hierarchical {
		// Hierarchical facets should have path-like values
		// This is just a validation check, actual validation happens during indexing
	}

	return nil
}

// HasDimension returns true if the configuration has the specified dimension.
func (fc *FacetsConfig) HasDimension(dim string) bool {
	_, exists := fc.dimConfigs[dim]
	return exists
}

// RemoveDimension removes the specified dimension from the configuration.
func (fc *FacetsConfig) RemoveDimension(dim string) bool {
	if _, exists := fc.dimConfigs[dim]; exists {
		delete(fc.dimConfigs, dim)
		return true
	}
	return false
}

// Clear removes all dimension configurations.
func (fc *FacetsConfig) Clear() {
	fc.dimConfigs = make(map[string]*DimConfig)
}

// GetDimensionCount returns the number of configured dimensions.
func (fc *FacetsConfig) GetDimensionCount() int {
	return len(fc.dimConfigs)
}

// IsEmpty returns true if no dimensions are configured.
func (fc *FacetsConfig) IsEmpty() bool {
	return len(fc.dimConfigs) == 0
}

// Clone creates a deep copy of the configuration.
func (fc *FacetsConfig) Clone() *FacetsConfig {
	clone := NewFacetsConfig()
	clone.indexFieldName = fc.indexFieldName
	clone.drillDownFieldName = fc.drillDownFieldName
	clone.validateFields = fc.validateFields
	clone.autoDetectHierarchical = fc.autoDetectHierarchical
	clone.defaultMultiValued = fc.defaultMultiValued
	clone.defaultHierarchical = fc.defaultHierarchical
	clone.defaultRequireDimCount = fc.defaultRequireDimCount

	for dim, config := range fc.dimConfigs {
		clone.dimConfigs[dim] = &DimConfig{
			Dim:             config.Dim,
			IndexFieldName:  config.IndexFieldName,
			MultiValued:     config.MultiValued,
			RequireDimCount: config.RequireDimCount,
			Hierarchical:    config.Hierarchical,
		}
	}

	return clone
}

// Merge merges another FacetsConfig into this one.
// Dimensions from the other config take precedence if there are conflicts.
func (fc *FacetsConfig) Merge(other *FacetsConfig) error {
	if other == nil {
		return fmt.Errorf("cannot merge nil configuration")
	}

	for dim, config := range other.dimConfigs {
		fc.dimConfigs[dim] = &DimConfig{
			Dim:             config.Dim,
			IndexFieldName:  config.IndexFieldName,
			MultiValued:     config.MultiValued,
			RequireDimCount: config.RequireDimCount,
			Hierarchical:    config.Hierarchical,
		}
	}

	return nil
}

// GetAllDimConfigs returns all dimension configurations.
func (fc *FacetsConfig) GetAllDimConfigs() map[string]*DimConfig {
	result := make(map[string]*DimConfig)
	for dim, config := range fc.dimConfigs {
		result[dim] = &DimConfig{
			Dim:             config.Dim,
			IndexFieldName:  config.IndexFieldName,
			MultiValued:     config.MultiValued,
			RequireDimCount: config.RequireDimCount,
			Hierarchical:    config.Hierarchical,
		}
	}
	return result
}

// GetHierarchicalDims returns all dimensions configured as hierarchical.
func (fc *FacetsConfig) GetHierarchicalDims() []string {
	var dims []string
	for dim, config := range fc.dimConfigs {
		if config.Hierarchical {
			dims = append(dims, dim)
		}
	}
	sort.Strings(dims)
	return dims
}

// GetMultiValuedDims returns all dimensions configured as multi-valued.
func (fc *FacetsConfig) GetMultiValuedDims() []string {
	var dims []string
	for dim, config := range fc.dimConfigs {
		if config.MultiValued {
			dims = append(dims, dim)
		}
	}
	sort.Strings(dims)
	return dims
}

// String returns a string representation of the configuration.
func (fc *FacetsConfig) String() string {
	var parts []string
	parts = append(parts, fmt.Sprintf("FacetsConfig{"))
	parts = append(parts, fmt.Sprintf("  indexFieldName: %q", fc.indexFieldName))
	parts = append(parts, fmt.Sprintf("  drillDownFieldName: %q", fc.drillDownFieldName))
	parts = append(parts, fmt.Sprintf("  validateFields: %v", fc.validateFields))
	parts = append(parts, fmt.Sprintf("  autoDetectHierarchical: %v", fc.autoDetectHierarchical))
	parts = append(parts, fmt.Sprintf("  defaultMultiValued: %v", fc.defaultMultiValued))
	parts = append(parts, fmt.Sprintf("  defaultHierarchical: %v", fc.defaultHierarchical))
	parts = append(parts, fmt.Sprintf("  defaultRequireDimCount: %v", fc.defaultRequireDimCount))
	parts = append(parts, fmt.Sprintf("  dimensions: [%d]", len(fc.dimConfigs)))

	dims := fc.GetDims()
	for _, dim := range dims {
		config := fc.dimConfigs[dim]
		parts = append(parts, fmt.Sprintf("    %s: {multiValued=%v, hierarchical=%v, requireDimCount=%v, indexFieldName=%q}",
			dim, config.MultiValued, config.Hierarchical, config.RequireDimCount, config.IndexFieldName))
	}
	parts = append(parts, "}")

	return strings.Join(parts, "\n")
}

// FacetsConfigField is a field that can be added to documents to configure
// how facets are built for that document.
type FacetsConfigField struct {
	*document.StoredField
	config *FacetsConfig
}

// NewFacetsConfigField creates a new FacetsConfigField with the given configuration.
func NewFacetsConfigField(config *FacetsConfig) *FacetsConfigField {
	field := &FacetsConfigField{
		config: config,
	}
	// Note: StoredField initialization would be done here
	// but we're keeping it minimal for the infrastructure
	return field
}

// GetConfig returns the FacetsConfig associated with this field.
func (fcf *FacetsConfigField) GetConfig() *FacetsConfig {
	return fcf.config
}

// FacetIndexingParams holds parameters for facet indexing operations.
type FacetIndexingParams struct {
	// FieldInfo contains information about the facet field
	FieldInfo *index.FieldInfo

	// DimConfig contains the dimension configuration
	DimConfig *DimConfig
}

// NewFacetIndexingParams creates new indexing params for the given field and config.
func NewFacetIndexingParams(fieldInfo *index.FieldInfo, dimConfig *DimConfig) *FacetIndexingParams {
	return &FacetIndexingParams{
		FieldInfo: fieldInfo,
		DimConfig: dimConfig,
	}
}
