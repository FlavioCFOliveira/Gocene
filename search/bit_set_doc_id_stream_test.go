// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// No Java test peer (BitSetDocIdStreamTest does not exist in Lucene 10.4.0).
// These tests cover NewBitSetDocIdStream, MayHaveRemaining, ForEachUpTo,
// CountUpTo, IntoArrayUpTo, and the offset/max clamping logic.

package search_test

import (
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

// makeBitSet builds a *util.FixedBitSet of size numBits and sets the given
// bit positions (local, 0-based).
func makeBitSet(numBits int, setBits ...int) *util.FixedBitSet {
	fs, err := util.NewFixedBitSet(numBits)
	if err != nil {
		panic(err)
	}
	for _, b := range setBits {
		fs.Set(b)
	}
	return fs
}

// collectAll drains stream via ForEachAll and returns doc IDs.
func collectAll(t *testing.T, s search.DocIdStream) []int {
	t.Helper()
	var got []int
	err := search.ForEachAll(s, func(docID int) error {
		got = append(got, docID)
		return nil
	})
	if err != nil {
		t.Fatalf("ForEachAll error: %v", err)
	}
	return got
}

// ─── construction ────────────────────────────────────────────────────────────

func TestBitSetDocIdStream_EmptyBitSet(t *testing.T) {
	s := search.NewBitSetDocIdStream(makeBitSet(64), 0)
	// MayHaveRemaining is a structural bound check (upTo < max), not a
	// "bits exist" check. An empty bitset with length 64 still reports
	// MayHaveRemaining=true until ForEachUpTo/CountUpTo advances upTo to max.
	// After consuming all, it must return false.
	got := collectAll(t, s)
	if len(got) != 0 {
		t.Errorf("got %v, want []", got)
	}
	if s.MayHaveRemaining() {
		t.Error("MayHaveRemaining() = true after full consume of empty bitset, want false")
	}
}

func TestBitSetDocIdStream_AllBitsSet(t *testing.T) {
	fs, err := util.NewFixedBitSet(4)
	if err != nil {
		t.Fatal(err)
	}
	fs.SetAll()
	s := search.NewBitSetDocIdStream(fs, 0)
	got := collectAll(t, s)
	want := []int{0, 1, 2, 3}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i, v := range want {
		if got[i] != v {
			t.Errorf("got[%d] = %d, want %d", i, got[i], v)
		}
	}
}

// ─── offset ──────────────────────────────────────────────────────────────────

func TestBitSetDocIdStream_Offset(t *testing.T) {
	// Bits 1, 3 set locally → global 101, 103 with offset 100.
	s := search.NewBitSetDocIdStream(makeBitSet(8, 1, 3), 100)
	got := collectAll(t, s)
	want := []int{101, 103}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i, v := range want {
		if got[i] != v {
			t.Errorf("got[%d] = %d, want %d", i, got[i], v)
		}
	}
}

// ─── MayHaveRemaining ─────────────────────────────────────────────────────────

func TestBitSetDocIdStream_MayHaveRemaining(t *testing.T) {
	s := search.NewBitSetDocIdStream(makeBitSet(16, 5), 0)
	if !s.MayHaveRemaining() {
		t.Error("MayHaveRemaining() = false before consuming, want true")
	}
	// Consume everything.
	_ = collectAll(t, s)
	if s.MayHaveRemaining() {
		t.Error("MayHaveRemaining() = true after full consume, want false")
	}
}

// ─── ForEachUpTo ─────────────────────────────────────────────────────────────

func TestBitSetDocIdStream_ForEachUpTo_Partial(t *testing.T) {
	// Bits 2, 5, 8 set; offset 0. Consume [0, 6) then [6, MAX).
	s := search.NewBitSetDocIdStream(makeBitSet(16, 2, 5, 8), 0)

	var first []int
	if err := s.ForEachUpTo(6, func(d int) error {
		first = append(first, d)
		return nil
	}); err != nil {
		t.Fatalf("ForEachUpTo(6) error: %v", err)
	}
	if len(first) != 2 || first[0] != 2 || first[1] != 5 {
		t.Errorf("first = %v, want [2 5]", first)
	}

	var second []int
	if err := search.ForEachAll(s, func(d int) error {
		second = append(second, d)
		return nil
	}); err != nil {
		t.Fatalf("ForEachAll error: %v", err)
	}
	if len(second) != 1 || second[0] != 8 {
		t.Errorf("second = %v, want [8]", second)
	}
}

func TestBitSetDocIdStream_ForEachUpTo_LowerBoundIsNoop(t *testing.T) {
	s := search.NewBitSetDocIdStream(makeBitSet(8, 3), 0)
	// ForEachUpTo with upTo <= current (0) must be no-op.
	var called int
	if err := s.ForEachUpTo(0, func(_ int) error { called++; return nil }); err != nil {
		t.Fatal(err)
	}
	if called != 0 {
		t.Errorf("consumer called %d times, want 0", called)
	}
}

func TestBitSetDocIdStream_ForEachUpTo_ErrorPropagated(t *testing.T) {
	sentinel := errors.New("stop")
	s := search.NewBitSetDocIdStream(makeBitSet(16, 2, 5, 8), 0)
	err := search.ForEachAll(s, func(_ int) error { return sentinel })
	if !errors.Is(err, sentinel) {
		t.Errorf("error = %v, want sentinel", err)
	}
}

// ─── CountUpTo ───────────────────────────────────────────────────────────────

func TestBitSetDocIdStream_CountUpTo(t *testing.T) {
	s := search.NewBitSetDocIdStream(makeBitSet(16, 1, 4, 7, 12), 0)
	n, err := s.CountUpTo(8)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("CountUpTo(8) = %d, want 3", n)
	}
	// Remaining: bit 12.
	rest, err := search.CountAll(s)
	if err != nil {
		t.Fatal(err)
	}
	if rest != 1 {
		t.Errorf("CountAll after partial = %d, want 1", rest)
	}
}

