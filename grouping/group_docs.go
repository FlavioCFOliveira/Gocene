// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// GroupDocs represents one group in the results.
//
// Mirrors the record org.apache.lucene.search.grouping.GroupDocs<T>. A Java
// record component is both a field and an accessor of the same name; Go
// forbids that pair, so the components are rendered as exported fields.
//
// lucene.experimental
type GroupDocs[T any] struct {
	// Score is the overall aggregated score of this group (currently only set
	// by join queries).
	Score float32

	// MaxScore is the max score in this group.
	MaxScore float32

	// TotalHits is the total hits within this group.
	TotalHits *search.TotalHits

	// ScoreDocs are the hits; these may be search.FieldDoc instances if the
	// withinGroupSort sorted by fields.
	ScoreDocs []*search.ScoreDoc

	// GroupValue is the groupField value for all docs in this group; this may
	// be null if hits did not have the groupField.
	GroupValue T

	// GroupSortValues matches the groupSort passed to
	// FirstPassGroupingCollector.
	GroupSortValues []any
}

// NewGroupDocs mirrors the canonical constructor of the record GroupDocs<T>.
func NewGroupDocs[T any](
	score float32,
	maxScore float32,
	totalHits *search.TotalHits,
	scoreDocs []*search.ScoreDoc,
	groupValue T,
	groupSortValues []any,
) *GroupDocs[T] {
	return &GroupDocs[T]{
		Score:           score,
		MaxScore:        maxScore,
		TotalHits:       totalHits,
		ScoreDocs:       scoreDocs,
		GroupValue:      groupValue,
		GroupSortValues: groupSortValues,
	}
}
