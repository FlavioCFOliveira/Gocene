// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

// Explanation is a placeholder for the Lucene Explanation type.
// A full port is deferred until search.Explanation is available in Gocene.
//
// Deviation from Java: Gocene does not yet have a native Explanation type.
// ExplainingMatch carries a string representation of the explanation instead.
type Explanation struct {
	// IsMatch indicates whether the explanation represents a match.
	IsMatch bool
	// Description is a human-readable explanation.
	Description string
	// Value is the score value of the explanation.
	Value float64
}

// ExplainingMatch extends QueryMatch with the score explanation.
//
// Port of org.apache.lucene.monitor.ExplainingMatch.
type ExplainingMatch struct {
	QueryMatch
	explanation *Explanation
}

// NewExplainingMatch creates an ExplainingMatch for the given query ID and explanation.
func NewExplainingMatch(queryID string, explanation *Explanation) *ExplainingMatch {
	return &ExplainingMatch{
		QueryMatch:  QueryMatch{queryID: queryID},
		explanation: explanation,
	}
}

// GetExplanation returns the explanation for this match.
func (m *ExplainingMatch) GetExplanation() *Explanation { return m.explanation }

// Equals returns true when both query IDs and explanations are equal.
func (m *ExplainingMatch) Equals(other *ExplainingMatch) bool {
	if m == other {
		return true
	}
	if m == nil || other == nil {
		return false
	}
	if m.queryID != other.queryID {
		return false
	}
	if m.explanation == other.explanation {
		return true
	}
	if m.explanation == nil || other.explanation == nil {
		return false
	}
	return m.explanation.IsMatch == other.explanation.IsMatch &&
		m.explanation.Description == other.explanation.Description &&
		m.explanation.Value == other.explanation.Value
}
