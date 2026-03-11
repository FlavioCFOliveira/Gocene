// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// ScoreDoc represents a scored document.
type ScoreDoc struct {
	Doc       int
	Score     float32
	ShardIndex int
}

// NewScoreDoc creates a new ScoreDoc.
func NewScoreDoc(doc int, score float32, shardIndex int) *ScoreDoc {
	return &ScoreDoc{
		Doc:        doc,
		Score:      score,
		ShardIndex: shardIndex,
	}
}
