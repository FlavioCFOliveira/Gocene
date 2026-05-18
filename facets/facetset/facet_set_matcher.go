package facetset

// FacetSetMatcher decides whether a single FacetSet (already unpacked into
// comparable int64 dimensions) matches a logical facet cell. Mirrors the
// abstract org.apache.lucene.facet.facetset.FacetSetMatcher.
type FacetSetMatcher interface {
	// Label is the human-readable identifier rendered in the FacetResult.
	Label() string

	// Dims is the dimensionality the matcher expects; matching against a
	// FacetSet of a different dimensionality must return false.
	Dims() int

	// Matches reports whether the supplied dimensions satisfy this matcher.
	Matches(dimValues []int64) bool
}
