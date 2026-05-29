// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package join contains tests porting
// org.apache.lucene.search.join.TestBlockJoinBulkScorer.
//
// These exercise the BlockJoinBulkScorer windowed bulk-scoring path
// (exhaustive child scoring per parent with min-competitive-score early
// termination) returned by ToParentBlockJoinWeight.scorerSupplier().bulkScorer()
// when scoreMode != None.
package join

import (
	"math/rand"
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// bulkScoreCollector records the (doc, score) pairs collected by a BulkScorer,
// optionally pushing a minimum competitive score on SetScorer. It mirrors the
// anonymous LeafCollector in TestBlockJoinBulkScorer.assertScores.
type bulkScoreCollector struct {
	t          *testing.T
	scorer     search.Scorer
	needsScore bool
	minScore   *float32
	scores     map[int]float32
}

func newBulkScoreCollector(t *testing.T, needsScore bool, minScore *float32) *bulkScoreCollector {
	return &bulkScoreCollector{t: t, needsScore: needsScore, minScore: minScore, scores: map[int]float32{}}
}

func (c *bulkScoreCollector) SetScorer(scorer search.Scorer) error {
	if scorer == nil {
		c.t.Fatal("SetScorer received nil scorer")
	}
	c.scorer = scorer
	if c.minScore != nil {
		mc, ok := scorer.(search.MinCompetitiveScorer)
		if !ok {
			c.t.Fatalf("scorer %T does not implement MinCompetitiveScorer", scorer)
		}
		if err := mc.SetMinCompetitiveScore(*c.minScore); err != nil {
			return err
		}
	}
	return nil
}

func (c *bulkScoreCollector) Collect(doc int) error {
	if c.scorer == nil {
		c.t.Fatal("Collect called before SetScorer")
	}
	var score float32
	if c.needsScore {
		score = c.scorer.Score()
	}
	c.scores[doc] = score
	return nil
}

// bulkScorerForLeaf rewrites q, builds a Weight (needsScores=needsScore), gets
// the first-leaf ScorerSupplier, marks it the top-level scoring clause, and
// returns its BulkScorer. Mirrors the Lucene idiom in TestBlockJoinBulkScorer.
func bulkScorerForLeaf(t *testing.T, searcher *search.IndexSearcher, reader *index.DirectoryReader, q search.Query, needsScore bool) search.BulkScorer {
	t.Helper()
	rewritten, err := q.Rewrite(reader)
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}
	weight, err := rewritten.CreateWeight(searcher, needsScore, 1.0)
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
	bsp, ok := ss.(interface {
		BulkScorer() (search.BulkScorer, error)
	})
	if !ok {
		t.Fatalf("supplier %T does not expose BulkScorer()", ss)
	}
	bs, err := bsp.BulkScorer()
	if err != nil {
		t.Fatalf("BulkScorer: %v", err)
	}
	if bs == nil {
		t.Fatal("nil bulk scorer")
	}
	return bs
}

// assertBulkScores drives bs over the whole doc space and compares the
// collected (doc, score) map against want. Mirrors
// TestBlockJoinBulkScorer.assertScores.
func assertBulkScores(t *testing.T, bs search.BulkScorer, needsScore bool, minScore *float32, want map[int]float32) {
	t.Helper()
	c := newBulkScoreCollector(t, needsScore, minScore)
	if _, err := bs.Score(c, nil, 0, search.NO_MORE_DOCS); err != nil {
		t.Fatalf("Score: %v", err)
	}
	if len(c.scores) != len(want) {
		t.Fatalf("collected %v, want %v", c.scores, want)
	}
	for doc, ws := range want {
		got, ok := c.scores[doc]
		if !ok {
			t.Fatalf("doc %d missing; collected %v, want %v", doc, c.scores, want)
		}
		if got != ws {
			t.Fatalf("doc %d: score = %v, want %v (collected %v)", doc, got, ws, c.scores)
		}
	}
}

// floatPtr returns a pointer to f, for the optional minScore argument.
func floatPtr(f float32) *float32 { return &f }

