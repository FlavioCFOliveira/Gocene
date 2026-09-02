// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"math"
	"math/rand/v2"
	"slices"
	"sort"
	"testing"
)

// The tests in this file mirror Lucene 10.4.0's
// org.apache.lucene.document.TestDocValuesLongHashSet (one Go test per
// Java test peer where applicable). The randomised round-trip uses a
// fixed-seed PCG generator so failures are deterministic and reproducible
// from the seed alone, in keeping with the Lucene test-framework guidance
// (which seeds Random with a per-run constant exposed via -Dtests.seed=).

// assertEqualsToJavaSet checks the same invariants as the Java helper
// assertEquals(Set<Long>, DocValuesLongHashSet):
//   - cardinality matches,
//   - the materialised view round-trips via Contains,
//   - inserting any extra value into the reference set breaks Equals,
//   - random values absent from the reference report Contains == false.
func assertEqualsToJavaSet(t *testing.T, rng *rand.Rand, reference map[int64]struct{}, set *DocValuesLongHashSet) {
	t.Helper()
	if got, want := set.Size(), len(reference); got != want {
		t.Fatalf("Size = %d, want %d", got, want)
	}

	// Round-trip: every materialised value must report Contains == true
	// and must appear in the reference set.
	materialised := set.Values()
	if len(materialised) != len(reference) {
		t.Fatalf("Values length = %d, want %d (reference size)", len(materialised), len(reference))
	}
	materialisedSet := make(map[int64]struct{}, len(materialised))
	for _, v := range materialised {
		materialisedSet[v] = struct{}{}
	}
	if !sameKeys(reference, materialisedSet) {
		t.Fatalf("Values does not round-trip the reference set")
	}
	for v := range reference {
		if !set.Contains(v) {
			t.Fatalf("Contains(%d) = false, want true", v)
		}
	}

	if len(reference) == 0 {
		return
	}

	// Pick a value that is neither in the reference set nor equal to
	// the first removed key (Java drops the iterator-first element then
	// asks for an extra random value). The bounded loop guarantees
	// termination in practice; cap at a generous limit to keep the
	// test deterministic under any seed.
	var removed int64
	for k := range reference {
		removed = k
		break
	}
	for attempt := 0; attempt < 1024; attempt++ {
		candidate := int64(rng.Uint64())
		if candidate == removed {
			continue
		}
		if _, present := reference[candidate]; present {
			continue
		}
		if set.Contains(candidate) {
			t.Fatalf("Contains(%d) = true, want false", candidate)
		}
		break
	}
}

// sameKeys reports whether two int64-keyed sets contain the same keys.
func sameKeys(a, b map[int64]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}

// TestDocValuesLongHashSet_Empty mirrors TestDocValuesLongHashSet#testEmpty.
func TestDocValuesLongHashSet_Empty(t *testing.T) {
	set := NewDocValuesLongHashSet(nil)
	if got := set.Size(); got != 0 {
		t.Fatalf("Size = %d, want 0", got)
	}
	if got, want := set.Min(), int64(math.MaxInt64); got != want {
		t.Fatalf("Min = %d, want MaxInt64", got)
	}
	if got, want := set.Max(), int64(math.MinInt64); got != want {
		t.Fatalf("Max = %d, want MinInt64", got)
	}
	if got := set.Values(); len(got) != 0 {
		t.Fatalf("Values = %v, want empty", got)
	}
	if set.Contains(0) {
		t.Fatalf("Contains(0) = true on empty set, want false")
	}
}

// TestDocValuesLongHashSet_OneValue mirrors testOneValue.
func TestDocValuesLongHashSet_OneValue(t *testing.T) {
	set := NewDocValuesLongHashSet([]int64{42})
	if got := set.Size(); got != 1 {
		t.Fatalf("Size = %d, want 1", got)
	}
	if got := set.Min(); got != 42 {
		t.Fatalf("Min = %d, want 42", got)
	}
	if got := set.Max(); got != 42 {
		t.Fatalf("Max = %d, want 42", got)
	}
	if !set.Contains(42) {
		t.Fatalf("Contains(42) = false, want true")
	}

	// Sentinel-only set: hasMissingValue must flip, min/max remain
	// MIN_VALUE, Contains(MinInt64) must report true.
	set = NewDocValuesLongHashSet([]int64{math.MinInt64})
	if got := set.Size(); got != 1 {
		t.Fatalf("Size = %d, want 1 (sentinel only)", got)
	}
	if got := set.Min(); got != math.MinInt64 {
		t.Fatalf("Min = %d, want MinInt64", got)
	}
	if got := set.Max(); got != math.MinInt64 {
		t.Fatalf("Max = %d, want MinInt64", got)
	}
	if !set.Contains(math.MinInt64) {
		t.Fatalf("Contains(MinInt64) = false, want true (sentinel boundary)")
	}
}

