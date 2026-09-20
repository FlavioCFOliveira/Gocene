// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// QueryRescorer re-scores documents using a secondary query.
// This is the Go port of Lucene's org.apache.lucene.search.QueryRescorer
// (Lucene 10.4.0).
//
// The first-pass top hits are re-scored against the secondary query and
// re-ranked using a configurable combine function. The default combine is
// firstPassScore + secondPassScore (matching the static helper in Lucene's
// QueryRescorer when called with weight=1.0).
type QueryRescorer struct {
	query   Query
	combine QueryRescorerCombine
}

// QueryRescorerCombine combines a first-pass and second-pass score for one
// document. secondPassMatches is false when the secondary query did not
// match the document; in that case secondPassScore is 0 and callers
// typically return firstPassScore unchanged.
//
// Mirrors QueryRescorer.combine(float, boolean, float) (Lucene 10.4.0).
type QueryRescorerCombine func(firstPassScore float32, secondPassMatches bool, secondPassScore float32) float32

// defaultQueryRescorerCombine implements the static helper variant where
// rescore weight=1.0: combined = firstPassScore + secondPassScore on a
// match, firstPassScore on a non-match.
func defaultQueryRescorerCombine(firstPassScore float32, secondPassMatches bool, secondPassScore float32) float32 {
	if secondPassMatches {
		return firstPassScore + secondPassScore
	}
	return firstPassScore
}

// NewQueryRescorer creates a new QueryRescorer using the default
// (firstPassScore + secondPassScore) combine.
func NewQueryRescorer(query Query) *QueryRescorer {
	return &QueryRescorer{query: query, combine: defaultQueryRescorerCombine}
}

// NewQueryRescorerWithCombine creates a new QueryRescorer with a custom
// combine function.
func NewQueryRescorerWithCombine(query Query, combine QueryRescorerCombine) *QueryRescorer {
	if combine == nil {
		combine = defaultQueryRescorerCombine
	}
	return &QueryRescorer{query: query, combine: combine}
}

// GetQuery returns the rescore query.
func (r *QueryRescorer) GetQuery() Query {
	return r.query
}

// Rescore re-scores the documents in topDocs using r.query.
//
// Port of QueryRescorer.rescore (Lucene 10.4.0). The first-pass hits are
// sorted by docID (so scoring within each leaf can advance forward),
// scored against the rewritten second-pass weight, and then sorted by the
// combined score descending (with docID ascending as the tie-breaker).
//
// Degradation: when the searcher's reader is not a DirectoryReader (i.e.
// there are no leaves to walk), the rescorer falls back to creating a
// single weight against the entire reader and treating it as one leaf.
// This keeps the rescore semantically correct for the single-segment case
// even before composite-reader leaf walking is wired through the searcher.
func (r *QueryRescorer) Rescore(searcher *IndexSearcher, topDocs *TopDocs) (*TopDocs, error) {
	if topDocs == nil || len(topDocs.ScoreDocs) == 0 || r.query == nil || searcher == nil {
		return topDocs, nil
	}

	// Clone the hits so we can rearrange / rescore without mutating the
	// caller's slice. Java sorts firstPassTopDocs.scoreDocs in place.
	hits := make([]*ScoreDoc, len(topDocs.ScoreDocs))
	for i, h := range topDocs.ScoreDocs {
		clone := *h
		hits[i] = &clone
	}

	sort.SliceStable(hits, func(i, j int) bool { return hits[i].Doc < hits[j].Doc })

	leaves, err := searcher.GetIndexReader().Leaves()
	if err != nil {
		return nil, err
	}

	rewritten, err := searcher.Rewrite(r.query)
	if err != nil {
		return nil, err
	}
	weight, err := searcher.CreateWeight(rewritten, ScoreModeComplete, 1)
	if err != nil {
		return nil, err
	}

	// Now merge sort docIDs from hits, with reader's leaves:
	hitUpto := 0
	readerUpto := -1
	endDoc := 0
	docBase := 0
	var scorer Scorer

	for hitUpto < len(hits) {
		hit := hits[hitUpto]
		docID := hit.Doc
		var readerContext *index.LeafReaderContext
		for docID >= endDoc {
			readerUpto++
			if readerUpto >= len(leaves) {
				return nil, fmt.Errorf("doc id %d is out of bounds", docID)
			}
			readerContext = leaves[readerUpto]
			endDoc = readerContext.DocBase + readerContext.LeafReader().MaxDoc()
		}

		if readerContext != nil {
			// We advanced to another segment:
			docBase = readerContext.DocBase
			scorer, err = weight.Scorer(readerContext)
			if err != nil {
				return nil, err
			}
		}

		if scorer != nil {
			targetDoc := docID - docBase
			actualDoc := scorer.DocID()
			if actualDoc < targetDoc {
				actualDoc, err = scorer.Iterator().Advance(targetDoc)
				if err != nil {
					return nil, err
				}
			}

			if actualDoc == targetDoc {
				// Query did match this doc:
				score, err := scorer.Score()
				if err != nil {
					return nil, err
				}
				hit.Score = r.combine(hit.Score, true, score)
			} else {
				// Query did not match this doc:
				hit.Score = r.combine(hit.Score, false, 0.0)
			}
		} else {
			// Query did not match this doc:
			hit.Score = r.combine(hit.Score, false, 0.0)
		}

		hitUpto++
	}

	// Sort by score descending, then docID ascending.
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].Doc < hits[j].Doc
	})

	return &TopDocs{TotalHits: topDocs.TotalHits, ScoreDocs: hits}, nil
}

// Ensure QueryRescorer implements Rescorer
var _ Rescorer = (*QueryRescorer)(nil)
