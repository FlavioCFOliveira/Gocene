// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"reflect"
	"sort"
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

// sortedExpected mirrors the Java reference's TreeSet ordering: ascending by
// freq. GetCompetitiveFreqNormPairs returns its slice in that same order, so
// expectations can be compared with reflect.DeepEqual once we sort the
// reference set the same way.
func sortedExpected(impacts []Impact) []Impact {
	out := make([]Impact, len(impacts))
	copy(out, impacts)
	sort.Slice(out, func(i, j int) bool { return out[i].Freq < out[j].Freq })
	return out
}

// TestCompetitiveImpactAccumulator_Basics ports
// org.apache.lucene.codecs.TestCompetitiveFreqNormAccumulator#testBasics.
func TestCompetitiveImpactAccumulator_Basics(t *testing.T) {
	t.Parallel()
	acc := NewCompetitiveImpactAccumulator()
	expected := []Impact{}

	acc.Add(3, 5)
	expected = append(expected, Impact{Freq: 3, Norm: 5})
	if got, want := acc.GetCompetitiveFreqNormPairs(), sortedExpected(expected); !reflect.DeepEqual(got, want) {
		t.Fatalf("after Add(3,5): got %v, want %v", got, want)
	}

	acc.Add(6, 11)
	expected = append(expected, Impact{Freq: 6, Norm: 11})
	if got, want := acc.GetCompetitiveFreqNormPairs(), sortedExpected(expected); !reflect.DeepEqual(got, want) {
		t.Fatalf("after Add(6,11): got %v, want %v", got, want)
	}

	acc.Add(10, 13)
	expected = append(expected, Impact{Freq: 10, Norm: 13})
	if got, want := acc.GetCompetitiveFreqNormPairs(), sortedExpected(expected); !reflect.DeepEqual(got, want) {
		t.Fatalf("after Add(10,13): got %v, want %v", got, want)
	}

	acc.Add(1, 2)
	expected = append(expected, Impact{Freq: 1, Norm: 2})
	if got, want := acc.GetCompetitiveFreqNormPairs(), sortedExpected(expected); !reflect.DeepEqual(got, want) {
		t.Fatalf("after Add(1,2): got %v, want %v", got, want)
	}

	acc.Add(7, 9)
	// Java: expected.remove(new Impact(6, 11)); expected.add(new Impact(7, 9));
	// Rationale: (7,9) dominates (6,11) — 7>=6 AND 9<=11.
	expected = removeImpact(expected, Impact{Freq: 6, Norm: 11})
	expected = append(expected, Impact{Freq: 7, Norm: 9})
	if got, want := acc.GetCompetitiveFreqNormPairs(), sortedExpected(expected); !reflect.DeepEqual(got, want) {
		t.Fatalf("after Add(7,9): got %v, want %v", got, want)
	}

	acc.Add(8, 2)
	// Java: expected.clear(); expected.add(new Impact(10, 13)); expected.add(new Impact(8, 2));
	// (8,2) dominates (1,2),(3,5),(7,9). (10,13) survives because 10>8 and 13>2.
	expected = []Impact{{Freq: 10, Norm: 13}, {Freq: 8, Norm: 2}}
	if got, want := acc.GetCompetitiveFreqNormPairs(), sortedExpected(expected); !reflect.DeepEqual(got, want) {
		t.Fatalf("after Add(8,2): got %v, want %v", got, want)
	}
}

