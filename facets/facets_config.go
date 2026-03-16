package facets

import (
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
)

// FacetsConfig manages the configuration for faceted fields.
// It determines how facet fields are indexed and what type of faceting
// is supported for each field.
type FacetsConfig struct {
	// dimConfigs maps dimension names to their configuration
	dimConfigs map[string]*DimConfig
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
		dimConfigs: make(map[string]*DimConfig),
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
	return dims
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
