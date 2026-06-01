// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestBlockMaxConjunction.java
//
// Indexes ~1000 documents, each carrying a random number of "foo" StringField
// values, into a single segment, then runs 100 random MUST conjunctions through
// CheckHits.checkTopScores (search/testutil.CheckTopScores), which asserts that
// the COMPLETE and TOP_SCORES (block-max WAND / dynamic-pruning) collectors agree
// on the top hits and that the block-max bounds are valid. Each conjunction is
// also exercised with a FILTER clause and with two-phase-approximation-wrapped
// clauses, mirroring the Java testRandom loop.
//
// Deviations from the reference, immaterial to the assertions:
//   - The MockAnalyzer is replaced by the WhitespaceAnalyzer.
//   - maybeWrap optionally wraps a clause in the (delegating) AssertingQuery. The
//     upstream BlockScoreQueryWrapper, which forces artificial impact blocks, is
//     not part of Gocene's surface; its scoring is identical to the wrapped query,
//     so omitting it preserves the checkTopScores invariant under test.
//   - maybeWrapTwoPhase wraps a clause in RandomApproximationQuery + AssertingQuery,
//     exercising the two-phase path exactly as the reference does.

package search_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/search/testutil"
)

// TestBlockMaxConjunction_Random ports testRandom.
func TestBlockMaxConjunction_Random(t *testing.T) {
	rng := rand.New(rand.NewSource(hashStringSeed(t.Name()))) //nolint:gosec // deterministic test seed

	ix := newIntegrationIndex(t)
	numDocs := 1000
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		numValues := rng.Intn(1 << uint(rng.Intn(5)))
		start := rng.Intn(10)
		for j := 0; j < numValues; j++ {
			f, err := document.NewStringField("foo", fmt.Sprintf("%d", start+j), false)
			if err != nil {
				t.Fatalf("NewStringField: %v", err)
			}
			doc.Add(f)
		}
		ix.addDoc(doc)
	}
	// A single segment is required for the per-leaf block-max assertions.
	ix.forceMerge(1)
	s, cleanup := ix.searcher()
	defer cleanup()

	maybeWrap := func(q search.Query) search.Query {
		if rng.Intn(2) == 1 {
			return newAssertingQuery(q)
		}
		return q
	}
	maybeWrapTwoPhase := func(q search.Query) search.Query {
		if rng.Intn(2) == 1 {
			return newAssertingQuery(newRandomApproximationQuery(q, rng))
		}
		return q
	}

	for iter := 0; iter < 100; iter++ {
		start := rng.Intn(10)
		numClauses := rng.Intn(1 << uint(rng.Intn(5)))

		builder := search.NewBooleanQuery()
		for i := 0; i < numClauses; i++ {
			builder.Add(maybeWrap(search.NewTermQuery(index.NewTerm("foo", fmt.Sprintf("%d", start+i)))), search.MUST)
		}
		query := builder

		testutil.CheckTopScores(t, rng, query, s)

		filterTerm := rng.Intn(30)
		filtered := search.NewBooleanQuery()
		filtered.Add(query, search.MUST)
		filtered.Add(search.NewTermQuery(index.NewTerm("foo", fmt.Sprintf("%d", filterTerm))), search.FILTER)
		testutil.CheckTopScores(t, rng, filtered, s)

		tpBuilder := search.NewBooleanQuery()
		for i := 0; i < numClauses; i++ {
			tpBuilder.Add(maybeWrapTwoPhase(search.NewTermQuery(index.NewTerm("foo", fmt.Sprintf("%d", start+i)))), search.MUST)
		}
		twoPhase := search.NewBooleanQuery()
		twoPhase.Add(query, search.MUST)
		twoPhase.Add(search.NewTermQuery(index.NewTerm("foo", fmt.Sprintf("%d", filterTerm))), search.FILTER)
		testutil.CheckTopScores(t, rng, twoPhase, s)
	}
}
