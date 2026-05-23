// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// stubIndexReaderForJoin is a minimal search.IndexReader stub used in join tests.
type stubIndexReaderForJoin struct{}

func (stubIndexReaderForJoin) DocCount() int { return 0 }
func (stubIndexReaderForJoin) NumDocs() int  { return 0 }
func (stubIndexReaderForJoin) MaxDoc() int   { return 0 }

// TestGlobalOrdinalsCollector_Construction verifies that NewGlobalOrdinalsCollector
// creates the collector correctly with the expected bitset size.
func TestGlobalOrdinalsCollector_Construction(t *testing.T) {
	c, err := NewGlobalOrdinalsCollector("field", nil, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil collector")
	}
	if c.GetCollectedOrds() == nil {
		t.Fatal("expected non-nil bitset")
	}
	if c.GetCollectedOrds().Length() != 10 {
		t.Errorf("bitset length = %d, want 10", c.GetCollectedOrds().Length())
	}
}

// TestGlobalOrdinalsCollector_ScoreMode verifies the collector requires no scores.
func TestGlobalOrdinalsCollector_ScoreMode(t *testing.T) {
	c, err := NewGlobalOrdinalsCollector("f", nil, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := c.ScoreMode(); got != search.COMPLETE_NO_SCORES {
		t.Errorf("ScoreMode() = %v, want COMPLETE_NO_SCORES", got)
	}
}

// TestGlobalOrdinalsCollector_NilSdvLeafCollect verifies that when sdv is nil
// (reader cannot provide doc-values), Collect is a no-op and does not panic.
func TestGlobalOrdinalsCollector_NilSdvLeafCollect(t *testing.T) {
	c, err := NewGlobalOrdinalsCollector("f", nil, 16)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// GetLeafCollector on a nil-sdv path (no *index.LeafReader in the stub reader).
	lc, err := c.GetLeafCollector(stubIndexReaderForJoin{})
	if err != nil {
		t.Fatalf("GetLeafCollector: %v", err)
	}
	if err := lc.SetScorer(nil); err != nil {
		t.Fatalf("SetScorer: %v", err)
	}
	// Collect should be a no-op when sdv is nil.
	for i := 0; i < 5; i++ {
		if err := lc.Collect(i); err != nil {
			t.Fatalf("Collect(%d): %v", i, err)
		}
	}
	// Nothing should have been set.
	if c.GetCollectedOrds().Cardinality() != 0 {
		t.Errorf("expected 0 bits set, got %d", c.GetCollectedOrds().Cardinality())
	}
}

// TestGlobalOrdinalsCollector_GetCollectedOrds verifies initial bitset is empty.
func TestGlobalOrdinalsCollector_GetCollectedOrds(t *testing.T) {
	const valueCount = 100
	c, err := NewGlobalOrdinalsCollector("myfield", nil, valueCount)
	if err != nil {
		t.Fatalf("NewGlobalOrdinalsCollector: %v", err)
	}
	bs := c.GetCollectedOrds()
	if bs.Length() != valueCount {
		t.Errorf("Length() = %d, want %d", bs.Length(), valueCount)
	}
	if bs.Cardinality() != 0 {
		t.Errorf("Cardinality() = %d, want 0 (freshly constructed)", bs.Cardinality())
	}
}
