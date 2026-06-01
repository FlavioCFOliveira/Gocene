// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search_test

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestTermScorerGetMaxScoreIsUpperBound proves the rmp #129 acceptance
// criterion: a TermQuery scorer's GetMaxScore(upTo) is a true upper bound on the
// score of every matching document in [current, upTo].
//
// It builds a small multi-document index in which the term "alpha" occurs with
// varying frequencies (so per-document TF/IDF scores differ and exceed 1.0) and
// only in a strict subset of the documents (so idf > 0). It then pulls a real
// TermScorer for the single leaf and, walking every matching document, checks
// two things at every position:
//
//   - GetMaxScore(upTo) >= Score() for the current document, and
//   - GetMaxScore(NO_MORE_DOCS) >= the maximum score observed across ALL
//     matching documents (the global block-max bound covering every block).
//
// The scorer reports a single block (SlowImpactsEnum fallback) whose bound is
// the global maximum, so it must dominate every individual document score; this
// is the conservative-yet-correct Lucene fallback the task contemplates.
func TestTermScorerGetMaxScoreIsUpperBound(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	// Document term-frequency plan for "alpha". Documents that do not contain
	// "alpha" carry "beta" instead, keeping docFreq("alpha") < maxDoc so that
	// idf = log(maxDoc/docFreq) is strictly positive and scores vary per doc.
	// Multiple distinct, non-trivial frequencies exercise the per-document
	// branch of the TF computation (tf = sqrt(freq)).
	alphaFreqs := []int{1, 5, 2, 9, 3, 7, 4, 6, 8, 1, 5, 3, 10, 2, 6}
	totalDocs := len(alphaFreqs) + 5 // 5 "beta-only" docs to keep idf > 0

	for _, freq := range alphaFreqs {
		doc := document.NewDocument()
		text := strings.TrimSpace(strings.Repeat("alpha ", freq))
		field, err := document.NewTextField("content", text, true)
		if err != nil {
			t.Fatalf("NewTextField: %v", err)
		}
		doc.Add(field)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	for i := 0; i < 5; i++ {
		doc := document.NewDocument()
		field, err := document.NewTextField("content", "beta", true)
		if err != nil {
			t.Fatalf("NewTextField: %v", err)
		}
		doc.Add(field)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)
	query := search.NewTermQuery(index.NewTerm("content", "alpha"))

	// needsScores = true so the TermWeight wires a real SimScorer and the
	// TermScorer builds its MaxScoreCache.
	weight, err := query.CreateWeight(searcher, true, 1.0)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}

	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("Leaves: %v", err)
	}
	if len(leaves) == 0 {
		t.Fatalf("expected at least one leaf, got none")
	}

	const noMoreDocs = search.NO_MORE_DOCS

	// Collect every (doc, score) pair across all leaves first; we will then
	// re-pull scorers to validate the block-max invariant against these values.
	type docScore struct {
		doc   int
		score float32
	}

	var (
		all          []docScore
		globalMax    float32
		sawAboveOne  bool
		matchedAny   bool
		checkedUpper int
	)

	for _, leaf := range leaves {
		scorer, err := weight.Scorer(leaf)
		if err != nil {
			t.Fatalf("Scorer: %v", err)
		}
		if scorer == nil {
			continue // term absent from this leaf
		}

		// The global bound must dominate every score the scorer produces.
		globalBound := scorer.GetMaxScore(noMoreDocs)

		var leafScores []docScore
		for {
			doc, err := scorer.NextDoc()
			if err != nil {
				t.Fatalf("NextDoc: %v", err)
			}
			if doc == noMoreDocs {
				break
			}
			matchedAny = true
			score := scorer.Score()
			leafScores = append(leafScores, docScore{doc: doc, score: score})

			if score > 1.0 {
				sawAboveOne = true
			}
			if score > globalMax {
				globalMax = score
			}

			// Per-document invariants against the current scorer position.
			if got := scorer.GetMaxScore(noMoreDocs); got < score {
				t.Errorf("doc %d: GetMaxScore(NO_MORE_DOCS)=%v < score=%v", doc, got, score)
			}
			if globalBound < score {
				t.Errorf("doc %d: global bound %v < score %v (bound must not shrink below any doc score)",
					doc, globalBound, score)
			}
			checkedUpper++
		}
		all = append(all, leafScores...)
	}

	if !matchedAny {
		t.Fatalf("expected the TermQuery to match documents, but none did")
	}
	if !sawAboveOne {
		t.Fatalf("test is not exercising the >1.0 path: max observed score was %v; "+
			"the stub GetMaxScore==1.0 would pass vacuously", globalMax)
	}
	if checkedUpper == 0 {
		t.Fatalf("no documents were checked against GetMaxScore")
	}

	// Second pass: for every prefix upTo (an actual matching doc id), the
	// block-max bound must dominate every score at or before that doc. With the
	// SlowImpactsEnum fallback the bound is constant (global), so this is the
	// strongest property the fallback guarantees; it must still never be below
	// any in-range score. This is the literal "GetMaxScore(upTo) >= every doc
	// score in [current, upTo]" acceptance check, evaluated for upTo == each
	// matching doc id and for the multi-block-covering NO_MORE_DOCS.
	for _, leaf := range leaves {
		scorer, err := weight.Scorer(leaf)
		if err != nil {
			t.Fatalf("Scorer (2nd pass): %v", err)
		}
		if scorer == nil {
			continue
		}
		var seen []docScore
		for {
			doc, err := scorer.NextDoc()
			if err != nil {
				t.Fatalf("NextDoc (2nd pass): %v", err)
			}
			if doc == noMoreDocs {
				break
			}
			seen = append(seen, docScore{doc: doc, score: scorer.Score()})

			bound := scorer.GetMaxScore(doc) // upTo = current doc id
			for _, ds := range seen {
				if bound < ds.score {
					t.Errorf("upTo=%d: GetMaxScore(%d)=%v < score(doc %d)=%v",
						doc, doc, bound, ds.doc, ds.score)
				}
			}
		}
	}

	// AdvanceShallow must return a legal, non-decreasing block boundary and
	// remain consistent with GetMaxScore (it must not strand a competitive
	// document). For the fallback it returns NO_MORE_DOCS (one block).
	for _, leaf := range leaves {
		scorer, err := weight.Scorer(leaf)
		if err != nil {
			t.Fatalf("Scorer (shallow pass): %v", err)
		}
		if scorer == nil {
			continue
		}
		if _, err := scorer.NextDoc(); err != nil {
			t.Fatalf("NextDoc (shallow pass): %v", err)
		}
		upTo, err := scorer.AdvanceShallow(0)
		if err != nil {
			t.Fatalf("AdvanceShallow: %v", err)
		}
		if upTo < 0 {
			t.Errorf("AdvanceShallow(0) returned negative block end %d", upTo)
		}
		// The bound computed for the advanced block must dominate the current
		// document score.
		if bound, score := scorer.GetMaxScore(upTo), scorer.Score(); bound < score {
			t.Errorf("after AdvanceShallow: GetMaxScore(%d)=%v < score=%v", upTo, bound, score)
		}
	}

	t.Logf("checked %d matching documents; global max score = %v (> 1.0 confirmed)", checkedUpper, globalMax)
	_ = totalDocs
}
