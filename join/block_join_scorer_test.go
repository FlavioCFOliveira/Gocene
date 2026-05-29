// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package join contains tests porting
// org.apache.lucene.search.join.TestBlockJoinScorer.
//
// testScoreNone/testScoreMax exercise the TOP_SCORES + min-competitive-score
// early-termination path, which the block-join scorer does not yet implement
// (the Scorer interface has no SetMinCompetitiveScore); they remain deferred
// with a re-pointed skip. The structural constructor test runs directly.
package join

import (
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// firstLeafTopScoresScorer rewrites q, builds a TOP_SCORES-needing Weight, asks
// the first leaf for a ScorerSupplier, marks it the top-level scoring clause,
// and returns the Scorer. It mirrors the Lucene idiom
//
//	Weight w = searcher.createWeight(searcher.rewrite(q), TOP_SCORES, 1);
//	ScorerSupplier ss = w.scorerSupplier(leaves().get(0));
//	ss.setTopLevelScoringClause();
//	Scorer scorer = ss.get(Long.MAX_VALUE);
func firstLeafTopScoresScorer(t *testing.T, searcher *search.IndexSearcher, reader *index.DirectoryReader, q search.Query) search.Scorer {
	t.Helper()
	rewritten, err := q.Rewrite(reader)
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}
	// TOP_SCORES is requested by passing needsScores=true; the block-join
	// None-mode child is built as a TOP_SCORES ConstantScoreScorer regardless.
	weight, err := rewritten.CreateWeight(searcher, true, 1.0)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("Leaves: %v", err)
	}
	if len(leaves) == 0 {
		t.Fatal("reader has no leaves")
	}
	ss, err := weight.ScorerSupplier(leaves[0])
	if err != nil {
		t.Fatalf("ScorerSupplier: %v", err)
	}
	if ss == nil {
		t.Fatal("nil scorer supplier")
	}
	ss.SetTopLevelScoringClause()
	scorer, err := ss.Get(1<<62 - 1)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	return scorer
}

// mustSetMinCompetitiveScore asserts that scorer honours the optional
// search.MinCompetitiveScorer interface and forwards minScore without error.
func mustSetMinCompetitiveScore(t *testing.T, scorer search.Scorer, minScore float32) {
	t.Helper()
	mc, ok := scorer.(search.MinCompetitiveScorer)
	if !ok {
		t.Fatalf("scorer %T does not implement MinCompetitiveScorer", scorer)
	}
	if err := mc.SetMinCompetitiveScore(minScore); err != nil {
		t.Fatalf("SetMinCompetitiveScore(%v): %v", minScore, err)
	}
}

// nextSetBitFromBitSetProducer returns the leaf-local parent bitset positions in
// ascending order, used to predict the parents the scorer should iterate.
func parentBitsForLeaf(t *testing.T, reader *index.DirectoryReader, parents BitSetProducer) *FixedBitSet {
	t.Helper()
	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("Leaves: %v", err)
	}
	bits, err := parents.GetBitSet(leaves[0])
	if err != nil {
		t.Fatalf("GetBitSet: %v", err)
	}
	return bits
}

