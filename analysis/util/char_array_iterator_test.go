// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import "testing"

func TestCharArrayIterator_Navigation(t *testing.T) {
	buf := []rune("hello")
	it := NewCharArrayIterator()
	it.SetText(buf, 0, len(buf))

	// First/Last
	if got := it.First(); got != 'h' {
		t.Errorf("First() = %c, want h", got)
	}
	if got := it.Last(); got != 'o' {
		t.Errorf("Last() = %c, want o", got)
	}

	// SetIndex
	it.First()
	if got := it.SetIndex(2); got != 'l' {
		t.Errorf("SetIndex(2) = %c, want l", got)
	}

	// Next / Previous
	if got := it.Next(); got != 'l' {
		t.Errorf("Next() = %c, want l", got)
	}
	if got := it.Previous(); got != 'l' {
		t.Errorf("Previous() = %c, want l", got)
	}
}

func TestCharArrayIterator_BoundaryReturnsDone(t *testing.T) {
	buf := []rune("ab")
	it := NewCharArrayIterator()
	it.SetText(buf, 0, 2)

	it.Last()
	next := it.Next()
	if next != 0 {
		t.Errorf("Next() past end = %c, want 0 (done)", next)
	}

	it.First()
	prev := it.Previous()
	if prev != 0 {
		t.Errorf("Previous() before start = %c, want 0 (done)", prev)
	}
}

func TestCharArrayIterator_GetIndex(t *testing.T) {
	buf := []rune("xyz")
	it := NewCharArrayIterator()
	it.SetText(buf, 0, 3)

	it.SetIndex(1)
	if got := it.GetIndex(); got != 1 {
		t.Errorf("GetIndex() = %d, want 1", got)
	}
}

func TestCharArrayIterator_Clone(t *testing.T) {
	buf := []rune("clone")
	it := NewCharArrayIterator()
	it.SetText(buf, 0, len(buf))
	it.SetIndex(2)

	cp := it.Clone()
	if cp.GetIndex() != it.GetIndex() {
		t.Errorf("Clone index mismatch: %d vs %d", cp.GetIndex(), it.GetIndex())
	}
	// Advancing clone must not affect original.
	cp.Next()
	if cp.GetIndex() == it.GetIndex() {
		t.Error("Clone and original share state")
	}
}
