// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// TopFieldDocs represents the top hits for a field-sorted query, augmenting
// TopDocs with the SortField metadata used to produce the ordering.
//
// Mirrors org.apache.lucene.search.TopFieldDocs.
//
// In Lucene, TopFieldDocs.scoreDocs[i] is itself a FieldDoc carrying the
// per-hit sort values (FieldDoc extends ScoreDoc). Go's TopDocs.ScoreDocs is
// the invariant []*ScoreDoc, which cannot hold *FieldDoc, so the field-sorted
// hits are also surfaced through the parallel FieldDocs slice: FieldDocs[i]
// embeds the same *ScoreDoc as ScoreDocs[i] and adds the Fields values. Callers
// that need the sort keys read FieldDocs; callers that only need doc/score may
// keep using ScoreDocs unchanged.
type TopFieldDocs struct {
	*TopDocs
	// Fields holds the SortField values that produced this result set.
	Fields []*SortField
	// FieldDocs are the per-hit FieldDocs, in the same order as ScoreDocs. Each
	// FieldDocs[i].ScoreDoc is the identical pointer held at ScoreDocs[i].
	FieldDocs []*FieldDoc
}

// NewTopFieldDocs creates a TopFieldDocs result from a slice of *ScoreDoc. When
// the slice elements are the embedded ScoreDocs of FieldDocs, prefer
// NewTopFieldDocsWithFieldDocs so the sort values are retained.
func NewTopFieldDocs(totalHits *TotalHits, scoreDocs []*ScoreDoc, fields []*SortField) *TopFieldDocs {
	return &TopFieldDocs{TopDocs: NewTopDocs(totalHits, scoreDocs), Fields: fields}
}

// NewTopFieldDocsWithFieldDocs creates a TopFieldDocs from the per-hit FieldDocs,
// populating both ScoreDocs (the embedded pointers) and FieldDocs.
func NewTopFieldDocsWithFieldDocs(totalHits *TotalHits, fieldDocs []*FieldDoc, fields []*SortField) *TopFieldDocs {
	scoreDocs := make([]*ScoreDoc, len(fieldDocs))
	for i, fd := range fieldDocs {
		scoreDocs[i] = fd.ScoreDoc
	}
	return &TopFieldDocs{
		TopDocs:   NewTopDocs(totalHits, scoreDocs),
		Fields:    fields,
		FieldDocs: fieldDocs,
	}
}
