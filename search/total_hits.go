// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// Relation indicates how the total hit count relates to the actual value.
type Relation int

const (
	// EQUAL_TO means the value is exact.
	EQUAL_TO Relation = iota
	// GREATER_THAN_OR_EQUAL_TO means the value is at least the given value.
	GREATER_THAN_OR_EQUAL_TO
)

// TotalHits represents the total number of hits.
type TotalHits struct {
	Value    int64
	Relation Relation
}

// NewTotalHits creates a new TotalHits.
func NewTotalHits(value int64, relation Relation) *TotalHits {
	return &TotalHits{
		Value:    value,
		Relation: relation,
	}
}

// IsExact returns true if the hit count is exact.
func (t *TotalHits) IsExact() bool {
	return t.Relation == EQUAL_TO
}
