// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// IndexReader is a minimal interface needed by Query.
type IndexReader interface {
	DocCount() int
	NumDocs() int
	MaxDoc() int
}

// Query is the abstract base class for all queries.
//
// The equals/hashCode half of org.apache.lucene.search.Query is declared by
// [spi.Query] and embedded here, because package index holds Query values too
// (BufferedUpdates, FrozenBufferedUpdates, DocumentsWriterDeleteQueue,
// IndexWriter) and Go, unlike Java, cannot let it import this package back.
// There is therefore one Query contract, not two: index.Query is an alias of
// spi.Query, and every search.Query is one.
type Query interface {
	// spi.Query contributes the two members Java's Query.java declares
	// abstract and that need no org.apache.lucene.search type:
	//
	//	@Override public abstract boolean equals(Object obj);
	//	@Override public abstract int hashCode();
	spi.Query

	// Rewrite expert: called to re-write queries into primitive queries.
	// Mirrors Query.rewrite(IndexSearcher) of Apache Lucene 10.5.0.
	Rewrite(searcher *IndexSearcher) (Query, error)
	// CreateWeight expert: constructs an appropriate Weight implementation for
	// this query. Mirrors Query.createWeight(IndexSearcher, ScoreMode, float).
	CreateWeight(searcher *IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error)
	// Visit recurses through the query tree, visiting any child queries.
	// Mirrors Query.visit(QueryVisitor) of Apache Lucene 10.5.0.
	Visit(visitor QueryVisitor)
}

// RewriteMethod and MultiTermQuery are declared by MultiTermQuery.java, not by
// Query.java: RewriteMethod is the nested class MultiTermQuery.RewriteMethod.
// Both live in multi_term_query.go.

// scoreModeWeightCreator is a Gocene-only interface with no counterpart in
// Apache Lucene 10.5.0, where Query.createWeight(IndexSearcher, ScoreMode,
// float) already carries the full ScoreMode enum (COMPLETE /
// COMPLETE_NO_SCORES / TOP_SCORES / TOP_DOCS / TOP_DOCS_WITH_SCORES).
//
// It was introduced while Query.CreateWeight still collapsed that enum to a
// needsScores bool, so that composite queries (BooleanQuery,
// ConstantScoreQuery) could still forward a precise mode to their children.
// Query.CreateWeight now takes the ScoreMode itself, so this interface is
// redundant; it is retained only because a number of queries and test wrappers
// still declare CreateWeightScoreMode, and IndexSearcher never depends on it.
type scoreModeWeightCreator interface {
	// CreateWeightScoreMode builds a Weight for the given full ScoreMode.
	CreateWeightScoreMode(searcher *IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error)
}

// BaseQuery provides common functionality for queries.
type BaseQuery struct{}

func (q *BaseQuery) Rewrite(searcher *IndexSearcher) (Query, error) { return q, nil }
func (q *BaseQuery) Equals(other spi.Query) bool                    { return false }
func (q *BaseQuery) HashCode() int                                  { return 0 }
func (q *BaseQuery) CreateWeight(searcher *IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error) {
	return nil, nil
}

// Visit renders `public abstract void visit(QueryVisitor visitor)` of
// Query.java:93. Java declares it abstract, so no Query instance can reach a
// body here: a concrete subclass that omitted it would not compile. Go cannot
// express that, because BaseQuery must satisfy Query for Query.rewrite's
// `return this` (Query.java:82) to type-check, so the abstract declaration is
// rendered as a panic rather than as a silent no-op. A no-op would be a
// fabricated traversal — it would report a query tree with no terms and no
// leaves, which Lucene never does.
func (q *BaseQuery) Visit(visitor QueryVisitor) {
	panic("search: Query.visit(QueryVisitor) is abstract in Apache Lucene 10.5.0 (Query.java:93); the concrete query must declare Visit")
}

// Apache Lucene 10.5.0 declares one further member on Query that Gocene's
// Query interface does not carry:
//
//	public abstract String toString(String field);
//
// It is absent because the port is incomplete, not because Lucene lacks it:
// against the current tree, adding ToString costs 255 further compile errors,
// because the implementors that still lack the member outnumber the call sites
// that want it. Until enough of the query tree carries it, the helper below
// renders the call through the method set each concrete query actually has.
//
// visit(QueryVisitor) was the other such member. It now sits on the Query
// interface above, so its shim (visitQuery) has been withdrawn and its call
// sites reduced to plain q.Visit(visitor) calls, exactly as the note that
// stood here required.
//
// queryToString must be withdrawn in the same way, and its call sites reduced
// to plain method calls, as soon as ToString moves onto the Query interface.

// queryToString renders Java's Query.toString(String field). Java's
// no-argument Query.toString() is toString("") and is spelled here as
// queryToString(q, "").
func queryToString(q Query, field string) string {
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
