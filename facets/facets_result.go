package facets

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// FacetsResult holds the results of a search run that also collected facets.
// It contains the top docs and the facets collector used to gather facet counts.
type FacetsResult struct {
	// TopDocs contains the top matching documents.
	TopDocs *search.TopDocs

	// FacetsCollector contains the facet counts collected during the search.
	FacetsCollector *FacetsCollector
}

// NewFacetsResult creates a new FacetsResult.
func NewFacetsResult(topDocs *search.TopDocs, facetsCollector *FacetsCollector) *FacetsResult {
	return &FacetsResult{
		TopDocs:         topDocs,
		FacetsCollector: facetsCollector,
	}
}
