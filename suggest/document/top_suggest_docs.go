package document

import "sort"

// TopSuggestDocs is the result container returned by
// TopSuggestDocsCollector. Mirrors
// org.apache.lucene.search.suggest.document.TopSuggestDocs.
type TopSuggestDocs struct {
	Hits []SuggestHit
}

// SuggestHit is a single (doc, key, score, contexts) tuple.
type SuggestHit struct {
	Doc      int
	Key      string
	Score    float32
	Contexts []string
}

// TopSuggestDocsCollector collects up to NumHits hits ranked by score. Mirrors
// org.apache.lucene.search.suggest.document.TopSuggestDocsCollector.
type TopSuggestDocsCollector struct {
	NumHits int
	hits    []SuggestHit
}

// NewTopSuggestDocsCollector builds the collector.
func NewTopSuggestDocsCollector(numHits int) *TopSuggestDocsCollector {
	if numHits < 1 {
		numHits = 10
	}
	return &TopSuggestDocsCollector{NumHits: numHits}
}

// Collect records a hit.
func (c *TopSuggestDocsCollector) Collect(hit SuggestHit) {
	c.hits = append(c.hits, hit)
	sort.SliceStable(c.hits, func(i, j int) bool { return c.hits[i].Score > c.hits[j].Score })
	if len(c.hits) > c.NumHits {
		c.hits = c.hits[:c.NumHits]
	}
}

// Get returns the accumulated TopSuggestDocs.
func (c *TopSuggestDocsCollector) Get() *TopSuggestDocs {
	out := &TopSuggestDocs{Hits: make([]SuggestHit, len(c.hits))}
	copy(out.Hits, c.hits)
	return out
}
