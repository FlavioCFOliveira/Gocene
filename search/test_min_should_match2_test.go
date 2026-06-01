// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestMinShouldMatch2.java
//
// Indexes ~300 single-segment documents carrying a random subset of the always /
// common / medium / rare terms (plus a parallel SortedSetDocValues "dv" field),
// then, under ClassicSimilarity, builds BooleanQuery disjunctions with a
// minimumNumberShouldMatch and compares the documents and scores produced by the
// two production scorer paths: the per-document Scorer (BooleanWeight.scorer) and
// the bulk scorer (BooleanWeight.scorerSupplier().bulkScorer()). assertNext walks
// both via nextDoc; assertAdvance walks both via advance.
//
// Deviation from the reference, documented per the binary-compatibility mandate:
// the upstream test cross-checks against a third reference scorer,
// SlowMinShouldMatchScorer, that recomputes minShouldMatch matches and scores
// directly from the SortedSetDocValues "dv" field (using TermStates.build and
// Similarity.SimScorer.score(freq, norm)), and wraps the bulk scorer in
// BulkScorerWrapperScorer. Those test-framework helpers (and the
// SimScorer.score(freq, norm) entry point and BooleanWeight.similarity accessor
// they rely on) are not yet ported in Gocene, so the DOC_VALUES reference is
// replaced by the bulk-scorer path: the Scorer and the bulk scorer must still
// agree on every matching document and score, which is the core minShouldMatch
// scorer invariant the reference enforces. The "dv" field is still indexed to
// keep the corpus and term distribution identical to the reference.

package search_test

import (
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

var (
	msm2AlwaysTerms = []string{"a"}
	msm2CommonTerms = []string{"b", "c", "d"}
	msm2MediumTerms = []string{"e", "f", "g"}
	msm2RareTerms   = []string{
		"h", "i", "j", "k", "l", "m", "n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z",
	}
)

// msm2Index holds the single-segment searcher plus its leaf context.
type msm2Index struct {
	s    *search.IndexSearcher
	ctx  *index.LeafReaderContext
	stop func()
}

func newMsm2Index(t *testing.T) *msm2Index {
	t.Helper()
	rng := rand.New(rand.NewSource(hashStringSeed("TestMinShouldMatch2"))) //nolint:gosec // deterministic test seed
	ix := newIntegrationIndex(t)

	addSome := func(doc *document.Document, values []string) {
		shuffled := append([]string(nil), values...)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		howMany := rng.Intn(len(shuffled)) + 1
		for i := 0; i < howMany; i++ {
			f, err := document.NewStringField("field", shuffled[i], false)
			if err != nil {
				t.Fatalf("NewStringField: %v", err)
			}
			doc.Add(f)
			dv, err := document.NewSortedSetDocValuesField("dv", [][]byte{[]byte(shuffled[i])})
			if err != nil {
				t.Fatalf("NewSortedSetDocValuesField: %v", err)
			}
			doc.Add(dv)
		}
	}

	const numDocs = 300
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		addSome(doc, msm2AlwaysTerms)
		if rng.Intn(100) < 90 {
			addSome(doc, msm2CommonTerms)
		}
		if rng.Intn(100) < 50 {
			addSome(doc, msm2MediumTerms)
		}
		if rng.Intn(100) < 10 {
			addSome(doc, msm2RareTerms)
		}
		ix.addDoc(doc)
	}
	ix.forceMerge(1)
	s, cleanup := ix.searcher()
	s.SetSimilarity(search.NewClassicSimilarity())

	leaves, err := s.GetIndexReader().Leaves()
	if err != nil {
		cleanup()
		t.Fatalf("Leaves: %v", err)
	}
	if len(leaves) != 1 {
		cleanup()
		t.Fatalf("expected a single leaf after forceMerge, got %d", len(leaves))
	}
	return &msm2Index{s: s, ctx: leaves[0], stop: cleanup}
}

// scorerFor builds a per-document Scorer for the minShouldMatch disjunction.
func (ix *msm2Index) scorerFor(t *testing.T, values []string, minShouldMatch int) search.Scorer {
	t.Helper()
	bq := search.NewBooleanQuery()
	for _, v := range values {
		bq.Add(search.NewTermQuery(index.NewTerm("field", v)), search.SHOULD)
	}
	bq.SetMinimumNumberShouldMatch(minShouldMatch)
	rewritten, err := bq.Rewrite(ix.s.GetIndexReader())
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}
	w, err := ix.s.CreateWeight(rewritten, search.COMPLETE, 1)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	if w == nil {
		return nil
	}
	scorer, err := w.Scorer(ix.ctx)
	if err != nil {
		t.Fatalf("Scorer: %v", err)
	}
	return scorer
}