func TestBitSetDocIdStream_CountUpTo_LowerBoundIsNoop(t *testing.T) {
	s := search.NewBitSetDocIdStream(makeBitSet(8, 3), 0)
	n, err := s.CountUpTo(0)
	if err != nil || n != 0 {
		t.Errorf("CountUpTo(0) = (%d, %v), want (0, nil)", n, err)
	}
}

// ─── IntoArrayUpTo ───────────────────────────────────────────────────────────

func TestBitSetDocIdStream_IntoArrayUpTo_Full(t *testing.T) {
	s := search.NewBitSetDocIdStream(makeBitSet(16, 0, 7, 15), 0)
	buf := make([]int, 8)
	n := search.IntoArray(s, buf)
	if n != 3 {
		t.Errorf("IntoArray n = %d, want 3", n)
	}
	want := []int{0, 7, 15}
	for i, v := range want {
		if buf[i] != v {
			t.Errorf("buf[%d] = %d, want %d", i, buf[i], v)
		}
	}
}

func TestBitSetDocIdStream_IntoArrayUpTo_ArrayFillsEarly(t *testing.T) {
	// Bits 0, 5, 10, 15; array capacity 2.
	s := search.NewBitSetDocIdStream(makeBitSet(16, 0, 5, 10, 15), 0)
	buf := make([]int, 2)
	n := s.IntoArrayUpTo(search.NO_MORE_DOCS, buf)
	if n != 2 {
		t.Errorf("n = %d, want 2", n)
	}
	if buf[0] != 0 || buf[1] != 5 {
		t.Errorf("buf = %v, want [0 5]", buf[:2])
	}
	// Stream should still have docs 10 and 15.
	if !s.MayHaveRemaining() {
		t.Error("MayHaveRemaining() = false, want true after partial fill")
	}
	rest := make([]int, 4)
	n2 := search.IntoArray(s, rest)
	if n2 != 2 {
		t.Errorf("remaining n = %d, want 2", n2)
	}
	if rest[0] != 10 || rest[1] != 15 {
		t.Errorf("rest = %v, want [10 15]", rest[:2])
	}
}

func TestBitSetDocIdStream_IntoArrayUpTo_EmptyArrayNoop(t *testing.T) {
	s := search.NewBitSetDocIdStream(makeBitSet(8, 3), 0)
	n := s.IntoArrayUpTo(search.NO_MORE_DOCS, nil)
	if n != 0 {
		t.Errorf("n = %d, want 0 on nil array", n)
	}
	// Stream must still be usable.
	got := collectAll(t, s)
	if len(got) != 1 || got[0] != 3 {
		t.Errorf("got %v, want [3]", got)
	}
}

// ─── interface satisfaction ───────────────────────────────────────────────────

func TestBitSetDocIdStream_ImplementsDocIdStream(t *testing.T) {
	var _ search.DocIdStream = search.NewBitSetDocIdStream(makeBitSet(8), 0)
}
