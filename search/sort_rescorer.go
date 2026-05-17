// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "sort"

// SortRescorer rescores top hits by re-sorting them using a Sort instance.
// It is the Lucene-canonical implementation used when the second-pass ranking
// is purely about reordering, without recomputing per-doc scores.
//
// Mirrors org.apache.lucene.search.SortRescorer.
type SortRescorer struct {
	sort *Sort
}

// NewSortRescorer creates a SortRescorer bound to sort.
func NewSortRescorer(sort *Sort) *SortRescorer {
	if sort == nil {
		panic("SortRescorer: sort is required")
	}
	return &SortRescorer{sort: sort}
}

// Sort returns the configured Sort.
func (r *SortRescorer) Sort() *Sort { return r.sort }

// Rescore re-sorts topDocs.ScoreDocs based on r.sort. Score values are left
// untouched; only the document order changes (matching Lucene's SortRescorer
// behaviour when the sort fields are independent of score).
func (r *SortRescorer) Rescore(searcher *IndexSearcher, topDocs *TopDocs) (*TopDocs, error) {
	if topDocs == nil || len(topDocs.ScoreDocs) <= 1 {
		return topDocs, nil
	}
	docs := append([]*ScoreDoc(nil), topDocs.ScoreDocs...)
	sort.SliceStable(docs, func(i, j int) bool {
		for _, f := range r.sort.Fields {
			cmp := compareForSortField(docs[i], docs[j], f)
			if cmp != 0 {
				if f.Reverse {
					return cmp > 0
				}
				return cmp < 0
			}
		}
		return false
	})
	return &TopDocs{TotalHits: topDocs.TotalHits, ScoreDocs: docs, MaxScore: topDocs.MaxScore}, nil
}

// Explain returns an explanation for the rescored doc id. Without per-doc
// field-value access at this level, the result is a passthrough of the inner
// explanation marked as a rescore step.
func (r *SortRescorer) Explain(searcher *IndexSearcher, firstPass Explanation, docID int) (Explanation, error) {
	exp := NewExplanation(true, firstPass.GetValue(), "SortRescorer applied")
	exp.AddDetail(firstPass)
	return exp, nil
}

func compareForSortField(a, b *ScoreDoc, f *SortField) int {
	switch f.Type {
	case SortFieldTypeScore:
		if a.Score == b.Score {
			return 0
		}
		if a.Score < b.Score {
			return -1
		}
		return 1
	case SortFieldTypeDoc:
		if a.Doc == b.Doc {
			return 0
		}
		if a.Doc < b.Doc {
			return -1
		}
		return 1
	default:
		// Without per-doc field accessor wired into ScoreDoc, leave ties.
		return 0
	}
}
