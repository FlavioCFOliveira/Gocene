// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package snowball

import "testing"

// TestSetCurrentGetCurrent verifies SetCurrent initialises the cursor/limit
// state and GetCurrent reflects the (possibly truncated) buffer.
func TestSetCurrentGetCurrent(t *testing.T) {
	t.Parallel()
	p := NewSnowballProgram()
	if got := p.GetCurrent(); got != "" {
		t.Fatalf("fresh program GetCurrent() = %q, want empty", got)
	}

	p.SetCurrent("hello")
	if got := p.GetCurrent(); got != "hello" {
		t.Errorf("GetCurrent() = %q, want hello", got)
	}
	if p.Cursor != 0 {
		t.Errorf("Cursor = %d, want 0", p.Cursor)
	}
	if p.Limit != 5 || p.Length != 5 {
		t.Errorf("Limit/Length = (%d,%d), want (5,5)", p.Limit, p.Length)
	}
	if p.Bra != 0 || p.Ket != 5 {
		t.Errorf("Bra/Ket = (%d,%d), want (0,5)", p.Bra, p.Ket)
	}
	if got := p.GetCurrentBufferLength(); got != 5 {
		t.Errorf("GetCurrentBufferLength() = %d, want 5", got)
	}
}

// TestEqSStr verifies forward literal matching advances the cursor on a match
// at the cursor and is a no-op on a mismatch.
func TestEqSStr(t *testing.T) {
	t.Parallel()
	p := NewSnowballProgram()
	p.SetCurrent("running")

	if !p.EqSStr("run") {
		t.Fatal("EqSStr(run) = false at start of \"running\", want true")
	}
	if p.Cursor != 3 {
		t.Errorf("Cursor after EqSStr(run) = %d, want 3", p.Cursor)
	}

	// Mismatch at the current cursor: no match, cursor unchanged.
	if p.EqSStr("XYZ") {
		t.Error("EqSStr(XYZ) = true on \"ning\", want false")
	}
	if p.Cursor != 3 {
		t.Errorf("Cursor after failed EqSStr = %d, want 3 (unchanged)", p.Cursor)
	}

	// Match the remainder.
	if !p.EqSStr("ning") {
		t.Error("EqSStr(ning) = false, want true")
	}
	if p.Cursor != 7 {
		t.Errorf("Cursor after EqSStr(ning) = %d, want 7", p.Cursor)
	}
}

// TestEqSBStr verifies backward literal matching from the limit, retreating the
// cursor on a match.
func TestEqSBStr(t *testing.T) {
	t.Parallel()
	p := NewSnowballProgram()
	p.SetCurrent("running")
	p.Cursor = p.Limit // backward matching starts from the end

	if !p.EqSBStr("ing") {
		t.Fatal("EqSBStr(ing) = false at end of \"running\", want true")
	}
	if p.Cursor != 4 {
		t.Errorf("Cursor after EqSBStr(ing) = %d, want 4", p.Cursor)
	}
	if p.EqSBStr("zz") {
		t.Error("EqSBStr(zz) = true, want false")
	}
	if p.Cursor != 4 {
		t.Errorf("Cursor after failed EqSBStr = %d, want 4 (unchanged)", p.Cursor)
	}
}

// TestFindAmong verifies the forward Among binary search returns the matching
// entry's result code and positions the cursor past the matched substring. The
// Among set must be sorted, as the search relies on ordering.
func TestFindAmong(t *testing.T) {
	t.Parallel()
	// Sorted suffix set with distinct result codes.
	among := []*Among{
		NewAmong("ed", -1, 1),
		NewAmong("ing", -1, 2),
		NewAmong("ings", 1, 3),
	}

	p := NewSnowballProgram()
	p.SetCurrent("ing")
	if got := p.FindAmong(among); got != 2 {
		t.Errorf("FindAmong on \"ing\" = %d, want 2", got)
	}
	if p.Cursor != 3 {
		t.Errorf("Cursor after FindAmong = %d, want 3 (past \"ing\")", p.Cursor)
	}

	// No matching entry returns 0.
	p.SetCurrent("xyz")
	if got := p.FindAmong(among); got != 0 {
		t.Errorf("FindAmong on \"xyz\" = %d, want 0 (no match)", got)
	}
}

