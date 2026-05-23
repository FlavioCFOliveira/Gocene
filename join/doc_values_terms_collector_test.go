// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestDocValuesTermsCollector_ScoreMode verifies the base collector always uses COMPLETE_NO_SCORES.
func TestDocValuesTermsCollector_ScoreMode(t *testing.T) {
	c := newDocValuesTermsCollector[any](nil, nil, search.COMPLETE_NO_SCORES)
	if c.ScoreMode() != search.COMPLETE_NO_SCORES {
		t.Errorf("ScoreMode() = %v, want COMPLETE_NO_SCORES", c.ScoreMode())
	}
}

// TestDocValuesTermsCollector_GetLeafCollector_NilDvFunc verifies that
// GetLeafCollector returns a non-nil leaf collector even when dvFunc is nil.
func TestDocValuesTermsCollector_GetLeafCollector_NilDvFunc(t *testing.T) {
	c := newDocValuesTermsCollector[any](nil, nil, search.COMPLETE_NO_SCORES)
	lc, err := c.GetLeafCollector(stubIndexReaderForJoin{})
	if err != nil {
		t.Fatalf("GetLeafCollector: %v", err)
	}
	if lc == nil {
		t.Fatal("expected non-nil leaf collector")
	}
	// SetScorer must not error.
	if err := lc.SetScorer(nil); err != nil {
		t.Fatalf("SetScorer: %v", err)
	}
	// Collect must not error when collectFn is nil.
	if err := lc.Collect(0); err != nil {
		t.Fatalf("Collect: %v", err)
	}
}

// TestDocValuesTermsCollector_CollectFn verifies the collectFn is called.
func TestDocValuesTermsCollector_CollectFn(t *testing.T) {
	var called []int
	collectFn := func(_ any, doc int) error {
		called = append(called, doc)
		return nil
	}
	c := newDocValuesTermsCollector[any](nil, collectFn, search.COMPLETE_NO_SCORES)
	lc, _ := c.GetLeafCollector(stubIndexReaderForJoin{})
	_ = lc.Collect(3)
	_ = lc.Collect(7)

	if len(called) != 2 || called[0] != 3 || called[1] != 7 {
		t.Errorf("collectFn calls = %v, want [3 7]", called)
	}
}
