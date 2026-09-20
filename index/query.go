// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// Query is org.apache.lucene.search.Query.
//
// Apache Lucene 10.5.0 declares exactly one Query class and
// org.apache.lucene.index imports it — BufferedUpdates, FrozenBufferedUpdates,
// DocumentsWriterDeleteQueue and IndexWriter all hold
// org.apache.lucene.search.Query values. This package used to declare a second,
// incompatible Query of its own, so no search query could ever be handed to
// IndexWriter.DeleteDocumentsQuery. There is now one contract: it is declared in
// package spi, which both index and search depend on, and search.Query embeds
// it. See [spi.Query] for why the split falls where it does.
type Query = spi.Query

// MatchAllDocsQuery is NOT org.apache.lucene.search.MatchAllDocsQuery — that
// class is ported in package search. This type exists only as the sentinel
// IndexWriter.DeleteAll pushes through DocumentsWriter.DeleteQueries, and
// IndexWriter.DeleteDocumentsQuery recognises it to reproduce Lucene's
// LUCENE-6379 specialisation:
//
//	for (Query query : queries) {
//	  if (query.getClass() == MatchAllDocsQuery.class) {
//	    return deleteAll();
//	  }
//	}
//
// DIVERGENCE, reported and not repaired here (it is outside the scope of the
// Query unification): Lucene's deleteAll() does not route through
// DocumentsWriter.deleteQueries at all — it takes the full-flush lock and calls
// docWriter.lockAndAbortAll() — and the class the specialisation above compares
// against is org.apache.lucene.search.MatchAllDocsQuery, so a caller passing the
// real search.MatchAllDocsQuery does not currently take the deleteAll path.
type MatchAllDocsQuery struct{}

// Equals renders Query.equals(Object): every MatchAllDocsQuery is equal to
// every other, as in Lucene's `return sameClassAs(o)`.
func (q *MatchAllDocsQuery) Equals(other Query) bool {
	_, ok := other.(*MatchAllDocsQuery)
	return ok
}

// HashCode renders Query.hashCode().
func (q *MatchAllDocsQuery) HashCode() int {
	return 0
}

// TopDocs represents the top-scoring documents.
//
// DIVERGENCE, reported and not repaired here: this is a second rendering of
// [spi.TopDocs] (which search.TopDocs aliases), reachable through the KNN
// vector-search members of CodecReader, FilterLeafReader and friends. Unifying
// it is a separate change from the Query unification and is not attempted here.
type TopDocs struct {
	TotalHits int
	ScoreDocs []ScoreDoc
}

// ScoreDoc represents a scored document. See the note on [TopDocs]: this is a
// second rendering of [spi.ScoreDoc].
type ScoreDoc struct {
	Doc   int
	Score float32
}
