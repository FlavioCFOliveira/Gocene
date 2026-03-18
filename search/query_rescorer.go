// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// QueryRescorer re-scores documents using a secondary query.
// This is the Go port of Lucene's org.apache.lucene.search.QueryRescorer.
type QueryRescorer struct {
	query Query
}

// NewQueryRescorer creates a new QueryRescorer.
func NewQueryRescorer(query Query) *QueryRescorer {
	return &QueryRescorer{query: query}
}

// GetQuery returns the rescore query.
func (r *QueryRescorer) GetQuery() Query {
	return r.query
}

// Rescore re-scores the top documents.
func (r *QueryRescorer) Rescore(searcher *IndexSearcher, topDocs *TopDocs) (*TopDocs, error) {
	// For now, return the original topDocs
	// In a full implementation, this would:
	// 1. Create a Weight for the rescore query
	// 2. Score each document in the topDocs
	// 3. Combine the original score with the rescore score
	// 4. Re-sort the documents
	return topDocs, nil
}

// Ensure QueryRescorer implements Rescorer
var _ Rescorer = (*QueryRescorer)(nil)
