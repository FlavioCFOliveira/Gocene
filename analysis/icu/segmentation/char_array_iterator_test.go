// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package segmentation_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis/icu/segmentation"
)

// TestCharArrayIterator_BasicUsage tests basic navigation.
// Port of TestCharArrayIterator.testBasicUsage.
func TestCharArrayIterator_BasicUsage(t *testing.T) {
	ci := segmentation.NewCharArrayIterator()
	text := []rune("testing")
	ci.SetText(text, 0, len(text))

	if ci.BeginIndex() != 0 {
		t.Errorf("BeginIndex: got %d, want 0", ci.BeginIndex())
	}
	if ci.EndIndex() != 7 {
		t.Errorf("EndIndex: got %d, want 7", ci.EndIndex())
	}
	if ci.GetIndex() != 0 {
		t.Errorf("GetIndex after SetText: got %d, want 0", ci.GetIndex())
	}
	if got := ci.Current(); got != 't' {
		t.Errorf("Current: got %q, want 't'", got)
	}
	if got := ci.Next(); got != 'e' {
		t.Errorf("Next: got %q, want 'e'", got)
	}
	if got := ci.Last(); got != 'g' {
		t.Errorf("Last: got %q, want 'g'", got)
	}
	if got := ci.Previous(); got != 'n' {
		t.Errorf("Previous: got %q, want 'n'", got)
	}
	if got := ci.First(); got != 't' {
		t.Errorf("First: got %q, want 't'", got)
	}
	// previous before first → 0 (DONE equivalent)
	if got := ci.Previous(); got != 0 {
		t.Errorf("Previous at start: got %q, want 0 (DONE)", got)
	}
}

// TestCharArrayIterator_First tests First() semantics on normal and empty text.
// Port of TestCharArrayIterator.testFirst.
func TestCharArrayIterator_First(t *testing.T) {
	ci := segmentation.NewCharArrayIterator()
	text := []rune("testing")
	ci.SetText(text, 0, len(text))
	ci.Next()
	if got := ci.First(); got != 't' {
		t.Errorf("First: got %q, want 't'", got)
	}
	if ci.GetIndex() != ci.BeginIndex() {
		t.Errorf("after First: index=%d, want BeginIndex=%d", ci.GetIndex(), ci.BeginIndex())
	}
	// empty text
	ci.SetText([]rune{}, 0, 0)
	if got := ci.First(); got != 0 {
		t.Errorf("First on empty: got %q, want 0 (DONE)", got)
	}
}

// TestCharArrayIterator_Last tests Last() semantics on normal and empty text.
// Port of TestCharArrayIterator.testLast.
func TestCharArrayIterator_Last(t *testing.T) {
	ci := segmentation.NewCharArrayIterator()
	text := []rune("testing")
	ci.SetText(text, 0, len(text))
	if got := ci.Last(); got != 'g' {
		t.Errorf("Last: got %q, want 'g'", got)
	}
	if ci.GetIndex() != ci.EndIndex()-1 {
		t.Errorf("after Last: index=%d, want EndIndex-1=%d", ci.GetIndex(), ci.EndIndex()-1)
	}
	// empty text
	ci.SetText([]rune{}, 0, 0)
	if got := ci.Last(); got != 0 {
		t.Errorf("Last on empty: got %q, want 0 (DONE)", got)
	}
	if ci.GetIndex() != ci.EndIndex() {
		t.Errorf("after Last on empty: index=%d, want EndIndex=%d", ci.GetIndex(), ci.EndIndex())
	}
}

// TestCharArrayIterator_Current tests Current() returning DONE past end.
// Port of TestCharArrayIterator.testCurrent.
func TestCharArrayIterator_Current(t *testing.T) {
	ci := segmentation.NewCharArrayIterator()
	text := []rune("testing")
	ci.SetText(text, 0, len(text))
	if got := ci.Current(); got != 't' {
		t.Errorf("Current at start: got %q, want 't'", got)
	}
	ci.Last()
	ci.Next()
	// index is now at limit — DONE
	if got := ci.Current(); got != 0 {
		t.Errorf("Current at limit: got %q, want 0 (DONE)", got)
	}
}

// TestCharArrayIterator_Next tests Next() incrementing and DONE at end.
// Port of TestCharArrayIterator.testNext.
func TestCharArrayIterator_Next(t *testing.T) {
	ci := segmentation.NewCharArrayIterator()
	ci.SetText([]rune("te"), 0, 2)
	if got := ci.Next(); got != 'e' {
		t.Errorf("Next: got %q, want 'e'", got)
	}
	if ci.GetIndex() != 1 {
		t.Errorf("GetIndex after Next: got %d, want 1", ci.GetIndex())
	}
	if got := ci.Next(); got != 0 {
		t.Errorf("Next at end: got %q, want 0 (DONE)", got)
	}
	if ci.GetIndex() != ci.EndIndex() {
		t.Errorf("GetIndex after end Next: got %d, want EndIndex=%d", ci.GetIndex(), ci.EndIndex())
	}
}

// TestCharArrayIterator_SetIndex tests SetIndex out-of-bounds panic.
// Port of TestCharArrayIterator.testSetIndex.
func TestCharArrayIterator_SetIndex(t *testing.T) {
	ci := segmentation.NewCharArrayIterator()
	ci.SetText([]rune("test"), 0, 4)
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for out-of-bounds SetIndex")
		}
	}()
	ci.SetIndex(5)
}

// TestCharArrayIterator_Clone tests that Clone produces an independent copy.
// Port of TestCharArrayIterator.testClone.
func TestCharArrayIterator_Clone(t *testing.T) {
	text := []rune("testing")
	ci := segmentation.NewCharArrayIterator()
	ci.SetText(text, 0, len(text))
	ci.Next()

	ci2 := ci.Clone()
	if ci.GetIndex() != ci2.GetIndex() {
		t.Errorf("clone GetIndex: got %d, want %d", ci2.GetIndex(), ci.GetIndex())
	}
	got1 := ci.Next()
	got2 := ci2.Next()
	if got1 != got2 {
		t.Errorf("clone Next mismatch: ci=%q ci2=%q", got1, got2)
	}
	gotLast1 := ci.Last()
	gotLast2 := ci2.Last()
	if gotLast1 != gotLast2 {
		t.Errorf("clone Last mismatch: ci=%q ci2=%q", gotLast1, gotLast2)
	}
}
