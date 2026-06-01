// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestMultiTermQueryRewrites.java
//
// These tests verify that a MultiTermQuery rewrites to the same Query whether it
// is rewritten against a single segment, a multi-segment MultiReader, or a
// multi-segment MultiReader containing duplicate terms — with the rewritten
// boolean clauses in sorted term order and with the correct per-term boosts —
// across every selectable rewrite method (SCORING_BOOLEAN_REWRITE,
// CONSTANT_SCORE_BOOLEAN_REWRITE, TopTermsScoringBooleanQueryRewrite,
// TopTermsBoostOnlyBooleanQueryRewrite), and that the maxClauseCount limit is
// enforced (or deliberately not enforced) per rewrite method.
//
// This requires API surface that is not yet ported in Gocene:
//   - IndexSearcher.rewrite(Query) (the rewrite-to-convergence entry point that
//     these tests call directly and compare the results of);
//   - the selectable MultiTermQuery.RewriteMethod objects
//     (SCORING_BOOLEAN_REWRITE, CONSTANT_SCORE_BOOLEAN_REWRITE,
//     TopTermsScoringBooleanQueryRewrite, TopTermsBoostOnlyBooleanQueryRewrite)
//     passed to TermRangeQuery.newStringRange and to a custom MultiTermQuery;
//   - the custom MultiTermQuery subclass with a FilteredTermsEnum + BoostAttribute
//     used by checkBoosts.
//
// The tests build the same multi-reader corpus the reference uses and assert the
// cross-reader rewrite equivalence, so they fail honestly until the rewrite
// surface above is ported (rather than being skipped or weakened).

package search_test

import (
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"

	_ "github.com/FlavioCFOliveira/Gocene/codecs"
)

// mtqrReaders builds the three readers the reference uses: a single-segment
// reader over docs 0..9, a multi-segment MultiReader over the even/odd split,
// and a multi-segment MultiReader that additionally duplicates the full set.
func mtqrReaders(t *testing.T) (single, multi, multiDupls index.IndexReaderInterface, cleanup func()) {
	t.Helper()

	build := func(values []int) (store.Directory, *index.DirectoryReader) {
		dir := store.NewByteBuffersDirectory()
		w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(nil))
		if err != nil {
			t.Fatalf("NewIndexWriter: %v", err)
		}
		for _, v := range values {
			doc := document.NewDocument()
			f, ferr := document.NewStringField("data", strconv.Itoa(v), false)
			if ferr != nil {
				t.Fatalf("NewStringField: %v", ferr)
			}
			doc.Add(f)
			if aerr := w.AddDocument(doc); aerr != nil {
				t.Fatalf("AddDocument: %v", aerr)
			}
		}
		if err := w.ForceMerge(1); err != nil {
			t.Fatalf("ForceMerge: %v", err)
		}
		if err := w.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
		r, err := index.OpenDirectoryReader(dir)
		if err != nil {
			t.Fatalf("OpenDirectoryReader: %v", err)
		}
		return dir, r
	}

	all := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	even := []int{0, 2, 4, 6, 8}
	odd := []int{1, 3, 5, 7, 9}

	dDir, dReader := build(all)
	eDir, eReader := build(even)
	oDir, oReader := build(odd)

	mr, err := index.NewMultiReader([]index.IndexReaderInterface{eReader, oReader})
	if err != nil {
		t.Fatalf("NewMultiReader: %v", err)
	}
	// Reuse a fresh even reader for the duplicates multi-reader.
	e2Dir, e2Reader := build(even)
	mrDupls, err := index.NewMultiReader([]index.IndexReaderInterface{e2Reader, dReader})
	if err != nil {
		t.Fatalf("NewMultiReader(dupls): %v", err)
	}

	cleanup = func() {
		_ = mrDupls.Close()
		_ = mr.Close()
		_ = dReader.Close()
		_ = dDir.Close()
		_ = eDir.Close()
		_ = oDir.Close()
		_ = e2Dir.Close()
	}
	return dReader, mr, mrDupls, cleanup
}

// rewriteToConvergence loops Query.Rewrite over the reader to a fixed point, the
// closest Gocene analogue of IndexSearcher.rewrite available without the
// clause-count visitor walk.
func rewriteToConvergence(t *testing.T, q search.Query, reader index.IndexReaderInterface) search.Query {
	t.Helper()
	current := q
	for {
		next, err := current.Rewrite(reader)
		if err != nil {
			t.Fatalf("Rewrite: %v", err)
		}
		if next == current {
			return current
		}
		current = next
	}
}

// TestMultiTermQueryRewrites_RewritesWithDuplicateTerms ports
// testRewritesWithDuplicateTerms.
func TestMultiTermQueryRewrites_RewritesWithDuplicateTerms(t *testing.T) {
	single, multi, multiDupls, cleanup := mtqrReaders(t)
	defer cleanup()

	mtq := search.NewTermRangeQueryWithStrings("data", "2", "7", true, true)
	q1 := rewriteToConvergence(t, mtq, single)
	q2 := rewriteToConvergence(t, mtq, multi)
	q3 := rewriteToConvergence(t, mtq, multiDupls)

	if !q1.Equals(q2) {
		t.Errorf("the multi-segment case must produce the same rewritten query as the single-segment case: " +
			"this requires the selectable MultiTermQuery.RewriteMethod surface and IndexSearcher.rewrite, " +
			"which are not yet ported in Gocene")
	}
	if !q1.Equals(q3) {
		t.Errorf("the multi-segment-with-duplicates case must produce the same rewritten query as the " +
			"single-segment case: requires the selectable MultiTermQuery.RewriteMethod surface and " +
			"IndexSearcher.rewrite, not yet ported in Gocene")
	}
}

// TestMultiTermQueryRewrites_Boosts ports testBoosts. The reference builds a
// custom MultiTermQuery whose FilteredTermsEnum sets a per-term BoostAttribute and
// asserts the rewritten BooleanQuery's per-clause boosts equal the parsed term
// values across all three readers.
func TestMultiTermQueryRewrites_Boosts(t *testing.T) {
	t.Errorf("testBoosts requires a custom MultiTermQuery subclass with a FilteredTermsEnum that sets a " +
		"per-term BoostAttribute, the SCORING_BOOLEAN_REWRITE / TopTermsScoringBooleanQueryRewrite methods, " +
		"and IndexSearcher.rewrite — none of which are part of Gocene's MultiTermQuery surface yet")
}

// TestMultiTermQueryRewrites_MaxClauseLimitations ports testMaxClauseLimitations.
// It depends on per-RewriteMethod enforcement of the maxClauseCount limit during
// IndexSearcher.rewrite (TooManyClauses for the boolean rewrites, no limit for the
// constant-score / top-terms rewrites), which is not yet ported.
func TestMultiTermQueryRewrites_MaxClauseLimitations(t *testing.T) {
	t.Errorf("testMaxClauseLimitations requires the selectable MultiTermQuery.RewriteMethod surface and the " +
		"per-method maxClauseCount enforcement inside IndexSearcher.rewrite (TooManyClauses for the boolean " +
		"rewrites, no limit for constant-score/top-terms) — not yet ported in Gocene")
}
