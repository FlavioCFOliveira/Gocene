// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// stubNoScoreCollector is a minimal Collector that returns COMPLETE_NO_SCORES.
type stubNoScoreCollector struct{}

func (s *stubNoScoreCollector) GetLeafCollector(_ search.IndexReader) (search.LeafCollector, error) {
	return nil, nil
}
func (s *stubNoScoreCollector) ScoreMode() search.ScoreMode { return search.COMPLETE_NO_SCORES }

var _ search.Collector = (*stubNoScoreCollector)(nil)

func TestGenericTermsCollector_NoScore(t *testing.T) {
	terms := util.NewBytesRefHash()
	c := NewGenericTermsCollectorNoScore(&stubNoScoreCollector{}, terms)

	if c.ScoreMode() != search.COMPLETE_NO_SCORES {
		t.Errorf("ScoreMode() = %v, want COMPLETE_NO_SCORES", c.ScoreMode())
	}
	if c.GetCollectedTerms() != terms {
		t.Error("GetCollectedTerms() != expected hash")
	}

	_, err := c.GetScoresPerTerm()
	if err != ErrScoresNotAvailable {
		t.Errorf("GetScoresPerTerm() error = %v, want ErrScoresNotAvailable", err)
	}
}

func TestGenericTermsCollector_WithScores(t *testing.T) {
	inner := &stubNoScoreCollector{}
	terms := util.NewBytesRefHash()
	scores := []float32{1.5, 2.5}
	c := NewGenericTermsCollectorWithScores(inner, terms, scores)

	if c.GetCollectedTerms() != terms {
		t.Error("GetCollectedTerms() != expected hash")
	}

	got, err := c.GetScoresPerTerm()
	if err != nil {
		t.Fatalf("GetScoresPerTerm(): %v", err)
	}
	if len(got) != 2 || got[0] != 1.5 || got[1] != 2.5 {
		t.Errorf("GetScoresPerTerm() = %v, want [1.5 2.5]", got)
	}
}

func TestGenericTermsCollector_GetLeafCollector(t *testing.T) {
	terms := util.NewBytesRefHash()
	c := NewGenericTermsCollectorNoScore(&stubNoScoreCollector{}, terms)
	lc, err := c.GetLeafCollector(stubIndexReaderForJoin{})
	// Inner returns nil, which is acceptable for a stub.
	if err != nil {
		t.Fatalf("GetLeafCollector: %v", err)
	}
	// lc may be nil for stub collector.
	_ = lc
}
