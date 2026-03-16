package facets

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// FacetsCollector is a collector that gathers matching documents for facet counting.
// It collects the documents that match a query so that facet counts can be computed.
type FacetsCollector struct {
	// matchingDocs holds the matching documents per segment
	matchingDocs []*MatchingDocs

	// totalHits is the total number of hits collected
	totalHits int

	// scorer is the current scorer
	scorer search.Scorer
}

// NewFacetsCollector creates a new FacetsCollector.
func NewFacetsCollector() *FacetsCollector {
	return &FacetsCollector{
		matchingDocs: make([]*MatchingDocs, 0),
	}
}

// GetMatchingDocs returns the collected matching documents.
// This is used by Facets implementations to compute facet counts.
func (fc *FacetsCollector) GetMatchingDocs() []*MatchingDocs {
	return fc.matchingDocs
}

// GetTotalHits returns the total number of hits collected.
func (fc *FacetsCollector) GetTotalHits() int {
	return fc.totalHits
}

// GetLeafCollector returns a LeafCollector for the given context.
func (fc *FacetsCollector) GetLeafCollector(reader search.IndexReader) (search.LeafCollector, error) {
	// Get the leaf reader context from the reader
	var leafReader *index.LeafReader
	if lr, ok := reader.(*index.LeafReader); ok {
		leafReader = lr
	} else {
		// For non-leaf readers, we can't create a facets collector
		// This would need to be handled by the caller
		return nil, nil
	}

	// Create a context for this leaf reader
	context := index.NewLeafReaderContext(leafReader, nil, 0, 0)

	return &facetsLeafCollector{
		parent:  fc,
		context: context,
		docs:    make([]int, 0),
	}, nil
}

// ScoreMode returns the score mode for this collector.
func (fc *FacetsCollector) ScoreMode() search.ScoreMode {
	// FacetsCollector doesn't need scores
	return search.COMPLETE_NO_SCORES
}

// facetsLeafCollector is a LeafCollector implementation for FacetsCollector.
type facetsLeafCollector struct {
	parent  *FacetsCollector
	context *index.LeafReaderContext
	docs    []int
	scorer  search.Scorer
}

// SetScorer sets the scorer for this leaf collector.
func (flc *facetsLeafCollector) SetScorer(scorer search.Scorer) error {
	flc.scorer = scorer
	return nil
}

// Collect collects a document.
func (flc *facetsLeafCollector) Collect(doc int) error {
	flc.docs = append(flc.docs, doc)
	flc.parent.totalHits++
	return nil
}

// DocIdSetBits implements Bits for a set of document IDs.
type DocIdSetBits struct {
	length int
	docs   map[int]struct{}
}

// NewDocIdSetBits creates a new DocIdSetBits for the given document IDs.
func NewDocIdSetBits(length int, docs []int) *DocIdSetBits {
	docSet := make(map[int]struct{}, len(docs))
	for _, doc := range docs {
		docSet[doc] = struct{}{}
	}
	return &DocIdSetBits{
		length: length,
		docs:   docSet,
	}
}

// Get returns true if the given document ID is in the set.
func (dsb *DocIdSetBits) Get(doc int) bool {
	_, exists := dsb.docs[doc]
	return exists
}

// Length returns the length of the bitset (max doc ID + 1).
func (dsb *DocIdSetBits) Length() int {
	return dsb.length
}

// Count returns the number of set bits.
func (dsb *DocIdSetBits) Count() int {
	return len(dsb.docs)
}

// SearchManagerWithFacets wraps a search with facet collection.
type SearchManagerWithFacets struct {
	// FacetsCollector collects matching documents
	FacetsCollector *FacetsCollector
}

// NewSearchManagerWithFacets creates a new SearchManagerWithFacets.
func NewSearchManagerWithFacets() *SearchManagerWithFacets {
	return &SearchManagerWithFacets{
		FacetsCollector: NewFacetsCollector(),
	}
}

// MultiCollector wraps multiple collectors into one.
type MultiCollector struct {
	collectors []search.Collector
}

// NewMultiCollector creates a new MultiCollector wrapping the given collectors.
func NewMultiCollector(collectors ...search.Collector) *MultiCollector {
	mc := &MultiCollector{
		collectors: make([]search.Collector, 0, len(collectors)),
	}
	for _, c := range collectors {
		if c != nil {
			mc.collectors = append(mc.collectors, c)
		}
	}
	return mc
}

// GetLeafCollector returns a LeafCollector that wraps all child collectors.
func (mc *MultiCollector) GetLeafCollector(reader search.IndexReader) (search.LeafCollector, error) {
	leafCollectors := make([]search.LeafCollector, 0, len(mc.collectors))
	for _, collector := range mc.collectors {
		lc, err := collector.GetLeafCollector(reader)
		if err != nil {
			return nil, err
		}
		if lc != nil {
			leafCollectors = append(leafCollectors, lc)
		}
	}
	return &multiLeafCollector{collectors: leafCollectors}, nil
}

// ScoreMode returns the score mode for this collector.
func (mc *MultiCollector) ScoreMode() search.ScoreMode {
	// Return the most restrictive score mode
	mode := search.COMPLETE_NO_SCORES
	for _, c := range mc.collectors {
		if c.ScoreMode() == search.COMPLETE {
			mode = search.COMPLETE
			break
		}
	}
	return mode
}

// multiLeafCollector wraps multiple LeafCollectors.
type multiLeafCollector struct {
	collectors []search.LeafCollector
}

// SetScorer sets the scorer for all child collectors.
func (mlc *multiLeafCollector) SetScorer(scorer search.Scorer) error {
	for _, lc := range mlc.collectors {
		if err := lc.SetScorer(scorer); err != nil {
			return err
		}
	}
	return nil
}

// Collect collects a document in all child collectors.
func (mlc *multiLeafCollector) Collect(doc int) error {
	for _, lc := range mlc.collectors {
		if err := lc.Collect(doc); err != nil {
			return err
		}
	}
	return nil
}
