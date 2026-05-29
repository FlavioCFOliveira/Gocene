package facets

import (
	"fmt"

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

// DocValuesLeafReader is the subset of the per-segment reader surface the
// facets accumulators need to count facet ordinals directly from the codec's
// doc-values producer. SegmentReader (the leaf reader returned by
// OpenDirectoryReader) satisfies it; LeafReaderContext.Reader() returns a value
// that can be type-asserted to it.
//
// The doc-values getters mirror the Lucene 10.4.0 reader contract used by
// org.apache.lucene.facet.sortedset.SortedSetDocValuesFacetCounts (SortedSet)
// and org.apache.lucene.facet.taxonomy.FastTaxonomyFacetCounts
// (SortedNumeric); both return nil when the field carries no values for that
// type, in which case the segment contributes nothing.
type DocValuesLeafReader interface {
	// GetSortedSetDocValues returns the SortedSetDocValues for field, or nil
	// when the field has no sorted-set doc values in this segment.
	GetSortedSetDocValues(field string) (index.SortedSetDocValues, error)

	// GetSortedNumericDocValues returns the SortedNumericDocValues for field,
	// or nil when the field has no sorted-numeric doc values in this segment.
	GetSortedNumericDocValues(field string) (index.SortedNumericDocValues, error)
}

// DocValuesReader returns the segment reader behind this MatchingDocs as a
// DocValuesLeafReader, or nil when the context does not expose the doc-values
// getters (for example test stubs that only implement the leaf reader surface).
// This is the entry point the accumulators use for the default, codec-driven
// counting path when no explicit resolver hook is installed.
func (md *MatchingDocs) DocValuesReader() DocValuesLeafReader {
	if md == nil || md.Context == nil {
		return nil
	}
	if dvr, ok := md.Context.Reader().(DocValuesLeafReader); ok {
		return dvr
	}
	return nil
}

// ForEachTaxonomyOrdinal drives the default, codec-driven taxonomy counting
// path shared by the ordinal-based accumulators (TaxonomyFacetsAccumulator,
// ConcurrentFacetsAccumulator, RandomSamplingFacetsAccumulator).
//
// It reads the segment's SortedNumericDocValues for field — the encoding
// Lucene's taxonomy faceting uses to persist the per-document ordinal stream
// (org.apache.lucene.facet.taxonomy.FastTaxonomyFacetCounts.countOneSegment) —
// and invokes visit(docID, ord) for every ordinal of every matching document,
// filtered by md.Bits. The doc IDs are leaf-local.
//
// When the context does not expose the doc-values SPI, or the field has no
// SortedNumericDocValues in this segment, the call is a no-op and returns nil.
func ForEachTaxonomyOrdinal(md *MatchingDocs, field string, visit func(docID, ord int)) error {
	if md == nil || md.Context == nil {
		return nil
	}
	dvr := md.DocValuesReader()
	if dvr == nil {
		return nil
	}

	dv, err := dvr.GetSortedNumericDocValues(field)
	if err != nil {
		return fmt.Errorf("getting sorted numeric doc values for %q: %w", field, err)
	}
	if dv == nil {
		return nil
	}

	bits := md.Bits
	maxDoc := md.Context.Reader().MaxDoc()
	for {
		doc, err := dv.NextDoc()
		if err != nil {
			return fmt.Errorf("advancing taxonomy ordinals: %w", err)
		}
		if doc < 0 || doc >= maxDoc {
			break
		}
		if bits != nil && !bits.Get(doc) {
			continue
		}
		count, err := dv.DocValueCount()
		if err != nil {
			return fmt.Errorf("reading ordinal count for doc %d: %w", doc, err)
		}
		for i := 0; i < count; i++ {
			v, err := dv.NextValue()
			if err != nil {
				return fmt.Errorf("reading ordinal %d for doc %d: %w", i, doc, err)
			}
			visit(doc, int(v))
		}
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
