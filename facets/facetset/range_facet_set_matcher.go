package facetset

// RangeFacetSetMatcher matches a FacetSet whose dimensions all fall within a
// per-dim DimRange. Mirrors
// org.apache.lucene.facet.facetset.RangeFacetSetMatcher.
type RangeFacetSetMatcher struct {
	label  string
	ranges []DimRange
}

// NewRangeFacetSetMatcher builds a matcher with the supplied label and
// per-dimension ranges.
func NewRangeFacetSetMatcher(label string, ranges ...DimRange) *RangeFacetSetMatcher {
	clone := make([]DimRange, len(ranges))
	copy(clone, ranges)
	return &RangeFacetSetMatcher{label: label, ranges: clone}
}

// Label returns the matcher label.
func (m *RangeFacetSetMatcher) Label() string { return m.label }

// Dims returns the dimensionality.
func (m *RangeFacetSetMatcher) Dims() int { return len(m.ranges) }

// Matches reports whether every dimension lies within its DimRange.
func (m *RangeFacetSetMatcher) Matches(values []int64) bool {
	if len(values) != len(m.ranges) {
		return false
	}
	for i, v := range values {
		if !m.ranges[i].Contains(v) {
			return false
		}
	}
	return true
}

var _ FacetSetMatcher = (*RangeFacetSetMatcher)(nil)
