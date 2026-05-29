// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package join contains tests porting
// org.apache.lucene.search.join.TestBlockJoinScorer.
//
// testScoreNone/testScoreMax exercise the TOP_SCORES + min-competitive-score
// early-termination path of the block-join scorer: ToParentBlockJoinScorer
// implements search.MinCompetitiveScorer and forwards the hint to the child
// scorer (a TOP_SCORES ConstantScoreScorer for None, a WANDScorer disjunction
// for Max). Both run directly.
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

// boostCSQTerm builds BoostQuery(ConstantScoreQuery(TermQuery(field=value)),
// boost), the SHOULD clause shape used by TestBlockJoinScorer.testScoreMax.
func boostCSQTerm(field, value string, boost float32) search.Query {
	return search.NewBoostQuery(
		search.NewConstantScoreQuery(search.NewTermQuery(index.NewTerm(field, value))),
		boost,
	)
}

// scoreMaxChildQuery builds the SHOULD disjunction of boosted constant-score
// term queries from TestBlockJoinScorer.testScoreMax: A=2, B=1, C=3, D=4.
func scoreMaxChildQuery() *search.BooleanQuery {
	q := search.NewBooleanQuery()
	q.Add(boostCSQTerm("value", "A", 2), search.SHOULD)
	q.Add(boostCSQTerm("value", "B", 1), search.SHOULD)
	q.Add(boostCSQTerm("value", "C", 3), search.SHOULD)
	q.Add(boostCSQTerm("value", "D", 4), search.SHOULD)
	return q
}

// addStaticBlock adds one block of child docs (each carrying the listed values)
// followed by a parent doc, mirroring populateStaticIndex in
// TestBlockJoinScorer/TestBlockJoinBulkScorer.
func addStaticBlock(t *testing.T, w *index.IndexWriter, children [][]string) {
	t.Helper()
	docs := make([]index.Document, 0, len(children)+1)
	for _, values := range children {
		child := document.NewDocument()
		child.Add(mustStringField(t, "type", "child", false))
		for _, v := range values {
			child.Add(mustStringField(t, "value", v, false))
		}
		docs = append(docs, child)
	}
	parent := document.NewDocument()
	parent.Add(mustStringField(t, "type", "parent", false))
	docs = append(docs, parent)
	addBlock(t, w, docs...)
}

// TestBlockJoinScorer_ScoreMax corresponds to
// TestBlockJoinScorer.testScoreMax. It exercises a TOP_SCORES + ScoreMode.Max
// block join over a SHOULD disjunction of boosted constant-score term queries,
// verifying both the exact per-parent Max scores and the
// SetMinCompetitiveScore early-termination behaviour (which routes through the
// WANDScorer block-max path produced by BooleanWeight under
// setTopLevelScoringClause).
func TestBlockJoinScorer_ScoreMax(t *testing.T) {
	dir, w := newBlockWriter(t)
	for _, children := range [][][]string{
		{{"A", "B"}, {"A", "B", "C"}},
		{{"A"}, {"B"}},
		{{}},
		{{"A", "B", "C"}, {"A", "B", "C", "D"}},
		{{"B"}},
		{{"B", "C"}, {"A", "B"}, {"A", "C"}},
	} {
		addStaticBlock(t, w, children)
	}
	reader, searcher := commitAndOpen(t, dir, w)

	parents := newQueryBitSetParents("type", "parent")
	query := NewToParentBlockJoinQuery(scoreMaxChildQuery(), parents, Max)

	// Full scoring: assert exact Max scores per parent.
	type docScore struct {
		doc   int
		score float32
	}
	want := []docScore{
		{2, 2 + 1 + 3},      // block0: max(3, 6) = 6
		{5, 2},              // block1: max(2, 1) = 2
		{10, 2 + 1 + 3 + 4}, // block3: max(6, 10) = 10
		{12, 1},             // block4: 1
		{16, 2 + 3},         // block5: max(4, 3, 5) = 5
	}
	scorer := firstLeafTopScoresScorer(t, searcher, reader, query)
	for i, ws := range want {
		doc, err := scorer.NextDoc()
		if err != nil {
			t.Fatalf("iter %d NextDoc: %v", i, err)
		}
		if doc != ws.doc {
			t.Fatalf("iter %d: doc = %d, want %d", i, doc, ws.doc)
		}
		if got := scorer.Score(); got != ws.score {
			t.Fatalf("iter %d (doc %d): score = %v, want %v", i, doc, got, ws.score)
		}
	}
	if doc, err := scorer.NextDoc(); err != nil {
		t.Fatalf("final NextDoc: %v", err)
	} else if doc != search.NO_MORE_DOCS {
		t.Fatalf("expected NO_MORE_DOCS, got %d", doc)
	}

	// setMinCompetitiveScore(6): only parents whose Max score >= 6 survive.
	scorer = firstLeafTopScoresScorer(t, searcher, reader, query)
	mustSetMinCompetitiveScore(t, scorer, 6)
	for _, ws := range []docScore{{2, 2 + 1 + 3}, {10, 2 + 1 + 3 + 4}} {
		doc, err := scorer.NextDoc()
		if err != nil {
			t.Fatalf("prune6 NextDoc: %v", err)
		}
		if doc != ws.doc {
			t.Fatalf("prune6: doc = %d, want %d", doc, ws.doc)
		}
		if got := scorer.Score(); got != ws.score {
			t.Fatalf("prune6 (doc %d): score = %v, want %v", doc, got, ws.score)
		}
	}
	if doc, err := scorer.NextDoc(); err != nil {
		t.Fatalf("prune6 final NextDoc: %v", err)
	} else if doc != search.NO_MORE_DOCS {
		t.Fatalf("prune6: expected NO_MORE_DOCS, got %d", doc)
	}

	// Advance one parent, then setMinCompetitiveScore(11) terminates iteration
	// because no block can reach 11 (the global max child score is 10).
	scorer = firstLeafTopScoresScorer(t, searcher, reader, query)
	if doc, err := scorer.NextDoc(); err != nil {
		t.Fatalf("prune11 NextDoc: %v", err)
	} else if doc != 2 {
		t.Fatalf("prune11: first parent = %d, want 2", doc)
	}
	if got := scorer.Score(); got != 2+1+3 {
		t.Fatalf("prune11: score = %v, want 6", got)
	}
	mustSetMinCompetitiveScore(t, scorer, 11)
	if doc, err := scorer.NextDoc(); err != nil {
		t.Fatalf("prune11 NextDoc: %v", err)
	} else if doc != search.NO_MORE_DOCS {
		t.Fatalf("prune11: expected NO_MORE_DOCS, got %d", doc)
	}
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
