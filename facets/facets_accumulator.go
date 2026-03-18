// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

// FacetsAccumulator is the interface for accumulating facet counts.
// Different implementations can provide different accumulation strategies.
//
// This is the Go port of Lucene's org.apache.lucene.facet.FacetsAccumulator.
type FacetsAccumulator interface {
	// Accumulate accumulates facet counts from the given facet results.
	// Parameters:
	//   - results: the facet results to accumulate
	// Returns:
	//   - error if accumulation fails
	Accumulate(results []*FacetResult) error

	// GetTopChildren returns the top N children for the specified dimension.
	// Parameters:
	//   - topN: maximum number of children to return
	//   - dim: the dimension/facet field name
	//   - path: optional path for hierarchical facets
	// Returns:
	//   - FacetResult containing the top children, or error if dimension not found
	GetTopChildren(topN int, dim string, path ...string) (*FacetResult, error)

	// GetAllChildren returns all children for the specified dimension.
	// Parameters:
	//   - dim: the dimension/facet field name
	//   - path: optional path for hierarchical facets
	// Returns:
	//   - FacetResult containing all children, or error if dimension not found
	GetAllChildren(dim string, path ...string) (*FacetResult, error)

	// GetSpecificValue returns the value for a specific label in a dimension.
	// Parameters:
	//   - dim: the dimension/facet field name
	//   - path: the path components for the specific label
	// Returns:
	//   - FacetResult containing the value, or error if not found
	GetSpecificValue(dim string, path ...string) (*FacetResult, error)

	// GetDimensions returns all dimensions that have been accumulated.
	// Returns:
	//   - slice of dimension names
	GetDimensions() []string

	// Reset resets the accumulator to its initial state.
	Reset()

	// IsEmpty returns true if no facets have been accumulated.
	IsEmpty() bool
}

// BaseFacetsAccumulator provides common functionality for FacetsAccumulator implementations.
type BaseFacetsAccumulator struct {
	// accumulated holds the accumulated facet results by dimension
	accumulated map[string]*FacetResult
}

// NewBaseFacetsAccumulator creates a new BaseFacetsAccumulator.
func NewBaseFacetsAccumulator() *BaseFacetsAccumulator {
	return &BaseFacetsAccumulator{
		accumulated: make(map[string]*FacetResult),
	}
}

// Accumulate accumulates facet results.
func (bfa *BaseFacetsAccumulator) Accumulate(results []*FacetResult) error {
	for _, result := range results {
		if result == nil {
			continue
		}
		bfa.accumulated[result.Dim] = result
	}
	return nil
}

// GetTopChildren returns the top N children for the specified dimension.
func (bfa *BaseFacetsAccumulator) GetTopChildren(topN int, dim string, path ...string) (*FacetResult, error) {
	result, ok := bfa.accumulated[dim]
	if !ok {
		return nil, nil
	}

	// Filter by path if provided
	if len(path) > 0 {
		result = bfa.filterByPath(result, path)
	}

	// Limit to topN
	if result != nil && len(result.LabelValues) > topN {
		result.LabelValues = result.LabelValues[:topN]
	}

	return result, nil
}

// GetAllChildren returns all children for the specified dimension.
func (bfa *BaseFacetsAccumulator) GetAllChildren(dim string, path ...string) (*FacetResult, error) {
	result, ok := bfa.accumulated[dim]
	if !ok {
		return nil, nil
	}

	// Filter by path if provided
	if len(path) > 0 {
		result = bfa.filterByPath(result, path)
	}

	return result, nil
}

// GetSpecificValue returns the value for a specific label.
func (bfa *BaseFacetsAccumulator) GetSpecificValue(dim string, path ...string) (*FacetResult, error) {
	result, ok := bfa.accumulated[dim]
	if !ok {
		return nil, nil
	}

	// Find the specific value
	for _, lv := range result.LabelValues {
		if bfa.matchesPath(lv.Label, path) {
			return &FacetResult{
				Dim:         dim,
				Path:        path,
				Value:       lv.Value,
				LabelValues: []*LabelAndValue{lv},
			}, nil
		}
	}

	return nil, nil
}

// GetDimensions returns all accumulated dimensions.
func (bfa *BaseFacetsAccumulator) GetDimensions() []string {
	dims := make([]string, 0, len(bfa.accumulated))
	for dim := range bfa.accumulated {
		dims = append(dims, dim)
	}
	return dims
}

// Reset resets the accumulator.
func (bfa *BaseFacetsAccumulator) Reset() {
	bfa.accumulated = make(map[string]*FacetResult)
}

// IsEmpty returns true if no facets have been accumulated.
func (bfa *BaseFacetsAccumulator) IsEmpty() bool {
	return len(bfa.accumulated) == 0
}

// filterByPath filters a result by path.
func (bfa *BaseFacetsAccumulator) filterByPath(result *FacetResult, path []string) *FacetResult {
	filtered := &FacetResult{
		Dim:         result.Dim,
		Path:        path,
		LabelValues: make([]*LabelAndValue, 0),
	}

	for _, lv := range result.LabelValues {
		if bfa.matchesPath(lv.Label, path) {
			filtered.LabelValues = append(filtered.LabelValues, lv)
		}
	}

	return filtered
}

// matchesPath checks if a label matches the given path.
func (bfa *BaseFacetsAccumulator) matchesPath(label string, path []string) bool {
	// Simple string comparison for now
	// In a full implementation, this would properly compare path components
	if len(path) == 0 {
		return true
	}
	if len(path) == 1 {
		return label == path[0]
	}
	return false
}

// Ensure BaseFacetsAccumulator implements FacetsAccumulator
var _ FacetsAccumulator = (*BaseFacetsAccumulator)(nil)

// FacetsAccumulatorConfig contains configuration for FacetsAccumulator implementations.
type FacetsAccumulatorConfig struct {
	// MaxCategories is the maximum number of categories to accumulate per dimension
	MaxCategories int

	// Hierarchical indicates if hierarchical facets are supported
	Hierarchical bool

	// IncludeZeroCounts indicates if categories with zero counts should be included
	IncludeZeroCounts bool
}

// NewDefaultFacetsAccumulatorConfig creates a default configuration.
func NewDefaultFacetsAccumulatorConfig() *FacetsAccumulatorConfig {
	return &FacetsAccumulatorConfig{
		MaxCategories:     1000,
		Hierarchical:      true,
		IncludeZeroCounts: false,
	}
}

// FacetsAccumulatorFactory creates FacetsAccumulator instances.
type FacetsAccumulatorFactory interface {
	// CreateAccumulator creates a new FacetsAccumulator.
	CreateAccumulator(config *FacetsAccumulatorConfig) (FacetsAccumulator, error)
}

// BaseFacetsAccumulatorFactory is a base implementation of FacetsAccumulatorFactory.
type BaseFacetsAccumulatorFactory struct{}

// NewBaseFacetsAccumulatorFactory creates a new BaseFacetsAccumulatorFactory.
func NewBaseFacetsAccumulatorFactory() *BaseFacetsAccumulatorFactory {
	return &BaseFacetsAccumulatorFactory{}
}

// CreateAccumulator creates a new BaseFacetsAccumulator.
func (bfaf *BaseFacetsAccumulatorFactory) CreateAccumulator(config *FacetsAccumulatorConfig) (FacetsAccumulator, error) {
	return NewBaseFacetsAccumulator(), nil
}

// Ensure BaseFacetsAccumulatorFactory implements FacetsAccumulatorFactory
var _ FacetsAccumulatorFactory = (*BaseFacetsAccumulatorFactory)(nil)
