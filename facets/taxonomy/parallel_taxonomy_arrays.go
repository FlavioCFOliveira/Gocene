package taxonomy

// ParallelTaxonomyArrays exposes the three parallel arrays used by the
// taxonomy reader to traverse the category hierarchy: parents, children
// (first-child of each ordinal), and siblings (next-sibling of each ordinal).
// Mirrors org.apache.lucene.facet.taxonomy.ParallelTaxonomyArrays.
type ParallelTaxonomyArrays interface {
	// Parents returns the parent array; index i holds the parent of ordinal i,
	// with the root mapped to -1.
	Parents() []int

	// Children returns the first-child array; index i holds the first child
	// ordinal under i (or -1 when no children).
	Children() []int

	// Siblings returns the next-sibling array; index i holds the next
	// sibling ordinal of i (or -1 when none).
	Siblings() []int
}

// InMemoryParallelTaxonomyArrays is a straightforward immutable triple of
// parents/children/siblings slices.
type InMemoryParallelTaxonomyArrays struct {
	parents  []int
	children []int
	siblings []int
}

// NewInMemoryParallelTaxonomyArrays builds the immutable triple.
func NewInMemoryParallelTaxonomyArrays(parents, children, siblings []int) *InMemoryParallelTaxonomyArrays {
	return &InMemoryParallelTaxonomyArrays{
		parents:  parents,
		children: children,
		siblings: siblings,
	}
}

// Parents returns the parent slice.
func (a *InMemoryParallelTaxonomyArrays) Parents() []int { return a.parents }

// Children returns the first-child slice.
func (a *InMemoryParallelTaxonomyArrays) Children() []int { return a.children }

// Siblings returns the next-sibling slice.
func (a *InMemoryParallelTaxonomyArrays) Siblings() []int { return a.siblings }

var _ ParallelTaxonomyArrays = (*InMemoryParallelTaxonomyArrays)(nil)
