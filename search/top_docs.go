// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"errors"
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

// MergeWithStart merges multiple TopDocs with pagination support (from/size).
// This is equivalent to TopDocs.merge(from, size, topDocs) in Lucene.
// Returns an error if shard indices are inconsistent (some set, some not).
func MergeWithStart(from, size int, topDocs []*TopDocs) (*TopDocs, error) {
	if len(topDocs) == 0 {
		return nil, nil
	}
	if len(topDocs) == 1 {
		return topDocs[0], nil
	}

	// Check for consistent shard indices
	hasSetShardIndex := false
	hasUnsetShardIndex := false
	for _, td := range topDocs {
		if td == nil {
			continue
		}
		for _, sd := range td.ScoreDocs {
			if sd.ShardIndex >= 0 {
				hasSetShardIndex = true
			} else {
				hasUnsetShardIndex = true
			}
		}
	}

	// Inconsistent shard indices - some set, some not
	if hasSetShardIndex && hasUnsetShardIndex {
		return nil, errors.New("inconsistent shard indices: some ScoreDocs have shardIndex set, others do not")
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

	// Collect all docs
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

	// Apply from/size pagination
	if from < len(allDocs) {
		end := from + size
		if end > len(allDocs) {
			end = len(allDocs)
		}
		allDocs = allDocs[from:end]
	} else {
		allDocs = make([]*ScoreDoc, 0)
	}

	return &TopDocs{
		TotalHits: NewTotalHits(totalHits, relation),
		ScoreDocs: allDocs,
		MaxScore:  maxScore,
	}, nil
}
