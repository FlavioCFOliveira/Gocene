package facetset

// ExactFacetSetMatcher matches a FacetSet whose dimensions equal a target
// tuple of int64 values. Mirrors
// org.apache.lucene.facet.facetset.ExactFacetSetMatcher.
type ExactFacetSetMatcher struct {
	label string
	dims  []int64
}

// NewExactFacetSetMatcher builds a matcher with the supplied label and the
// reference dimensions.
func NewExactFacetSetMatcher(label string, dims ...int64) *ExactFacetSetMatcher {
	clone := make([]int64, len(dims))
	copy(clone, dims)
	return &ExactFacetSetMatcher{label: label, dims: clone}
}

// Label returns the matcher label.
func (m *ExactFacetSetMatcher) Label() string { return m.label }

// Dims returns the dimensionality.
func (m *ExactFacetSetMatcher) Dims() int { return len(m.dims) }

// Matches returns true iff every dimension equals the corresponding target.
func (m *ExactFacetSetMatcher) Matches(values []int64) bool {
	if len(values) != len(m.dims) {
		return false
	}
	for i, v := range values {
		if v != m.dims[i] {
			return false
		}
	}
	return true
}

var _ FacetSetMatcher = (*ExactFacetSetMatcher)(nil)
