// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// MatchAllDocsQuery matches all documents in the index.
type MatchAllDocsQuery struct {
	*BaseQuery
}

// NewMatchAllDocsQuery creates a new MatchAllDocsQuery.
func NewMatchAllDocsQuery() *MatchAllDocsQuery {
	return &MatchAllDocsQuery{
		BaseQuery: &BaseQuery{},
	}
}
