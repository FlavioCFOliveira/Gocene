// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

// Query is org.apache.lucene.search.Query.
//
// PORT NOTE: Apache Lucene 10.5.0 declares exactly one Query class, in
// org.apache.lucene.search, and org.apache.lucene.index imports it directly —
// BufferedUpdates, FrozenBufferedUpdates, DocumentsWriterDeleteQueue and
// IndexWriter all hold org.apache.lucene.search.Query values. Java tolerates
// that because its packages do not form an import cycle; Go's do, because
// package search imports package index. This is the same problem
// [IndexSearcher] in this package solves, and it is solved the same way: the
// members of Query that do not themselves need a search-only type are declared
// here, in the package both sides already depend on, and *every* concrete
// org.apache.lucene.search.Query satisfies them structurally.
//
// The contract carries exactly the two members Java's Query.java declares
// abstract that are expressible without naming a search type:
//
//	@Override public abstract boolean equals(Object obj);
//	@Override public abstract int hashCode();
//
// Query.java's other members — toString(String), visit(QueryVisitor),
// rewrite(IndexSearcher) and createWeight(IndexSearcher, ScoreMode, float) —
// name IndexSearcher, ScoreMode, Weight and QueryVisitor, which are
// org.apache.lucene.search classes. They stay declared on search.Query, in the
// package Lucene declares them in; search.Query embeds this interface, so there
// is one contract and not two, and a search query is an index query by
// construction rather than by coincidence.
//
// Java's equals takes Object because it overrides Object.equals. Gocene's
// rendering narrows the parameter to Query, which is the type every Gocene
// implementation already asserts on; that narrowing is pre-existing and
// unchanged here.
type Query interface {
	// Equals renders `public abstract boolean equals(Object obj)`.
	Equals(other Query) bool

	// HashCode renders `public abstract int hashCode()`.
	HashCode() int
}
