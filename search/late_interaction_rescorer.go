// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "sort"

// LateInteractionRescorer rescores the top-N hits from a first-pass query
// using a LateInteractionFloatValuesSource. The combine policy controls what
// happens when a document has no indexed multi-vector value: the default
// drops the score to 0; the fallback variant preserves the first-pass score.
//
// Mirrors org.apache.lucene.search.LateInteractionRescorer.
type LateInteractionRescorer struct {
	src             *LateInteractionFloatValuesSource
	fetchVector     func(docID int) ([][]float32, bool, error)
	fallbackToFirst bool
}

// NewLateInteractionRescorer creates a rescorer wired to a value source.
// fetchVector is called once per top-N hit to retrieve the document's
// multi-vector; returning ok=false signals the value is missing.
func NewLateInteractionRescorer(src *LateInteractionFloatValuesSource, fetchVector func(docID int) ([][]float32, bool, error)) *LateInteractionRescorer {
	return &LateInteractionRescorer{src: src, fetchVector: fetchVector}
}

// WithFallbackToFirstPassScore enables the fallback variant.
func (r *LateInteractionRescorer) WithFallbackToFirstPassScore() *LateInteractionRescorer {
	r.fallbackToFirst = true
	return r
}

// Rescore replaces topDocs.ScoreDocs scores with rescored values and re-sorts
// the slice by descending score.
func (r *LateInteractionRescorer) Rescore(searcher *IndexSearcher, topDocs *TopDocs) (*TopDocs, error) {
	if topDocs == nil || len(topDocs.ScoreDocs) == 0 {
		return topDocs, nil
	}
	out := make([]*ScoreDoc, len(topDocs.ScoreDocs))
	for i, sd := range topDocs.ScoreDocs {
		newScore := float32(0)
		if r.fetchVector != nil {
			vec, ok, err := r.fetchVector(sd.Doc)
			if err != nil {
				return nil, err
			}
			if ok {
				newScore = r.src.Score(vec)
			} else if r.fallbackToFirst {
				newScore = sd.Score
			}
		} else if r.fallbackToFirst {
			newScore = sd.Score
		}
		out[i] = &ScoreDoc{Doc: sd.Doc, Score: newScore, ShardIndex: sd.ShardIndex}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].Doc < out[j].Doc
		}
		return out[i].Score > out[j].Score
	})
	max := float32(0)
	if len(out) > 0 {
		max = out[0].Score
	}
	return &TopDocs{TotalHits: topDocs.TotalHits, ScoreDocs: out, MaxScore: max}, nil
}

// Explain returns the rescored explanation.
func (r *LateInteractionRescorer) Explain(searcher *IndexSearcher, firstPass Explanation, docID int) (Explanation, error) {
	exp := NewExplanation(true, firstPass.GetValue(), "LateInteractionRescorer applied")
	exp.AddDetail(firstPass)
	return exp, nil
}
