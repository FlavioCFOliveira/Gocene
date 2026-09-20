// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/spans/SpanQuery.java

package spans

import (
	"math/bits"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// SpanQuery is the base interface for span-based queries.
//
// Mirrors org.apache.lucene.queries.spans.SpanQuery (abstract class).
//
// Deviations from Java:
//   - Java declares a single covariant createWeight(IndexSearcher, ScoreMode,
//     float) returning SpanWeight. Go has no covariant return types, so the
//     method is split in two: CreateWeight satisfies search.Query and returns
//     search.Weight, while CreateSpanWeight carries the covariant *SpanWeight
//     return. Both take the same (IndexSearcher, ScoreMode, float32) parameters
//     as Java's createWeight.
//   - The static GetTermStates helpers are package-level functions here.
type SpanQuery interface {
	search.Query

	// GetField returns the name of the field matched by this query.
	GetField() string

	// CreateSpanWeight creates a SpanWeight for this query.
	// searcher may be nil in tests (scoring will be disabled).
	CreateSpanWeight(searcher *search.IndexSearcher, scoreMode search.ScoreMode, boost float32) (*SpanWeight, error)

	// ToString prints a query to a string, with field assumed to be the
	// default field and omitted.
	//
	// In Apache Lucene 10.5.0 `SpanQuery extends Query` and inherits the
	// abstract Query.toString(String). Gocene's search.Query interface does
	// not declare it (see the note at search/query.go:72-89), so SpanQuery
	// restates the inherited member for the span family, which composes its
	// own toString out of its clauses'.
	ToString(field string) string
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

// Apache Lucene 10.5.0 declares two members on Query that Gocene's search.Query
// interface does not yet carry — `public abstract String toString(String
// field)` and `public abstract void visit(QueryVisitor visitor)` — for the
// reason recorded at search/query.go:72-89: against the current tree, putting
// either on the interface costs more compile errors than it removes. The two
// helpers below are this package's copies of search's queryToString and
// visitQuery: they render the call through the method set the concrete query
// actually has, and must be withdrawn once the members move onto the
// interface.

// spanQueryToString renders Java's Query.toString(String field). Java's
// no-argument Query.toString() is toString(""), spelled here as
// spanQueryToString(q, "").
func spanQueryToString(q SpanQuery, field string) string {
	if q == nil {
		return ""
	}
	if ts, ok := q.(interface{ ToString(string) string }); ok {
		return ts.ToString(field)
	}
	if s, ok := q.(interface{ String(string) string }); ok {
		return s.String(field)
	}
	if s, ok := q.(interface{ String() string }); ok {
		return s.String()
	}
	return ""
}

// visitSpanQuery renders Java's Query.visit(QueryVisitor visitor).
func visitSpanQuery(q SpanQuery, visitor search.QueryVisitor) {
	if q == nil {
		return
	}
	if v, ok := q.(interface{ Visit(search.QueryVisitor) }); ok {
		v.Visit(visitor)
	}
}

// rotateLeft32 renders java.lang.Integer.rotateLeft(int, int), which every
// Query.hashCode() in the spans package folds its class hash through. Gocene's
// int is 64 bits wide, so the rotation is performed on the low 32 bits and
// sign-extended back, exactly reproducing the Java result.
func rotateLeft32(value int, distance int) int {
	return int(int32(bits.RotateLeft32(uint32(int32(value)), distance)))
}

// javaListHashCode renders java.util.List.hashCode() over a list of
// SpanQuery — `int h = 1; for (e : list) h = 31 * h + e.hashCode();` — with
// Java's 32-bit wraparound, which SpanOrQuery.hashCode() folds its clauses
// through.
func javaListHashCode(clauses []SpanQuery) int {
	h := int32(1)
	for _, c := range clauses {
		h = 31*h + int32(c.HashCode())
	}
	return int(h)
}

// javaStringHashCode renders java.lang.String.hashCode() —
// `s[0]*31^(n-1) + s[1]*31^(n-2) + ... + s[n-1]` over the UTF-16 code units of
// the string, with Java's 32-bit wraparound. Every Java identifier Gocene
// mirrors is ASCII, for which a Go string's bytes and Java's UTF-16 code units
// coincide.
func javaStringHashCode(s string) int {
	h := int32(0)
	for _, c := range []byte(s) {
		h = 31*h + int32(c)
	}
	return int(h)
}