// TestCompetitiveImpactAccumulator_ExtremeNorms ports
// org.apache.lucene.codecs.TestCompetitiveFreqNormAccumulator#testExtremeNorms.
// Negative norms exercise Java's unsigned-long comparison: -100 and -3 are
// among the LARGEST unsigned values, so they only stay competitive at strictly
// higher freqs than the smaller (lower-magnitude positive) norms.
func TestCompetitiveImpactAccumulator_ExtremeNorms(t *testing.T) {
	t.Parallel()
	acc := NewCompetitiveImpactAccumulator()
	expected := []Impact{}

	acc.Add(3, 5)
	expected = append(expected, Impact{Freq: 3, Norm: 5})
	if got, want := acc.GetCompetitiveFreqNormPairs(), sortedExpected(expected); !reflect.DeepEqual(got, want) {
		t.Fatalf("after Add(3,5): got %v, want %v", got, want)
	}

	acc.Add(10, 10000)
	expected = append(expected, Impact{Freq: 10, Norm: 10000})
	if got, want := acc.GetCompetitiveFreqNormPairs(), sortedExpected(expected); !reflect.DeepEqual(got, want) {
		t.Fatalf("after Add(10,10000): got %v, want %v", got, want)
	}

	acc.Add(5, 200)
	expected = append(expected, Impact{Freq: 5, Norm: 200})
	if got, want := acc.GetCompetitiveFreqNormPairs(), sortedExpected(expected); !reflect.DeepEqual(got, want) {
		t.Fatalf("after Add(5,200): got %v, want %v", got, want)
	}

	acc.Add(20, -100)
	expected = append(expected, Impact{Freq: 20, Norm: -100})
	if got, want := acc.GetCompetitiveFreqNormPairs(), sortedExpected(expected); !reflect.DeepEqual(got, want) {
		t.Fatalf("after Add(20,-100): got %v, want %v", got, want)
	}

	acc.Add(30, -3)
	expected = append(expected, Impact{Freq: 30, Norm: -3})
	if got, want := acc.GetCompetitiveFreqNormPairs(), sortedExpected(expected); !reflect.DeepEqual(got, want) {
		t.Fatalf("after Add(30,-3): got %v, want %v", got, want)
	}
}

// TestCompetitiveImpactAccumulator_Copy ports
// org.apache.lucene.codecs.TestCompetitiveFreqNormAccumulator#testCopy.
// At every step Copy(src) must produce the same competitive set as
// Clear()+AddAll(src), exercising both production operations together.
func TestCompetitiveImpactAccumulator_Copy(t *testing.T) {
	t.Parallel()
	acc := NewCompetitiveImpactAccumulator()
	copied := NewCompetitiveImpactAccumulator()
	merged := NewCompetitiveImpactAccumulator()

	steps := []struct {
		freq int
		norm int64
	}{
		{3, 5},
		{10, 10000},
		{5, 200},
		{20, -100},
		{30, -3},
	}

	for i, step := range steps {
		acc.Add(step.freq, step.norm)

		copied.Copy(acc)
		if got, want := copied.GetCompetitiveFreqNormPairs(), acc.GetCompetitiveFreqNormPairs(); !reflect.DeepEqual(got, want) {
			t.Fatalf("step %d Copy mismatch: got %v, want %v", i, got, want)
		}

		merged.Clear()
		merged.AddAll(acc)
		if got, want := copied.GetCompetitiveFreqNormPairs(), merged.GetCompetitiveFreqNormPairs(); !reflect.DeepEqual(got, want) {
			t.Fatalf("step %d Copy vs AddAll mismatch: got %v, want %v", i, got, want)
		}
	}
}

// TestCompetitiveImpactAccumulator_OmitFreqs ports
// org.apache.lucene.codecs.TestCompetitiveFreqNormAccumulator#testOmitFreqs.
// All freqs equal: the highest unsigned norm at that freq dominates because
// dominance requires norm' <= norm — equal freq lets the smaller norm win.
// Java's expectation is Impact(1, 4): freq=1, norm=4 — the smallest norm.
func TestCompetitiveImpactAccumulator_OmitFreqs(t *testing.T) {
	t.Parallel()
	acc := NewCompetitiveImpactAccumulator()
	acc.Add(1, 5)
	acc.Add(1, 7)
	acc.Add(1, 4)
	got := acc.GetCompetitiveFreqNormPairs()
	want := []Impact{{Freq: 1, Norm: 4}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestCompetitiveImpactAccumulator_OmitNorms ports
// org.apache.lucene.codecs.TestCompetitiveFreqNormAccumulator#testOmitNorms.
// All norms equal: the highest freq dominates. Java's expectation is
// Impact(7, 1).
func TestCompetitiveImpactAccumulator_OmitNorms(t *testing.T) {
	t.Parallel()
	acc := NewCompetitiveImpactAccumulator()
	acc.Add(5, 1)
	acc.Add(7, 1)
	acc.Add(4, 1)
	got := acc.GetCompetitiveFreqNormPairs()
	want := []Impact{{Freq: 7, Norm: 1}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// removeImpact returns a copy of impacts with the first match of target
// removed; matches Java TreeSet#remove semantics for the testBasics fixture.
func removeImpact(impacts []Impact, target Impact) []Impact {
	out := make([]Impact, 0, len(impacts))
	removed := false
	for _, imp := range impacts {
		if !removed && imp == target {
			removed = true
			continue
		}
		out = append(out, imp)
	}
	return out
}
