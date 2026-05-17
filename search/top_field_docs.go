// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// TopFieldDocs represents the top hits for a field-sorted query, augmenting
// TopDocs with the SortField metadata used to produce the ordering.
//
// Mirrors org.apache.lucene.search.TopFieldDocs.
type TopFieldDocs struct {
	*TopDocs
	// Fields holds the SortField values that produced this result set.
	Fields []*SortField
}

// NewTopFieldDocs creates a TopFieldDocs result.
func NewTopFieldDocs(totalHits *TotalHits, scoreDocs []*ScoreDoc, fields []*SortField) *TopFieldDocs {
	return &TopFieldDocs{TopDocs: NewTopDocs(totalHits, scoreDocs), Fields: fields}
}