// TestBlockJoinBulkScorer_SetMinCompetitiveScoreWithScoreModeMax is the port of
// TestBlockJoinBulkScorer.testSetMinCompetitiveScoreWithScoreModeMax. It uses
// the static index and the boosted constant-score SHOULD disjunction, scored
// under ScoreMode.Max + TOP_SCORES, and checks the per-parent Max scores plus
// the min-competitive-score early termination.
func TestBlockJoinBulkScorer_SetMinCompetitiveScoreWithScoreModeMax(t *testing.T) {
	reader, searcher := staticIndexReader(t)
	parents := newQueryBitSetParents("type", "parent")
	query := NewToParentBlockJoinQuery(scoreMaxChildQuery(), parents, Max)

	// No min score: all parents with scoring children, exact Max scores.
	assertBulkScores(t, bulkScorerForLeaf(t, searcher, reader, query, true), true, nil, map[int]float32{
		2: 6, 5: 2, 10: 10, 12: 1, 16: 5,
	})

	// minScore=6: only parents whose Max score >= 6 survive. Gocene scores the
	// whole space in a single batch, so this matches Lucene's single-batch
	// expectation (docs 2 and 10).
	assertBulkScores(t, bulkScorerForLeaf(t, searcher, reader, query, true), true, floatPtr(6), map[int]float32{
		2: 6, 10: 10,
	})

	// minScore=11: no block can reach 11 (global child max is 10).
	assertBulkScores(t, bulkScorerForLeaf(t, searcher, reader, query, true), true, floatPtr(11), map[int]float32{})
}

// TestBlockJoinBulkScorer_SetMinCompetitiveScoreWithScoreModeNone is the port of
// TestBlockJoinBulkScorer.testSetMinCompetitiveScoreWithScoreModeNone. Under
// ScoreMode.None every matched parent scores 0, and only minScore > 0 prunes.
func TestBlockJoinBulkScorer_SetMinCompetitiveScoreWithScoreModeNone(t *testing.T) {
	reader, searcher := staticIndexReader(t)
	parents := newQueryBitSetParents("type", "parent")
	query := NewToParentBlockJoinQuery(scoreMaxChildQuery(), parents, None)

	allZero := map[int]float32{2: 0, 5: 0, 10: 0, 12: 0, 16: 0}

	assertBulkScores(t, bulkScorerForLeaf(t, searcher, reader, query, true), true, nil, allZero)
	assertBulkScores(t, bulkScorerForLeaf(t, searcher, reader, query, true), true, floatPtr(0), allZero)
	assertBulkScores(t, bulkScorerForLeaf(t, searcher, reader, query, true), true, floatPtr(nextUpZero()), map[int]float32{})
}

// staticIndexReader builds the TestBlockJoinBulkScorer static index and returns
// an open reader and searcher.
func staticIndexReader(t *testing.T) (*index.DirectoryReader, *search.IndexSearcher) {
	t.Helper()
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
	return commitAndOpen(t, dir, w)
}

// TestBlockJoinBulkScorer_ScoreRandomIndices is the port of
// TestBlockJoinBulkScorer.testScoreRandomIndices. It builds a deterministic
// pseudo-random index of parent blocks, scores it with the BlockJoinBulkScorer,
// and compares the per-parent scores against an independently computed
// expectation across all join score modes.
func TestBlockJoinBulkScorer_ScoreRandomIndices(t *testing.T) {
	values := []string{"A", "B", "C", "D"}
	scoreOf := map[string]float32{"A": 1, "B": 2, "C": 3, "D": 4}

	for _, joinScoreMode := range []ScoreMode{None, Avg, Max, Total, Min} {
		for _, needsScore := range []bool{true, false} {
			rng := rand.New(rand.NewSource(int64(joinScoreMode)*7 + boolSeed(needsScore)))

			dir, w := newBlockWriter(t)
			// parentDoc -> list of child match-sets (each a slice of values).
			expectedMatches := map[int][][]string{}
			currentDocID := 0
			parentCount := rng.Intn(20) + 1
			for p := 0; p < parentCount; p++ {
				childCount := rng.Intn(8)
				docs := make([]index.Document, 0, childCount+1)
				var childMatchSets [][]string
				for c := 0; c < childCount; c++ {
					child := document.NewDocument()
					child.Add(mustStringField(t, "type", "child", false))
					matchCount := rng.Intn(4)
					var matches []string
					for m := 0; m < matchCount; m++ {
						v := values[rng.Intn(len(values))]
						matches = append(matches, v)
						child.Add(mustStringField(t, "value", v, false))
					}
					docs = append(docs, child)
					childMatchSets = append(childMatchSets, matches)
					currentDocID++
				}
				parent := document.NewDocument()
				parent.Add(mustStringField(t, "type", "parent", false))
				docs = append(docs, parent)
				if childCount > 0 {
					expectedMatches[currentDocID] = childMatchSets
				}
				currentDocID++
				addBlock(t, w, docs...)
			}
			reader, searcher := commitAndOpen(t, dir, w)

			want := computeExpectedScores(expectedMatches, joinScoreMode, needsScore, scoreOf)

			parents := newQueryBitSetParents("type", "parent")
			childQuery := randomChildQuery(values, scoreOf)
			query := NewToParentBlockJoinQuery(childQuery, parents, joinScoreMode)

			ss := supplierForLeaf(t, searcher, reader, query, needsScore)
			if ss == nil {
				if len(want) != 0 {
					t.Fatalf("nil supplier but expected %v", want)
				}
				continue
			}
			bsp := ss.(interface {
				BulkScorer() (search.BulkScorer, error)
			})
			bs, err := bsp.BulkScorer()
			if err != nil {
				t.Fatalf("BulkScorer: %v", err)
			}
			assertBulkScores(t, bs, needsScore, nil, want)
		}
	}
}

