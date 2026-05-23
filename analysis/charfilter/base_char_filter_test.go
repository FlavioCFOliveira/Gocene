// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package charfilter

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// TestBaseCharFilter_NoMappings — Correct returns the input unchanged when no
// offset corrections have been recorded.
// ---------------------------------------------------------------------------

func TestBaseCharFilter_NoMappings(t *testing.T) {
	f := NewBaseCharFilter(strings.NewReader("hello"))
	for _, off := range []int{0, 1, 5, 100} {
		if got := f.Correct(off); got != off {
			t.Errorf("Correct(%d) = %d; want %d (no mappings)", off, got, off)
		}
	}
}

// ---------------------------------------------------------------------------
// TestBaseCharFilter_SingleMapping — Correct applies a single recorded entry.
// ---------------------------------------------------------------------------

func TestBaseCharFilter_SingleMapping(t *testing.T) {
	f := NewBaseCharFilter(strings.NewReader(""))
	// At output offset 3, cumulative diff is +2 (two chars were inserted before pos 3).
	f.AddOffCorrectMap(3, 2)

	cases := []struct {
		off, want int
	}{
		{0, 0}, // before the mapping → no correction
		{2, 2}, // before the mapping → no correction
		{3, 5}, // at the mapping → 3+2=5
		{5, 7}, // after the mapping → 5+2=7
		{10, 12},
	}
	for _, tc := range cases {
		if got := f.Correct(tc.off); got != tc.want {
			t.Errorf("Correct(%d) = %d; want %d", tc.off, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// TestBaseCharFilter_MultipleStepMappings — mirrors the Lucene test that
// checks correct() across a sequence of cumulative diffs.
// ---------------------------------------------------------------------------

func TestBaseCharFilter_MultipleStepMappings(t *testing.T) {
	f := NewBaseCharFilter(strings.NewReader(""))
	// Simulate three deletions (each removes one char, so diff decreases by 1):
	//   output offset 2 → diff -1 (one char removed up to here)
	//   output offset 5 → diff -2 (two chars removed up to here)
	//   output offset 9 → diff -3 (three chars removed up to here)
	f.AddOffCorrectMap(2, -1)
	f.AddOffCorrectMap(5, -2)
	f.AddOffCorrectMap(9, -3)

	cases := []struct {
		off, want int
	}{
		{0, 0},
		{1, 1},
		{2, 1},  // 2 + (-1) = 1
		{3, 2},  // 3 + (-1) = 2
		{5, 3},  // 5 + (-2) = 3
		{7, 5},  // 7 + (-2) = 5
		{9, 6},  // 9 + (-3) = 6
		{12, 9}, // 12 + (-3) = 9
	}
	for _, tc := range cases {
		if got := f.Correct(tc.off); got != tc.want {
			t.Errorf("Correct(%d) = %d; want %d", tc.off, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// TestBaseCharFilter_OverwriteSameOffset — adding a mapping at the same
// offset as the last entry replaces it.
// ---------------------------------------------------------------------------

func TestBaseCharFilter_OverwriteSameOffset(t *testing.T) {
	f := NewBaseCharFilter(strings.NewReader(""))
	f.AddOffCorrectMap(5, 10)
	f.AddOffCorrectMap(5, 20) // overwrite
	if got := f.Correct(5); got != 25 {
		t.Errorf("Correct(5) = %d; want 25 after overwrite", got)
	}
}

// ---------------------------------------------------------------------------
// TestBaseCharFilter_GetLastCumulativeDiff — reports the last recorded diff.
// ---------------------------------------------------------------------------

func TestBaseCharFilter_GetLastCumulativeDiff(t *testing.T) {
	f := NewBaseCharFilter(strings.NewReader(""))
	if got := f.GetLastCumulativeDiff(); got != 0 {
		t.Errorf("GetLastCumulativeDiff() = %d; want 0 (no mappings)", got)
	}

	f.AddOffCorrectMap(3, 7)
	if got := f.GetLastCumulativeDiff(); got != 7 {
		t.Errorf("GetLastCumulativeDiff() = %d; want 7", got)
	}

	f.AddOffCorrectMap(10, 15)
	if got := f.GetLastCumulativeDiff(); got != 15 {
		t.Errorf("GetLastCumulativeDiff() = %d; want 15", got)
	}
}

// ---------------------------------------------------------------------------
// TestBaseCharFilter_GrowthBeyondInitialCapacity — verify the map grows when
// more than 64 entries are added without panic or data loss.
// ---------------------------------------------------------------------------

func TestBaseCharFilter_GrowthBeyondInitialCapacity(t *testing.T) {
	f := NewBaseCharFilter(strings.NewReader(""))
	n := 200
	for i := 0; i < n; i++ {
		f.AddOffCorrectMap(i, i*2)
	}
	// Last entry: off=199, diff=398.
	if got := f.GetLastCumulativeDiff(); got != (n-1)*2 {
		t.Errorf("GetLastCumulativeDiff() = %d; want %d", got, (n-1)*2)
	}
	// Spot-check a mid entry.
	mid := n / 2
	want := mid + mid*2 // mid + diff[mid] = mid*3
	if got := f.Correct(mid); got != want {
		t.Errorf("Correct(%d) = %d; want %d", mid, got, want)
	}
}

// ---------------------------------------------------------------------------
// TestBaseCharFilter_CorrectOffsetChaining — CorrectOffset chains through a
// nested BaseCharFilter that also has mappings.
// ---------------------------------------------------------------------------

func TestBaseCharFilter_CorrectOffsetChaining(t *testing.T) {
	inner := NewBaseCharFilter(strings.NewReader("inner"))
	inner.AddOffCorrectMap(5, 10) // inner adds 10 at offset 5

	outer := NewBaseCharFilter(inner)
	outer.AddOffCorrectMap(3, 2) // outer adds 2 at offset 3

	// For off=3:
	//   outer.Correct(3)  = 3+2 = 5
	//   inner.CorrectOffset(5) = 5+10 = 15
	if got := outer.CorrectOffset(3); got != 15 {
		t.Errorf("CorrectOffset(3) = %d; want 15 (chained)", got)
	}
}

// ---------------------------------------------------------------------------
// TestBaseCharFilter_Read — Read proxies to the wrapped reader.
// ---------------------------------------------------------------------------

func TestBaseCharFilter_Read(t *testing.T) {
	text := "hello"
	f := NewBaseCharFilter(strings.NewReader(text))
	buf := make([]byte, len(text))
	n, err := f.Read(buf)
	if err != nil && err.Error() != "EOF" {
		t.Fatalf("Read: %v", err)
	}
	if n != len(text) || string(buf[:n]) != text {
		t.Errorf("Read = %q; want %q", string(buf[:n]), text)
	}
}
