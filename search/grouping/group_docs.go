// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// Ported from Apache Lucene 10.5.0:
//   lucene/grouping/src/java/org/apache/lucene/search/grouping/GroupDocs.java

// GroupDocs represents one group in the results.
//
// Mirrors the record org.apache.lucene.search.grouping.GroupDocs<T>, whose
// components are score, maxScore, totalHits, scoreDocs, groupValue and
// groupSortValues.
type GroupDocs[T any] struct {
	// Score is the score of this group, or NaN when the score merge mode is
	// None.
	Score float32

	// MaxScore is the max score in this group.
	MaxScore float32

	// TotalHits is the total hits count for this group.
	TotalHits *search.TotalHits

	// ScoreDocs are the hits of this group.
	ScoreDocs []*search.ScoreDoc

	// FieldDocs are the per-hit FieldDocs when the within-group sort is not by
	// relevance, in the same order as ScoreDocs, each embedding the identical
	// *ScoreDoc. In Lucene the scoreDocs array of a field-sorted group holds
	// FieldDoc instances directly; Go's invariant []*ScoreDoc cannot, so the
	// sort values travel alongside, exactly as [search.TopFieldDocs] already
	// renders the same Lucene fact.
	FieldDocs []*search.FieldDoc

	// GroupValue is the value that defines this group.
	GroupValue T

	// GroupSortValues are the sort values used during sorting. They are nil
	// when fillFields=false was passed to the first-pass collector.
	GroupSortValues []any
}

// NewGroupDocs mirrors the canonical GroupDocs record constructor.
func NewGroupDocs[T any](score, maxScore float32, totalHits *search.TotalHits, scoreDocs []*search.ScoreDoc, groupValue T, groupSortValues []any) *GroupDocs[T] {
	return &GroupDocs[T]{
		Score:           score,
		MaxScore:        maxScore,
		TotalHits:       totalHits,
		ScoreDocs:       scoreDocs,
		GroupValue:      groupValue,
		GroupSortValues: groupSortValues,
	}
}
