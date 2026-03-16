// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// Matches represents the matching positions for a document and a query.
// This is the Go port of Lucene's org.apache.lucene.search.Matches.
type Matches interface {
	// GetQuery returns the query that produced these matches.
	GetQuery() Query

	// GetDocID returns the document ID for these matches.
	GetDocID() int

	// GetSubMatches returns sub-matches (for nested queries).
	GetSubMatches() []Matches
}

// BaseMatches provides a basic implementation of Matches.
type BaseMatches struct {
	query Query
	docID int
}

// NewBaseMatches creates a new BaseMatches.
func NewBaseMatches(query Query, docID int) *BaseMatches {
	return &BaseMatches{query: query, docID: docID}
}

// GetQuery returns the query that produced these matches.
func (m *BaseMatches) GetQuery() Query {
	return m.query
}

// GetDocID returns the document ID for these matches.
func (m *BaseMatches) GetDocID() int {
	return m.docID
}

// GetSubMatches returns sub-matches (for nested queries).
func (m *BaseMatches) GetSubMatches() []Matches {
	return nil
}

// Ensure BaseMatches implements Matches
var _ Matches = (*BaseMatches)(nil)
