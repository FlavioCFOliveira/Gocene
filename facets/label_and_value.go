package facets

import "fmt"

// LabelAndValue represents a single facet entry with its label and count.
// This is the basic unit returned by facet counting operations.
type LabelAndValue struct {
	// Label is the facet value/category (e.g., "electronics", "books")
	Label string

	// Value is the aggregated count or value for this facet entry.
	// For count-based facets, this is the number of documents matching this label.
	Value int64

	// ChildCount is the number of child facets under this label (for hierarchical facets)
	ChildCount int
}

// NewLabelAndValue creates a new LabelAndValue with the given label and value.
func NewLabelAndValue(label string, value int64) *LabelAndValue {
	return &LabelAndValue{
		Label: label,
		Value: value,
	}
}

// String returns a string representation of this LabelAndValue.
func (lv *LabelAndValue) String() string {
	return fmt.Sprintf("%s (%d)", lv.Label, lv.Value)
}

// FacetResult represents the result of a facet counting operation for a single dimension.
// It contains all the facet values and their counts for a specific facet field.
type FacetResult struct {
	// Dim is the dimension/facet field name (e.g., "category", "price_range")
	Dim string

	// Path is the path for hierarchical facets (e.g., ["electronics", "phones"])
	Path []string

	// Value is the total value/count for this facet dimension
	Value int64

	// LabelValues contains all the individual facet entries for this dimension
	LabelValues []*LabelAndValue

	// ChildCount is the number of child facets
	ChildCount int
}

// NewFacetResult creates a new FacetResult for the given dimension.
func NewFacetResult(dim string) *FacetResult {
	return &FacetResult{
		Dim:         dim,
		Path:        make([]string, 0),
		LabelValues: make([]*LabelAndValue, 0),
	}
}

// NewFacetResultWithPath creates a new FacetResult with a hierarchical path.
func NewFacetResultWithPath(dim string, path []string) *FacetResult {
	fr := NewFacetResult(dim)
	fr.Path = append(fr.Path, path...)
	return fr
}

// AddLabelValue adds a LabelAndValue to this FacetResult.
func (fr *FacetResult) AddLabelValue(lv *LabelAndValue) {
	fr.LabelValues = append(fr.LabelValues, lv)
}

// AddLabelValueWithCount adds a LabelAndValue with the given label and count.
func (fr *FacetResult) AddLabelValueWithCount(label string, count int64) *LabelAndValue {
	lv := NewLabelAndValue(label, count)
	fr.AddLabelValue(lv)
	return lv
}

// String returns a string representation of this FacetResult.
func (fr *FacetResult) String() string {
	if len(fr.Path) > 0 {
		return fmt.Sprintf("%s/%v=%d", fr.Dim, fr.Path, fr.Value)
	}
	return fmt.Sprintf("%s=%d", fr.Dim, fr.Value)
}

// FacetResults is a collection of FacetResult for multiple dimensions.
type FacetResults struct {
	// Results contains all facet results, one per dimension
	Results []*FacetResult
}

// NewFacetResults creates a new empty FacetResults collection.
func NewFacetResults() *FacetResults {
	return &FacetResults{
		Results: make([]*FacetResult, 0),
	}
}

// AddResult adds a FacetResult to this collection.
func (fr *FacetResults) AddResult(result *FacetResult) {
	fr.Results = append(fr.Results, result)
}

// GetResult returns the FacetResult for the given dimension, or nil if not found.
func (fr *FacetResults) GetResult(dim string) *FacetResult {
	for _, r := range fr.Results {
		if r.Dim == dim {
			return r
		}
	}
	return nil
}

// Size returns the number of facet results in this collection.
func (fr *FacetResults) Size() int {
	return len(fr.Results)
}
