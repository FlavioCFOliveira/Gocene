// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/spans/SpanQuery.java

package spans

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// SpanQuery is the base interface for span-based queries.
//
// Mirrors org.apache.lucene.queries.spans.SpanQuery (abstract class).
//
// Deviations from Java:
//   - Java's abstract createWeight signature returns ScoreMode; Gocene uses bool
//     needsScores + float32 boost to match Gocene's search.Query.CreateWeight.
//   - The static GetTermStates helpers are package-level functions here.
type SpanQuery interface {
	search.Query

	// GetField returns the name of the field matched by this query.
	GetField() string

	// CreateSpanWeight creates a SpanWeight for this query.
	// searcher may be nil in tests (scoring will be disabled).
	CreateSpanWeight(searcher *search.IndexSearcher, needsScores bool, boost float32) (*SpanWeight, error)
}

// GetTermStates builds a map of terms to *index.TermStates from a set of SpanWeights.
// Mirrors org.apache.lucene.queries.spans.SpanQuery.getTermStates (static).
func GetTermStates(weights ...*SpanWeight) map[string]*index.TermStates {
	terms := make(map[string]*index.TermStates)
	for _, w := range weights {
		w.ExtractTermStates(terms)
	}
	return terms
}

// GetTermStatesFromSlice builds a map of terms to *index.TermStates from a slice.
// Mirrors org.apache.lucene.queries.spans.SpanQuery.getTermStates (Collection overload).
func GetTermStatesFromSlice(weights []*SpanWeight) map[string]*index.TermStates {
	terms := make(map[string]*index.TermStates)
	for _, w := range weights {
		w.ExtractTermStates(terms)
	}
	return terms
}