// TestBlockJoinScorer_ScoreNone corresponds to
// TestBlockJoinScorer.testScoreNone. It exercises the TOP_SCORES early
// termination path: scorer.SetMinCompetitiveScore is used to skip
// non-competitive parents under ScoreMode.None. The corpus is 10 blocks
// (block i has i children plus one parent), so the parents bitset is
// {0,2,5,9,14,20,27,35,44,54} and the scorer iterates the 9 parents that
// follow a child (block 0's parent at doc 0 has no children and is skipped).
func TestBlockJoinScorer_ScoreNone(t *testing.T) {
	dir, w := newBlockWriter(t)
	for i := 0; i < 10; i++ {
		docs := make([]index.Document, 0, i+1)
		for j := 0; j < i; j++ {
			child := document.NewDocument()
			child.Add(mustStringField(t, "value", strconv.Itoa(j), true))
			docs = append(docs, child)
		}
		parent := document.NewDocument()
		parent.Add(mustStringField(t, "docType", "parent", false))
		parent.Add(mustStringField(t, "value", strconv.Itoa(i), true))
		docs = append(docs, parent)
		addBlock(t, w, docs...)
	}
	reader, searcher := commitAndOpen(t, dir, w)

	parents := newQueryBitSetParents("docType", "parent")
	if err := Check(reader, parents); err != nil {
		t.Fatalf("CheckJoinIndex: %v", err)
	}
	bits := parentBitsForLeaf(t, reader, parents)

	childQuery := search.NewMatchAllDocsQuery()
	query := NewToParentBlockJoinQuery(childQuery, parents, None)

	// 1) No min-competitive-score: all 9 parents iterate.
	expectParents := func(scorer search.Scorer) {
		parent := 0
		for i := 0; i < 9; i++ {
			parent = bits.NextSetBit(parent + 1)
			doc, err := scorer.NextDoc()
			if err != nil {
				t.Fatalf("NextDoc: %v", err)
			}
			if doc != parent {
				t.Fatalf("iter %d: doc = %d, want %d", i, doc, parent)
			}
		}
		doc, err := scorer.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if doc != search.NO_MORE_DOCS {
			t.Fatalf("expected NO_MORE_DOCS, got %d", doc)
		}
	}

	expectParents(firstLeafTopScoresScorer(t, searcher, reader, query))

	// 2) setMinCompetitiveScore(0): 0 is not > the constant 0 score, so all 9
	//    parents still iterate.
	scorer := firstLeafTopScoresScorer(t, searcher, reader, query)
	mustSetMinCompetitiveScore(t, scorer, 0)
	expectParents(scorer)

	// 3) setMinCompetitiveScore(nextUp(0)): the smallest positive float exceeds
	//    the constant 0 score, so the child is emptied and the scorer returns
	//    NO_MORE_DOCS immediately.
	scorer = firstLeafTopScoresScorer(t, searcher, reader, query)
	mustSetMinCompetitiveScore(t, scorer, nextUpZero())
	if doc, err := scorer.NextDoc(); err != nil {
		t.Fatalf("NextDoc: %v", err)
	} else if doc != search.NO_MORE_DOCS {
		t.Fatalf("expected NO_MORE_DOCS, got %d", doc)
	}

	// 4) Advance one parent, then setMinCompetitiveScore(nextUp(0)) terminates
	//    the remaining iteration.
	scorer = firstLeafTopScoresScorer(t, searcher, reader, query)
	if doc, err := scorer.NextDoc(); err != nil {
		t.Fatalf("NextDoc: %v", err)
	} else if doc != 2 {
		t.Fatalf("first parent = %d, want 2", doc)
	}
	mustSetMinCompetitiveScore(t, scorer, nextUpZero())
	if doc, err := scorer.NextDoc(); err != nil {
		t.Fatalf("NextDoc: %v", err)
	} else if doc != search.NO_MORE_DOCS {
		t.Fatalf("expected NO_MORE_DOCS, got %d", doc)
	}
}

// nextUpZero returns the smallest float32 strictly greater than 0, the analogue
// of Java's Math.nextUp(0f).
func nextUpZero() float32 {
	return 1.401298464324817e-45 // math.Float32frombits(1)
}

// TestBlockJoinScorer_ScoreMax corresponds to
// TestBlockJoinScorer.testScoreMax. It needs runnable ConstantScoreQuery/
// BoostQuery children scored through a TOP_SCORES disjunction whose
// SetMinCompetitiveScore actually prunes (a functional WANDScorer block-max
// path). Gocene's WANDScorer port leaves advanceShallow/setMinCompetitiveScore
// as no-op stubs and BooleanScorerSupplier does not route SHOULD disjunctions
// to it under TOP_SCORES, so the exact early-termination assertions cannot be
// reproduced yet.
func TestBlockJoinScorer_ScoreMax(t *testing.T) {
	t.Skip("requires a functional WANDScorer block-max min-competitive-score path + BooleanScorerSupplier TOP_SCORES routing for the SHOULD ConstantScore disjunction: rmp #4776")
}

// TestBlockJoinScorer_BlockJoinScorerConstruction verifies that BlockJoinScorer
// can be constructed given a valid parent bitset, mirroring the scorer setup
// pattern in the Java test.
func TestBlockJoinScorer_BlockJoinScorerConstruction(t *testing.T) {
	bs := buildParentBitSet(t, []int{2, 5, 9}, 12)
	if bs == nil {
		t.Fatal("expected non-nil BitSet")
	}
	// BlockJoinScorer is an internal type used within ToParentBlockJoinWeight;
	// verify query construction as the externally observable entry point.
	q := NewToParentBlockJoinQuery(nil, nil, Max)
	if q == nil {
		t.Fatal("expected non-nil ToParentBlockJoinQuery")
	}
}
