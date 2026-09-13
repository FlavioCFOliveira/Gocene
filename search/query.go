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

// Apache Lucene 10.5.0 declares two members on Query that Gocene's Query
// interface does not carry:
//
//	public abstract String toString(String field);
//	public abstract void visit(QueryVisitor visitor);
//
// They are absent because the port is incomplete, not because Lucene lacks
// them. Declaring either on the interface today is measurably net-negative:
// against the current tree, adding ToString costs 255 further compile errors
// and adding Visit costs 151, because the implementors that still lack the
// member outnumber the call sites that want it. Until enough of the query
// tree carries them, the two helpers below render the calls through the
// method set each concrete query actually has — the idiom already used by
// IndriQuery (search/indri_query.go) and ConstantScoreQuery.Visit
// (search/constant_score_query.go).
//
// Both helpers must be withdrawn, and their call sites reduced to plain method
// calls, as soon as the members move onto the Query interface.

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

// visitQuery renders Java's Query.visit(QueryVisitor visitor).
func visitQuery(q Query, visitor QueryVisitor) {
	if q == nil {
		return
	}
	if v, ok := q.(interface{ Visit(QueryVisitor) }); ok {
		v.Visit(visitor)
	}
}
