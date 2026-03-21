// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// JoinScoreMode controls how scores are combined/accumulated in join queries.
// This is used by ToParentBlockJoinQuery and ToChildBlockJoinQuery to
// determine how child document scores contribute to parent document scores.
type JoinScoreMode int

const (
	// JoinScoreModeNone means no scoring is done.
	// All documents receive a score of 1.0.
	JoinScoreModeNone JoinScoreMode = iota

	// JoinScoreModeAvg means the score is the average of all child scores.
	JoinScoreModeAvg

	// JoinScoreModeMax means the score is the maximum of all child scores.
	JoinScoreModeMax

	// JoinScoreModeTotal means the score is the sum of all child scores.
	JoinScoreModeTotal

	// JoinScoreModeMin means the score is the minimum of all child scores.
	JoinScoreModeMin
)

// String returns a string representation of the JoinScoreMode.
func (s JoinScoreMode) String() string {
	switch s {
	case JoinScoreModeNone:
		return "None"
	case JoinScoreModeAvg:
		return "Avg"
	case JoinScoreModeMax:
		return "Max"
	case JoinScoreModeTotal:
		return "Total"
	case JoinScoreModeMin:
		return "Min"
	default:
		return "Unknown"
	}
}

// NeedsScores returns true if this score mode requires scoring.
func (s JoinScoreMode) NeedsScores() bool {
	return s != JoinScoreModeNone
}

// CombineScores combines multiple scores into a single score based on the mode.
func (s JoinScoreMode) CombineScores(scores []float32) float32 {
	if len(scores) == 0 {
		return 0
	}

	switch s {
	case JoinScoreModeNone:
		return 1.0

	case JoinScoreModeAvg:
		total := float32(0)
		for _, score := range scores {
			total += score
		}
		return total / float32(len(scores))

	case JoinScoreModeMax:
		max := scores[0]
		for _, score := range scores[1:] {
			if score > max {
				max = score
			}
		}
		return max

	case JoinScoreModeMin:
		min := scores[0]
		for _, score := range scores[1:] {
			if score < min {
				min = score
			}
		}
		return min

	case JoinScoreModeTotal:
		total := float32(0)
		for _, score := range scores {
			total += score
		}
		return total

	default:
		return 1.0
	}
}
