// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package monitor is a port of org.apache.lucene.monitor.
//
// The monitor package provides a "percolation" engine: you register queries in
// a Monitor, then pass documents to it and get back the set of registered
// queries that match each document.
package monitor

// QueryMatch represents a match between a registered query and a document.
//
// Derived types may carry additional information (scores, highlights, etc.).
//
// Port of org.apache.lucene.monitor.QueryMatch.
type QueryMatch struct {
	queryID string
}

// NewQueryMatch creates a QueryMatch for the given query ID.
func NewQueryMatch(queryID string) *QueryMatch {
	if queryID == "" {
		panic("queryID must not be empty")
	}
	return &QueryMatch{queryID: queryID}
}

// GetQueryID returns the ID of the query that produced this match.
func (m *QueryMatch) GetQueryID() string { return m.queryID }

// Equals returns true when two QueryMatch values have the same query ID.
func (m *QueryMatch) Equals(other *QueryMatch) bool {
	if m == other {
		return true
	}
	if m == nil || other == nil {
		return false
	}
	return m.queryID == other.queryID
}

// String returns a human-readable representation.
func (m *QueryMatch) String() string { return "Match(query=" + m.queryID + ")" }
