// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.sandbox.search.TestQueryProfilerWeight.
package search

import (
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// fakeInnerScorerSupplier is a ScorerSupplier whose cost changes after
// SetTopLevelScoringClause is called (mirroring the Java FakeWeight's supplier).
type fakeInnerScorerSupplier struct {
	cost int64
}

func (s *fakeInnerScorerSupplier) Get(_ int64) (search.Scorer, error) {
	return &fakeScorer{maxScore: 42}, nil
}

func (s *fakeInnerScorerSupplier) Cost() int64 { return s.cost }

func (s *fakeInnerScorerSupplier) SetTopLevelScoringClause() error { s.cost = 42; return nil }

var _ search.ScorerSupplier = (*fakeInnerScorerSupplier)(nil)

// fakeScorer is a minimal scorer that returns a fixed max score.
type fakeScorer struct {
	search.BaseDocIdSetIterator
	maxScore float32
}

func (s *fakeScorer) Score() (float32, error)            { return 0, nil }
func (s *fakeScorer) GetMaxScore(_ int) (float32, error) { return s.maxScore, nil }
func (s *fakeScorer) AdvanceShallow(int) (int, error) {
	return search.NO_MORE_DOCS, nil
}
func (s *fakeScorer) DocIDRunEnd() (int, error)  { return s.DocID() + 1, nil }
func (s *fakeScorer) NextDoc() (int, error)      { return search.NO_MORE_DOCS, nil }
func (s *fakeScorer) Advance(_ int) (int, error) { return search.NO_MORE_DOCS, nil }

var _ search.Scorer = (*fakeScorer)(nil)

// fakeWeight is a minimal Weight that delegates to fakeInnerScorerSupplier and
// returns a canned Explanation.
type fakeWeight struct {
	search.BaseWeight
}

func newFakeWeight(q search.Query) *fakeWeight {
	return &fakeWeight{BaseWeight: *search.NewBaseWeight(q)}
}

func (fw *fakeWeight) ScorerSupplier(_ *index.LeafReaderContext) (search.ScorerSupplier, error) {
	return &fakeInnerScorerSupplier{}, nil
}

func (fw *fakeWeight) Explain(_ *index.LeafReaderContext, _ int) (search.Explanation, error) {
	return search.MatchExplanation(1, "fake_description"), nil
}

func (fw *fakeWeight) IsCacheable(_ *index.LeafReaderContext) bool { return false }

func (fw *fakeWeight) Scorer(ctx *index.LeafReaderContext) (search.Scorer, error) {
	sup, err := fw.ScorerSupplier(ctx)
	if err != nil {
		return nil, err
	}
	if sup == nil {
		return nil, nil
	}
	return sup.Get(0)
}

func (fw *fakeWeight) BulkScorer(ctx *index.LeafReaderContext) (search.BulkScorer, error) {
	scorer, err := fw.Scorer(ctx)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return search.NewDefaultBulkScorer(scorer), nil
}

var _ search.Weight = (*fakeWeight)(nil)

// TestQueryProfilerWeight_PropagateExplain verifies that Explain is delegated.
func TestQueryProfilerWeight_PropagateExplain(t *testing.T) {
	fw := newFakeWeight(nil)
	profile := newQueryProfilerBreakdown()
	pw := newQueryProfilerWeight(fw, profile)

	exp, err := pw.Explain(nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	if exp.GetDescription() != "fake_description" {
		t.Errorf("description = %q; want %q", exp.GetDescription(), "fake_description")
	}
}

// TestQueryProfilerWeight_PropagateScorer verifies that the scorer returned is a
// QueryProfilerScorer that delegates GetMaxScore to the inner scorer.
func TestQueryProfilerWeight_PropagateScorer(t *testing.T) {
	fw := newFakeWeight(nil)
	profile := newQueryProfilerBreakdown()
	pw := newQueryProfilerWeight(fw, profile)

	scorer, err := pw.Scorer(nil)
	if err != nil {
		t.Fatal(err)
	}
	if scorer == nil {
		t.Fatal("Scorer() returned nil")
	}
	got, err := scorer.GetMaxScore(search.NO_MORE_DOCS)
	if err != nil {
		t.Fatalf("scorer.GetMaxScore: %v", err)
	}
	if got != 42 {
		t.Errorf("GetMaxScore = %v; want 42", got)
	}
}

// TestQueryProfilerWeight_PropagateTopLevelScoringClause verifies that
// SetTopLevelScoringClause is delegated and affects Cost() on the supplier.
func TestQueryProfilerWeight_PropagateTopLevelScoringClause(t *testing.T) {
	fw := newFakeWeight(nil)
	profile := newQueryProfilerBreakdown()
	pw := newQueryProfilerWeight(fw, profile)

	supplier, err := pw.ScorerSupplier(nil)
	if err != nil {
		t.Fatal(err)
	}
	if supplier == nil {
		t.Fatal("ScorerSupplier() returned nil")
	}
	supplier.SetTopLevelScoringClause()
	if got := supplier.Cost(); got != 42 {
		t.Errorf("Cost() after SetTopLevelScoringClause = %d; want 42", got)
	}
}

// TestQueryProfilerWeight_BuildScorerTimerIncrements verifies that calling
// ScorerSupplier increments the BuildScorer timer.
func TestQueryProfilerWeight_BuildScorerTimerIncrements(t *testing.T) {
	fw := newFakeWeight(nil)
	profile := newQueryProfilerBreakdown()
	pw := newQueryProfilerWeight(fw, profile)

	for i := 0; i < 5; i++ {
		sup, err := pw.ScorerSupplier(nil)
		if err != nil {
			t.Fatal(err)
		}
		if sup == nil {
			t.Fatal("unexpected nil supplier")
		}
	}
	timer := profile.GetTimer(TimingTypeBuildScorer)
	if timer.GetCount() != 5 {
		t.Errorf("BuildScorer count = %d; want 5", timer.GetCount())
	}
}

// TestQueryProfilerWeight_CountTimerIncrements verifies that calling Count
// increments the Count timer.
func TestQueryProfilerWeight_CountTimerIncrements(t *testing.T) {
	fw := newFakeWeight(nil)
	profile := newQueryProfilerBreakdown()
	pw := newQueryProfilerWeight(fw, profile)

	for i := 0; i < 3; i++ {
		if _, err := pw.Count(nil); err != nil {
			t.Fatal(err)
		}
	}
	timer := profile.GetTimer(TimingTypeCount)
	if timer.GetCount() != 3 {
		t.Errorf("Count timer count = %d; want 3", timer.GetCount())
	}
}

// TestQueryProfilerWeight_IsCacheable verifies that IsCacheable always returns false.
func TestQueryProfilerWeight_IsCacheable(t *testing.T) {
	fw := newFakeWeight(nil)
	profile := newQueryProfilerBreakdown()
	pw := newQueryProfilerWeight(fw, profile)
	if pw.IsCacheable(nil) {
		t.Error("IsCacheable() should return false")
	}
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene
// 10.5.0, which every subclass inherits unless it overrides it.
func (s *fakeScorer) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(s, upTo, bitSet, offset)
}

// Cost is abstract in Lucene's DocIdSetIterator; this double does not support it.
func (s *fakeScorer) Cost() int64 {
	panic("fakeScorer.Cost: unsupported operation")
}

// DocID is abstract in Lucene's DocIdSetIterator; this double does not support it.
func (s *fakeScorer) DocID() int {
	panic("fakeScorer.DocID: unsupported operation")
}

// BulkScorer is abstract in Lucene's ScorerSupplier; this double does not support it.
func (s *fakeInnerScorerSupplier) BulkScorer() (search.BulkScorer, error) {
	return nil, errors.New("fakeInnerScorerSupplier.BulkScorer: unsupported operation")
}

// GetChildren carries the default body Lucene gives Scorer.GetChildren.
func (s *fakeScorer) GetChildren() ([]search.ChildScorable, error) {
	return nil, nil
}

// Iterator is abstract in Lucene's Scorer; this double does not support it.
func (s *fakeScorer) Iterator() search.DocIdSetIterator {
	panic("fakeScorer.Iterator: unsupported operation")
}

// NextDocsAndScores carries the default body Lucene gives Scorer.NextDocsAndScores.
func (s *fakeScorer) NextDocsAndScores(upTo int, liveDocs util.Bits, buffer *search.DocAndFloatFeatureBuffer) error {
	return search.DefaultNextDocsAndScores(s, upTo, liveDocs, buffer)
}

// SetMinCompetitiveScore carries the default body Lucene gives Scorer.SetMinCompetitiveScore.
func (s *fakeScorer) SetMinCompetitiveScore(minScore float32) error {
	return nil
}

// SmoothingScore carries the default body Lucene gives Scorer.SmoothingScore.
func (s *fakeScorer) SmoothingScore(docID int) (float32, error) {
	return 0, nil
}

// TwoPhaseIterator carries the default body Lucene gives Scorer.TwoPhaseIterator.
func (s *fakeScorer) TwoPhaseIterator() *search.TwoPhaseIterator {
	return search.DefaultTwoPhaseIterator()
}
