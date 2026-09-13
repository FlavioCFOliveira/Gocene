package facets

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// FacetsCollector is a collector that gathers matching documents for facet counting.
// It collects the documents that match a query so that facet counts can be computed.
//
// Usage:
//
//	fc := facets.NewFacetsCollector()
//	searcher.SearchWithCollector(query, fc)
//	fc.Finish() // must be called to finalize MatchingDocs from each segment
//	matchingDocs := fc.GetMatchingDocs()
//
// This is the Go port of Lucene's org.apache.lucene.facet.FacetsCollector.
type FacetsCollector struct {
	// BaseCollector carries the default body of Collector.setWeight(Weight).
	*search.BaseCollector

	// matchingDocs holds the matching documents per segment
	matchingDocs []*MatchingDocs

	// totalHits is the total number of hits collected
	totalHits int

	// scorer is the current scorer
	scorer search.Scorer

	// keepScores indicates whether to keep scores
	keepScores bool

	// scores holds scores per document if keepScores is true
	scores []float32

	// leafCollectors tracks per-segment leaf collectors so that Finish()
	// can finalize them after the search completes.
	leafCollectors []*facetsLeafCollector
}

// NewFacetsCollector creates a new FacetsCollector.
func NewFacetsCollector() *FacetsCollector {
	return &FacetsCollector{
		BaseCollector:  &search.BaseCollector{},
		matchingDocs:   make([]*MatchingDocs, 0),
		scores:         nil,
		leafCollectors: make([]*facetsLeafCollector, 0),
	}
}

// NewFacetsCollectorWithScores creates a new FacetsCollector that keeps scores.
func NewFacetsCollectorWithScores() *FacetsCollector {
	fc := NewFacetsCollector()
	fc.keepScores = true
	fc.scores = make([]float32, 0)
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
	if !fc.keepScores || doc < 0 || doc >= len(fc.scores) {
		return 0
	}
	return fc.scores[doc]
}

// GetLeafCollector returns a LeafCollector for the given context.
func (fc *FacetsCollector) GetLeafCollector(context *index.LeafReaderContext) (search.LeafCollector, error) {
	// A facets leaf collector needs the leaf reader context (it reads the
	// segment's MaxDoc and records it in MatchingDocs); without one there is
	// nothing to collect, so degrade gracefully as before.
	if context == nil {
		return nil, nil
	}

	flc := &facetsLeafCollector{
		parent:  fc,
		context: context,
		docs:    make([]int, 0),
		scores:  make([]float32, 0),
	}
	fc.leafCollectors = append(fc.leafCollectors, flc)
	return flc, nil
}

// ScoreMode returns the score mode for this collector.
func (fc *FacetsCollector) ScoreMode() search.ScoreMode {
	// FacetsCollector doesn't need scores by default
	if fc.keepScores {
		return search.COMPLETE
	}
	return search.COMPLETE_NO_SCORES
}

// Finish finalizes collection for all segments, building MatchingDocs from
// every leaf collector. Must be called after the search completes and before
// calling GetMatchingDocs().
func (fc *FacetsCollector) Finish() error {
	for _, flc := range fc.leafCollectors {
		if err := flc.Finish(); err != nil {
			return err
		}
	}
	fc.leafCollectors = nil
	return nil
}

// Reset clears all collected data, allowing the collector to be reused.
func (fc *FacetsCollector) Reset() {
	fc.matchingDocs = fc.matchingDocs[:0]
	fc.totalHits = 0
	fc.scores = nil
	fc.leafCollectors = nil
}

// facetsLeafCollector is a LeafCollector implementation for FacetsCollector.
type facetsLeafCollector struct {
	parent  *FacetsCollector
	context *index.LeafReaderContext
	docs    []int
	scores  []float32
	scorer  search.Scorable
}

// SetScorer sets the scorer for this leaf collector.
func (flc *facetsLeafCollector) SetScorer(scorer search.Scorable) error {
	flc.scorer = scorer
	return nil
}

// Collect collects a document.
func (flc *facetsLeafCollector) Collect(doc int) error {
	flc.docs = append(flc.docs, doc)
	flc.parent.totalHits++

	// Store score if keeping scores
	if flc.parent.keepScores && flc.scorer != nil {
		score, err := flc.scorer.Score()
		if err != nil {
			return err
		}
		// Ensure scores slice is large enough for this docID
		for len(flc.scores) <= doc {
			flc.scores = append(flc.scores, 0)
		}
		flc.scores[doc] = score
	}

	return nil
}

// CollectRange mirrors the default body of LeafCollector.collectRange(int, int).
func (flc *facetsLeafCollector) CollectRange(min, max int) error {
	return search.DefaultCollectRange(flc, min, max)
}

// CollectStream mirrors the default body of LeafCollector.collect(DocIdStream).
func (flc *facetsLeafCollector) CollectStream(stream search.DocIdStream) error {
	return search.DefaultCollectStream(flc, stream)
}

// CompetitiveIterator mirrors the default body of
// LeafCollector.competitiveIterator(), which returns null.
func (flc *facetsLeafCollector) CompetitiveIterator() (search.DocIdSetIterator, error) {
	return nil, nil
}

// Finish finalizes collection for this leaf and creates the MatchingDocs.
func (flc *facetsLeafCollector) Finish() error {
	if len(flc.docs) > 0 {
		// Create a FixedBitSet for the matching documents
		maxDoc := flc.context.Reader().MaxDoc()
		bits := NewDocIdSetBits(maxDoc, flc.docs)

		var scores []float32
		if flc.parent.keepScores {
			scores = flc.scores
		}

		md := NewMatchingDocs(flc.context, bits, len(flc.docs), scores)
		flc.parent.matchingDocs = append(flc.parent.matchingDocs, md)

		// Copy scores to parent if keeping scores
		if flc.parent.keepScores && scores != nil {
			for doc, score := range scores {
				if doc >= len(flc.parent.scores) {
					// Grow parent scores slice
					newScores := make([]float32, doc+1)
					copy(newScores, flc.parent.scores)
					flc.parent.scores = newScores
				}
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
