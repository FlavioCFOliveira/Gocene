// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/FieldDoc.java

import "fmt"

// FieldDoc is a ScoreDoc that also carries sort field values for the document.
//
// In addition to the document number and score, FieldDoc contains an array of
// values for the document from the field(s) used to sort.  For example, if the
// sort criteria was "a", "b", "c", the Fields slice will have three elements
// corresponding to the term values for the document in those fields.  Each
// element is typed according to the field's SortField.Type (int32, float32,
// string, etc.).
//
// Ported from org.apache.lucene.search.FieldDoc.
type FieldDoc struct {
	*ScoreDoc

	// Fields holds the sort key values for this document, in the same order
	// as the SortField slice passed to Sort.  Each element type matches the
	// FieldComparator used for that sort field.
	Fields []any
}

// NewFieldDoc creates a FieldDoc with empty sort information.
func NewFieldDoc(doc int, score float32) *FieldDoc {
	return &FieldDoc{
		ScoreDoc: NewScoreDoc(doc, score, -1),
	}
}

// NewFieldDocWithFields creates a FieldDoc with sort field values.
func NewFieldDocWithFields(doc int, score float32, fields []any) *FieldDoc {
	return &FieldDoc{
		ScoreDoc: NewScoreDoc(doc, score, -1),
		Fields:   fields,
	}
}

// NewFieldDocWithShard creates a FieldDoc with sort field values and a shard index.
func NewFieldDocWithShard(doc int, score float32, fields []any, shardIndex int) *FieldDoc {
	return &FieldDoc{
		ScoreDoc: NewScoreDoc(doc, score, shardIndex),
		Fields:   fields,
	}
}

// String returns a human-readable representation of this FieldDoc.
func (f *FieldDoc) String() string {
	return fmt.Sprintf("%s fields=%v", scoreDocString(f.ScoreDoc), f.Fields)
}

// scoreDocString formats a ScoreDoc for inclusion in FieldDoc.String().
func scoreDocString(s *ScoreDoc) string {
	return fmt.Sprintf("doc=%d score=%g shardIndex=%d", s.Doc, s.Score, s.ShardIndex)
}
