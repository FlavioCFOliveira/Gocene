// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// IndexReader is a minimal interface needed by Query.
type IndexReader interface {
	DocCount() int
	NumDocs() int
	MaxDoc() int
}

// Query is the abstract base class for all queries.
type Query interface {
	// Rewrite rewrites the query to a simpler form.
	Rewrite(reader IndexReader) (Query, error)
	// Clone creates a copy of this query.
	Clone() Query
	// Equals checks if this query equals another.
	Equals(other Query) bool
	// HashCode returns a hash code for this query.
	HashCode() int
}

// BaseQuery provides common functionality for queries.
type BaseQuery struct{}

func (q *BaseQuery) Rewrite(reader IndexReader) (Query, error) { return q, nil }
func (q *BaseQuery) Clone() Query                               { return q }
func (q *BaseQuery) Equals(other Query) bool                    { return false }
func (q *BaseQuery) HashCode() int                             { return 0 }