// supplierForLeaf returns the first-leaf ScorerSupplier for q (or nil when the
// query has no matches on the leaf).
func supplierForLeaf(t *testing.T, searcher *search.IndexSearcher, reader *index.DirectoryReader, q search.Query, needsScore bool) search.ScorerSupplier {
	t.Helper()
	rewritten, err := q.Rewrite(reader)
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}
	weight, err := rewritten.CreateWeight(searcher, needsScore, 1.0)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("Leaves: %v", err)
	}
	ss, err := weight.ScorerSupplier(leaves[0])
	if err != nil {
		t.Fatalf("ScorerSupplier: %v", err)
	}
	return ss
}

// randomChildQuery builds the SHOULD disjunction of boosted constant-score term
// queries used by the random test (boost == the value's score).
func randomChildQuery(values []string, scoreOf map[string]float32) *search.BooleanQuery {
	q := search.NewBooleanQuery()
	for _, v := range values {
		q.Add(boostCSQTerm("value", v, scoreOf[v]), search.SHOULD)
	}
	return q
}

// computeExpectedScores reproduces TestBlockJoinBulkScorer.computeExpectedScores:
// per parent, filter out children with no matches, then aggregate per-child
// scores (a child's score is the sum over the SET of distinct matched values)
// according to the join score mode.
func computeExpectedScores(expectedMatches map[int][][]string, joinScoreMode ScoreMode, needsScore bool, scoreOf map[string]float32) map[int]float32 {
	want := map[int]float32{}
	for parent, childSets := range expectedMatches {
		var matching [][]string
		for _, set := range childSets {
			if len(set) > 0 {
				matching = append(matching, set)
			}
		}
		if len(matching) == 0 {
			continue
		}

		var expected float64
		if needsScore {
			first := true
			for _, set := range matching {
				cs := float64(childScore(set, scoreOf))
				switch joinScoreMode {
				case Total, Avg:
					expected += cs
				case Min:
					if first || cs < expected {
						expected = cs
					}
				case Max:
					if cs > expected {
						expected = cs
					}
				case None:
				}
				first = false
			}
			if joinScoreMode == Avg {
				expected /= float64(len(matching))
			}
		}
		want[parent] = float32(expected)
	}
	return want
}

// childScore sums the scores of the DISTINCT matched values in a child doc,
// mirroring TestBlockJoinBulkScorer.computeExpectedScore (which uses a Set).
func childScore(matches []string, scoreOf map[string]float32) float32 {
	seen := map[string]bool{}
	var sum float32
	keys := append([]string(nil), matches...)
	sort.Strings(keys)
	for _, v := range keys {
		if !seen[v] {
			seen[v] = true
			sum += scoreOf[v]
		}
	}
	return sum
}

// boolSeed maps a bool to a seed offset for deterministic randomness.
func boolSeed(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// TestBlockJoinBulkScorer_ScoreModes verifies that ScoreMode constants can be
// used as ToParentBlockJoinQuery arguments, mirroring the test setup pattern.
func TestBlockJoinBulkScorer_ScoreModes(t *testing.T) {
	for _, sm := range []ScoreMode{Avg, Max, Total, Min, None} {
		q := NewToParentBlockJoinQuery(nil, nil, sm)
		if q == nil {
			t.Fatalf("NewToParentBlockJoinQuery(scoreMode=%v) returned nil", sm)
		}
		if q.GetScoreMode() != sm {
			t.Errorf("GetScoreMode() = %v, want %v", q.GetScoreMode(), sm)
		}
	}
}
