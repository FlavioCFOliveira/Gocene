package facets

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// FacetsCollector is a collector that gathers matching documents for facet counting.
// It collects the documents that match a query so that facet counts can be computed.
//
// This is the Go port of Lucene's org.apache.lucene.facet.FacetsCollector.
type FacetsCollector struct {
	// matchingDocs holds the matching documents per segment
	matchingDocs []*MatchingDocs

	// totalHits is the total number of hits collected
	totalHits int

	// scorer is the current scorer
	scorer search.Scorer

	// keepScores indicates whether to keep scores
	keepScores bool

	// scores holds scores per document if keepScores is true
	scores map[int]float32
}

// NewFacetsCollector creates a new FacetsCollector.
func NewFacetsCollector() *FacetsCollector {
	return &FacetsCollector{
		matchingDocs: make([]*MatchingDocs, 0),
		scores:       make(map[int]float32),
	}
}

// NewFacetsCollectorWithScores creates a new FacetsCollector that keeps scores.
func NewFacetsCollectorWithScores() *FacetsCollector {
	fc := NewFacetsCollector()
	fc.keepScores = true
	return fc
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

// GetScore returns the score for a document if scores are being kept.
// Returns 0 if scores are not being kept or document not found.
func (fc *FacetsCollector) GetScore(doc int) float32 {
	if !fc.keepScores {
		return 0
	}
	if score, ok := fc.scores[doc]; ok {
		return score
	}
	return 0
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
	ctx := index.NewLeafReaderContext(leafReader, nil, 0, 0)

	return &facetsLeafCollector{
		parent:  fc,
		context: ctx,
		docs:    make([]int, 0),
		scores:  make(map[int]float32),
	}, nil
}

// ScoreMode returns the score mode for this collector.
func (fc *FacetsCollector) ScoreMode() search.ScoreMode {
	// FacetsCollector doesn't need scores by default
	if fc.keepScores {
		return search.COMPLETE
	}
	return search.COMPLETE_NO_SCORES
}

// Reset clears all collected data, allowing the collector to be reused.
func (fc *FacetsCollector) Reset() {
	fc.matchingDocs = fc.matchingDocs[:0]
	fc.totalHits = 0
	fc.scores = make(map[int]float32)
}

// facetsLeafCollector is a LeafCollector implementation for FacetsCollector.
type facetsLeafCollector struct {
	parent  *FacetsCollector
	context *index.LeafReaderContext
	docs    []int
	scores  map[int]float32
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

	// Store score if keeping scores
	if flc.parent.keepScores && flc.scorer != nil {
		score := flc.scorer.Score()
		flc.scores[doc] = score
	}

	return nil
}

// Finish finalizes collection for this leaf and creates the MatchingDocs.
func (flc *facetsLeafCollector) Finish() error {
	if len(flc.docs) > 0 {
		// Create a FixedBitSet for the matching documents
		maxDoc := flc.context.Reader().MaxDoc()
		bits := NewDocIdSetBits(maxDoc, flc.docs)
		md := NewMatchingDocs(flc.context, bits, len(flc.docs))
		flc.parent.matchingDocs = append(flc.parent.matchingDocs, md)

		// Copy scores to parent if keeping scores
		if flc.parent.keepScores {
			for doc, score := range flc.scores {
				flc.parent.scores[doc] = score
			}
		}
	}
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

// Search performs a search and collects facet information.
func (smwf *SearchManagerWithFacets) Search(searcher *search.IndexSearcher, query search.Query, collector search.Collector) (*FacetsCollector, error) {
	// Create a multi-collector that collects both the original collector and facets
	multiCollector := NewMultiCollector(collector, smwf.FacetsCollector)
	err := searcher.SearchWithCollector(query, multiCollector)
	if err != nil {
		return nil, err
	}
	return smwf.FacetsCollector, nil
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

// FacetsCollectorManager manages FacetsCollector instances for search.
// This provides a convenient way to perform searches with facet collection.
type FacetsCollectorManager struct {
	// keepScores indicates whether to keep scores
	keepScores bool
}

// NewFacetsCollectorManager creates a new FacetsCollectorManager.
func NewFacetsCollectorManager() *FacetsCollectorManager {
	return &FacetsCollectorManager{
		keepScores: false,
	}
}

// NewFacetsCollectorManagerWithScores creates a manager that keeps scores.
func NewFacetsCollectorManagerWithScores() *FacetsCollectorManager {
	return &FacetsCollectorManager{
		keepScores: true,
	}
}

// NewCollector creates a new FacetsCollector.
func (fcm *FacetsCollectorManager) NewCollector() *FacetsCollector {
	if fcm.keepScores {
		return NewFacetsCollectorWithScores()
	}
	return NewFacetsCollector()
}

// Search performs a search with facet collection and returns the FacetsCollector.
func (fcm *FacetsCollectorManager) Search(searcher *search.IndexSearcher, query search.Query) (*FacetsCollector, error) {
	fc := fcm.NewCollector()
	err := searcher.SearchWithCollector(query, fc)
	if err != nil {
		return nil, fmt.Errorf("search with facets failed: %w", err)
	}
	return fc, nil
}

// SearchWithCollector performs a search with both a result collector and facet collection.
func (fcm *FacetsCollectorManager) SearchWithCollector(searcher *search.IndexSearcher, query search.Query, collector search.Collector) (*FacetsCollector, error) {
	fc := fcm.NewCollector()
	multiCollector := NewMultiCollector(collector, fc)
	err := searcher.SearchWithCollector(query, multiCollector)
	if err != nil {
		return nil, fmt.Errorf("search with facets and collector failed: %w", err)
	}
	return fc, nil
}
