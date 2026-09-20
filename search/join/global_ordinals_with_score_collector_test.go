// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

func TestGlobalOrdinalsWithScoreCollector_Min_Construction(t *testing.T) {
	c, err := NewGlobalOrdinalsWithScoreCollectorMin("f", nil, 10, 1, math.MaxInt32)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.ScoreMode() != search.COMPLETE {
		t.Errorf("ScoreMode() = %v, want COMPLETE", c.ScoreMode())
	}
	if c.GetCollectedOrds().Length() != 10 {
		t.Errorf("bitset length = %d, want 10", c.GetCollectedOrds().Length())
	}
}

func TestGlobalOrdinalsWithScoreCollector_Max_Construction(t *testing.T) {
	c, err := NewGlobalOrdinalsWithScoreCollectorMax("f", nil, 10, 1, math.MaxInt32)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.ScoreMode() != search.COMPLETE {
		t.Errorf("ScoreMode() = %v, want COMPLETE", c.ScoreMode())
	}
}

func TestGlobalOrdinalsWithScoreCollector_Sum_Construction(t *testing.T) {
	c, err := NewGlobalOrdinalsWithScoreCollectorSum("f", nil, 10, 1, math.MaxInt32)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.ScoreMode() != search.COMPLETE {
		t.Errorf("ScoreMode() = %v, want COMPLETE", c.ScoreMode())
	}
}

func TestGlobalOrdinalsWithScoreCollector_Avg_Construction(t *testing.T) {
	c, err := NewGlobalOrdinalsWithScoreCollectorAvg("f", nil, 10, 1, math.MaxInt32)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.ScoreMode() != search.COMPLETE {
		t.Errorf("ScoreMode() = %v, want COMPLETE", c.ScoreMode())
	}
}

func TestGlobalOrdinalsWithScoreCollector_NoScore_Construction(t *testing.T) {
	c, err := NewGlobalOrdinalsWithScoreCollectorNoScore("f", nil, 10, 1, math.MaxInt32)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.ScoreMode() != search.COMPLETE_NO_SCORES {
		t.Errorf("ScoreMode() = %v, want COMPLETE_NO_SCORES", c.ScoreMode())
	}
}

func TestGlobalOrdinalsWithScoreCollector_TooManyOrdinals(t *testing.T) {
	_, err := NewGlobalOrdinalsWithScoreCollectorMin("f", nil, math.MaxInt32+1, 1, math.MaxInt32)
	if err == nil {
		t.Fatal("expected error for valueCount > MaxInt32")
	}
}

func TestGlobalOrdinalsWithScoreCollector_Match_NoOccurrence(t *testing.T) {
	c, err := NewGlobalOrdinalsWithScoreCollectorNoScore("f", nil, 16, 1, math.MaxInt32)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Ordinal 5 was never collected → Match should return false.
	if c.Match(5) {
		t.Error("Match(5): expected false for uncollected ordinal")
	}
}

func TestGlobalOrdinalsWithScoreCollector_NilSdvCollect(t *testing.T) {
	c, err := NewGlobalOrdinalsWithScoreCollectorSum("f", nil, 20, 1, math.MaxInt32)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lc, err := c.GetLeafCollector(nil)
	if err != nil {
		t.Fatalf("GetLeafCollector: %v", err)
	}
	for i := 0; i < 5; i++ {
		if err := lc.Collect(i); err != nil {
			t.Fatalf("Collect(%d): %v", i, err)
		}
	}
	if c.GetCollectedOrds().Cardinality() != 0 {
		t.Errorf("expected 0 bits set, got %d", c.GetCollectedOrds().Cardinality())
	}
}

func TestOrdinalScores_SetGet(t *testing.T) {
	s := newOrdinalScores(100, 0)
	s.set(0, 1.5)
	s.set(50, 2.5)
	if got := s.get(0); got != 1.5 {
		t.Errorf("get(0) = %v, want 1.5", got)
	}
	if got := s.get(50); got != 2.5 {
		t.Errorf("get(50) = %v, want 2.5", got)
	}
	// Unset slot returns the unset value.
	if got := s.get(10); got != 0 {
		t.Errorf("get(10) = %v, want 0 (unset)", got)
	}
}

func TestOrdinalOccurrences_IncrementGet(t *testing.T) {
	o := newOrdinalOccurrences(100)
	o.increment(7)
	o.increment(7)
	o.increment(7)
	if got := o.get(7); got != 3 {
		t.Errorf("get(7) = %d, want 3", got)
	}
	if got := o.get(99); got != 0 {
		t.Errorf("get(99) = %d, want 0 (unincremented)", got)
	}
}
