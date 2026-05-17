// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// Pruning enumerates the document pruning strategies used by
// LeafFieldComparator implementations.
//
// Mirrors org.apache.lucene.search.Pruning.
type Pruning int

const (
	// PruningNone disables document pruning entirely.
	PruningNone Pruning = iota
	// PruningGreaterThan allows skipping documents that compare strictly better
	// than the current top value (or strictly worse than the bottom value).
	PruningGreaterThan
	// PruningGreaterThanOrEqualTo allows skipping documents that compare better
	// than or equal to the top value (or worse than or equal to the bottom value).
	PruningGreaterThanOrEqualTo
)

// String returns the canonical name of the pruning mode.
func (p Pruning) String() string {
	switch p {
	case PruningNone:
		return "NONE"
	case PruningGreaterThan:
		return "GREATER_THAN"
	case PruningGreaterThanOrEqualTo:
		return "GREATER_THAN_OR_EQUAL_TO"
	default:
		return "UNKNOWN"
	}
}