// TestSliceFromAndDel verifies the [Bra,Ket) slice region is replaced by
// SliceFrom and removed by SliceDel, and that Length tracks the edit.
func TestSliceFromAndDel(t *testing.T) {
	t.Parallel()
	p := NewSnowballProgram()
	p.SetCurrent("happiness")

	// Replace the "iness" suffix (indices 4..9 of "happiness") with "y" to
	// obtain "happy".
	p.Bra = 4
	p.Ket = 9
	p.SliceFromStr("y")
	if got := p.GetCurrent(); got != "happy" {
		t.Errorf("after SliceFrom(iness->y), GetCurrent() = %q, want happy", got)
	}
	if p.Length != 5 {
		t.Errorf("Length after SliceFrom = %d, want 5", p.Length)
	}

	// Now delete the "y" suffix -> "happ".
	p.Bra = 4
	p.Ket = 5
	p.SliceDel()
	if got := p.GetCurrent(); got != "happ" {
		t.Errorf("after SliceDel(y), GetCurrent() = %q, want happ", got)
	}
	if p.Length != 4 {
		t.Errorf("Length after SliceDel = %d, want 4", p.Length)
	}
}

// TestReplaceSStr verifies the return value (length delta) and the resulting
// buffer for growth, shrink, and same-length replacements.
func TestReplaceSStr(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name           string
		input          string
		bra, ket       int
		repl           string
		wantAdjustment int
		wantResult     string
	}{
		{"shrink", "testing", 4, 7, "", -3, "test"},   // drop "ing"
		{"grow", "cat", 3, 3, "ss", 2, "catss"},       // append "ss"
		{"same length", "cars", 3, 4, "t", 0, "cart"}, // 's' -> 't'
		{"replace middle", "abcdef", 2, 4, "X", -1, "abXef"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := NewSnowballProgram()
			p.SetCurrent(tc.input)
			got := p.ReplaceSStr(tc.bra, tc.ket, tc.repl)
			if got != tc.wantAdjustment {
				t.Errorf("ReplaceSStr adjustment = %d, want %d", got, tc.wantAdjustment)
			}
			if res := p.GetCurrent(); res != tc.wantResult {
				t.Errorf("ReplaceSStr result = %q, want %q", res, tc.wantResult)
			}
		})
	}
}

// TestInGrouping verifies the forward grouping membership test against an
// explicit bitset. The bitset marks the lowercase English vowels relative to
// base 'a': bit (ch - 'a') set means "in group". InGrouping advances the cursor
// only when the character at the cursor is in the group.
func TestInGrouping(t *testing.T) {
	t.Parallel()
	// Build a bitset over 'a'..'z' (26 letters -> 4 bytes) with the vowels set.
	const base = 'a'
	g := make([]byte, 4)
	for _, v := range []rune{'a', 'e', 'i', 'o', 'u'} {
		idx := v - base
		g[idx>>3] |= 1 << (idx & 7)
	}

	p := NewSnowballProgram()
	p.SetCurrent("aebc")

	// 'a' is a vowel: in-group, cursor advances.
	if !p.InGrouping(g, base, 'z') {
		t.Fatal("InGrouping on 'a' = false, want true")
	}
	if p.Cursor != 1 {
		t.Errorf("Cursor after vowel 'a' = %d, want 1", p.Cursor)
	}
	// 'e' is a vowel: in-group, cursor advances.
	if !p.InGrouping(g, base, 'z') {
		t.Fatal("InGrouping on 'e' = false, want true")
	}
	if p.Cursor != 2 {
		t.Errorf("Cursor after vowel 'e' = %d, want 2", p.Cursor)
	}
	// 'b' is not a vowel: not in-group, cursor unchanged.
	if p.InGrouping(g, base, 'z') {
		t.Error("InGrouping on 'b' = true, want false")
	}
	if p.Cursor != 2 {
		t.Errorf("Cursor after consonant 'b' = %d, want 2 (unchanged)", p.Cursor)
	}

	// OutGrouping is the complement: 'b' is outside the vowel group, so it
	// matches and advances.
	if !p.OutGrouping(g, base, 'z') {
		t.Error("OutGrouping on 'b' = false, want true")
	}
	if p.Cursor != 3 {
		t.Errorf("Cursor after OutGrouping('b') = %d, want 3", p.Cursor)
	}
}

// TestCopyFrom verifies CopyFrom transfers the full cursor/limit state.
func TestCopyFrom(t *testing.T) {
	t.Parallel()
	src := NewSnowballProgram()
	src.SetCurrent("source")
	src.Cursor = 2
	src.Ket = 4

	dst := NewSnowballProgram()
	dst.CopyFrom(src)

	if dst.GetCurrent() != "source" {
		t.Errorf("CopyFrom current = %q, want source", dst.GetCurrent())
	}
	if dst.Cursor != 2 || dst.Ket != 4 || dst.Limit != src.Limit {
		t.Errorf("CopyFrom state = (cursor=%d,ket=%d,limit=%d), want (2,4,%d)",
			dst.Cursor, dst.Ket, dst.Limit, src.Limit)
	}
}
