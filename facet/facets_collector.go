// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facet

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// MatchingDocs holds the documents that were matched in the LeafReaderContext.
// If scores were required, then Scores is not null.
type MatchingDocs struct {
	Context   *index.LeafReaderContext
	Bits      search.DocIdSet
	TotalHits int
	Scores    []float32
}

// FacetsCollector collects hits for subsequent faceting.
//
// This is the Go port of Lucene's org.apache.lucene.facet.FacetsCollector.
type FacetsCollector struct {
	context      *index.LeafReaderContext
	scorer       search.Scorable
	totalHits    int
	scores       []float32
	keepScores   bool
	matchingDocs []*MatchingDocs
	docsBuilder  *util.DocIdSetBuilder
}

// NewFacetsCollector creates a new FacetsCollector.
func NewFacetsCollector() *FacetsCollector {
	return NewFacetsCollectorWithScores(false)
}

// NewFacetsCollectorWithScores creates a new FacetsCollector; if keepScores is true then a float32 slice is allocated to hold score of all hits.
func NewFacetsCollectorWithScores(keepScores bool) *FacetsCollector {
	return &FacetsCollector{
		keepScores: keepScores,
	}
}

// GetKeepScores returns true if scores were saved.
func (c *FacetsCollector) GetKeepScores() bool {
	return c.keepScores
}

// GetMatchingDocs returns the documents matched by the query, one MatchingDocs per visited segment.
func (c *FacetsCollector) GetMatchingDocs() []*MatchingDocs {
	return c.matchingDocs
}

// Collect collects the document.
func (c *FacetsCollector) Collect(doc int) {
	c.docsBuilder.Grow(1)
	c.docsBuilder.Add(doc)

	if c.keepScores {
		if doc >= len(c.scores) {
			newScores := make([]float32, util.Oversize(doc+1, 4))
			copy(newScores, c.scores)
			c.scores = newScores
		}
		c.scores[doc] = c.scorer.Score()
	}
	c.totalHits++
}

// ScoreMode returns the score mode.
func (c *FacetsCollector) ScoreMode() search.ScoreMode {
	if c.keepScores {
		return search.ScoreModeComplete
	}
	return search.ScoreModeCompleteNoScores
}

// SetScorer sets the scorer.
func (c *FacetsCollector) SetScorer(scorer search.Scorable) {
	c.scorer = scorer
}

// SetNextReader is called before collecting hits from a new segment.
func (c *FacetsCollector) SetNextReader(context *index.LeafReaderContext) {
	if c.docsBuilder != nil {
		panic("docsBuilder should be nil")
	}
	c.docsBuilder = util.NewDocIdSetBuilder(context.Reader().MaxDoc())
	c.totalHits = 0
	if c.keepScores {
		c.scores = make([]float32, 64) // some initial size
	}
	c.context = context
}

// Finish finishes the collection and adds the result to matchingDocs.
func (c *FacetsCollector) Finish() {
	var bits search.DocIdSet
	if c.docsBuilder != nil {
		bits = c.docsBuilder.Build()
		c.docsBuilder = nil
	} else {
		bits = search.DocIdSetEmpty
	}
	c.matchingDocs = append(c.matchingDocs, &MatchingDocs{
		Context:   c.context,
		Bits:      bits,
		TotalHits: c.totalHits,
		Scores:    c.scores,
	})
	c.scores = nil
	c.context = nil
}
