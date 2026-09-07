package facets

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// FacetsCollectorManager is a CollectorManager implementation which produces
// FacetsCollector and produces a merged FacetsCollector. This is used for
// concurrent FacetsCollection.
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
func (fcm *FacetsCollectorManager) NewCollector() (*FacetsCollector, error) {
	if fcm.keepScores {
		return NewFacetsCollectorWithScores(), nil
	}
	return NewFacetsCollector(), nil
}

// Reduce reduces the results of individual collectors into a merged FacetsCollector.
func (fcm *FacetsCollectorManager) Reduce(collectors []*FacetsCollector) (*FacetsCollector, error) {
	if len(collectors) == 0 {
		return NewFacetsCollector(), nil
	}
	if len(collectors) == 1 {
		return collectors[0], nil
	}

	// Merge matching docs held by the provided facets collectors, merging
	// matching docs for the same leaf into a single matching docs instance.
	matchingDocsMap := make(map[*index.LeafReaderContext]*MatchingDocs)
	for _, fc := range collectors {
		for _, md := range fc.GetMatchingDocs() {
			if existing, ok := matchingDocsMap[md.Context]; ok {
				matchingDocsMap[md.Context] = mergeMatchingDocs(existing, md)
			} else {
				matchingDocsMap[md.Context] = md
			}
		}
	}

	// Create a new collector and populate it with the reduced matching docs.
	reduced := NewFacetsCollector()
	if fcm.keepScores {
		reduced.keepScores = true
	}

	for _, md := range matchingDocsMap {
		reduced.matchingDocs = append(reduced.matchingDocs, md)
		reduced.totalHits += md.TotalHits
	}

	// Note: in a real Lucene implementation, we'd also merge the global scores
	// map if keepScores is true. Here we'll just let the search utility handle it.

	return reduced, nil
}

func mergeMatchingDocs(md1, md2 *MatchingDocs) *MatchingDocs {
	// Merge the bits
	maxDoc := md1.Context.Reader().MaxDoc()
	bits1 := md1.Bits
	bits2 := md2.Bits

	// Create a new bitset that is the union of bits1 and bits2.
	// In Gocene, we can use a map-based bitset for this.
	unionDocs := make([]int, 0, md1.TotalHits+md2.TotalHits)

	// This is inefficient for large sets, but matches the provided DocIdSetBits implementation.
	// A better way would be to use a proper BitSet.
	mergedBits := NewDocIdSetBits(maxDoc, nil)

	// Add all from md1
	for i := 0; i < maxDoc; i++ {
		if bits1.Get(i) {
			mergedBits.docs[i] = struct{}{}
		}
		if bits2.Get(i) {
			mergedBits.docs[i] = struct{}{}
		}
	}

	// Recalculate total hits
	totalHits := 0
	for i := 0; i < maxDoc; i++ {
		if mergedBits.Get(i) {
			totalHits++
		}
	}

	// Merge scores if present
	var mergedScores []float32
	if md1.Scores != nil || md2.Scores != nil {
		len1 := len(md1.Scores)
		len2 := len(md2.Scores)
		maxLen := len1
		if len2 > maxLen {
			maxLen = len2
		}
		mergedScores = make([]float32, maxLen)
		for i := 0; i < maxLen; i++ {
			var s1, s2 float32
			if i < len1 {
				s1 = md1.Scores[i]
			}
			if i < len2 {
				s2 = md2.Scores[i]
			}
			if s1 > s2 {
				mergedScores[i] = s1
			} else {
				mergedScores[i] = s2
			}
		}
	}

	return NewMatchingDocs(md1.Context, mergedBits, totalHits, mergedScores)
}

// Search performs a search with facet collection and returns the FacetsResult.
func Search(searcher *search.IndexSearcher, query search.Query, n int, fcm *FacetsCollectorManager) (*FacetsResult, error) {
	// This is a simplified version of the Lucene doSearch utility.
	// It uses the Gocene SearchWithCollectorManager to perform the concurrent search.

	// We need a CollectorManager that produces *FacetsCollector and reduces to *FacetsCollector.
	// FacetsCollectorManager implements search.CollectorManager[*FacetsCollector, *FacetsCollector].

	fc, err := search.SearchWithCollectorManager(searcher, query, fcm)
	if err != nil {
		return nil, err
	}

	// In Lucene, search() also returns TopDocs.
	// To get TopDocs, we need to run a separate search or use a MultiCollector.
	// For this port, we'll run a simple Search to get TopDocs.
	topDocs, err := searcher.Search(query, n)
	if err != nil {
		return nil, err
	}

	return NewFacetsResult(topDocs, fc), nil
}