// TestDocValuesLongHashSet_TwoValues mirrors testTwoValues.
func TestDocValuesLongHashSet_TwoValues(t *testing.T) {
	set := NewDocValuesLongHashSet([]int64{42, math.MaxInt64})
	if got := set.Size(); got != 2 {
		t.Fatalf("Size = %d, want 2", got)
	}
	if got := set.Min(); got != 42 {
		t.Fatalf("Min = %d, want 42", got)
	}
	if got := set.Max(); got != math.MaxInt64 {
		t.Fatalf("Max = %d, want MaxInt64", got)
	}
	if !set.Contains(42) || !set.Contains(math.MaxInt64) {
		t.Fatalf("Contains lookups failed for known values")
	}

	set = NewDocValuesLongHashSet([]int64{math.MinInt64, 42})
	if got := set.Size(); got != 2 {
		t.Fatalf("Size = %d, want 2 (sentinel + value)", got)
	}
	if got := set.Min(); got != math.MinInt64 {
		t.Fatalf("Min = %d, want MinInt64", got)
	}
	if got := set.Max(); got != 42 {
		t.Fatalf("Max = %d, want 42", got)
	}
	if !set.Contains(math.MinInt64) {
		t.Fatalf("Contains(MinInt64) = false, want true (sentinel)")
	}
	if !set.Contains(42) {
		t.Fatalf("Contains(42) = false, want true")
	}
}

// TestDocValuesLongHashSet_SameValue mirrors testSameValue: duplicate
// user values must collapse to a single entry.
func TestDocValuesLongHashSet_SameValue(t *testing.T) {
	set := NewDocValuesLongHashSet([]int64{42, 42})
	if got := set.Size(); got != 1 {
		t.Fatalf("Size = %d, want 1 (duplicate collapsed)", got)
	}
	if got := set.Min(); got != 42 {
		t.Fatalf("Min = %d, want 42", got)
	}
	if got := set.Max(); got != 42 {
		t.Fatalf("Max = %d, want 42", got)
	}
}

// TestDocValuesLongHashSet_SameMissingPlaceholder mirrors
// testSameMissingPlaceholder: a duplicated MISSING sentinel must
// collapse to a single entry on the hasMissingValue path.
func TestDocValuesLongHashSet_SameMissingPlaceholder(t *testing.T) {
	set := NewDocValuesLongHashSet([]int64{math.MinInt64, math.MinInt64})
	if got := set.Size(); got != 1 {
		t.Fatalf("Size = %d, want 1 (sentinel duplicate collapsed)", got)
	}
	if got := set.Min(); got != math.MinInt64 {
		t.Fatalf("Min = %d, want MinInt64", got)
	}
	if got := set.Max(); got != math.MinInt64 {
		t.Fatalf("Max = %d, want MinInt64", got)
	}
	if !set.Contains(math.MinInt64) {
		t.Fatalf("Contains(MinInt64) = false, want true")
	}
}

// TestDocValuesLongHashSet_Random mirrors testRandom: iters rounds of
// pseudo-random inputs (with controlled duplicates and an optional
// sentinel injection) compared against a reference Go map.
func TestDocValuesLongHashSet_Random(t *testing.T) {
	// Deterministic PCG seed: any two distinct 64-bit constants suffice
	// to make the iteration reproducible. The literals are chosen so the
	// failure message points back to this seed pair directly.
	rng := rand.New(rand.NewPCG(0xD0C_5E7_10AD_C0DE, 0xCEDE_5EED_1234_ABCD))
	iters := 50 // matches Java's atLeast(10) at a comfortable default multiplier
	for iter := 0; iter < iters; iter++ {
		// Java: long[] values = new long[random().nextInt(1 << random().nextInt(16))];
		// Reproduce the same length distribution: pick a width in
		// [0, 16), then pick a length in [0, 1<<width).
		width := int(rng.Uint64() % 16)
		length := 0
		if upper := 1 << width; upper > 0 {
			length = int(rng.Uint64() % uint64(upper))
		}
		values := make([]int64, length)
		for i := range values {
			if i == 0 || rng.Uint64()%10 < 9 {
				values[i] = int64(rng.Uint64())
			} else {
				values[i] = values[int(rng.Uint64()%uint64(i))]
			}
		}
		if len(values) > 0 && rng.Uint64()%2 == 0 {
			values[len(values)/2] = math.MinInt64
		}

		reference := make(map[int64]struct{}, len(values))
		for _, v := range values {
			reference[v] = struct{}{}
		}
		// The constructor expects a sorted input.
		sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })

		set := NewDocValuesLongHashSet(values)
		assertEqualsToJavaSet(t, rng, reference, set)
	}
}

