// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

// ScoringMatch extends QueryMatch with a relevance score.
//
// Port of org.apache.lucene.monitor.ScoringMatch.
type ScoringMatch struct {
	QueryMatch
	score float32
}

// NewScoringMatch creates a ScoringMatch for the given query ID and score.
func NewScoringMatch(queryID string, score float32) *ScoringMatch {
	return &ScoringMatch{
		QueryMatch: QueryMatch{queryID: queryID},
		score:      score,
	}
}

// GetScore returns the relevance score of this match.
func (m *ScoringMatch) GetScore() float32 { return m.score }

// Equals returns true when both query IDs and scores are equal.
func (m *ScoringMatch) Equals(other *ScoringMatch) bool {
	if m == other {
		return true
	}
	if m == nil || other == nil {
		return false
	}
	return m.queryID == other.queryID && m.score == other.score
}
