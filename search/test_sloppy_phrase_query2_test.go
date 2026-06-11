// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestSloppyPhraseQuery2.java
//   (the testRandomIncreasingSloppiness / randomPhraseQuery slice).
//
// The remaining TestSloppyPhraseQuery2 methods are implemented in
// sloppy_phrase_query_test.go and share its createTestIndex / assertSubsetOf
// helpers. This file completes the suite with the MultiPhraseQuery case the
// reference exercises: MultiPhraseQuery~N ⊆ MultiPhraseQuery~N+1.
//
// Deviation from the reference, immaterial to the subset invariant: Lucene seeds
// randomPhraseQuery from the global test PRNG; Gocene uses a fixed deterministic
// seed so the suite is reproducible. The same seed is reused for q1 and q2 so
// they are structurally identical before their slops are set, exactly as the
// reference requires.

package search_test

import (
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestSloppyPhraseQuery2_RandomIncreasingSloppiness ports
// testRandomIncreasingSloppiness: a randomly-shaped MultiPhraseQuery at slop N
// matches a subset of the documents it matches at slop N+1.
func TestSloppyPhraseQuery2_RandomIncreasingSloppiness(t *testing.T) {
	_, searcher, cleanup := createTestIndex(t)
	defer cleanup()

	const seed int64 = 0x53104A2B17C9F1E3
	for i := 0; i < 10; i++ {
		q1 := randomMultiPhraseQuery(seed)
		q2 := randomMultiPhraseQuery(seed)
		q1 = search.NewMultiPhraseQueryBuilderFromQuery(q1).SetSlop(i).Build()
		q2 = search.NewMultiPhraseQueryBuilderFromQuery(q2).SetSlop(i + 1).Build()
		assertSubsetOf(t, searcher, q1, q2)
	}

// randomMultiPhraseQuery ports TestSloppyPhraseQuery2.randomPhraseQuery: a
// MultiPhraseQuery of 2-5 positions, each holding 1-3 single-character terms
// (a-z), with random positional gaps of 1-3. Reusing the same seed yields a
// structurally identical query, so the only difference between the q1/q2 pair
// is the slop the caller sets afterwards.
func randomMultiPhraseQuery(seed int64) *search.MultiPhraseQuery {
	rng := rand.New(rand.NewSource(seed))
	length := nextIntInclusive(rng, 2, 5)
	b := search.NewMultiPhraseQueryBuilder()
	position := 0
	for i := 0; i < length; i++ {
		depth := nextIntInclusive(rng, 1, 3)
		terms := make([]*index.Term, depth)
		for j := 0; j < depth; j++ {
			c := byte(nextIntInclusive(rng, 'a', 'z'))
			terms[j] = index.NewTerm("field", string(c))
		}
		b.AddTermsAtPosition(terms, position)
		position += nextIntInclusive(rng, 1, 3)
	}
	return b.Build()
}

// nextIntInclusive returns a uniformly random int in [min, max], mirroring
// TestUtil.nextInt(random, min, max) (both bounds inclusive).
func nextIntInclusive(rng *rand.Rand, min, max int) int {
	return min + rng.Intn(max-min+1)
}