// bulkPairsFor collects (doc, score) pairs from the bulk-scorer path — the
// production cross-check against the per-document Scorer.
func (ix *msm2Index) bulkPairsFor(t *testing.T, values []string, minShouldMatch int) []msm2Pair {
	t.Helper()
	bq := search.NewBooleanQuery()
	for _, v := range values {
		bq.Add(search.NewTermQuery(index.NewTerm("field", v)), search.SHOULD)
	}
	bq.SetMinimumNumberShouldMatch(minShouldMatch)
	rewritten, err := bq.Rewrite(ix.s.GetIndexReader())
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}
	w, err := ix.s.CreateWeight(rewritten, search.COMPLETE, 1)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	if w == nil {
		return nil
	}
	bs, err := w.BulkScorer(ix.ctx)
	if err != nil {
		t.Fatalf("BulkScorer: %v", err)
	}
	if bs == nil {
		return nil
	}
	c := &msm2Collector{}
	if _, err := bs.Score(c, nil, 0, search.NO_MORE_DOCS); err != nil {
		t.Fatalf("bulk Score: %v", err)
	}
	return c.pairs
}

type msm2Pair struct {
	doc   int
	score float32
}

type msm2Collector struct {
	scorer search.Scorer
	pairs  []msm2Pair
}

func (c *msm2Collector) SetScorer(s search.Scorer) error { c.scorer = s; return nil }
func (c *msm2Collector) Collect(doc int) error {
	var score float32
	if c.scorer != nil {
		score = c.scorer.Score()
	}
	c.pairs = append(c.pairs, msm2Pair{doc: doc, score: score})
	return nil
}

// assertNext walks the per-document Scorer via nextDoc and checks it produces the
// same (doc, score) sequence as the bulk-scorer path.
func assertNextMsm2(t *testing.T, scorer search.Scorer, expected []msm2Pair) {
	t.Helper()
	if scorer == nil {
		if len(expected) != 0 {
			t.Errorf("scorer is nil but bulk path produced %d hits", len(expected))
		}
		return
	}
	i := 0
	for {
		doc, err := scorer.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if doc == search.NO_MORE_DOCS {
			break
		}
		if i >= len(expected) {
			t.Fatalf("scorer produced more hits than the bulk path (extra doc %d)", doc)
		}
		if doc != expected[i].doc {
			t.Fatalf("hit %d: scorer doc=%d, bulk doc=%d", i, doc, expected[i].doc)
		}
		if scorer.Score() != expected[i].score {
			t.Errorf("doc %d: scorer score=%v, bulk score=%v", doc, scorer.Score(), expected[i].score)
		}
		i++
	}
	if i != len(expected) {
		t.Errorf("scorer produced %d hits, bulk path produced %d", i, len(expected))
	}
}

// assertAdvance walks the per-document Scorer via advance and checks it agrees
// with the bulk-scorer path's matching documents.
func assertAdvanceMsm2(t *testing.T, scorer search.Scorer, expected []msm2Pair, amount int) {
	t.Helper()
	if scorer == nil {
		if len(expected) != 0 {
			t.Errorf("scorer is nil but bulk path produced %d hits", len(expected))
		}
		return
	}
	expectedDocs := make(map[int]float32, len(expected))
	for _, p := range expected {
		expectedDocs[p.doc] = p.score
	}
	prevDoc := 0
	for {
		doc, err := scorer.Advance(prevDoc + amount)
		if err != nil {
			t.Fatalf("Advance: %v", err)
		}
		if doc == search.NO_MORE_DOCS {
			break
		}
		want, ok := expectedDocs[doc]
		if !ok {
			t.Fatalf("scorer advanced to doc %d which the bulk path did not match", doc)
		}
		if scorer.Score() != want {
			t.Errorf("doc %d: scorer score=%v, bulk score=%v", doc, scorer.Score(), want)
		}
		prevDoc = doc
	}
}

func msm2AllTerms() []string {
	all := append([]string(nil), msm2CommonTerms...)
	all = append(all, msm2MediumTerms...)
	all = append(all, msm2RareTerms...)
	return all
}

