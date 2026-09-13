// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.5.0:
//   lucene/join/src/java/org/apache/lucene/search/join/ScoreMode.java

package join

// ScoreMode describes how to aggregate multiple child hit scores into a single
// parent score.
//
// Mirrors the enum org.apache.lucene.search.join.ScoreMode. The constants are
// declared in the Java declaration order, so their ordinals match Java's.
type ScoreMode int

const (
	// None does no scoring.
	None ScoreMode = iota
	// Avg makes the parent hit's score the average of all child scores.
	Avg
	// Max makes the parent hit's score the max of all child scores.
	Max
	// Total makes the parent hit's score the sum of all child scores.
	Total
	// Min makes the parent hit's score the min of all child scores.
	Min
)

// String mirrors the name of the Java enum constant, which is what
// Enum.toString() returns.
func (m ScoreMode) String() string {
	switch m {
	case None:
		return "None"
	case Avg:
		return "Avg"
	case Max:
		return "Max"
	case Total:
		return "Total"
	case Min:
		return "Min"
	default:
		return "ScoreMode"
	}
}
