// Package cutters implements org.apache.lucene.sandbox.facet.cutters.
package cutters

// FacetCutter is the contract every facet cutter implements: given a
// document, it returns a list of facet ordinals.
type FacetCutter interface {
	Ordinals(docID int) []int
}

// LeafFacetCutter is the per-segment variant.
type LeafFacetCutter interface {
	FacetCutter
}

// TaxonomyFacetsCutter slices facets along a taxonomy hierarchy.
type TaxonomyFacetsCutter struct {
	OrdinalsFn func(docID int) []int
}

// NewTaxonomyFacetsCutter builds the cutter.
func NewTaxonomyFacetsCutter(fn func(docID int) []int) *TaxonomyFacetsCutter {
	return &TaxonomyFacetsCutter{OrdinalsFn: fn}
}

// Ordinals delegates to the wrapped function.
func (c *TaxonomyFacetsCutter) Ordinals(docID int) []int {
	if c.OrdinalsFn == nil {
		return nil
	}
	return c.OrdinalsFn(docID)
}

var _ FacetCutter = (*TaxonomyFacetsCutter)(nil)

// LongValueFacetCutter slices facets based on a per-document long value.
type LongValueFacetCutter struct {
	ValueFn func(docID int) (int64, bool)
}

// NewLongValueFacetCutter builds the cutter.
func NewLongValueFacetCutter(fn func(docID int) (int64, bool)) *LongValueFacetCutter {
	return &LongValueFacetCutter{ValueFn: fn}
}

// Ordinals returns a single-element slice with the long value (or nil when
// absent).
func (c *LongValueFacetCutter) Ordinals(docID int) []int {
	if c.ValueFn == nil {
		return nil
	}
	if v, ok := c.ValueFn(docID); ok {
		return []int{int(v)}
	}
	return nil
}

var _ FacetCutter = (*LongValueFacetCutter)(nil)