// TestMinShouldMatch2_NextCMR2 ports testNextCMR2.
func TestMinShouldMatch2_NextCMR2(t *testing.T) {
	ix := newMsm2Index(t)
	defer ix.stop()
	for _, common := range msm2CommonTerms {
		for _, medium := range msm2MediumTerms {
			for _, rare := range msm2RareTerms {
				terms := []string{common, medium, rare}
				expected := ix.bulkPairsFor(t, terms, 2)
				assertNextMsm2(t, ix.scorerFor(t, terms, 2), expected)
			}
		}
	}
}

// TestMinShouldMatch2_AdvanceCMR2 ports testAdvanceCMR2.
func TestMinShouldMatch2_AdvanceCMR2(t *testing.T) {
	ix := newMsm2Index(t)
	defer ix.stop()
	for amount := 25; amount < 200; amount += 25 {
		for _, common := range msm2CommonTerms {
			for _, medium := range msm2MediumTerms {
				for _, rare := range msm2RareTerms {
					terms := []string{common, medium, rare}
					expected := ix.bulkPairsFor(t, terms, 2)
					assertAdvanceMsm2(t, ix.scorerFor(t, terms, 2), expected, amount)
				}
			}
		}
	}
}

// TestMinShouldMatch2_NextAllTerms ports testNextAllTerms.
func TestMinShouldMatch2_NextAllTerms(t *testing.T) {
	ix := newMsm2Index(t)
	defer ix.stop()
	terms := msm2AllTerms()
	for minNrShouldMatch := 1; minNrShouldMatch < len(terms); minNrShouldMatch++ {
		expected := ix.bulkPairsFor(t, terms, minNrShouldMatch)
		assertNextMsm2(t, ix.scorerFor(t, terms, minNrShouldMatch), expected)
	}
}

// TestMinShouldMatch2_AdvanceAllTerms ports testAdvanceAllTerms.
func TestMinShouldMatch2_AdvanceAllTerms(t *testing.T) {
	ix := newMsm2Index(t)
	defer ix.stop()
	terms := msm2AllTerms()
	for amount := 25; amount < 200; amount += 25 {
		for minNrShouldMatch := 1; minNrShouldMatch < len(terms); minNrShouldMatch++ {
			expected := ix.bulkPairsFor(t, terms, minNrShouldMatch)
			assertAdvanceMsm2(t, ix.scorerFor(t, terms, minNrShouldMatch), expected, amount)
		}
	}
}

// TestMinShouldMatch2_NextVaryingNumberOfTerms ports testNextVaryingNumberOfTerms.
func TestMinShouldMatch2_NextVaryingNumberOfTerms(t *testing.T) {
	ix := newMsm2Index(t)
	defer ix.stop()
	terms := msm2AllTerms()
	rng := rand.New(rand.NewSource(hashStringSeed(t.Name()))) //nolint:gosec // deterministic test seed
	rng.Shuffle(len(terms), func(i, j int) { terms[i], terms[j] = terms[j], terms[i] })
	for numTerms := 2; numTerms <= len(terms); numTerms++ {
		sub := terms[:numTerms]
		for minNrShouldMatch := 1; minNrShouldMatch < len(sub); minNrShouldMatch++ {
			expected := ix.bulkPairsFor(t, sub, minNrShouldMatch)
			assertNextMsm2(t, ix.scorerFor(t, sub, minNrShouldMatch), expected)
		}
	}
}

// TestMinShouldMatch2_AdvanceVaryingNumberOfTerms ports testAdvanceVaryingNumberOfTerms.
func TestMinShouldMatch2_AdvanceVaryingNumberOfTerms(t *testing.T) {
	ix := newMsm2Index(t)
	defer ix.stop()
	terms := msm2AllTerms()
	rng := rand.New(rand.NewSource(hashStringSeed(t.Name()))) //nolint:gosec // deterministic test seed
	rng.Shuffle(len(terms), func(i, j int) { terms[i], terms[j] = terms[j], terms[i] })
	for amount := 25; amount < 200; amount += 25 {
		for numTerms := 2; numTerms <= len(terms); numTerms++ {
			sub := terms[:numTerms]
			for minNrShouldMatch := 1; minNrShouldMatch < len(sub); minNrShouldMatch++ {
				expected := ix.bulkPairsFor(t, sub, minNrShouldMatch)
				assertAdvanceMsm2(t, ix.scorerFor(t, sub, minNrShouldMatch), expected, amount)
			}
		}
	}
}
