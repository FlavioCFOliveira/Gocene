// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.sandbox.search.ProfilerCollectorWrapper tests.
// (No dedicated Java test peer was identified; tests verify the observable timing
// contract directly.)
package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// countingCollector is a minimal search.Collector that counts Collect calls.
type countingCollector struct {
	collectCount int
	scorerSet    int
}

func (c *countingCollector) ScoreMode() search.ScoreMode { return search.COMPLETE }

func (c *countingCollector) GetLeafCollector(_ *index.LeafReaderContext) (search.LeafCollector, error) {
	return c, nil
}

func (c *countingCollector) SetScorer(_ search.Scorer) error {
	c.scorerSet++
	return nil
}

func (c *countingCollector) Collect(_ int) error {
	c.collectCount++
	return nil
}

var _ search.Collector = (*countingCollector)(nil)
var _ search.LeafCollector = (*countingCollector)(nil)

// TestProfilerCollectorWrapper_TimerPositiveAfterScoreMode verifies that
// GetTime is positive after ScoreMode is called.
func TestProfilerCollectorWrapper_TimerPositiveAfterScoreMode(t *testing.T) {
	inner := &countingCollector{}
	pw := NewProfilerCollectorWrapper(inner)
	_ = pw.ScoreMode()
	if pw.GetTime() <= 0 {
		t.Errorf("GetTime() = %d after ScoreMode; want > 0", pw.GetTime())
	}
}

// TestProfilerCollectorWrapper_ScoreModeDelegate verifies that ScoreMode
// returns the inner collector's score mode.
func TestProfilerCollectorWrapper_ScoreModeDelegate(t *testing.T) {
	inner := &countingCollector{}
	pw := NewProfilerCollectorWrapper(inner)
	if got := pw.ScoreMode(); got != search.COMPLETE {
		t.Errorf("ScoreMode() = %v; want COMPLETE", got)
	}
}

// TestProfilerCollectorWrapper_TimerAccumulatesAcrossCalls verifies that
// GetTime grows with each additional timed call.
func TestProfilerCollectorWrapper_TimerAccumulatesAcrossCalls(t *testing.T) {
	inner := &countingCollector{}
	pw := NewProfilerCollectorWrapper(inner)

	lc, err := pw.GetLeafCollector(nil)
	if err != nil {
		t.Fatal(err)
	}
	t1 := pw.GetTime()
	if t1 <= 0 {
		t.Errorf("time after GetLeafCollector = %d; want > 0", t1)
	}

	if err := lc.Collect(0); err != nil {
		t.Fatal(err)
	}
	t2 := pw.GetTime()
	if t2 <= t1 {
		t.Errorf("time after Collect = %d; should be > %d", t2, t1)
	}

	if err := lc.SetScorer(nil); err != nil {
		t.Fatal(err)
	}
	t3 := pw.GetTime()
	if t3 <= t2 {
		t.Errorf("time after SetScorer = %d; should be > %d", t3, t2)
	}
}

// TestProfilerCollectorWrapper_CollectDelegated verifies that Collect calls
// are forwarded to the inner collector.
func TestProfilerCollectorWrapper_CollectDelegated(t *testing.T) {
	inner := &countingCollector{}
	pw := NewProfilerCollectorWrapper(inner)

	lc, err := pw.GetLeafCollector(nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if err := lc.Collect(i); err != nil {
			t.Fatal(err)
		}
	}
	if inner.collectCount != 5 {
		t.Errorf("inner.collectCount = %d; want 5", inner.collectCount)
	}
}

// TestProfilerCollectorWrapper_SetScorerDelegated verifies SetScorer calls
// are forwarded to the inner collector.
func TestProfilerCollectorWrapper_SetScorerDelegated(t *testing.T) {
	inner := &countingCollector{}
	pw := NewProfilerCollectorWrapper(inner)

	lc, err := pw.GetLeafCollector(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := lc.SetScorer(nil); err != nil {
		t.Fatal(err)
	}
	if inner.scorerSet != 1 {
		t.Errorf("inner.scorerSet = %d; want 1", inner.scorerSet)
	}
}
