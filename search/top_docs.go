// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"sort"
)

// TopDocs holds the top scoring documents.
type TopDocs struct {
	TotalHits *TotalHits
	ScoreDocs []*ScoreDoc
	MaxScore  float32
}

// NewTopDocs creates a new TopDocs.
func NewTopDocs(totalHits *TotalHits, scoreDocs []*ScoreDoc) *TopDocs {
	return &TopDocs{
		TotalHits: totalHits,
		ScoreDocs: scoreDocs,
		MaxScore:  0,
	}
}

// Merge merges multiple TopDocs into one.
func Merge(topDocs []*TopDocs, n int) *TopDocs {
	if len(topDocs) == 0 {
		return nil
	}
	if len(topDocs) == 1 {
		return topDocs[0]
	}

	var totalHits int64
	var maxScore float32
	relation := EQUAL_TO

	// Count total hits and find max score
	for _, td := range topDocs {
		if td == nil {
			continue
		}
		totalHits += td.TotalHits.Value
		if td.TotalHits.Relation == GREATER_THAN_OR_EQUAL_TO {
			relation = GREATER_THAN_OR_EQUAL_TO
		}
		if td.MaxScore > maxScore {
			maxScore = td.MaxScore
		}
	}

	// Simple merge for now: collect all and sort
	// In production, use a priority queue for efficiency
	allDocs := make([]*ScoreDoc, 0)
	for _, td := range topDocs {
		if td != nil {
			allDocs = append(allDocs, td.ScoreDocs...)
		}
	}

	// Sort by score descending, then by doc ID ascending
	sort.Slice(allDocs, func(i, j int) bool {
		if allDocs[i].Score != allDocs[j].Score {
			return allDocs[i].Score > allDocs[j].Score
		}
		return allDocs[i].Doc < allDocs[j].Doc
	})

	// Limit to n
	if len(allDocs) > n {
		allDocs = allDocs[:n]
	}

	return &TopDocs{
		TotalHits: NewTotalHits(totalHits, relation),
		ScoreDocs: allDocs,
		MaxScore:  maxScore,
	}
}
