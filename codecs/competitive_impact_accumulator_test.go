// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"reflect"
	"testing"
)

func TestCompetitiveImpactAccumulator_Empty(t *testing.T) {
	t.Parallel()
	a := NewCompetitiveImpactAccumulator()
	if got := a.GetCompetitiveFreqNormPairs(); got != nil && len(got) != 0 {
		t.Errorf("empty accumulator returned %v, want nil/empty", got)
	}
}

func TestCompetitiveImpactAccumulator_PerNormDeduplication(t *testing.T) {
	t.Parallel()
	a := NewCompetitiveImpactAccumulator()
	a.Add(3, 5)
	a.Add(7, 5) // should overwrite freq=3 for norm=5
	a.Add(2, 5) // should be ignored
	got := a.GetCompetitiveFreqNormPairs()
	want := []Impact{{Freq: 7, Norm: 5}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestCompetitiveImpactAccumulator_PruneDominated(t *testing.T) {
	t.Parallel()
	a := NewCompetitiveImpactAccumulator()
	a.Add(1, 7)
	a.Add(3, 9)  // dominated by (7, 10) only if 7>=3 AND 10<=9 — 10<=9 false, so survives
	a.Add(7, 10) // dominates (3, 9)? need 7>=3 AND 10<=9 — false (10>9). So (3,9) NOT dominated.
	a.Add(15, 11)
	a.Add(20, 13)
	a.Add(28, 14)
	got := a.GetCompetitiveFreqNormPairs()
	// All are competitive: every later impact has a strictly higher norm,
	// so the lower-freq predecessors keep their smaller norms intact.
	want := []Impact{
		{Freq: 1, Norm: 7},
		{Freq: 3, Norm: 9},
		{Freq: 7, Norm: 10},
		{Freq: 15, Norm: 11},
		{Freq: 20, Norm: 13},
		{Freq: 28, Norm: 14},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestCompetitiveImpactAccumulator_DominationRemovesEntries(t *testing.T) {
	t.Parallel()
	a := NewCompetitiveImpactAccumulator()
	// (5, 20) is dominated by (10, 5): 10>=5 AND 5<=20.
	a.Add(5, 20)
	a.Add(10, 5)
	got := a.GetCompetitiveFreqNormPairs()
	want := []Impact{{Freq: 10, Norm: 5}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestCompetitiveImpactAccumulator_NegativeNormUnsignedCompare(t *testing.T) {
	t.Parallel()
	a := NewCompetitiveImpactAccumulator()
	// Java treats norms as unsigned. -1 (= 0xFFFFFFFFFFFFFFFF) is the
	// LARGEST unsigned norm and therefore the LEAST competitive at equal freq.
	a.Add(5, -1)
	a.Add(5, 1)
	got := a.GetCompetitiveFreqNormPairs()
	// (5, 1) dominates (5, -1) because 5>=5 AND uint64(1)<uint64(-1).
	want := []Impact{{Freq: 5, Norm: 1}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestCompetitiveImpactAccumulator_Clear(t *testing.T) {
	t.Parallel()
	a := NewCompetitiveImpactAccumulator()
	a.Add(1, 1)
	a.Add(2, 2)
	a.Clear()
	if got := a.GetCompetitiveFreqNormPairs(); got != nil && len(got) != 0 {
		t.Errorf("after Clear: got %v, want empty", got)
	}
	a.Add(99, 99)
	got := a.GetCompetitiveFreqNormPairs()
	want := []Impact{{Freq: 99, Norm: 99}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("after Clear+Add: got %v, want %v", got, want)
	}
}

func TestCompetitiveImpactAccumulator_AddAll(t *testing.T) {
	t.Parallel()
	a := NewCompetitiveImpactAccumulator()
	a.Add(3, 5)
	b := NewCompetitiveImpactAccumulator()
	b.Add(7, 5) // wins over a's (3, 5)
	b.Add(2, 9)
	a.AddAll(b)
	got := a.GetCompetitiveFreqNormPairs()
	// (2,9) is dominated by (7,5): 7>=2 AND 5<=9.
	want := []Impact{{Freq: 7, Norm: 5}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("after AddAll: got %v, want %v", got, want)
	}
}

func TestCompetitiveImpactAccumulator_AddAllNil(t *testing.T) {
	t.Parallel()
	a := NewCompetitiveImpactAccumulator()
	a.Add(1, 1)
	a.AddAll(nil) // no-op; must not panic
	got := a.GetCompetitiveFreqNormPairs()
	want := []Impact{{Freq: 1, Norm: 1}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
