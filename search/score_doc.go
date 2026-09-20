// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "github.com/FlavioCFOliveira/Gocene/spi"

// ScoreDoc represents a scored document.
type ScoreDoc = spi.ScoreDoc

// NewScoreDoc creates a new ScoreDoc.
func NewScoreDoc(doc int, score float32, shardIndex int) *ScoreDoc {
	return spi.NewScoreDoc(doc, score, shardIndex)
}
