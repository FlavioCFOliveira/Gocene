// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/BatchScoreBulkScorer.java
//   (no Java test peer located; tests cover the Go public contract.)

package search_test

import (
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// fullWindowScore drives a BulkScorer over [0, NO_MORE_DOCS), the analogue of
// score(collector, acceptDocs, 0, NO_MORE_DOCS) in the Lucene tests.
func fullWindowScore(bs search.BulkScorer, lc search.LeafCollector, acceptDocs util.Bits) error {
	_, err := bs.Score(lc, acceptDocs, 0, search.NO_MORE_DOCS)
	return err
}

// bitsOf returns a util.Bits of the given length with the listed docs set.
func bitsOf(length int, docs ...int) util.Bits {
	bs, err := util.NewFixedBitSet(length)
	if err != nil {
		panic(err)
	}
	for _, d := range docs {
		bs.Set(d)
	}
	return bs
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// batchLeafCollector is a minimal LeafCollector that records docs/scores.
type batchLeafCollector struct {
	scorer search.Scorer
	docs   []int
	scores []float32
}

func (c *batchLeafCollector) SetScorer(s search.Scorer) error { c.scorer = s; return nil }
func (c *batchLeafCollector) Collect(doc int) error {
	c.docs = append(c.docs, doc)
	if c.scorer != nil {
		c.scores = append(c.scores, c.scorer.Score())
	}
	return nil
}
func (c *batchLeafCollector) ScoreMode() search.ScoreMode { return search.COMPLETE }
func (c *batchLeafCollector) GetLeafCollector(_ *index.LeafReaderContext) (search.LeafCollector, error) {
	return c, nil
}

// batchLeafCollectorWithError returns an error on the first Collect call.
type batchLeafCollectorWithError struct {
	batchLeafCollector
}

func (c *batchLeafCollectorWithError) Collect(_ int) error {
	return errors.New("collect error")
}

// ─── tests ───────────────────────────────────────────────────────────────────

// TestBatchScoreBulkScorer_CollectsAllDocs verifies all matching docs are
// collected in order when acceptDocs is nil.
func TestBatchScoreBulkScorer_CollectsAllDocs(t *testing.T) {
	scorer := newConstantScorer([]int{0, 2, 4, 6}, 1.5, 1.5)
	bs := search.NewBatchScoreBulkScorer(scorer)

	lc := &batchLeafCollector{}
	if err := fullWindowScore(bs, lc, nil); err != nil {
		t.Fatalf("Score error: %v", err)
	}
	wantDocs := []int{0, 2, 4, 6}
	if len(lc.docs) != len(wantDocs) {
		t.Fatalf("docs=%v, want %v", lc.docs, wantDocs)
	}
	for i, d := range wantDocs {
		if lc.docs[i] != d {
			t.Errorf("docs[%d]=%d, want %d", i, lc.docs[i], d)
		}
	}
}

// TestBatchScoreBulkScorer_ScoreInjected verifies the scorer is injected
// into the LeafCollector via SetScorer.
func TestBatchScoreBulkScorer_ScoreInjected(t *testing.T) {
	scorer := newConstantScorer([]int{3}, 2.5, 2.5)
	bs := search.NewBatchScoreBulkScorer(scorer)
	lc := &batchLeafCollector{}
	if err := fullWindowScore(bs, lc, nil); err != nil {
		t.Fatalf("Score error: %v", err)
	}
	if lc.scorer == nil {
		t.Fatal("SetScorer was not called")
	}
	if len(lc.scores) != 1 || lc.scores[0] != 2.5 {
		t.Errorf("scores=%v, want [2.5]", lc.scores)
	}
}

// TestBatchScoreBulkScorer_Cost verifies Cost() equals the scorer's Cost().
func TestBatchScoreBulkScorer_Cost(t *testing.T) {
	scorer := newConstantScorer([]int{0, 1, 2, 3, 4}, 1, 1)
	bs := search.NewBatchScoreBulkScorer(scorer)
	if bs.Cost() != 5 {
		t.Errorf("Cost()=%d, want 5", bs.Cost())
	}
}

// TestBatchScoreBulkScorer_EmptyScorer verifies no docs are collected for an
// empty scorer.
func TestBatchScoreBulkScorer_EmptyScorer(t *testing.T) {
	scorer := newConstantScorer([]int{}, 1, 1)
	bs := search.NewBatchScoreBulkScorer(scorer)
	lc := &batchLeafCollector{}
	if err := fullWindowScore(bs, lc, nil); err != nil {
		t.Fatalf("Score error: %v", err)
	}
	if len(lc.docs) != 0 {
		t.Errorf("docs=%v, want empty", lc.docs)
	}
}

// TestBatchScoreBulkScorer_ImplementsBulkScorer checks interface satisfaction.
func TestBatchScoreBulkScorer_ImplementsBulkScorer(t *testing.T) {
	scorer := newConstantScorer([]int{0}, 1, 1)
	var _ search.BulkScorer = search.NewBatchScoreBulkScorer(scorer)
}

// TestBatchScoreBulkScorer_AcceptDocsFilters verifies docs filtered by
// acceptDocs are skipped.
func TestBatchScoreBulkScorer_AcceptDocsFilters(t *testing.T) {
	scorer := newConstantScorer([]int{0, 1, 2, 3, 4}, 1, 1)
	bs := search.NewBatchScoreBulkScorer(scorer)
	// acceptDocs covers only docs 1, 2 and 3.
	acceptDocs := bitsOf(5, 1, 2, 3)
	lc := &batchLeafCollector{}
	if err := fullWindowScore(bs, lc, acceptDocs); err != nil {
		t.Fatalf("Score error: %v", err)
	}
	// Expect docs 1,2,3 (those that appear in both scorer and acceptDocs).
	wantDocs := []int{1, 2, 3}
	if len(lc.docs) != len(wantDocs) {
		t.Fatalf("docs=%v, want %v", lc.docs, wantDocs)
	}
	for i, d := range wantDocs {
		if lc.docs[i] != d {
			t.Errorf("docs[%d]=%d, want %d", i, lc.docs[i], d)
		}
}	}
