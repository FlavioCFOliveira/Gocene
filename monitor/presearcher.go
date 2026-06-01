// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

import (
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Presearcher reduces the number of queries run against each document by
// first selecting a candidate set from the query index.
//
// Port of org.apache.lucene.monitor.Presearcher.
//
// Deviation: LeafReader / Document from Gocene's index/document packages are
// referenced by interface{} placeholders until those packages fully stabilise.
// Full integration is deferred to backlog #2693.
type Presearcher interface {
	// BuildQuery returns a query to run over the Monitor's query index, given a
	// set of documents to monitor.
	//
	// termAcceptor returns true when a (field, term) pair should be included.
	BuildQuery(reader interface{}, termAcceptor func(field string, term *util.BytesRef) bool) search.Query

	// IndexQuery returns a Document to index in the Monitor's query index for
	// the given query and metadata.
	IndexQuery(query search.Query, metadata map[string]string) interface{}
}

// NoFilteringPresearcher is a Presearcher that runs all registered queries without
// pre-filtering.  Matches Java's Presearcher.NO_FILTERING.
type NoFilteringPresearcher struct{}

// NoFiltering is the default no-op presearcher.
var NoFiltering Presearcher = &NoFilteringPresearcher{}

// BuildQuery returns MatchAllDocsQuery so that all stored queries are candidates.
func (p *NoFilteringPresearcher) BuildQuery(_ interface{}, _ func(string, *util.BytesRef) bool) search.Query {
	return search.NewMatchAllDocsQuery()
}

// IndexQuery returns an empty document.
func (p *NoFilteringPresearcher) IndexQuery(_ search.Query, _ map[string]string) interface{} {
	return nil
}