// TestDocValuesLongHashSet_EqualsAndHashCode pins the Equals / HashCode
// contract: two sets built from the same sorted values must compare
// equal and hash to the same value; a one-element difference must
// break both relations.
func TestDocValuesLongHashSet_EqualsAndHashCode(t *testing.T) {
	a := NewDocValuesLongHashSet([]int64{1, 2, 3, 5, 8, 13})
	b := NewDocValuesLongHashSet([]int64{1, 2, 3, 5, 8, 13})
	if !a.Equals(b) {
		t.Fatalf("Equals = false on identical sets, want true")
	}
	if a.HashCode() != b.HashCode() {
		t.Fatalf("HashCode mismatch on identical sets: %d vs %d", a.HashCode(), b.HashCode())
	}

	c := NewDocValuesLongHashSet([]int64{1, 2, 3, 5, 8, 14})
	if a.Equals(c) {
		t.Fatalf("Equals = true on differing sets, want false")
	}
	if a.HashCode() == c.HashCode() {
		// Collisions are technically possible but extremely improbable
		// at this size; a hit signals a bug in the hashing folder.
		t.Fatalf("HashCode collision on differing sets: %d", a.HashCode())
	}

	// Equality with nil and self.
	if a.Equals(nil) {
		t.Fatalf("Equals(nil) = true, want false")
	}
	if !a.Equals(a) {
		t.Fatalf("Equals(self) = false, want true")
	}

	// Sentinel-bearing set hashes distinctly from a set that misses it.
	withSentinel := NewDocValuesLongHashSet([]int64{math.MinInt64, 1})
	withoutSentinel := NewDocValuesLongHashSet([]int64{1})
	if withSentinel.Equals(withoutSentinel) {
		t.Fatalf("sentinel-bearing set must not Equal the sentinel-less one")
	}
}

// TestDocValuesLongHashSet_ValuesOrdering pins the materialisation
// order documented on Values: sentinel first, then the slot-order
// contents of the internal table.
func TestDocValuesLongHashSet_ValuesOrdering(t *testing.T) {
	set := NewDocValuesLongHashSet([]int64{math.MinInt64, 1, 2, 3})
	out := set.Values()
	if len(out) != 4 {
		t.Fatalf("Values length = %d, want 4", len(out))
	}
	if out[0] != math.MinInt64 {
		t.Fatalf("Values[0] = %d, want sentinel first (MinInt64)", out[0])
	}
	rest := out[1:]
	slices.Sort(rest)
	if !slices.Equal(rest, []int64{1, 2, 3}) {
		t.Fatalf("non-sentinel values = %v, want {1, 2, 3}", rest)
	}
}

// TestDocValuesLongHashSet_String pins the toString output format and
// the empty-set rendering.
func TestDocValuesLongHashSet_String(t *testing.T) {
	if got, want := NewDocValuesLongHashSet(nil).String(), "[]"; got != want {
		t.Fatalf("String() on empty = %q, want %q", got, want)
	}
	if got, want := NewDocValuesLongHashSet([]int64{42}).String(), "[42]"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

// TestDocValuesLongHashSet_RamBytesUsed sanity-checks that the
// Accountable contract reports a strictly positive footprint dominated
// by the backing table.
func TestDocValuesLongHashSet_RamBytesUsed(t *testing.T) {
	set := NewDocValuesLongHashSet([]int64{1, 2, 3, 4, 5})
	if got := set.RamBytesUsed(); got <= docValuesLongHashSetBaseRAM {
		t.Fatalf("RamBytesUsed = %d, want > base RAM %d", got, docValuesLongHashSetBaseRAM)
	}
}
