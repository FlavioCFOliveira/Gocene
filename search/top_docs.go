// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

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
