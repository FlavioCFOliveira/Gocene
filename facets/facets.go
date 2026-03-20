package facets

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// Facets is the base interface for all facets implementations.
// It provides methods to retrieve facet counts and values for search results.
type Facets interface {
	// GetTopChildren returns the top N facet counts for the specified dimension.
	// Parameters:
	//   - topN: maximum number of children to return
	//   - dim: the dimension/facet field name
	//   - path: optional path for hierarchical facets
	// Returns:
	//   - FacetResult containing the top children, or error if dimension not found
	GetTopChildren(topN int, dim string, path ...string) (*FacetResult, error)

	// GetAllDims returns all dimensions available in this facets instance.
	// Returns a slice of dimension names (facet field names).
	GetAllDims(dims ...string) ([]*FacetResult, error)

	// GetSpecificValue returns the value for a specific label in a dimension.
	// This is useful for getting the count of a specific facet value.
	GetSpecificValue(dim string, path ...string) (*FacetResult, error)
}

// FacetsReader provides access to facet data from an index.
// This is typically used by Facets implementations to read facet values.
type FacetsReader interface {
	// GetIndexReader returns the index reader for accessing doc values.
	GetIndexReader() index.IndexReaderInterface
}

// FacetsWriter handles writing facet data to the index.
// This is used during indexing to store facet values.
type FacetsWriter interface {
	// WriteFacet writes a facet value for the given document and dimension.
	WriteFacet(docID int, dim string, value string) error

	// WriteHierarchicalFacet writes a hierarchical facet value.
	WriteHierarchicalFacet(docID int, dim string, path []string) error

	// Flush flushes any pending facet data to the index.
	Flush() error
}

// MatchingDocs tracks which documents match the current search criteria
// and provides access to their facet values.
type MatchingDocs struct {
	// Context is the leaf reader context for the matching documents
	Context *index.LeafReaderContext

	// Bits is the bitset of matching documents (may be nil if all docs match)
	Bits Bits

	// TotalHits is the total number of matching documents
	TotalHits int
}

// Bits is a simple bitset interface for matching documents.
type Bits interface {
	// Get returns true if the bit at the given index is set.
	Get(index int) bool

	// Length returns the number of bits in the set.
	Length() int
}

// NewMatchingDocs creates a new MatchingDocs for the given context.
func NewMatchingDocs(context *index.LeafReaderContext, bits Bits, totalHits int) *MatchingDocs {
	return &MatchingDocs{
		Context:   context,
		Bits:      bits,
		TotalHits: totalHits,
	}
}

// GetDocCount returns the number of matching documents in this segment.
func (md *MatchingDocs) GetDocCount() int {
	return md.TotalHits
}

// GetLeafReader returns the leaf reader for this matching docs set.
func (md *MatchingDocs) GetLeafReader() index.LeafReaderInterface {
	if md.Context != nil {
		return md.Context.LeafReader()
	}
	return nil
}

// FacetsCalculator is the interface for calculating facet counts.
// Different implementations can provide different counting strategies.
type FacetsCalculator interface {
	// Calculate calculates facet counts for the given matching documents.
	Calculate(matchingDocs []*MatchingDocs, dim string) (*FacetResult, error)

	// CalculateTopChildren calculates the top N facet children.
	CalculateTopChildren(matchingDocs []*MatchingDocs, topN int, dim string, path []string) (*FacetResult, error)
}

// DrillDownQuery represents a query that drills down into a specific facet value.
// This is used to filter search results to documents matching a specific facet.
type DrillDownQuery struct {
	// Dim is the dimension being drilled down
	Dim string

	// Path is the path for hierarchical facets
	Path []string

	// Value is the specific facet value to drill down to
	Value string
}

// NewDrillDownQuery creates a new DrillDownQuery.
func NewDrillDownQuery(dim string, value string) *DrillDownQuery {
	return &DrillDownQuery{
		Dim:   dim,
		Value: value,
		Path:  make([]string, 0),
	}
}

// NewDrillDownQueryWithPath creates a new DrillDownQuery with a hierarchical path.
func NewDrillDownQueryWithPath(dim string, path []string, value string) *DrillDownQuery {
	ddq := NewDrillDownQuery(dim, value)
	ddq.Path = append(ddq.Path, path...)
	return ddq
}
